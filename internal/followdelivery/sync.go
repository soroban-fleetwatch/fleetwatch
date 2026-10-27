// Package followdelivery bridges a replay-validated native follow store to the
// separate delivery outbox. It never queries a source or dispatches a message.
package followdelivery

import (
	"context"
	"encoding/json"
	"errors"
	"reflect"
	"time"

	"github.com/soroban-fleetwatch/fleetwatch/internal/follow"
	"github.com/soroban-fleetwatch/fleetwatch/internal/history"
	"github.com/soroban-fleetwatch/fleetwatch/internal/store"
	bolt "go.etcd.io/bbolt"
)

// Sync must run while the owning worker is paused between polls, or while the
// source is opened read-only. The bridge scans all IDs, never a hash watermark.
func Sync(ctx context.Context, source *follow.Store, outbox *store.Store, baseline history.Baseline, stream string) (store.FollowSyncResult, error) {
	var empty store.FollowSyncResult
	if source == nil || outbox == nil || store.ValidateReconstructionScope(baseline.Snapshot.Network, stream) != nil {
		return empty, errors.New("invalid follow sync configuration")
	}
	head := source.Head()
	origin, err := history.NewProjection(baseline)
	if err != nil || origin.OriginDigest != head.Projection.OriginDigest || origin.OriginHeaderHash != head.Projection.OriginHeaderHash || baseline.Snapshot.Network != head.Projection.Snapshot.Network {
		return empty, errors.New("follow sync baseline does not match native origin")
	}
	events := []store.ReconstructionEventAlert{}
	cursor, bytesSeen := "", 0
	for {
		rows, more, err := source.Findings(ctx, cursor, 100)
		if err != nil {
			return empty, err
		}
		for _, f := range rows {
			e := store.ReconstructionEventAlert{BaselineDigest: f.BaselineDigest, BaselineHeaderHash: f.BaselineHeaderHash, Ledger: f.Ledger, HeaderHash: f.HeaderHash, MetadataSHA256: f.MetadataSHA256, Event: f.Event}
			raw, err := json.Marshal(e)
			bytesSeen += len(raw) + 1
			if err != nil || bytesSeen > 24<<20 || len(events) >= 10000 {
				return empty, errors.New("follow import exceeds bounded delivery manifest")
			}
			events = append(events, e)
			cursor = f.Event.ID
		}
		if !more {
			break
		}
	}
	if len(events) != head.PendingFindings || !reflect.DeepEqual(head, source.Head()) {
		return empty, errors.New("follow source changed during sync")
	}
	binding := store.FollowBinding{Network: baseline.Snapshot.Network, Stream: stream, BaselineDigest: head.Projection.OriginDigest, BaselineHeaderHash: head.Projection.OriginHeaderHash, BaselineLedger: baseline.Snapshot.Ledger, ProcessedThrough: head.Projection.Snapshot.Ledger, HeaderHash: head.Projection.HeaderHash}
	previous, err := outbox.FollowImportBinding(ctx, binding.Network, binding.Stream)
	if err != nil {
		return empty, err
	}
	if previous != nil {
		anchor, err := source.HeaderAt(ctx, previous.ProcessedThrough)
		if err != nil || anchor != previous.HeaderHash {
			return empty, errors.New("source rewound or prior sync header diverged")
		}
		if reflect.DeepEqual(binding, *previous) {
			rows, err := outbox.FollowAlerts(ctx, binding.Network, binding.Stream)
			if err != nil {
				return empty, err
			}
			expected := make(map[string]store.ReconstructionEventAlert, len(events))
			for _, event := range events {
				expected[event.Event.ID] = event
			}
			if len(rows) != len(events) {
				return empty, errors.New("unchanged follow binding has conflicting findings")
			}
			for _, row := range rows {
				if row.Payload.Reconstruction == nil || !reflect.DeepEqual(expected[row.Payload.Reconstruction.Event.ID], *row.Payload.Reconstruction) {
					return empty, errors.New("unchanged follow binding has conflicting payload")
				}
			}
			return store.FollowSyncResult{SchemaVersion: 1, Binding: binding, Retained: len(events), Offline: true, FinalVerdict: "UNDETERMINED"}, nil
		}
	}
	return outbox.ImportFollow(ctx, binding, events, previous)
}

// SyncFile opens the outbox only for this sync, allowing a separately operated
// dispatcher to own the file between syncs. Lock contention retries until ctx
// expires; other storage/lineage errors are immediately fatal to the hook.
func SyncFile(ctx context.Context, source *follow.Store, path string, baseline history.Baseline, stream string) (store.FollowSyncResult, error) {
	for {
		if ctx.Err() != nil {
			return store.FollowSyncResult{}, ctx.Err()
		}
		outbox, err := store.Open(path)
		if err == nil {
			result, syncErr := Sync(ctx, source, outbox, baseline, stream)
			closeErr := outbox.Close()
			if syncErr != nil {
				return store.FollowSyncResult{}, syncErr
			}
			return result, closeErr
		}
		if !errors.Is(err, bolt.ErrTimeout) {
			return store.FollowSyncResult{}, err
		}
		timer := time.NewTimer(250 * time.Millisecond)
		select {
		case <-ctx.Done():
			timer.Stop()
			return store.FollowSyncResult{}, ctx.Err()
		case <-timer.C:
		}
	}
}
