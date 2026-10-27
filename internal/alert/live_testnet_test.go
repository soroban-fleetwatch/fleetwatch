package alert

import (
	"context"
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"io"
	"net/http"
	"os"
	"strings"
	"sync"
	"testing"

	"github.com/soroban-fleetwatch/fleetwatch/internal/store"
)

// Opt-in: the validation runner passes a backup of its real Testnet outbox.
// The receiver is loopback-only with explicitly trusted test TLS; production
// URL/DNS/proxy restrictions remain unchanged. No external receiver is contacted.
func TestLiveTestnetOutboxLocalTLS(t *testing.T) {
	path := os.Getenv("FLEETWATCH_TESTNET_STATE")
	output := os.Getenv("FLEETWATCH_TESTNET_RECEIVER_REPORT")
	if path == "" && output == "" {
		t.Skip("opt-in Testnet outbox backup not provided")
	}
	if path == "" || output == "" {
		t.Fatal("both Testnet state/report paths are required")
	}
	network := "Test SDF Network ; September 2015"
	stream := "testnet-demo"
	s, err := store.Open(path)
	if err != nil {
		t.Fatal(err)
	}
	defer func() {
		if s != nil {
			_ = s.Close()
		}
	}()
	rows, err := s.ReconstructionAlerts(context.Background(), network, stream)
	if err != nil || len(rows) == 0 {
		t.Fatal("real reconstruction outbox unavailable", err)
	}
	for _, row := range rows {
		if row.Status != "PENDING" {
			t.Fatal("use a fresh outbox backup")
		}
	}
	var lock sync.Mutex
	seen := map[string]bool{}
	server, roots := localHook(t, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		data, err := io.ReadAll(r.Body)
		if err != nil {
			t.Error(err)
			w.WriteHeader(400)
			return
		}
		var body struct {
			ID      string             `json:"id"`
			Payload store.AlertPayload `json:"payload"`
		}
		if json.Unmarshal(data, &body) != nil || body.Payload.Network != network || body.Payload.Stream != stream || body.Payload.Assurance.FinalVerdict != "UNDETERMINED" || body.Payload.EvaluationStatus != "UNVERIFIED_PROJECTED_STATE" {
			t.Error("live alert assurance changed")
			w.WriteHeader(400)
			return
		}
		mac := hmac.New(sha256.New, hookKey())
		mac.Write([]byte("v1\n" + r.Header.Get("X-FleetWatch-Timestamp") + "\n" + body.ID + "\n"))
		mac.Write(data)
		sig, err := hex.DecodeString(strings.TrimPrefix(r.Header.Get("X-FleetWatch-Signature"), "v1="))
		if err != nil || !hmac.Equal(sig, mac.Sum(nil)) || r.Header.Get("Idempotency-Key") != body.ID || strings.Contains(string(data), "lease_token") {
			t.Error("invalid delivery MAC/identity")
			w.WriteHeader(400)
			return
		}
		lock.Lock()
		defer lock.Unlock()
		if seen[body.ID] {
			t.Error("acknowledged alert duplicated")
		}
		seen[body.ID] = true
		w.WriteHeader(204)
	}))
	sender := testWebhook(t, server, roots)
	source, err := store.AlertSource(network, "rpc_ledger_meta", stream)
	if err != nil {
		t.Fatal(err)
	}
	result, err := Dispatch(context.Background(), s, sender, network, source, sender.SinkID(), 50)
	if err != nil || result.Delivered != len(rows) || result.Failed != 0 {
		t.Fatal(result, err)
	}
	if err := s.Close(); err != nil {
		t.Fatal(err)
	}
	s, err = store.Open(path)
	if err != nil {
		t.Fatal(err)
	}
	repeat, err := Dispatch(context.Background(), s, sender, network, source, sender.SinkID(), 50)
	if err != nil || repeat.Attempted != 0 {
		t.Fatal("delivery repeated after cold reopen", repeat, err)
	}
	lock.Lock()
	defer lock.Unlock()
	if len(seen) != len(rows) {
		t.Fatal("receiver count mismatch")
	}
	file, err := os.OpenFile(output, os.O_CREATE|os.O_EXCL|os.O_WRONLY, 0600)
	if err != nil {
		t.Fatal(err)
	}
	defer file.Close()
	if err := json.NewEncoder(file).Encode(map[string]any{"receiver": "LOCAL_VERIFIED_TLS_TEST_FIXTURE", "delivered": result.Delivered, "exact_body_hmac_verified": true, "cold_reopen_repeated_attempts": repeat.Attempted, "ids": seen, "external_deliveries": 0}); err != nil {
		t.Fatal(err)
	}
}
