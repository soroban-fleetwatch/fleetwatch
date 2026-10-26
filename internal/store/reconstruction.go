package store

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/json"
	"errors"
	"fmt"

	"github.com/soroban-fleetwatch/fleetwatch/internal/fleetwatch"
	"github.com/soroban-fleetwatch/fleetwatch/internal/history"
	"github.com/soroban-fleetwatch/fleetwatch/internal/jsoninput"
	bolt "go.etcd.io/bbolt"
)

var reconstructionBucket = []byte("fleetwatch.watched-reconstruction.v1")
var ErrReconstructionIncomplete = errors.New("reconstruction source/application incomplete")
var ErrReconstructionBaseline = errors.New("stream is bound to another baseline")
var ErrReconstructionBound = errors.New("reconstruction stream exceeds its bounded range or storage limit")

type ReconstructionState struct {
	SchemaVersion int                    `json:"schema_version"`
	Stream        string                 `json:"stream"`
	Baseline      history.Baseline       `json:"baseline"`
	Capture       history.Capture        `json:"capture"`
	Report        history.Reconstruction `json:"report"`
	AlertsEnabled bool                   `json:"alerts_enabled,omitempty"`
}

func ValidateReconstructionScope(network, stream string) error {
	if history.ValidateRequest(network, 1, 1) != nil || len(stream) == 0 || len(stream) > 128 {
		return errors.New("invalid reconstruction scope")
	}
	for _, c := range stream {
		if !((c >= 'a' && c <= 'z') || (c >= 'A' && c <= 'Z') || (c >= '0' && c <= '9') || c == '_' || c == '-' || c == '.') {
			return errors.New("invalid stream identifier")
		}
	}
	return nil
}
func reconstructionScope(network, stream string) []byte {
	raw, _ := json.Marshal([]string{"fleetwatch.reconstruction-scope.v1", network, stream})
	sum := sha256.Sum256(raw)
	return sum[:]
}

func decodeReconstruction(ctx context.Context, raw []byte, network, stream string) (ReconstructionState, error) {
	var state ReconstructionState
	if len(raw) > maxStateBytes {
		return state, errors.New("oversized reconstruction state")
	}
	if err := jsoninput.Decode(bytes.NewReader(raw), &state); err != nil {
		return state, errors.New("invalid reconstruction state JSON")
	}
	if state.SchemaVersion != 1 || state.Stream != stream || state.Baseline.Snapshot.Network != network || state.Capture.Network != network || len(state.Capture.Ledgers) == 0 || state.Capture.SequenceCoverage != "COMPLETE_ENDPOINT_SEQUENCE" {
		return ReconstructionState{}, errors.New("invalid reconstruction state scope/coverage")
	}
	recomputed, err := history.Reconstruct(ctx, state.Baseline, state.Capture)
	if err != nil {
		return ReconstructionState{}, errors.New("retained reconstruction evidence failed validation")
	}
	expected, _ := json.Marshal(recomputed)
	stored, _ := json.Marshal(state.Report)
	if !bytes.Equal(expected, stored) {
		return ReconstructionState{}, errors.New("stored reconstruction differs from retained evidence")
	}
	return state, nil
}

// LoadReconstruction validates one retained prefix in a read transaction. It does
// not establish that the latest attempted extension succeeded or was retained.
func (s *Store) LoadReconstruction(ctx context.Context, network, stream string) (ReconstructionState, bool, error) {
	var state ReconstructionState
	found := false
	if err := ValidateReconstructionScope(network, stream); err != nil {
		return state, false, err
	}
	err := s.db.View(func(tx *bolt.Tx) error {
		b := tx.Bucket(reconstructionBucket)
		if b == nil {
			return nil
		}
		raw := b.Get(reconstructionScope(network, stream))
		if raw == nil {
			return nil
		}
		var err error
		state, err = decodeReconstruction(ctx, raw, network, stream)
		if err != nil {
			return err
		}
		found = true
		return nil
	})
	if err != nil {
		return ReconstructionState{}, false, err
	}
	return state, found, nil
}

// Reconstruct retains the accepted raw prefix and its canonical recomputed
// report atomically. The returned report describes this attempt, which can be
// partial even when an earlier accepted prefix was committed successfully.
func (s *Store) Reconstruct(ctx context.Context, stream string, baseline history.Baseline, incoming history.Capture) (history.Reconstruction, error) {
	return s.reconstruct(ctx, stream, baseline, incoming, nil)
}
func (s *Store) reconstruct(ctx context.Context, stream string, baseline history.Baseline, incoming history.Capture, beforeCommit func() error) (history.Reconstruction, error) {
	return s.reconstructAt(ctx, stream, baseline, incoming, nil, beforeCommit)
}
func (s *Store) reconstructAt(ctx context.Context, stream string, baseline history.Baseline, incoming history.Capture, expectedThrough *uint32, beforeCommit func() error) (history.Reconstruction, error) {
	if ValidateReconstructionScope(baseline.Snapshot.Network, stream) != nil || history.ValidateBaseline(baseline) != nil || history.ValidateCaptureEnvelope(incoming) != nil {
		return history.Reconstruction{}, errors.New("invalid reconstruction input")
	}
	if incoming.Network != baseline.Snapshot.Network {
		return history.Reconstruction{}, ErrReconstructionBaseline
	}
	var result history.Reconstruction
	var applicationErr error
	err := s.db.Update(func(tx *bolt.Tx) error {
		if ctx.Err() != nil {
			return ctx.Err()
		}
		b := tx.Bucket(reconstructionBucket)
		lastAttempt, err := loadAttempt(tx, incoming.Network, stream)
		if err != nil {
			return err
		}
		baselineDigest, _ := fleetwatch.SnapshotDigest(baseline.Snapshot)
		if lastAttempt != nil && (lastAttempt.BaselineDigest != baselineDigest || lastAttempt.BaselineHeaderHash != baseline.HeaderHash || lastAttempt.BaselineLedger != baseline.Snapshot.Ledger) {
			return ErrReconstructionBaseline
		}
		var old *ReconstructionState
		if b != nil {
			if raw := b.Get(reconstructionScope(incoming.Network, stream)); raw != nil {
				state, err := decodeReconstruction(ctx, raw, incoming.Network, stream)
				if err != nil {
					return err
				}
				old = &state
				oldDigest, _ := fleetwatch.SnapshotDigest(state.Baseline.Snapshot)
				newDigest, _ := fleetwatch.SnapshotDigest(baseline.Snapshot)
				if oldDigest != newDigest || state.Baseline.HeaderHash != baseline.HeaderHash {
					return ErrReconstructionBaseline
				}
				baseline = state.Baseline
			}
		}

		if expectedThrough != nil {
			actual := baseline.Snapshot.Ledger
			if old != nil {
				actual = old.Report.ProcessedThrough
			}
			if actual != *expectedThrough {
				return ErrReconstructionCursor
			}
		}
		merged, err := mergeReconstructionCapture(baseline, old, incoming)
		if errors.Is(err, ErrConflict) && old != nil {
			result = old.Report
			result.Status = "PARTIAL"
			result.Errors = []string{"EVIDENCE_CONFLICT"}
			applicationErr = ErrConflict
		} else if err != nil {
			return err
		} else {
			result, applicationErr = history.Reconstruct(ctx, baseline, merged)
		}
		if ctx.Err() != nil {
			return ctx.Err()
		}
		if incoming.SequenceCoverage == "PARTIAL" && applicationErr == nil {
			result.Status = "PARTIAL"
			result.Errors = []string{"SOURCE_ATTEMPT_INCOMPLETE"}
			applicationErr = ErrReconstructionIncomplete
		}
		if len(result.Ledgers) == 0 {
			attempt := makeReconstructionAttempt(stream, baseline, incoming, result, applicationErr)
			if err := saveAttempt(tx, attempt); err != nil {
				return err
			}
			if err := enqueueReconstruction(tx, stream, result, attempt); err != nil {
				return err
			}
			if beforeCommit != nil {
				if err := beforeCommit(); err != nil {
					return err
				}
			}
			return ctx.Err()
		}
		if errors.Is(applicationErr, ErrConflict) {
			merged = old.Capture
		}
		retained := merged
		retained.ToLedger = result.ProcessedThrough
		retained.ProcessedThrough = result.ProcessedThrough
		retained.SequenceCoverage = "COMPLETE_ENDPOINT_SEQUENCE"
		retained.Errors = []string{}
		retained.Ledgers = []history.Evidence{}
		for _, e := range merged.Ledgers {
			if e.Sequence <= result.ProcessedThrough {
				retained.Ledgers = append(retained.Ledgers, e)
			}
		}
		committed := result
		if applicationErr != nil {
			committed, err = history.Reconstruct(ctx, baseline, retained)
			if err != nil {
				return errors.New("accepted reconstruction prefix failed revalidation")
			}
		}
		encoded, err := json.Marshal(ReconstructionState{SchemaVersion: 1, Stream: stream, Baseline: baseline, Capture: retained, Report: committed, AlertsEnabled: true})
		if err != nil {
			return err
		}
		if len(encoded) > maxStateBytes {
			return ErrReconstructionBound
		}
		if b == nil {
			b, err = tx.CreateBucket(reconstructionBucket)
			if err != nil {
				return err
			}
		}
		key := reconstructionScope(incoming.Network, stream)
		if b.Get(key) == nil && b.Stats().KeyN >= 128 {
			return ErrReconstructionBound
		}
		if err := b.Put(key, encoded); err != nil {
			return err
		}
		attempt := makeReconstructionAttempt(stream, baseline, incoming, result, applicationErr)
		if err := saveAttempt(tx, attempt); err != nil {
			return err
		}
		if err := enqueueReconstruction(tx, stream, committed, attempt); err != nil {
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
	if applicationErr != nil {
		return result, fmt.Errorf("%w: %w", ErrReconstructionIncomplete, applicationErr)
	}
	return result, nil
}

func mergeReconstructionCapture(b history.Baseline, old *ReconstructionState, incoming history.Capture) (history.Capture, error) {
	first := uint64(b.Snapshot.Ledger) + 1
	if uint64(incoming.FromLedger) < first {
		return history.Capture{}, ErrReconstructionBaseline
	}
	to := incoming.ToLedger
	known := map[uint32]history.Evidence{}
	if old != nil {
		to = max(to, old.Capture.ToLedger)
		for _, e := range old.Capture.Ledgers {
			known[e.Sequence] = e
		}
	}
	if first > uint64(to) || uint64(to)-first >= history.MaxLedgers {
		return history.Capture{}, ErrReconstructionBound
	}
	for _, e := range incoming.Ledgers {
		if previous, ok := known[e.Sequence]; ok && previous != e {
			return history.Capture{}, ErrConflict
		}
		known[e.Sequence] = e
	}
	merged := incoming
	merged.FromLedger = uint32(first)
	merged.ToLedger = to
	merged.ProcessedThrough = 0
	merged.Ledgers = []history.Evidence{}
	merged.Errors = []string{}
	for seq := first; seq <= uint64(to); seq++ {
		e, ok := known[uint32(seq)]
		if !ok {
			break
		}
		merged.Ledgers = append(merged.Ledgers, e)
		merged.ProcessedThrough = uint32(seq)
	}
	merged.SequenceCoverage = "COMPLETE_ENDPOINT_SEQUENCE"
	if merged.ProcessedThrough != to {
		merged.SequenceCoverage = "PARTIAL"
		merged.Errors = []string{"MISSING_OR_UNEXPECTED_LEDGER"}
	}
	return merged, nil
}
