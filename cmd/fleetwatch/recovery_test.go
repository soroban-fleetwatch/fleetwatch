package main

import (
	"bytes"
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/soroban-fleetwatch/fleetwatch/internal/fleetwatch"
	"github.com/soroban-fleetwatch/fleetwatch/internal/store"
)

func TestRecoveryCLIOutputFailureAndIdempotentReceipt(t *testing.T) {
	path := filepath.Join(t.TempDir(), "state.db")
	j, err := fleetwatch.LoadJournal("../../testdata/replay-transient.json")
	if err != nil {
		t.Fatal(err)
	}
	j.Snapshots = j.Snapshots[:1]
	s, err := store.Open(path)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := s.Replay(j); err != nil {
		t.Fatal(err)
	}
	now := time.Now()
	a, err := s.Claim(j.Network, j.Source, "local-test-sink", now, time.Minute)
	if err != nil || a == nil {
		t.Fatal(err)
	}
	if err := s.Resolve(j.Network, j.Source, a.ID, a.LeaseToken, false, true, now); err != nil {
		t.Fatal(err)
	}
	s.Close()
	args := []string{"requeue", "--state", path, "--network", j.Network, "--alert-id", a.ID, "--request-id", "operator-001", "--expected-cycle", "0", "--reason", "Receiver repaired"}
	var out, stderr bytes.Buffer
	if code := run(args, reportFailWriter{}, &stderr); code != 5 {
		t.Fatal(code, stderr.String())
	}
	if code := run(args, &out, &stderr); code != 0 {
		t.Fatal(code, stderr.String())
	}
	var result store.RecoveryResult
	if err := json.Unmarshal(out.Bytes(), &result); err != nil || result.NewRecovery || result.Receipt.RecoveryCycle != 1 {
		t.Fatal(out.String(), err)
	}
	out.Reset()
	list := []string{"recoveries", "--state", path, "--network", j.Network, "--alert-id", a.ID}
	if code := run(list, &out, &stderr); code != 0 {
		t.Fatal(code, stderr.String())
	}
	var rows []store.RecoveryReceipt
	if err := json.Unmarshal(out.Bytes(), &rows); err != nil || len(rows) != 1 || rows[0] != result.Receipt {
		t.Fatal(out.String(), err)
	}
	if code := run(list, reportFailWriter{}, &stderr); code != 5 {
		t.Fatal(code)
	}
	s, err = store.OpenReadOnly(path)
	if err != nil {
		t.Fatal(err)
	}
	defer s.Close()
	audit, err := s.Recoveries(context.Background(), j.Network, j.Source, a.ID)
	if err != nil || len(audit) != 1 {
		t.Fatal(audit, err)
	}
}
func TestRecoveryCLIRejectsMissingArgumentsAndDatabase(t *testing.T) {
	path := filepath.Join(t.TempDir(), "missing.db")
	var out, stderr bytes.Buffer
	for _, args := range [][]string{{"requeue"}, {"recoveries"}, {"requeue", "--state", path, "--network", "N", "--alert-id", "id", "--request-id", "r", "--reason", "reason"}, {"recoveries", "--state", path, "--network", "N", "--alert-id", "id"}} {
		if code := run(args, &out, &stderr); code != 3 && code != 5 {
			t.Fatal(args, code)
		}
	}
	if _, err := os.Stat(path); !os.IsNotExist(err) {
		t.Fatal("missing state created", err)
	}
}
