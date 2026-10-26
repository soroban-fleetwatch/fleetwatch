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
	"path/filepath"
	"strings"
	"sync"
	"sync/atomic"
	"testing"

	"github.com/soroban-fleetwatch/fleetwatch/internal/history"
	"github.com/soroban-fleetwatch/fleetwatch/internal/jsoninput"
	"github.com/soroban-fleetwatch/fleetwatch/internal/store"
)

func TestReconstructionDispatchSignsFindingsAndDiagnostics(t *testing.T) {
	var b history.Baseline
	var capture history.Capture
	if err := jsoninput.File("../../testdata/reconstruction-baseline.json", &b); err != nil {
		t.Fatal(err)
	}
	if err := jsoninput.File("../../testdata/reconstruction-capture.json", &capture); err != nil {
		t.Fatal(err)
	}
	s, err := store.Open(filepath.Join(t.TempDir(), "state.db"))
	if err != nil {
		t.Fatal(err)
	}
	defer s.Close()
	ctx := context.Background()
	if _, err := s.Reconstruct(ctx, "receiver-test", b, capture); err != nil {
		t.Fatal(err)
	}
	partial := capture
	partial.ToLedger = 101
	partial.SequenceCoverage = "PARTIAL"
	partial.Errors = []string{"LEDGER_QUERY_FAILED"}
	if _, err := s.Reconstruct(ctx, "receiver-test", b, partial); !errors.Is(err, store.ErrReconstructionIncomplete) {
		t.Fatal(err)
	}
	var count atomic.Int32
	var diagnostics atomic.Int32
	var seen sync.Map
	server, roots := localHook(t, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		data, err := io.ReadAll(r.Body)
		if err != nil {
			t.Error(err)
			return
		}
		var envelope struct {
			SchemaVersion int                `json:"schema_version"`
			ID            string             `json:"id"`
			Payload       store.AlertPayload `json:"payload"`
		}
		if err := json.Unmarshal(data, &envelope); err != nil {
			t.Error(err)
			return
		}
		if envelope.SchemaVersion != 1 || envelope.Payload.SchemaVersion != 2 || envelope.Payload.Stream != "receiver-test" || envelope.Payload.EvaluationStatus != "UNVERIFIED_PROJECTED_STATE" || envelope.Payload.Assurance.FinalVerdict != "UNDETERMINED" || strings.Contains(string(data), "lease_token") {
			t.Error("unsafe projection delivery")
		}
		if r.Header.Get("Idempotency-Key") != envelope.ID {
			t.Error("wrong idempotency key")
		}
		mac := hmac.New(sha256.New, hookKey())
		mac.Write([]byte("v1\n" + r.Header.Get("X-FleetWatch-Timestamp") + "\n" + envelope.ID + "\n"))
		mac.Write(data)
		signature, err := hex.DecodeString(strings.TrimPrefix(r.Header.Get("X-FleetWatch-Signature"), "v1="))
		if err != nil || !hmac.Equal(signature, mac.Sum(nil)) {
			t.Error("invalid exact-body MAC")
		}
		if _, loaded := seen.LoadOrStore(envelope.ID, true); loaded {
			t.Error("acknowledged alert duplicated")
		}
		if envelope.Payload.Diagnostic != nil {
			diagnostics.Add(1)
		} else if envelope.Payload.Reconstruction.Event.Finding.Authorization != "UNVERIFIED" {
			t.Error("false finding authorization")
		}
		count.Add(1)
		w.WriteHeader(204)
	}))
	sender := testWebhook(t, server, roots)
	source, _ := store.AlertSource(capture.Network, "rpc_ledger_meta", "receiver-test")
	r, err := Dispatch(ctx, s, sender, capture.Network, source, sender.SinkID(), 10)
	if err != nil || r.Delivered != 3 || count.Load() != 3 || diagnostics.Load() != 1 {
		t.Fatal(r, err, count.Load(), diagnostics.Load())
	}
	if _, err := s.Reconstruct(ctx, "receiver-test", b, capture); err != nil {
		t.Fatal(err)
	}
	r, err = Dispatch(ctx, s, sender, capture.Network, source, sender.SinkID(), 10)
	if err != nil || r.Attempted != 0 {
		t.Fatal("replay redelivered acknowledged alerts", r, err)
	}
	rows, err := s.ReconstructionAlerts(ctx, capture.Network, "receiver-test")
	if err != nil || len(rows) != 3 {
		t.Fatal(rows, err)
	}
	for _, row := range rows {
		if row.Status != "DELIVERED" {
			t.Fatal(row)
		}
	}
	bad := rows[0]
	bad.Payload.Assurance.FinalVerdict = "APPROVED"
	if err := sender.Send(ctx, bad); err == nil || count.Load() != 3 {
		t.Fatal("forged assurance was sent", err)
	}
}
