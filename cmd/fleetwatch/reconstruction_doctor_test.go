package main

import (
	"bytes"
	"encoding/json"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/soroban-fleetwatch/fleetwatch/internal/history"
	"github.com/soroban-fleetwatch/fleetwatch/internal/store"
)

func TestCLIReconstructionDoctorHealthAndExitPrecedence(t *testing.T) {
	path := filepath.Join(t.TempDir(), "state.db")
	var out, errs bytes.Buffer
	args := []string{"doctor", "--state", path, "--network", "SYNTHETIC-NETWORK", "--source", "rpc_ledger_meta", "--stream", "health"}
	check := func(want int) store.ReconstructionHealth {
		t.Helper()
		out.Reset()
		errs.Reset()
		code := run(args, &out, &errs)
		if code != want {
			t.Fatalf("got %d want %d: %s %s", code, want, out.String(), errs.String())
		}
		var h store.ReconstructionHealth
		if want == 5 {
			if out.Len() != 0 {
				t.Fatal("success output on failure")
			}
		} else if err := json.Unmarshal(out.Bytes(), &h); err != nil {
			t.Fatal(err)
		}
		return h
	}
	check(5)
	if _, err := os.Stat(path); !os.IsNotExist(err) {
		t.Fatal("doctor created DB", err)
	}
	reconstructArgs := []string{"reconstruct", "--baseline", fixture("reconstruction-baseline.json"), "--capture", fixture("reconstruction-capture.json"), "--state", path, "--stream", "health"}
	if code := run(reconstructArgs, &out, &errs); code != 2 {
		t.Fatal(code, errs.String())
	}
	before, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	h := check(0)
	if h.Status != "OK_OFFLINE" || h.Events != 2 || h.Outbox.Pending != 2 || h.Assurance.FinalVerdict != "UNDETERMINED" {
		t.Fatal(h)
	}
	args[len(args)-1] = "missing"
	check(4)
	args[len(args)-1] = "health"
	after, err := os.ReadFile(path)
	if err != nil || !bytes.Equal(before, after) {
		t.Fatal("inspection changed DB", err)
	}
	if code := run(args, reportFailWriter{}, &errs); code != 5 {
		t.Fatal(code)
	}
	s, err := store.Open(path)
	if err != nil {
		t.Fatal(err)
	}
	source, _ := store.AlertSource(h.Network, "rpc_ledger_meta", "health")
	now := time.Now()
	a, err := s.Claim(h.Network, source, "test-sink", now, time.Minute)
	if err != nil || a == nil {
		t.Fatal(err)
	}
	if err := s.Resolve(h.Network, source, a.ID, a.LeaseToken, false, true, now); err != nil {
		t.Fatal(err)
	}
	s.Close()
	h = check(2)
	if h.Outbox.Dead != 1 {
		t.Fatal(h)
	}
	raw, err := os.ReadFile(fixture("reconstruction-capture.json"))
	if err != nil {
		t.Fatal(err)
	}
	var c history.Capture
	if err := json.Unmarshal(raw, &c); err != nil {
		t.Fatal(err)
	}
	c.ToLedger = 101
	c.SequenceCoverage = "PARTIAL"
	c.Errors = []string{"LEDGER_QUERY_FAILED"}
	partial := filepath.Join(t.TempDir(), "partial.json")
	raw, _ = json.Marshal(c)
	if err := os.WriteFile(partial, raw, 0600); err != nil {
		t.Fatal(err)
	}
	reconstructArgs[4] = partial
	if code := run(reconstructArgs, &out, &errs); code != 4 {
		t.Fatal(code, errs.String())
	}
	h = check(4)
	if !h.StateFound || h.ProcessedThrough != 100 || h.LastAttempt.Status != "PARTIAL" || h.Outbox.Dead != 1 {
		t.Fatal(h)
	}
}
func TestCLIReconstructionDoctorRejectsInvalidScope(t *testing.T) {
	for _, args := range [][]string{
		{"doctor", "--state", "missing.db", "--network", "N", "--source", "rpc_ledger_meta"},
		{"doctor", "--state", "missing.db", "--network", "N", "--stream", "health"},
		{"doctor", "--state", "missing.db", "--network", "N", "--source", "rpc_ledger_meta", "--stream", "../bad"},
		{"doctor", "--state", "missing.db", "--network", "N", "--source", "rpc_ledger_meta", "--stream", "health", "extra"},
	} {
		var out, errs bytes.Buffer
		if code := run(args, &out, &errs); code != 3 || out.Len() != 0 {
			t.Fatal(args, code, out.String(), errs.String())
		}
	}
}
