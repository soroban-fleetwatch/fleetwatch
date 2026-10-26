package store

import (
	"context"
	"errors"
	"path/filepath"
	"testing"
)

func TestIngestionPlanBindsBaselineAndResumes(t *testing.T) {
	s := openTest(t, filepath.Join(t.TempDir(), "state.db"))
	defer s.Close()
	b, c := reconstructionFixture(t)
	ctx := context.Background()
	p, err := s.PlanReconstruction(ctx, "ingest", b, 101)
	if err != nil || p.FromLedger != 100 || p.ExpectedThrough != 99 || p.AlreadyRetained || p.LastAttemptStatus != "UNKNOWN" {
		t.Fatal(p, err)
	}
	if _, err := s.ReconstructFrom(ctx, "ingest", b, c, p.ExpectedThrough); err != nil {
		t.Fatal(err)
	}
	p, err = s.PlanReconstruction(ctx, "ingest", b, 101)
	if err != nil || p.FromLedger != 101 || p.ExpectedThrough != 100 || p.LastAttemptStatus != "COMPLETE" {
		t.Fatal(p, err)
	}
	p, err = s.PlanReconstruction(ctx, "ingest", b, 100)
	if err != nil || !p.AlreadyRetained || p.FromLedger != 0 {
		t.Fatal(p, err)
	}
	changed := b
	changed.HeaderHash = "0000000000000000000000000000000000000000000000000000000000000001"
	if _, err := s.PlanReconstruction(ctx, "ingest", changed, 101); !errors.Is(err, ErrReconstructionBaseline) {
		t.Fatal(err)
	}
	for _, to := range []uint32{0, 99, 116} {
		if _, err := s.PlanReconstruction(ctx, "ingest", b, to); !errors.Is(err, ErrReconstructionBound) {
			t.Fatal(to, err)
		}
	}
	cancelled, cancel := context.WithCancel(ctx)
	cancel()
	if _, err := s.PlanReconstruction(cancelled, "ingest", b, 101); !errors.Is(err, context.Canceled) {
		t.Fatal(err)
	}
}
func TestIngestionCompareCursorInsideCommitAndRollback(t *testing.T) {
	s := openTest(t, filepath.Join(t.TempDir(), "state.db"))
	defer s.Close()
	b, c := reconstructionFixture(t)
	ctx := context.Background()
	expected := uint32(99)
	injected := errors.New("abort")
	if _, err := s.reconstructAt(ctx, "ingest", b, c, &expected, func() error { return injected }); !errors.Is(err, injected) {
		t.Fatal(err)
	}
	p, err := s.PlanReconstruction(ctx, "ingest", b, 100)
	if err != nil || p.ExpectedThrough != 99 || p.LastAttemptStatus != "UNKNOWN" {
		t.Fatal(p, err)
	}
	if _, err := s.ReconstructFrom(ctx, "ingest", b, c, expected); err != nil {
		t.Fatal(err)
	}
	if _, err := s.ReconstructFrom(ctx, "ingest", b, c, expected); !errors.Is(err, ErrReconstructionCursor) {
		t.Fatal("stale cursor accepted", err)
	}
	next := nextReconstructionCapture(t, b, c, false)
	if _, err := s.ReconstructFrom(ctx, "ingest", b, next, 99); !errors.Is(err, ErrReconstructionCursor) {
		t.Fatal("incorrect fetch range accepted", err)
	}
	st, found, err := s.LoadReconstruction(ctx, c.Network, "ingest")
	if err != nil || !found || st.Report.ProcessedThrough != 100 {
		t.Fatal(found, err)
	}
	a, err := s.LastReconstructionAttempt(ctx, c.Network, "ingest")
	if err != nil || a.Status != "COMPLETE" || a.ToLedger != 100 {
		t.Fatal(a, err)
	}
}
func TestIngestionPlanFailureOnlyScopeRemainsBound(t *testing.T) {
	s := openTest(t, filepath.Join(t.TempDir(), "state.db"))
	defer s.Close()
	b, c := reconstructionFixture(t)
	ctx := context.Background()
	if _, err := s.Reconstruct(ctx, "ingest", b, nextReconstructionCapture(t, b, c, false)); !errors.Is(err, ErrReconstructionIncomplete) {
		t.Fatal(err)
	}
	p, err := s.PlanReconstruction(ctx, "ingest", b, 101)
	if err != nil || p.ExpectedThrough != 99 || p.FromLedger != 100 || p.LastAttemptStatus != "PARTIAL" {
		t.Fatal(p, err)
	}
	changed := b
	changed.HeaderHash = "0000000000000000000000000000000000000000000000000000000000000001"
	if _, err := s.PlanReconstruction(ctx, "ingest", changed, 101); !errors.Is(err, ErrReconstructionBaseline) {
		t.Fatal(err)
	}
}
