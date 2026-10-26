package monitor

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/soroban-fleetwatch/fleetwatch/internal/history"
	"github.com/soroban-fleetwatch/fleetwatch/internal/ingest"
	"github.com/soroban-fleetwatch/fleetwatch/internal/store"
)

func options() Options { return Options{time.Second, 3, time.Minute} }
func scope() Scope     { return Scope{"SYNTHETIC-NETWORK", "watch", 101} }
func result(status string) ingest.Result {
	r := ingest.Result{SchemaVersion: 1, Network: scope().Network, Stream: scope().Stream, RequestedThrough: 101, StartedThrough: 99, ProcessedThrough: 100, FetchFrom: 100, FetchedLedgers: 1, Status: status, SourceQuery: "PARTIAL", LatestAttemptStatus: "PARTIAL", FetchErrors: []string{"LEDGER_QUERY_FAILED"}, Assurance: store.UnverifiedProjectionAssurance(), Report: &history.Reconstruction{Errors: []string{"NORMALIZATION_INCOMPLETE"}}}
	if status == "COMPLETE" {
		r.ProcessedThrough = 101
		r.FetchedLedgers = 2
		r.SourceQuery = "COMPLETE_ENDPOINT_SEQUENCE"
		r.LatestAttemptStatus = "COMPLETE"
		r.FetchErrors = []string{}
		r.Report.Errors = []string{}
	}
	return r
}
func instant(context.Context, time.Duration) error { return nil }
func TestWatchRetryThenCompleteCounters(t *testing.T) {
	calls := 0
	events := []Event{}
	pauses := 0
	last, err := run(context.Background(), scope(), options(), func(context.Context) (ingest.Result, error) {
		calls++
		if calls == 1 {
			return result("PARTIAL"), store.ErrReconstructionIncomplete
		}
		r := result("COMPLETE")
		r.StartedThrough = 100
		r.FetchFrom = 101
		r.FetchedLedgers = 1
		return r, nil
	}, func(e Event) error { events = append(events, e); return nil }, func(context.Context, time.Duration) error { pauses++; return nil }, time.Now)
	if err != nil || last.Status != "COMPLETE" || last.Attempts != 2 || last.PartialAttempts != 1 || last.FetchedLedgers != 2 || last.AcceptedLedgers != 2 || *last.ProcessedThrough != 101 || *last.TargetRemaining != 0 || pauses != 1 || len(events) != 2 || events[0].Status != "RETRYING" || events[0].Sequence != 1 || events[1].Sequence != 2 {
		t.Fatal(last, events, err)
	}
	if *events[0].ProcessedThrough != 100 || *events[0].TargetRemaining != 1 || events[0].FetchErrors[0] != "LEDGER_QUERY_FAILED" {
		t.Fatal("prior event mutated", events[0])
	}
}
func TestWatchAttemptLimitAndCursorReplan(t *testing.T) {
	for _, cursor := range []bool{false, true} {
		t.Run(map[bool]string{false: "source", true: "cursor"}[cursor], func(t *testing.T) {
			calls := 0
			events := []Event{}
			last, err := run(context.Background(), scope(), options(), func(context.Context) (ingest.Result, error) {
				calls++
				if cursor {
					return ingest.Result{}, store.ErrReconstructionCursor
				}
				return result("PARTIAL"), store.ErrReconstructionIncomplete
			}, func(e Event) error { events = append(events, e); return nil }, instant, time.Now)
			if !errors.Is(err, ErrAttemptLimit) || calls != 3 || len(events) != 3 || last.Status != "STOPPED" || last.Code != "ATTEMPT_LIMIT" {
				t.Fatal(last, err)
			}
			if cursor && (last.CursorConflicts != 3 || last.ProcessedThrough != nil || last.SourceQuery != "UNKNOWN") {
				t.Fatal(last)
			}
		})
	}
}
func TestWatchRejectsEvidenceWithoutRetry(t *testing.T) {
	for _, kind := range []string{"network", "metadata", "application", "rejected-prefix"} {
		t.Run(kind, func(t *testing.T) {
			r := result("PARTIAL")
			want := "APPLICATION_REJECTED"
			switch kind {
			case "network":
				r.FetchErrors = []string{"NETWORK_MISMATCH"}
				want = "SOURCE_REJECTED"
			case "metadata":
				r.FetchErrors = []string{"INVALID_OR_UNSUPPORTED_LEDGER_METADATA"}
				want = "SOURCE_REJECTED"
			case "application":
				r.SourceQuery = "COMPLETE_ENDPOINT_SEQUENCE"
				r.FetchErrors = []string{}
				r.Report.Errors = []string{"PREIMAGE_MISMATCH"}
			case "rejected-prefix":
				r.FetchedLedgers = 2
			}
			calls := 0
			last, err := run(context.Background(), scope(), options(), func(context.Context) (ingest.Result, error) { calls++; return r, store.ErrReconstructionIncomplete }, func(Event) error { return nil }, func(context.Context, time.Duration) error { t.Fatal("permanent rejection waited"); return nil }, time.Now)
			if !errors.Is(err, store.ErrReconstructionIncomplete) || calls != 1 || last.Code != want || last.Status != "STOPPED" {
				t.Fatal(last, err)
			}
		})
	}
}
func TestWatchCancellationBeforeAttemptAndDuringWait(t *testing.T) {
	for _, before := range []bool{true, false} {
		t.Run(map[bool]string{true: "before", false: "wait"}[before], func(t *testing.T) {
			ctx, cancel := context.WithCancel(context.Background())
			defer cancel()
			if before {
				cancel()
			}
			calls := 0
			last, err := Run(ctx, scope(), options(), func(context.Context) (ingest.Result, error) {
				calls++
				return result("PARTIAL"), store.ErrReconstructionIncomplete
			}, func(e Event) error {
				if e.Status == "RETRYING" {
					cancel()
				}
				return nil
			})
			wantCalls := 1
			if before {
				wantCalls = 0
			}
			if !errors.Is(err, context.Canceled) || calls != wantCalls || last.Status != "STOPPED" || last.Code != "CANCELLED" {
				t.Fatal(last, err)
			}
			if before && last.ProcessedThrough != nil {
				t.Fatal("fabricated progress")
			}
		})
	}
}
func TestWatchDeadlineAndAttemptTimeout(t *testing.T) {
	ctx, cancel := context.WithDeadline(context.Background(), time.Now().Add(-time.Second))
	defer cancel()
	last, err := Run(ctx, scope(), options(), func(context.Context) (ingest.Result, error) {
		t.Fatal("expired deadline attempted")
		return ingest.Result{}, nil
	}, func(Event) error { return nil })
	if !errors.Is(err, context.DeadlineExceeded) || last.Code != "DEADLINE_EXCEEDED" || last.Attempts != 0 {
		t.Fatal(last, err)
	}
	last, err = Run(context.Background(), scope(), options(), func(context.Context) (ingest.Result, error) { return ingest.Result{}, context.DeadlineExceeded }, func(Event) error { return nil })
	if !errors.Is(err, context.DeadlineExceeded) || last.Code != "DEADLINE_EXCEEDED" || last.FailedAttempts != 1 {
		t.Fatal(last, err)
	}
}
func TestWatchOutputFailureStopsImmediately(t *testing.T) {
	calls := 0
	last, err := run(context.Background(), scope(), options(), func(context.Context) (ingest.Result, error) {
		calls++
		return result("PARTIAL"), store.ErrReconstructionIncomplete
	}, func(Event) error { return errors.New("sensitive output error") }, func(context.Context, time.Duration) error { t.Fatal("wait after failed output"); return nil }, time.Now)
	if !errors.Is(err, ErrOutput) || calls != 1 || last.PartialAttempts != 1 {
		t.Fatal(last, err)
	}
}
func TestWatchNoopPreservesUnknownFindingsAndFailedAttempt(t *testing.T) {
	r := result("PARTIAL")
	r.Status = "ALREADY_RETAINED"
	r.ProcessedThrough = 101
	r.StartedThrough = 101
	r.FetchFrom = 0
	r.FetchedLedgers = 0
	r.SourceQuery = "NOT_QUERIED"
	r.FetchErrors = []string{}
	r.Report = nil
	last, err := Run(context.Background(), scope(), options(), func(context.Context) (ingest.Result, error) { return r, nil }, func(Event) error { return nil })
	if err != nil || last.Status != "ALREADY_RETAINED" || last.LatestAttemptStatus != "PARTIAL" || last.ObservedEvents != nil || last.FetchedLedgers != 0 || last.Assurance.FinalVerdict != "UNDETERMINED" || last.LedgerLag != nil {
		t.Fatal(last, err)
	}
}
func TestWatchInvalidOptionsAndStateFailure(t *testing.T) {
	for _, o := range []Options{{0, 3, time.Minute}, {time.Second, 0, time.Minute}, {time.Second, 121, time.Minute}, {time.Second, 3, 0}, {time.Second, 3, 2 * time.Hour}, {2 * time.Minute, 3, time.Minute}} {
		if _, err := Run(context.Background(), scope(), o, func(context.Context) (ingest.Result, error) {
			t.Fatal("invalid options attempted")
			return ingest.Result{}, nil
		}, func(Event) error { return nil }); !errors.Is(err, ErrOptions) {
			t.Fatal(o, err)
		}
	}
	injected := errors.New("sensitive storage error")
	last, err := Run(context.Background(), scope(), options(), func(context.Context) (ingest.Result, error) { return ingest.Result{}, injected }, func(Event) error { return nil })
	if !errors.Is(err, injected) || last.Code != "STATE_FAILURE" || last.FailedAttempts != 1 {
		t.Fatal(last, err)
	}
	_, err = Run(context.Background(), scope(), options(), func(context.Context) (ingest.Result, error) {
		r := result("COMPLETE")
		r.Network = "other"
		return r, nil
	}, func(Event) error { return nil })
	if err == nil {
		t.Fatal("mismatched result accepted")
	}
}
