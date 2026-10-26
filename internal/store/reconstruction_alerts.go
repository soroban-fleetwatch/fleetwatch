package store

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"strings"
	"time"

	"github.com/soroban-fleetwatch/fleetwatch/internal/fleetwatch"
	"github.com/soroban-fleetwatch/fleetwatch/internal/history"
	"github.com/soroban-fleetwatch/fleetwatch/internal/jsoninput"
	bolt "go.etcd.io/bbolt"
)

var reconstructionAttemptsBucket = []byte("fleetwatch.reconstruction-attempts.v1")

const reconstructionQueuePrefix = "reconstruction:"

type ProjectionAssurance struct {
	BaselineAuthentication string `json:"baseline_authentication"`
	SourceAuthentication   string `json:"source_authentication"`
	MetadataCommitment     string `json:"metadata_commitment"`
	MembershipCoverage     string `json:"membership_coverage"`
	HistoryCompleteness    string `json:"history_completeness"`
	FinalVerdict           string `json:"final_verdict"`
}

func projectionAssurance() ProjectionAssurance {
	return ProjectionAssurance{"UNVERIFIED", "UNVERIFIED", "NOT_VERIFIED", "UNKNOWN", "UNKNOWN", "UNDETERMINED"}
}

type ReconstructionEventAlert struct {
	BaselineDigest     string               `json:"baseline_digest"`
	BaselineHeaderHash string               `json:"baseline_header_hash"`
	Ledger             uint32               `json:"ledger"`
	HeaderHash         string               `json:"header_hash"`
	MetadataSHA256     string               `json:"metadata_sha256"`
	Event              history.DerivedEvent `json:"event"`
}
type ReconstructionAttempt struct {
	SchemaVersion      int    `json:"schema_version"`
	Network            string `json:"network"`
	Stream             string `json:"stream"`
	BaselineLedger     uint32 `json:"baseline_ledger"`
	BaselineDigest     string `json:"baseline_digest"`
	BaselineHeaderHash string `json:"baseline_header_hash"`
	FromLedger         uint32 `json:"from_ledger"`
	ToLedger           uint32 `json:"to_ledger"`
	RetainedThrough    uint32 `json:"retained_through"`
	InputDigest        string `json:"input_digest"`
	Status             string `json:"status"`
	Code               string `json:"code,omitempty"`
}

func validDigest(s string) bool {
	raw, err := hex.DecodeString(s)
	return err == nil && len(raw) == 32 && s == strings.ToLower(s)
}
func diagnosticCode(code string) bool {
	switch code {
	case "BASELINE_RANGE_OR_NETWORK_MISMATCH", "BASELINE_HEADER_LINK_MISMATCH", "NORMALIZATION_INCOMPLETE", "RECONSTRUCTION_LIMIT", "UNRESOLVED_WATCHED_REFERENCE", "PREIMAGE_MISMATCH", "UNSUPPORTED_FEE_EXECUTABLE_MUTATION", "AMBIGUOUS_GROUP_MUTATION_ORDER", "MISSING_GROUP_PREIMAGE", "INVALID_PROJECTED_STATE", "ENTRY_ALREADY_PRESENT", "WATCHED_INSTANCE_UNAVAILABLE", "WATCHED_REFERENCE_UNAVAILABLE", "UNSUPPORTED_PROJECTED_CHANGE", "UNORDERED_WATCHED_EVICTION", "SOURCE_ATTEMPT_INCOMPLETE", "EVIDENCE_CONFLICT":
		return true
	}
	return false
}
func validateAttempt(a ReconstructionAttempt) error {
	if a.SchemaVersion != 1 || ValidateReconstructionScope(a.Network, a.Stream) != nil || !validDigest(a.BaselineDigest) || !validDigest(a.BaselineHeaderHash) || !validDigest(a.InputDigest) || a.BaselineLedger == 0 || uint64(a.FromLedger) <= uint64(a.BaselineLedger) || a.ToLedger < a.FromLedger || uint64(a.ToLedger)-uint64(a.BaselineLedger) > history.MaxLedgers || a.RetainedThrough < a.BaselineLedger || uint64(a.RetainedThrough)-uint64(a.BaselineLedger) > history.MaxLedgers {
		return errors.New("invalid reconstruction attempt")
	}
	if a.Status == "COMPLETE" && a.Code == "" && a.RetainedThrough >= a.ToLedger {
		return nil
	}
	if a.Status == "PARTIAL" && diagnosticCode(a.Code) {
		return nil
	}
	return errors.New("invalid reconstruction attempt status")
}
func AlertSource(network, source, stream string) (string, error) {
	if source == "rpc_ledger_meta" {
		if err := ValidateReconstructionScope(network, stream); err != nil {
			return "", err
		}
		return reconstructionQueuePrefix + stream, nil
	}
	if stream != "" || (source != "synthetic" && source != "user_snapshot") {
		return "", errors.New("invalid alert source/stream")
	}
	return source, nil
}
func reconstructionAlertID(network, stream string, e ReconstructionEventAlert) string {
	raw, _ := json.Marshal(struct {
		Domain, Network, Stream string
		Event                   ReconstructionEventAlert
	}{"fleetwatch.reconstruction-alert.v1", network, stream, e})
	h := sha256.Sum256(raw)
	return hex.EncodeToString(h[:])
}
func diagnosticAlertID(a ReconstructionAttempt) string {
	raw, _ := json.Marshal(struct {
		Domain  string
		Attempt ReconstructionAttempt
	}{"fleetwatch.reconstruction-diagnostic.v1", a})
	h := sha256.Sum256(raw)
	return hex.EncodeToString(h[:])
}
func makeReconstructionAttempt(stream string, b history.Baseline, c history.Capture, r history.Reconstruction, applicationErr error) ReconstructionAttempt {
	raw, _ := json.Marshal(struct {
		Domain  string
		Capture history.Capture
	}{"fleetwatch.reconstruction-input.v1", c})
	sum := sha256.Sum256(raw)
	digest, _ := fleetwatch.SnapshotDigest(b.Snapshot)
	a := ReconstructionAttempt{SchemaVersion: 1, Network: b.Snapshot.Network, Stream: stream, BaselineLedger: b.Snapshot.Ledger, BaselineDigest: digest, BaselineHeaderHash: b.HeaderHash, FromLedger: c.FromLedger, ToLedger: c.ToLedger, RetainedThrough: r.ProcessedThrough, InputDigest: hex.EncodeToString(sum[:]), Status: "COMPLETE"}
	if applicationErr != nil {
		a.Status = "PARTIAL"
		if len(r.Errors) > 0 {
			a.Code = r.Errors[0]
		}
	}
	return a
}

// ValidateAlertPayload is shared by storage and the sender. HMAC authenticates
// delivery, never the underlying ledger or operator baseline.
func ValidateAlertPayload(id string, p AlertPayload) error {
	if !validDigest(id) || p.Network == "" {
		return errors.New("invalid alert identity")
	}
	if p.SchemaVersion == 1 {
		if p.Stream != "" || p.Assurance != nil || p.Reconstruction != nil || p.Diagnostic != nil || (p.Source != "synthetic" && p.Source != "user_snapshot") || p.EvaluationStatus != "UNVERIFIED_INPUT" || (p.Event == nil) == (p.Issue == nil) {
			return errors.New("invalid legacy alert payload")
		}
		if p.Event != nil && (p.Event.ID != id || p.Event.Finding.Authorization != "UNVERIFIED") {
			return errors.New("invalid legacy event assurance")
		}
		return nil
	}
	if p.SchemaVersion != 2 || p.Source != "rpc_ledger_meta" || ValidateReconstructionScope(p.Network, p.Stream) != nil || p.EvaluationStatus != "UNVERIFIED_PROJECTED_STATE" || p.Assurance == nil || *p.Assurance != projectionAssurance() || p.Event != nil || p.Issue != nil || (p.Reconstruction == nil) == (p.Diagnostic == nil) {
		return errors.New("invalid reconstruction alert payload")
	}
	if p.Diagnostic != nil {
		a := *p.Diagnostic
		if validateAttempt(a) != nil || a.Network != p.Network || a.Stream != p.Stream || a.Status != "PARTIAL" || id != diagnosticAlertID(a) {
			return errors.New("invalid reconstruction diagnostic")
		}
		return nil
	}
	e := p.Reconstruction
	severityOK := map[string]string{"UNAPPROVED_REFERENCE_CHANGE": "critical", "CONTRACT_EXECUTABLE_REPOINTED": "high", "REFERENCE_NOT_OBSERVED": "high", "UNRESOLVED_EXTERNAL_REFERENCE": "high", "CONTRACT_NOT_OBSERVED": "high", "NEW_REFERENCE_OBSERVED": "info", "NEW_CONTRACT_OBSERVED": "info"}[e.Event.Finding.Code] == e.Event.Finding.Severity && e.Event.Finding.Severity != ""
	if e.Ledger == 0 || !validDigest(e.BaselineDigest) || !validDigest(e.BaselineHeaderHash) || !validDigest(e.HeaderHash) || !validDigest(e.MetadataSHA256) || !validDigest(e.Event.GroupID) || e.Event.Finding.Authorization != "UNVERIFIED" || !severityOK {
		return errors.New("invalid reconstruction event")
	}
	// Recompute the existing v1 event identity, including its complete finding.
	raw, _ := json.Marshal(struct {
		Domain, Network, BaselineDigest, BaselineHash, HeaderHash, MetadataSHA256, GroupID string
		Finding                                                                            fleetwatch.Finding
	}{"fleetwatch.reconstruction-event.v1", p.Network, e.BaselineDigest, e.BaselineHeaderHash, e.HeaderHash, e.MetadataSHA256, e.Event.GroupID, e.Event.Finding})
	h := sha256.Sum256(raw)
	if e.Event.ID != hex.EncodeToString(h[:]) || id != reconstructionAlertID(p.Network, p.Stream, *e) {
		return errors.New("reconstruction event identity mismatch")
	}
	return nil
}

func reconstructionEventPayload(network, stream string, r history.Reconstruction, l history.AppliedLedger, e history.DerivedEvent) (string, AlertPayload) {
	assurance := projectionAssurance()
	event := ReconstructionEventAlert{r.BaselineDigest, r.BaselineHeaderHash, l.Sequence, l.HeaderHash, l.MetadataSHA256, e}
	p := AlertPayload{SchemaVersion: 2, Network: network, Source: "rpc_ledger_meta", Stream: stream, EvaluationStatus: "UNVERIFIED_PROJECTED_STATE", Assurance: &assurance, Reconstruction: &event}
	return reconstructionAlertID(network, stream, event), p
}
func loadAttempt(tx *bolt.Tx, network, stream string) (*ReconstructionAttempt, error) {
	b := tx.Bucket(reconstructionAttemptsBucket)
	if b == nil {
		return nil, nil
	}
	raw := b.Get(reconstructionScope(network, stream))
	if raw == nil {
		return nil, nil
	}
	var a ReconstructionAttempt
	if len(raw) > 4096 || jsoninput.Decode(bytes.NewReader(raw), &a) != nil || validateAttempt(a) != nil || a.Network != network || a.Stream != stream {
		return nil, errors.New("invalid stored reconstruction attempt")
	}
	return &a, nil
}
func saveAttempt(tx *bolt.Tx, a ReconstructionAttempt) error {
	if err := validateAttempt(a); err != nil {
		return err
	}
	root, err := tx.CreateBucketIfNotExists(reconstructionAttemptsBucket)
	if err != nil {
		return err
	}
	key := reconstructionScope(a.Network, a.Stream)
	if root.Get(key) == nil {
		keys := map[string]bool{}
		for _, name := range [][]byte{reconstructionBucket, reconstructionAttemptsBucket} {
			b := tx.Bucket(name)
			if b != nil {
				if err := b.ForEach(func(k, v []byte) error { keys[string(k)] = true; return nil }); err != nil {
					return err
				}
			}
		}
		if !keys[string(key)] && len(keys) >= 128 {
			return ErrReconstructionBound
		}
	}
	raw, _ := json.Marshal(a)
	return root.Put(key, raw)
}
func enqueueReconstruction(tx *bolt.Tx, stream string, r history.Reconstruction, attempt ReconstructionAttempt) error {
	source, _ := AlertSource(r.Network, "rpc_ledger_meta", stream)
	b, err := queue(tx, r.Network, source, true)
	if err != nil {
		return err
	}
	count := b.Stats().KeyN
	add := func(id string, p AlertPayload) error {
		if err := ValidateAlertPayload(id, p); err != nil {
			return err
		}
		if raw := b.Get([]byte(id)); raw != nil {
			a, err := decodeAlert(raw, id, r.Network, source)
			if err != nil {
				return err
			}
			old, _ := json.Marshal(a.Payload)
			fresh, _ := json.Marshal(p)
			if !bytes.Equal(old, fresh) {
				return errors.New("stored reconstruction alert differs from evidence")
			}
			return nil
		}
		if count >= 10000 {
			return ErrReconstructionBound
		}
		if err := writeAlert(b, Alert{ID: id, Payload: p, Status: "PENDING"}); err != nil {
			return err
		}
		count++
		return nil
	}
	for _, ledger := range r.Ledgers {
		for _, event := range ledger.Events {
			id, p := reconstructionEventPayload(r.Network, stream, r, ledger, event)
			if err := add(id, p); err != nil {
				return err
			}
		}
	}
	if attempt.Status == "PARTIAL" {
		assurance := projectionAssurance()
		p := AlertPayload{SchemaVersion: 2, Network: r.Network, Source: "rpc_ledger_meta", Stream: stream, EvaluationStatus: "UNVERIFIED_PROJECTED_STATE", Assurance: &assurance, Diagnostic: &attempt}
		return add(diagnosticAlertID(attempt), p)
	}
	return nil
}

// inspectReconstructionQueue checks the accepted event projection and latest
// attempt together with its queue in one transaction. Historical diagnostics stay.
func inspectReconstructionQueue(ctx context.Context, tx *bolt.Tx, network, stream string) ([]Alert, error) {
	rows := []Alert{}
	source, err := AlertSource(network, "rpc_ledger_meta", stream)
	if err != nil {
		return nil, err
	}
	attempt, err := loadAttempt(tx, network, stream)
	if err != nil {
		return nil, err
	}
	var state *ReconstructionState
	if b := tx.Bucket(reconstructionBucket); b != nil {
		if raw := b.Get(reconstructionScope(network, stream)); raw != nil {
			s, err := decodeReconstruction(ctx, raw, network, stream)
			if err != nil {
				return nil, err
			}
			state = &s
		}
	}
	if state != nil && attempt != nil && (state.Report.BaselineDigest != attempt.BaselineDigest || state.Baseline.HeaderHash != attempt.BaselineHeaderHash || state.Baseline.Snapshot.Ledger != attempt.BaselineLedger || state.Report.ProcessedThrough != attempt.RetainedThrough || !state.AlertsEnabled) {
		return nil, errors.New("checkpoint/attempt mismatch")
	}
	if state == nil && attempt != nil && (attempt.Status != "PARTIAL" || attempt.RetainedThrough != attempt.BaselineLedger) {
		return nil, errors.New("attempt refers to missing checkpoint")
	}
	expected := map[string]AlertPayload{}
	if state != nil {
		for _, l := range state.Report.Ledgers {
			for _, e := range l.Events {
				id, p := reconstructionEventPayload(network, stream, state.Report, l, e)
				expected[id] = p
			}
		}
	}
	latestDiagnostic := false
	b, err := queue(tx, network, source, false)
	if err != nil {
		return nil, err
	}
	if b != nil {
		if err := b.ForEach(func(k, raw []byte) error {
			if ctx.Err() != nil {
				return ctx.Err()
			}
			if len(rows) >= 10000 {
				return ErrReconstructionBound
			}
			a, err := decodeAlert(raw, string(k), network, source)
			if err != nil {
				return err
			}
			if a.Payload.Reconstruction != nil {
				p, ok := expected[a.ID]
				old, _ := json.Marshal(a.Payload)
				fresh, _ := json.Marshal(p)
				if !ok || !bytes.Equal(old, fresh) {
					return errors.New("queued finding differs from checkpoint")
				}
				delete(expected, a.ID)
			} else {
				d := a.Payload.Diagnostic
				if attempt == nil || d.BaselineDigest != attempt.BaselineDigest || d.BaselineHeaderHash != attempt.BaselineHeaderHash {
					return errors.New("diagnostic scope mismatch")
				}
				if attempt.Status == "PARTIAL" && a.ID == diagnosticAlertID(*attempt) {
					latestDiagnostic = true
				}
			}
			a.Token = ""
			a.LeaseToken = ""
			rows = append(rows, a)
			return nil
		}); err != nil {
			return nil, err
		}
	}
	if state != nil && state.AlertsEnabled && (attempt == nil || len(expected) > 0) {
		return nil, errors.New("checkpoint missing reconstruction alert records")
	}
	if attempt != nil && attempt.Status == "PARTIAL" && !latestDiagnostic {
		return nil, errors.New("latest reconstruction diagnostic missing")
	}
	return rows, nil
}
func (s *Store) ReconstructionAlerts(ctx context.Context, network, stream string) ([]Alert, error) {
	var rows []Alert
	err := s.db.View(func(tx *bolt.Tx) error {
		var err error
		rows, err = inspectReconstructionQueue(ctx, tx, network, stream)
		return err
	})
	if err != nil {
		return nil, err
	}
	return rows, nil
}
func (s *Store) LastReconstructionAttempt(ctx context.Context, network, stream string) (*ReconstructionAttempt, error) {
	if ValidateReconstructionScope(network, stream) != nil {
		return nil, errors.New("invalid attempt scope")
	}
	var a *ReconstructionAttempt
	err := s.db.View(func(tx *bolt.Tx) error {
		if err := ctx.Err(); err != nil {
			return err
		}
		var err error
		a, err = loadAttempt(tx, network, stream)
		return err
	})
	return a, err
}

func validateQueueBeforeClaim(ctx context.Context, tx *bolt.Tx, network, source string) error {
	if !strings.HasPrefix(source, reconstructionQueuePrefix) {
		return nil
	}
	ctx, cancel := context.WithTimeout(ctx, 30*time.Second)
	defer cancel()
	_, err := inspectReconstructionQueue(ctx, tx, network, strings.TrimPrefix(source, reconstructionQueuePrefix))
	return err
}
