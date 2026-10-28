package follow

import (
	"context"
	"crypto/sha256"
	"encoding/binary"
	"encoding/hex"
	"errors"
	"hash"
	"io"
	"os"
	"path/filepath"
	"reflect"

	"github.com/soroban-fleetwatch/fleetwatch/internal/history"
	bolt "go.etcd.io/bbolt"
)

type CompactReceipt struct {
	SchemaVersion      int    `json:"schema_version"`
	SourceBytes        int64  `json:"source_bytes"`
	CompactedBytes     int64  `json:"compacted_bytes"`
	StateSHA256        string `json:"state_sha256"`
	LogicalSHA256      string `json:"logical_sha256"`
	ProcessedThrough   uint32 `json:"processed_through"`
	RetainedLedgers    int    `json:"retained_ledgers"`
	PendingFindings    int    `json:"pending_findings"`
	OperationalHistory string `json:"operational_history"`
	SemanticValidation string `json:"semantic_validation"`
	SourceModified     bool   `json:"source_modified"`
	FinalVerdict       string `json:"final_verdict"`
}

// CompactState copies every bucket, value and sequence into a NEW database,
// replay-validates it, and publishes without replacing the source or any target.
// It reclaims physical allocation only; all logical retention limits remain.
func CompactState(ctx context.Context, source string, baseline history.Baseline, limits Limits, destination string) (CompactReceipt, error) {
	return compactState(ctx, source, baseline, limits, destination, nil)
}

func compactState(ctx context.Context, source string, baseline history.Baseline, limits Limits, destination string, beforePublish func() error) (CompactReceipt, error) {
	var empty CompactReceipt
	if ctx.Err() != nil {
		return empty, ctx.Err()
	}
	if destination == "" {
		return empty, errors.New("new compaction destination required")
	}
	if _, err := os.Lstat(destination); !os.IsNotExist(err) {
		return empty, errors.New("compaction destination exists or is unavailable")
	}
	s, err := OpenExisting(ctx, source, baseline, limits)
	if err != nil {
		return empty, err
	}
	defer s.Close()
	cycle, err := s.Cycle()
	if err != nil {
		return empty, err
	}
	info, err := os.Stat(source)
	if err != nil {
		return empty, err
	}
	parent := filepath.Dir(destination)
	stage, err := os.MkdirTemp(parent, ".fleetwatch-compact-*")
	if err != nil {
		return empty, err
	}
	defer os.RemoveAll(stage)
	path := filepath.Join(stage, "follow.db")
	dst, err := bolt.Open(path, 0600, nil)
	if err != nil {
		return empty, err
	}
	// Avoid copying the source's large allocation reserve into this offline copy.
	// Normal future writes can grow the file again with bbolt's standard policy.
	dst.AllocSize = dst.Info().PageSize
	logical, err := copyCompact(ctx, s.db, dst, 16<<20)
	closeErr := dst.Close()
	if err != nil {
		return empty, err
	}
	if closeErr != nil {
		return empty, closeErr
	}
	reopened, err := OpenExisting(ctx, path, baseline, limits)
	if err != nil {
		return empty, err
	}
	newCycle, err := reopened.Cycle()
	if err != nil {
		reopened.Close()
		return empty, err
	}
	newLogical, err := compactDigest(ctx, reopened.db)
	matched := reflect.DeepEqual(s.Head(), reopened.Head()) && reflect.DeepEqual(cycle, newCycle) && logical == newLogical
	closeErr = reopened.Close()
	if err != nil {
		return empty, err
	}
	if closeErr != nil {
		return empty, closeErr
	}
	if !matched {
		return empty, ErrState
	}
	f, err := os.OpenFile(path, os.O_RDWR, 0)
	if err != nil {
		return empty, err
	}
	defer f.Close()
	compacted, err := f.Stat()
	if err != nil || compacted.Size() > MaxArchiveBytes {
		return empty, errors.New("compacted file exceeds physical byte limit")
	}
	digest := sha256.New()
	if _, err := io.Copy(digest, archiveReader{ctx, f}); err != nil {
		return empty, err
	}
	if err := f.Sync(); err != nil {
		return empty, err
	}
	if err := f.Close(); err != nil {
		return empty, err
	}
	if beforePublish != nil {
		if err := beforePublish(); err != nil {
			return empty, err
		}
	}
	if ctx.Err() != nil {
		return empty, ctx.Err()
	}
	if err := os.Link(path, destination); err != nil {
		return empty, err
	}
	// An exposed destination survives subsequent failures for operator inspection.
	dir, err := os.Open(parent)
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
	head := s.Head()
	return CompactReceipt{SchemaVersion: 1, SourceBytes: info.Size(), CompactedBytes: compacted.Size(), StateSHA256: hex.EncodeToString(digest.Sum(nil)), LogicalSHA256: logical,
		ProcessedThrough: head.Projection.Snapshot.Ledger, RetainedLedgers: head.RetainedLedgers, PendingFindings: head.PendingFindings, OperationalHistory: "PRESERVED",
		SemanticValidation: "REPLAYED_AND_ALL_BUCKETS_MATCH", SourceModified: false, FinalVerdict: "UNDETERMINED"}, nil
}

// walkCompact visits all buckets in bbolt's key order, retaining empty buckets,
// empty values and sequences. Extra buckets are copied and hashed, never ignored.
// Limits also apply to extra operator/application data outside the known root.
func walkCompact(ctx context.Context, db *bolt.DB, visit func([][]byte, []byte, []byte, uint64) error) (string, error) {
	h := sha256.New()
	h.Write([]byte("fleetwatch.bbolt.logical.v1\n"))
	var bytesSeen int64
	keys := 0
	var walk func(*bolt.Bucket, [][]byte, []byte) error
	walk = func(b *bolt.Bucket, parent [][]byte, name []byte) error {
		if ctx.Err() != nil {
			return ctx.Err()
		}
		if len(parent) >= 64 {
			return errors.New("compaction nesting limit exceeded")
		}
		keys++
		bytesSeen += int64(len(name))
		if keys > 1000000 || bytesSeen > MaxArchiveBytes {
			return ErrCapacity
		}
		h.Write([]byte{'B'})
		hashCompactField(h, name)
		var seq [8]byte
		binary.BigEndian.PutUint64(seq[:], b.Sequence())
		h.Write(seq[:])
		if visit != nil {
			if err := visit(parent, name, nil, b.Sequence()); err != nil {
				return err
			}
		}
		path := append(append([][]byte{}, parent...), name)
		if err := b.ForEach(func(k, v []byte) error {
			if ctx.Err() != nil {
				return ctx.Err()
			}
			if v == nil {
				return walk(b.Bucket(k), path, k)
			}
			keys++
			bytesSeen += int64(len(k) + len(v))
			if keys > 1000000 || bytesSeen > MaxArchiveBytes {
				return ErrCapacity
			}
			h.Write([]byte{'V'})
			hashCompactField(h, k)
			hashCompactField(h, v)
			if visit != nil {
				return visit(path, k, v, 0)
			}
			return nil
		}); err != nil {
			return err
		}
		h.Write([]byte{'E'})
		return nil
	}
	err := db.View(func(tx *bolt.Tx) error {
		return tx.ForEach(func(name []byte, b *bolt.Bucket) error { return walk(b, nil, name) })
	})
	return hex.EncodeToString(h.Sum(nil)), err
}

func hashCompactField(h hash.Hash, value []byte) {
	var length [8]byte
	binary.BigEndian.PutUint64(length[:], uint64(len(value)))
	h.Write(length[:])
	h.Write(value)
}

func compactDigest(ctx context.Context, db *bolt.DB) (string, error) {
	return walkCompact(ctx, db, nil)
}

func copyCompact(ctx context.Context, src, dst *bolt.DB, transactionBytes int64) (string, error) {
	tx, err := dst.Begin(true)
	if err != nil {
		return "", err
	}
	defer func() {
		if tx != nil {
			_ = tx.Rollback()
		}
	}()
	var pending int64
	digest, err := walkCompact(ctx, src, func(path [][]byte, key, value []byte, sequence uint64) error {
		size := int64(len(key) + len(value))
		if pending > 0 && pending+size > transactionBytes {
			if err := tx.Commit(); err != nil {
				return err
			}
			tx, err = dst.Begin(true)
			if err != nil {
				return err
			}
			pending = 0
		}
		pending += size
		if len(path) == 0 {
			b, err := tx.CreateBucket(key)
			if err != nil {
				return err
			}
			return b.SetSequence(sequence)
		}
		b := tx.Bucket(path[0])
		for _, component := range path[1:] {
			b = b.Bucket(component)
		}
		b.FillPercent = 1
		if value == nil {
			child, err := b.CreateBucket(key)
			if err != nil {
				return err
			}
			return child.SetSequence(sequence)
		}
		return b.Put(key, value)
	})
	if err != nil {
		return "", err
	}
	if err := ctx.Err(); err != nil {
		return "", err
	}
	return digest, tx.Commit()
}
