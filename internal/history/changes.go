package history

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"

	"github.com/soroban-fleetwatch/fleetwatch/internal/fleetwatch"
	"github.com/soroban-fleetwatch/fleetwatch/internal/source"
	protocol "github.com/stellar/go-stellar-sdk/protocols/rpc"
	"github.com/stellar/go-stellar-sdk/xdr"
)

const MaxNormalizedChanges = 10000

type Position struct {
	Phase            string `json:"phase"`
	TransactionIndex *int   `json:"transaction_index"`
	OperationIndex   *int   `json:"operation_index"`
	UpgradeIndex     *int   `json:"upgrade_index"`
	ChangeIndex      int    `json:"change_index"`
}

type Change struct {
	ID              string                `json:"id,omitempty"`
	Position        Position              `json:"position"`
	TransactionHash string                `json:"transaction_hash,omitempty"`
	Type            string                `json:"type"`
	KeyXDR          string                `json:"key_xdr"`
	EntryXDR        string                `json:"entry_xdr,omitempty"`
	LastModified    uint32                `json:"last_modified,omitempty"`
	ContractID      string                `json:"contract_id,omitempty"`
	Owner           string                `json:"owner,omitempty"`
	TagHex          string                `json:"tag_hex,omitempty"`
	Contract        *fleetwatch.Contract  `json:"contract,omitempty"`
	Reference       *fleetwatch.Reference `json:"reference,omitempty"`
}

type LedgerChanges struct {
	Sequence           uint32   `json:"sequence"`
	HeaderHash         string   `json:"header_hash"`
	MetadataSHA256     string   `json:"metadata_sha256"`
	Changes            []Change `json:"changes"`
	UnorderedEvictions []Change `json:"unordered_evictions"`
}

type ChangeReport struct {
	SchemaVersion        int             `json:"schema_version"`
	Kind                 string          `json:"kind"`
	Source               string          `json:"source"`
	Network              string          `json:"network"`
	ProcessedThrough     uint32          `json:"processed_through"`
	NormalizationStatus  string          `json:"normalization_status"`
	FleetScope           string          `json:"fleet_scope"`
	MembershipCoverage   string          `json:"membership_coverage"`
	SourceAuthentication string          `json:"source_authentication"`
	MetadataCommitment   string          `json:"metadata_commitment"`
	HistoryCompleteness  string          `json:"history_completeness"`
	StateChanges         string          `json:"state_changes"`
	FinalVerdict         string          `json:"final_verdict"`
	Ledgers              []LedgerChanges `json:"ledgers"`
	Errors               []string        `json:"errors"`
}

// Normalize extracts CAP-85 reference/instance observations and mutations. It
// preserves STATE preimages and intermediate writes; it never applies a baseline
// or treats eviction-list position as transaction order. No database is written.
func Normalize(ctx context.Context, capture Capture) (ChangeReport, error) {
	out := ChangeReport{SchemaVersion: 1, Kind: "ledger_change_diagnostic", Source: "rpc_ledger_meta", Network: capture.Network,
		NormalizationStatus: "PARTIAL", FleetScope: "CHANGED_ENTRIES_ONLY", MembershipCoverage: "UNKNOWN", SourceAuthentication: "UNVERIFIED", MetadataCommitment: "NOT_VERIFIED", HistoryCompleteness: "UNKNOWN", StateChanges: "NOT_APPLIED", FinalVerdict: "UNDETERMINED", Ledgers: []LedgerChanges{}, Errors: []string{}}
	fail := func(code string) (ChangeReport, error) {
		out.Errors = append(out.Errors, code)
		return out, errors.New(code)
	}
	if err := ValidateCaptureEnvelope(capture); err != nil {
		return fail("INVALID_CAPTURE_ENVELOPE")
	}
	count, total, encodedInput := 0, 0, 0
	for i, e := range capture.Ledgers {
		if ctx.Err() != nil {
			return fail("CANCELLED")
		}
		encodedInput += len(e.HeaderXDR) + len(e.MetadataXDR)
		if encodedInput > maxEvidenceBytes {
			return fail("INPUT_BYTE_LIMIT")
		}
		info := protocol.LedgerInfo{Sequence: e.Sequence, Hash: e.Hash, LedgerCloseTime: e.CloseTime, LedgerHeader: e.HeaderXDR, LedgerMetadata: e.MetadataXDR}
		validated, err := Decode(info)
		if err != nil || validated != e {
			return fail("INVALID_LEDGER_EVIDENCE")
		}
		if i > 0 && e.PreviousHash != capture.Ledgers[i-1].Hash {
			return fail("HEADER_CHAIN_DIVERGENCE")
		}
		var meta xdr.LedgerCloseMeta
		if err := xdr.SafeUnmarshalBase64WithOptions(e.MetadataXDR, &meta, xdr.DecodeOptions{MaxDepth: 64, MaxMemoryBytes: 16 << 20}); err != nil {
			return fail("INVALID_LEDGER_EVIDENCE")
		}
		ledger, err := normalizeLedger(ctx, capture.Network, e, meta)
		if err != nil {
			if ctx.Err() != nil {
				return fail("CANCELLED")
			}
			return fail("INVALID_OR_UNSUPPORTED_CHANGES")
		}
		count += len(ledger.Changes) + len(ledger.UnorderedEvictions)
		raw, err := json.Marshal(ledger)
		if err != nil || len(raw) > maxEvidenceBytes-total || count > MaxNormalizedChanges {
			return fail("OUTPUT_LIMIT")
		}
		total += len(raw)
		out.Ledgers = append(out.Ledgers, ledger)
		out.ProcessedThrough = e.Sequence
	}
	if capture.SequenceCoverage != "COMPLETE_ENDPOINT_SEQUENCE" {
		return fail("CAPTURE_INCOMPLETE")
	}
	out.NormalizationStatus = "COMPLETE_CAPTURE"
	return out, nil
}

// ValidateCaptureEnvelope checks shape and declared coverage, not raw XDR evidence.
func ValidateCaptureEnvelope(c Capture) error {
	if ValidateRequest(c.Network, c.FromLedger, c.ToLedger) != nil || c.SchemaVersion != 1 || c.Kind != "ledger_metadata_capture" || c.Source != "rpc_ledger_meta" || c.SourceAuthentication != "UNVERIFIED" || c.HistoryCompleteness != "UNKNOWN" || c.MetadataCommitment != "NOT_VERIFIED" || c.StateChanges != "NOT_APPLIED" || c.FinalVerdict != "UNDETERMINED" || len(c.Ledgers) > MaxLedgers || len(c.Errors) > 1 {
		return errors.New("invalid capture envelope")
	}
	expected := uint64(c.FromLedger)
	for _, e := range c.Ledgers {
		if uint64(e.Sequence) != expected || e.Sequence > c.ToLedger {
			return errors.New("non-contiguous capture")
		}
		expected++
	}
	processed := uint32(0)
	if len(c.Ledgers) > 0 {
		processed = c.Ledgers[len(c.Ledgers)-1].Sequence
	}
	if c.ProcessedThrough != processed {
		return errors.New("capture cursor mismatch")
	}
	if c.SequenceCoverage == "COMPLETE_ENDPOINT_SEQUENCE" && expected == uint64(c.ToLedger)+1 && len(c.Errors) == 0 {
		return nil
	}
	if c.SequenceCoverage == "PARTIAL" && expected <= uint64(c.ToLedger) && len(c.Errors) == 1 {
		return nil
	}
	return errors.New("capture coverage mismatch")
}

type transactionChanges struct {
	hash  xdr.Hash
	fee   xdr.LedgerEntryChanges
	apply xdr.TransactionMeta
	post  xdr.LedgerEntryChanges
}

func normalizeLedger(ctx context.Context, network string, e Evidence, meta xdr.LedgerCloseMeta) (LedgerChanges, error) {
	out := LedgerChanges{Sequence: e.Sequence, HeaderHash: e.Hash, MetadataSHA256: e.MetadataSHA256, Changes: []Change{}, UnorderedEvictions: []Change{}}
	fail := func() (LedgerChanges, error) {
		return LedgerChanges{}, errors.New("unsupported or invalid ledger changes")
	}
	var txs []transactionChanges
	var upgrades []xdr.UpgradeEntryMeta
	var evictions []xdr.LedgerKey
	switch {
	case meta.V == 0 && meta.V0 != nil:
		for _, t := range meta.V0.TxProcessing {
			txs = append(txs, transactionChanges{hash: t.Result.TransactionHash, fee: t.FeeProcessing, apply: t.TxApplyProcessing})
		}
		upgrades = meta.V0.UpgradesProcessing
	case meta.V == 1 && meta.V1 != nil:
		if len(meta.V1.Unused) != 0 {
			return fail()
		}
		for _, t := range meta.V1.TxProcessing {
			txs = append(txs, transactionChanges{hash: t.Result.TransactionHash, fee: t.FeeProcessing, apply: t.TxApplyProcessing})
		}
		upgrades = meta.V1.UpgradesProcessing
		evictions = meta.V1.EvictedKeys
	case meta.V == 2 && meta.V2 != nil:
		for _, t := range meta.V2.TxProcessing {
			txs = append(txs, transactionChanges{hash: t.Result.TransactionHash, fee: t.FeeProcessing, apply: t.TxApplyProcessing, post: t.PostTxApplyFeeProcessing})
		}
		upgrades = meta.V2.UpgradesProcessing
		evictions = meta.V2.EvictedKeys
	default:
		return fail()
	}
	if len(txs) > 10000 || len(upgrades) > 10000 || len(evictions) > 10000 {
		return fail()
	}
	scanned, bytes := 0, 0
	appendChange := func(c xdr.LedgerEntryChange, p Position, hash string, eviction bool) error {
		if ctx.Err() != nil {
			return ctx.Err()
		}
		scanned++
		if scanned > 100000 {
			return errors.New("change scan limit")
		}
		record, relevant, err := normalizeChange(c, e.Sequence)
		if err != nil || !relevant {
			return err
		}
		record.Position = p
		record.TransactionHash = hash
		if eviction {
			record.Type = "EVICTED"
		}
		payload, _ := json.Marshal(struct {
			Domain, Network, HeaderHash, MetadataSHA256 string
			Change                                      Change
		}{"fleetwatch.metadata-change.v1", network, e.Hash, e.MetadataSHA256, record})
		sum := sha256.Sum256(payload)
		record.ID = hex.EncodeToString(sum[:])
		encoded, err := json.Marshal(record)
		if err != nil {
			return err
		}
		bytes += len(encoded)
		if bytes > maxEvidenceBytes || len(out.Changes)+len(out.UnorderedEvictions) >= MaxNormalizedChanges {
			return errors.New("normalized change limit")
		}
		if eviction {
			out.UnorderedEvictions = append(out.UnorderedEvictions, record)
		} else {
			out.Changes = append(out.Changes, record)
		}
		return nil
	}
	emit := func(changes xdr.LedgerEntryChanges, p Position, hash string) error {
		for i, c := range changes {
			p.ChangeIndex = i
			if err := appendChange(c, p, hash, false); err != nil {
				return err
			}
		}
		return nil
	}
	// SDK LedgerChangeReader explicitly applies ALL fee groups before any tx.
	for i, t := range txs {
		idx := i
		if err := emit(t.fee, Position{Phase: "FEE", TransactionIndex: &idx}, hex.EncodeToString(t.hash[:])); err != nil {
			return fail()
		}
	}
	for i, t := range txs {
		idx := i
		hash := hex.EncodeToString(t.hash[:])
		before, ops, after, err := transactionGroups(t.apply)
		if err != nil {
			return fail()
		}
		if err := emit(before, Position{Phase: "TX_BEFORE", TransactionIndex: &idx}, hash); err != nil {
			return fail()
		}
		for j, op := range ops {
			opIdx := j
			if err := emit(op, Position{Phase: "OPERATION", TransactionIndex: &idx, OperationIndex: &opIdx}, hash); err != nil {
				return fail()
			}
		}
		if err := emit(after, Position{Phase: "TX_AFTER", TransactionIndex: &idx}, hash); err != nil {
			return fail()
		}
	}
	for i, t := range txs {
		idx := i
		if err := emit(t.post, Position{Phase: "POST_FEE", TransactionIndex: &idx}, hex.EncodeToString(t.hash[:])); err != nil {
			return fail()
		}
	}
	for i, u := range upgrades {
		idx := i
		if err := emit(u.Changes, Position{Phase: "UPGRADE", UpgradeIndex: &idx}, ""); err != nil {
			return fail()
		}
	}
	for i, k := range evictions {
		if err := appendChange(xdr.LedgerEntryChange{Type: xdr.LedgerEntryChangeTypeLedgerEntryRemoved, Removed: &k}, Position{Phase: "EVICTION_UNORDERED", ChangeIndex: i}, "", true); err != nil {
			return fail()
		}
	}
	return out, nil
}

func transactionGroups(meta xdr.TransactionMeta) (xdr.LedgerEntryChanges, []xdr.LedgerEntryChanges, xdr.LedgerEntryChanges, error) {
	var before, after xdr.LedgerEntryChanges
	var ops []xdr.LedgerEntryChanges
	switch {
	case meta.V == 0 && meta.Operations != nil:
		for _, op := range *meta.Operations {
			ops = append(ops, op.Changes)
		}
	case meta.V == 1 && meta.V1 != nil:
		before = meta.V1.TxChanges
		for _, op := range meta.V1.Operations {
			ops = append(ops, op.Changes)
		}
	case meta.V == 2 && meta.V2 != nil:
		before = meta.V2.TxChangesBefore
		after = meta.V2.TxChangesAfter
		for _, op := range meta.V2.Operations {
			ops = append(ops, op.Changes)
		}
	case meta.V == 3 && meta.V3 != nil:
		before = meta.V3.TxChangesBefore
		after = meta.V3.TxChangesAfter
		for _, op := range meta.V3.Operations {
			ops = append(ops, op.Changes)
		}
	case meta.V == 4 && meta.V4 != nil:
		before = meta.V4.TxChangesBefore
		after = meta.V4.TxChangesAfter
		for _, op := range meta.V4.Operations {
			ops = append(ops, op.Changes)
		}
	default:
		return nil, nil, nil, errors.New("unsupported transaction meta")
	}
	if len(ops) > 10000 {
		return nil, nil, nil, errors.New("operation limit")
	}
	return before, ops, after, nil
}

func normalizeChange(c xdr.LedgerEntryChange, ledger uint32) (Change, bool, error) {
	var entry *xdr.LedgerEntry
	var key xdr.LedgerKey
	var err error
	out := Change{}
	switch c.Type {
	case xdr.LedgerEntryChangeTypeLedgerEntryCreated:
		entry = c.Created
		out.Type = "CREATED"
	case xdr.LedgerEntryChangeTypeLedgerEntryUpdated:
		entry = c.Updated
		out.Type = "UPDATED"
	case xdr.LedgerEntryChangeTypeLedgerEntryRemoved:
		if c.Removed == nil {
			return out, false, errors.New("missing removed key")
		}
		key = *c.Removed
		out.Type = "REMOVED"
	case xdr.LedgerEntryChangeTypeLedgerEntryState:
		entry = c.State
		out.Type = "STATE"
	case xdr.LedgerEntryChangeTypeLedgerEntryRestored:
		entry = c.Restored
		out.Type = "RESTORED"
	default:
		return out, false, errors.New("unknown change type")
	}
	if out.Type != "REMOVED" {
		if entry == nil {
			return out, false, errors.New("missing change entry")
		}
		key, err = entry.LedgerKey()
		if err != nil {
			return out, false, err
		}
	}
	if key.Type != xdr.LedgerEntryTypeContractData || key.ContractData == nil {
		return out, false, nil
	}
	k := key.ContractData
	if k.Key.Type != xdr.ScValTypeScvLedgerKeyContractInstance && k.Key.Type != xdr.ScValTypeScvExecutableTag {
		return out, false, nil
	}
	if k.Contract.Type != xdr.ScAddressTypeScAddressTypeContract || k.Durability != xdr.ContractDataDurabilityPersistent {
		return out, false, errors.New("invalid executable entry key")
	}
	address, err := k.Contract.String()
	if err != nil {
		return out, false, err
	}
	if k.Key.Type == xdr.ScValTypeScvLedgerKeyContractInstance {
		out.ContractID = address
	} else {
		if k.Key.ExecutableTag == nil {
			return out, false, errors.New("missing executable tag")
		}
		out.Owner = address
		out.TagHex = hex.EncodeToString([]byte(*k.Key.ExecutableTag))
		if _, err := source.ReferenceKey(out.Owner, out.TagHex); err != nil {
			return out, false, err
		}
	}
	out.KeyXDR, err = xdr.MarshalBase64(key)
	if err != nil {
		return out, false, err
	}
	if entry != nil {
		if entry.LastModifiedLedgerSeq == 0 || uint32(entry.LastModifiedLedgerSeq) > ledger {
			return out, false, errors.New("invalid entry modification ledger")
		}
		out.LastModified = uint32(entry.LastModifiedLedgerSeq)
		out.EntryXDR, err = xdr.MarshalBase64(*entry)
		if err != nil {
			return out, false, err
		}
		data, err := xdr.MarshalBase64(entry.Data)
		if err != nil {
			return out, false, err
		}
		if out.ContractID != "" {
			contract, err := source.DecodeInstance(out.ContractID, data)
			if err != nil {
				return out, false, err
			}
			out.Contract = &contract
		} else {
			ref, err := source.DecodeReference(out.Owner, out.TagHex, data)
			if err != nil {
				return out, false, err
			}
			out.Reference = &ref
		}
	}
	return out, true, nil
}
