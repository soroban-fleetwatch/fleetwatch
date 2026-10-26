package fleetwatch

import "testing"

func TestCompareWithinLedgerKeepsSnapshotChronologyStrict(t *testing.T) {
	before, err := LoadSnapshot("../../testdata/before.json")
	if err != nil {
		t.Fatal(err)
	}
	after, err := LoadSnapshot("../../testdata/after.json")
	if err != nil {
		t.Fatal(err)
	}
	after.Ledger = before.Ledger
	if _, err := Compare(before, after, Policy{SchemaVersion: 1}); err == nil {
		t.Fatal("ordinary compare accepted same ledger")
	}
	r, err := CompareWithinLedger(before, after)
	if err != nil || len(r.Findings) == 0 || r.BeforeLedger != r.AfterLedger || r.Coverage != "UNKNOWN" {
		t.Fatal(r, err)
	}
	for _, f := range r.Findings {
		if f.Authorization != "UNVERIFIED" {
			t.Fatal("false authorization")
		}
	}
	after.Ledger++
	if _, err := CompareWithinLedger(before, after); err == nil {
		t.Fatal("operation comparison accepted mismatched ledgers")
	}
}
