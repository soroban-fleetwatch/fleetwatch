package store

import (
	"bytes"
	"context"
	"crypto/rand"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"io"
	"time"

	"github.com/soroban-fleetwatch/fleetwatch/internal/fleetwatch"
	bolt "go.etcd.io/bbolt"
)

var outboxBucket = []byte("fleetwatch.offline-outbox.v1")
var ErrStaleLease = errors.New("alert lease expired or superseded")

const MaxDeliveryAttempts = 5

type AlertPayload struct {
	SchemaVersion    int                       `json:"schema_version"`
	Network          string                    `json:"network"`
	Source           string                    `json:"source"`
	EvaluationStatus string                    `json:"evaluation_status"`
	Event            *fleetwatch.ReplayEvent   `json:"event,omitempty"`
	Issue            *fleetwatch.ReplayIssue   `json:"issue,omitempty"`
	Stream           string                    `json:"stream,omitempty"`
	Assurance        *ProjectionAssurance      `json:"assurance,omitempty"`
	Reconstruction   *ReconstructionEventAlert `json:"reconstruction_event,omitempty"`
	Diagnostic       *ReconstructionAttempt    `json:"reconstruction_diagnostic,omitempty"`
}
type Alert struct {
	ID            string       `json:"id"`
	Payload       AlertPayload `json:"payload"`
	Status        string       `json:"status"`
	Attempts      int          `json:"attempts"`
	RecoveryCycle int          `json:"recovery_cycle,omitempty"`
	Sink          string       `json:"sink,omitempty"`
	NextAttempt   int64        `json:"next_attempt,omitempty"`
	LeaseUntil    int64        `json:"lease_until,omitempty"`
	LeaseToken    string       `json:"-"`
	Token         string       `json:"lease_token,omitempty"` // persistence only; list hides it
	LastErrorCode string       `json:"last_error_code,omitempty"`
}

func decodeAlert(data []byte, key, network, source string) (Alert, error) {
	var a Alert
	if len(data) > 1<<20 {
		return a, errors.New("oversized stored alert")
	}
	dec := json.NewDecoder(bytes.NewReader(data))
	dec.DisallowUnknownFields()
	if err := dec.Decode(&a); err != nil {
		return a, err
	}
	var trailing any
	if err := dec.Decode(&trailing); err != io.EOF {
		return a, errors.New("invalid stored alert JSON")
	}
	raw, err := hex.DecodeString(a.ID)
	expectedSource := a.Payload.Source
	if a.Payload.SchemaVersion == 2 {
		expectedSource = reconstructionQueuePrefix + a.Payload.Stream
	}
	if a.Payload.SchemaVersion == 3 {
		expectedSource = followQueuePrefix + a.Payload.Stream
	}
	if err != nil || len(raw) != 32 || a.ID != key || a.Payload.Network != network || expectedSource != source || ValidateAlertPayload(a.ID, a.Payload) != nil || a.RecoveryCycle < 0 || a.RecoveryCycle > MaxRecoveryCycles || a.Attempts < 0 || a.Attempts > MaxDeliveryAttempts || a.NextAttempt < 0 || a.LeaseUntil < 0 {
		return a, errors.New("invalid stored alert invariants")
	}
	switch a.Status {
	case "PENDING", "DELIVERED", "DEAD":
		if a.Token != "" || a.LeaseUntil != 0 {
			return a, errors.New("invalid inactive alert lease")
		}
	case "INFLIGHT":
		if len(a.Token) != 32 || a.LeaseUntil == 0 || a.Attempts == 0 {
			return a, errors.New("invalid active alert lease")
		}
	default:
		return a, errors.New("unknown stored alert status")
	}
	return a, nil
}

func queue(tx *bolt.Tx, network, source string, create bool) (*bolt.Bucket, error) {
	root := tx.Bucket(outboxBucket)
	if create {
		var err error
		root, err = tx.CreateBucketIfNotExists(outboxBucket)
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
func writeAlert(b *bolt.Bucket, a Alert) error {
	data, err := json.Marshal(a)
	if err != nil {
		return err
	}
	if len(data) > 1<<20 {
		return errors.New("alert exceeds 1 MiB local limit")
	}
	return b.Put([]byte(a.ID), data)
}
func enqueue(tx *bolt.Tx, r fleetwatch.ReplayReport) error {
	b, err := queue(tx, r.Network, r.Source, true)
	if err != nil {
		return err
	}
	count := b.Stats().KeyN
	add := func(id string, payload AlertPayload) error {
		if raw := b.Get([]byte(id)); raw != nil {
			_, err := decodeAlert(raw, id, r.Network, r.Source)
			return err
		}
		if count >= fleetwatch.MaxRecords {
			return errors.New("outbox exceeds 10000 alerts per scope")
		}
		if err := writeAlert(b, Alert{ID: id, Payload: payload, Status: "PENDING"}); err != nil {
			return err
		}
		count++
		return nil
	}
	for _, event := range r.Events {
		event := event
		if err := add(event.ID, AlertPayload{SchemaVersion: 1, Network: r.Network, Source: r.Source, EvaluationStatus: r.EvaluationStatus, Event: &event}); err != nil {
			return err
		}
	}
	for _, issue := range r.Issues {
		issue := issue
		if err := add(issueAlertID(r.Network, r.Source, issue), AlertPayload{SchemaVersion: 1, Network: r.Network, Source: r.Source, EvaluationStatus: r.EvaluationStatus, Issue: &issue}); err != nil {
			return err
		}
	}
	return nil
}
func issueAlertID(network, source string, issue fleetwatch.ReplayIssue) string {
	raw, _ := json.Marshal(struct {
		Domain, Network, Source string
		Issue                   fleetwatch.ReplayIssue
	}{"fleetwatch.offline-issue.v1", network, source, issue})
	hash := sha256.Sum256(raw)
	return hex.EncodeToString(hash[:])
}
func (s *Store) Alerts(network, source string) ([]Alert, error) {
	result := []Alert{}
	err := s.db.View(func(tx *bolt.Tx) error {
		b, err := queue(tx, network, source, false)
		if err != nil || b == nil {
			return err
		}
		return b.ForEach(func(key, data []byte) error {
			a, err := decodeAlert(data, string(key), network, source)
			if err != nil {
				return err
			}
			a.Token = ""
			a.LeaseToken = ""
			result = append(result, a)
			return nil
		})
	})
	return result, err
}

// Claim grants one bounded lease in the same transaction that increments the
// attempt counter. Expired leases can be reclaimed; acknowledgments are fenced.
func (s *Store) Claim(network, source, sink string, now time.Time, lease time.Duration) (*Alert, error) {
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	return s.ClaimContext(ctx, network, source, sink, now, lease)
}

func (s *Store) ClaimContext(ctx context.Context, network, source, sink string, now time.Time, lease time.Duration) (*Alert, error) {
	started := time.Now()
	if sink == "" || len(sink) > 256 || now.Unix() <= 0 || lease < time.Second || lease > 5*time.Minute {
		return nil, errors.New("invalid claim configuration")
	}
	var claimed *Alert
	err := s.db.Update(func(tx *bolt.Tx) error {
		if ctx.Err() != nil {
			return ctx.Err()
		}
		if err := validateQueueBeforeClaim(ctx, tx, network, source); err != nil {
			return err
		}
		b, err := queue(tx, network, source, false)
		if err != nil || b == nil {
			return err
		}
		cursor := b.Cursor()
		for key, data := cursor.First(); key != nil; key, data = cursor.Next() {
			if ctx.Err() != nil {
				return ctx.Err()
			}
			grantTime := now.Add(time.Since(started))
			a, err := decodeAlert(data, string(key), network, source)
			if err != nil {
				return err
			}
			if a.Status == "DELIVERED" || a.Status == "DEAD" {
				continue
			}
			if a.Status != "PENDING" && a.Status != "INFLIGHT" {
				return errors.New("invalid outbox status")
			}
			if a.Status == "INFLIGHT" && a.LeaseUntil > grantTime.Unix() {
				continue
			}
			if a.NextAttempt > grantTime.Unix() {
				continue
			}
			if a.Sink != "" && a.Sink != sink {
				return errors.New("alert already bound to another sink")
			}
			if a.Attempts >= MaxDeliveryAttempts {
				a.Status = "DEAD"
				a.Token = ""
				a.LeaseUntil = 0
				a.LastErrorCode = "ATTEMPT_LIMIT"
				if err := writeAlert(b, a); err != nil {
					return err
				}
				continue
			}
			var token [16]byte
			if _, err := rand.Read(token[:]); err != nil {
				return err
			}
			a.Status = "INFLIGHT"
			a.Sink = sink
			a.Attempts++
			a.Token = hex.EncodeToString(token[:])
			a.LeaseUntil = grantTime.Add(lease).Unix()
			a.NextAttempt = 0
			if err := writeAlert(b, a); err != nil {
				return err
			}
			a.LeaseToken = a.Token
			a.Token = ""
			claimed = &a
			break
		}
		return nil
	})
	return claimed, err
}
func (s *Store) Resolve(network, source, id, token string, success, permanent bool, now time.Time) error {
	return s.db.Update(func(tx *bolt.Tx) error {
		b, err := queue(tx, network, source, false)
		if err != nil {
			return err
		}
		if b == nil {
			return ErrStaleLease
		}
		data := b.Get([]byte(id))
		if data == nil {
			return ErrStaleLease
		}
		a, err := decodeAlert(data, id, network, source)
		if err != nil {
			return err
		}
		if a.Status != "INFLIGHT" || token == "" || token != a.Token || now.Unix() >= a.LeaseUntil {
			return ErrStaleLease
		}
		a.Token = ""
		a.LeaseUntil = 0
		a.NextAttempt = 0
		if success {
			a.Status = "DELIVERED"
			a.LastErrorCode = ""
		} else if permanent || a.Attempts >= MaxDeliveryAttempts {
			a.Status = "DEAD"
			a.LastErrorCode = "PERMANENT_DELIVERY_FAILURE"
		} else {
			a.Status = "PENDING"
			a.LastErrorCode = "TRANSIENT_DELIVERY_FAILURE"
			a.NextAttempt = now.Add(time.Duration(1<<a.Attempts) * time.Second).Unix()
		}
		return writeAlert(b, a)
	})
}
