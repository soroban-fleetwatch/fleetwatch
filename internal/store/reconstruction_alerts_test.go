package store

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/soroban-fleetwatch/fleetwatch/internal/history"
	bolt "go.etcd.io/bbolt"
)

func TestReconstructionAlertsAtomicDedupAndAcknowledgedResume(t *testing.T) {
	path := filepath.Join(t.TempDir(), "state.db")
	s := openTest(t, path)
	b, c := reconstructionFixture(t)
	ctx := context.Background()
	r, err := s.Reconstruct(ctx, "alerts", b, c)
	if err != nil {
		t.Fatal(err)
	}
	rows, err := s.ReconstructionAlerts(ctx, c.Network, "alerts")
	if err != nil || len(rows) != 2 {
		t.Fatal(rows, err)
	}
	source, _ := AlertSource(c.Network, "rpc_ledger_meta", "alerts")
	now := time.Unix(1000, 0)
	a, err := s.Claim(c.Network, source, "sink", now, time.Minute)
	if err != nil || a == nil {
		t.Fatal(err)
	}
	if err := s.Resolve(c.Network, source, a.ID, a.LeaseToken, true, false, now); err != nil {
		t.Fatal(err)
	}
	s.Close()
	s = openTest(t, path)
	defer s.Close()
	if _, err := s.Reconstruct(ctx, "alerts", b, c); err != nil {
		t.Fatal(err)
	}
	if _, err := s.Reconstruct(ctx, "alerts", b, nextReconstructionCapture(t, b, c, false)); err != nil {
		t.Fatal(err)
	}
	rows, err = s.ReconstructionAlerts(ctx, c.Network, "alerts")
	if err != nil || len(rows) != 3 {
		t.Fatal(rows, err)
	}
	delivered := 0
	for _, row := range rows {
		if row.Status == "DELIVERED" {
			delivered++
		}
		if row.Payload.SchemaVersion != 2 || row.Payload.Source != "rpc_ledger_meta" || row.Payload.Stream != "alerts" || row.Payload.Assurance.FinalVerdict != "UNDETERMINED" || row.Payload.Reconstruction == nil || row.Token != "" || row.LeaseToken != "" {
			t.Fatal("unsafe reconstruction alert", row)
		}
		if row.ID == row.Payload.Reconstruction.Event.ID {
			t.Fatal("alert omitted stream-scoped identity")
		}
	}
	if delivered != 1 {
		t.Fatal("acknowledged finding was requeued")
	}
	other, err := s.ReconstructionAlerts(ctx, c.Network, "other")
	if err != nil || len(other) != 0 {
		t.Fatal("stream leak", err)
	}
	if r.Ledgers[0].Events[0].Finding.Authorization != "UNVERIFIED" {
		t.Fatal("authorization changed")
	}
}

func TestReconstructionFailureDiagnosticSurvivesRepairAndBindsBaseline(t *testing.T) {
	s := openTest(t, filepath.Join(t.TempDir(), "state.db"))
	defer s.Close()
	b, c := reconstructionFixture(t)
	ctx := context.Background()
	next := nextReconstructionCapture(t, b, c, false)
	for i := 0; i < 2; i++ {
		if _, err := s.Reconstruct(ctx, "gap", b, next); !errors.Is(err, ErrReconstructionIncomplete) {
			t.Fatal(err)
		}
	}
	rows, err := s.ReconstructionAlerts(ctx, c.Network, "gap")
	if err != nil || len(rows) != 1 || rows[0].Payload.Diagnostic == nil || rows[0].Payload.Diagnostic.RetainedThrough != 99 {
		t.Fatal(rows, err)
	}
	changed := b
	changed.HeaderHash = strings.Repeat("1", 64)
	if _, err := s.Reconstruct(ctx, "gap", changed, c); !errors.Is(err, ErrReconstructionBaseline) {
		t.Fatal("failure-only stream baseline changed", err)
	}
	if _, err := s.Reconstruct(ctx, "gap", b, c); err != nil {
		t.Fatal(err)
	}
	if _, err := s.Reconstruct(ctx, "gap", b, next); err != nil {
		t.Fatal(err)
	}
	rows, err = s.ReconstructionAlerts(ctx, c.Network, "gap")
	if err != nil || len(rows) != 4 {
		t.Fatal(rows, err)
	}
	last, err := s.LastReconstructionAttempt(ctx, c.Network, "gap")
	if err != nil || last.Status != "COMPLETE" || last.RetainedThrough != 101 {
		t.Fatal(last, err)
	}
	bad := c
	bad.Ledgers = append([]history.Evidence{}, c.Ledgers...)
	bad.Ledgers[0].MetadataSHA256 = strings.Repeat("0", 64)
	if _, err := s.Reconstruct(ctx, "gap", b, bad); !errors.Is(err, ErrConflict) {
		t.Fatal(err)
	}
	last, err = s.LastReconstructionAttempt(ctx, c.Network, "gap")
	if err != nil || last.Code != "EVIDENCE_CONFLICT" || last.RetainedThrough != 101 {
		t.Fatal(last, err)
	}
	rows, err = s.ReconstructionAlerts(ctx, c.Network, "gap")
	if err != nil || len(rows) != 5 {
		t.Fatal(rows, err)
	}
}

func TestReconstructionAlertAbortAndFullQueueCannotAdvanceCheckpoint(t *testing.T) {
	s := openTest(t, filepath.Join(t.TempDir(), "state.db"))
	defer s.Close()
	b, c := reconstructionFixture(t)
	ctx := context.Background()
	injected := errors.New("abort")
	if _, err := s.reconstruct(ctx, "atomic", b, c, func() error { return injected }); !errors.Is(err, injected) {
		t.Fatal(err)
	}
	if rows, err := s.ReconstructionAlerts(ctx, c.Network, "atomic"); err != nil || len(rows) != 0 {
		t.Fatal("queue survived rollback", err)
	}
	if last, err := s.LastReconstructionAttempt(ctx, c.Network, "atomic"); err != nil || last != nil {
		t.Fatal("attempt survived rollback", err)
	}
	if _, err := s.Reconstruct(ctx, "full", b, c); err != nil {
		t.Fatal(err)
	}
	source, _ := AlertSource(c.Network, "rpc_ledger_meta", "full")
	if err := s.db.Update(func(tx *bolt.Tx) error {
		bucket, err := queue(tx, c.Network, source, false)
		if err != nil {
			return err
		}
		template := makeReconstructionAttempt("full", b, c, history.Reconstruction{ProcessedThrough: 99, Errors: []string{"NORMALIZATION_INCOMPLETE"}}, ErrReconstructionIncomplete)
		for i := 0; i < 9998; i++ {
			a := template
			a.InputDigest = fmt.Sprintf("%064x", i+1)
			assurance := projectionAssurance()
			p := AlertPayload{SchemaVersion: 2, Network: c.Network, Source: "rpc_ledger_meta", Stream: "full", EvaluationStatus: "UNVERIFIED_PROJECTED_STATE", Assurance: &assurance, Diagnostic: &a}
			if err := writeAlert(bucket, Alert{ID: diagnosticAlertID(a), Payload: p, Status: "PENDING"}); err != nil {
				return err
			}
		}
		return nil
	}); err != nil {
		t.Fatal(err)
	}
	if r, err := s.Reconstruct(ctx, "full", b, nextReconstructionCapture(t, b, c, false)); !errors.Is(err, ErrReconstructionBound) || r.SchemaVersion != 0 {
		t.Fatal("queue full did not abort transaction", r, err)
	}
	state, found, err := s.LoadReconstruction(ctx, c.Network, "full")
	if err != nil || !found || state.Report.ProcessedThrough != 100 {
		t.Fatal(state, err)
	}
	last, err := s.LastReconstructionAttempt(ctx, c.Network, "full")
	if err != nil || last.RetainedThrough != 100 || last.ToLedger != 100 {
		t.Fatal("failed enqueue advanced attempt marker", last, err)
	}
}

func TestReconstructionAlertsRejectTamperingAndMissingEvidence(t *testing.T) {
	for _, kind := range []string{"assurance", "finding", "missing"} {
		t.Run(kind, func(t *testing.T) {
			s := openTest(t, filepath.Join(t.TempDir(), "state.db"))
			defer s.Close()
			b, c := reconstructionFixture(t)
			ctx := context.Background()
			if _, err := s.Reconstruct(ctx, "tamper", b, c); err != nil {
				t.Fatal(err)
			}
			source, _ := AlertSource(c.Network, "rpc_ledger_meta", "tamper")
			if err := s.db.Update(func(tx *bolt.Tx) error {
				bucket, err := queue(tx, c.Network, source, false)
				if err != nil {
					return err
				}
				k, raw := bucket.Cursor().First()
				if kind == "missing" {
					return bucket.Delete(k)
				}
				var row Alert
				if err := json.Unmarshal(raw, &row); err != nil {
					return err
				}
				if kind == "assurance" {
					row.Payload.Assurance.FinalVerdict = "APPROVED"
				} else {
					row.Payload.Reconstruction.Event.Finding.Message = "forged"
				}
				return writeAlert(bucket, row)
			}); err != nil {
				t.Fatal(err)
			}
			if _, err := s.ReconstructionAlerts(ctx, c.Network, "tamper"); err == nil {
				t.Fatal("tampered queue listed")
			}
			if _, err := s.Claim(c.Network, source, "sink", time.Unix(1000, 0), time.Minute); err == nil {
				t.Fatal("tampered queue delivered")
			}
		})
	}
}

func TestReconstructionBackfillsPreAlertCheckpoint(t *testing.T) {
	s := openTest(t, filepath.Join(t.TempDir(), "state.db"))
	defer s.Close()
	b, c := reconstructionFixture(t)
	ctx := context.Background()
	if _, err := s.Reconstruct(ctx, "old", b, c); err != nil {
		t.Fatal(err)
	}
	source, _ := AlertSource(c.Network, "rpc_ledger_meta", "old")
	if err := s.db.Update(func(tx *bolt.Tx) error {
		root := tx.Bucket(reconstructionBucket)
		key := reconstructionScope(c.Network, "old")
		var state ReconstructionState
		if err := json.Unmarshal(root.Get(key), &state); err != nil {
			return err
		}
		state.AlertsEnabled = false
		raw, _ := json.Marshal(state)
		if err := root.Put(key, raw); err != nil {
			return err
		}
		if err := tx.Bucket(reconstructionAttemptsBucket).Delete(key); err != nil {
			return err
		}
		return tx.Bucket(outboxBucket).DeleteBucket(scope(c.Network, source))
	}); err != nil {
		t.Fatal(err)
	}
	if rows, err := s.ReconstructionAlerts(ctx, c.Network, "old"); err != nil || len(rows) != 0 {
		t.Fatal("legacy inspection should not initialize delivery", rows, err)
	}
	if _, err := s.Reconstruct(ctx, "old", b, c); err != nil {
		t.Fatal(err)
	}
	if rows, err := s.ReconstructionAlerts(ctx, c.Network, "old"); err != nil || len(rows) != 2 {
		t.Fatal("historical findings not backfilled", rows, err)
	}
}

func TestReconstructionClaimLeaseStartsAfterWriterWait(t *testing.T) {
	s := openTest(t, filepath.Join(t.TempDir(), "state.db"))
	defer s.Close()
	b, c := reconstructionFixture(t)
	if _, err := s.Reconstruct(context.Background(), "wait", b, c); err != nil {
		t.Fatal(err)
	}
	locked, release := make(chan struct{}), make(chan struct{})
	writerDone := make(chan error, 1)
	go func() { writerDone <- s.db.Update(func(*bolt.Tx) error { close(locked); <-release; return nil }) }()
	<-locked
	source, _ := AlertSource(c.Network, "rpc_ledger_meta", "wait")
	result := make(chan *Alert, 1)
	claimErr := make(chan error, 1)
	go func() {
		a, err := s.Claim(c.Network, source, "sink", time.Now(), 2*time.Second)
		result <- a
		claimErr <- err
	}()
	timer := time.NewTimer(2100 * time.Millisecond)
	<-timer.C
	close(release)
	if err := <-writerDone; err != nil {
		t.Fatal(err)
	}
	a, err := <-result, <-claimErr
	if err != nil || a == nil {
		t.Fatal(a, err)
	}
	if err := s.Resolve(c.Network, source, a.ID, a.LeaseToken, true, false, time.Now()); err != nil {
		t.Fatal("lease expired before caller could acknowledge", err)
	}
}
