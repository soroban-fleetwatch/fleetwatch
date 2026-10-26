package store

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"strings"
	"time"
	"unicode"
	"unicode/utf8"

	bolt "go.etcd.io/bbolt"
)

var recoveryBucket = []byte("fleetwatch.alert-recoveries.v1")

const MaxRecoveryCycles = 10
const maxRecoveryReceipts = 10000

// RecoveryRequest identifies one operator action. RequestID is scoped to a queue.
type RecoveryRequest struct {
	AlertID       string `json:"alert_id"`
	RequestID     string `json:"request_id"`
	ExpectedCycle int    `json:"expected_cycle"`
	Reason        string `json:"reason"`
}
type RecoveryReceipt struct {
	SchemaVersion int `json:"schema_version"`
	RecoveryRequest
	Network            string `json:"network"`
	Source             string `json:"source"`
	Stream             string `json:"stream,omitempty"`
	PayloadSHA256      string `json:"payload_sha256"`
	PreviousStatus     string `json:"previous_status"`
	PreviousAttempts   int    `json:"previous_attempts"`
	PreviousErrorCode  string `json:"previous_error_code,omitempty"`
	PreviousLeaseUntil int64  `json:"previous_lease_until,omitempty"`
	Sink               string `json:"sink"`
	RecoveryCycle      int    `json:"recovery_cycle"`
	RecoveredAt        string `json:"recovered_at"`
}
type RecoveryResult struct {
	SchemaVersion int             `json:"schema_version"`
	NewRecovery   bool            `json:"new_recovery"`
	Receipt       RecoveryReceipt `json:"receipt"`
}

func validRecoveryRequest(r RecoveryRequest) bool {
	if !validDigest(r.AlertID) || len(r.RequestID) < 1 || len(r.RequestID) > 128 || r.ExpectedCycle < 0 || r.ExpectedCycle >= MaxRecoveryCycles || len(r.Reason) < 1 || len(r.Reason) > 512 || !utf8.ValidString(r.Reason) || strings.TrimSpace(r.Reason) == "" {
		return false
	}
	for _, c := range r.RequestID {
		if !(c >= 'a' && c <= 'z' || c >= 'A' && c <= 'Z' || c >= '0' && c <= '9' || c == '.' || c == '_' || c == '-') {
			return false
		}
	}
	for _, c := range r.Reason {
		if unicode.IsControl(c) {
			return false
		}
	}
	return true
}
func payloadDigest(a Alert) string {
	b, _ := json.Marshal(a.Payload)
	h := sha256.Sum256(b)
	return hex.EncodeToString(h[:])
}
func recoveryQueue(tx *bolt.Tx, network, source string, create bool) (*bolt.Bucket, error) {
	root := tx.Bucket(recoveryBucket)
	if create {
		var err error
		root, err = tx.CreateBucketIfNotExists(recoveryBucket)
		if err != nil {
			return nil, err
		}
		return root.CreateBucketIfNotExists(scope(network, source))
	}
	if root == nil {
		return nil, nil
	}
	return root.Bucket(scope(network, source)), nil
}

// readRecoveries checks the bounded local audit sequence against the alert row.
// It provides consistency checking, not authentication against a DB writer.
func readRecoveries(ctx context.Context, tx *bolt.Tx, network, source string, a Alert) ([]RecoveryReceipt, error) {
	rows := make([]RecoveryReceipt, 0)
	b, err := recoveryQueue(tx, network, source, false)
	if err != nil {
		return nil, err
	}
	count := 0
	if b != nil {
		err = b.ForEach(func(k, v []byte) error {
			if err := ctx.Err(); err != nil {
				return err
			}
			count++
			if count > maxRecoveryReceipts || v == nil || len(v) > 4096 {
				return errors.New("invalid recovery audit bounds")
			}
			var r RecoveryReceipt
			dec := json.NewDecoder(bytes.NewReader(v))
			dec.DisallowUnknownFields()
			if err := dec.Decode(&r); err != nil {
				return err
			}
			var trailing any
			if dec.Decode(&trailing) != io.EOF {
				return errors.New("invalid recovery audit JSON")
			}
			at, timeErr := time.Parse(time.RFC3339Nano, r.RecoveredAt)
			queueSource, scopeErr := AlertSource(r.Network, r.Source, r.Stream)
			if r.SchemaVersion != 1 || !validRecoveryRequest(r.RecoveryRequest) || string(k) != r.RequestID || r.Network != network || scopeErr != nil || queueSource != source || !validDigest(r.PayloadSHA256) || r.RecoveryCycle != r.ExpectedCycle+1 || r.PreviousAttempts < 1 || r.PreviousAttempts > MaxDeliveryAttempts || r.Sink == "" || len(r.Sink) > 256 || timeErr != nil || at.Unix() <= 0 || at.Location() != time.UTC || !(r.PreviousStatus == "DEAD" && r.PreviousLeaseUntil == 0 || r.PreviousStatus == "INFLIGHT" && r.PreviousAttempts == MaxDeliveryAttempts && r.PreviousLeaseUntil > 0 && r.PreviousLeaseUntil <= at.Unix()) {
				return errors.New("invalid recovery audit invariants")
			}
			if r.AlertID == a.ID {
				rows = append(rows, r)
			}
			return nil
		})
		if err != nil {
			return nil, err
		}
	}
	if len(rows) != a.RecoveryCycle {
		return nil, errors.New("missing or excess recovery audit")
	}
	ordered := make([]RecoveryReceipt, len(rows))
	seen := make(map[int]bool)
	for _, r := range rows {
		if r.RecoveryCycle > len(rows) || seen[r.RecoveryCycle] || r.PayloadSHA256 != payloadDigest(a) || r.Sink != a.Sink {
			return nil, errors.New("recovery audit does not match alert")
		}
		seen[r.RecoveryCycle] = true
		ordered[r.RecoveryCycle-1] = r
	}
	return ordered, nil
}
func recoveryAlert(ctx context.Context, tx *bolt.Tx, network, source, id string) (*bolt.Bucket, Alert, error) {
	if err := ctx.Err(); err != nil {
		return nil, Alert{}, err
	}
	if err := validateQueueBeforeClaim(ctx, tx, network, source); err != nil {
		return nil, Alert{}, err
	}
	b, err := queue(tx, network, source, false)
	if err != nil {
		return nil, Alert{}, err
	}
	if b == nil {
		return nil, Alert{}, errors.New("alert not found")
	}
	a, err := decodeAlert(b.Get([]byte(id)), id, network, source)
	if err != nil {
		return nil, Alert{}, err
	}
	if a.Payload.SchemaVersion == 1 {
		var state State
		if err := decodeState(tx.Bucket(stateBucket).Get(scope(network, source)), &state); err != nil {
			return nil, Alert{}, err
		}
		if state.Report.Network != network || state.Report.Source != source {
			return nil, Alert{}, errors.New("replay scope mismatch")
		}
		if a.Payload.Event != nil {
			found := false
			want, _ := json.Marshal(a.Payload.Event)
			for _, e := range state.Report.Events {
				raw, _ := json.Marshal(e)
				if bytes.Equal(raw, want) {
					found = true
					break
				}
			}
			if !found {
				return nil, Alert{}, errors.New("alert evidence mismatch")
			}
		} else if issueAlertID(network, source, *a.Payload.Issue) != a.ID {
			return nil, Alert{}, errors.New("diagnostic identity mismatch")
		}
	}
	return b, a, nil
}

// Requeue makes no network request and never changes the payload or bound sink.
func (s *Store) Requeue(ctx context.Context, network, source string, r RecoveryRequest, now time.Time) (RecoveryResult, error) {
	return s.requeue(ctx, network, source, r, now, nil)
}
func (s *Store) requeue(ctx context.Context, network, source string, r RecoveryRequest, now time.Time, beforeCommit func() error) (RecoveryResult, error) {
	var result RecoveryResult
	if !validRecoveryRequest(r) || network == "" || now.Unix() <= 0 {
		return result, errors.New("invalid recovery request")
	}
	err := s.db.Update(func(tx *bolt.Tx) error {
		b, a, err := recoveryAlert(ctx, tx, network, source, r.AlertID)
		if err != nil {
			return err
		}
		receipts, err := readRecoveries(ctx, tx, network, source, a)
		if err != nil {
			return err
		}
		rb, err := recoveryQueue(tx, network, source, true)
		if err != nil {
			return err
		}
		if data := rb.Get([]byte(r.RequestID)); data != nil {
			for _, receipt := range receipts {
				if receipt.RequestID == r.RequestID {
					if receipt.RecoveryRequest != r {
						return errors.New("recovery request ID conflict")
					}
					result = RecoveryResult{1, false, receipt}
					return nil
				}
			}
			return errors.New("recovery request ID conflict")
		}
		if a.RecoveryCycle != r.ExpectedCycle || a.RecoveryCycle >= MaxRecoveryCycles {
			return errors.New("stale recovery cycle or cycle limit")
		}
		if a.Status != "DEAD" && !(a.Status == "INFLIGHT" && a.Attempts == MaxDeliveryAttempts && a.LeaseUntil <= now.Unix()) {
			return errors.New("alert is not dead or exhausted with an expired lease")
		}
		if a.Sink == "" || a.Attempts == 0 {
			return errors.New("dead alert has no delivery history")
		}
		count := 0
		if err := rb.ForEach(func(k, v []byte) error { count++; return nil }); err != nil {
			return err
		}
		if count >= maxRecoveryReceipts {
			return errors.New("recovery audit capacity reached")
		}
		receipt := RecoveryReceipt{SchemaVersion: 1, RecoveryRequest: r, Network: network, Source: a.Payload.Source, Stream: a.Payload.Stream, PayloadSHA256: payloadDigest(a), PreviousStatus: a.Status, PreviousAttempts: a.Attempts, PreviousErrorCode: a.LastErrorCode, PreviousLeaseUntil: a.LeaseUntil, Sink: a.Sink, RecoveryCycle: a.RecoveryCycle + 1, RecoveredAt: now.UTC().Format(time.RFC3339Nano)}
		raw, err := json.Marshal(receipt)
		if err != nil {
			return err
		}
		if len(raw) > 4096 {
			return errors.New("recovery receipt exceeds bound")
		}
		if err := rb.Put([]byte(r.RequestID), raw); err != nil {
			return err
		}
		a.Status = "PENDING"
		a.RecoveryCycle++
		a.Attempts = 0
		a.NextAttempt = 0
		a.LeaseUntil = 0
		a.Token = ""
		a.LeaseToken = ""
		a.LastErrorCode = ""
		if err := writeAlert(b, a); err != nil {
			return err
		}
		if beforeCommit != nil {
			if err := beforeCommit(); err != nil {
				return err
			}
		}
		if err := ctx.Err(); err != nil {
			return err
		}
		result = RecoveryResult{1, true, receipt}
		return nil
	})
	if err != nil {
		return RecoveryResult{}, fmt.Errorf("recovery refused: %w", err)
	}
	return result, nil
}

func (s *Store) Recoveries(ctx context.Context, network, source, id string) ([]RecoveryReceipt, error) {
	var rows []RecoveryReceipt
	if !validDigest(id) {
		return nil, errors.New("invalid alert ID")
	}
	err := s.db.View(func(tx *bolt.Tx) error {
		_, a, err := recoveryAlert(ctx, tx, network, source, id)
		if err != nil {
			return err
		}
		rows, err = readRecoveries(ctx, tx, network, source, a)
		return err
	})
	return rows, err
}
