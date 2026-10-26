package main

import (
	"bytes"
	"encoding/json"
	"os"
	"path/filepath"
	"testing"

	"github.com/soroban-fleetwatch/fleetwatch/internal/history"
	"github.com/soroban-fleetwatch/fleetwatch/internal/store"
)

func TestCLIReconstructionAlertsAndLatestAttempt(t *testing.T) {
	path := filepath.Join(t.TempDir(), "state.db")
	var out, errs bytes.Buffer
	args := []string{"reconstruct", "--baseline", fixture("reconstruction-baseline.json"), "--capture", fixture("reconstruction-capture.json"), "--state", path, "--stream", "notify"}
	if code := run(args, &out, &errs); code != 2 {
		t.Fatal(code, errs.String())
	}
	out.Reset()
	list := []string{"alerts", "--state", path, "--network", "SYNTHETIC-NETWORK", "--source", "rpc_ledger_meta", "--stream", "notify"}
	before, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	if code := run(list, &out, &errs); code != 0 {
		t.Fatal(code, errs.String())
	}
	var rows []store.Alert
	if err := json.Unmarshal(out.Bytes(), &rows); err != nil || len(rows) != 2 || rows[0].Payload.SchemaVersion != 2 {
		t.Fatal(rows, err)
	}
	after, err := os.ReadFile(path)
	if err != nil || !bytes.Equal(before, after) {
		t.Fatal("alert listing changed database", err)
	}
	raw, err := os.ReadFile(fixture("reconstruction-capture.json"))
	if err != nil {
		t.Fatal(err)
	}
	var c history.Capture
	_ = json.Unmarshal(raw, &c)
	c.ToLedger = 101
	c.SequenceCoverage = "PARTIAL"
	c.Errors = []string{"LEDGER_QUERY_FAILED"}
	f := filepath.Join(t.TempDir(), "partial.json")
	encoded, _ := json.Marshal(c)
	if err := os.WriteFile(f, encoded, 0600); err != nil {
		t.Fatal(err)
	}
	args[4] = f
	out.Reset()
	if code := run(args, &out, &errs); code != 4 {
		t.Fatal(code, errs.String())
	}
	out.Reset()
	if code := run(list, &out, &errs); code != 0 {
		t.Fatal(code)
	}
	if err := json.Unmarshal(out.Bytes(), &rows); err != nil || len(rows) != 3 {
		t.Fatal(rows, err)
	}
	out.Reset()
	if code := run([]string{"checkpoint", "--state", path, "--network", c.Network, "--stream", "notify"}, &out, &errs); code != 0 {
		t.Fatal(code)
	}
	var view checkpointView
	if err := json.Unmarshal(out.Bytes(), &view); err != nil || view.LastAttempt == nil || view.LastAttempt.Status != "PARTIAL" || view.Report.ProcessedThrough != 100 {
		t.Fatal(view, err)
	}
	for _, args := range [][]string{{"alerts", "--state", path, "--network", c.Network, "--source", "rpc_ledger_meta"}, {"alerts", "--state", path, "--network", c.Network, "--stream", "notify"}, {"deliver", "--state", path, "--network", c.Network, "--source", "rpc_ledger_meta", "--webhook-url", "https://hooks.example.com", "--allow-host", "hooks.example.com"}} {
		out.Reset()
		if code := run(args, &out, &errs); code != 3 || out.Len() != 0 {
			t.Fatal(code, out.String())
		}
	}
}
