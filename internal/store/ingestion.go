package store

import (
	"context"
	"errors"

	"github.com/soroban-fleetwatch/fleetwatch/internal/fleetwatch"
	"github.com/soroban-fleetwatch/fleetwatch/internal/history"
	bolt "go.etcd.io/bbolt"
)

var ErrReconstructionCursor = errors.New("reconstruction cursor changed; replan before retrying")

// ReconstructionPlan is an observation, not a lease. ReconstructFrom checks the
// expected cursor again in its write transaction after the source fetch.
type ReconstructionPlan struct {
	Network           string
	Stream            string
	FromLedger        uint32
	ToLedger          uint32
	ExpectedThrough   uint32
	AlreadyRetained   bool
	LastAttemptStatus string
}

func (s *Store) PlanReconstruction(ctx context.Context, stream string, b history.Baseline, to uint32) (ReconstructionPlan, error) {
	if ValidateReconstructionScope(b.Snapshot.Network, stream) != nil || history.ValidateBaseline(b) != nil {
		return ReconstructionPlan{}, errors.New("invalid ingestion baseline or scope")
	}
	if uint64(to) <= uint64(b.Snapshot.Ledger) || uint64(to)-uint64(b.Snapshot.Ledger) > history.MaxLedgers {
		return ReconstructionPlan{}, ErrReconstructionBound
	}
	p := ReconstructionPlan{Network: b.Snapshot.Network, Stream: stream, ToLedger: to, ExpectedThrough: b.Snapshot.Ledger, LastAttemptStatus: "UNKNOWN"}
	digest, _ := fleetwatch.SnapshotDigest(b.Snapshot)
	err := s.db.View(func(tx *bolt.Tx) error {
		if err := ctx.Err(); err != nil {
			return err
		}
		view, err := inspectReconstruction(ctx, tx, p.Network, stream)
		if err != nil {
			return err
		}
		if view.State != nil {
			state := view.State
			if state.Report.BaselineDigest != digest || state.Baseline.HeaderHash != b.HeaderHash || state.Baseline.Snapshot.Ledger != b.Snapshot.Ledger {
				return ErrReconstructionBaseline
			}
			p.ExpectedThrough = state.Report.ProcessedThrough
		}
		if view.Attempt != nil {
			a := view.Attempt
			if a.BaselineDigest != digest || a.BaselineHeaderHash != b.HeaderHash || a.BaselineLedger != b.Snapshot.Ledger {
				return ErrReconstructionBaseline
			}
			p.LastAttemptStatus = a.Status
		}
		p.AlreadyRetained = to <= p.ExpectedThrough
		if !p.AlreadyRetained {
			p.FromLedger = p.ExpectedThrough + 1
		}
		return ctx.Err()
	})
	if err != nil {
		return ReconstructionPlan{}, err
	}
	return p, nil
}

// ReconstructFrom prevents a batch fetched from a stale cursor being committed.
// Other offline reconstruction entry points retain their overlap/replay behavior.
func (s *Store) ReconstructFrom(ctx context.Context, stream string, b history.Baseline, c history.Capture, expectedThrough uint32) (history.Reconstruction, error) {
	if uint64(c.FromLedger) != uint64(expectedThrough)+1 {
		return history.Reconstruction{}, ErrReconstructionCursor
	}
	return s.reconstructAt(ctx, stream, b, c, &expectedThrough, nil)
}
