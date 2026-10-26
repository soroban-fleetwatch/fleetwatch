package main

import (
	"bytes"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/soroban-fleetwatch/fleetwatch/internal/history"
)

func TestCLIDurableReconstructionAndReadOnlyCheckpoint(t *testing.T) {
	path := filepath.Join(t.TempDir(), "state.db")
	base := []string{"reconstruct", "--baseline", fixture("reconstruction-baseline.json"), "--capture", fixture("reconstruction-capture.json"), "--state", path, "--stream", "operator"}
	var out, errs bytes.Buffer
	if code := run(base, &out, &errs); code != 2 {
		t.Fatal(code, errs.String())
	}
	var initial history.Reconstruction
	if err := json.Unmarshal(out.Bytes(), &initial); err != nil {
		t.Fatal(err)
	}
	base[4] = fixture("reconstruction-next-capture.json")
	out.Reset()
	if code := run(base, &out, &errs); code != 2 {
		t.Fatal(code, errs.String())
	}
	var complete history.Reconstruction
	if err := json.Unmarshal(out.Bytes(), &complete); err != nil || complete.ProcessedThrough != 101 || len(complete.Ledgers) != 2 {
		t.Fatal(complete, err)
	}
	if complete.Ledgers[0].Events[0].ID != initial.Ledgers[0].Events[0].ID {
		t.Fatal("prior finding changed across resume")
	}
	before, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	out.Reset()
	inspect := []string{"checkpoint", "--state", path, "--network", initial.Network, "--stream", "operator"}
	if code := run(inspect, &out, &errs); code != 0 {
		t.Fatal(code, errs.String())
	}
	var v checkpointView
	if err := json.Unmarshal(out.Bytes(), &v); err != nil || !v.Found || v.Report == nil || v.Report.ProcessedThrough != 101 || v.Coverage != "RETAINED_PREFIX_ONLY" || v.Report.FinalVerdict != "UNDETERMINED" {
		t.Fatal(v, err)
	}
	after, err := os.ReadFile(path)
	if err != nil || !bytes.Equal(before, after) {
		t.Fatal("checkpoint inspection changed database", err)
	}
	inspect[len(inspect)-1] = "missing-stream"
	out.Reset()
	if code := run(inspect, &out, &errs); code != 4 {
		t.Fatal(code)
	}
	if err := json.Unmarshal(out.Bytes(), &v); err != nil || v.Found || v.Report != nil {
		t.Fatal(v, err)
	}
	// Receipt/report output can fail after a requested durable commit.
	if code := run(base, reportFailWriter{}, &errs); code != 5 {
		t.Fatal("write failure exit", code)
	}
	inspect[len(inspect)-1] = "operator"
	out.Reset()
	if code := run(inspect, &out, &errs); code != 0 {
		t.Fatal(code)
	}
	for _, args := range [][]string{{"checkpoint"}, {"checkpoint", "--state", path, "--network", initial.Network, "--stream", "bad/id"}, {"reconstruct", "--baseline", fixture("reconstruction-baseline.json"), "--capture", fixture("reconstruction-capture.json"), "--state", path}, {"reconstruct", "--baseline", fixture("reconstruction-baseline.json"), "--capture", fixture("reconstruction-capture.json"), "--stream", "unused"}} {
		out.Reset()
		if code := run(args, &out, &errs); code != 3 || out.Len() != 0 {
			t.Fatal(code, out.String())
		}
	}
	missing := filepath.Join(t.TempDir(), "missing.db")
	inspect[2] = missing
	out.Reset()
	if code := run(inspect, &out, &errs); code != 5 || out.Len() != 0 {
		t.Fatal(code)
	}
	if _, err := os.Stat(missing); !os.IsNotExist(err) {
		t.Fatal("checkpoint created a missing database")
	}
}

func TestCLICheckpointPartialAttemptRetainsOnlyValidPrefix(t *testing.T) {
	path := filepath.Join(t.TempDir(), "state.db")
	var out, errs bytes.Buffer
	base := []string{"reconstruct", "--baseline", fixture("reconstruction-baseline.json"), "--capture", fixture("reconstruction-capture.json"), "--state", path, "--stream", "partial"}
	if code := run(base, &out, &errs); code != 2 {
		t.Fatal(code)
	}
	raw, err := os.ReadFile(fixture("reconstruction-capture.json"))
	if err != nil {
		t.Fatal(err)
	}
	var capture history.Capture
	if err := json.Unmarshal(raw, &capture); err != nil {
		t.Fatal(err)
	}
	capture.ToLedger = 101
	capture.SequenceCoverage = "PARTIAL"
	capture.Errors = []string{"LEDGER_QUERY_FAILED"}
	f := filepath.Join(t.TempDir(), "partial.json")
	encoded, _ := json.Marshal(capture)
	if err := os.WriteFile(f, encoded, 0600); err != nil {
		t.Fatal(err)
	}
	base[4] = f
	out.Reset()
	if code := run(base, &out, &errs); code != 4 || !strings.Contains(out.String(), `"status": "PARTIAL"`) {
		t.Fatal(code, out.String())
	}
	out.Reset()
	if code := run([]string{"checkpoint", "--state", path, "--network", capture.Network, "--stream", "partial"}, &out, &errs); code != 0 {
		t.Fatal(code)
	}
	var view checkpointView
	if err := json.Unmarshal(out.Bytes(), &view); err != nil || view.Report.ProcessedThrough != 100 || view.Report.Status != "COMPLETE_CAPTURE_APPLICATION" || view.Coverage != "RETAINED_PREFIX_ONLY" {
		t.Fatal(view, err)
	}
}
