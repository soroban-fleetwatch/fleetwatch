package alert

import (
	"context"
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"github.com/soroban-fleetwatch/fleetwatch/internal/follow"
	"github.com/soroban-fleetwatch/fleetwatch/internal/followdelivery"
	"github.com/soroban-fleetwatch/fleetwatch/internal/history"
	"github.com/soroban-fleetwatch/fleetwatch/internal/jsoninput"
	"github.com/soroban-fleetwatch/fleetwatch/internal/store"
)

type loopSender func(context.Context, store.Alert) error

func (s loopSender) Send(ctx context.Context, a store.Alert) error { return s(ctx, a) }

func loopFixture(t testing.TB) LoopConfig {
	t.Helper()
	var b history.Baseline
	var c history.Capture
	baselinePath, capturePath := "../../testdata/reconstruction-baseline.json", "../../testdata/reconstruction-capture.json"
	if os.Getenv("FLEETWATCH_DELIVERY_LOOP_BASELINE") != "" || os.Getenv("FLEETWATCH_DELIVERY_LOOP_CAPTURE") != "" {
		baselinePath, capturePath = os.Getenv("FLEETWATCH_DELIVERY_LOOP_BASELINE"), os.Getenv("FLEETWATCH_DELIVERY_LOOP_CAPTURE")
		if baselinePath == "" || capturePath == "" {
			t.Fatal("both optional captured baseline/capture paths required")
		}
	}
	if jsoninput.File(baselinePath, &b) != nil || jsoninput.File(capturePath, &c) != nil {
		t.Fatal("fixtures unavailable")
	}
	dir := t.TempDir()
	native, err := follow.Open(context.Background(), filepath.Join(dir, "native.db"), b, follow.Limits{MaxLedgers: 100, MaxPayloadBytes: 64 << 20})
	if err != nil {
		t.Fatal(err)
	}
	defer native.Close()
	if _, err := native.Commit(context.Background(), c); err != nil {
		t.Fatal(err)
	}
	path := filepath.Join(dir, "delivery.db")
	if _, err := followdelivery.SyncFile(context.Background(), native, path, b, "loop-test"); err != nil {
		t.Fatal(err)
	}
	return LoopConfig{State: path, Network: b.Snapshot.Network, Stream: "loop-test", Sink: "local-test", PollIntervalSeconds: 1, BatchSize: 1}
}

func TestDeliveryLoopLocalTLSSignsAndAcknowledgesAcrossRestart(t *testing.T) {
	cfg := loopFixture(t)
	probe, err := store.OpenReadOnly(cfg.State)
	if err != nil {
		t.Fatal(err)
	}
	initial, err := probe.FollowAlerts(context.Background(), cfg.Network, cfg.Stream)
	probe.Close()
	if err != nil || len(initial) == 0 {
		t.Fatal("nonempty loop fixture required", err)
	}
	expectedCount := len(initial)
	var received atomic.Int32
	server, roots := localHook(t, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		body, err := io.ReadAll(r.Body)
		if err != nil {
			t.Error(err)
			return
		}
		var envelope struct {
			ID      string             `json:"id"`
			Payload store.AlertPayload `json:"payload"`
		}
		if json.Unmarshal(body, &envelope) != nil || store.ValidateAlertPayload(envelope.ID, envelope.Payload) != nil || envelope.Payload.SchemaVersion != 3 || envelope.Payload.Assurance.FinalVerdict != "UNDETERMINED" || r.Header.Get("Idempotency-Key") != envelope.ID || strings.Contains(string(body), "lease_token") {
			t.Error("invalid loop envelope")
			w.WriteHeader(400)
			return
		}
		mac := hmac.New(sha256.New, hookKey())
		mac.Write([]byte("v1\n" + r.Header.Get("X-FleetWatch-Timestamp") + "\n" + envelope.ID + "\n"))
		mac.Write(body)
		actual, err := hex.DecodeString(strings.TrimPrefix(r.Header.Get("X-FleetWatch-Signature"), "v1="))
		if err != nil || !hmac.Equal(actual, mac.Sum(nil)) {
			t.Error("invalid loop HMAC")
		}
		received.Add(1)
		w.WriteHeader(204)
	}))
	sender := testWebhook(t, server, roots)
	cfg.Sink = sender.SinkID()
	for pass := 0; pass < 2; pass++ {
		ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
		stopped := false
		err := DeliveryLoop(ctx, cfg, sender, func(s LoopStatus) error {
			if s.Code == "STOPPED" {
				stopped = true
				return nil
			}
			// The dispatcher must release the file before emitting status.
			probe, err := store.OpenReadOnly(cfg.State)
			if err != nil {
				t.Fatal("outbox held across status callback", err)
			}
			probe.Close()
			if s.Queue != nil && s.Queue.Delivered == expectedCount {
				if pass == 1 && s.Batch.Attempted != 0 {
					t.Error("restart repeated acknowledged delivery")
				}
				cancel()
			}
			return nil
		})
		cancel()
		if err != nil || !stopped {
			t.Fatal(err, stopped)
		}
	}
	if int(received.Load()) != expectedCount {
		t.Fatal("unexpected receiver requests", received.Load())
	}
	if output := os.Getenv("FLEETWATCH_DELIVERY_LOOP_REPORT"); output != "" {
		data, err := json.MarshalIndent(map[string]any{"mode": "LOCAL_TLS_DELIVERY_LOOP_OF_CAPTURED_FINDINGS", "delivered_findings": expectedCount, "received_requests": received.Load(),
			"restart_repeat_attempts": 0, "hmac_and_idempotency_verified": true, "outbox_released_between_batches": true, "graceful_stop_verified": true,
			"external_deliveries": 0, "new_chain_requests": 0, "final_verdict": "UNDETERMINED"}, "", "  ")
		if err != nil {
			t.Fatal(err)
		}
		file, err := os.OpenFile(output, os.O_WRONLY|os.O_CREATE|os.O_EXCL, 0600)
		if err != nil {
			t.Fatal(err)
		}
		_, writeErr := file.Write(append(data, '\n'))
		closeErr := file.Close()
		if writeErr != nil || closeErr != nil {
			t.Fatal(writeErr, closeErr)
		}
	}
}

func TestDeliveryLoopWaitsWithoutCreatingAndReleasesBusyOutbox(t *testing.T) {
	for _, mode := range []string{"missing", "busy", "no-import", "empty", "corrupt"} {
		t.Run(mode, func(t *testing.T) {
			cfg := LoopConfig{State: filepath.Join(t.TempDir(), "delivery.db"), Network: "SYNTHETIC-NETWORK", Stream: "pilot", Sink: "test", PollIntervalSeconds: 1, BatchSize: 1}
			if mode == "busy" || mode == "no-import" {
				s, err := store.Open(cfg.State)
				if err != nil {
					t.Fatal(err)
				}
				if mode == "busy" {
					defer s.Close()
				} else {
					s.Close()
				}
			}
			if mode == "corrupt" {
				if err := os.WriteFile(cfg.State, []byte("invalid"), 0600); err != nil {
					t.Fatal(err)
				}
			}
			if mode == "empty" {
				if err := os.WriteFile(cfg.State, nil, 0600); err != nil {
					t.Fatal(err)
				}
			}
			ctx, cancel := context.WithCancel(context.Background())
			defer cancel()
			want := map[string]string{"missing": "WAITING_FOR_OUTBOX", "busy": "OUTBOX_BUSY", "no-import": "WAITING_FOR_IMPORT", "empty": "OUTBOX_EMPTY", "corrupt": "STATE_FAILURE"}[mode]
			err := DeliveryLoop(ctx, cfg, loopSender(func(context.Context, store.Alert) error { t.Error("unexpected send"); return nil }), func(s LoopStatus) error {
				if s.Code != "STOPPED" {
					if s.Code != want || s.Queue != nil {
						t.Error(s)
					}
					if mode != "corrupt" {
						cancel()
					}
				}
				return nil
			})
			if (mode == "corrupt") != (err != nil) {
				t.Fatal(mode, err)
			}
			if mode == "missing" {
				if _, err := os.Stat(cfg.State); !os.IsNotExist(err) {
					t.Fatal("waiting dispatcher created outbox")
				}
			}
			if mode == "empty" {
				info, err := os.Stat(cfg.State)
				if err != nil || info.Size() != 0 {
					t.Fatal("dispatcher initialized an empty file", err)
				}
			}
		})
	}
}

func TestDeliveryLoopRetriesAndLeavesPermanentDeadLetters(t *testing.T) {
	for _, permanent := range []bool{false, true} {
		cfg := loopFixture(t)
		calls := 0
		ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
		retrySeen, deadSeen := false, false
		err := DeliveryLoop(ctx, cfg, loopSender(func(context.Context, store.Alert) error {
			calls++
			if permanent {
				return PermanentError{}
			}
			if calls == 1 {
				return errors.New("private transport detail")
			}
			return nil
		}), func(s LoopStatus) error {
			retrySeen = retrySeen || s.Code == "DELIVERY_RETRY"
			deadSeen = deadSeen || s.Code == "DEAD_LETTERS_PRESENT"
			if s.Queue != nil && (s.Queue.Delivered == 2 || s.Queue.Dead == 2) {
				cancel()
			}
			return nil
		})
		cancel()
		if err != nil || permanent && (!deadSeen || calls != 2) || !permanent && (!retrySeen || calls != 3) {
			t.Fatal(err, calls, retrySeen, deadSeen)
		}
		if permanent {
			ctx, cancel := context.WithCancel(context.Background())
			err := DeliveryLoop(ctx, cfg, loopSender(func(context.Context, store.Alert) error { t.Error("dead letter resurrected"); return nil }), func(s LoopStatus) error {
				if s.Code != "STOPPED" {
					if s.Code != "DEAD_LETTERS_PRESENT" || s.Batch.Attempted != 0 {
						t.Error(s)
					}
					cancel()
				}
				return nil
			})
			cancel()
			if err != nil {
				t.Fatal(err)
			}
		}
	}
}

func TestDeliveryLoopCancellationAndOutputFailurePreserveOutbox(t *testing.T) {
	for _, cancelSend := range []bool{true, false} {
		cfg := loopFixture(t)
		ctx, cancel := context.WithCancel(context.Background())
		calls := 0
		err := DeliveryLoop(ctx, cfg, loopSender(func(ctx context.Context, _ store.Alert) error {
			calls++
			if cancelSend {
				cancel()
				return ctx.Err()
			}
			return nil
		}), func(s LoopStatus) error {
			if !cancelSend {
				return errors.New("injected output failure")
			}
			return nil
		})
		cancel()
		if cancelSend && err != nil || !cancelSend && err == nil || calls != 1 {
			t.Fatal(err, calls)
		}
		s, err := store.OpenReadOnly(cfg.State)
		if err != nil {
			t.Fatal(err)
		}
		rows, err := s.FollowAlerts(context.Background(), cfg.Network, cfg.Stream)
		s.Close()
		if err != nil {
			t.Fatal(err)
		}
		delivered := 0
		for _, row := range rows {
			if row.Status == "DELIVERED" {
				delivered++
			}
		}
		if cancelSend && delivered != 0 || !cancelSend && delivered != 1 {
			t.Fatal("output/cancellation lost delivery state", delivered)
		}
	}
}
