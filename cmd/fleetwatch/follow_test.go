package main

import (
	"bytes"
	"encoding/json"
	"os"
	"path/filepath"
	"testing"

	"github.com/soroban-fleetwatch/fleetwatch/internal/follow"
)

func TestFollowCheckAndInspectDoNotCreateState(t *testing.T) {
	dir := t.TempDir()
	baseline, err := os.ReadFile(fixture("reconstruction-baseline.json"))
	if err != nil {
		t.Fatal(err)
	}
	os.WriteFile(filepath.Join(dir, "baseline.json"), baseline, 0600)
	cfg := follow.Config{SchemaVersion: 1, RPCURL: "https://rpc.example.invalid", Baseline: "baseline.json", State: "state.db", Listen: "127.0.0.1:0", PollIntervalSeconds: 1, BatchSize: 8, HeadDelayLedgers: 1, MaxEndpointLag: 4, MaxRetainedLedgers: 100, MaxRetainedPayloadBytes: 64 << 20}
	path := filepath.Join(dir, "follow.json")
	raw, _ := json.Marshal(cfg)
	os.WriteFile(path, raw, 0600)
	var out, errs bytes.Buffer
	if code := run([]string{"follow", "--config", path, "--check"}, &out, &errs); code != 0 {
		t.Fatal(code, errs.String())
	}
	delivery := filepath.Join(dir, "delivery.db")
	if code := run([]string{"follow", "--config", path, "--check", "--delivery-state", delivery, "--delivery-stream", "pilot"}, &out, &errs); code != 0 {
		t.Fatal(code, errs.String())
	}
	if _, err := os.Stat(delivery); !os.IsNotExist(err) {
		t.Fatal("offline check created delivery database")
	}
	for _, options := range [][]string{
		{"--delivery-state", delivery}, {"--delivery-stream", "pilot"},
		{"--delivery-state", delivery, "--delivery-stream", "invalid stream"},
		{"--delivery-state", filepath.Join(dir, "state.db"), "--delivery-stream", "pilot"},
		{"--delivery-state", filepath.Join(dir, "baseline.json"), "--delivery-stream", "pilot"},
		{"--delivery-state", path, "--delivery-stream", "pilot"},
	} {
		if code := run(append([]string{"follow", "--config", path, "--check"}, options...), &out, &errs); code != 3 {
			t.Fatal("invalid automatic sync options accepted", options, code)
		}
	}
	if _, err := os.Stat(filepath.Join(dir, "state.db")); !os.IsNotExist(err) {
		t.Fatal("check created a state DB")
	}
	if code := run([]string{"follow-inspect", "--config", path}, &out, &errs); code != 5 {
		t.Fatal(code, errs.String())
	}
	if _, err := os.Stat(filepath.Join(dir, "state.db")); !os.IsNotExist(err) {
		t.Fatal("inspection created a state DB")
	}
	if code := run([]string{"follow-inspect", "--config", path, "--after-id", "bad"}, &out, &errs); code != 3 {
		t.Fatal(code)
	}
}
