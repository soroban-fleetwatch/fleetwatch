package store

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"reflect"

	"github.com/soroban-fleetwatch/fleetwatch/internal/jsoninput"
	bolt "go.etcd.io/bbolt"
)

const followQueuePrefix = "follow:"

var followImportsBucket = []byte("fleetwatch.follow-imports.v1")

// FollowBinding identifies a replay-validated follow snapshot at sync time.
// Its source chain remains unverified. The adapter checks prior-header ancestry.
type FollowBinding struct {
	Network            string `json:"network"`
	Stream             string `json:"stream"`
	BaselineDigest     string `json:"baseline_digest"`
	BaselineHeaderHash string `json:"baseline_header_hash"`
	BaselineLedger     uint32 `json:"baseline_ledger"`
	ProcessedThrough   uint32 `json:"processed_through"`
	HeaderHash         string `json:"header_hash"`
}
type followImport struct {
	SchemaVersion int                        `json:"schema_version"`
	Binding       FollowBinding              `json:"binding"`
	Events        []ReconstructionEventAlert `json:"events"`
}
type FollowSyncResult struct {
	SchemaVersion int           `json:"schema_version"`
	Binding       FollowBinding `json:"binding"`
	Added         int           `json:"added"`
	Retained      int           `json:"retained"`
	Offline       bool          `json:"offline"`
	FinalVerdict  string        `json:"final_verdict"`
}

func followAlertID(network, stream string, e ReconstructionEventAlert) string {
	raw, _ := json.Marshal(struct {
		Domain, Network, Stream string
		Event                   ReconstructionEventAlert
	}{"fleetwatch.follow-alert.v1", network, stream, e})
	h := sha256.Sum256(raw)
	return hex.EncodeToString(h[:])
}
func followPayload(binding FollowBinding, e ReconstructionEventAlert) (string, AlertPayload) {
	assurance := projectionAssurance()
	return followAlertID(binding.Network, binding.Stream, e), AlertPayload{SchemaVersion: 3, Network: binding.Network, Source: "follow_ledger_meta", Stream: binding.Stream,
		EvaluationStatus: "UNVERIFIED_PROJECTED_STATE", Assurance: &assurance, Reconstruction: &e}
}
func validateFollowImport(state followImport) error {
	b := state.Binding
	if state.SchemaVersion != 1 || ValidateReconstructionScope(b.Network, b.Stream) != nil || !validDigest(b.BaselineDigest) || !validDigest(b.BaselineHeaderHash) || !validDigest(b.HeaderHash) || b.BaselineLedger == 0 || b.ProcessedThrough < b.BaselineLedger || len(state.Events) > 10000 || state.Events == nil {
		return errors.New("invalid follow import binding")
	}
	previous := ""
	for _, e := range state.Events {
		id, payload := followPayload(b, e)
		if e.Event.ID <= previous || e.BaselineDigest != b.BaselineDigest || e.BaselineHeaderHash != b.BaselineHeaderHash || e.Ledger <= b.BaselineLedger || e.Ledger > b.ProcessedThrough || ValidateAlertPayload(id, payload) != nil {
			return errors.New("invalid follow import event")
		}
		previous = e.Event.ID
	}
	return nil
}
func loadFollowImport(tx *bolt.Tx, network, stream string) (*followImport, error) {
	root := tx.Bucket(followImportsBucket)
	if root == nil {
		return nil, nil
	}
	raw := root.Get(scope(network, followQueuePrefix+stream))
	if raw == nil {
		return nil, nil
	}
	var state followImport
	if len(raw) > maxStateBytes || jsoninput.Decode(bytes.NewReader(raw), &state) != nil || validateFollowImport(state) != nil || state.Binding.Network != network || state.Binding.Stream != stream {
		return nil, errors.New("invalid retained follow import")
	}
	return &state, nil
}

func (s *Store) FollowImportBinding(ctx context.Context, network, stream string) (*FollowBinding, error) {
	if ValidateReconstructionScope(network, stream) != nil {
		return nil, errors.New("invalid follow import scope")
	}
	var binding *FollowBinding
	err := s.db.View(func(tx *bolt.Tx) error {
		if ctx.Err() != nil {
			return ctx.Err()
		}
		old, err := loadFollowImport(tx, network, stream)
		if old != nil {
			b := old.Binding
			binding = &b
		}
		return err
	})
	return binding, err
}

// ImportFollow atomically merges a COMPLETE bounded finding set supplied by the
// replay-validating follow adapter. It preserves existing lease/ack/retry state.
// It is not a raw-evidence verifier or an independent chain authentication API.
func (s *Store) ImportFollow(ctx context.Context, binding FollowBinding, events []ReconstructionEventAlert, expectedPrevious *FollowBinding) (FollowSyncResult, error) {
	return s.importFollow(ctx, binding, events, expectedPrevious, nil)
}
func (s *Store) importFollow(ctx context.Context, binding FollowBinding, events []ReconstructionEventAlert, expectedPrevious *FollowBinding, hook func() error) (FollowSyncResult, error) {
	result := FollowSyncResult{SchemaVersion: 1, Binding: binding, Offline: true, FinalVerdict: "UNDETERMINED"}
	state := followImport{1, binding, events}
	if validateFollowImport(state) != nil {
		return FollowSyncResult{}, errors.New("invalid follow import")
	}
	raw, err := json.Marshal(state)
	if err != nil || len(raw) > maxStateBytes {
		return FollowSyncResult{}, errors.New("follow import exceeds 25 MiB")
	}
	err = s.db.Update(func(tx *bolt.Tx) error {
		if ctx.Err() != nil {
			return ctx.Err()
		}
		old, err := loadFollowImport(tx, binding.Network, binding.Stream)
		if err != nil {
			return err
		}
		var previous *FollowBinding
		if old != nil {
			previous = &old.Binding
		}
		if !reflect.DeepEqual(previous, expectedPrevious) {
			return errors.New("follow import changed concurrently")
		}
		if err := inspectFollowQueue(ctx, tx, binding.Network, binding.Stream); err != nil {
			return err
		}
		if old != nil {
			p := old.Binding
			if p.BaselineDigest != binding.BaselineDigest || p.BaselineHeaderHash != binding.BaselineHeaderHash || p.BaselineLedger != binding.BaselineLedger || binding.ProcessedThrough < p.ProcessedThrough || (binding.ProcessedThrough == p.ProcessedThrough && binding.HeaderHash != p.HeaderHash) {
				return errors.New("follow import baseline or head conflict")
			}
		}
		q, err := queue(tx, binding.Network, followQueuePrefix+binding.Stream, true)
		if err != nil {
			return err
		}
		incoming := make(map[string]bool, len(events))
		for _, e := range events {
			if ctx.Err() != nil {
				return ctx.Err()
			}
			id, payload := followPayload(binding, e)
			incoming[id] = true
			if existing := q.Get([]byte(id)); existing != nil {
				a, err := decodeAlert(existing, id, binding.Network, followQueuePrefix+binding.Stream)
				if err != nil || !reflect.DeepEqual(a.Payload, payload) {
					return errors.New("conflicting retained follow payload")
				}
				result.Retained++
				continue
			}
			if err := writeAlert(q, Alert{ID: id, Payload: payload, Status: "PENDING"}); err != nil {
				return err
			}
			result.Added++
		}
		// No earlier imported finding may disappear on a later source scan.
		count := 0
		if err := q.ForEach(func(key, _ []byte) error {
			if !incoming[string(key)] {
				return errors.New("follow import omitted retained findings")
			}
			count++
			return nil
		}); err != nil {
			return err
		}
		if count != len(incoming) {
			return errors.New("follow import queue count mismatch")
		}
		root, err := tx.CreateBucketIfNotExists(followImportsBucket)
		if err != nil {
			return err
		}
		if err := root.Put(scope(binding.Network, followQueuePrefix+binding.Stream), raw); err != nil {
			return err
		}
		if hook != nil {
			if err := hook(); err != nil {
				return err
			}
		}
		return ctx.Err()
	})
	if err != nil {
		return FollowSyncResult{}, err
	}
	return result, nil
}

func inspectFollowQueue(ctx context.Context, tx *bolt.Tx, network, stream string) error {
	state, err := loadFollowImport(tx, network, stream)
	if err != nil {
		return err
	}
	q, err := queue(tx, network, followQueuePrefix+stream, false)
	if err != nil {
		return err
	}
	if state == nil {
		if q != nil && q.Stats().KeyN != 0 {
			return errors.New("follow queue has no import manifest")
		}
		return nil
	}
	if q == nil || q.Stats().KeyN != len(state.Events) {
		return errors.New("follow queue/manifest count mismatch")
	}
	for _, e := range state.Events {
		if ctx.Err() != nil {
			return ctx.Err()
		}
		id, payload := followPayload(state.Binding, e)
		a, err := decodeAlert(q.Get([]byte(id)), id, network, followQueuePrefix+stream)
		if err != nil || !reflect.DeepEqual(a.Payload, payload) {
			return errors.New("follow queue/manifest payload mismatch")
		}
	}
	return nil
}

func (s *Store) FollowAlerts(ctx context.Context, network, stream string) ([]Alert, error) {
	if ValidateReconstructionScope(network, stream) != nil {
		return nil, errors.New("invalid follow alert scope")
	}
	rows := []Alert{}
	err := s.db.View(func(tx *bolt.Tx) error {
		if err := inspectFollowQueue(ctx, tx, network, stream); err != nil {
			return err
		}
		q, err := queue(tx, network, followQueuePrefix+stream, false)
		if err != nil || q == nil {
			return err
		}
		return q.ForEach(func(key, raw []byte) error {
			if ctx.Err() != nil {
				return ctx.Err()
			}
			a, err := decodeAlert(raw, string(key), network, followQueuePrefix+stream)
			if err != nil {
				return err
			}
			a.Token, a.LeaseToken = "", ""
			rows = append(rows, a)
			return nil
		})
	})
	return rows, err
}
