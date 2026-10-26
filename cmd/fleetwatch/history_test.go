package main

import (
	"bytes"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"

	"github.com/soroban-fleetwatch/fleetwatch/internal/history"
	"github.com/soroban-fleetwatch/fleetwatch/internal/source"
	protocol "github.com/stellar/go-stellar-sdk/protocols/rpc"
	"github.com/stellar/go-stellar-sdk/xdr"
)

func TestCLIHistoryRejectsInvalidRequestsBeforeNetwork(t *testing.T) {
	for _, args := range [][]string{
		{"history"},
		{"history", "--rpc-url", "http://example.com", "--network-passphrase", "test", "--from-ledger", "100", "--to-ledger", "101"},
		{"history", "--rpc-url", "https://example.com", "--network-passphrase", "test", "--from-ledger", "0", "--to-ledger", "1"},
		{"history", "--rpc-url", "https://example.com", "--network-passphrase", "test", "--from-ledger", "100", "--to-ledger", "116"},
		{"history", "--rpc-url", "https://example.com", "--network-passphrase", "test", "--from-ledger", "4294967296", "--to-ledger", "4294967297"},
		{"history", "--rpc-url", "https://example.com", "--network-passphrase", "test", "--from-ledger", "100", "--to-ledger", "101", "extra"},
	} {
		var out, errs bytes.Buffer
		if code := run(args, &out, &errs); code != 3 || out.Len() != 0 {
			t.Fatalf("code=%d out=%s err=%s", code, &out, &errs)
		}
	}
}

func TestCLIHistoryTLSCapturePartialAndOutputErrors(t *testing.T) {
	header := xdr.LedgerHeader{LedgerVersion: 29, LedgerSeq: 100, ScpValue: xdr.StellarValue{CloseTime: 1700000000}}
	raw, err := header.MarshalBinary()
	if err != nil {
		t.Fatal(err)
	}
	hash := sha256.Sum256(raw)
	entry := xdr.LedgerHeaderHistoryEntry{Header: header, Hash: xdr.Hash(hash)}
	headerXDR, err := xdr.MarshalBase64(entry)
	if err != nil {
		t.Fatal(err)
	}
	metadataXDR, err := xdr.MarshalBase64(xdr.LedgerCloseMeta{V: 0, V0: &xdr.LedgerCloseMetaV0{LedgerHeader: entry}})
	if err != nil {
		t.Fatal(err)
	}
	info := protocol.LedgerInfo{Sequence: 100, Hash: hex.EncodeToString(hash[:]), LedgerCloseTime: 1700000000, LedgerHeader: headerXDR, LedgerMetadata: metadataXDR}
	server := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var req struct {
			ID     uint64                     `json:"id"`
			Method string                     `json:"method"`
			Params protocol.GetLedgersRequest `json:"params"`
		}
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			t.Error(err)
			return
		}
		var result any
		switch req.Method {
		case "getNetwork":
			result = source.Network{Passphrase: "SYNTHETIC-NETWORK", ProtocolVersion: 29}
		case "getLedgers":
			if req.Params.StartLedger != 100 || req.Params.Pagination == nil || req.Params.Pagination.Limit != 1 {
				t.Error("invalid ledger request")
			}
			ledger := info
			if r.URL.Path == "/partial" {
				ledger.LedgerMetadata = "bad"
			}
			result = protocol.GetLedgersResponse{OldestLedger: 100, LatestLedger: 101, Ledgers: []protocol.LedgerInfo{ledger}}
		default:
			t.Error("unexpected RPC method")
		}
		_ = json.NewEncoder(w).Encode(map[string]any{"jsonrpc": "2.0", "id": req.ID, "result": result})
	}))
	defer server.Close()
	// Use the local server's generated CA with certificate verification enabled.
	// Tests in this package do not run in parallel; restore the global transport.
	original := http.DefaultTransport
	http.DefaultTransport = server.Client().Transport
	defer func() { http.DefaultTransport = original }()
	args := []string{"history", "--rpc-url", server.URL, "--network-passphrase", "SYNTHETIC-NETWORK", "--from-ledger", "100", "--to-ledger", "100"}
	var out, errs bytes.Buffer
	if code := run(args, &out, &errs); code != 0 {
		t.Fatal(code, errs.String())
	}
	var capture history.Capture
	if err := json.Unmarshal(out.Bytes(), &capture); err != nil || capture.SequenceCoverage != "COMPLETE_ENDPOINT_SEQUENCE" || capture.FinalVerdict != "UNDETERMINED" {
		t.Fatal(capture, err)
	}
	path := filepath.Join(t.TempDir(), "capture.json")
	if err := os.WriteFile(path, out.Bytes(), 0600); err != nil {
		t.Fatal(err)
	}
	out.Reset()
	if code := run([]string{"replay", "--journal", path}, &out, &errs); code != 3 {
		t.Fatal("diagnostic metadata accepted as snapshot journal", code)
	}
	if code := run(args, reportFailWriter{}, &errs); code != 5 {
		t.Fatal("output failure", code)
	}
	args[2] = server.URL + "/partial"
	out.Reset()
	if code := run(args, &out, &errs); code != 4 {
		t.Fatal(code, errs.String())
	}
	if err := json.Unmarshal(out.Bytes(), &capture); err != nil || capture.ProcessedThrough != 0 || capture.SequenceCoverage != "PARTIAL" || len(capture.Errors) != 1 {
		t.Fatal(capture, err)
	}
}
