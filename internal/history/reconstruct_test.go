package history

import (
	"context"
	"encoding/hex"
	"encoding/json"
	"reflect"
	"strings"
	"testing"

	"github.com/soroban-fleetwatch/fleetwatch/internal/fleetwatch"
	"github.com/soroban-fleetwatch/fleetwatch/internal/source"
	protocol "github.com/stellar/go-stellar-sdk/protocols/rpc"
	"github.com/stellar/go-stellar-sdk/strkey"
	"github.com/stellar/go-stellar-sdk/xdr"
)

func reconstructFixture(t testing.TB) (Baseline, Capture) {
	t.Helper()
	anchor := xdr.Hash{7}
	info := ledgerFixture(t, 100, anchor, 2)
	var meta xdr.LedgerCloseMeta
	_ = xdr.SafeUnmarshalBase64(info.LedgerMetadata, &meta)
	a, b := changeEntry(t, 0xaa), changeEntry(t, 0xbb)
	meta.V2.TxProcessing = []xdr.TransactionResultMetaV1{{Result: testTxResult(1), TxApplyProcessing: xdr.TransactionMeta{V: 4, V4: &xdr.TransactionMetaV4{Operations: []xdr.OperationMetaV2{
		{Changes: xdr.LedgerEntryChanges{stateChange(a), updateChange(b)}},
		{Changes: xdr.LedgerEntryChanges{stateChange(b), updateChange(a)}},
	}}}}}
	info.LedgerMetadata, _ = xdr.MarshalBase64(meta)
	rpc := historyFixture(t)
	rpc.infos = append(rpc.infos[:0], info)
	c, err := Fetch(context.Background(), rpc, rpc.network.Passphrase, 100, 100)
	if err != nil {
		t.Fatal(err)
	}
	owner, _ := strkey.Encode(strkey.VersionByteContract, make([]byte, 32))
	base := Baseline{SchemaVersion: 1, Kind: "watched_baseline", Source: "user_asserted", HeaderHash: hex.EncodeToString(anchor[:]), Snapshot: fleetwatch.Snapshot{SchemaVersion: 1, Network: c.Network, Ledger: 99, Contracts: []fleetwatch.Contract{}, References: []fleetwatch.Reference{{Owner: owner, TagHex: "FF0061", WasmHash: strings.Repeat("AA", 32)}}}}
	for _, n := range []byte{1, 2} {
		raw := make([]byte, 32)
		raw[0] = n
		id, _ := strkey.Encode(strkey.VersionByteContract, raw)
		base.Snapshot.Contracts = append(base.Snapshot.Contracts, fleetwatch.Contract{ID: id, Executable: fleetwatch.Executable{Kind: "external_ref", Owner: owner, TagHex: "FF0061"}})
	}
	return base, c
}

func mutateReconstruction(t testing.TB, c Capture, index int, fn func(*xdr.LedgerCloseMeta)) Capture {
	t.Helper()
	c.Ledgers = append([]Evidence{}, c.Ledgers...)
	e := c.Ledgers[index]
	var meta xdr.LedgerCloseMeta
	if err := xdr.SafeUnmarshalBase64(e.MetadataXDR, &meta); err != nil {
		t.Fatal(err)
	}
	fn(&meta)
	encoded, err := xdr.MarshalBase64(meta)
	if err != nil {
		t.Fatal(err)
	}
	e, err = Decode(protocol.LedgerInfo{Sequence: e.Sequence, Hash: e.Hash, LedgerCloseTime: e.CloseTime, LedgerHeader: e.HeaderXDR, LedgerMetadata: encoded})
	if err != nil {
		t.Fatal(err)
	}
	c.Ledgers[index] = e
	return c
}

func instanceProjectionEntry(t testing.TB, c fleetwatch.Contract) xdr.LedgerEntry {
	t.Helper()
	key, err := source.InstanceKey(c.ID)
	if err != nil {
		t.Fatal(err)
	}
	var ex xdr.ContractExecutable
	switch c.Executable.Kind {
	case "wasm":
		var h xdr.Hash
		raw, _ := hex.DecodeString(c.Executable.WasmHash)
		copy(h[:], raw)
		ex = xdr.ContractExecutable{Type: xdr.ContractExecutableTypeContractExecutableWasm, WasmHash: &h}
	case "external_ref":
		ownerKey, err := source.ReferenceKey(c.Executable.Owner, c.Executable.TagHex)
		if err != nil {
			t.Fatal(err)
		}
		tag, _ := hex.DecodeString(c.Executable.TagHex)
		ex = xdr.ContractExecutable{Type: xdr.ContractExecutableTypeContractExecutableExternalRef, ExternalRef: &xdr.ContractExecutableExternalRef{ExecutableOwner: ownerKey.ContractData.Contract, Tag: xdr.ScString(string(tag))}}
	case "stellar_asset":
		ex = xdr.ContractExecutable{Type: xdr.ContractExecutableTypeContractExecutableStellarAsset}
	}
	return xdr.LedgerEntry{LastModifiedLedgerSeq: 90, Data: xdr.LedgerEntryData{Type: xdr.LedgerEntryTypeContractData, ContractData: &xdr.ContractDataEntry{Contract: key.ContractData.Contract, Key: key.ContractData.Key, Durability: key.ContractData.Durability, Val: xdr.ScVal{Type: xdr.ScValTypeScvContractInstance, Instance: &xdr.ScContractInstance{Executable: ex}}}}}
}

func TestReconstructWithinLedgerTransientChanges(t *testing.T) {
	b, c := reconstructFixture(t)
	original, _ := json.Marshal([]any{b, c})
	r, err := Reconstruct(context.Background(), b, c)
	if err != nil || r.Status != "COMPLETE_CAPTURE_APPLICATION" || r.ProcessedThrough != 100 || len(r.Ledgers) != 1 || len(r.Ledgers[0].Events) != 2 {
		t.Fatalf("%+v %v", r, err)
	}
	for i, event := range r.Ledgers[0].Events {
		f := event.Finding
		if f.Code != "UNAPPROVED_REFERENCE_CHANGE" || f.Severity != "critical" || f.Authorization != "UNVERIFIED" || len(f.AffectedContracts) != 2 {
			t.Fatal(f)
		}
		old, newHash := strings.Repeat("aa", 32), strings.Repeat("bb", 32)
		if i == 1 {
			old, newHash = newHash, old
		}
		if f.OldHash != old || f.NewHash != newHash {
			t.Fatal("lost transient update", f)
		}
	}
	if r.Ledgers[0].Events[0].ID == r.Ledgers[0].Events[1].ID || r.Snapshot.References[0].WasmHash != strings.Repeat("aa", 32) || r.Snapshot.Ledger != 100 {
		t.Fatal("identity or final projection incorrect")
	}
	if r.FinalVerdict != "UNDETERMINED" || r.SourceAuthentication != "UNVERIFIED" || r.BaselineAuthentication != "UNVERIFIED" || r.PreimageAssurance != "EXECUTABLE_FIELDS_ONLY" {
		t.Fatal("false assurance")
	}
	again, err := Reconstruct(context.Background(), b, c)
	if err != nil || !reflect.DeepEqual(r, again) {
		t.Fatal("not deterministic", err)
	}
	b.Snapshot.Contracts[0], b.Snapshot.Contracts[1] = b.Snapshot.Contracts[1], b.Snapshot.Contracts[0]
	reordered, err := Reconstruct(context.Background(), b, c)
	if err != nil || !reflect.DeepEqual(r, reordered) {
		t.Fatal("baseline list order changed identity", err)
	}
	b.Snapshot.Contracts[0], b.Snapshot.Contracts[1] = b.Snapshot.Contracts[1], b.Snapshot.Contracts[0]
	after, _ := json.Marshal([]any{b, c})
	if string(original) != string(after) {
		t.Fatal("modified caller input")
	}
}

func TestReconstructRejectsConflictsAndAmbiguousOrder(t *testing.T) {
	for _, kind := range []string{"preimage", "missing", "ambiguous", "reference_removed", "instance_removed", "restore_present", "unresolved", "fee_mutation", "eviction"} {
		t.Run(kind, func(t *testing.T) {
			b, c := reconstructFixture(t)
			c = mutateReconstruction(t, c, 0, func(m *xdr.LedgerCloseMeta) {
				a, bad := changeEntry(t, 0xaa), changeEntry(t, 0xbb)
				op := &m.V2.TxProcessing[0].TxApplyProcessing.V4.Operations[0]
				switch kind {
				case "preimage":
					op.Changes[0] = stateChange(bad)
				case "missing":
					op.Changes = op.Changes[1:]
				case "ambiguous":
					op.Changes = append(op.Changes, stateChange(bad), updateChange(a))
				case "reference_removed":
					k, _ := a.LedgerKey()
					op.Changes = xdr.LedgerEntryChanges{stateChange(a), {Type: xdr.LedgerEntryChangeTypeLedgerEntryRemoved, Removed: &k}}
				case "instance_removed":
					entry := instanceProjectionEntry(t, b.Snapshot.Contracts[0])
					k, _ := entry.LedgerKey()
					op.Changes = xdr.LedgerEntryChanges{stateChange(entry), {Type: xdr.LedgerEntryChangeTypeLedgerEntryRemoved, Removed: &k}}
				case "restore_present":
					op.Changes = xdr.LedgerEntryChanges{{Type: xdr.LedgerEntryChangeTypeLedgerEntryRestored, Restored: &a}}
				case "unresolved":
					old := instanceProjectionEntry(t, b.Snapshot.Contracts[0])
					next := b.Snapshot.Contracts[0]
					next.Executable.TagHex = "ff006100"
					op.Changes = xdr.LedgerEntryChanges{stateChange(old), updateChange(instanceProjectionEntry(t, next))}
				case "fee_mutation":
					m.V2.TxProcessing[0].FeeProcessing = xdr.LedgerEntryChanges{stateChange(a), updateChange(bad)}
				case "eviction":
					k, _ := a.LedgerKey()
					m.V2.EvictedKeys = []xdr.LedgerKey{k}
				}
			})
			r, err := Reconstruct(context.Background(), b, c)
			if err == nil || r.ProcessedThrough != 99 || len(r.Ledgers) != 0 || r.Snapshot.References[0].WasmHash != strings.Repeat("aa", 32) || r.Status != "PARTIAL" {
				t.Fatalf("bad ledger leaked: %+v %v", r, err)
			}
		})
	}
}

func TestReconstructMembershipAndNewReferenceScope(t *testing.T) {
	b, c := reconstructFixture(t)
	newRef := changeEntry(t, 0xcc)
	tag := xdr.ScString(string([]byte{0xff, 0, 0x61, 0}))
	newRef.Data.ContractData.Key.ExecutableTag = &tag
	c = mutateReconstruction(t, c, 0, func(m *xdr.LedgerCloseMeta) {
		old := instanceProjectionEntry(t, b.Snapshot.Contracts[0])
		next := b.Snapshot.Contracts[0]
		next.Executable.TagHex = "ff006100"
		op := &m.V2.TxProcessing[0].TxApplyProcessing.V4.Operations[0]
		op.Changes = append(op.Changes, stateChange(newRef), stateChange(old), updateChange(instanceProjectionEntry(t, next)))
		// An unrelated instance appears in metadata, but is not added to the watchlist.
		raw := make([]byte, 32)
		raw[0] = 3
		id, _ := strkey.Encode(strkey.VersionByteContract, raw)
		unknown := next
		unknown.ID = id
		entry := instanceProjectionEntry(t, unknown)
		op.Changes = append(op.Changes, xdr.LedgerEntryChange{Type: xdr.LedgerEntryChangeTypeLedgerEntryCreated, Created: &entry})
	})
	r, err := Reconstruct(context.Background(), b, c)
	if err != nil {
		t.Fatal(err)
	}
	var transitions []fleetwatch.Finding
	for _, e := range r.Ledgers[0].Events {
		if e.Finding.Code == "UNAPPROVED_REFERENCE_CHANGE" {
			transitions = append(transitions, e.Finding)
		}
	}
	if len(transitions) != 2 || len(transitions[0].AffectedContracts) != 2 || len(transitions[1].AffectedContracts) != 1 || transitions[1].AffectedContracts[0] != b.Snapshot.Contracts[1].ID {
		t.Fatalf("wrong membership union: %+v", transitions)
	}
	if len(r.Watchlist) != 2 || len(r.Snapshot.Contracts) != 2 || len(r.Snapshot.References) != 2 || r.ImpactAssurance != "OBSERVED_MEMBERSHIP_UNION" {
		t.Fatal("scope expanded", r)
	}
	groups := r.Ledgers[0].Groups
	for i := 1; i < len(groups); i++ {
		if groups[i-1].AfterDigest != groups[i].BeforeDigest {
			t.Fatal("active projection digest discontinuity")
		}
	}
}

func TestReconstructRestoreBeforeRepointAndUnwatchedEviction(t *testing.T) {
	b, c := reconstructFixture(t)
	c = mutateReconstruction(t, c, 0, func(m *xdr.LedgerCloseMeta) {
		ref := changeEntry(t, 0xcc)
		tag := xdr.ScString(string([]byte{0xff, 0, 0x61, 0}))
		ref.Data.ContractData.Key.ExecutableTag = &tag
		ops := m.V2.TxProcessing[0].TxApplyProcessing.V4.Operations
		ops[0].Changes = append(ops[0].Changes, xdr.LedgerEntryChange{Type: xdr.LedgerEntryChangeTypeLedgerEntryRestored, Restored: &ref})
		old := instanceProjectionEntry(t, b.Snapshot.Contracts[0])
		next := b.Snapshot.Contracts[0]
		next.Executable.TagHex = "ff006100"
		ops[1].Changes = append(ops[1].Changes, stateChange(old), updateChange(instanceProjectionEntry(t, next)))
		owner := b.Snapshot.References[0].Owner
		k, err := source.ReferenceKey(owner, "ee")
		if err != nil {
			t.Fatal(err)
		}
		m.V2.EvictedKeys = []xdr.LedgerKey{k}
	})
	r, err := Reconstruct(context.Background(), b, c)
	if err != nil || r.Status != "COMPLETE_CAPTURE_APPLICATION" || len(r.Snapshot.References) != 2 {
		t.Fatal(r, err)
	}
	for _, ref := range r.Snapshot.References {
		if ref.TagHex == "ff006100" && ref.WasmHash != strings.Repeat("cc", 32) {
			t.Fatal("restored value lost")
		}
	}
}

func TestReconstructBaselineGapsAndLaterLedgerRollback(t *testing.T) {
	b, c := reconstructFixture(t)
	wrong := b
	wrong.HeaderHash = strings.Repeat("1", 64)
	if r, err := Reconstruct(context.Background(), wrong, c); err == nil || r.ProcessedThrough != 99 || r.Errors[0] != "BASELINE_HEADER_LINK_MISMATCH" {
		t.Fatal(r, err)
	}
	wrong = b
	wrong.Snapshot.Ledger = 98
	if _, err := Reconstruct(context.Background(), wrong, c); err == nil {
		t.Fatal("interpolated missing ledger")
	}
	wrong = b
	wrong.Snapshot.Network = "WRONG-NETWORK"
	if _, err := Reconstruct(context.Background(), wrong, c); err == nil {
		t.Fatal("cross-network baseline accepted")
	}
	raw, _ := hex.DecodeString(c.Ledgers[0].Hash)
	var prev xdr.Hash
	copy(prev[:], raw)
	info := ledgerFixture(t, 101, prev, 2)
	e, err := Decode(info)
	if err != nil {
		t.Fatal(err)
	}
	c.ToLedger = 101
	c.ProcessedThrough = 101
	c.Ledgers = append(c.Ledgers, e)
	r, err := Reconstruct(context.Background(), b, c)
	if err != nil || r.ProcessedThrough != 101 || len(r.Ledgers) != 2 {
		t.Fatal(r, err)
	}
	bad := mutateReconstruction(t, c, 1, func(m *xdr.LedgerCloseMeta) {
		op := xdr.OperationMetaV2{Changes: xdr.LedgerEntryChanges{stateChange(changeEntry(t, 0xbb)), updateChange(changeEntry(t, 0xaa))}}
		apply := xdr.TransactionMeta{V: 4, V4: &xdr.TransactionMetaV4{Operations: []xdr.OperationMetaV2{op}}}
		m.V2.TxProcessing = []xdr.TransactionResultMetaV1{{Result: testTxResult(2), TxApplyProcessing: apply}}
	})
	r, err = Reconstruct(context.Background(), b, bad)
	if err == nil || r.ProcessedThrough != 100 || len(r.Ledgers) != 1 || len(r.Ledgers[0].Events) != 2 {
		t.Fatal("prior ledger lost", r, err)
	}
	c.Ledgers = c.Ledgers[:1]
	c.ProcessedThrough = 100
	c.SequenceCoverage = "PARTIAL"
	c.Errors = []string{"LEDGER_QUERY_FAILED"}
	r, err = Reconstruct(context.Background(), b, c)
	if err == nil || r.ProcessedThrough != 100 || len(r.Ledgers) != 1 || r.Errors[0] != "NORMALIZATION_INCOMPLETE" {
		t.Fatal(r, err)
	}
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	if r, err := Reconstruct(ctx, b, c); err == nil || len(r.Ledgers) != 0 {
		t.Fatal("cancelled application advanced", r, err)
	}
}

func TestValidateWatchedBaseline(t *testing.T) {
	b, _ := reconstructFixture(t)
	for _, mutate := range []func(*Baseline){func(b *Baseline) { b.Source = "verified" }, func(b *Baseline) { b.HeaderHash = strings.Repeat("0", 64) }, func(b *Baseline) { b.Snapshot.References = nil }, func(b *Baseline) { b.Snapshot.Contracts[0].ID = "SYNTHETIC_LABEL" }, func(b *Baseline) { b.Snapshot.Contracts = nil }} {
		raw, _ := json.Marshal(b)
		var bad Baseline
		_ = json.Unmarshal(raw, &bad)
		mutate(&bad)
		if ValidateBaseline(bad) == nil {
			t.Fatal("invalid baseline accepted")
		}
	}
}
