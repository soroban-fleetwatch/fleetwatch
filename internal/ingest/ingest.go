// Package ingest connects bounded RPC metadata capture to durable diagnostic
// reconstruction. It never authenticates the chain or sends notifications.
package ingest

import (
	"context"
	"errors"
	"time"

	"github.com/soroban-fleetwatch/fleetwatch/internal/history"
	"github.com/soroban-fleetwatch/fleetwatch/internal/store"
)

type Result struct {
	SchemaVersion       int                       `json:"schema_version"`
	Network             string                    `json:"network"`
	Stream              string                    `json:"stream"`
	RequestedThrough    uint32                    `json:"requested_through"`
	StartedThrough      uint32                    `json:"started_through"`
	ProcessedThrough    uint32                    `json:"processed_through"`
	FetchFrom           uint32                    `json:"fetch_from"`
	FetchedLedgers      int                       `json:"fetched_ledgers"`
	Status              string                    `json:"status"`
	SourceQuery         string                    `json:"source_query"`
	LatestAttemptStatus string                    `json:"latest_attempt_status"`
	FetchErrors         []string                  `json:"fetch_errors"`
	Assurance           store.ProjectionAssurance `json:"assurance"`
	Report              *history.Reconstruction   `json:"report"`
}

// Run performs one finite fetch. Source failures can commit a validated prefix
// plus a diagnostic; cancellation and storage failures return no success result.
func Run(ctx context.Context, rpc history.RPC, s *store.Store, stream string, b history.Baseline, to uint32) (Result, error) {
	ctx, cancel := context.WithTimeout(ctx, 2*time.Minute)
	defer cancel()
	planCtx, planCancel := context.WithTimeout(ctx, 30*time.Second)
	p, err := s.PlanReconstruction(planCtx, stream, b, to)
	planErr := planCtx.Err()
	planCancel()
	if planErr != nil {
		return Result{}, planErr
	}
	if err != nil {
		return Result{}, err
	}
	out := Result{SchemaVersion: 1, Network: p.Network, Stream: stream, RequestedThrough: to, StartedThrough: p.ExpectedThrough, ProcessedThrough: p.ExpectedThrough, Status: "ALREADY_RETAINED", SourceQuery: "NOT_QUERIED", LatestAttemptStatus: p.LastAttemptStatus, FetchErrors: []string{}, Assurance: store.UnverifiedProjectionAssurance()}
	if p.AlreadyRetained {
		return out, nil
	}
	if rpc == nil {
		return Result{}, errors.New("ingestion RPC is required")
	}
	captured, fetchErr := history.Fetch(ctx, rpc, p.Network, p.FromLedger, to)
	if err := ctx.Err(); err != nil {
		return Result{}, err
	}
	commitCtx, commitCancel := context.WithTimeout(ctx, 30*time.Second)
	defer commitCancel()
	report, err := s.ReconstructFrom(commitCtx, stream, b, captured, p.ExpectedThrough)
	if err != nil && commitCtx.Err() != nil {
		return Result{}, commitCtx.Err()
	}
	if err != nil && !errors.Is(err, store.ErrReconstructionIncomplete) {
		return Result{}, err
	}
	out.FetchFrom = p.FromLedger
	out.FetchedLedgers = len(captured.Ledgers)
	out.SourceQuery = captured.SequenceCoverage
	out.FetchErrors = append(out.FetchErrors, captured.Errors...)
	out.ProcessedThrough = report.ProcessedThrough
	out.Report = &report
	out.Status = "COMPLETE"
	out.LatestAttemptStatus = "COMPLETE"
	if err != nil || fetchErr != nil {
		out.Status = "PARTIAL"
		out.LatestAttemptStatus = "PARTIAL"
		return out, store.ErrReconstructionIncomplete
	}
	return out, nil
}
