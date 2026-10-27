package history

import (
	"context"
	"encoding/hex"
	"encoding/json"
	"errors"
	"reflect"
	"sort"
	"strings"

	"github.com/soroban-fleetwatch/fleetwatch/internal/fleetwatch"
	"github.com/soroban-fleetwatch/fleetwatch/internal/source"
)

// Projection carries the complete executable reducer state between finite
// batches. It is not a new trusted baseline: its origin stays bound to the
// original operator assertion, and the store rederives it from retained XDR.
type Projection struct {
	SchemaVersion    int                    `json:"schema_version"`
	OriginDigest     string                 `json:"origin_digest"`
	OriginHeaderHash string                 `json:"origin_header_hash"`
	HeaderHash       string                 `json:"header_hash"`
	Snapshot         fleetwatch.Snapshot    `json:"snapshot"`
	CachedReferences []fleetwatch.Reference `json:"cached_references"`
	AbsentReferences []ReferenceKey         `json:"absent_references"`
}

type ReferenceKey struct {
	Owner  string `json:"owner"`
	TagHex string `json:"tag_hex"`
}

func NewProjection(b Baseline) (Projection, error) {
	if ValidateBaseline(b) != nil {
		return Projection{}, errors.New("invalid projection baseline")
	}
	digest, _ := fleetwatch.SnapshotDigest(b.Snapshot)
	return exportProjection(stateFromBaseline(b), b.Snapshot.Network, b.Snapshot.Ledger, digest, b.HeaderHash, b.HeaderHash)
}

func exportProjection(state projectedState, network string, ledger uint32, origin, anchor, tip string) (Projection, error) {
	snapshot, err := state.snapshot(network, ledger, nil)
	if err != nil {
		return Projection{}, err
	}
	p := Projection{SchemaVersion: 1, OriginDigest: origin, OriginHeaderHash: anchor, HeaderHash: tip, Snapshot: snapshot, CachedReferences: []fleetwatch.Reference{}, AbsentReferences: []ReferenceKey{}}
	for _, r := range state.refs {
		p.CachedReferences = append(p.CachedReferences, r)
	}
	for k := range state.absent {
		owner, tag, ok := strings.Cut(k, "/")
		if !ok {
			return Projection{}, errors.New("invalid absent key")
		}
		p.AbsentReferences = append(p.AbsentReferences, ReferenceKey{owner, tag})
	}
	sort.Slice(p.CachedReferences, func(i, j int) bool {
		return referenceID(p.CachedReferences[i].Owner, p.CachedReferences[i].TagHex) < referenceID(p.CachedReferences[j].Owner, p.CachedReferences[j].TagHex)
	})
	sort.Slice(p.AbsentReferences, func(i, j int) bool {
		return referenceID(p.AbsentReferences[i].Owner, p.AbsentReferences[i].TagHex) < referenceID(p.AbsentReferences[j].Owner, p.AbsentReferences[j].TagHex)
	})
	return p, nil
}

func projectionState(p Projection) (projectedState, error) {
	fail := func() (projectedState, error) { return projectedState{}, errors.New("invalid continuation projection") }
	if p.SchemaVersion != 1 || p.Snapshot.Validate() != nil || p.Snapshot.Ledger == 0 || len(p.CachedReferences)+len(p.AbsentReferences) > 10000 {
		return fail()
	}
	for _, h := range []string{p.OriginDigest, p.OriginHeaderHash, p.HeaderHash} {
		raw, err := hex.DecodeString(h)
		if err != nil || len(raw) != 32 || h != strings.ToLower(h) || h == strings.Repeat("0", 64) {
			return fail()
		}
	}
	ids := []string{}
	s := projectedState{contracts: map[string]fleetwatch.Contract{}, refs: map[string]fleetwatch.Reference{}, absent: map[string]bool{}}
	for _, c := range p.Snapshot.Contracts {
		ids = append(ids, c.ID)
		s.contracts[c.ID] = c
	}
	if source.ValidateWatchlist(ids) != nil {
		return fail()
	}
	last := ""
	for _, r := range p.CachedReferences {
		k := referenceID(r.Owner, r.TagHex)
		if k <= last || r.TagHex != strings.ToLower(r.TagHex) || r.WasmHash != strings.ToLower(r.WasmHash) {
			return fail()
		}
		last = k
		if _, err := source.ReferenceKey(r.Owner, r.TagHex); err != nil {
			return fail()
		}
		raw, err := hex.DecodeString(r.WasmHash)
		if err != nil || len(raw) != 32 {
			return fail()
		}
		s.refs[k] = r
	}
	last = ""
	for _, r := range p.AbsentReferences {
		k := referenceID(r.Owner, r.TagHex)
		if k <= last || r.TagHex != strings.ToLower(r.TagHex) {
			return fail()
		}
		last = k
		if _, err := source.ReferenceKey(r.Owner, r.TagHex); err != nil {
			return fail()
		}
		if _, present := s.refs[k]; present {
			return fail()
		}
		s.absent[k] = true
	}
	canonical, err := s.snapshot(p.Snapshot.Network, p.Snapshot.Ledger, nil)
	if err != nil || !reflect.DeepEqual(canonical, p.Snapshot) {
		return fail()
	}
	return s, nil
}

// Advance uses at most sixteen source ledgers per batch while preserving all
// known present/absent reference preimages and original event-ID provenance.
// On rejection it returns only the accepted prefix, never the rejected ledger.
func Advance(ctx context.Context, p Projection, c Capture) (Projection, Reconstruction, error) {
	state, err := projectionState(p)
	if err != nil {
		return Projection{}, Reconstruction{}, err
	}
	out := Reconstruction{SchemaVersion: 1, Kind: "watched_reconstruction", Source: "rpc_ledger_meta", Network: p.Snapshot.Network, BaselineDigest: p.OriginDigest, BaselineHeaderHash: p.OriginHeaderHash,
		Status: "PARTIAL", FleetScope: "EXPLICIT_BASELINE_WATCHLIST", MembershipCoverage: "UNKNOWN", BaselineAuthentication: "UNVERIFIED", SourceAuthentication: "UNVERIFIED", MetadataCommitment: "NOT_VERIFIED", HistoryCompleteness: "UNKNOWN", EvaluationStatus: "UNVERIFIED_PROJECTED_STATE", PreimageAssurance: "EXECUTABLE_FIELDS_ONLY", ImpactAssurance: "OBSERVED_MEMBERSHIP_UNION", FinalVerdict: "UNDETERMINED", Watchlist: []string{}, Ledgers: []AppliedLedger{}, Errors: []string{}, Snapshot: p.Snapshot, ProcessedThrough: p.Snapshot.Ledger}
	for _, contract := range p.Snapshot.Contracts {
		out.Watchlist = append(out.Watchlist, contract.ID)
	}
	fail := func(code string) (Projection, Reconstruction, error) {
		out.Errors = append(out.Errors, code)
		return p, out, errors.New(code)
	}
	if ValidateCaptureEnvelope(c) != nil || c.Network != p.Snapshot.Network || uint64(p.Snapshot.Ledger)+1 != uint64(c.FromLedger) {
		return fail("BASELINE_RANGE_OR_NETWORK_MISMATCH")
	}
	normalized, normalizeErr := Normalize(ctx, c)
	if ctx.Err() != nil {
		return fail("CANCELLED")
	}
	if len(normalized.Ledgers) > 0 && c.Ledgers[0].PreviousHash != p.HeaderHash {
		return fail("BASELINE_HEADER_LINK_MISMATCH")
	}
	bytes, events := 0, 0
	for _, ledger := range normalized.Ledgers {
		candidate := state.clone()
		applied, err := applyLedger(ctx, &candidate, c.Network, p.OriginDigest, p.OriginHeaderHash, ledger)
		if err != nil {
			return fail(err.Error())
		}
		if ctx.Err() != nil {
			return fail("CANCELLED")
		}
		raw, err := json.Marshal(applied)
		if err != nil || len(raw) > maxEvidenceBytes-bytes || len(applied.Events) > 10000-events {
			return fail("RECONSTRUCTION_LIMIT")
		}
		next, err := exportProjection(candidate, c.Network, ledger.Sequence, p.OriginDigest, p.OriginHeaderHash, ledger.HeaderHash)
		if err != nil {
			return fail("UNRESOLVED_WATCHED_REFERENCE")
		}
		bytes += len(raw)
		events += len(applied.Events)
		state = candidate
		p = next
		out.Snapshot = p.Snapshot
		out.ProcessedThrough = ledger.Sequence
		out.Ledgers = append(out.Ledgers, applied)
	}
	if normalizeErr != nil {
		return fail("NORMALIZATION_INCOMPLETE")
	}
	out.Status = "COMPLETE_CAPTURE_APPLICATION"
	return p, out, nil
}
