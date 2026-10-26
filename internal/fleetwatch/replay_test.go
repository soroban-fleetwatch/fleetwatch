package fleetwatch

import (
	"encoding/json"
	"math"
	"reflect"
	"testing"
)

func journal(states ...Snapshot) Journal {
	return Journal{SchemaVersion: 1, Source: "synthetic", Network: "TESTNET", FromLedger: 100, ToLedger: 102, Snapshots: states}
}
func TestReplayCapturesTransientChangesAndIsIdempotent(t *testing.T) {
	j := journal(sample(100, hashA), sample(101, hashB), sample(102, hashA))
	first, err := ReplayJournal(j)
	if err != nil {
		t.Fatal(err)
	}
	if len(first.Events) != 2 || first.Events[0].Ledger != 101 || first.Events[1].Ledger != 102 || first.Events[0].ID == first.Events[1].ID || first.SequenceCoverage != "COMPLETE_INPUT_SEQUENCE" {
		t.Fatalf("%+v", first)
	}
	if first.Events[1].Finding.NewHash != hashA || first.EvaluationStatus != "UNVERIFIED_INPUT" {
		t.Fatal("wrong replay assurance or final state")
	}
	// Harmless list order and repeated same-ledger observations do not change IDs.
	dup := sample(101, hashB)
	dup.Contracts[0], dup.Contracts[1] = dup.Contracts[1], dup.Contracts[0]
	j.Snapshots = []Snapshot{sample(100, hashA), sample(101, hashB), dup, sample(102, hashA)}
	second, err := ReplayJournal(j)
	if err != nil || !reflect.DeepEqual(first, second) {
		t.Fatalf("replay not idempotent: %v", err)
	}
	other := j
	other.Network = "other network"
	other.Snapshots = nil
	for _, s := range j.Snapshots {
		s.Network = other.Network
		other.Snapshots = append(other.Snapshots, s)
	}
	third, err := ReplayJournal(other)
	if err != nil || third.Events[0].ID == first.Events[0].ID {
		t.Fatal("event identity not network scoped")
	}
}
func TestReplayStopsAtEveryGapBoundary(t *testing.T) {
	for _, tc := range []struct {
		states              []Snapshot
		processed, from, to uint32
	}{{nil, 0, 100, 102}, {[]Snapshot{sample(101, hashA)}, 0, 100, 100}, {[]Snapshot{sample(100, hashA), sample(102, hashB)}, 100, 101, 101}, {[]Snapshot{sample(100, hashA)}, 100, 101, 102}} {
		r, err := ReplayJournal(journal(tc.states...))
		if err != nil {
			t.Fatal(err)
		}
		if r.ProcessedThrough != tc.processed || len(r.Events) != 0 || len(r.Issues) != 1 || r.Issues[0].FromLedger != tc.from || r.Issues[0].ToLedger != tc.to || r.SequenceCoverage != "PARTIAL" {
			t.Fatalf("%+v", r)
		}
	}
}
func TestReplayQuarantinesConflictingLedger(t *testing.T) {
	r, err := ReplayJournal(journal(sample(100, hashA), sample(101, hashB), sample(101, hashA), sample(102, hashA)))
	if err != nil || r.ProcessedThrough != 100 || len(r.Events) != 0 || len(r.Issues) != 1 || r.Issues[0].Code != "SOURCE_DIVERGENCE" {
		t.Fatalf("%+v %v", r, err)
	}
}
func TestReplayRejectsInvalidOrderingNetworkAndRange(t *testing.T) {
	for _, j := range []Journal{journal(sample(101, hashA), sample(100, hashB)), journal(sample(99, hashA)), journal(sample(103, hashA)), {SchemaVersion: 1, Source: "ledger_meta", Network: "TESTNET", FromLedger: 100, ToLedger: 102}, {SchemaVersion: 1, Source: "synthetic", Network: "TESTNET", FromLedger: 1, ToLedger: math.MaxUint32}} {
		if _, err := ReplayJournal(j); err == nil {
			t.Fatal("invalid journal accepted")
		}
	}
	bad := sample(100, hashA)
	bad.Network = "other"
	if _, err := ReplayJournal(journal(bad)); err == nil {
		t.Fatal("cross-network observation accepted")
	}
}
func TestReplayUint32BoundaryAndNoMutation(t *testing.T) {
	s := sample(math.MaxUint32, hashA)
	j := Journal{SchemaVersion: 1, Source: "synthetic", Network: s.Network, FromLedger: s.Ledger, ToLedger: s.Ledger, Snapshots: []Snapshot{s}}
	before, _ := json.Marshal(j)
	r, err := ReplayJournal(j)
	after, _ := json.Marshal(j)
	if err != nil || r.ProcessedThrough != math.MaxUint32 || r.SequenceCoverage != "COMPLETE_INPUT_SEQUENCE" || string(before) != string(after) {
		t.Fatalf("%+v %v", r, err)
	}
}
