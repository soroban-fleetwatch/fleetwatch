package history

import (
	"context"
	"encoding/base64"
	"encoding/hex"
	"reflect"
	"strings"
	"testing"

	"github.com/soroban-fleetwatch/fleetwatch/internal/source"
	protocol "github.com/stellar/go-stellar-sdk/protocols/rpc"
	"github.com/stellar/go-stellar-sdk/strkey"
	"github.com/stellar/go-stellar-sdk/xdr"
)

func changeEntry(t testing.TB, hash byte) xdr.LedgerEntry {
	t.Helper()
	owner, _ := strkey.Encode(strkey.VersionByteContract, make([]byte, 32))
	key, err := source.ReferenceKey(owner, "ff0061")
	if err != nil {
		t.Fatal(err)
	}
	value := xdr.ScBytes(make([]byte, 32))
	for i := range value {
		value[i] = hash
	}
	return xdr.LedgerEntry{LastModifiedLedgerSeq: 90, Data: xdr.LedgerEntryData{Type: xdr.LedgerEntryTypeContractData, ContractData: &xdr.ContractDataEntry{Contract: key.ContractData.Contract, Key: key.ContractData.Key, Durability: key.ContractData.Durability, Val: xdr.ScVal{Type: xdr.ScValTypeScvBytes, Bytes: &value}}}}
}
func stateChange(entry xdr.LedgerEntry) xdr.LedgerEntryChange {
	return xdr.LedgerEntryChange{Type: xdr.LedgerEntryChangeTypeLedgerEntryState, State: &entry}
}
func updateChange(entry xdr.LedgerEntry) xdr.LedgerEntryChange {
	return xdr.LedgerEntryChange{Type: xdr.LedgerEntryChangeTypeLedgerEntryUpdated, Updated: &entry}
}

func changeCapture(t testing.TB) Capture {
	t.Helper()
	rpc := historyFixture(t)
	info := ledgerFixture(t, 100, xdr.Hash{}, 2)
	var meta xdr.LedgerCloseMeta
	if err := xdr.SafeUnmarshalBase64(info.LedgerMetadata, &meta); err != nil {
		t.Fatal(err)
	}
	a, b := changeEntry(t, 0xaa), changeEntry(t, 0xbb)
	key, _ := a.LedgerKey()
	meta.V2.TxProcessing = []xdr.TransactionResultMetaV1{
		{Result: testTxResult(1), FeeProcessing: xdr.LedgerEntryChanges{stateChange(a)},
			TxApplyProcessing: xdr.TransactionMeta{V: 4, V4: &xdr.TransactionMetaV4{TxChangesBefore: xdr.LedgerEntryChanges{stateChange(a)}, Operations: []xdr.OperationMetaV2{
				{Changes: xdr.LedgerEntryChanges{updateChange(b)}}, {Changes: xdr.LedgerEntryChanges{stateChange(b), updateChange(a)}},
			}, TxChangesAfter: xdr.LedgerEntryChanges{stateChange(a)}}}, PostTxApplyFeeProcessing: xdr.LedgerEntryChanges{stateChange(a)}},
		{Result: testTxResult(2), FeeProcessing: xdr.LedgerEntryChanges{stateChange(a)},
			TxApplyProcessing: xdr.TransactionMeta{V: 3, V3: &xdr.TransactionMetaV3{Operations: []xdr.OperationMeta{{Changes: xdr.LedgerEntryChanges{{Type: xdr.LedgerEntryChangeTypeLedgerEntryRestored, Restored: &a}}}}}}, PostTxApplyFeeProcessing: xdr.LedgerEntryChanges{stateChange(a)}},
	}
	version := xdr.Uint32(29)
	meta.V2.UpgradesProcessing = []xdr.UpgradeEntryMeta{{Upgrade: xdr.LedgerUpgrade{Type: xdr.LedgerUpgradeTypeLedgerUpgradeVersion, NewLedgerVersion: &version}, Changes: xdr.LedgerEntryChanges{{Type: xdr.LedgerEntryChangeTypeLedgerEntryRemoved, Removed: &key}}}}
	meta.V2.EvictedKeys = []xdr.LedgerKey{key}
	var err error
	info.LedgerMetadata, err = xdr.MarshalBase64(meta)
	if err != nil {
		t.Fatal(err)
	}
	rpc.infos = append(rpc.infos[:0], info)
	c, err := Fetch(context.Background(), rpc, rpc.network.Passphrase, 100, 100)
	if err != nil {
		t.Fatal(err)
	}
	return c
}

func TestNormalizeOrderRawTagsAndIntermediateWrites(t *testing.T) {
	c := changeCapture(t)
	r, err := Normalize(context.Background(), c)
	if err != nil || r.NormalizationStatus != "COMPLETE_CAPTURE" || r.ProcessedThrough != 100 || len(r.Ledgers) != 1 {
		t.Fatalf("%+v %v", r, err)
	}
	changes := r.Ledgers[0].Changes
	want := []string{"FEE", "FEE", "TX_BEFORE", "OPERATION", "OPERATION", "OPERATION", "TX_AFTER", "OPERATION", "POST_FEE", "POST_FEE", "UPGRADE"}
	if len(changes) != len(want) {
		t.Fatalf("%+v", changes)
	}
	seen := map[string]bool{}
	for i, ch := range changes {
		if ch.Position.Phase != want[i] || ch.TagHex != "ff0061" || ch.KeyXDR == "" || seen[ch.ID] {
			t.Fatalf("%d %+v", i, ch)
		}
		seen[ch.ID] = true
	}
	if *changes[0].Position.TransactionIndex != 0 || *changes[1].Position.TransactionIndex != 1 || *changes[3].Position.OperationIndex != 0 || *changes[5].Position.OperationIndex != 1 {
		t.Fatal("positions lost")
	}
	if changes[3].Type != "UPDATED" || changes[3].Reference.WasmHash != strings.Repeat("bb", 32) || changes[5].Reference.WasmHash != strings.Repeat("aa", 32) {
		t.Fatal("within-ledger B->A updates compacted")
	}
	if changes[4].Type != "STATE" || changes[7].Type != "RESTORED" || changes[10].EntryXDR != "" || changes[10].Type != "REMOVED" {
		t.Fatal("change semantics lost")
	}
	evicted := r.Ledgers[0].UnorderedEvictions
	if len(evicted) != 1 || evicted[0].Type != "EVICTED" || evicted[0].Position.Phase != "EVICTION_UNORDERED" || evicted[0].Position.TransactionIndex != nil {
		t.Fatal("fabricated eviction order")
	}
	if r.StateChanges != "NOT_APPLIED" || r.MetadataCommitment != "NOT_VERIFIED" || r.FinalVerdict != "UNDETERMINED" || r.MembershipCoverage != "UNKNOWN" {
		t.Fatal("false assurance")
	}
	again, err := Normalize(context.Background(), c)
	if err != nil || !reflect.DeepEqual(r, again) {
		t.Fatal("nondeterministic record identities", err)
	}
	c.Network = "ANOTHER-SYNTHETIC-NETWORK"
	other, err := Normalize(context.Background(), c)
	if err != nil || other.Ledgers[0].Changes[0].ID == changes[0].ID {
		t.Fatal("record ID not network scoped", err)
	}
}

func TestNormalizeEntryVariantsAndFiltering(t *testing.T) {
	entry := changeEntry(t, 0xaa)
	for _, kind := range []xdr.LedgerEntryChangeType{xdr.LedgerEntryChangeTypeLedgerEntryCreated, xdr.LedgerEntryChangeTypeLedgerEntryUpdated, xdr.LedgerEntryChangeTypeLedgerEntryState, xdr.LedgerEntryChangeTypeLedgerEntryRestored} {
		c := xdr.LedgerEntryChange{Type: kind}
		switch kind {
		case 0:
			c.Created = &entry
		case 1:
			c.Updated = &entry
		case 3:
			c.State = &entry
		case 4:
			c.Restored = &entry
		}
		got, relevant, err := normalizeChange(c, 100)
		if err != nil || !relevant || got.Reference == nil || got.EntryXDR == "" {
			t.Fatal(got, relevant, err)
		}
	}
	for _, ex := range []xdr.ContractExecutable{{Type: xdr.ContractExecutableTypeContractExecutableStellarAsset}, {Type: xdr.ContractExecutableTypeContractExecutableWasm, WasmHash: &xdr.Hash{1}}} {
		instance := changeEntry(t, 0xaa)
		d := instance.Data.ContractData
		d.Key = xdr.ScVal{Type: xdr.ScValTypeScvLedgerKeyContractInstance}
		d.Val = xdr.ScVal{Type: xdr.ScValTypeScvContractInstance, Instance: &xdr.ScContractInstance{Executable: ex}}
		got, relevant, err := normalizeChange(updateChange(instance), 100)
		if err != nil || !relevant || got.Contract == nil || got.ContractID == "" {
			t.Fatal(got, relevant, err)
		}
	}
	ordinary := changeEntry(t, 0xaa)
	ordinary.Data.ContractData.Key = xdr.ScVal{Type: xdr.ScValTypeScvBool, B: ptrBool(true)}
	if _, relevant, err := normalizeChange(updateChange(ordinary), 100); err != nil || relevant {
		t.Fatal("unrelated contract data included", err)
	}
	for _, kind := range []string{"future", "zero", "bad_value", "temporary", "empty_tag"} {
		bad := changeEntry(t, 0xaa)
		switch kind {
		case "future":
			bad.LastModifiedLedgerSeq = 101
		case "zero":
			bad.LastModifiedLedgerSeq = 0
		case "bad_value":
			bad.Data.ContractData.Val = xdr.ScVal{Type: xdr.ScValTypeScvVoid}
		case "temporary":
			bad.Data.ContractData.Durability = xdr.ContractDataDurabilityTemporary
		case "empty_tag":
			empty := xdr.ScString("")
			bad.Data.ContractData.Key.ExecutableTag = &empty
		}
		if _, _, err := normalizeChange(updateChange(bad), 100); err == nil {
			t.Fatal("invalid executable change accepted", kind)
		}
	}
}
func ptrBool(b bool) *bool { return &b }

func TestNormalizeCaptureValidationAndLedgerAtomicity(t *testing.T) {
	c := changeCapture(t)
	for _, mutate := range []func(*Capture){func(c *Capture) { c.SourceAuthentication = "VERIFIED" }, func(c *Capture) { c.ProcessedThrough++ }, func(c *Capture) { c.Ledgers[0].Sequence++ }, func(c *Capture) { c.Ledgers[0].MetadataSHA256 = strings.Repeat("0", 64) }} {
		bad := c
		bad.Ledgers = append([]Evidence{}, c.Ledgers...)
		mutate(&bad)
		if r, err := Normalize(context.Background(), bad); err == nil || len(r.Ledgers) != 0 || r.ProcessedThrough != 0 {
			t.Fatal("corrupt capture accepted", r, err)
		}
	}
	// The next ledger has a valid first change followed by an invalid one. None
	// of that ledger may be emitted even though the previous ledger succeeded.
	prevRaw, _ := hex.DecodeString(c.Ledgers[0].Hash)
	var prev xdr.Hash
	copy(prev[:], prevRaw)
	info := ledgerFixture(t, 101, prev, 2)
	var meta xdr.LedgerCloseMeta
	_ = xdr.SafeUnmarshalBase64(info.LedgerMetadata, &meta)
	good, bad := changeEntry(t, 0xbb), changeEntry(t, 0xaa)
	bad.LastModifiedLedgerSeq = 102
	meta.V2.TxProcessing = []xdr.TransactionResultMetaV1{{Result: testTxResult(3), TxApplyProcessing: xdr.TransactionMeta{V: 4, V4: &xdr.TransactionMetaV4{Operations: []xdr.OperationMetaV2{{Changes: xdr.LedgerEntryChanges{updateChange(good), updateChange(bad)}}}}}}}
	info.LedgerMetadata, _ = xdr.MarshalBase64(meta)
	e, err := Decode(info)
	if err != nil {
		t.Fatal(err)
	}
	c.ToLedger = 101
	c.ProcessedThrough = 101
	c.Ledgers = append(c.Ledgers, e)
	r, err := Normalize(context.Background(), c)
	if err == nil || r.ProcessedThrough != 100 || len(r.Ledgers) != 1 {
		t.Fatalf("ledger leaked on failure: %+v %v", r, err)
	}
	c.Ledgers = c.Ledgers[:1]
	c.ProcessedThrough = 100
	c.SequenceCoverage = "PARTIAL"
	c.Errors = []string{"LEDGER_QUERY_FAILED"}
	r, err = Normalize(context.Background(), c)
	if err == nil || len(r.Ledgers) != 1 || r.Errors[0] != "CAPTURE_INCOMPLETE" {
		t.Fatal(r, err)
	}
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	if r, err := Normalize(ctx, c); err == nil || len(r.Ledgers) != 0 || r.Errors[0] != "CANCELLED" {
		t.Fatal(r, err)
	}
}

func TestTransactionMetaVersionsAndChangeLimit(t *testing.T) {
	for _, meta := range []xdr.TransactionMeta{
		{V: 0, Operations: &[]xdr.OperationMeta{{Changes: xdr.LedgerEntryChanges{stateChange(changeEntry(t, 1))}}}},
		{V: 1, V1: &xdr.TransactionMetaV1{TxChanges: xdr.LedgerEntryChanges{stateChange(changeEntry(t, 1))}}},
		{V: 2, V2: &xdr.TransactionMetaV2{TxChangesBefore: xdr.LedgerEntryChanges{stateChange(changeEntry(t, 1))}}},
		{V: 3, V3: &xdr.TransactionMetaV3{TxChangesAfter: xdr.LedgerEntryChanges{stateChange(changeEntry(t, 1))}}},
		{V: 4, V4: &xdr.TransactionMetaV4{Operations: []xdr.OperationMetaV2{{Changes: xdr.LedgerEntryChanges{stateChange(changeEntry(t, 1))}}}}},
	} {
		before, ops, after, err := transactionGroups(meta)
		total := len(before) + len(after)
		for _, op := range ops {
			total += len(op)
		}
		if err != nil || total != 1 {
			t.Fatal(meta.V, err)
		}
	}
	if _, _, _, err := transactionGroups(xdr.TransactionMeta{V: 9}); err == nil {
		t.Fatal("unsupported transaction meta accepted")
	}
	c := changeCapture(t)
	var meta xdr.LedgerCloseMeta
	_ = xdr.SafeUnmarshalBase64(c.Ledgers[0].MetadataXDR, &meta)
	changes := make(xdr.LedgerEntryChanges, MaxNormalizedChanges+1)
	for i := range changes {
		changes[i] = stateChange(changeEntry(t, 1))
	}
	meta.V2.TxProcessing = []xdr.TransactionResultMetaV1{{Result: testTxResult(3), TxApplyProcessing: xdr.TransactionMeta{V: 4, V4: &xdr.TransactionMetaV4{TxChangesBefore: changes}}}}
	if r, err := normalizeLedger(context.Background(), c.Network, c.Ledgers[0], meta); err == nil || len(r.Changes) != 0 {
		t.Fatal("limit emitted partial ledger")
	}
}

func FuzzNormalizeMetadataCapture(f *testing.F) {
	c := changeCapture(f)
	seed, _ := base64.StdEncoding.DecodeString(c.Ledgers[0].MetadataXDR)
	f.Add(seed)
	f.Fuzz(func(t *testing.T, raw []byte) {
		if len(raw) > MaxMetadataBytes {
			return
		}
		candidate := c
		candidate.Ledgers = append([]Evidence{}, c.Ledgers...)
		original := candidate.Ledgers[0]
		validated, err := Decode(protocol.LedgerInfo{Sequence: original.Sequence, Hash: original.Hash, LedgerCloseTime: original.CloseTime, LedgerHeader: original.HeaderXDR, LedgerMetadata: base64.StdEncoding.EncodeToString(raw)})
		if err != nil {
			return
		}
		candidate.Ledgers[0] = validated
		_, _ = Normalize(context.Background(), candidate)
	})
}

func TestNormalizeLedgerMetaVariantsAndUnusedField(t *testing.T) {
	for _, version := range []int32{0, 1, 2} {
		info := ledgerFixture(t, 100, xdr.Hash{}, version)
		var meta xdr.LedgerCloseMeta
		if err := xdr.SafeUnmarshalBase64(info.LedgerMetadata, &meta); err != nil {
			t.Fatal(err)
		}
		apply := xdr.TransactionMeta{V: 1, V1: &xdr.TransactionMetaV1{TxChanges: xdr.LedgerEntryChanges{stateChange(changeEntry(t, 1))}}}
		tx := xdr.TransactionResultMeta{Result: testTxResult(1), TxApplyProcessing: apply}
		switch version {
		case 0:
			meta.V0.TxProcessing = []xdr.TransactionResultMeta{tx}
		case 1:
			meta.V1.TxProcessing = []xdr.TransactionResultMeta{tx}
		case 2:
			meta.V2.TxProcessing = []xdr.TransactionResultMetaV1{{Result: tx.Result, TxApplyProcessing: apply}}
		}
		info.LedgerMetadata, _ = xdr.MarshalBase64(meta)
		e, err := Decode(info)
		if err != nil {
			t.Fatal(err)
		}
		got, err := normalizeLedger(context.Background(), "SYNTHETIC-NETWORK", e, meta)
		if err != nil || len(got.Changes) != 1 || got.Changes[0].Type != "STATE" {
			t.Fatal(version, got, err)
		}
		if version == 1 {
			meta.V1.Unused = []xdr.LedgerEntry{changeEntry(t, 1)}
			if _, err := normalizeLedger(context.Background(), "SYNTHETIC-NETWORK", e, meta); err == nil {
				t.Fatal("silently ignored reserved unused entries")
			}
		}
	}
}

func testTxResult(n byte) xdr.TransactionResultPair {
	return xdr.TransactionResultPair{TransactionHash: xdr.Hash{n}, Result: xdr.TransactionResult{Result: xdr.TransactionResultResult{Code: xdr.TransactionResultCodeTxSuccess, Results: &[]xdr.OperationResult{}}}}
}
