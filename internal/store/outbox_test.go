package store

import (
	"errors"
	"path/filepath"
	"testing"
	"time"
)

func TestOutboxCommitsWithReplayAndDeduplicates(t *testing.T) {
	s := openTest(t, filepath.Join(t.TempDir(), "state.db"))
	defer s.Close()
	j := fixture(t)
	injected := errors.New("abort before commit")
	if _, err := s.replay(j, func() error { return injected }); !errors.Is(err, injected) {
		t.Fatal(err)
	}
	rows, err := s.Alerts(j.Network, j.Source)
	if err != nil || len(rows) != 0 {
		t.Fatal("outbox survived aborted replay", err)
	}
	if _, found, _ := s.Load(j.Network, j.Source); found {
		t.Fatal("cursor survived aborted transaction")
	}
	for i := 0; i < 2; i++ {
		if _, err := s.Replay(j); err != nil {
			t.Fatal(err)
		}
	}
	rows, err = s.Alerts(j.Network, j.Source)
	if err != nil || len(rows) != 2 {
		t.Fatalf("rows=%d err=%v", len(rows), err)
	}
	for _, a := range rows {
		if a.Status != "PENDING" || a.Attempts != 0 || a.Payload.EvaluationStatus != "UNVERIFIED_INPUT" || a.Payload.Event == nil || a.Payload.Event.Finding.Authorization != "UNVERIFIED" {
			t.Fatalf("%+v", a)
		}
	}
}
func TestOutboxGapIsDurableAndScoped(t *testing.T) {
	path := filepath.Join(t.TempDir(), "state.db")
	s := openTest(t, path)
	j := fixture(t)
	j.Snapshots = j.Snapshots[:1]
	if _, err := s.Replay(j); err != nil {
		t.Fatal(err)
	}
	s.Close()
	s = openTest(t, path)
	defer s.Close()
	rows, err := s.Alerts(j.Network, j.Source)
	if err != nil || len(rows) != 1 || rows[0].Payload.Issue == nil || rows[0].Payload.Issue.Code != "OBSERVATION_GAP" {
		t.Fatalf("%+v %v", rows, err)
	}
	others, err := s.Alerts("other network", j.Source)
	if err != nil || len(others) != 0 {
		t.Fatal("scope leak", err)
	}
}
func TestOutboxLeaseExpiryAndFencing(t *testing.T) {
	s := openTest(t, filepath.Join(t.TempDir(), "state.db"))
	defer s.Close()
	j := fixture(t)
	j.Snapshots = j.Snapshots[:1]
	if _, err := s.Replay(j); err != nil {
		t.Fatal(err)
	}
	now := time.Unix(1000, 0)
	a, err := s.Claim(j.Network, j.Source, "sink", now, time.Second)
	if err != nil || a == nil {
		t.Fatal(err)
	}
	none, err := s.Claim(j.Network, j.Source, "sink", now, time.Second)
	if err != nil || none != nil {
		t.Fatal("live lease claimed twice", err)
	}
	b, err := s.Claim(j.Network, j.Source, "sink", now.Add(time.Second), time.Second)
	if err != nil || b == nil || b.LeaseToken == a.LeaseToken || b.Attempts != 2 {
		t.Fatal("expired lease not recovered", err)
	}
	if err := s.Resolve(j.Network, j.Source, a.ID, a.LeaseToken, true, false, now.Add(time.Second)); !errors.Is(err, ErrStaleLease) {
		t.Fatal("old worker acknowledged new lease", err)
	}
	if err := s.Resolve(j.Network, j.Source, b.ID, b.LeaseToken, true, false, now.Add(time.Second)); err != nil {
		t.Fatal(err)
	}
	if _, err := s.Replay(j); err != nil {
		t.Fatal(err)
	}
	rows, _ := s.Alerts(j.Network, j.Source)
	if rows[0].Status != "DELIVERED" || rows[0].Token != "" || rows[0].LeaseToken != "" {
		t.Fatal("delivered event requeued or lease leaked")
	}
}
func TestOutboxRetryLimitAndBackoff(t *testing.T) {
	s := openTest(t, filepath.Join(t.TempDir(), "state.db"))
	defer s.Close()
	j := fixture(t)
	j.Snapshots = j.Snapshots[:1]
	if _, err := s.Replay(j); err != nil {
		t.Fatal(err)
	}
	now := time.Unix(1000, 0)
	for attempt := 1; attempt <= MaxDeliveryAttempts; attempt++ {
		a, err := s.Claim(j.Network, j.Source, "sink", now, time.Second)
		if err != nil || a == nil || a.Attempts != attempt {
			t.Fatalf("attempt=%d alert=%+v err=%v", attempt, a, err)
		}
		if err := s.Resolve(j.Network, j.Source, a.ID, a.LeaseToken, false, false, now); err != nil {
			t.Fatal(err)
		}
		none, err := s.Claim(j.Network, j.Source, "sink", now, time.Second)
		if err != nil || none != nil {
			t.Fatal("retry ignored backoff", err)
		}
		now = now.Add(time.Duration(1<<attempt) * time.Second)
	}
	rows, _ := s.Alerts(j.Network, j.Source)
	if rows[0].Status != "DEAD" || rows[0].Attempts != MaxDeliveryAttempts {
		t.Fatalf("%+v", rows[0])
	}
}
func TestOutboxCannotSilentlyChangeSink(t *testing.T) {
	s := openTest(t, filepath.Join(t.TempDir(), "state.db"))
	defer s.Close()
	j := fixture(t)
	j.Snapshots = j.Snapshots[:1]
	s.Replay(j)
	now := time.Unix(1000, 0)
	a, err := s.Claim(j.Network, j.Source, "first", now, time.Second)
	if err != nil {
		t.Fatal(err)
	}
	s.Resolve(j.Network, j.Source, a.ID, a.LeaseToken, false, false, now)
	if _, err := s.Claim(j.Network, j.Source, "second", now.Add(3*time.Second), time.Second); err == nil {
		t.Fatal("silent sink change allowed")
	}
}

func TestOutboxRejectsCorruptState(t *testing.T) {
	for _, data := range []string{`{}`, `{"id":"bad","attempts":-1}`, `{"status":"APPROVED"}`} {
		if _, err := decodeAlert([]byte(data), "id", "network", "synthetic"); err == nil {
			t.Fatal("corrupt alert accepted")
		}
	}
}
