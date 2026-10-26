package history

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"reflect"
	"sort"
	"strings"

	"github.com/soroban-fleetwatch/fleetwatch/internal/fleetwatch"
	"github.com/soroban-fleetwatch/fleetwatch/internal/source"
)

type Baseline struct {
	SchemaVersion int                 `json:"schema_version"`
	Kind          string              `json:"kind"`
	Source        string              `json:"source"`
	HeaderHash    string              `json:"header_hash"`
	Snapshot      fleetwatch.Snapshot `json:"snapshot"`
}

type AppliedGroup struct {
	ID           string   `json:"id"`
	Position     Position `json:"position"`
	ChangeIDs    []string `json:"change_ids"`
	BeforeDigest string   `json:"before_digest"`
	AfterDigest  string   `json:"after_digest"`
}
type DerivedEvent struct {
	ID      string             `json:"id"`
	GroupID string             `json:"group_id"`
	Finding fleetwatch.Finding `json:"finding"`
}
type AppliedLedger struct {
	Sequence       uint32         `json:"sequence"`
	HeaderHash     string         `json:"header_hash"`
	MetadataSHA256 string         `json:"metadata_sha256"`
	Groups         []AppliedGroup `json:"groups"`
	Events         []DerivedEvent `json:"events"`
}
type Reconstruction struct {
	SchemaVersion          int                 `json:"schema_version"`
	Kind                   string              `json:"kind"`
	Source                 string              `json:"source"`
	Network                string              `json:"network"`
	BaselineDigest         string              `json:"baseline_digest"`
	BaselineHeaderHash     string              `json:"baseline_header_hash"`
	Watchlist              []string            `json:"watchlist"`
	ProcessedThrough       uint32              `json:"processed_through"`
	Status                 string              `json:"status"`
	FleetScope             string              `json:"fleet_scope"`
	MembershipCoverage     string              `json:"membership_coverage"`
	BaselineAuthentication string              `json:"baseline_authentication"`
	SourceAuthentication   string              `json:"source_authentication"`
	MetadataCommitment     string              `json:"metadata_commitment"`
	HistoryCompleteness    string              `json:"history_completeness"`
	EvaluationStatus       string              `json:"evaluation_status"`
	PreimageAssurance      string              `json:"preimage_assurance"`
	ImpactAssurance        string              `json:"impact_assurance"`
	FinalVerdict           string              `json:"final_verdict"`
	Snapshot               fleetwatch.Snapshot `json:"snapshot"`
	Ledgers                []AppliedLedger     `json:"ledgers"`
	Errors                 []string            `json:"errors"`
}

type projectedState struct {
	contracts map[string]fleetwatch.Contract
	refs      map[string]fleetwatch.Reference
	absent    map[string]bool
}

func referenceID(owner, tag string) string                { return owner + "/" + strings.ToLower(tag) }
func executableReference(ex fleetwatch.Executable) string { return referenceID(ex.Owner, ex.TagHex) }

func ValidateBaseline(b Baseline) error {
	raw, err := hex.DecodeString(b.HeaderHash)
	if b.SchemaVersion != 1 || b.Kind != "watched_baseline" || b.Source != "user_asserted" || err != nil || len(raw) != 32 || b.HeaderHash != strings.ToLower(b.HeaderHash) || b.HeaderHash == strings.Repeat("0", 64) || b.Snapshot.Validate() != nil || len(b.Snapshot.References) > 1000 {
		return errors.New("invalid watched baseline")
	}
	ids := []string{}
	for _, c := range b.Snapshot.Contracts {
		if c.ID != strings.ToUpper(c.ID) {
			return errors.New("baseline addresses must be canonical")
		}
		ids = append(ids, c.ID)
	}
	if err := source.ValidateWatchlist(ids); err != nil {
		return err
	}
	for _, r := range b.Snapshot.References {
		if r.Owner != strings.ToUpper(r.Owner) {
			return errors.New("baseline addresses must be canonical")
		}
		if _, err := source.ReferenceKey(r.Owner, r.TagHex); err != nil {
			return err
		}
	}
	s := stateFromBaseline(b)
	if _, err := s.snapshot(b.Snapshot.Network, b.Snapshot.Ledger, nil); err != nil {
		return err
	}
	return nil
}

func stateFromBaseline(b Baseline) projectedState {
	s := projectedState{contracts: map[string]fleetwatch.Contract{}, refs: map[string]fleetwatch.Reference{}, absent: map[string]bool{}}
	for _, c := range b.Snapshot.Contracts {
		c.Executable.TagHex = strings.ToLower(c.Executable.TagHex)
		c.Executable.WasmHash = strings.ToLower(c.Executable.WasmHash)
		s.contracts[c.ID] = c
	}
	for _, r := range b.Snapshot.References {
		r.TagHex = strings.ToLower(r.TagHex)
		r.WasmHash = strings.ToLower(r.WasmHash)
		s.refs[referenceID(r.Owner, r.TagHex)] = r
	}
	return s
}
func (s projectedState) clone() projectedState {
	c := projectedState{contracts: map[string]fleetwatch.Contract{}, refs: map[string]fleetwatch.Reference{}, absent: map[string]bool{}}
	for k, v := range s.contracts {
		c.contracts[k] = v
	}
	for k, v := range s.refs {
		c.refs[k] = v
	}
	for k, v := range s.absent {
		c.absent[k] = v
	}
	return c
}
func (s projectedState) usedReferences() map[string]bool {
	out := map[string]bool{}
	for _, c := range s.contracts {
		if c.Executable.Kind == "external_ref" {
			out[executableReference(c.Executable)] = true
		}
	}
	return out
}
func (s projectedState) snapshot(network string, ledger uint32, extra map[string]bool) (fleetwatch.Snapshot, error) {
	out := fleetwatch.Snapshot{SchemaVersion: 1, Network: network, Ledger: ledger, Contracts: []fleetwatch.Contract{}, References: []fleetwatch.Reference{}}
	used := s.usedReferences()
	for k := range used {
		if _, ok := s.refs[k]; !ok {
			return out, errors.New("UNRESOLVED_WATCHED_REFERENCE")
		}
	}
	for k := range extra {
		used[k] = true
	}
	for _, c := range s.contracts {
		out.Contracts = append(out.Contracts, c)
	}
	for k := range used {
		if r, ok := s.refs[k]; ok {
			out.References = append(out.References, r)
		}
	}
	sort.Slice(out.Contracts, func(i, j int) bool { return out.Contracts[i].ID < out.Contracts[j].ID })
	sort.Slice(out.References, func(i, j int) bool {
		return referenceID(out.References[i].Owner, out.References[i].TagHex) < referenceID(out.References[j].Owner, out.References[j].TagHex)
	})
	return out, nil
}

// Reconstruct applies an unverified baseline to revalidated raw metadata. Ledger
// candidates are isolated: any ambiguity/conflict discards that entire ledger.
// This is an offline diagnostic, not a persisted or authenticated chain cursor.
func Reconstruct(ctx context.Context, b Baseline, c Capture) (Reconstruction, error) {
	out := Reconstruction{SchemaVersion: 1, Kind: "watched_reconstruction", Source: "rpc_ledger_meta", Network: b.Snapshot.Network, BaselineHeaderHash: b.HeaderHash,
		Status: "PARTIAL", FleetScope: "EXPLICIT_BASELINE_WATCHLIST", MembershipCoverage: "UNKNOWN", BaselineAuthentication: "UNVERIFIED", SourceAuthentication: "UNVERIFIED", MetadataCommitment: "NOT_VERIFIED", HistoryCompleteness: "UNKNOWN", EvaluationStatus: "UNVERIFIED_PROJECTED_STATE", PreimageAssurance: "EXECUTABLE_FIELDS_ONLY", ImpactAssurance: "OBSERVED_MEMBERSHIP_UNION", FinalVerdict: "UNDETERMINED", Watchlist: []string{}, Ledgers: []AppliedLedger{}, Errors: []string{}}
	fail := func(code string) (Reconstruction, error) {
		out.Errors = append(out.Errors, code)
		return out, errors.New(code)
	}
	if ValidateBaseline(b) != nil {
		return Reconstruction{}, errors.New("INVALID_BASELINE")
	}
	out.BaselineDigest, _ = fleetwatch.SnapshotDigest(b.Snapshot)
	state := stateFromBaseline(b)
	out.Snapshot, _ = state.snapshot(c.Network, b.Snapshot.Ledger, nil)
	out.ProcessedThrough = b.Snapshot.Ledger
	for _, contract := range out.Snapshot.Contracts {
		out.Watchlist = append(out.Watchlist, contract.ID)
	}
	if ValidateCaptureEnvelope(c) != nil || b.Snapshot.Network != c.Network || uint64(b.Snapshot.Ledger)+1 != uint64(c.FromLedger) {
		return fail("BASELINE_RANGE_OR_NETWORK_MISMATCH")
	}
	normalized, normalizeErr := Normalize(ctx, c)
	if ctx.Err() != nil {
		return fail("CANCELLED")
	}
	if len(normalized.Ledgers) > 0 && c.Ledgers[0].PreviousHash != b.HeaderHash {
		return fail("BASELINE_HEADER_LINK_MISMATCH")
	}
	totalEvents, totalBytes := 0, 0
	for _, ledger := range normalized.Ledgers {
		if ctx.Err() != nil {
			return fail("CANCELLED")
		}
		candidate := state.clone()
		applied, err := applyLedger(ctx, &candidate, c.Network, out.BaselineDigest, b.HeaderHash, ledger)
		if err != nil {
			return fail(err.Error())
		}
		if ctx.Err() != nil {
			return fail("CANCELLED")
		}
		raw, err := json.Marshal(applied)
		if err != nil || len(raw) > maxEvidenceBytes-totalBytes || len(applied.Events) > 10000-totalEvents {
			return fail("RECONSTRUCTION_LIMIT")
		}
		final, err := candidate.snapshot(c.Network, ledger.Sequence, nil)
		if err != nil {
			return fail("UNRESOLVED_WATCHED_REFERENCE")
		}
		totalBytes += len(raw)
		totalEvents += len(applied.Events)
		state = candidate
		out.Snapshot = final
		out.ProcessedThrough = ledger.Sequence
		out.Ledgers = append(out.Ledgers, applied)
	}
	if normalizeErr != nil {
		return fail("NORMALIZATION_INCOMPLETE")
	}
	out.Status = "COMPLETE_CAPTURE_APPLICATION"
	return out, nil
}

func sameGroup(a, b Position) bool {
	a.ChangeIndex = 0
	b.ChangeIndex = 0
	return reflect.DeepEqual(a, b)
}
func applyLedger(ctx context.Context, state *projectedState, network, baselineDigest, baselineHash string, l LedgerChanges) (AppliedLedger, error) {
	out := AppliedLedger{Sequence: l.Sequence, HeaderHash: l.HeaderHash, MetadataSHA256: l.MetadataSHA256, Groups: []AppliedGroup{}, Events: []DerivedEvent{}}
	everUsed := state.usedReferences()
	for start := 0; start < len(l.Changes); {
		if ctx.Err() != nil {
			return AppliedLedger{}, errors.New("CANCELLED")
		}
		end := start + 1
		for end < len(l.Changes) && sameGroup(l.Changes[start].Position, l.Changes[end].Position) {
			end++
		}
		changes := l.Changes[start:end]
		oldUsed := state.usedReferences()
		before, err := state.snapshot(network, l.Sequence, nil)
		if err != nil {
			return AppliedLedger{}, err
		}
		if err := applyGroup(ctx, state, changes, oldUsed); err != nil {
			return AppliedLedger{}, err
		}
		after, err := state.snapshot(network, l.Sequence, oldUsed)
		if err != nil {
			return AppliedLedger{}, err
		}
		for k := range state.usedReferences() {
			everUsed[k] = true
		}
		comparison, err := fleetwatch.CompareWithinLedger(before, after)
		if err != nil {
			return AppliedLedger{}, errors.New("INVALID_PROJECTED_STATE")
		}
		group := AppliedGroup{Position: changes[0].Position, ChangeIDs: []string{}}
		for _, change := range changes {
			group.ChangeIDs = append(group.ChangeIDs, change.ID)
		}
		group.BeforeDigest, _ = fleetwatch.SnapshotDigest(before)
		projectedAfter, err := state.snapshot(network, l.Sequence, nil)
		if err != nil {
			return AppliedLedger{}, err
		}
		group.AfterDigest, _ = fleetwatch.SnapshotDigest(projectedAfter)
		payload, _ := json.Marshal(struct {
			Domain, Network, BaselineDigest, HeaderHash, MetadataSHA256 string
			Group                                                       AppliedGroup
		}{"fleetwatch.applied-group.v1", network, baselineDigest, l.HeaderHash, l.MetadataSHA256, group})
		h := sha256.Sum256(payload)
		group.ID = hex.EncodeToString(h[:])
		out.Groups = append(out.Groups, group)
		for _, finding := range comparison.Findings {
			payload, _ := json.Marshal(struct {
				Domain, Network, BaselineDigest, BaselineHash, HeaderHash, MetadataSHA256, GroupID string
				Finding                                                                            fleetwatch.Finding
			}{"fleetwatch.reconstruction-event.v1", network, baselineDigest, baselineHash, l.HeaderHash, l.MetadataSHA256, group.ID, finding})
			h := sha256.Sum256(payload)
			out.Events = append(out.Events, DerivedEvent{ID: hex.EncodeToString(h[:]), GroupID: group.ID, Finding: finding})
			if len(out.Events) > 10000 {
				return AppliedLedger{}, errors.New("RECONSTRUCTION_LIMIT")
			}
		}
		start = end
	}
	for _, eviction := range l.UnorderedEvictions {
		if _, watched := state.contracts[eviction.ContractID]; watched {
			return AppliedLedger{}, errors.New("UNORDERED_WATCHED_EVICTION")
		}
		key := referenceID(eviction.Owner, eviction.TagHex)
		if everUsed[key] {
			return AppliedLedger{}, errors.New("UNORDERED_WATCHED_EVICTION")
		}
		delete(state.refs, key)
		delete(state.absent, key)
	}
	return out, nil
}

func applyGroup(ctx context.Context, state *projectedState, changes []Change, oldUsed map[string]bool) error {
	preimages, mutated := map[string]bool{}, map[string]bool{}
	for _, c := range changes {
		if ctx.Err() != nil {
			return errors.New("CANCELLED")
		}
		contract, watched := state.contracts[c.ContractID]
		if c.ContractID != "" && !watched {
			continue
		}
		key := referenceID(c.Owner, c.TagHex)
		if c.Type == "STATE" {
			if watched {
				if c.Contract == nil || *c.Contract != contract {
					return errors.New("PREIMAGE_MISMATCH")
				}
			} else {
				if c.Reference == nil || state.absent[key] {
					return errors.New("PREIMAGE_MISMATCH")
				}
				if old, known := state.refs[key]; known && old != *c.Reference {
					return errors.New("PREIMAGE_MISMATCH")
				}
				state.refs[key] = *c.Reference
				if len(state.refs)+len(state.absent) > 10000 {
					return errors.New("RECONSTRUCTION_LIMIT")
				}
			}
			preimages[c.KeyXDR] = true
			continue
		}
		if c.Position.Phase == "FEE" || c.Position.Phase == "POST_FEE" {
			return errors.New("UNSUPPORTED_FEE_EXECUTABLE_MUTATION")
		}
		if mutated[c.KeyXDR] {
			return errors.New("AMBIGUOUS_GROUP_MUTATION_ORDER")
		}
		mutated[c.KeyXDR] = true
		switch c.Type {
		case "UPDATED":
			if !preimages[c.KeyXDR] {
				return errors.New("MISSING_GROUP_PREIMAGE")
			}
			if watched {
				if c.Contract == nil {
					return errors.New("INVALID_PROJECTED_STATE")
				}
				state.contracts[c.ContractID] = *c.Contract
			} else {
				if c.Reference == nil {
					return errors.New("INVALID_PROJECTED_STATE")
				}
				state.refs[key] = *c.Reference
				delete(state.absent, key)
			}
		case "CREATED", "RESTORED":
			if watched {
				return errors.New("ENTRY_ALREADY_PRESENT")
			}
			if _, exists := state.refs[key]; exists {
				return errors.New("ENTRY_ALREADY_PRESENT")
			}
			if c.Reference == nil {
				return errors.New("INVALID_PROJECTED_STATE")
			}
			state.refs[key] = *c.Reference
			delete(state.absent, key)
		case "REMOVED":
			if !preimages[c.KeyXDR] {
				return errors.New("MISSING_GROUP_PREIMAGE")
			}
			if watched {
				return errors.New("WATCHED_INSTANCE_UNAVAILABLE")
			}
			if oldUsed[key] {
				return errors.New("WATCHED_REFERENCE_UNAVAILABLE")
			}
			delete(state.refs, key)
			state.absent[key] = true
		default:
			return errors.New("UNSUPPORTED_PROJECTED_CHANGE")
		}
		delete(preimages, c.KeyXDR)
		if len(state.refs)+len(state.absent) > 10000 {
			return errors.New("RECONSTRUCTION_LIMIT")
		}
	}
	return nil
}
