package main

import (
	"bytes"
	"encoding/json"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/soroban-fleetwatch/fleetwatch/internal/fleetwatch"
	"github.com/soroban-fleetwatch/fleetwatch/internal/store"
)

func TestCLIDoctorCoverageDeliveryAndNoCreation(t *testing.T) {
	path := filepath.Join(t.TempDir(), "state.db")
	check := func(want int, network string) store.Health {
		t.Helper()
		var out, errs bytes.Buffer
		code := run([]string{"doctor", "--state", path, "--network", network}, &out, &errs)
		if code != want {
			t.Fatalf("code=%d want=%d out=%s err=%s", code, want, &out, &errs)
		}
		var h store.Health
		if want == 5 {
			if out.Len() != 0 {
				t.Fatal("emitted success report on failure")
			}
		} else if err := json.Unmarshal(out.Bytes(), &h); err != nil {
			t.Fatal(err)
		}
		return h
	}
	check(5, "TESTNET-SYNTHETIC")
	if _, err := os.Stat(path); !os.IsNotExist(err) {
		t.Fatal("doctor created database")
	}
	s, err := store.Open(path)
	if err != nil {
		t.Fatal(err)
	}
	j, err := fleetwatch.LoadJournal(fixture("replay-transient.json"))
	if err != nil {
		t.Fatal(err)
	}
	gap := j
	gap.Snapshots = gap.Snapshots[:1]
	if _, err := s.Replay(gap); err != nil {
		t.Fatal(err)
	}
	s.Close()
	check(4, j.Network)
	check(4, "another network")
	s, err = store.Open(path)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := s.Replay(j); err != nil {
		t.Fatal(err)
	}
	s.Close()
	h := check(0, j.Network)
	if h.Status != "OK_OFFLINE" || h.FinalVerdict != "UNDETERMINED" {
		t.Fatalf("%+v", h)
	}
	s, err = store.Open(path)
	if err != nil {
		t.Fatal(err)
	}
	now := time.Now()
	a, err := s.Claim(j.Network, j.Source, "test-sink", now, time.Minute)
	if err != nil || a == nil {
		t.Fatal(err)
	}
	if err := s.Resolve(j.Network, j.Source, a.ID, a.LeaseToken, false, true, now); err != nil {
		t.Fatal(err)
	}
	s.Close()
	check(2, j.Network)
	for _, args := range [][]string{{"doctor"}, {"doctor", "--state", path, "--network", j.Network, "--source", "rpc_current"}, {"doctor", "--state", path, "--network", j.Network, "extra"}} {
		var out, errs bytes.Buffer
		if code := run(args, &out, &errs); code != 3 || out.Len() != 0 {
			t.Fatalf("code=%d out=%s err=%s", code, &out, &errs)
		}
	}
}
