package followdelivery

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/soroban-fleetwatch/fleetwatch/internal/follow"
	"github.com/soroban-fleetwatch/fleetwatch/internal/history"
	"github.com/soroban-fleetwatch/fleetwatch/internal/jsoninput"
	"github.com/soroban-fleetwatch/fleetwatch/internal/source"
	"github.com/soroban-fleetwatch/fleetwatch/internal/store"
	protocol "github.com/stellar/go-stellar-sdk/protocols/rpc"
)

type fixtureRPC struct {
	b history.Baseline
	c history.Capture
}

func (r fixtureRPC) Network(context.Context) (source.Network, error) {
	return source.Network{Passphrase: r.b.Snapshot.Network, ProtocolVersion: 29}, nil
}
func (r fixtureRPC) Head(context.Context) (source.Head, error) {
	return source.Head{Sequence: r.c.ToLedger, ProtocolVersion: 29, CloseTime: time.Now().Unix()}, nil
}
func (r fixtureRPC) Ledgers(_ context.Context, req protocol.GetLedgersRequest) (protocol.GetLedgersResponse, error) {
	out := protocol.GetLedgersResponse{OldestLedger: r.c.FromLedger, LatestLedger: r.c.ToLedger}
	for _, e := range r.c.Ledgers {
		if e.Sequence == req.StartLedger {
			out.Ledgers = []protocol.LedgerInfo{{Sequence: e.Sequence, Hash: e.Hash, LedgerCloseTime: e.CloseTime, LedgerHeader: e.HeaderXDR, LedgerMetadata: e.MetadataXDR}}
		}
	}
	return out, nil
}

func fixture(t testing.TB) (history.Baseline, history.Capture, follow.Config) {
	t.Helper()
	var b history.Baseline
	var c history.Capture
	baselinePath, capturePath := "../../testdata/reconstruction-baseline.json", "../../testdata/reconstruction-capture.json"
	if os.Getenv("FLEETWATCH_FOLLOW_AUTOSYNC_BASELINE") != "" || os.Getenv("FLEETWATCH_FOLLOW_AUTOSYNC_CAPTURE") != "" {
		baselinePath, capturePath = os.Getenv("FLEETWATCH_FOLLOW_AUTOSYNC_BASELINE"), os.Getenv("FLEETWATCH_FOLLOW_AUTOSYNC_CAPTURE")
		if baselinePath == "" || capturePath == "" {
			t.Fatal("both optional captured baseline and capture paths are required")
		}
	}
	if jsoninput.File(baselinePath, &b) != nil || jsoninput.File(capturePath, &c) != nil {
		t.Fatal("fixtures unavailable")
	}
	return b, c, follow.Config{SchemaVersion: 1, RPCURL: "https://rpc.example.invalid", Baseline: "baseline.json", State: "follow.db", Listen: "127.0.0.1:0", PollIntervalSeconds: 1, BatchSize: 16, MaxEndpointLag: 2, MaxRetainedLedgers: 100, MaxRetainedPayloadBytes: 64 << 20}
}

func TestAutomaticSyncBeforeReadinessAndCrashWindowRecovery(t *testing.T) {
	b, c, cfg := fixture(t)
	reconstruction, err := history.Reconstruct(context.Background(), b, c)
	if err != nil {
		t.Fatal(err)
	}
	expectedFindings := 0
	for _, ledger := range reconstruction.Ledgers {
		expectedFindings += len(ledger.Events)
	}
	dir := t.TempDir()
	path, outboxPath := filepath.Join(dir, "follow.db"), filepath.Join(dir, "delivery.db")
	s, err := follow.Open(context.Background(), path, b, cfg.Limits())
	if err != nil {
		t.Fatal(err)
	}
	health := follow.NewHealth(s.Head(), cfg)
	calls := 0
	syncHook := func(ctx context.Context) error {
		calls++
		if calls == 2 {
			return errors.New("injected crash window after source commit")
		}
		_, err := SyncFile(ctx, s, outboxPath, b, "automatic")
		return err
	}
	err = follow.RunWithSync(context.Background(), fixtureRPC{b, c}, s, cfg, health, func(status follow.Status) error {
		if status.Ready || status.Code != "OUTBOX_SYNC_FAILED" {
			t.Error("queue failure published readiness", status)
		}
		return nil
	}, syncHook)
	if !errors.Is(err, follow.ErrSync) || s.Head().Projection.Snapshot.Ledger != c.ToLedger || s.Head().PendingFindings != expectedFindings || health.Snapshot(time.Now()).Ready {
		t.Fatal("sync failure lost committed findings or appeared ready", err)
	}
	cycle, _ := s.Cycle()
	if cycle.Code != "OUTBOX_SYNC_FAILED" || cycle.SuccessfulAt != 0 {
		t.Fatal(cycle)
	}
	s.Close()
	// Fresh process validates native evidence and repairs the unsynced commit at
	// startup before any healthy cycle is emitted. No webhook sender is involved.
	s, err = follow.Open(context.Background(), path, b, cfg.Limits())
	if err != nil {
		t.Fatal(err)
	}
	defer s.Close()
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	health = follow.NewHealth(s.Head(), cfg)
	err = follow.RunWithSync(ctx, fixtureRPC{b, c}, s, cfg, health, func(status follow.Status) error {
		outbox, err := store.OpenReadOnly(outboxPath)
		if err != nil {
			t.Fatal(err)
		}
		rows, err := outbox.FollowAlerts(context.Background(), b.Snapshot.Network, "automatic")
		outbox.Close()
		if err != nil || len(rows) != expectedFindings || !status.Ready {
			t.Error("readiness preceded outbox catch-up", status, err)
		}
		cancel()
		return nil
	}, func(ctx context.Context) error { _, err := SyncFile(ctx, s, outboxPath, b, "automatic"); return err })
	if err != nil {
		t.Fatal(err)
	}
	if cycle, err := s.Cycle(); err != nil || cycle.Code != "STOPPED" {
		t.Fatal("graceful stop failed", cycle, err)
	}
	if output := os.Getenv("FLEETWATCH_FOLLOW_AUTOSYNC_REPORT"); output != "" {
		raw, err := json.MarshalIndent(map[string]any{"kind": "OFFLINE_REPLAY_WITH_SYNTHETIC_SOURCE_HEAD", "baseline_ledger": b.Snapshot.Ledger, "processed_through": c.ToLedger,
			"retained_ledgers": len(c.Ledgers), "recovered_findings": expectedFindings, "sync_failure_not_ready": true, "restart_catches_up_before_ready": true,
			"graceful_stop": true, "external_deliveries": 0, "new_chain_requests": 0, "real_source_freshness_verified": false, "final_verdict": "UNDETERMINED"}, "", "  ")
		if err != nil {
			t.Fatal(err)
		}
		file, err := os.OpenFile(output, os.O_WRONLY|os.O_CREATE|os.O_EXCL, 0600)
		if err != nil {
			t.Fatal(err)
		}
		_, writeErr := file.Write(append(raw, '\n'))
		closeErr := file.Close()
		if writeErr != nil || closeErr != nil {
			t.Fatal(writeErr, closeErr)
		}
	}
}

func TestAutomaticSyncLockRetryAndCancellation(t *testing.T) {
	b, _, cfg := fixture(t)
	s, err := follow.Open(context.Background(), filepath.Join(t.TempDir(), "follow.db"), b, cfg.Limits())
	if err != nil {
		t.Fatal(err)
	}
	defer s.Close()
	path := filepath.Join(t.TempDir(), "delivery.db")
	locked, err := store.Open(path)
	if err != nil {
		t.Fatal(err)
	}
	// The first open attempt times out, then the temporary dispatcher releases.
	done := make(chan struct{})
	time.AfterFunc(1100*time.Millisecond, func() { locked.Close(); close(done) })
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	if _, err := SyncFile(ctx, s, path, b, "automatic"); err != nil {
		t.Fatal(err)
	}
	<-done
	before, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := SyncFile(ctx, s, path, b, "automatic"); err != nil {
		t.Fatal(err)
	}
	after, err := os.ReadFile(path)
	if err != nil || !bytes.Equal(before, after) {
		t.Fatal("unchanged sync rewrote outbox", err)
	}
	ctx, cancel = context.WithCancel(context.Background())
	cancel()
	missing := filepath.Join(t.TempDir(), "missing.db")
	if _, err := SyncFile(ctx, s, missing, b, "automatic"); !errors.Is(err, context.Canceled) {
		t.Fatal(err)
	}
	if _, err := os.Stat(missing); !os.IsNotExist(err) {
		t.Fatal("cancelled sync created outbox")
	}
}

func TestAutomaticSyncStartupFailureAndCancellation(t *testing.T) {
	b, c, cfg := fixture(t)
	s, err := follow.Open(context.Background(), filepath.Join(t.TempDir(), "follow.db"), b, cfg.Limits())
	if err != nil {
		t.Fatal(err)
	}
	defer s.Close()
	health := follow.NewHealth(s.Head(), cfg)
	err = follow.RunWithSync(context.Background(), fixtureRPC{b, c}, s, cfg, health, func(follow.Status) error { return nil }, func(context.Context) error { return errors.New("private storage detail") })
	if !errors.Is(err, follow.ErrSync) || s.Head().RetainedLedgers != 0 || health.Snapshot(time.Now()).Ready {
		t.Fatal("startup sync failure reached source ingestion", err)
	}
	ctx, cancel := context.WithCancel(context.Background())
	err = follow.RunWithSync(ctx, fixtureRPC{b, c}, s, cfg, health, func(follow.Status) error { return nil }, func(context.Context) error { cancel(); return ctx.Err() })
	if err != nil {
		t.Fatal("cancellation not graceful", err)
	}
	if cycle, err := s.Cycle(); err != nil || cycle.Code != "STOPPED" {
		t.Fatal(cycle, err)
	}
}
