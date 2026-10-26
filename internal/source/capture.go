package source

import (
	"context"
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
	"errors"
	"fmt"
	"sort"
	"time"

	"github.com/soroban-fleetwatch/fleetwatch/internal/fleetwatch"
	"github.com/stellar/go-stellar-sdk/xdr"
)

const MaxWatchlist = 50

type Network struct {
	Passphrase      string `json:"passphrase"`
	ProtocolVersion int    `json:"protocolVersion"`
}
type Entry struct {
	Key          string `json:"key"`
	XDR          string `json:"xdr"`
	LastModified uint32 `json:"lastModifiedLedgerSeq"`
}
type Entries struct {
	Entries      []Entry `json:"entries"`
	LatestLedger uint32  `json:"latestLedger"`
}
type Head struct {
	Sequence        uint32 `json:"sequence"`
	ProtocolVersion uint32 `json:"protocolVersion"`
	CloseTime       int64  `json:"closeTime,string"`
}
type RPC interface {
	Head(context.Context) (Head, error)
	Network(context.Context) (Network, error)
	Entries(context.Context, []string) (Entries, error)
}
type Evidence struct {
	KeyXDR         string `json:"key_xdr"`
	DataXDR        string `json:"data_xdr"`
	RawSHA256      string `json:"raw_sha256"`
	ResponseLedger uint32 `json:"response_ledger"`
	LastModified   uint32 `json:"last_modified_ledger"`
}
type Capture struct {
	Head                 Head                `json:"head"`
	SchemaVersion        int                 `json:"schema_version"`
	Source               string              `json:"source"`
	CollectedAt          string              `json:"collected_at"`
	FleetScope           string              `json:"fleet_scope"`
	Watchlist            []string            `json:"watchlist"`
	Completeness         string              `json:"completeness"`
	LedgerConsistency    string              `json:"ledger_consistency"`
	HistoryCompleteness  string              `json:"history_completeness"`
	SourceAuthentication string              `json:"source_authentication"`
	Snapshot             fleetwatch.Snapshot `json:"snapshot"`
	Evidence             []Evidence          `json:"evidence"`
	Errors               []string            `json:"errors"`
}

func ValidateWatchlist(ids []string) error {
	if len(ids) == 0 || len(ids) > MaxWatchlist {
		return fmt.Errorf("watchlist requires 1-%d contracts", MaxWatchlist)
	}
	seen := map[string]bool{}
	for _, id := range ids {
		if _, err := contractAddress(id); err != nil {
			return err
		}
		if seen[id] {
			return errors.New("duplicate watchlist contract")
		}
		seen[id] = true
	}
	return nil
}

// CaptureCurrent records endpoint claims. It never claims authenticated chain
// history or a consistent ledger across multiple RPC calls.
func CaptureCurrent(ctx context.Context, rpc RPC, network string, ids []string) (Capture, error) {
	c := Capture{SchemaVersion: 2, Source: "rpc_current", CollectedAt: time.Now().UTC().Format(time.RFC3339), FleetScope: "EXPLICIT_WATCHLIST", Watchlist: append([]string(nil), ids...), Completeness: "PARTIAL", LedgerConsistency: "UNKNOWN", HistoryCompleteness: "UNKNOWN", SourceAuthentication: "UNVERIFIED", Evidence: []Evidence{}, Errors: []string{}}
	if err := ValidateWatchlist(ids); err != nil {
		return c, err
	}
	if network == "" {
		return c, errors.New("expected network passphrase required")
	}
	n, err := rpc.Network(ctx)
	if err != nil {
		return c, fmt.Errorf("network query failed: %w", err)
	}
	if n.Passphrase != network {
		return c, errors.New("RPC network passphrase mismatch")
	}
	if n.ProtocolVersion < 28 {
		return c, errors.New("RPC protocol does not support CAP-85")
	}
	head, err := rpc.Head(ctx)
	if err != nil {
		return c, fmt.Errorf("head query failed: %w", err)
	}
	c.Head = head
	age := time.Since(time.Unix(head.CloseTime, 0))
	if head.Sequence == 0 || head.ProtocolVersion < 28 || int(head.ProtocolVersion) != n.ProtocolVersion || head.CloseTime <= 0 || age > 60*time.Second || age < -10*time.Second {
		return c, errors.New("RPC ledger head is stale, invalid, or protocol-inconsistent")
	}
	c.Snapshot = fleetwatch.Snapshot{SchemaVersion: 1, Network: network, Contracts: []fleetwatch.Contract{}, References: []fleetwatch.Reference{}}
	keys := make([]string, 0, len(ids))
	for _, id := range ids {
		key, _ := InstanceKey(id)
		encoded, err := xdr.MarshalBase64(key)
		if err != nil {
			return c, err
		}
		keys = append(keys, encoded)
	}
	result, err := rpc.Entries(ctx, keys)
	if err != nil {
		return c, fmt.Errorf("instance query failed: %w", err)
	}
	c.Snapshot.Ledger = result.LatestLedger
	if result.LatestLedger < head.Sequence {
		c.Errors = append(c.Errors, "entry response predates sampled ledger head")
	}
	found := indexEntries(&c, result, keys)
	refs := map[string]fleetwatch.Executable{}
	for i, id := range ids {
		entry, ok := found[keys[i]]
		if !ok {
			c.Errors = append(c.Errors, "contract instance not observed: "+id)
			continue
		}
		contract, err := DecodeInstance(id, entry.XDR)
		if err != nil {
			c.Errors = append(c.Errors, "invalid contract instance: "+id)
			continue
		}
		c.Snapshot.Contracts = append(c.Snapshot.Contracts, contract)
		if contract.Executable.Kind == "external_ref" {
			ex := contract.Executable
			key, err := ReferenceKey(ex.Owner, ex.TagHex)
			if err != nil {
				c.Errors = append(c.Errors, "invalid external reference")
				continue
			}
			encoded, err := xdr.MarshalBase64(key)
			if err != nil {
				return c, err
			}
			refs[encoded] = ex
		}
	}
	refKeys := make([]string, 0, len(refs))
	for key := range refs {
		refKeys = append(refKeys, key)
	}
	sort.Strings(refKeys)
	if len(refKeys) > 0 {
		result, err := rpc.Entries(ctx, refKeys)
		if err != nil {
			c.Errors = append(c.Errors, "owner reference query failed")
		} else {
			if result.LatestLedger != c.Snapshot.Ledger {
				c.LedgerConsistency = "INCONSISTENT"
				c.Errors = append(c.Errors, "mixed response ledger heights")
			}
			found := indexEntries(&c, result, refKeys)
			for _, key := range refKeys {
				ex := refs[key]
				entry, ok := found[key]
				if !ok {
					c.Errors = append(c.Errors, "owner reference not observed")
					continue
				}
				ref, err := DecodeReference(ex.Owner, ex.TagHex, entry.XDR)
				if err != nil {
					c.Errors = append(c.Errors, "invalid owner reference value")
					continue
				}
				c.Snapshot.References = append(c.Snapshot.References, ref)
			}
		}
		// Equal latestLedger fields alone do not establish an atomic state view.
		c.Errors = append(c.Errors, "multi-call ledger consistency is not proven; capture is diagnostic")
	} else if len(c.Errors) == 0 && result.LatestLedger > 0 {
		c.Completeness = "COMPLETE"
		c.LedgerConsistency = "SINGLE_RESPONSE_CLAIM"
	}
	if result.LatestLedger == 0 {
		c.Errors = append(c.Errors, "RPC omitted response ledger")
	}
	return c, nil
}

func indexEntries(c *Capture, result Entries, requested []string) map[string]Entry {
	wanted := map[string]bool{}
	for _, key := range requested {
		wanted[key] = true
	}
	found := map[string]Entry{}
	if len(result.Entries) > len(requested) {
		c.Errors = append(c.Errors, "RPC returned excess entries")
		return found
	}
	duplicates := map[string]bool{}
	for _, e := range result.Entries {
		if !wanted[e.Key] {
			c.Errors = append(c.Errors, "RPC returned unsolicited key")
			continue
		}
		if _, ok := found[e.Key]; ok || duplicates[e.Key] {
			delete(found, e.Key)
			duplicates[e.Key] = true
			c.Errors = append(c.Errors, "RPC returned duplicate key")
			continue
		}
		if e.LastModified == 0 || e.LastModified > result.LatestLedger {
			c.Errors = append(c.Errors, "invalid last-modified ledger")
			continue
		}
		if len(e.XDR) > maxXDRBytes {
			c.Errors = append(c.Errors, "oversized entry XDR")
			continue
		}
		raw, err := base64.StdEncoding.DecodeString(e.XDR)
		if err != nil {
			c.Errors = append(c.Errors, "invalid entry encoding")
			continue
		}
		sum := sha256.Sum256(raw)
		c.Evidence = append(c.Evidence, Evidence{KeyXDR: e.Key, DataXDR: e.XDR, RawSHA256: hex.EncodeToString(sum[:]), ResponseLedger: result.LatestLedger, LastModified: e.LastModified})
		found[e.Key] = e
	}
	return found
}
