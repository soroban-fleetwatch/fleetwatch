package alert

import (
	"context"
	"errors"
	"os"
	"time"

	"github.com/soroban-fleetwatch/fleetwatch/internal/store"
	bolt "go.etcd.io/bbolt"
)

type LoopConfig struct {
	State               string
	Network             string
	Stream              string
	Sink                string
	PollIntervalSeconds int
	BatchSize           int
}

func (c LoopConfig) Validate() error {
	if c.State == "" || store.ValidateReconstructionScope(c.Network, c.Stream) != nil || c.Sink == "" || len(c.Sink) > 256 || c.PollIntervalSeconds < 1 || c.PollIntervalSeconds > 60 || c.BatchSize < 1 || c.BatchSize > 5 {
		return errors.New("invalid follow delivery loop configuration")
	}
	return nil
}

type QueueCounts struct {
	Pending   int `json:"pending"`
	Inflight  int `json:"inflight"`
	Delivered int `json:"delivered"`
	Dead      int `json:"dead"`
}
type LoopStatus struct {
	SchemaVersion int          `json:"schema_version"`
	Mode          string       `json:"mode"`
	Cycle         uint64       `json:"cycle"`
	At            int64        `json:"at_unix"`
	Code          string       `json:"code"`
	Batch         Result       `json:"batch"`
	Queue         *QueueCounts `json:"queue"`
	FinalVerdict  string       `json:"final_verdict"`
}

// DeliveryLoop runs only for imported follow alerts. A receiver is selected by
// the explicit sender, never discovered from source state. File ownership lasts
// one bounded batch; waits release the DB for the follower's automatic sync.
func DeliveryLoop(ctx context.Context, cfg LoopConfig, sender Sender, emit func(LoopStatus) error) error {
	if cfg.Validate() != nil || sender == nil || emit == nil {
		return errors.New("invalid follow delivery loop")
	}
	var cycle uint64
	for ctx.Err() == nil {
		cycle++
		status := LoopStatus{SchemaVersion: 1, Mode: "EXPERIMENTAL_FOLLOW_DISPATCH", Cycle: cycle, At: time.Now().Unix(), FinalVerdict: "UNDETERMINED"}
		err := deliveryCycle(ctx, cfg, sender, &status)
		if ctx.Err() != nil {
			break
		}
		if err != nil {
			status.Code = "STATE_FAILURE"
			status.Queue = nil
			if emitErr := emit(status); emitErr != nil {
				return emitErr
			}
			return err
		}
		if err := emit(status); err != nil {
			return err
		}
		timer := time.NewTimer(time.Duration(cfg.PollIntervalSeconds) * time.Second)
		select {
		case <-ctx.Done():
			timer.Stop()
		case <-timer.C:
		}
	}
	return emit(LoopStatus{SchemaVersion: 1, Mode: "EXPERIMENTAL_FOLLOW_DISPATCH", Cycle: cycle, At: time.Now().Unix(), Code: "STOPPED", FinalVerdict: "UNDETERMINED"})
}

func deliveryCycle(ctx context.Context, cfg LoopConfig, sender Sender, status *LoopStatus) (resultErr error) {
	// A concurrently starting producer briefly exposes an empty file before its
	// initialization transaction. Do not initialize it, or mislabel it healthy.
	if info, err := os.Lstat(cfg.State); err == nil && info.Mode().IsRegular() && info.Size() == 0 {
		status.Code = "OUTBOX_EMPTY"
		return nil
	}
	s, err := store.OpenExisting(cfg.State)
	if errors.Is(err, os.ErrNotExist) {
		status.Code = "WAITING_FOR_OUTBOX"
		return nil
	}
	if errors.Is(err, bolt.ErrTimeout) {
		status.Code = "OUTBOX_BUSY"
		return nil
	}
	if err != nil {
		return errors.New("delivery outbox unavailable or invalid")
	}
	// Close before returning to the caller's status output or interval wait.
	defer func() {
		if err := s.Close(); err != nil && resultErr == nil {
			resultErr = errors.New("delivery outbox close failed")
		}
	}()
	if err := ctx.Err(); err != nil {
		return err
	}
	batchCtx, cancel := context.WithTimeout(ctx, 90*time.Second)
	defer cancel()
	binding, err := s.FollowImportBinding(batchCtx, cfg.Network, cfg.Stream)
	if err != nil {
		return errors.New("delivery import manifest rejected")
	}
	if binding == nil {
		// Also refuse an orphan queue with no manifest, instead of treating it
		// as an empty healthy queue.
		if _, err := s.FollowAlerts(batchCtx, cfg.Network, cfg.Stream); err != nil {
			return errors.New("delivery queue without valid manifest")
		}
		status.Code = "WAITING_FOR_IMPORT"
		return nil
	}
	source, _ := store.AlertSource(cfg.Network, "follow_ledger_meta", cfg.Stream)
	result, dispatchErr := Dispatch(batchCtx, s, sender, cfg.Network, source, cfg.Sink, cfg.BatchSize)
	status.Batch = result
	if dispatchErr != nil && !errors.Is(dispatchErr, context.DeadlineExceeded) && !errors.Is(dispatchErr, store.ErrStaleLease) && ctx.Err() == nil {
		return errors.New("delivery batch state or acknowledgement failed")
	}
	if ctx.Err() != nil {
		return ctx.Err()
	}
	inspectCtx, inspectCancel := context.WithTimeout(ctx, 5*time.Second)
	defer inspectCancel()
	rows, err := s.FollowAlerts(inspectCtx, cfg.Network, cfg.Stream)
	if err != nil {
		return errors.New("delivery queue failed post-batch validation")
	}
	counts := &QueueCounts{}
	for _, row := range rows {
		switch row.Status {
		case "PENDING":
			counts.Pending++
		case "INFLIGHT":
			counts.Inflight++
		case "DELIVERED":
			counts.Delivered++
		case "DEAD":
			counts.Dead++
		}
	}
	status.Queue = counts
	switch {
	case errors.Is(dispatchErr, store.ErrStaleLease):
		status.Code = "ACK_UNCERTAIN"
	case dispatchErr != nil:
		status.Code = "BATCH_TIMEOUT"
	case counts.Dead > 0:
		status.Code = "DEAD_LETTERS_PRESENT"
	case result.Failed > 0:
		status.Code = "DELIVERY_RETRY"
	case result.Delivered > 0:
		status.Code = "DISPATCHED"
	case counts.Pending > 0 || counts.Inflight > 0:
		status.Code = "WAITING_FOR_RETRY"
	default:
		status.Code = "IDLE"
	}
	return nil
}
