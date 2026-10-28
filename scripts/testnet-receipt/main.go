// Read-only consistency check of a Testnet transaction receipt. A matching
// envelope hash is not an independent proof of ledger inclusion or consensus.
package main

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"os"
	"time"

	"github.com/soroban-fleetwatch/fleetwatch/internal/source"
	"github.com/stellar/go-stellar-sdk/network"
	"github.com/stellar/go-stellar-sdk/xdr"
)

func main() {
	if len(os.Args) != 2 {
		fmt.Fprintln(os.Stderr, "usage: testnet-receipt TRANSACTION_HASH")
		os.Exit(3)
	}
	if err := run(os.Args[1]); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(4)
	}
}

func run(hash string) error {
	ctx, cancel := context.WithTimeout(context.Background(), time.Minute)
	defer cancel()
	rpc, err := source.NewHTTPRPC("https://soroban-testnet.stellar.org")
	if err != nil {
		return err
	}
	n, err := rpc.Network(ctx)
	if err != nil || n.Passphrase != network.TestNetworkPassphrase || n.ProtocolVersion < 28 || n.ProtocolVersion > 29 {
		return fmt.Errorf("Testnet identity/protocol unavailable")
	}
	r, err := rpc.Transaction(ctx, hash)
	if err != nil || r.Status != "SUCCESS" || r.Ledger == 0 || r.TransactionHash != hash || r.FeeBump {
		return fmt.Errorf("successful non-fee-bump transaction receipt unavailable")
	}
	var env xdr.TransactionEnvelope
	var result xdr.TransactionResult
	var meta xdr.TransactionMeta
	opts := xdr.DecodeOptions{MaxDepth: 64, MaxMemoryBytes: 16 << 20}
	if xdr.SafeUnmarshalBase64WithOptions(r.EnvelopeXDR, &env, opts) != nil || xdr.SafeUnmarshalBase64WithOptions(r.ResultXDR, &result, opts) != nil || xdr.SafeUnmarshalBase64WithOptions(r.ResultMetaXDR, &meta, opts) != nil || !result.Successful() {
		return fmt.Errorf("invalid transaction XDR or unsuccessful result")
	}
	actual, err := network.HashTransactionInEnvelope(env, network.TestNetworkPassphrase)
	if err != nil || hex.EncodeToString(actual[:]) != hash {
		return fmt.Errorf("Testnet envelope hash mismatch")
	}
	var value *xdr.ScVal
	if meta.V == 3 && meta.V3 != nil && meta.V3.SorobanMeta != nil {
		value = &meta.V3.SorobanMeta.ReturnValue
	}
	if meta.V == 4 && meta.V4 != nil && meta.V4.SorobanMeta != nil {
		value = meta.V4.SorobanMeta.ReturnValue
	}
	metaRaw, err := meta.MarshalBinary()
	if err != nil {
		return fmt.Errorf("invalid transaction metadata")
	}
	digest := sha256.Sum256(metaRaw)
	out := map[string]any{"transaction_hash": hash, "ledger": r.Ledger, "status": r.Status, "envelope_hash_matches_testnet": true, "result_successful": true, "metadata_version": meta.V, "metadata_sha256": hex.EncodeToString(digest[:]), "return_value": value, "source_authentication": "UNVERIFIED", "inclusion_proof": "NOT_VERIFIED", "final_verdict": "UNDETERMINED", "receipt": r}
	enc := json.NewEncoder(os.Stdout)
	enc.SetIndent("", "  ")
	return enc.Encode(out)
}
