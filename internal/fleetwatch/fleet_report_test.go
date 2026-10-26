package fleetwatch

import (
	"encoding/json"
	"reflect"
	"strings"
	"testing"
)

func fleetFixture() (FleetQuery, Snapshot, Snapshot) {
	q := FleetQuery{Network: "SYNTHETIC-NETWORK", Ledger: 101, Owner: "OWNER_DEMO", TagHex: "FF0061"}
	ex := Executable{Kind: "external_ref", Owner: q.Owner, TagHex: "ff0061"}
	before := Snapshot{SchemaVersion: 1, Network: q.Network, Ledger: 100,
		Contracts:  []Contract{{ID: "A", Executable: ex}, {ID: "B", Executable: ex}},
		References: []Reference{{Owner: q.Owner, TagHex: "ff0061", WasmHash: strings.Repeat("a", 64)}}}
	otherTag := ex
	otherTag.TagHex = "ff006100"
	otherOwner := ex
	otherOwner.Owner = "OTHER_OWNER"
	after := Snapshot{SchemaVersion: 1, Network: q.Network, Ledger: 101,
		Contracts:  []Contract{{ID: "C", Executable: ex}, {ID: "B", Executable: ex}, {ID: "WRONG_TAG", Executable: otherTag}, {ID: "WRONG_OWNER", Executable: otherOwner}, {ID: "DIRECT", Executable: Executable{Kind: "wasm", WasmHash: strings.Repeat("b", 64)}}},
		References: []Reference{{Owner: q.Owner, TagHex: "FF0061", WasmHash: strings.Repeat("B", 64)}}}
	return q, before, after
}

func TestDescribeFleetExactBytesMembershipAndEvidence(t *testing.T) {
	q, before, after := fleetFixture()
	r, err := DescribeFleet(q, &before, &after)
	if err != nil {
		t.Fatal(err)
	}
	want := &MembershipChange{Joined: []string{"C"}, Left: []string{"A"}, Retained: []string{"B"}, ObservedUnion: []string{"A", "B", "C"}}
	if !reflect.DeepEqual(r.MembershipChange, want) || r.TagHex != "ff0061" || *r.Current.WasmHash != strings.Repeat("b", 64) || *r.Previous.WasmHash != strings.Repeat("a", 64) {
		t.Fatalf("%+v", r)
	}
	digest, _ := SnapshotDigest(after)
	if r.Current.SnapshotDigest != digest || r.MembershipCoverage != "UNKNOWN" || r.FinalVerdict != "UNDETERMINED" || r.FleetScope != "OBSERVED_SNAPSHOT_MEMBERS" {
		t.Fatalf("%+v", r)
	}
	first, _ := json.Marshal(r)
	after.Contracts[0], after.Contracts[1] = after.Contracts[1], after.Contracts[0]
	q.TagHex = "ff0061"
	reordered, err := DescribeFleet(q, &before, &after)
	if err != nil {
		t.Fatal(err)
	}
	second, _ := json.Marshal(reordered)
	if string(first) != string(second) {
		t.Fatal("report changed with list order or hex representation")
	}
}

func TestDescribeFleetBaselineMissingAndUnresolved(t *testing.T) {
	q, _, after := fleetFixture()
	r, err := DescribeFleet(q, nil, nil)
	if err != nil || r.Current != nil || r.SnapshotStatus != "NOT_RETAINED" || r.RelatedEvents == nil {
		t.Fatalf("%+v %v", r, err)
	}
	r, err = DescribeFleet(q, nil, &after)
	if err != nil || r.Previous != nil || r.MembershipChange != nil {
		t.Fatalf("%+v %v", r, err)
	}
	after.References = nil
	r, err = DescribeFleet(q, nil, &after)
	if err != nil || r.Current.WasmHash != nil || r.Current.ReferenceStatus != "NOT_OBSERVED" || len(r.Current.Members) != 2 {
		t.Fatalf("%+v %v", r, err)
	}
	after.References = []Reference{{Owner: q.Owner, TagHex: q.TagHex, WasmHash: strings.Repeat("a", 64)}}
	after.Contracts = nil
	r, err = DescribeFleet(q, nil, &after)
	if err != nil || r.Current.ReferenceStatus != "OBSERVED" || len(r.Current.Members) != 0 || r.MembershipCoverage != "UNKNOWN" {
		t.Fatalf("%+v %v", r, err)
	}
}

func TestDescribeFleetRejectsInvalidOrNonadjacentInputs(t *testing.T) {
	q, before, after := fleetFixture()
	for _, mutate := range []func(*FleetQuery){
		func(q *FleetQuery) { q.Network = "" }, func(q *FleetQuery) { q.Ledger = 0 }, func(q *FleetQuery) { q.Owner = "owner/alias" },
		func(q *FleetQuery) { q.TagHex = "" }, func(q *FleetQuery) { q.TagHex = "0" }, func(q *FleetQuery) { q.TagHex = "zz" },
		func(q *FleetQuery) { q.TagHex = strings.Repeat("ff", MaxTagBytes+1) },
	} {
		bad := q
		mutate(&bad)
		if _, err := DescribeFleet(bad, nil, nil); err == nil {
			t.Fatalf("accepted %+v", bad)
		}
	}
	if _, err := DescribeFleet(q, &before, nil); err == nil {
		t.Fatal("accepted previous without current")
	}
	bad := after
	bad.Network = "OTHER_NETWORK"
	if _, err := DescribeFleet(q, &before, &bad); err == nil {
		t.Fatal("accepted wrong network")
	}
	bad = after
	bad.Ledger++
	if _, err := DescribeFleet(q, &before, &bad); err == nil {
		t.Fatal("interpolated current ledger")
	}
	before.Ledger--
	if _, err := DescribeFleet(q, &before, &after); err == nil {
		t.Fatal("accepted nonadjacent previous ledger")
	}
	before.Ledger = 100
	before.Network = "OTHER_NETWORK"
	if _, err := DescribeFleet(q, &before, &after); err == nil {
		t.Fatal("accepted wrong previous network")
	}
}
