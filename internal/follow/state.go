// Package follow implements an experimental single-writer, continuously
// advancing diagnostic observer. It does not authenticate chain evidence.
package follow

import (
	"bytes"
	"context"
	"encoding/binary"
	"encoding/hex"
	"encoding/json"
	"errors"
	"os"
	"reflect"
	"strings"
	"sync"
	"time"

	"github.com/soroban-fleetwatch/fleetwatch/internal/history"
	"github.com/soroban-fleetwatch/fleetwatch/internal/jsoninput"
	"github.com/soroban-fleetwatch/fleetwatch/internal/stateformat"
	bolt "go.etcd.io/bbolt"
)

var ErrCapacity = errors.New("follow retention capacity reached")
var ErrState = errors.New("follow state is inconsistent")
var ErrEvidence = errors.New("follow evidence rejected")
var rootName = []byte(stateformat.Follow)
var ledgerName = []byte("ledgers")
var findingName = []byte("pending-findings")

type Limits struct {
	MaxLedgers      int
	MaxPayloadBytes int64
}

func (l Limits) Validate() error {
	if l.MaxLedgers < 1 || l.MaxLedgers > 100000 || l.MaxPayloadBytes < 1<<20 || l.MaxPayloadBytes > 256<<20 {
		return errors.New("invalid follow retention limits")
	}
	return nil
}

type Record struct {
	Evidence history.Evidence      `json:"evidence"`
	Applied  history.AppliedLedger `json:"applied"`
}
type Finding struct {
	SchemaVersion          int                  `json:"schema_version"`
	Network                string               `json:"network"`
	BaselineDigest         string               `json:"baseline_digest"`
	BaselineHeaderHash     string               `json:"baseline_header_hash"`
	Ledger                 uint32               `json:"ledger"`
	HeaderHash             string               `json:"header_hash"`
	MetadataSHA256         string               `json:"metadata_sha256"`
	Event                  history.DerivedEvent `json:"event"`
	FinalVerdict           string               `json:"final_verdict"`
	SourceAuthentication   string               `json:"source_authentication"`
	BaselineAuthentication string               `json:"baseline_authentication"`
	MetadataCommitment     string               `json:"metadata_commitment"`
	MembershipCoverage     string               `json:"membership_coverage"`
	HistoryCompleteness    string               `json:"history_completeness"`
	EvaluationStatus       string               `json:"evaluation_status"`
}
type Head struct {
	Projection           history.Projection `json:"projection"`
	RetainedLedgers      int                `json:"retained_ledgers"`
	RetainedPayloadBytes int64              `json:"retained_payload_bytes"`
	PendingFindings      int                `json:"pending_findings"`
}
type Store struct {
	db     *bolt.DB
	mu     sync.Mutex
	head   Head
	limits Limits
}

func encode(v any) []byte    { raw, _ := json.Marshal(v); return raw }
func seqKey(n uint32) []byte { key := make([]byte, 4); binary.BigEndian.PutUint32(key, n); return key }
func evidenceCapture(network string, e history.Evidence) history.Capture {
	return history.Capture{SchemaVersion: 1, Kind: "ledger_metadata_capture", Source: "rpc_ledger_meta", Network: network, FromLedger: e.Sequence, ToLedger: e.Sequence, ProcessedThrough: e.Sequence, SequenceCoverage: "COMPLETE_ENDPOINT_SEQUENCE", SourceAuthentication: "UNVERIFIED", HistoryCompleteness: "UNKNOWN", MetadataCommitment: "NOT_VERIFIED", StateChanges: "NOT_APPLIED", FinalVerdict: "UNDETERMINED", Ledgers: []history.Evidence{e}, Errors: []string{}}
}
func finding(p history.Projection, l history.AppliedLedger, e history.DerivedEvent) Finding {
	return Finding{SchemaVersion: 1, Network: p.Snapshot.Network, BaselineDigest: p.OriginDigest, BaselineHeaderHash: p.OriginHeaderHash, Ledger: l.Sequence, HeaderHash: l.HeaderHash, MetadataSHA256: l.MetadataSHA256, Event: e, FinalVerdict: "UNDETERMINED", SourceAuthentication: "UNVERIFIED", BaselineAuthentication: "UNVERIFIED", MetadataCommitment: "NOT_VERIFIED", MembershipCoverage: "UNKNOWN", HistoryCompleteness: "UNKNOWN", EvaluationStatus: "UNVERIFIED_PROJECTED_STATE"}
}

// Open replays every retained record from the ORIGINAL asserted baseline.
// A cached head never substitutes for retained evidence during restart.
func Open(ctx context.Context, path string, baseline history.Baseline, limits Limits) (*Store, error) {
	initial, err := history.NewProjection(baseline)
	if err != nil {
		return nil, err
	}
	if limits.Validate() != nil {
		return nil, errors.New("invalid follow limits")
	}
	if int64(len(encode(baseline))+len(encode(initial))) > limits.MaxPayloadBytes {
		return nil, ErrCapacity
	}
	if ctx.Err() != nil {
		return nil, ctx.Err()
	}
	existed := false
	if info, err := os.Lstat(path); err == nil {
		existed = true
		if !info.Mode().IsRegular() || info.Size() == 0 {
			return nil, errors.New("existing follow state must be a nonempty regular file")
		}
	} else if err != nil && !os.IsNotExist(err) {
		return nil, err
	}
	db, err := bolt.Open(path, 0600, &bolt.Options{Timeout: time.Second})
	if err != nil {
		return nil, err
	}
	s := &Store{db: db, limits: limits}
	initialize := false
	if err = db.View(func(tx *bolt.Tx) error {
		if tx.Bucket(rootName) != nil {
			return nil
		}
		if existed {
			return ErrState
		}
		if k, _ := tx.Cursor().First(); k != nil {
			return ErrState
		}
		initialize = true
		return nil
	}); err != nil {
		db.Close()
		return nil, err
	}
	if initialize {
		if err = db.Update(func(tx *bolt.Tx) error {
			root, err := tx.CreateBucket(rootName)
			if err != nil {
				return err
			}
			for _, name := range [][]byte{ledgerName, findingName} {
				if _, err := root.CreateBucket(name); err != nil {
					return err
				}
			}
			if err := root.Put([]byte("baseline"), encode(baseline)); err != nil {
				return err
			}
			head := Head{Projection: initial, RetainedPayloadBytes: int64(len(encode(baseline)) + len(encode(initial)))}
			if head.RetainedPayloadBytes > limits.MaxPayloadBytes {
				return ErrCapacity
			}
			return root.Put([]byte("head"), encode(head))
		}); err != nil {
			db.Close()
			return nil, err
		}
	}
	err = db.View(func(tx *bolt.Tx) error {
		root := tx.Bucket(rootName)
		ledgers := root.Bucket(ledgerName)
		findings := root.Bucket(findingName)
		if ledgers == nil || findings == nil {
			return ErrState
		}
		var storedBaseline history.Baseline
		var storedHead Head
		if jsoninput.Decode(bytes.NewReader(root.Get([]byte("baseline"))), &storedBaseline) != nil || jsoninput.Decode(bytes.NewReader(root.Get([]byte("head"))), &storedHead) != nil {
			return ErrState
		}
		p, err := history.NewProjection(storedBaseline)
		if err != nil || p.OriginDigest != initial.OriginDigest || p.OriginHeaderHash != initial.OriginHeaderHash || p.Snapshot.Network != initial.Snapshot.Network {
			return ErrState
		}
		head := Head{Projection: p, RetainedPayloadBytes: int64(len(root.Get([]byte("baseline"))) + len(encode(p)))}
		if err := ledgers.ForEach(func(key, raw []byte) error {
			if ctx.Err() != nil {
				return ctx.Err()
			}
			if len(key) != 4 || raw == nil || len(raw) > 12<<20 {
				return ErrState
			}
			var record Record
			if jsoninput.Decode(bytes.NewReader(raw), &record) != nil || !bytes.Equal(key, seqKey(record.Evidence.Sequence)) {
				return ErrState
			}
			next, report, err := history.Advance(ctx, head.Projection, evidenceCapture(p.Snapshot.Network, record.Evidence))
			if err != nil || len(report.Ledgers) != 1 || !reflect.DeepEqual(report.Ledgers[0], record.Applied) {
				return ErrState
			}
			head.RetainedPayloadBytes += int64(len(raw) + len(encode(next)) - len(encode(head.Projection)))
			head.Projection = next
			head.RetainedLedgers++
			for _, event := range record.Applied.Events {
				expected := encode(finding(next, record.Applied, event))
				actual := findings.Get([]byte(event.ID))
				if !bytes.Equal(expected, actual) {
					return ErrState
				}
				head.PendingFindings++
				head.RetainedPayloadBytes += int64(len(actual))
			}
			if head.RetainedLedgers > limits.MaxLedgers || head.RetainedPayloadBytes > limits.MaxPayloadBytes {
				return ErrCapacity
			}
			return nil
		}); err != nil {
			return err
		}
		if findings.Stats().KeyN != head.PendingFindings || !reflect.DeepEqual(head, storedHead) {
			return ErrState
		}
		s.head = head
		return nil
	})
	if err != nil {
		db.Close()
		return nil, err
	}
	return s, nil
}
func (s *Store) Close() error { s.mu.Lock(); defer s.mu.Unlock(); return s.db.Close() }
func (s *Store) Head() Head {
	s.mu.Lock()
	defer s.mu.Unlock()
	var result Head
	_ = json.Unmarshal(encode(s.head), &result)
	return result
}

// Findings returns a bounded page ordered by stable event ID, not wall clock.
// The follow journal has no delivery/acknowledgement implementation yet.
func (s *Store) Findings(ctx context.Context, after string, limit int) ([]Finding, bool, error) {
	if limit < 1 || limit > 100 {
		return nil, false, errors.New("finding page requires 1-100 entries")
	}
	if after != "" {
		raw, err := hex.DecodeString(after)
		if err != nil || len(raw) != 32 || after != strings.ToLower(after) {
			return nil, false, errors.New("invalid finding cursor")
		}
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	rows := []Finding{}
	more := false
	err := s.db.View(func(tx *bolt.Tx) error {
		cursor := tx.Bucket(rootName).Bucket(findingName).Cursor()
		key, raw := cursor.First()
		if after != "" {
			key, raw = cursor.Seek([]byte(after))
			if string(key) == after {
				key, raw = cursor.Next()
			}
		}
		for ; key != nil; key, raw = cursor.Next() {
			if ctx.Err() != nil {
				return ctx.Err()
			}
			if len(rows) == limit {
				more = true
				break
			}
			var f Finding
			if jsoninput.Decode(bytes.NewReader(raw), &f) != nil || f.Event.ID != string(key) {
				return ErrState
			}
			rows = append(rows, f)
		}
		return nil
	})
	return rows, more, err
}

// Commit publishes raw accepted evidence, projection, and findings together.
// Capacity/cancellation/transaction failures leave the preceding head intact.
func (s *Store) Commit(ctx context.Context, c history.Capture) (history.Reconstruction, error) {
	return s.commit(ctx, c, nil)
}
func (s *Store) commit(ctx context.Context, c history.Capture, beforeCommit func() error) (history.Reconstruction, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	p, report, applicationErr := history.Advance(ctx, s.head.Projection, c)
	if ctx.Err() != nil {
		return history.Reconstruction{}, ctx.Err()
	}
	if report.SchemaVersion != 1 {
		return history.Reconstruction{}, ErrEvidence
	}
	if len(report.Ledgers) == 0 {
		return report, applicationErr
	}
	if len(report.Ledgers) > len(c.Ledgers) {
		return history.Reconstruction{}, ErrState
	}
	next := s.head
	next.Projection = p
	next.RetainedPayloadBytes += int64(len(encode(p)) - len(encode(s.head.Projection)))
	err := s.db.Update(func(tx *bolt.Tx) error {
		root := tx.Bucket(rootName)
		records := root.Bucket(ledgerName)
		findings := root.Bucket(findingName)
		if !bytes.Equal(root.Get([]byte("head")), encode(s.head)) {
			return ErrState
		}
		for i, l := range report.Ledgers {
			if ctx.Err() != nil {
				return ctx.Err()
			}
			key := seqKey(l.Sequence)
			if records.Get(key) != nil {
				return ErrState
			}
			raw := encode(Record{c.Ledgers[i], l})
			if len(raw) > 12<<20 {
				return ErrCapacity
			}
			next.RetainedLedgers++
			next.RetainedPayloadBytes += int64(len(raw))
			for _, event := range l.Events {
				if findings.Get([]byte(event.ID)) != nil {
					return ErrState
				}
				value := encode(finding(p, l, event))
				next.PendingFindings++
				next.RetainedPayloadBytes += int64(len(value))
				if err := findings.Put([]byte(event.ID), value); err != nil {
					return err
				}
			}
			if next.RetainedLedgers > s.limits.MaxLedgers || next.RetainedPayloadBytes > s.limits.MaxPayloadBytes || next.PendingFindings > 10000 {
				return ErrCapacity
			}
			if err := records.Put(key, raw); err != nil {
				return err
			}
		}
		if err := root.Put([]byte("head"), encode(next)); err != nil {
			return err
		}
		if beforeCommit != nil {
			if err := beforeCommit(); err != nil {
				return err
			}
		}
		return ctx.Err()
	})
	if err != nil {
		return history.Reconstruction{}, err
	}
	s.head = next
	return report, applicationErr
}
