package store

import (
	"bytes"
	"encoding/json"
	"os"
	"path/filepath"
	"testing"

	"github.com/soroban-fleetwatch/fleetwatch/internal/fleetwatch"
	bolt "go.etcd.io/bbolt"
)

func TestFleetReportReopenEvidenceAndRelatedRepoints(t *testing.T) {
	path := filepath.Join(t.TempDir(), "state.db")
	s := openTest(t, path)
	j := fixture(t)
	j.ToLedger = 101
	j.Snapshots = j.Snapshots[:2]
	// A leaves, C joins and B stays while the shared hash changes. D is unrelated.
	j.Snapshots[1].Contracts[0].Executable = j.Snapshots[0].Contracts[2].Executable
	j.Snapshots[1].Contracts[2].Executable = j.Snapshots[0].Contracts[0].Executable
	j.Snapshots[1].Contracts = append(j.Snapshots[1].Contracts, fleetwatch.Contract{ID: "UNRELATED_D", Executable: j.Snapshots[0].Contracts[2].Executable})
	r, err := s.Replay(j)
	if err != nil {
		t.Fatal(err)
	}
	s.Close()
	before, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	s, err = OpenReadOnly(path)
	if err != nil {
		t.Fatal(err)
	}
	q := fleetwatch.FleetQuery{Network: j.Network, Ledger: 101, Owner: "OWNER_DEMO", TagHex: j.Snapshots[1].References[0].TagHex}
	f, err := s.ReportFleet(q, j.Source)
	if err != nil || !f.StateFound || f.Source != j.Source || len(f.RelatedEvents) != 3 || len(f.MembershipChange.ObservedUnion) != 3 {
		t.Fatalf("%+v %v", f, err)
	}
	if len(f.MembershipChange.Joined) != 1 || f.MembershipChange.Joined[0] != "CONTRACT_DEMO_C" || f.MembershipChange.Left[0] != "CONTRACT_DEMO_A" {
		t.Fatalf("%+v", f.MembershipChange)
	}
	want := map[string]bool{}
	for _, e := range r.Events {
		if e.Finding.ContractID != "UNRELATED_D" {
			want[e.ID] = true
		}
	}
	for _, e := range f.RelatedEvents {
		if !want[e.ID] {
			t.Fatal("event lacks retained replay evidence")
		}
	}
	q.Ledger = 100
	f, err = s.ReportFleet(q, j.Source)
	if err != nil || f.Previous != nil || f.MembershipChange != nil || len(f.RelatedEvents) != 0 {
		t.Fatalf("%+v %v", f, err)
	}
	s.Close()
	after, err := os.ReadFile(path)
	if err != nil || !bytes.Equal(before, after) {
		t.Fatal("report mutated store", err)
	}
}

func TestFleetReportGapScopeAndExactLedger(t *testing.T) {
	s := openTest(t, filepath.Join(t.TempDir(), "state.db"))
	defer s.Close()
	j := fixture(t)
	gap := j
	gap.Snapshots = gap.Snapshots[:1]
	if _, err := s.Replay(gap); err != nil {
		t.Fatal(err)
	}
	q := fleetwatch.FleetQuery{Network: j.Network, Ledger: 102, Owner: "OWNER_DEMO", TagHex: j.Snapshots[0].References[0].TagHex}
	f, err := s.ReportFleet(q, j.Source)
	if err != nil || f.Current != nil || f.SnapshotStatus != "NOT_RETAINED" || len(f.InputIssues) != 1 {
		t.Fatalf("%+v %v", f, err)
	}
	q.Ledger = 100
	f, err = s.ReportFleet(q, j.Source)
	if err != nil || f.Current == nil || f.SequenceCoverage != "PARTIAL" {
		t.Fatalf("%+v %v", f, err)
	}
	for _, ledger := range []uint32{99, 103} {
		q.Ledger = ledger
		f, err = s.ReportFleet(q, j.Source)
		if err != nil || f.Current != nil {
			t.Fatalf("%+v %v", f, err)
		}
	}
	q.Ledger = 100
	f, err = s.ReportFleet(q, "user_snapshot")
	if err != nil || f.StateFound || f.Current != nil || f.SequenceCoverage != "UNKNOWN" {
		t.Fatalf("%+v %v", f, err)
	}
	q.Network = "OTHER_NETWORK"
	f, err = s.ReportFleet(q, j.Source)
	if err != nil || f.StateFound || f.Current != nil {
		t.Fatalf("%+v %v", f, err)
	}
}

func TestFleetReportRejectsCorruptFindings(t *testing.T) {
	s := openTest(t, filepath.Join(t.TempDir(), "state.db"))
	defer s.Close()
	j := fixture(t)
	if _, err := s.Replay(j); err != nil {
		t.Fatal(err)
	}
	if err := s.db.Update(func(tx *bolt.Tx) error {
		b := tx.Bucket(stateBucket)
		var state State
		if err := json.Unmarshal(b.Get(scope(j.Network, j.Source)), &state); err != nil {
			return err
		}
		state.Report.Events[0].Finding.AffectedContracts = []string{"FORGED_MEMBER"}
		raw, _ := json.Marshal(state)
		return b.Put(scope(j.Network, j.Source), raw)
	}); err != nil {
		t.Fatal(err)
	}
	q := fleetwatch.FleetQuery{Network: j.Network, Ledger: 101, Owner: "OWNER_DEMO", TagHex: j.Snapshots[0].References[0].TagHex}
	f, err := s.ReportFleet(q, j.Source)
	if err == nil || f.SchemaVersion != 0 {
		t.Fatalf("corrupt state returned report: %+v %v", f, err)
	}
}
