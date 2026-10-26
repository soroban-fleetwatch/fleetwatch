package store

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"sync"
	"testing"
	"time"

	bolt "go.etcd.io/bbolt"
)

func recoveryFixture(t *testing.T, reconstruction bool) (*Store, string, string) {
	t.Helper()
	s := openTest(t, filepath.Join(t.TempDir(), "state.db"))
	t.Cleanup(func() { s.Close() })
	if reconstruction {
		b, c := reconstructionFixture(t)
		if _, err := s.Reconstruct(context.Background(), "recovery", b, c); err != nil {
			t.Fatal(err)
		}
		source, _ := AlertSource(c.Network, "rpc_ledger_meta", "recovery")
		return s, c.Network, source
	}
	j := fixture(t)
	j.Snapshots = j.Snapshots[:1]
	if _, err := s.Replay(j); err != nil {
		t.Fatal(err)
	}
	return s, j.Network, j.Source
}
func killAlert(t *testing.T, s *Store, n, source string, now time.Time) Alert {
	t.Helper()
	a, err := s.Claim(n, source, "original-sink", now, time.Minute)
	if err != nil || a == nil {
		t.Fatal(a, err)
	}
	if err := s.Resolve(n, source, a.ID, a.LeaseToken, false, true, now); err != nil {
		t.Fatal(err)
	}
	return *a
}
func TestRecoveryBothSourcesPreserveIdentityAndAudit(t *testing.T) {
	for _, reconstruction := range []bool{false, true} {
		t.Run(fmt.Sprint(reconstruction), func(t *testing.T) {
			s, n, source := recoveryFixture(t, reconstruction)
			now := time.Unix(1000, 0)
			a := killAlert(t, s, n, source, now)
			r := RecoveryRequest{a.ID, "repair-001", 0, "Corrected delivery configuration"}
			result, err := s.Requeue(context.Background(), n, source, r, now)
			if err != nil || !result.NewRecovery || result.Receipt.PreviousAttempts != 1 || result.Receipt.Sink != a.Sink {
				t.Fatal(result, err)
			}
			again, err := s.Requeue(context.Background(), n, source, r, now.Add(time.Hour))
			if err != nil || again.NewRecovery || again.Receipt != result.Receipt {
				t.Fatal(again, err)
			}
			claimed, err := s.Claim(n, source, a.Sink, now, time.Minute)
			if err != nil || claimed == nil || claimed.ID != a.ID || claimed.RecoveryCycle != 1 || claimed.Attempts != 1 || payloadDigest(*claimed) != payloadDigest(a) || claimed.LeaseToken == a.LeaseToken {
				t.Fatal(claimed, err)
			}
			if err := s.Resolve(n, source, a.ID, a.LeaseToken, true, false, now); !errors.Is(err, ErrStaleLease) {
				t.Fatal("stale lease accepted", err)
			}
			if err := s.Resolve(n, source, a.ID, claimed.LeaseToken, true, false, now); err != nil {
				t.Fatal(err)
			}
			again, err = s.Requeue(context.Background(), n, source, r, now)
			if err != nil || again.NewRecovery {
				t.Fatal(again, err)
			}
			rows, err := s.Recoveries(context.Background(), n, source, a.ID)
			if err != nil || len(rows) != 1 || rows[0] != result.Receipt {
				t.Fatal(rows, err)
			}
			r.Reason = "changed"
			if _, err := s.Requeue(context.Background(), n, source, r, now); err == nil {
				t.Fatal("request ID reused")
			}
			r.RequestID = "another"
			r.ExpectedCycle = 1
			if _, err := s.Requeue(context.Background(), n, source, r, now); err == nil {
				t.Fatal("delivered alert requeued")
			}
		})
	}
}
func TestRecoveryRefusesActiveAndExhaustedLeaseIsFenced(t *testing.T) {
	s, n, source := recoveryFixture(t, false)
	ctx := context.Background()
	now := time.Unix(1000, 0)
	rows, _ := s.Alerts(n, source)
	r := RecoveryRequest{rows[0].ID, "repair", 0, "Retry exhausted delivery"}
	if _, err := s.Requeue(ctx, n, source, r, now); err == nil {
		t.Fatal("pending accepted")
	}
	var a *Alert
	for i := 0; i < MaxDeliveryAttempts; i++ {
		var err error
		a, err = s.Claim(n, source, "original-sink", now.Add(time.Duration(i)*time.Minute), time.Minute)
		if err != nil || a == nil {
			t.Fatal(err)
		}
	}
	if _, err := s.Requeue(ctx, n, source, r, now.Add(4*time.Minute)); err == nil {
		t.Fatal("active exhausted lease accepted")
	}
	result, err := s.Requeue(ctx, n, source, r, now.Add(5*time.Minute))
	if err != nil || result.Receipt.PreviousStatus != "INFLIGHT" || result.Receipt.PreviousAttempts != 5 {
		t.Fatal(result, err)
	}
	if err := s.Resolve(n, source, a.ID, a.LeaseToken, true, false, now.Add(5*time.Minute)); !errors.Is(err, ErrStaleLease) {
		t.Fatal(err)
	}
	if _, err := s.Claim(n, source, "different-sink", now.Add(5*time.Minute), time.Minute); err == nil {
		t.Fatal("recovery retargeted sink")
	}
}
func TestRecoveryCycleBoundAndConcurrentFence(t *testing.T) {
	s, n, source := recoveryFixture(t, false)
	now := time.Unix(1000, 0)
	ctx := context.Background()
	a := killAlert(t, s, n, source, now)
	var wg sync.WaitGroup
	results := make(chan error, 2)
	for _, id := range []string{"a", "b"} {
		wg.Add(1)
		go func(id string) {
			defer wg.Done()
			_, err := s.Requeue(ctx, n, source, RecoveryRequest{a.ID, id, 0, "Concurrent recovery"}, now)
			results <- err
		}(id)
	}
	wg.Wait()
	close(results)
	wins := 0
	for err := range results {
		if err == nil {
			wins++
		}
	}
	if wins != 1 {
		t.Fatal(wins)
	}
	for cycle := 1; cycle < MaxRecoveryCycles; cycle++ {
		killAlert(t, s, n, source, now)
		_, err := s.Requeue(ctx, n, source, RecoveryRequest{a.ID, fmt.Sprint("repair-", cycle), cycle, "Receiver repaired"}, now)
		if err != nil {
			t.Fatal(err)
		}
	}
	killAlert(t, s, n, source, now)
	if _, err := s.Requeue(ctx, n, source, RecoveryRequest{a.ID, "over-limit", 10, "Retry"}, now); err == nil {
		t.Fatal("cycle bound bypassed")
	}
	audit, err := s.Recoveries(ctx, n, source, a.ID)
	if err != nil || len(audit) != 10 {
		t.Fatal(len(audit), err)
	}
}
func TestRecoveryRollbackCancellationAndCorruptAudit(t *testing.T) {
	s, n, source := recoveryFixture(t, false)
	now := time.Unix(1000, 0)
	a := killAlert(t, s, n, source, now)
	r := RecoveryRequest{a.ID, "repair", 0, "Receiver repaired"}
	ctx := context.Background()
	injected := errors.New("abort")
	if _, err := s.requeue(ctx, n, source, r, now, func() error { return injected }); !errors.Is(err, injected) {
		t.Fatal(err)
	}
	rows, _ := s.Alerts(n, source)
	audit, err := s.Recoveries(ctx, n, source, a.ID)
	if err != nil || len(audit) != 0 || rows[0].Status != "DEAD" || rows[0].RecoveryCycle != 0 {
		t.Fatal(rows, audit, err)
	}
	cancelled, cancel := context.WithCancel(ctx)
	cancel()
	if _, err := s.Requeue(cancelled, n, source, r, now); !errors.Is(err, context.Canceled) {
		t.Fatal(err)
	}
	result, err := s.Requeue(ctx, n, source, r, now)
	if err != nil {
		t.Fatal(err)
	}
	if err := s.db.Update(func(tx *bolt.Tx) error {
		b, _ := recoveryQueue(tx, n, source, false)
		result.Receipt.PayloadSHA256 = fmt.Sprintf("%064d", 0)
		raw, _ := json.Marshal(result.Receipt)
		return b.Put([]byte(r.RequestID), raw)
	}); err != nil {
		t.Fatal(err)
	}
	if _, err := s.Recoveries(ctx, n, source, a.ID); err == nil {
		t.Fatal("tampered audit accepted")
	}
	if _, err := s.Requeue(ctx, n, source, r, now); err == nil {
		t.Fatal("tampered idempotency receipt accepted")
	}
}
func TestRecoveryInvalidRequestsAndScope(t *testing.T) {
	s, n, source := recoveryFixture(t, false)
	now := time.Unix(1000, 0)
	a := killAlert(t, s, n, source, now)
	r := RecoveryRequest{a.ID, "repair", 0, "Repair"}
	cases := []RecoveryRequest{r, r, r, r, r, r}
	cases[0].RequestID = "../bad"
	cases[1].Reason = "\nsecret"
	cases[2].Reason = " "
	cases[3].ExpectedCycle = -1
	cases[4].AlertID = "bad"
	cases[5].Reason = string([]byte{255})
	for _, bad := range cases {
		if _, err := s.Requeue(context.Background(), n, source, bad, now); err == nil {
			t.Fatal(bad)
		}
	}
	if _, err := s.Requeue(context.Background(), "other network", source, r, now); err == nil {
		t.Fatal("scope crossed")
	}
	if _, err := s.Requeue(context.Background(), n, source, r, time.Time{}); err == nil {
		t.Fatal("invalid clock")
	}
}
func TestRecoveryCrashBeforeCommit(t *testing.T) {
	if path := os.Getenv("FLEETWATCH_TEST_RECOVERY_CRASH_DB"); path != "" {
		s, err := Open(path)
		if err != nil {
			t.Fatal(err)
		}
		j := fixture(t)
		rows, err := s.Alerts(j.Network, j.Source)
		if err != nil {
			t.Fatal(err)
		}
		_, err = s.requeue(context.Background(), j.Network, j.Source, RecoveryRequest{rows[0].ID, "crash", 0, "Crash test"}, time.Unix(1000, 0), func() error { os.Exit(23); return nil })
		t.Fatal("crash hook not reached", err)
	}
	path := filepath.Join(t.TempDir(), "state.db")
	s := openTest(t, path)
	j := fixture(t)
	j.Snapshots = j.Snapshots[:1]
	if _, err := s.Replay(j); err != nil {
		t.Fatal(err)
	}
	a := killAlert(t, s, j.Network, j.Source, time.Unix(1000, 0))
	s.Close()
	cmd := exec.Command(os.Args[0], "-test.run=^TestRecoveryCrashBeforeCommit$")
	cmd.Env = append(os.Environ(), "FLEETWATCH_TEST_RECOVERY_CRASH_DB="+path)
	output, err := cmd.CombinedOutput()
	if exit, ok := err.(*exec.ExitError); !ok || exit.ExitCode() != 23 {
		t.Fatalf("%v %s", err, output)
	}
	s = openTest(t, path)
	defer s.Close()
	rows, err := s.Alerts(j.Network, j.Source)
	if err != nil || rows[0].Status != "DEAD" || rows[0].RecoveryCycle != 0 {
		t.Fatal(rows, err)
	}
	audit, err := s.Recoveries(context.Background(), j.Network, j.Source, a.ID)
	if err != nil || len(audit) != 0 {
		t.Fatal(audit, err)
	}
}

func TestRecoveryBackupAndMissingAudit(t *testing.T) {
	s, n, source := recoveryFixture(t, false)
	now := time.Unix(1000, 0)
	a := killAlert(t, s, n, source, now)
	r := RecoveryRequest{a.ID, "repair", 0, "Receiver repaired"}
	ctx := context.Background()
	result, err := s.Requeue(ctx, n, source, r, now)
	if err != nil {
		t.Fatal(err)
	}
	path := filepath.Join(t.TempDir(), "restored.db")
	if _, err := s.Backup(ctx, path); err != nil {
		t.Fatal(err)
	}
	restored := openTest(t, path)
	defer restored.Close()
	again, err := restored.Requeue(ctx, n, source, r, now)
	if err != nil || again.NewRecovery || again.Receipt != result.Receipt {
		t.Fatal(again, err)
	}
	if err := restored.db.Update(func(tx *bolt.Tx) error {
		b, _ := recoveryQueue(tx, n, source, false)
		return b.Delete([]byte(r.RequestID))
	}); err != nil {
		t.Fatal(err)
	}
	if _, err := restored.Recoveries(ctx, n, source, a.ID); err == nil {
		t.Fatal("missing audit accepted")
	}
}

func TestRecoveryAuditQuotaRollsBack(t *testing.T) {
	s, n, source := recoveryFixture(t, false)
	now := time.Unix(1000, 0)
	a := killAlert(t, s, n, source, now)
	ctx := context.Background()
	if err := s.db.Update(func(tx *bolt.Tx) error {
		b, err := recoveryQueue(tx, n, source, true)
		if err != nil {
			return err
		}
		for i := 0; i < maxRecoveryReceipts; i++ {
			id := fmt.Sprintf("receipt-%05d", i)
			r := RecoveryReceipt{SchemaVersion: 1, RecoveryRequest: RecoveryRequest{fmt.Sprintf("%064x", i+1), id, 0, "Quota test"}, Network: n, Source: source, PayloadSHA256: payloadDigest(a), PreviousStatus: "DEAD", PreviousAttempts: 1, Sink: a.Sink, RecoveryCycle: 1, RecoveredAt: now.UTC().Format(time.RFC3339Nano)}
			if r.AlertID == a.ID {
				t.Fatal("fixture identity collision")
			}
			raw, _ := json.Marshal(r)
			if err := b.Put([]byte(id), raw); err != nil {
				return err
			}
		}
		return nil
	}); err != nil {
		t.Fatal(err)
	}
	if _, err := s.Requeue(ctx, n, source, RecoveryRequest{a.ID, "over-quota", 0, "Retry"}, now); err == nil {
		t.Fatal("quota bypassed")
	}
	rows, err := s.Alerts(n, source)
	if err != nil || rows[0].Status != "DEAD" || rows[0].RecoveryCycle != 0 {
		t.Fatal(rows, err)
	}
}

func TestRecoverySnapshotFindingRevalidatesEvidence(t *testing.T) {
	s := openTest(t, filepath.Join(t.TempDir(), "state.db"))
	defer s.Close()
	j := fixture(t)
	if _, err := s.Replay(j); err != nil {
		t.Fatal(err)
	}
	now := time.Unix(1000, 0)
	a := killAlert(t, s, j.Network, j.Source, now)
	if a.Payload.Event == nil {
		t.Fatal("expected snapshot finding")
	}
	request := RecoveryRequest{a.ID, "finding-repair", 0, "Receiver repaired"}
	if _, err := s.Requeue(context.Background(), j.Network, j.Source, request, now); err != nil {
		t.Fatal(err)
	}
	if err := s.db.Update(func(tx *bolt.Tx) error {
		b, _ := queue(tx, j.Network, j.Source, false)
		var row Alert
		if err := json.Unmarshal(b.Get([]byte(a.ID)), &row); err != nil {
			return err
		}
		row.Payload.Event.Finding.Severity = "info"
		return writeAlert(b, row)
	}); err != nil {
		t.Fatal(err)
	}
	if _, err := s.Requeue(context.Background(), j.Network, j.Source, request, now); err == nil {
		t.Fatal("changed finding accepted")
	}
}

func TestRecoveryHistoricalReconstructionDiagnostic(t *testing.T) {
	s := openTest(t, filepath.Join(t.TempDir(), "state.db"))
	defer s.Close()
	b, c := reconstructionFixture(t)
	ctx := context.Background()
	if _, err := s.Reconstruct(ctx, "gap-recovery", b, nextReconstructionCapture(t, b, c, false)); !errors.Is(err, ErrReconstructionIncomplete) {
		t.Fatal(err)
	}
	source, _ := AlertSource(c.Network, "rpc_ledger_meta", "gap-recovery")
	now := time.Unix(1000, 0)
	a := killAlert(t, s, c.Network, source, now)
	if a.Payload.Diagnostic == nil {
		t.Fatal("expected reconstruction diagnostic")
	}
	// Repairing the source does not erase a historical failed delivery.
	if _, err := s.Reconstruct(ctx, "gap-recovery", b, c); err != nil {
		t.Fatal(err)
	}
	result, err := s.Requeue(ctx, c.Network, source, RecoveryRequest{a.ID, "gap-repair", 0, "Receiver repaired after source gap"}, now)
	if err != nil || !result.NewRecovery || result.Receipt.PayloadSHA256 != payloadDigest(a) {
		t.Fatal(result, err)
	}
}
