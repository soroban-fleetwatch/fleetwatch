package follow

import (
	"bufio"
	"bytes"
	"compress/gzip"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"io"
	"os"
	"path/filepath"
	"reflect"

	"github.com/soroban-fleetwatch/fleetwatch/internal/history"
	"github.com/soroban-fleetwatch/fleetwatch/internal/jsoninput"
	bolt "go.etcd.io/bbolt"
)

// Both the compressed file and expanded NDJSON have independent hard bounds.
const MaxArchiveBytes int64 = 512 << 20

type archiveHeader struct {
	SchemaVersion int              `json:"schema_version"`
	Kind          string           `json:"kind"`
	Baseline      history.Baseline `json:"baseline"`
}
type archiveTrailer struct {
	Head          Head   `json:"head"`
	ContentSHA256 string `json:"content_sha256"`
}
type archiveLine struct {
	Header  *archiveHeader  `json:"header,omitempty"`
	Record  *Record         `json:"record,omitempty"`
	Trailer *archiveTrailer `json:"trailer,omitempty"`
}

// ArchiveReceipt reports local replay consistency, never chain authentication.
// The journal is reconstructed from retained events, not delivered or cleared.
type ArchiveReceipt struct {
	SchemaVersion          int    `json:"schema_version"`
	Kind                   string `json:"kind"`
	ArchiveSHA256          string `json:"archive_sha256"`
	ContentSHA256          string `json:"content_sha256"`
	BaselineDigest         string `json:"baseline_digest"`
	BaselineHeaderHash     string `json:"baseline_header_hash"`
	BaselineLedger         uint32 `json:"baseline_ledger"`
	ProcessedThrough       uint32 `json:"processed_through"`
	RetainedLedgers        int    `json:"retained_ledgers"`
	ReconstructedFindings  int    `json:"reconstructed_findings"`
	SemanticValidation     string `json:"semantic_validation"`
	Offline                bool   `json:"offline"`
	SourceAuthentication   string `json:"source_authentication"`
	BaselineAuthentication string `json:"baseline_authentication"`
	MetadataCommitment     string `json:"metadata_commitment"`
	MembershipCoverage     string `json:"membership_coverage"`
	HistoryCompleteness    string `json:"history_completeness"`
	FinalVerdict           string `json:"final_verdict"`
}

type archiveWriter struct {
	ctx context.Context
	w   io.Writer
	n   int64
}

func (w *archiveWriter) Write(p []byte) (int, error) {
	if err := w.ctx.Err(); err != nil {
		return 0, err
	}
	if int64(len(p)) > MaxArchiveBytes-w.n {
		return 0, ErrCapacity
	}
	n, err := w.w.Write(p)
	w.n += int64(n)
	return n, err
}

type archiveReader struct {
	ctx context.Context
	r   io.Reader
}

func (r archiveReader) Read(p []byte) (int, error) {
	if err := r.ctx.Err(); err != nil {
		return 0, err
	}
	return r.r.Read(p)
}

// ExportArchive creates a new fsynced, atomically published gzip NDJSON file.
// It verifies its temporary archive before publication. Existing destinations,
// including symlinks introduced during export, are never replaced. No DB records
// are deleted or modified. Callers must first Open/validate the stopped store.
func (s *Store) ExportArchive(ctx context.Context, destination string) (ArchiveReceipt, error) {
	return s.exportArchive(ctx, destination, nil)
}

func (s *Store) exportArchive(ctx context.Context, destination string, hook func() error) (ArchiveReceipt, error) {
	var empty ArchiveReceipt
	if ctx.Err() != nil {
		return empty, ctx.Err()
	}
	if destination == "" {
		return empty, errors.New("archive destination required")
	}
	if _, err := os.Lstat(destination); !os.IsNotExist(err) {
		return empty, errors.New("archive destination exists or is unavailable")
	}
	f, err := os.CreateTemp(filepath.Dir(destination), ".fleetwatch-archive-*")
	if err != nil {
		return empty, err
	}
	defer os.Remove(f.Name())
	defer f.Close()
	compressed := &archiveWriter{ctx: ctx, w: f}
	gz := gzip.NewWriter(compressed)
	expanded := &archiveWriter{ctx: ctx, w: gz}
	contentHash := sha256.New()
	write := func(line archiveLine, digest bool) error {
		raw := append(encode(line), '\n')
		if len(raw) > jsoninput.MaxBytes {
			return ErrCapacity
		}
		if digest {
			_, _ = contentHash.Write(raw)
		}
		_, err := expanded.Write(raw)
		return err
	}
	s.mu.Lock()
	err = s.db.View(func(tx *bolt.Tx) error {
		root := tx.Bucket(rootName)
		var baseline history.Baseline
		if jsoninput.Decode(bytes.NewReader(root.Get([]byte("baseline"))), &baseline) != nil || !bytes.Equal(root.Get([]byte("head")), encode(s.head)) {
			return ErrState
		}
		if err := write(archiveLine{Header: &archiveHeader{1, "fleetwatch_follow_evidence_archive", baseline}}, true); err != nil {
			return err
		}
		if err := root.Bucket(ledgerName).ForEach(func(key, raw []byte) error {
			var record Record
			if jsoninput.Decode(bytes.NewReader(raw), &record) != nil || !bytes.Equal(key, seqKey(record.Evidence.Sequence)) {
				return ErrState
			}
			return write(archiveLine{Record: &record}, true)
		}); err != nil {
			return err
		}
		return write(archiveLine{Trailer: &archiveTrailer{s.head, hex.EncodeToString(contentHash.Sum(nil))}}, false)
	})
	s.mu.Unlock()
	closeErr := gz.Close()
	if err != nil {
		return empty, err
	}
	if closeErr != nil {
		return empty, closeErr
	}
	if err := f.Sync(); err != nil {
		return empty, err
	}
	if _, err := f.Seek(0, io.SeekStart); err != nil {
		return empty, err
	}
	receipt, err := VerifyArchive(ctx, f)
	if err != nil {
		return empty, err
	}
	if err := f.Close(); err != nil {
		return empty, err
	}
	if hook != nil {
		if err := hook(); err != nil {
			return empty, err
		}
	}
	if ctx.Err() != nil {
		return empty, ctx.Err()
	}
	if err := os.Link(f.Name(), destination); err != nil {
		return empty, err
	}
	// Publication has happened: retain destination even if directory fsync fails.
	dir, err := os.Open(filepath.Dir(destination))
	if err != nil {
		return empty, err
	}
	err = dir.Sync()
	closeErr = dir.Close()
	if err != nil {
		return empty, err
	}
	if closeErr != nil {
		return empty, closeErr
	}
	return receipt, nil
}

// VerifyArchive streams and replays an untrusted archive without creating state,
// querying a source, or accepting its cached final projection as a baseline.
func VerifyArchive(ctx context.Context, input io.Reader) (ArchiveReceipt, error) {
	return verifyArchive(ctx, input, MaxArchiveBytes, MaxArchiveBytes)
}

func verifyArchive(ctx context.Context, input io.Reader, compressedLimit, expandedLimit int64) (ArchiveReceipt, error) {
	return replayArchive(ctx, input, compressedLimit, expandedLimit, nil, nil)
}

// Restore hooks receive only individually validated content. Their output must
// remain unpublished until the complete stream, trailer and digest are checked.
func replayArchive(ctx context.Context, input io.Reader, compressedLimit, expandedLimit int64, baselineHook func(history.Baseline) error, recordHook func(Record) error) (ArchiveReceipt, error) {
	var empty ArchiveReceipt
	fileHash, contentHash := sha256.New(), sha256.New()
	compressed := &io.LimitedReader{R: io.TeeReader(archiveReader{ctx, input}, fileHash), N: compressedLimit + 1}
	gz, err := gzip.NewReader(compressed)
	if err != nil {
		return empty, err
	}
	defer gz.Close()
	expanded := &io.LimitedReader{R: gz, N: expandedLimit + 1}
	scanner := bufio.NewScanner(expanded)
	scanner.Buffer(make([]byte, 64<<10), jsoninput.MaxBytes+1)
	scanner.Split(func(data []byte, atEOF bool) (int, []byte, error) {
		if i := bytes.IndexByte(data, '\n'); i >= 0 {
			return i + 1, data[:i], nil
		}
		if atEOF && len(data) > 0 {
			return 0, nil, errors.New("archive line missing terminator")
		}
		return 0, nil, nil
	})
	var header *archiveHeader
	var trailer *archiveTrailer
	var head Head
	var baselineLedger uint32
	for scanner.Scan() {
		if ctx.Err() != nil {
			return empty, ctx.Err()
		}
		raw := scanner.Bytes()
		var line archiveLine
		if trailer != nil || jsoninput.Decode(bytes.NewReader(raw), &line) != nil || !bytes.Equal(raw, encode(line)) {
			return empty, ErrState
		}
		if header == nil {
			if line.Header == nil || line.Record != nil || line.Trailer != nil || line.Header.SchemaVersion != 1 || line.Header.Kind != "fleetwatch_follow_evidence_archive" {
				return empty, ErrState
			}
			header = line.Header
			p, err := history.NewProjection(header.Baseline)
			if err != nil {
				return empty, ErrEvidence
			}
			baselineLedger = p.Snapshot.Ledger
			head = Head{Projection: p, RetainedPayloadBytes: int64(len(encode(header.Baseline)) + len(encode(p)))}
			if baselineHook != nil {
				if err := baselineHook(header.Baseline); err != nil {
					return empty, err
				}
			}
		} else if line.Record != nil && line.Header == nil && line.Trailer == nil {
			r := line.Record
			if len(encode(r)) > 12<<20 || head.RetainedLedgers >= 100000 {
				return empty, ErrCapacity
			}
			next, report, err := history.Advance(ctx, head.Projection, evidenceCapture(head.Projection.Snapshot.Network, r.Evidence))
			if err != nil || len(report.Ledgers) != 1 || !reflect.DeepEqual(r.Applied, report.Ledgers[0]) {
				return empty, ErrEvidence
			}
			head.RetainedPayloadBytes += int64(len(encode(r)) + len(encode(next)) - len(encode(head.Projection)))
			head.Projection = next
			head.RetainedLedgers++
			for _, event := range r.Applied.Events {
				head.PendingFindings++
				head.RetainedPayloadBytes += int64(len(encode(finding(next, r.Applied, event))))
			}
		} else if line.Trailer != nil && line.Header == nil && line.Record == nil {
			trailer = line.Trailer
			if trailer.ContentSHA256 != hex.EncodeToString(contentHash.Sum(nil)) || !reflect.DeepEqual(trailer.Head, head) {
				return empty, ErrState
			}
			continue
		} else {
			return empty, ErrState
		}
		if head.RetainedPayloadBytes > 256<<20 || head.PendingFindings > 10000 {
			return empty, ErrCapacity
		}
		if line.Record != nil && recordHook != nil {
			if err := recordHook(*line.Record); err != nil {
				return empty, err
			}
		}
		_, _ = contentHash.Write(raw)
		_, _ = contentHash.Write([]byte{'\n'})
	}
	if err := scanner.Err(); err != nil {
		return empty, err
	}
	if compressed.N <= 0 || expanded.N <= 0 || header == nil || trailer == nil {
		return empty, ErrState
	}
	if ctx.Err() != nil {
		return empty, ctx.Err()
	}
	return ArchiveReceipt{SchemaVersion: 1, Kind: "fleetwatch_follow_evidence_archive", ArchiveSHA256: hex.EncodeToString(fileHash.Sum(nil)), ContentSHA256: trailer.ContentSHA256,
		BaselineDigest: head.Projection.OriginDigest, BaselineHeaderHash: head.Projection.OriginHeaderHash, BaselineLedger: baselineLedger, ProcessedThrough: head.Projection.Snapshot.Ledger,
		RetainedLedgers: head.RetainedLedgers, ReconstructedFindings: head.PendingFindings, SemanticValidation: "REPLAYED_FROM_ORIGINAL_ASSERTION", Offline: true,
		SourceAuthentication: "UNVERIFIED", BaselineAuthentication: "UNVERIFIED", MetadataCommitment: "NOT_VERIFIED", MembershipCoverage: "UNKNOWN", HistoryCompleteness: "UNKNOWN", FinalVerdict: "UNDETERMINED"}, nil
}
