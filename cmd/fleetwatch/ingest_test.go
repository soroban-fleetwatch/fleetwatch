package main

import (
	"bytes"
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"testing"

	"github.com/soroban-fleetwatch/fleetwatch/internal/history"
	"github.com/soroban-fleetwatch/fleetwatch/internal/ingest"
	"github.com/soroban-fleetwatch/fleetwatch/internal/jsoninput"
	"github.com/soroban-fleetwatch/fleetwatch/internal/source"
	protocol "github.com/stellar/go-stellar-sdk/protocols/rpc"
)

type ingestionRPC struct {
	calls int
	fail  bool
}

func (r *ingestionRPC) Network(context.Context) (source.Network, error) {
	return source.Network{Passphrase: "SYNTHETIC-NETWORK", ProtocolVersion: 29}, nil
}
func (r *ingestionRPC) Ledgers(ctx context.Context, req protocol.GetLedgersRequest) (protocol.GetLedgersResponse, error) {
	r.calls++
	out := protocol.GetLedgersResponse{OldestLedger: 100, LatestLedger: 101}
	if r.fail {
		return out, nil
	}
	name := "reconstruction-capture.json"
	if req.StartLedger == 101 {
		name = "reconstruction-next-capture.json"
	}
	var c history.Capture
	if err := jsoninput.File(fixture(name), &c); err != nil {
		return out, err
	}
	e := c.Ledgers[0]
	out.Ledgers = []protocol.LedgerInfo{{Sequence: e.Sequence, Hash: e.Hash, LedgerCloseTime: e.CloseTime, LedgerHeader: e.HeaderXDR, LedgerMetadata: e.MetadataXDR}}
	return out, nil
}
func TestCLIIngestOutputFailureResumeAndNoop(t *testing.T) {
	path := filepath.Join(t.TempDir(), "state.db")
	args := []string{"--rpc-url", "https://rpc.example.com", "--baseline", fixture("reconstruction-baseline.json"), "--state", path, "--stream", "ingest", "--to-ledger", "100"}
	r := &ingestionRPC{}
	factory := func(endpoint string) (history.RPC, error) {
		if _, err := source.NewHTTPRPC(endpoint); err != nil {
			return nil, err
		}
		return r, nil
	}
	var out, errs bytes.Buffer
	if code := ingestHistoryWithRPC(args, reportFailWriter{}, &errs, factory); code != 5 {
		t.Fatal(code, errs.String())
	}
	if code := ingestHistoryWithRPC(args, &out, &errs, factory); code != 0 {
		t.Fatal(code, errs.String())
	}
	var result ingest.Result
	if err := json.Unmarshal(out.Bytes(), &result); err != nil || result.Status != "ALREADY_RETAINED" || result.ProcessedThrough != 100 || r.calls != 1 {
		t.Fatal(result, err, r.calls)
	}
	out.Reset()
	args[len(args)-1] = "101"
	if code := ingestHistoryWithRPC(args, &out, &errs, factory); code != 2 {
		t.Fatal(code, errs.String())
	}
	if err := json.Unmarshal(out.Bytes(), &result); err != nil || result.Status != "COMPLETE" || result.FetchFrom != 101 || r.calls != 2 {
		t.Fatal(result, err, r.calls)
	}
	out.Reset()
	if code := run([]string{"doctor", "--state", path, "--network", "SYNTHETIC-NETWORK", "--source", "rpc_ledger_meta", "--stream", "ingest"}, &out, &errs); code != 0 {
		t.Fatal(code, errs.String())
	}
}
func TestCLIIngestPartialDiagnosticAndInvalidArguments(t *testing.T) {
	path := filepath.Join(t.TempDir(), "state.db")
	args := []string{"--rpc-url", "https://rpc.example.com", "--baseline", fixture("reconstruction-baseline.json"), "--state", path, "--stream", "ingest", "--to-ledger", "100"}
	r := &ingestionRPC{fail: true}
	factory := func(string) (history.RPC, error) { return r, nil }
	var out, errs bytes.Buffer
	if code := ingestHistoryWithRPC(args, &out, &errs, factory); code != 4 {
		t.Fatal(code, errs.String())
	}
	var result ingest.Result
	if err := json.Unmarshal(out.Bytes(), &result); err != nil || result.Status != "PARTIAL" || result.ProcessedThrough != 99 || result.LatestAttemptStatus != "PARTIAL" {
		t.Fatal(result, err)
	}
	invalid := filepath.Join(t.TempDir(), "never-created.db")
	for _, values := range [][]string{{"ingest"}, {"ingest", "--rpc-url", "http://untrusted", "--baseline", fixture("reconstruction-baseline.json"), "--state", invalid, "--stream", "ingest", "--to-ledger", "100"}, {"ingest", "--rpc-url", "https://rpc.example.com", "--baseline", fixture("reconstruction-baseline.json"), "--state", invalid, "--stream", "ingest", "--to-ledger", "116"}, {"ingest", "--rpc-url", "https://rpc.example.com", "--baseline", fixture("reconstruction-baseline.json"), "--state", invalid, "--stream", "../bad", "--to-ledger", "100"}} {
		out.Reset()
		if code := run(values, &out, &errs); code != 3 || out.Len() != 0 {
			t.Fatal(values, code, out.String(), errs.String())
		}
	}
	if _, err := os.Stat(invalid); !os.IsNotExist(err) {
		t.Fatal("invalid input created DB", err)
	}
}
