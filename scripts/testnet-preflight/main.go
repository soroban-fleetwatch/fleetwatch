// This read-only developer tool checks the fixed public Testnet endpoint.
// It neither funds accounts nor signs/submits transactions.
package main

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/soroban-fleetwatch/fleetwatch/internal/history"
	"github.com/soroban-fleetwatch/fleetwatch/internal/source"
)

const endpoint = "https://soroban-testnet.stellar.org"
const passphrase = "Test SDF Network ; September 2015"

type summary struct {
	CheckedAt            string         `json:"checked_at"`
	Endpoint             string         `json:"endpoint"`
	Status               string         `json:"status"`
	Error                string         `json:"error,omitempty"`
	HistoryErrors        []string       `json:"history_errors,omitempty"`
	Network              source.Network `json:"network"`
	Head                 source.Head    `json:"head"`
	Ledger               uint32         `json:"ledger,omitempty"`
	HeaderHash           string         `json:"header_hash,omitempty"`
	PreviousHash         string         `json:"previous_hash,omitempty"`
	MetadataSHA256       string         `json:"metadata_sha256,omitempty"`
	MetadataVersion      int32          `json:"metadata_version"`
	SourceAuthentication string         `json:"source_authentication"`
	MetadataCommitment   string         `json:"metadata_commitment"`
	FinalVerdict         string         `json:"final_verdict"`
	TransactionsSent     int            `json:"transactions_sent"`
}

func check(ctx context.Context) summary {
	out := summary{CheckedAt: time.Now().UTC().Format(time.RFC3339), Endpoint: endpoint,
		Status: "BLOCKED", SourceAuthentication: "UNVERIFIED", MetadataCommitment: "NOT_VERIFIED", FinalVerdict: "UNDETERMINED"}
	fail := func(code string) summary { out.Error = code; return out }
	rpc, err := source.NewHTTPRPC(endpoint)
	if err != nil {
		return fail("INVALID_ENDPOINT")
	}
	out.Network, err = rpc.Network(ctx)
	if err != nil {
		return fail("NETWORK_QUERY_FAILED")
	}
	if out.Network.Passphrase != passphrase || out.Network.ProtocolVersion < 28 || out.Network.ProtocolVersion > 29 {
		return fail("NETWORK_OR_PROTOCOL_MISMATCH")
	}
	out.Head, err = rpc.Head(ctx)
	if err != nil {
		return fail("HEAD_QUERY_FAILED")
	}
	age := time.Since(time.Unix(out.Head.CloseTime, 0))
	if out.Head.Sequence < 3 || out.Head.ProtocolVersion != uint32(out.Network.ProtocolVersion) || age < -30*time.Second || age > 2*time.Minute {
		return fail("INVALID_OR_STALE_HEAD")
	}
	// RPC history indexing may lag its latest closed ledger. Sample two ledgers
	// behind the head; report this sequence separately, without calling it a tip.
	sample := out.Head.Sequence - 2
	capture, err := history.Fetch(ctx, rpc, passphrase, sample, sample)
	if err != nil || capture.SequenceCoverage != "COMPLETE_ENDPOINT_SEQUENCE" || len(capture.Ledgers) != 1 {
		out.HistoryErrors = capture.Errors
		return fail("LEDGER_METADATA_QUERY_OR_VALIDATION_FAILED")
	}
	l := capture.Ledgers[0]
	out.Ledger, out.HeaderHash, out.PreviousHash = l.Sequence, l.Hash, l.PreviousHash
	out.MetadataSHA256, out.MetadataVersion = l.MetadataSHA256, l.MetadataVersion
	out.Status = "READ_ONLY_PREFLIGHT_PASS"
	return out
}

func main() {
	if len(os.Args) != 1 {
		fmt.Fprintln(os.Stderr, "testnet-preflight takes no arguments; endpoint and network are fixed to Testnet")
		os.Exit(3)
	}
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()
	ctx, cancel := context.WithTimeout(ctx, time.Minute)
	defer cancel()
	out := check(ctx)
	enc := json.NewEncoder(os.Stdout)
	enc.SetIndent("", "  ")
	if enc.Encode(out) != nil {
		os.Exit(5)
	}
	if out.Error != "" {
		os.Exit(4)
	}
}
