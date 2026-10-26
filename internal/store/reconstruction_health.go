package store

import (
	"context"
	"errors"
	"time"

	bolt "go.etcd.io/bbolt"
)

// ReconstructionHealth describes local consistency, never live source readiness.
// Its separate schema leaves existing snapshot health output unchanged.
type ReconstructionHealth struct {
	SchemaVersion    int                    `json:"schema_version"`
	Network          string                 `json:"network"`
	Source           string                 `json:"source"`
	Stream           string                 `json:"stream"`
	CheckedAt        string                 `json:"checked_at"`
	Status           string                 `json:"status"`
	Reasons          []string               `json:"reasons"`
	StateFound       bool                   `json:"state_found"`
	Coverage         string                 `json:"coverage"`
	ProcessedThrough uint32                 `json:"processed_through"`
	RetainedLedgers  int                    `json:"retained_ledgers"`
	Events           int                    `json:"events"`
	Diagnostics      int                    `json:"diagnostics"`
	OutboxIntegrated bool                   `json:"outbox_integrated"`
	LastAttempt      *ReconstructionAttempt `json:"last_attempt"`
	Outbox           OutboxHealth           `json:"outbox"`
	RecoveryReceipts int                    `json:"recovery_receipts"`
	RecoveredAlerts  int                    `json:"recovered_alerts"`
	EvaluationStatus string                 `json:"evaluation_status"`
	Assurance        ProjectionAssurance    `json:"assurance"`
	SourceFreshness  string                 `json:"source_freshness"`
	LedgerLag        *uint32                `json:"ledger_lag"`
}

func (s *Store) ReconstructionHealth(ctx context.Context, network, stream string, now time.Time) (ReconstructionHealth, error) {
	if ValidateReconstructionScope(network, stream) != nil || now.Unix() <= 0 {
		return ReconstructionHealth{}, errors.New("invalid reconstruction health scope or clock")
	}
	out := ReconstructionHealth{SchemaVersion: 1, Network: network, Source: "rpc_ledger_meta", Stream: stream, CheckedAt: now.UTC().Format(time.RFC3339Nano), Status: "NO_STATE", Reasons: []string{}, Coverage: "RETAINED_PREFIX_ONLY", EvaluationStatus: "UNVERIFIED_PROJECTED_STATE", Assurance: projectionAssurance(), SourceFreshness: "UNKNOWN"}
	err := s.db.View(func(tx *bolt.Tx) error {
		if err := ctx.Err(); err != nil {
			return err
		}
		view, err := inspectReconstruction(ctx, tx, network, stream)
		if err != nil {
			return err
		}
		if view.State != nil {
			state := view.State
			out.StateFound = true
			out.ProcessedThrough = state.Report.ProcessedThrough
			out.RetainedLedgers = len(state.Capture.Ledgers)
			out.OutboxIntegrated = state.AlertsEnabled
			for _, l := range state.Report.Ledgers {
				out.Events += len(l.Events)
			}
		}
		out.LastAttempt = view.Attempt
		if !out.StateFound && view.Attempt != nil {
			out.ProcessedThrough = view.Attempt.RetainedThrough
		}
		source, _ := AlertSource(network, "rpc_ledger_meta", stream)
		audit, err := readRecoveryAudit(ctx, tx, network, source)
		if err != nil {
			return err
		}
		for _, a := range view.Alerts {
			if err := ctx.Err(); err != nil {
				return err
			}
			receipts, err := validateAlertRecoveries(a, audit[a.ID])
			if err != nil {
				return err
			}
			out.RecoveryReceipts += len(receipts)
			if len(receipts) > 0 {
				out.RecoveredAlerts++
			}
			delete(audit, a.ID)
			if a.Payload.Diagnostic != nil {
				out.Diagnostics++
			}
			countOutboxHealth(&out.Outbox, a, now)
		}
		if len(audit) > 0 {
			return errors.New("recovery audit refers to missing alert")
		}
		return ctx.Err()
	})
	if err != nil {
		return ReconstructionHealth{}, err
	}
	if out.StateFound {
		out.Status = "OK_OFFLINE"
	}
	add := func(failed bool, code string) {
		if failed {
			out.Reasons = append(out.Reasons, code)
			out.Status = "NEEDS_ATTENTION"
		}
	}
	if !out.StateFound {
		out.Reasons = append(out.Reasons, "NO_RECONSTRUCTION_STATE")
	}
	add(out.LastAttempt != nil && out.LastAttempt.Status == "PARTIAL", "LATEST_ATTEMPT_INCOMPLETE")
	add(out.StateFound && out.LastAttempt == nil, "LATEST_ATTEMPT_UNKNOWN")
	add(out.StateFound && !out.OutboxIntegrated, "ALERTS_NOT_INTEGRATED")
	add(out.Outbox.Dead > 0, "DELIVERY_DEAD_LETTERS")
	add(out.Outbox.ExpiredLeases > 0, "DELIVERY_EXPIRED_LEASES")
	add(out.Outbox.ExhaustedUnsettled > 0, "DELIVERY_ATTEMPTS_EXHAUSTED")
	return out, nil
}

// countOutboxHealth is shared by both report formats so lease boundaries agree.
func countOutboxHealth(out *OutboxHealth, a Alert, now time.Time) {
	out.Total++
	switch a.Status {
	case "PENDING":
		out.Pending++
		if a.NextAttempt <= now.Unix() {
			out.Due++
		} else {
			out.Scheduled++
		}
	case "INFLIGHT":
		out.Inflight++
		if a.LeaseUntil <= now.Unix() {
			out.ExpiredLeases++
		}
	case "DELIVERED":
		out.Delivered++
	case "DEAD":
		out.Dead++
	}
	if (a.Status == "PENDING" || a.Status == "INFLIGHT" && a.LeaseUntil <= now.Unix()) && a.Attempts >= MaxDeliveryAttempts {
		out.ExhaustedUnsettled++
	}
}
