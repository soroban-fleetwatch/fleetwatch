package follow

import (
	"context"
	"path/filepath"
	"testing"
	"time"

	"github.com/soroban-fleetwatch/fleetwatch/internal/store"
)

func TestFollowDeliverySyncAcrossContinuousBatchBoundary(t *testing.T) {
	ctx := context.Background()
	b, all := chain(t)
	s := openState(t, filepath.Join(t.TempDir(), "follow.db"), b)
	defer s.Close()
	delivery, err := store.Open(filepath.Join(t.TempDir(), "delivery.db"))
	if err != nil {
		t.Fatal(err)
	}
	defer delivery.Close()
	sync := func(previous *store.FollowBinding) store.FollowSyncResult {
		t.Helper()
		head := s.Head()
		binding := store.FollowBinding{Network: b.Snapshot.Network, Stream: "continuous", BaselineDigest: head.Projection.OriginDigest, BaselineHeaderHash: head.Projection.OriginHeaderHash, BaselineLedger: b.Snapshot.Ledger, ProcessedThrough: head.Projection.Snapshot.Ledger, HeaderHash: head.Projection.HeaderHash}
		if previous != nil {
			anchor, err := s.HeaderAt(ctx, previous.ProcessedThrough)
			if err != nil || anchor != previous.HeaderHash {
				t.Fatal("prior import ancestry mismatch")
			}
		}
		findings, more, err := s.Findings(ctx, "", 100)
		if err != nil || more {
			t.Fatal(err)
		}
		events := []store.ReconstructionEventAlert{}
		for _, f := range findings {
			events = append(events, store.ReconstructionEventAlert{BaselineDigest: f.BaselineDigest, BaselineHeaderHash: f.BaselineHeaderHash, Ledger: f.Ledger, HeaderHash: f.HeaderHash, MetadataSHA256: f.MetadataSHA256, Event: f.Event})
		}
		r, err := delivery.ImportFollow(ctx, binding, events, previous)
		if err != nil {
			t.Fatal(err)
		}
		return r
	}
	if _, err := s.Commit(ctx, batch(b.Snapshot.Network, all[:16])); err != nil {
		t.Fatal(err)
	}
	r := sync(nil)
	if r.Added != 1 {
		t.Fatal(r)
	}
	scope, _ := store.AlertSource(b.Snapshot.Network, "follow_ledger_meta", "continuous")
	a, err := delivery.Claim(b.Snapshot.Network, scope, "local-test", time.Now(), time.Minute)
	if err != nil || a == nil {
		t.Fatal(err)
	}
	if err := delivery.Resolve(b.Snapshot.Network, scope, a.ID, a.LeaseToken, true, false, time.Now()); err != nil {
		t.Fatal(err)
	}
	for i := 16; i < 40; i += 8 {
		if _, err := s.Commit(ctx, batch(b.Snapshot.Network, all[i:i+8])); err != nil {
			t.Fatal(err)
		}
	}
	next := sync(&r.Binding)
	if next.Added != 1 || next.Retained != 1 {
		t.Fatal("later rollback was missed or upgrade reset", next)
	}
	rows, err := delivery.FollowAlerts(ctx, b.Snapshot.Network, "continuous")
	if err != nil || len(rows) != 2 {
		t.Fatal(err)
	}
	for _, row := range rows {
		if row.ID == a.ID && row.Status != "DELIVERED" {
			t.Fatal("acknowledgement lost across batch extension")
		}
	}
}
