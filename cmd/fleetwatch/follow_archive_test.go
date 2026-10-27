package main

import (
	"bytes"
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"reflect"
	"testing"

	"github.com/soroban-fleetwatch/fleetwatch/internal/follow"
	"github.com/soroban-fleetwatch/fleetwatch/internal/history"
	"github.com/soroban-fleetwatch/fleetwatch/internal/jsoninput"
)

func TestFollowArchiveCLIAndOfflineVerification(t *testing.T) {
	dir := t.TempDir()
	var baseline history.Baseline
	var capture history.Capture
	if jsoninput.File(fixture("reconstruction-baseline.json"), &baseline) != nil || jsoninput.File(fixture("reconstruction-capture.json"), &capture) != nil {
		t.Fatal("fixtures unavailable")
	}
	writeJSON := func(name string, value any) {
		t.Helper()
		raw, err := json.Marshal(value)
		if err != nil || os.WriteFile(filepath.Join(dir, name), raw, 0600) != nil {
			t.Fatal("fixture write failed")
		}
	}
	writeJSON("baseline.json", baseline)
	cfg := follow.Config{SchemaVersion: 1, RPCURL: "https://rpc.example.invalid", Baseline: "baseline.json", State: "follow.db", Listen: "127.0.0.1:0", PollIntervalSeconds: 1, BatchSize: 8, HeadDelayLedgers: 1, MaxEndpointLag: 4, MaxRetainedLedgers: 100, MaxRetainedPayloadBytes: 64 << 20}
	writeJSON("follow.json", cfg)
	config := filepath.Join(dir, "follow.json")
	output := filepath.Join(dir, "archive.gz")
	var out, errs bytes.Buffer
	args := []string{"follow-archive", "--config", config, "--output", output}
	if code := run(args, &out, &errs); code != 5 {
		t.Fatal("missing state accepted", code, errs.String())
	}
	if _, err := os.Stat(filepath.Join(dir, cfg.State)); !os.IsNotExist(err) {
		t.Fatal("export created missing state")
	}
	if _, err := os.Stat(output); !os.IsNotExist(err) {
		t.Fatal("failed export created output")
	}
	s, err := follow.Open(context.Background(), filepath.Join(dir, cfg.State), baseline, cfg.Limits())
	if err != nil {
		t.Fatal(err)
	}
	if _, err := s.Commit(context.Background(), capture); err != nil {
		t.Fatal(err)
	}
	s.Close()
	before, _ := os.ReadFile(filepath.Join(dir, cfg.State))
	out.Reset()
	if code := run(args, &out, &errs); code != 0 {
		t.Fatal(code, errs.String())
	}
	var exported follow.ArchiveReceipt
	if json.Unmarshal(out.Bytes(), &exported) != nil || exported.RetainedLedgers != 1 {
		t.Fatal(out.String())
	}
	out.Reset()
	if code := run([]string{"verify-follow-archive", "--input", output}, &out, &errs); code != 0 {
		t.Fatal(code, errs.String())
	}
	var verified follow.ArchiveReceipt
	if json.Unmarshal(out.Bytes(), &verified) != nil || !reflect.DeepEqual(exported, verified) {
		t.Fatal("receipts differ", out.String())
	}
	restoredPath := filepath.Join(dir, "restored.db")
	out.Reset()
	if code := run([]string{"restore-follow-archive", "--input", output, "--output", restoredPath, "--sha256", verified.ArchiveSHA256}, &out, &errs); code != 0 {
		t.Fatal(code, errs.String())
	}
	var restored follow.RestoreReceipt
	if json.Unmarshal(out.Bytes(), &restored) != nil || !reflect.DeepEqual(restored.Archive, verified) || !reflect.DeepEqual(restored.Baseline, baseline) {
		t.Fatal("restore receipt mismatch", out.String())
	}
	if code := run([]string{"restore-follow-archive", "--input", output, "--output", restoredPath, "--sha256", verified.ArchiveSHA256}, &out, &errs); code != 5 {
		t.Fatal("existing restore overwritten", code)
	}
	if code := run([]string{"restore-follow-archive", "--input", output, "--output", restoredPath}, &out, &errs); code != 3 {
		t.Fatal("restore without digest accepted", code)
	}
	after, _ := os.ReadFile(filepath.Join(dir, cfg.State))
	if !bytes.Equal(before, after) {
		t.Fatal("archive mutated state")
	}
	if code := run(args, &out, &errs); code != 5 {
		t.Fatal("archive overwritten", code)
	}
	bad := filepath.Join(dir, "bad.gz")
	if err := os.WriteFile(bad, []byte("invalid"), 0600); err != nil {
		t.Fatal(err)
	}
	if code := run([]string{"verify-follow-archive", "--input", bad}, &out, &errs); code != 4 {
		t.Fatal("invalid archive accepted", code)
	}
	for _, args := range [][]string{{"verify-follow-archive"}, {"follow-archive", "--config", config}, {"verify-follow-archive", "--input", filepath.Join(dir, "missing")}} {
		if code := run(args, &out, &errs); code != 3 {
			t.Fatal("bad arguments accepted", args, code)
		}
	}
}
