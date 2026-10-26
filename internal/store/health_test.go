package store

import (
	"bytes"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	bolt "go.etcd.io/bbolt"
)

func TestHealthReadOnlyScopeAndUnchangedDatabase(t *testing.T) {
	path := filepath.Join(t.TempDir(), "state.db")
	s := openTest(t, path)
	j := fixture(t)
	if _, err := s.Replay(j); err != nil {
		t.Fatal(err)
	}
	s.Close()
	before, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	r, err := OpenReadOnly(path)
	if err != nil {
		t.Fatal(err)
	}
	h, err := r.Health(j.Network, j.Source, time.Unix(1000, 0))
	if err != nil || h.Status != "OK_OFFLINE" || h.ProcessedThrough != 102 || h.Outbox.Pending != 2 || h.Outbox.Due != 2 || h.LedgerLag != nil || h.FinalVerdict != "UNDETERMINED" || h.ChainHistory != "UNKNOWN" {
		t.Fatalf("%+v %v", h, err)
	}
	other, err := r.Health(j.Network, "user_snapshot", time.Unix(1000, 0))
	if err != nil || other.StateFound || other.Status != "NO_STATE" || other.Outbox.Total != 0 {
		t.Fatalf("%+v %v", other, err)
	}
	if _, err := r.Replay(j); err == nil {
		t.Fatal("read-only store accepted replay")
	}
	r.Close()
	after, err := os.ReadFile(path)
	if err != nil || !bytes.Equal(before, after) {
		t.Fatal("health changed database bytes", err)
	}
}

func TestHealthGapRepairRetainsHistoricalAlerts(t *testing.T) {
	s := openTest(t, filepath.Join(t.TempDir(), "state.db"))
	defer s.Close()
	j := fixture(t)
	gap := j
	gap.Snapshots = nil
	if _, err := s.Replay(gap); err != nil {
		t.Fatal(err)
	}
	missing, err := s.Health(j.Network, j.Source, time.Unix(1000, 0))
	if err != nil || missing.ProcessedThrough != 0 || missing.Status != "NEEDS_ATTENTION" {
		t.Fatalf("missing baseline: %+v %v", missing, err)
	}
	gap.Snapshots = j.Snapshots[:1]
	if _, err := s.Replay(gap); err != nil {
		t.Fatal(err)
	}
	h, err := s.Health(j.Network, j.Source, time.Unix(1000, 0))
	if err != nil || h.Status != "NEEDS_ATTENTION" || len(h.Issues) != 1 || h.Outbox.Total != 2 {
		t.Fatalf("%+v %v", h, err)
	}
	if _, err := s.Replay(j); err != nil {
		t.Fatal(err)
	}
	h, err = s.Health(j.Network, j.Source, time.Unix(1000, 0))
	if err != nil || h.Status != "OK_OFFLINE" || len(h.Issues) != 0 || h.Outbox.Total != 4 {
		t.Fatalf("%+v %v", h, err)
	}
}

func TestHealthLeaseRetryAndDeadLetterBoundaries(t *testing.T) {
	s := openTest(t, filepath.Join(t.TempDir(), "state.db"))
	defer s.Close()
	j := fixture(t)
	if _, err := s.Replay(j); err != nil {
		t.Fatal(err)
	}
	now := time.Unix(1000, 0)
	a, err := s.Claim(j.Network, j.Source, "sink", now, time.Second)
	if err != nil || a == nil {
		t.Fatal(err)
	}
	h, err := s.Health(j.Network, j.Source, now)
	if err != nil || h.Outbox.Inflight != 1 || h.Outbox.ExpiredLeases != 0 || h.Status != "OK_OFFLINE" {
		t.Fatalf("%+v %v", h, err)
	}
	h, err = s.Health(j.Network, j.Source, now.Add(time.Second))
	if err != nil || h.Outbox.ExpiredLeases != 1 || h.Status != "NEEDS_ATTENTION" {
		t.Fatalf("%+v %v", h, err)
	}
	if err := s.Resolve(j.Network, j.Source, a.ID, a.LeaseToken, false, false, now); err != nil {
		t.Fatal(err)
	}
	h, err = s.Health(j.Network, j.Source, now)
	if err != nil || h.Outbox.Scheduled != 1 || h.Outbox.Due != 1 {
		t.Fatalf("%+v %v", h, err)
	}
	// Deliver the other event, then retry this one through its durable limit.
	b, err := s.Claim(j.Network, j.Source, "sink", now, time.Second)
	if err != nil || b == nil {
		t.Fatal(err)
	}
	if err := s.Resolve(j.Network, j.Source, b.ID, b.LeaseToken, true, false, now); err != nil {
		t.Fatal(err)
	}
	for attempt := 2; attempt <= MaxDeliveryAttempts; attempt++ {
		now = now.Add(time.Minute)
		a, err = s.Claim(j.Network, j.Source, "sink", now, time.Second)
		if err != nil || a == nil {
			t.Fatal(err)
		}
		if attempt == MaxDeliveryAttempts {
			h, err = s.Health(j.Network, j.Source, now)
			if err != nil || h.Outbox.ExhaustedUnsettled != 0 {
				t.Fatalf("%+v %v", h, err)
			}
			h, err = s.Health(j.Network, j.Source, now.Add(time.Second))
			if err != nil || h.Outbox.ExhaustedUnsettled != 1 {
				t.Fatalf("%+v %v", h, err)
			}
		}
		if err := s.Resolve(j.Network, j.Source, a.ID, a.LeaseToken, false, false, now); err != nil {
			t.Fatal(err)
		}
	}
	h, err = s.Health(j.Network, j.Source, now)
	if err != nil || h.Outbox.Dead != 1 || h.Outbox.Delivered != 1 || h.Outbox.ExhaustedUnsettled != 0 || h.Status != "NEEDS_ATTENTION" {
		t.Fatalf("%+v %v", h, err)
	}
	raw, _ := json.Marshal(h)
	if strings.Contains(string(raw), "lease_token") || strings.Contains(string(raw), a.LeaseToken) || strings.Contains(string(raw), "sink") {
		t.Fatal("health leaked delivery metadata")
	}
}

func TestHealthRejectsCorruptionWithoutPartialReport(t *testing.T) {
	for _, kind := range []string{"coverage", "missing_event", "event_payload", "missing_issue", "issue_identity"} {
		t.Run(kind, func(t *testing.T) {
			s := openTest(t, filepath.Join(t.TempDir(), "state.db"))
			defer s.Close()
			j := fixture(t)
			if kind == "missing_issue" || kind == "issue_identity" {
				j.Snapshots = j.Snapshots[:1]
			}
			if _, err := s.Replay(j); err != nil {
				t.Fatal(err)
			}
			if err := s.db.Update(func(tx *bolt.Tx) error {
				if kind == "coverage" {
					b := tx.Bucket(stateBucket)
					var state State
					if err := json.Unmarshal(b.Get(scope(j.Network, j.Source)), &state); err != nil {
						return err
					}
					state.Report.SequenceCoverage = "SAFE"
					raw, _ := json.Marshal(state)
					return b.Put(scope(j.Network, j.Source), raw)
				}
				b, err := queue(tx, j.Network, j.Source, false)
				if err != nil {
					return err
				}
				k, raw := b.Cursor().First()
				if kind == "missing_event" || kind == "missing_issue" {
					return b.Delete(k)
				}
				var a Alert
				if err := json.Unmarshal(raw, &a); err != nil {
					return err
				}
				if kind == "event_payload" {
					a.Payload.Event.Finding.Message = "altered"
				} else {
					a.Payload.Issue.ToLedger++
				}
				return writeAlert(b, a)
			}); err != nil {
				t.Fatal(err)
			}
			h, err := s.Health(j.Network, j.Source, time.Unix(1000, 0))
			if err == nil || h.SchemaVersion != 0 {
				t.Fatalf("corruption yielded report: %+v %v", h, err)
			}
		})
	}
}

func TestReadOnlyOpenRejectsMissingSymlinkEmptyForeignAndLocked(t *testing.T) {
	dir := t.TempDir()
	missing := filepath.Join(dir, "missing.db")
	if s, err := OpenReadOnly(missing); err == nil {
		s.Close()
		t.Fatal("created missing db")
	}
	if _, err := os.Stat(missing); !os.IsNotExist(err) {
		t.Fatal("missing path was created")
	}
	empty := filepath.Join(dir, "empty.db")
	if err := os.WriteFile(empty, nil, 0600); err != nil {
		t.Fatal(err)
	}
	if s, err := OpenReadOnly(empty); err == nil {
		s.Close()
		t.Fatal("accepted empty db")
	}
	foreign := filepath.Join(dir, "foreign.db")
	f, err := bolt.Open(foreign, 0600, nil)
	if err != nil {
		t.Fatal(err)
	}
	f.Close()
	if s, err := OpenReadOnly(foreign); err == nil {
		s.Close()
		t.Fatal("accepted foreign db")
	}
	path := filepath.Join(dir, "valid.db")
	s := openTest(t, path)
	if r, err := OpenReadOnly(path); err == nil {
		r.Close()
		t.Fatal("accepted locked db")
	}
	s.Close()
	link := filepath.Join(dir, "link.db")
	if err := os.Symlink(path, link); err != nil {
		t.Fatal(err)
	}
	if r, err := OpenReadOnly(link); err == nil {
		r.Close()
		t.Fatal("accepted symlink")
	}
}
