package ingest

import (
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"testing"

	"github.com/soroban-fleetwatch/fleetwatch/internal/history"
	"github.com/soroban-fleetwatch/fleetwatch/internal/jsoninput"
	"github.com/soroban-fleetwatch/fleetwatch/internal/source"
	"github.com/soroban-fleetwatch/fleetwatch/internal/store"
	protocol "github.com/stellar/go-stellar-sdk/protocols/rpc"
)

// replayLiveRPC replays unchanged captured Testnet records. Its sole injected
// fault is returning the next actual record for one requested, closed ledger.
type replayLiveRPC struct {
	capture history.Capture
	omit    uint32
}

func (r replayLiveRPC) Network(context.Context) (source.Network, error) {
	return source.Network{Passphrase: r.capture.Network, ProtocolVersion: int(r.capture.Ledgers[0].ProtocolVersion)}, nil
}
func (r replayLiveRPC) Ledgers(_ context.Context, req protocol.GetLedgersRequest) (protocol.GetLedgersResponse, error) {
	wanted := req.StartLedger
	if wanted == r.omit {
		wanted++
	}
	out := protocol.GetLedgersResponse{OldestLedger: r.capture.FromLedger, LatestLedger: r.capture.ToLedger}
	for _, e := range r.capture.Ledgers {
		if e.Sequence == wanted {
			out.Ledgers = []protocol.LedgerInfo{{Sequence: e.Sequence, Hash: e.Hash, LedgerCloseTime: e.CloseTime, LedgerHeader: e.HeaderXDR, LedgerMetadata: e.MetadataXDR}}
			break
		}
	}
	return out, nil
}

func TestLiveTestnetHistoryGapRecovery(t *testing.T) {
	baselinePath, capturePath, output := os.Getenv("FLEETWATCH_TESTNET_BASELINE"), os.Getenv("FLEETWATCH_TESTNET_CAPTURE"), os.Getenv("FLEETWATCH_TESTNET_GAP_REPORT")
	if baselinePath == "" && capturePath == "" && output == "" {
		t.Skip("opt-in Testnet capture not provided")
	}
	var baseline history.Baseline
	var capture history.Capture
	if jsoninput.File(baselinePath, &baseline) != nil || jsoninput.File(capturePath, &capture) != nil || capture.Network != "Test SDF Network ; September 2015" || len(capture.Ledgers) < 3 {
		t.Fatal("complete real Testnet baseline/capture required")
	}
	ctx := context.Background()
	path := filepath.Join(t.TempDir(), "gap.db")
	s, err := store.Open(path)
	if err != nil {
		t.Fatal(err)
	}
	defer func() {
		if s != nil {
			_ = s.Close()
		}
	}()
	missing := capture.FromLedger + 1
	rpc := replayLiveRPC{capture: capture, omit: missing}
	first, err := Run(ctx, rpc, s, "gap-replay", baseline, capture.ToLedger)
	if err == nil || first.Status != "PARTIAL" || first.ProcessedThrough != missing-1 || len(first.FetchErrors) != 1 || first.FetchErrors[0] != "MISSING_OR_UNEXPECTED_LEDGER" {
		t.Fatal("skipped closed ledger was not stopped", first, err)
	}
	if err := s.Close(); err != nil {
		t.Fatal(err)
	}
	s, err = store.Open(path)
	if err != nil {
		t.Fatal(err)
	}
	rpc.omit = 0
	repaired, err := Run(ctx, rpc, s, "gap-replay", baseline, capture.ToLedger)
	if err != nil || repaired.Status != "COMPLETE" || repaired.ProcessedThrough != capture.ToLedger || repaired.Assurance.FinalVerdict != "UNDETERMINED" {
		t.Fatal("gap recovery failed", repaired, err)
	}
	rows, err := s.ReconstructionAlerts(ctx, capture.Network, "gap-replay")
	if err != nil {
		t.Fatal(err)
	}
	repeat, err := Run(ctx, nil, s, "gap-replay", baseline, capture.ToLedger)
	if err != nil || repeat.Status != "ALREADY_RETAINED" || repeat.SourceQuery != "NOT_QUERIED" {
		t.Fatal("duplicate replay was not a no-op", repeat, err)
	}
	after, err := s.ReconstructionAlerts(ctx, capture.Network, "gap-replay")
	if err != nil || len(after) != len(rows) {
		t.Fatal("duplicate outbox entries", err)
	}
	for i := range rows {
		if after[i].ID != rows[i].ID {
			t.Fatal("unstable alert identity")
		}
	}
	f, err := os.OpenFile(output, os.O_CREATE|os.O_EXCL|os.O_WRONLY, 0600)
	if err != nil {
		t.Fatal(err)
	}
	defer f.Close()
	if json.NewEncoder(f).Encode(map[string]any{"kind": "LOCAL_FAULT_INJECTION_OVER_CAPTURED_TESTNET_METADATA", "omitted_closed_ledger": missing, "retained_prefix": first.ProcessedThrough, "error": first.FetchErrors[0], "cold_reopen_repaired_through": repaired.ProcessedThrough, "repeat_status": repeat.Status, "outbox_entries": len(rows), "final_verdict": "UNDETERMINED"}) != nil {
		t.Fatal("cannot save gap evidence")
	}
}
