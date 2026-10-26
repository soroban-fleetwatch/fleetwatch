// Package monitor runs finite, resumable diagnostic ingestion jobs.
package monitor

import (
	"context"
	"errors"
	"time"

	"github.com/soroban-fleetwatch/fleetwatch/internal/history"
	"github.com/soroban-fleetwatch/fleetwatch/internal/ingest"
	"github.com/soroban-fleetwatch/fleetwatch/internal/store"
)

var ErrAttemptLimit = errors.New("watch attempt limit reached")
var ErrOutput = errors.New("watch progress output failed")
var ErrOptions = errors.New("invalid bounded watch options")

type Options struct {
	PollInterval time.Duration
	MaxAttempts  int
	MaxDuration  time.Duration
}

func (o Options) Validate() error {
	if o.PollInterval < time.Second || o.PollInterval > time.Minute || o.MaxAttempts < 1 || o.MaxAttempts > 120 || o.MaxDuration < time.Second || o.MaxDuration > time.Hour {
		return ErrOptions
	}
	return nil
}

type Scope struct {
	Network, Stream string
	ToLedger        uint32
}

// Event contains process-local counters, not durable metrics or live chain lag.
type Event struct {
	SchemaVersion       int                       `json:"schema_version"`
	Kind                string                    `json:"kind"`
	Sequence            int                       `json:"sequence"`
	Network             string                    `json:"network"`
	Stream              string                    `json:"stream"`
	TargetLedger        uint32                    `json:"target_ledger"`
	Status              string                    `json:"status"`
	Code                string                    `json:"code"`
	Attempts            int                       `json:"attempts"`
	PartialAttempts     int                       `json:"partial_attempts"`
	CursorConflicts     int                       `json:"cursor_conflicts"`
	FailedAttempts      int                       `json:"failed_attempts"`
	FetchedLedgers      int                       `json:"fetched_ledgers"`
	AcceptedLedgers     int                       `json:"accepted_ledgers"`
	ProcessedThrough    *uint32                   `json:"processed_through"`
	TargetRemaining     *uint32                   `json:"target_remaining"`
	ObservedEvents      *int                      `json:"observed_events"`
	LatestAttemptStatus string                    `json:"latest_attempt_status"`
	SourceQuery         string                    `json:"source_query"`
	FetchErrors         []string                  `json:"fetch_errors"`
	ElapsedMilliseconds int64                     `json:"elapsed_milliseconds"`
	Assurance           store.ProjectionAssurance `json:"assurance"`
	SourceFreshness     string                    `json:"source_freshness"`
	LedgerLag           *uint32                   `json:"ledger_lag"`
}

type Attempt func(context.Context) (ingest.Result, error)
type Emit func(Event) error

func Run(ctx context.Context, scope Scope, options Options, attempt Attempt, emit Emit) (Event, error) {
	return run(ctx, scope, options, attempt, emit, wait, time.Now)
}
func wait(ctx context.Context, delay time.Duration) error {
	timer := time.NewTimer(delay)
	defer timer.Stop()
	select {
	case <-ctx.Done():
		return ctx.Err()
	case <-timer.C:
		return nil
	}
}
func run(ctx context.Context, scope Scope, options Options, attempt Attempt, emit Emit, pause func(context.Context, time.Duration) error, clock func() time.Time) (Event, error) {
	if options.Validate() != nil || store.ValidateReconstructionScope(scope.Network, scope.Stream) != nil || scope.ToLedger == 0 || attempt == nil || emit == nil {
		return Event{}, ErrOptions
	}
	ctx, cancel := context.WithTimeout(ctx, options.MaxDuration)
	defer cancel()
	started := clock()
	out := Event{SchemaVersion: 1, Kind: "watch_progress", Network: scope.Network, Stream: scope.Stream, TargetLedger: scope.ToLedger, LatestAttemptStatus: "UNKNOWN", SourceQuery: "UNKNOWN", FetchErrors: []string{}, Assurance: store.UnverifiedProjectionAssurance(), SourceFreshness: "UNKNOWN"}
	publish := func(status, code string) error {
		out.Sequence++
		out.Status = status
		out.Code = code
		out.ElapsedMilliseconds = max(0, clock().Sub(started).Milliseconds())
		if err := emit(out); err != nil {
			return ErrOutput
		}
		return nil
	}
	stop := func(code string, err error) (Event, error) {
		if outputErr := publish("STOPPED", code); outputErr != nil {
			return out, outputErr
		}
		return out, err
	}
	contextCode := func(err error) string {
		if errors.Is(err, context.DeadlineExceeded) {
			return "DEADLINE_EXCEEDED"
		}
		return "CANCELLED"
	}
	for {
		if err := ctx.Err(); err != nil {
			return stop(contextCode(err), err)
		}
		result, err := attempt(ctx)
		out.Attempts++
		out.SourceQuery = "UNKNOWN"
		out.FetchErrors = []string{}
		retry := false
		code := ""
		switch {
		case err == nil || errors.Is(err, store.ErrReconstructionIncomplete):
			if result.SchemaVersion != 1 || result.Network != scope.Network || result.Stream != scope.Stream || result.RequestedThrough != scope.ToLedger || result.StartedThrough == 0 || result.ProcessedThrough < result.StartedThrough || uint64(result.ProcessedThrough)-uint64(result.StartedThrough) > history.MaxLedgers || result.FetchedLedgers < 0 || result.FetchedLedgers > history.MaxLedgers || result.Assurance != store.UnverifiedProjectionAssurance() {
				out.FailedAttempts++
				return stop("STATE_FAILURE", errors.New("inconsistent ingestion result"))
			}
			through := result.ProcessedThrough
			out.ProcessedThrough = &through
			remaining := uint32(0)
			if through < scope.ToLedger {
				remaining = scope.ToLedger - through
			}
			out.TargetRemaining = &remaining
			out.FetchedLedgers += result.FetchedLedgers
			out.AcceptedLedgers += int(result.ProcessedThrough - result.StartedThrough)
			out.LatestAttemptStatus = result.LatestAttemptStatus
			out.SourceQuery = result.SourceQuery
			out.FetchErrors = append([]string{}, result.FetchErrors...)
			if result.Report != nil {
				count := 0
				for _, l := range result.Report.Ledgers {
					count += len(l.Events)
				}
				out.ObservedEvents = &count
			}
			if err == nil {
				if result.ProcessedThrough < scope.ToLedger || (result.Status != "COMPLETE" && result.Status != "ALREADY_RETAINED") || (result.Status == "COMPLETE" && result.Report == nil) {
					out.FailedAttempts++
					return stop("STATE_FAILURE", errors.New("incomplete successful ingestion result"))
				}
				if outputErr := publish(result.Status, "TARGET_RETAINED"); outputErr != nil {
					return out, outputErr
				}
				return out, nil
			}
			out.PartialAttempts++
			retry = retryable(result)
			code = "RETRYABLE_SOURCE_FAILURE"
			if !retry {
				code = "APPLICATION_REJECTED"
				if result.SourceQuery == "PARTIAL" && !availabilityFailure(result.FetchErrors) {
					code = "SOURCE_REJECTED"
				}
				return stop(code, err)
			}
		case errors.Is(err, store.ErrReconstructionCursor):
			out.CursorConflicts++
			retry = true
			code = "CURSOR_CHANGED"
		case errors.Is(err, context.Canceled) || errors.Is(err, context.DeadlineExceeded):
			out.FailedAttempts++
			return stop(contextCode(err), err)
		default:
			out.FailedAttempts++
			return stop("STATE_FAILURE", err)
		}
		if retry {
			if out.Attempts >= options.MaxAttempts {
				return stop("ATTEMPT_LIMIT", ErrAttemptLimit)
			}
			if err := ctx.Err(); err != nil {
				return stop(contextCode(err), err)
			}
			if err := publish("RETRYING", code); err != nil {
				return out, err
			}
			if err := pause(ctx, options.PollInterval); err != nil {
				return stop(contextCode(err), err)
			}
		}
	}
}

// Retry only if every decoded source record was accepted and the remaining
// failure is a bounded availability/missing-range condition, not rejected data.
func retryable(r ingest.Result) bool {
	if r.Status != "PARTIAL" || r.SourceQuery != "PARTIAL" || len(r.FetchErrors) != 1 || r.Report == nil || len(r.Report.Errors) != 1 {
		return false
	}
	if r.FetchedLedgers > 0 && uint64(r.ProcessedThrough) < uint64(r.FetchFrom)+uint64(r.FetchedLedgers)-1 {
		return false
	}
	if r.Report.Errors[0] != "NORMALIZATION_INCOMPLETE" && r.Report.Errors[0] != "SOURCE_ATTEMPT_INCOMPLETE" {
		return false
	}
	return availabilityFailure(r.FetchErrors)
}
func availabilityFailure(codes []string) bool {
	if len(codes) != 1 {
		return false
	}
	switch codes[0] {
	case "NETWORK_QUERY_FAILED", "LEDGER_QUERY_FAILED", "RETENTION_RANGE_UNAVAILABLE", "MISSING_OR_UNEXPECTED_LEDGER":
		return true
	}
	return false
}
