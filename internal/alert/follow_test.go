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
	"path/filepath"
	"sort"
	"strings"
	"sync/atomic"
	"testing"

	"github.com/soroban-fleetwatch/fleetwatch/internal/follow"
	"github.com/soroban-fleetwatch/fleetwatch/internal/history"
	"github.com/soroban-fleetwatch/fleetwatch/internal/jsoninput"
	"github.com/soroban-fleetwatch/fleetwatch/internal/store"
)

func TestFollowDeliverySignsPayloadAndRetainsAcknowledgements(t *testing.T) {
	ctx := context.Background()
	var baseline history.Baseline
	var capture history.Capture
	baselinePath, capturePath := "../../testdata/reconstruction-baseline.json", "../../testdata/reconstruction-capture.json"
	if os.Getenv("FLEETWATCH_FOLLOW_TEST_BASELINE") != "" || os.Getenv("FLEETWATCH_FOLLOW_TEST_CAPTURE") != "" {
		baselinePath, capturePath = os.Getenv("FLEETWATCH_FOLLOW_TEST_BASELINE"), os.Getenv("FLEETWATCH_FOLLOW_TEST_CAPTURE")
		if baselinePath == "" || capturePath == "" {
			t.Fatal("both optional captured baseline and capture paths are required")
		}
	}
	if jsoninput.File(baselinePath, &baseline) != nil || jsoninput.File(capturePath, &capture) != nil {
		t.Fatal("fixtures unavailable")
	}
	native, err := follow.Open(ctx, filepath.Join(t.TempDir(), "follow.db"), baseline, follow.Limits{MaxLedgers: 100000, MaxPayloadBytes: 256 << 20})
	if err != nil {
		t.Fatal(err)
	}
	defer native.Close()
	if _, err := native.Commit(ctx, capture); err != nil {
		t.Fatal(err)
	}
	head := native.Head()
	findings, more, err := native.Findings(ctx, "", 100)
	if err != nil || more || len(findings) == 0 {
		t.Fatal("bounded nonempty findings required", err)
	}
	events := []store.ReconstructionEventAlert{}
	for _, f := range findings {
		events = append(events, store.ReconstructionEventAlert{BaselineDigest: f.BaselineDigest, BaselineHeaderHash: f.BaselineHeaderHash, Ledger: f.Ledger, HeaderHash: f.HeaderHash, MetadataSHA256: f.MetadataSHA256, Event: f.Event})
	}
	sort.Slice(events, func(i, j int) bool { return events[i].Event.ID < events[j].Event.ID })
	binding := store.FollowBinding{Network: capture.Network, Stream: "follow-local-tls", BaselineDigest: head.Projection.OriginDigest, BaselineHeaderHash: baseline.HeaderHash, BaselineLedger: baseline.Snapshot.Ledger, ProcessedThrough: head.Projection.Snapshot.Ledger, HeaderHash: head.Projection.HeaderHash}
	path := filepath.Join(t.TempDir(), "delivery.db")
	s, err := store.Open(path)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := s.ImportFollow(ctx, binding, events, nil); err != nil {
		t.Fatal(err)
	}
	var received atomic.Int32
	server, roots := localHook(t, http.HandlerFunc(func(w http.ResponseWriter, req *http.Request) {
		body, err := io.ReadAll(req.Body)
		if err != nil {
			t.Error(err)
			return
		}
		var envelope struct {
			SchemaVersion int                `json:"schema_version"`
			ID            string             `json:"id"`
			Payload       store.AlertPayload `json:"payload"`
		}
		if json.Unmarshal(body, &envelope) != nil || envelope.SchemaVersion != 1 || envelope.Payload.SchemaVersion != 3 || envelope.Payload.Source != "follow_ledger_meta" || store.ValidateAlertPayload(envelope.ID, envelope.Payload) != nil || req.Header.Get("Idempotency-Key") != envelope.ID || strings.Contains(string(body), "lease_token") {
			t.Error("invalid follow envelope")
			w.WriteHeader(400)
			return
		}
		mac := hmac.New(sha256.New, hookKey())
		mac.Write([]byte("v1\n" + req.Header.Get("X-FleetWatch-Timestamp") + "\n" + envelope.ID + "\n"))
		mac.Write(body)
		actual, err := hex.DecodeString(strings.TrimPrefix(req.Header.Get("X-FleetWatch-Signature"), "v1="))
		if err != nil || !hmac.Equal(actual, mac.Sum(nil)) || envelope.Payload.Assurance.FinalVerdict != "UNDETERMINED" || envelope.Payload.Reconstruction.Event.Finding.Authorization != "UNVERIFIED" {
			t.Error("invalid follow signature or assurance")
		}
		received.Add(1)
		w.WriteHeader(204)
	}))
	sender := testWebhook(t, server, roots)
	source, _ := store.AlertSource(binding.Network, "follow_ledger_meta", binding.Stream)
	result, err := Dispatch(ctx, s, sender, binding.Network, source, sender.SinkID(), 10)
	if err != nil || result.Delivered != len(events) || int(received.Load()) != len(events) {
		t.Fatal(result, err)
	}
	s.Close()
	s, err = store.Open(path)
	if err != nil {
		t.Fatal(err)
	}
	defer s.Close()
	if _, err := s.ImportFollow(ctx, binding, events, &binding); err != nil {
		t.Fatal(err)
	}
	result, err = Dispatch(ctx, s, sender, binding.Network, source, sender.SinkID(), 10)
	if err != nil || result.Attempted != 0 || int(received.Load()) != len(events) {
		t.Fatal("reopened/resynced findings redelivered", result, err)
	}
	rows, err := s.FollowAlerts(ctx, binding.Network, binding.Stream)
	if err != nil {
		t.Fatal(err)
	}
	bad := rows[0]
	bad.Payload.Assurance.FinalVerdict = "APPROVED"
	if err := sender.Send(ctx, bad); err == nil || int(received.Load()) != len(events) {
		t.Fatal("forged assurance sent")
	}
	if output := os.Getenv("FLEETWATCH_FOLLOW_DELIVERY_REPORT"); output != "" {
		data, err := json.MarshalIndent(map[string]any{"baseline_ledger": baseline.Snapshot.Ledger, "processed_through": binding.ProcessedThrough, "retained_ledgers": head.RetainedLedgers,
			"delivered_findings": len(events), "received_requests": received.Load(), "reopened_resync_attempted": result.Attempted, "hmac_and_idempotency_verified": true, "forged_assurance_rejected": true,
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
