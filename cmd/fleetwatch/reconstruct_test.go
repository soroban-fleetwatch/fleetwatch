package main

import (
	"bytes"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/soroban-fleetwatch/fleetwatch/internal/fleetwatch"
	"github.com/soroban-fleetwatch/fleetwatch/internal/history"
)

func TestCLIReconstructionTransientAndFailureExits(t *testing.T) {
	args := []string{"reconstruct", "--baseline", fixture("reconstruction-baseline.json"), "--capture", fixture("reconstruction-capture.json")}
	var out, errs bytes.Buffer
	if code := run(args, &out, &errs); code != 2 {
		t.Fatal(code, errs.String())
	}
	var r history.Reconstruction
	if err := json.Unmarshal(out.Bytes(), &r); err != nil || r.ProcessedThrough != 100 || r.Status != "COMPLETE_CAPTURE_APPLICATION" || len(r.Ledgers) != 1 || len(r.Ledgers[0].Events) != 2 || r.FinalVerdict != "UNDETERMINED" {
		t.Fatal(r, err)
	}
	if code := run(args, reportFailWriter{}, &errs); code != 5 {
		t.Fatal("output failure", code)
	}
	path := filepath.Join(t.TempDir(), "reconstruction.json")
	if err := os.WriteFile(path, out.Bytes(), 0600); err != nil {
		t.Fatal(err)
	}
	out.Reset()
	if code := run([]string{"replay", "--journal", path}, &out, &errs); code != 3 {
		t.Fatal("diagnostic accepted as replay journal", code)
	}
	for _, args := range [][]string{{"reconstruct"}, {"reconstruct", "--baseline", fixture("before.json"), "--capture", fixture("reconstruction-capture.json")}, {"reconstruct", "--baseline", "missing.json", "--capture", fixture("reconstruction-capture.json")}, append(append([]string{}, args...), "extra")} {
		out.Reset()
		if code := run(args, &out, &errs); code != 3 || out.Len() != 0 {
			t.Fatal(code, out.String())
		}
	}
	raw, err := os.ReadFile(fixture("reconstruction-baseline.json"))
	if err != nil {
		t.Fatal(err)
	}
	var b history.Baseline
	if err := json.Unmarshal(raw, &b); err != nil {
		t.Fatal(err)
	}
	b.HeaderHash = strings.Repeat("1", 64)
	encoded, _ := json.Marshal(b)
	if err := os.WriteFile(path, encoded, 0600); err != nil {
		t.Fatal(err)
	}
	partial := append([]string{}, args...)
	partial[2] = path
	out.Reset()
	if code := run(partial, &out, &errs); code != 4 {
		t.Fatal(code, errs.String())
	}
	if err := json.Unmarshal(out.Bytes(), &r); err != nil || r.ProcessedThrough != 99 || len(r.Ledgers) != 0 || r.Snapshot.Ledger != 99 {
		t.Fatal(r, err)
	}
	// A missing/changed input baseline is a usage/input failure, not an empty clean report.
	b.Snapshot.References = nil
	encoded, _ = json.Marshal(b)
	if err := os.WriteFile(path, encoded, 0600); err != nil {
		t.Fatal(err)
	}
	out.Reset()
	if code := run(partial, &out, &errs); code != 3 || out.Len() != 0 {
		t.Fatal(code, out.String())
	}
}

func TestCLIReconstructionNoWatchedFindingsStillUnverified(t *testing.T) {
	raw, err := os.ReadFile(fixture("reconstruction-baseline.json"))
	if err != nil {
		t.Fatal(err)
	}
	var b history.Baseline
	if err := json.Unmarshal(raw, &b); err != nil {
		t.Fatal(err)
	}
	for i := range b.Snapshot.Contracts {
		b.Snapshot.Contracts[i].Executable = fleetwatch.Executable{Kind: "wasm", WasmHash: strings.Repeat("aa", 32)}
	}
	b.Snapshot.References = nil
	encoded, err := json.Marshal(b)
	if err != nil {
		t.Fatal(err)
	}
	path := filepath.Join(t.TempDir(), "baseline.json")
	if err := os.WriteFile(path, encoded, 0600); err != nil {
		t.Fatal(err)
	}
	var out, errs bytes.Buffer
	if code := run([]string{"reconstruct", "--baseline", path, "--capture", fixture("reconstruction-capture.json")}, &out, &errs); code != 0 {
		t.Fatal(code, errs.String())
	}
	var r history.Reconstruction
	if err := json.Unmarshal(out.Bytes(), &r); err != nil || r.FinalVerdict != "UNDETERMINED" || r.MembershipCoverage != "UNKNOWN" || r.EvaluationStatus != "UNVERIFIED_PROJECTED_STATE" || len(r.Ledgers[0].Events) != 0 {
		t.Fatal(r, err)
	}
}
