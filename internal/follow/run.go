package follow

import (
	"context"
	"errors"
	"time"

	"github.com/soroban-fleetwatch/fleetwatch/internal/history"
	"github.com/soroban-fleetwatch/fleetwatch/internal/source"
)

type RPC interface {
	history.RPC
	Head(context.Context) (source.Head, error)
}

func available(code string) bool {
	switch code {
	case "NETWORK_QUERY_FAILED", "LEDGER_QUERY_FAILED", "RETENTION_RANGE_UNAVAILABLE", "MISSING_OR_UNEXPECTED_LEDGER":
		return true
	}
	return false
}

// Poll targets only closed ledgers reported by this endpoint, never guesses a
// future target, skips missing history or refreshes the original baseline.
func Poll(ctx context.Context, rpc RPC, s *Store, cfg Config, previous Cycle, now time.Time) (Cycle, error) {
	started := time.Now()
	c := previous
	c.At = now.Unix()
	c.Polls++
	c.EndpointHead = nil
	fail := func(code string, err error) (Cycle, error) {
		c.Code = code
		if code == "SOURCE_UNAVAILABLE" && previous.Code != "SOURCE_UNAVAILABLE" {
			c.GapEpisodes++
		}
		return c, err
	}
	n, err := rpc.Network(ctx)
	if err != nil {
		return fail("SOURCE_UNAVAILABLE", nil)
	}
	head := s.Head()
	network := head.Projection.Snapshot.Network
	if n.Passphrase != network || n.ProtocolVersion < 28 || n.ProtocolVersion > 29 {
		return fail("EVIDENCE_REJECTED", ErrEvidence)
	}
	h, err := rpc.Head(ctx)
	if err != nil {
		return fail("SOURCE_UNAVAILABLE", nil)
	}
	// Account for time spent obtaining network/head responses. Freshness in
	// status still conservatively uses the cycle's start, not completion.
	age := now.Add(time.Since(started)).Sub(time.Unix(h.CloseTime, 0))
	if h.Sequence == 0 || h.ProtocolVersion != uint32(n.ProtocolVersion) || age < -30*time.Second || age > 2*time.Minute || h.Sequence < head.Projection.Snapshot.Ledger || h.Sequence < previous.HighWaterHead {
		return fail("EVIDENCE_REJECTED", ErrEvidence)
	}
	c.EndpointHead = &h.Sequence
	c.HighWaterHead = h.Sequence
	through := head.Projection.Snapshot.Ledger
	if h.Sequence <= cfg.HeadDelayLedgers || h.Sequence-cfg.HeadDelayLedgers <= through {
		c.Code = "IDLE"
		c.SuccessfulAt = now.Unix()
		return c, nil
	}
	if head.RetainedLedgers >= cfg.MaxRetainedLedgers || head.PendingFindings >= 10000 {
		return fail("CAPACITY_REACHED", ErrCapacity)
	}
	to := min(uint64(h.Sequence-cfg.HeadDelayLedgers), uint64(through)+uint64(cfg.BatchSize), uint64(through)+uint64(cfg.MaxRetainedLedgers-head.RetainedLedgers))
	capture, fetchErr := history.Fetch(ctx, rpc, network, through+1, uint32(to))
	if ctx.Err() != nil {
		return c, ctx.Err()
	}
	report, applicationErr := s.Commit(ctx, capture)
	if ctx.Err() != nil {
		return c, ctx.Err()
	}
	if errors.Is(applicationErr, ErrCapacity) {
		return fail("CAPACITY_REACHED", ErrCapacity)
	}
	if report.SchemaVersion == 0 {
		return fail("STATE_FAILURE", ErrState)
	}
	if fetchErr != nil || applicationErr != nil {
		if len(capture.Errors) == 1 && available(capture.Errors[0]) && len(report.Errors) == 1 && report.Errors[0] == "NORMALIZATION_INCOMPLETE" && len(report.Ledgers) == len(capture.Ledgers) {
			return fail("SOURCE_UNAVAILABLE", nil)
		}
		return fail("EVIDENCE_REJECTED", ErrEvidence)
	}
	c.Code = "FOLLOWING"
	c.SuccessfulAt = now.Unix()
	return c, nil
}

// Run is cancellable without a fixed ledger lifetime. Configured retained-byte,
// ledger and finding budgets still stop it safely; no history is auto-pruned.
func Run(ctx context.Context, rpc RPC, s *Store, cfg Config, health *Health, emit func(Status) error) error {
	return RunWithSync(ctx, rpc, s, cfg, health, emit, nil)
}

var ErrSync = errors.New("follow outbox synchronization failed")

// RunWithSync calls an optional bounded hook at startup and after each nonfatal
// poll, before publishing a successful cycle. It serializes the hook with source
// commits. The two stores are not one transaction: startup sync repairs a crash
// after a source commit but before outbox import. No native finding is cleared.
func RunWithSync(ctx context.Context, rpc RPC, s *Store, cfg Config, health *Health, emit func(Status) error, syncOutbox func(context.Context) error) error {
	if cfg.Validate() != nil || rpc == nil || s == nil || health == nil || emit == nil {
		return ErrState
	}
	previous, err := s.Cycle()
	if err != nil {
		return err
	}
	syncCycle := func(cycle *Cycle) error {
		if syncOutbox == nil || ctx.Err() != nil {
			return ctx.Err()
		}
		callCtx, cancel := context.WithTimeout(ctx, 2*time.Minute)
		err := syncOutbox(callCtx)
		cancel()
		if ctx.Err() != nil {
			return ctx.Err()
		}
		if err == nil {
			return nil
		}
		cycle.Code = "OUTBOX_SYNC_FAILED"
		cycle.At = time.Now().Unix()
		cycle.SuccessfulAt = previous.SuccessfulAt
		cycle.EndpointHead = nil
		health.Update(s.Head(), *cycle)
		if saveErr := s.SaveCycle(ctx, *cycle); saveErr != nil {
			return saveErr
		}
		if emitErr := emit(health.Snapshot(time.Now())); emitErr != nil {
			return emitErr
		}
		return ErrSync
	}
	if err := syncCycle(&previous); err != nil && ctx.Err() == nil {
		return err
	}
	for {
		if ctx.Err() != nil {
			break
		}
		callCtx, cancel := context.WithTimeout(ctx, 2*time.Minute)
		cycle, pollErr := Poll(callCtx, rpc, s, cfg, previous, time.Now())
		cancel()
		if ctx.Err() != nil {
			break
		}
		if errors.Is(pollErr, context.DeadlineExceeded) {
			cycle.Code = "SOURCE_UNAVAILABLE"
			if previous.Code != "SOURCE_UNAVAILABLE" {
				cycle.GapEpisodes++
			}
			pollErr = nil
		}
		if pollErr == nil {
			if err := syncCycle(&cycle); err != nil {
				if ctx.Err() != nil {
					break
				}
				return err
			}
		}
		if err := s.SaveCycle(ctx, cycle); err != nil {
			return err
		}
		previous = cycle
		health.Update(s.Head(), cycle)
		if err := emit(health.Snapshot(time.Now())); err != nil {
			return err
		}
		if pollErr != nil {
			return pollErr
		}
		timer := time.NewTimer(time.Duration(cfg.PollIntervalSeconds) * time.Second)
		select {
		case <-ctx.Done():
			timer.Stop()
		case <-timer.C:
		}
	}
	previous.Code = "STOPPED"
	previous.At = time.Now().Unix()
	previous.EndpointHead = nil
	stopCtx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	if err := s.SaveCycle(stopCtx, previous); err != nil {
		return err
	}
	health.Update(s.Head(), previous)
	return nil
}
