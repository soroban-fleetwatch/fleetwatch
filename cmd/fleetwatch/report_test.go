package main

import (
	"bytes"
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"testing"

	"github.com/soroban-fleetwatch/fleetwatch/internal/fleetwatch"
	"github.com/soroban-fleetwatch/fleetwatch/internal/store"
)

func TestCLIFleetReportExactLedgerAndScope(t *testing.T) {
	path := filepath.Join(t.TempDir(), "state.db")
	j, err := fleetwatch.LoadJournal(fixture("replay-transient.json"))
	if err != nil {
		t.Fatal(err)
	}
	s, err := store.Open(path)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := s.Replay(j); err != nil {
		t.Fatal(err)
	}
	s.Close()
	base := []string{"report", "--state", path, "--network", j.Network, "--ledger", "101", "--owner", "OWNER_DEMO", "--tag-hex", j.Snapshots[0].References[0].TagHex}
	for _, tc := range []struct {
		flag, value string
		code        int
		current     bool
	}{
		{"--ledger", "101", 0, true}, {"--ledger", "100", 0, true}, {"--ledger", "102", 0, true},
		{"--ledger", "99", 4, false}, {"--ledger", "103", 4, false}, {"--network", "OTHER_NETWORK", 4, false},
		{"--owner", "UNOBSERVED_OWNER", 4, true}, {"--tag-hex", "ff0061", 4, true},
	} {
		args := append([]string{}, base...)
		for i := 1; i < len(args); i += 2 {
			if args[i] == tc.flag {
				args[i+1] = tc.value
			}
		}
		var out, errs bytes.Buffer
		code := run(args, &out, &errs)
		var r fleetwatch.FleetReport
		if code != tc.code || errs.Len() != 0 || json.Unmarshal(out.Bytes(), &r) != nil {
			t.Fatalf("%v code=%d out=%s err=%s", tc, code, &out, &errs)
		}
		if (r.Current != nil) != tc.current || r.FinalVerdict != "UNDETERMINED" || r.MembershipCoverage != "UNKNOWN" {
			t.Fatalf("%+v", r)
		}
		if tc.flag == "--ledger" && tc.value == "101" && (len(r.Current.Members) != 2 || len(r.RelatedEvents) != 1 || r.Current.SnapshotDigest == "") {
			t.Fatalf("%+v", r)
		}
	}
	for _, tc := range []struct{ flag, value string }{{"--ledger", "0"}, {"--ledger", "-1"}, {"--ledger", "4294967296"}, {"--ledger", "18446744073709551616"}, {"--owner", "bad/owner"}, {"--tag-hex", ""}, {"--tag-hex", "xyz"}} {
		args := append([]string{}, base...)
		for i := 1; i < len(args); i += 2 {
			if args[i] == tc.flag {
				args[i+1] = tc.value
			}
		}
		var out, errs bytes.Buffer
		if code := run(args, &out, &errs); code != 3 || out.Len() != 0 {
			t.Fatalf("%v code=%d out=%s", tc, code, &out)
		}
	}
	var out, errs bytes.Buffer
	if code := run(append(append([]string{}, base...), "--source", "rpc_current"), &out, &errs); code != 3 {
		t.Fatal(code)
	}
	if code := run(append(append([]string{}, base...), "extra"), &out, &errs); code != 3 {
		t.Fatal(code)
	}
	before, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	if code := run(base, reportFailWriter{}, &errs); code != 5 {
		t.Fatal("output failure exit", code)
	}
	after, err := os.ReadFile(path)
	if err != nil || !bytes.Equal(before, after) {
		t.Fatal("output failure mutated database", err)
	}
	missing := filepath.Join(t.TempDir(), "missing.db")
	args := append([]string{}, base...)
	args[2] = missing
	out.Reset()
	if code := run(args, &out, &errs); code != 5 || out.Len() != 0 {
		t.Fatal("missing state", code)
	}
	if _, err := os.Stat(missing); !os.IsNotExist(err) {
		t.Fatal("report created state file")
	}
}

type reportFailWriter struct{}

func (reportFailWriter) Write(p []byte) (int, error) { return 0, errors.New("output unavailable") }

func TestCLIFleetReportGapUnresolvedAndEmptyObservedSet(t *testing.T) {
	for _, kind := range []string{"gap", "unresolved", "empty_members"} {
		t.Run(kind, func(t *testing.T) {
			j, err := fleetwatch.LoadJournal(fixture("replay-transient.json"))
			if err != nil {
				t.Fatal(err)
			}
			if kind == "gap" {
				j.Snapshots = j.Snapshots[:1]
			}
			if kind == "unresolved" {
				j.Snapshots[1].References = nil
			}
			if kind == "empty_members" {
				j.Snapshots[1].Contracts = nil
			}
			path := filepath.Join(t.TempDir(), "state.db")
			s, err := store.Open(path)
			if err != nil {
				t.Fatal(err)
			}
			if _, err := s.Replay(j); err != nil {
				t.Fatal(err)
			}
			s.Close()
			ledger := "101"
			if kind == "gap" {
				ledger = "100"
			}
			var out, errs bytes.Buffer
			code := run([]string{"report", "--state", path, "--network", j.Network, "--ledger", ledger, "--owner", "OWNER_DEMO", "--tag-hex", j.Snapshots[0].References[0].TagHex}, &out, &errs)
			want := 4
			if kind == "empty_members" {
				want = 0
			}
			var r fleetwatch.FleetReport
			if code != want || json.Unmarshal(out.Bytes(), &r) != nil || r.Current == nil {
				t.Fatalf("code=%d out=%s err=%s", code, &out, &errs)
			}
			if kind == "empty_members" && (len(r.Current.Members) != 0 || r.MembershipCoverage != "UNKNOWN") {
				t.Fatalf("%+v", r)
			}
			if kind == "unresolved" && (r.Current.WasmHash != nil || len(r.Current.Members) != 2) {
				t.Fatalf("%+v", r)
			}
		})
	}
}
