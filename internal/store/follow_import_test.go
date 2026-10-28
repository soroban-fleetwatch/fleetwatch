package store

import (
	"context"
	"errors"
	"path/filepath"
	"sort"
	"strings"
	"testing"
	"time"

	"github.com/soroban-fleetwatch/fleetwatch/internal/history"
	"github.com/soroban-fleetwatch/fleetwatch/internal/jsoninput"
	bolt "go.etcd.io/bbolt"
)

func followImportFixture(t testing.TB) (FollowBinding, []ReconstructionEventAlert) {
	t.Helper()
	var b history.Baseline
	var c history.Capture
	if jsoninput.File("../../testdata/reconstruction-baseline.json", &b) != nil || jsoninput.File("../../testdata/reconstruction-capture.json", &c) != nil {
		t.Fatal("fixtures unavailable")
	}
	r, err := history.Reconstruct(context.Background(), b, c)
	if err != nil {
		t.Fatal(err)
	}
	events := []ReconstructionEventAlert{}
	for _, l := range r.Ledgers {
		for _, e := range l.Events {
			events = append(events, ReconstructionEventAlert{r.BaselineDigest, r.BaselineHeaderHash, l.Sequence, l.HeaderHash, l.MetadataSHA256, e})
		}
	}
	sort.Slice(events, func(i, j int) bool { return events[i].Event.ID < events[j].Event.ID })
	return FollowBinding{Network: c.Network, Stream: "follow-test", BaselineDigest: r.BaselineDigest, BaselineHeaderHash: b.HeaderHash, BaselineLedger: b.Snapshot.Ledger, ProcessedThrough: r.ProcessedThrough, HeaderHash: r.Ledgers[len(r.Ledgers)-1].HeaderHash}, events
}

func TestFollowImportPreservesAcknowledgementsAcrossSyncReopenAndBackup(t *testing.T) {
	ctx := context.Background()
	b, events := followImportFixture(t)
	path := filepath.Join(t.TempDir(), "delivery.db")
	s, err := Open(path)
	if err != nil {
		t.Fatal(err)
	}
	r, err := s.ImportFollow(ctx, b, events, nil)
	if err != nil || r.Added != len(events) || r.Retained != 0 {
		t.Fatal(r, err)
	}
	source, _ := AlertSource(b.Network, "follow_ledger_meta", b.Stream)
	a, err := s.Claim(b.Network, source, "test-sink", time.Now(), time.Minute)
	if err != nil || a == nil || a.Payload.SchemaVersion != 3 || a.Payload.Source != "follow_ledger_meta" || a.Payload.Assurance.FinalVerdict != "UNDETERMINED" {
		t.Fatal(a, err)
	}
	if err := s.Resolve(b.Network, source, a.ID, a.LeaseToken, true, false, time.Now()); err != nil {
		t.Fatal(err)
	}
	r, err = s.ImportFollow(ctx, b, events, &b)
	if err != nil || r.Added != 0 || r.Retained != len(events) {
		t.Fatal(r, err)
	}
	backup := filepath.Join(t.TempDir(), "backup.db")
	if _, err := s.Backup(ctx, backup); err != nil {
		t.Fatal(err)
	}
	s.Close()
	for _, file := range []string{path, backup} {
		s, err := Open(file)
		if err != nil {
			t.Fatal(err)
		}
		rows, err := s.FollowAlerts(ctx, b.Network, b.Stream)
		if err != nil || len(rows) != len(events) {
			t.Fatal(rows, err)
		}
		found := false
		for _, row := range rows {
			if row.ID == a.ID {
				found = row.Status == "DELIVERED" && row.Attempts == 1
			}
			if row.Token != "" || row.LeaseToken != "" {
				t.Fatal("inspection leaked lease")
			}
		}
		if !found {
			t.Fatal("acknowledged event reset")
		}
		s.Close()
	}
}

func TestFollowImportRejectsConflictsRollbackAndTampering(t *testing.T) {
	ctx := context.Background()
	b, events := followImportFixture(t)
	s, err := Open(filepath.Join(t.TempDir(), "delivery.db"))
	if err != nil {
		t.Fatal(err)
	}
	defer s.Close()
	if _, err := s.importFollow(ctx, b, events, nil, func() error { return errors.New("interrupted") }); err == nil {
		t.Fatal("interrupted import published")
	}
	if prior, err := s.FollowImportBinding(ctx, b.Network, b.Stream); err != nil || prior != nil {
		t.Fatal("rollback retained binding", prior, err)
	}
	if _, err := s.ImportFollow(ctx, b, events, nil); err != nil {
		t.Fatal(err)
	}
	for _, mode := range []string{"missing", "head", "baseline", "stale", "assurance"} {
		bad := b
		incoming := append([]ReconstructionEventAlert{}, events...)
		previous := &b
		switch mode {
		case "missing":
			incoming = incoming[:len(incoming)-1]
		case "head":
			bad.HeaderHash = strings.Repeat("a", 64)
		case "baseline":
			bad.BaselineHeaderHash = strings.Repeat("a", 64)
		case "stale":
			previous = nil
		case "assurance":
			incoming[0].Event.Finding.Authorization = "VERIFIED"
		}
		if _, err := s.ImportFollow(ctx, bad, incoming, previous); err == nil {
			t.Fatal("invalid import accepted", mode)
		}
	}
	rows, err := s.FollowAlerts(ctx, b.Network, b.Stream)
	if err != nil || len(rows) != len(events) {
		t.Fatal("failed import changed valid queue", err)
	}
	if err := s.db.Update(func(tx *bolt.Tx) error {
		q, err := queue(tx, b.Network, followQueuePrefix+b.Stream, false)
		if err != nil {
			return err
		}
		return q.Delete([]byte(rows[0].ID))
	}); err != nil {
		t.Fatal(err)
	}
	source, _ := AlertSource(b.Network, "follow_ledger_meta", b.Stream)
	if _, err := s.Claim(b.Network, source, "test-sink", time.Now(), time.Minute); err == nil {
		t.Fatal("orphaned manifest permitted claim")
	}
	if _, err := s.FollowAlerts(ctx, b.Network, b.Stream); err == nil {
		t.Fatal("corrupt queue inspection succeeded")
	}
}

func TestFollowDeliveryLeaseRetryAndAuditedRecovery(t *testing.T) {
	ctx := context.Background()
	b, events := followImportFixture(t)
	s, err := Open(filepath.Join(t.TempDir(), "delivery.db"))
	if err != nil {
		t.Fatal(err)
	}
	defer s.Close()
	if _, err := s.ImportFollow(ctx, b, events, nil); err != nil {
		t.Fatal(err)
	}
	source, _ := AlertSource(b.Network, "follow_ledger_meta", b.Stream)
	now := time.Now()
	first, err := s.Claim(b.Network, source, "test-sink", now, time.Second)
	if err != nil || first == nil {
		t.Fatal(err)
	}
	// An expired acknowledgement is fenced; the event remains recoverable.
	if err := s.Resolve(b.Network, source, first.ID, first.LeaseToken, true, false, now.Add(2*time.Second)); !errors.Is(err, ErrStaleLease) {
		t.Fatal("expired acknowledgement accepted", err)
	}
	retried, err := s.Claim(b.Network, source, "test-sink", now.Add(2*time.Second), time.Minute)
	if err != nil || retried == nil || retried.ID != first.ID || retried.LeaseToken == first.LeaseToken {
		t.Fatal("expired lease not safely retried", err)
	}
	if err := s.Resolve(b.Network, source, retried.ID, retried.LeaseToken, false, true, now.Add(3*time.Second)); err != nil {
		t.Fatal(err)
	}
	if _, err := s.ImportFollow(ctx, b, events, &b); err != nil {
		t.Fatal(err)
	}
	rows, err := s.FollowAlerts(ctx, b.Network, b.Stream)
	if err != nil {
		t.Fatal(err)
	}
	for _, row := range rows {
		if row.ID == retried.ID && row.Status != "DEAD" {
			t.Fatal("sync resurrected dead letter")
		}
	}
	request := RecoveryRequest{AlertID: retried.ID, RequestID: "local-follow-recovery", ExpectedCycle: 0, Reason: "local retry test"}
	// Recovery uses the same generic audited path as legacy deliveries.
	_, err = s.Requeue(ctx, b.Network, source, request, now.Add(4*time.Second))
	if err != nil {
		t.Fatal("follow recovery failed", err)
	}
}
