package main

import (
	"bytes"
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/soroban-fleetwatch/fleetwatch/internal/follow"
	"github.com/soroban-fleetwatch/fleetwatch/internal/history"
	"github.com/soroban-fleetwatch/fleetwatch/internal/jsoninput"
	"github.com/soroban-fleetwatch/fleetwatch/internal/store"
)

func TestFollowSyncCLIValidatesSourceAndDeduplicates(t *testing.T) {
	ctx := context.Background()
	dir := t.TempDir()
	var b history.Baseline
	var c history.Capture
	if jsoninput.File(fixture("reconstruction-baseline.json"), &b) != nil || jsoninput.File(fixture("reconstruction-capture.json"), &c) != nil {
		t.Fatal("fixtures unavailable")
	}
	write := func(name string, v any) {
		t.Helper()
		raw, err := json.Marshal(v)
		if err != nil || os.WriteFile(filepath.Join(dir, name), raw, 0600) != nil {
			t.Fatal("fixture write failed")
		}
	}
	write("baseline.json", b)
	cfg := follow.Config{SchemaVersion: 1, RPCURL: "https://rpc.example.invalid", Baseline: "baseline.json", State: "follow.db", Listen: "127.0.0.1:0", PollIntervalSeconds: 1, BatchSize: 8, MaxEndpointLag: 4, MaxRetainedLedgers: 100, MaxRetainedPayloadBytes: 64 << 20}
	write("follow.json", cfg)
	config := filepath.Join(dir, "follow.json")
	delivery := filepath.Join(dir, "delivery.db")
	args := []string{"follow-sync", "--config", config, "--delivery-state", delivery, "--stream", "pilot"}
	var out, errs bytes.Buffer
	if code := run(args, &out, &errs); code != 5 {
		t.Fatal("missing follow source accepted", code)
	}
	if _, err := os.Stat(delivery); !os.IsNotExist(err) {
		t.Fatal("invalid source created delivery database")
	}
	sourcePath := filepath.Join(dir, cfg.State)
	s, err := follow.Open(ctx, sourcePath, b, cfg.Limits())
	if err != nil {
		t.Fatal(err)
	}
	if _, err := s.Commit(ctx, c); err != nil {
		t.Fatal(err)
	}
	head := s.Head()
	if h, err := s.HeaderAt(ctx, b.Snapshot.Ledger); err != nil || h != b.HeaderHash {
		t.Fatal("original anchor unavailable", h, err)
	}
	s.Close()
	before, _ := os.ReadFile(sourcePath)
	for i := 0; i < 2; i++ {
		out.Reset()
		if code := run(args, &out, &errs); code != 0 {
			t.Fatal(code, errs.String())
		}
		var r store.FollowSyncResult
		if json.Unmarshal(out.Bytes(), &r) != nil || r.Added+r.Retained != head.PendingFindings || i == 1 && r.Added != 0 || !r.Offline {
			t.Fatal(out.String())
		}
	}
	out.Reset()
	if code := run([]string{"alerts", "--state", delivery, "--network", c.Network, "--source", "follow_ledger_meta", "--stream", "pilot"}, &out, &errs); code != 0 {
		t.Fatal(code, errs.String())
	}
	var rows []store.Alert
	if json.Unmarshal(out.Bytes(), &rows) != nil || len(rows) != head.PendingFindings {
		t.Fatal(out.String())
	}
	after, _ := os.ReadFile(sourcePath)
	if !bytes.Equal(before, after) {
		t.Fatal("sync modified follow source")
	}
	if code := run([]string{"follow-sync", "--config", config, "--delivery-state", sourcePath, "--stream", "pilot"}, &out, &errs); code != 3 {
		t.Fatal("source collision accepted", code)
	}
	// A valid-looking manifest with an incompatible prior header must never be
	// accepted by the native replay-validating adapter on a later sync.
	state, err := store.Open(delivery)
	if err != nil {
		t.Fatal(err)
	}
	prior, err := state.FollowImportBinding(ctx, c.Network, "pilot")
	if err != nil {
		t.Fatal(err)
	}
	changed := *prior
	changed.ProcessedThrough++
	changed.HeaderHash = strings.Repeat("a", 64)
	events := []store.ReconstructionEventAlert{}
	for _, row := range rows {
		events = append(events, *row.Payload.Reconstruction)
	}
	// Queue order is by delivery ID; import order is by original event ID.
	if len(events) == 2 && events[0].Event.ID > events[1].Event.ID {
		events[0], events[1] = events[1], events[0]
	}
	if _, err := state.ImportFollow(ctx, changed, events, prior); err != nil {
		t.Fatal(err)
	}
	state.Close()
	if code := run(args, &out, &errs); code != 5 {
		t.Fatal("rewound source accepted", code)
	}
}
