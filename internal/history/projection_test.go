package history

import (
	"context"
	"encoding/json"
	"reflect"
	"testing"

	"github.com/soroban-fleetwatch/fleetwatch/internal/fleetwatch"
	"github.com/stellar/go-stellar-sdk/xdr"
)

func TestProjectionMatchesLegacyAndPreservesProvenance(t *testing.T) {
	b, c := reconstructFixture(t)
	legacy, err := Reconstruct(context.Background(), b, c)
	if err != nil {
		t.Fatal(err)
	}
	p, err := NewProjection(b)
	if err != nil {
		t.Fatal(err)
	}
	p, report, err := Advance(context.Background(), p, c)
	if err != nil {
		t.Fatal(err)
	}
	a, _ := json.Marshal(legacy)
	z, _ := json.Marshal(report)
	if string(a) != string(z) {
		t.Fatal("continuation changed legacy event IDs/report semantics")
	}
	if p.OriginDigest != legacy.BaselineDigest || p.HeaderHash != c.Ledgers[0].Hash || p.Snapshot.Ledger != 100 {
		t.Fatal("invalid continued origin/head")
	}
	if _, err := projectionState(p); err != nil {
		t.Fatal(err)
	}
}

func TestProjectionCarriesInactiveAndAbsentKnowledge(t *testing.T) {
	b, _ := reconstructFixture(t)
	p, err := NewProjection(b)
	if err != nil {
		t.Fatal(err)
	}
	s, err := projectionState(p)
	if err != nil {
		t.Fatal(err)
	}
	ref := p.CachedReferences[0]
	ref.TagHex = "01"
	s.refs[referenceID(ref.Owner, ref.TagHex)] = ref
	s.absent[referenceID(ref.Owner, "02")] = true
	p, err = exportProjection(s, p.Snapshot.Network, 100, p.OriginDigest, p.OriginHeaderHash, p.HeaderHash)
	if err != nil {
		t.Fatal(err)
	}
	roundTrip, err := projectionState(p)
	if err != nil || !reflect.DeepEqual(s, roundTrip) {
		t.Fatal("continuation forgot inactive preimages or absence", err)
	}
	if len(p.Snapshot.References) != 1 || len(p.CachedReferences) != 2 || len(p.AbsentReferences) != 1 {
		t.Fatal("cache was confused with active membership")
	}
	bad := p
	bad.CachedReferences = append([]fleetwatch.Reference{}, p.CachedReferences...)
	bad.CachedReferences[1] = bad.CachedReferences[0]
	if _, err := projectionState(bad); err == nil {
		t.Fatal("duplicate continuation key accepted")
	}
	bad = p
	bad.AbsentReferences = append([]ReferenceKey{}, p.AbsentReferences...)
	bad.AbsentReferences[0] = ReferenceKey{ref.Owner, "01"}
	if _, err := projectionState(bad); err == nil {
		t.Fatal("present/absent contradiction accepted")
	}
}

func TestProjectionRejectsInactiveAndAbsentPreimages(t *testing.T) {
	b, c := reconstructFixture(t)
	p, err := NewProjection(b)
	if err != nil {
		t.Fatal(err)
	}
	s, err := projectionState(p)
	if err != nil {
		t.Fatal(err)
	}
	ref := p.CachedReferences[0]
	ref.TagHex = "01"
	s.refs[referenceID(ref.Owner, ref.TagHex)] = ref
	entry := changeEntry(t, 0xbb)
	tag := xdr.ScString(string([]byte{1}))
	entry.Data.ContractData.Key.ExecutableTag = &tag
	c = mutateReconstruction(t, c, 0, func(m *xdr.LedgerCloseMeta) {
		m.V2.TxProcessing[0].TxApplyProcessing.V4.Operations = []xdr.OperationMetaV2{{Changes: xdr.LedgerEntryChanges{stateChange(entry)}}}
	})
	for _, absent := range []bool{false, true} {
		if absent {
			delete(s.refs, referenceID(ref.Owner, "01"))
			s.absent[referenceID(ref.Owner, "01")] = true
		}
		continued, err := exportProjection(s, p.Snapshot.Network, p.Snapshot.Ledger, p.OriginDigest, p.OriginHeaderHash, p.HeaderHash)
		if err != nil {
			t.Fatal(err)
		}
		next, report, err := Advance(context.Background(), continued, c)
		if err == nil || len(report.Errors) != 1 || report.Errors[0] != "PREIMAGE_MISMATCH" || next.Snapshot.Ledger != p.Snapshot.Ledger {
			t.Fatal("inactive/absent preimage was forgotten", report, err)
		}
	}
}
