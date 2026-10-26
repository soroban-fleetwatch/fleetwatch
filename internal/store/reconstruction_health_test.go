package store

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	bolt "go.etcd.io/bbolt"
)

func TestReconstructionHealthReadOnlyAndScope(t *testing.T) {
	ctx := context.Background()
	path := filepath.Join(t.TempDir(), "state.db")
	s := openTest(t, path)
	b, c := reconstructionFixture(t)
	if _, err := s.Reconstruct(ctx, "health", b, c); err != nil {
		t.Fatal(err)
	}
	s.Close()
	before, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	s, err = OpenReadOnly(path)
	if err != nil {
		t.Fatal(err)
	}
	h, err := s.ReconstructionHealth(ctx, c.Network, "health", time.Unix(1000, 0))
	if err != nil || h.Status != "OK_OFFLINE" || !h.StateFound || h.ProcessedThrough != 100 || h.RetainedLedgers != 1 || h.Events != 2 || h.Diagnostics != 0 || h.Outbox.Pending != 2 || h.Outbox.Due != 2 || h.LastAttempt == nil || h.LastAttempt.Status != "COMPLETE" || !h.OutboxIntegrated || h.Assurance != projectionAssurance() || h.LedgerLag != nil || h.SourceFreshness != "UNKNOWN" || h.Coverage != "RETAINED_PREFIX_ONLY" {
		t.Fatal(h, err)
	}
	for _, scope := range [][2]string{{c.Network, "other"}, {"other-network", "health"}} {
		h, err = s.ReconstructionHealth(ctx, scope[0], scope[1], time.Unix(1000, 0))
		if err != nil || h.Status != "NO_STATE" || h.StateFound || h.LastAttempt != nil || h.Outbox.Total != 0 || len(h.Reasons) != 1 || h.Reasons[0] != "NO_RECONSTRUCTION_STATE" {
			t.Fatal(h, err)
		}
	}
	s.Close()
	after, err := os.ReadFile(path)
	if err != nil || !bytes.Equal(before, after) {
		t.Fatal("health changed database", err)
	}
}
func TestReconstructionHealthPartialAttemptRepairAndHistoricalDeadLetter(t *testing.T) {
	s := openTest(t, filepath.Join(t.TempDir(), "state.db"))
	defer s.Close()
	ctx := context.Background()
	b, c := reconstructionFixture(t)
	next := nextReconstructionCapture(t, b, c, false)
	now := time.Unix(1000, 0)
	// A failed first attempt has a diagnostic but no accepted checkpoint.
	if _, err := s.Reconstruct(ctx, "health", b, next); !errors.Is(err, ErrReconstructionIncomplete) {
		t.Fatal(err)
	}
	h, err := s.ReconstructionHealth(ctx, c.Network, "health", now)
	if err != nil || h.StateFound || h.Status != "NEEDS_ATTENTION" || h.ProcessedThrough != 99 || h.Diagnostics != 1 || h.Outbox.Total != 1 || h.LastAttempt.Status != "PARTIAL" {
		t.Fatal(h, err)
	}
	source, _ := AlertSource(c.Network, "rpc_ledger_meta", "health")
	a := killAlert(t, s, c.Network, source, now)
	if a.Payload.Diagnostic == nil {
		t.Fatal("expected diagnostic")
	}
	if _, err := s.Reconstruct(ctx, "health", b, c); err != nil {
		t.Fatal(err)
	}
	h, err = s.ReconstructionHealth(ctx, c.Network, "health", now)
	if err != nil || !h.StateFound || h.Status != "NEEDS_ATTENTION" || h.LastAttempt.Status != "COMPLETE" || h.Diagnostics != 1 || h.Outbox.Dead != 1 || len(h.Reasons) != 1 || h.Reasons[0] != "DELIVERY_DEAD_LETTERS" {
		t.Fatal(h, err)
	}
	if _, err := s.Requeue(ctx, c.Network, source, RecoveryRequest{a.ID, "health-repair", 0, "Recovered receiver"}, now); err != nil {
		t.Fatal(err)
	}
	h, err = s.ReconstructionHealth(ctx, c.Network, "health", now)
	if err != nil || h.Status != "OK_OFFLINE" || h.RecoveredAlerts != 1 || h.RecoveryReceipts != 1 || h.Outbox.Pending != 3 {
		t.Fatal(h, err)
	}
	raw, _ := json.Marshal(h)
	for _, secret := range []string{a.LeaseToken, "original-sink", "Recovered receiver", "health-repair"} {
		if strings.Contains(string(raw), secret) {
			t.Fatalf("health exposed delivery audit details: %s", secret)
		}
	}
	// A later attempted extension can fail while the retained prefix is still valid.
	partial := next
	partial.ToLedger = 102
	partial.SequenceCoverage = "PARTIAL"
	partial.Errors = []string{"LEDGER_QUERY_FAILED"}
	if _, err := s.Reconstruct(ctx, "health", b, partial); !errors.Is(err, ErrReconstructionIncomplete) {
		t.Fatal(err)
	}
	h, err = s.ReconstructionHealth(ctx, c.Network, "health", now)
	if err != nil || h.Status != "NEEDS_ATTENTION" || h.ProcessedThrough != 101 || h.RetainedLedgers != 2 || h.LastAttempt.Status != "PARTIAL" || h.Reasons[0] != "LATEST_ATTEMPT_INCOMPLETE" {
		t.Fatal(h, err)
	}
}
func TestReconstructionHealthDeliveryBoundaries(t *testing.T) {
	s, n, source := recoveryFixture(t, true)
	ctx := context.Background()
	now := time.Unix(1000, 0)
	for i := 0; i < MaxDeliveryAttempts; i++ {
		if a, err := s.Claim(n, source, "sink", now.Add(time.Duration(i)*time.Minute), time.Minute); err != nil || a == nil {
			t.Fatal(err)
		}
	}
	h, err := s.ReconstructionHealth(ctx, n, "recovery", now.Add(4*time.Minute))
	if err != nil || h.Status != "OK_OFFLINE" || h.Outbox.Inflight != 1 || h.Outbox.ExhaustedUnsettled != 0 || h.Outbox.ExpiredLeases != 0 {
		t.Fatal(h, err)
	}
	h, err = s.ReconstructionHealth(ctx, n, "recovery", now.Add(5*time.Minute))
	if err != nil || h.Status != "NEEDS_ATTENTION" || h.Outbox.Inflight != 1 || h.Outbox.ExhaustedUnsettled != 1 || h.Outbox.ExpiredLeases != 1 || len(h.Reasons) != 2 {
		t.Fatal(h, err)
	}
}
func TestReconstructionHealthLegacyCheckpointNeedsBackfill(t *testing.T) {
	s, n, source := recoveryFixture(t, true)
	if err := s.db.Update(func(tx *bolt.Tx) error {
		b := tx.Bucket(reconstructionBucket)
		key := reconstructionScope(n, "recovery")
		var st ReconstructionState
		if err := json.Unmarshal(b.Get(key), &st); err != nil {
			return err
		}
		st.AlertsEnabled = false
		raw, _ := json.Marshal(st)
		if err := b.Put(key, raw); err != nil {
			return err
		}
		if err := tx.Bucket(reconstructionAttemptsBucket).Delete(key); err != nil {
			return err
		}
		return tx.Bucket(outboxBucket).DeleteBucket(scope(n, source))
	}); err != nil {
		t.Fatal(err)
	}
	h, err := s.ReconstructionHealth(context.Background(), n, "recovery", time.Unix(1000, 0))
	if err != nil || h.Status != "NEEDS_ATTENTION" || !h.StateFound || h.OutboxIntegrated || h.Events != 2 || h.Outbox.Total != 0 || h.LastAttempt != nil || len(h.Reasons) != 2 {
		t.Fatal(h, err)
	}
}
func TestReconstructionHealthRejectsInconsistentState(t *testing.T) {
	for _, kind := range []string{"missing-finding", "missing-attempt", "corrupt-evidence", "missing-audit", "orphan-audit", "duplicate-audit-json", "audit-sink"} {
		t.Run(kind, func(t *testing.T) {
			s, n, source := recoveryFixture(t, true)
			ctx := context.Background()
			now := time.Unix(1000, 0)
			a := killAlert(t, s, n, source, now)
			r, err := s.Requeue(ctx, n, source, RecoveryRequest{a.ID, "health-repair", 0, "Receiver repaired"}, now)
			if err != nil {
				t.Fatal(err)
			}
			if err := s.db.Update(func(tx *bolt.Tx) error {
				switch kind {
				case "missing-finding":
					b, _ := queue(tx, n, source, false)
					return b.Delete([]byte(a.ID))
				case "missing-attempt":
					return tx.Bucket(reconstructionAttemptsBucket).Delete(reconstructionScope(n, "recovery"))
				case "corrupt-evidence":
					return tx.Bucket(reconstructionBucket).Put(reconstructionScope(n, "recovery"), []byte(`{}`))
				case "missing-audit":
					b, _ := recoveryQueue(tx, n, source, false)
					return b.Delete([]byte("health-repair"))
				case "orphan-audit":
					b, _ := recoveryQueue(tx, n, source, false)
					r.Receipt.AlertID = strings.Repeat("0", 64)
					raw, _ := json.Marshal(r.Receipt)
					return b.Put([]byte("health-repair"), raw)
				case "duplicate-audit-json":
					b, _ := recoveryQueue(tx, n, source, false)
					raw := append([]byte(`{"schema_version":1,`), b.Get([]byte("health-repair"))[1:]...)
					return b.Put([]byte("health-repair"), raw)
				case "audit-sink":
					b, _ := recoveryQueue(tx, n, source, false)
					r.Receipt.Sink = "changed-sink"
					raw, _ := json.Marshal(r.Receipt)
					return b.Put([]byte("health-repair"), raw)
				}
				return nil
			}); err != nil {
				t.Fatal(err)
			}
			h, err := s.ReconstructionHealth(ctx, n, "recovery", now)
			if err == nil || h.SchemaVersion != 0 || h.Status != "" {
				t.Fatal("partial success report after corruption", h, err)
			}
		})
	}
}
func TestReconstructionHealthRejectsClockScopeAndCancellation(t *testing.T) {
	s, n, _ := recoveryFixture(t, true)
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	if _, err := s.ReconstructionHealth(ctx, n, "recovery", time.Unix(1000, 0)); !errors.Is(err, context.Canceled) {
		t.Fatal(err)
	}
	for _, scope := range [][2]string{{n, ""}, {n, "../bad"}, {"", "health"}} {
		if _, err := s.ReconstructionHealth(context.Background(), scope[0], scope[1], time.Unix(1000, 0)); err == nil {
			t.Fatal(scope)
		}
	}
	if _, err := s.ReconstructionHealth(context.Background(), n, "recovery", time.Time{}); err == nil {
		t.Fatal("invalid clock")
	}
}
