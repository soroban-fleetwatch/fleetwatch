// Package store persists bounded offline replay state. It is not a live ledger
// ingestion database and does not authenticate snapshot provenance.
package store

import (
	"bytes"
	"crypto/sha256"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"sort"
	"time"

	"github.com/soroban-fleetwatch/fleetwatch/internal/fleetwatch"
	"github.com/soroban-fleetwatch/fleetwatch/internal/stateformat"
	bolt "go.etcd.io/bbolt"
)

var ErrConflict = errors.New("stored observation conflicts with supplied journal")
var stateBucket = []byte(stateformat.Replay)

const maxStateBytes = 25 << 20

type State struct {
	SchemaVersion int                     `json:"schema_version"`
	Journal       fleetwatch.Journal      `json:"journal"`
	Report        fleetwatch.ReplayReport `json:"report"`
}
type Store struct{ db *bolt.DB }

func Open(path string) (*Store, error) {
	existed := false
	if info, err := os.Lstat(path); err == nil {
		existed = true
		if !info.Mode().IsRegular() {
			return nil, errors.New("state path must be a regular database file")
		}
		if info.Size() == 0 {
			return nil, errors.New("refusing pre-existing empty state file")
		}
	} else if !os.IsNotExist(err) {
		return nil, err
	}
	db, err := bolt.Open(path, 0600, &bolt.Options{Timeout: time.Second})
	if err != nil {
		return nil, fmt.Errorf("open replay state: %w", err)
	}
	if existed {
		err = db.View(func(tx *bolt.Tx) error {
			if tx.Bucket(stateBucket) == nil {
				return errors.New("unrecognized replay database")
			}
			return nil
		})
	} else {
		err = db.Update(func(tx *bolt.Tx) error { _, err := tx.CreateBucket(stateBucket); return err })
	}
	if err != nil {
		db.Close()
		return nil, err
	}
	return &Store{db: db}, nil
}
func (s *Store) Close() error { return s.db.Close() }
func scope(network, source string) []byte {
	payload, _ := json.Marshal([]string{"fleetwatch.offline-scope.v1", network, source})
	sum := sha256.Sum256(payload)
	return sum[:]
}
func (s *Store) Load(network, source string) (State, bool, error) {
	var result State
	found := false
	err := s.db.View(func(tx *bolt.Tx) error {
		bucket := tx.Bucket(stateBucket)
		if bucket == nil {
			return nil
		}
		raw := bucket.Get(scope(network, source))
		if raw == nil {
			return nil
		}
		if err := decodeState(raw, &result); err != nil {
			return err
		}
		if result.Journal.Network != network || result.Journal.Source != source {
			return errors.New("stored replay scope mismatch")
		}
		found = true
		return nil
	})
	return result, found, err
}
func decodeState(raw []byte, target *State) error {
	if len(raw) > maxStateBytes {
		return errors.New("stored replay state exceeds local limit")
	}
	decoder := json.NewDecoder(bytes.NewReader(raw))
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(target); err != nil {
		return fmt.Errorf("invalid replay state: %w", err)
	}
	var trailing any
	if err := decoder.Decode(&trailing); err != io.EOF {
		return errors.New("invalid trailing replay state")
	}
	if target.SchemaVersion != 1 {
		return errors.New("unsupported replay state schema")
	}
	report, err := fleetwatch.ReplayJournal(target.Journal)
	// Persisted Journal is only the validated contiguous input prefix. Report may
	// additionally describe the gap in the latest attempted extension.
	if err != nil || report.ProcessedThrough != target.Report.ProcessedThrough || report.Network != target.Report.Network || report.Source != target.Report.Source {
		return errors.New("stored replay state invariants failed")
	}
	if target.Report.SchemaVersion != 1 || target.Report.EvaluationStatus != "UNVERIFIED_INPUT" {
		return errors.New("invalid stored report assurance")
	}
	expected, _ := json.Marshal(report.Events)
	actual, _ := json.Marshal(target.Report.Events)
	if !bytes.Equal(expected, actual) {
		return errors.New("stored replay findings do not match observations")
	}
	return nil
}
func (s *Store) Replay(j fleetwatch.Journal) (fleetwatch.ReplayReport, error) {
	return s.replay(j, nil)
}

// replay updates the retained observations, recomputed findings and cursor in
// one synchronous bbolt transaction. The hook allows fault injection before
// commit; it is never called by production entry points.
func (s *Store) replay(j fleetwatch.Journal, beforeCommit func() error) (fleetwatch.ReplayReport, error) {
	var result fleetwatch.ReplayReport
	if _, err := fleetwatch.ReplayJournal(j); err != nil {
		return result, err
	}
	err := s.db.Update(func(tx *bolt.Tx) error {
		bucket, err := tx.CreateBucketIfNotExists(stateBucket)
		if err != nil {
			return err
		}
		merged := j
		if raw := bucket.Get(scope(j.Network, j.Source)); raw != nil {
			var old State
			if err := decodeState(raw, &old); err != nil {
				return err
			}
			if old.Journal.Network != j.Network || old.Journal.Source != j.Source {
				return errors.New("stored replay scope mismatch")
			}
			if j.FromLedger < old.Journal.FromLedger {
				return errors.New("cannot prepend observations to an existing replay baseline")
			}
			merged.FromLedger = old.Journal.FromLedger
			if old.Journal.ToLedger > merged.ToLedger {
				merged.ToLedger = old.Journal.ToLedger
			}
			merged.Snapshots = append(append([]fleetwatch.Snapshot{}, old.Journal.Snapshots...), j.Snapshots...)
		}
		// Sort across restart segments; each incoming segment was independently
		// checked for ordering before entering this transaction.
		sort.SliceStable(merged.Snapshots, func(i, k int) bool { return merged.Snapshots[i].Ledger < merged.Snapshots[k].Ledger })
		compact := make([]fleetwatch.Snapshot, 0, len(merged.Snapshots))
		for _, snapshot := range merged.Snapshots {
			if len(compact) > 0 && compact[len(compact)-1].Ledger == snapshot.Ledger {
				a, _ := fleetwatch.SnapshotDigest(compact[len(compact)-1])
				b, _ := fleetwatch.SnapshotDigest(snapshot)
				if a != b {
					return ErrConflict
				}
				continue
			}
			compact = append(compact, snapshot)
		}
		merged.Snapshots = compact
		report, err := fleetwatch.ReplayJournal(merged)
		if err != nil {
			return err
		}
		retained := merged
		retained.Snapshots = nil
		for _, snapshot := range merged.Snapshots {
			if snapshot.Ledger <= report.ProcessedThrough {
				retained.Snapshots = append(retained.Snapshots, snapshot)
			}
		}
		// A missing baseline has no resumable state. Preserve the diagnostic range
		// until the baseline arrives without manufacturing a cursor.
		if report.ProcessedThrough > 0 {
			retained.ToLedger = report.ProcessedThrough
		}
		encoded, err := json.Marshal(State{SchemaVersion: 1, Journal: retained, Report: report})
		if err != nil {
			return err
		}
		if len(encoded) > maxStateBytes {
			return errors.New("replay state exceeds local byte limit")
		}
		if err := bucket.Put(scope(j.Network, j.Source), encoded); err != nil {
			return err
		}
		if err := enqueue(tx, report); err != nil {
			return err
		}
		if beforeCommit != nil {
			if err := beforeCommit(); err != nil {
				return err
			}
		}
		result = report
		return nil
	})
	return result, err
}
