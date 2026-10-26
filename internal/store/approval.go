package store

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"sort"
	"time"

	"github.com/soroban-fleetwatch/fleetwatch/internal/policy"
	bolt "go.etcd.io/bbolt"
)

var ErrApprovalNonceUsed = errors.New("approval nonce already belongs to another binding")
var ErrApprovalEventBound = errors.New("event already has a different approval binding")
var ErrApprovalEventUnavailable = errors.New("supported persisted reference-change event is unavailable")
var approvalBucket = []byte("fleetwatch.offline-approval-binding.v1")
var nonceBucket = []byte("fleetwatch.offline-approval-nonce.v1")

type ApprovalBinding struct {
	SchemaVersion   int             `json:"schema_version"`
	EventID         string          `json:"event_id"`
	Network         string          `json:"network"`
	Source          string          `json:"source"`
	StatementDigest string          `json:"statement_digest"`
	TrustDigest     string          `json:"trust_digest"`
	BoundAt         string          `json:"bound_at"`
	Envelope        policy.Envelope `json:"envelope"`
	Trust           policy.Trust    `json:"trust"`
	Verification    policy.Result   `json:"verification"`
}
type BindResult struct {
	Verification policy.Result    `json:"verification"`
	Binding      *ApprovalBinding `json:"binding,omitempty"`
	NewBinding   bool             `json:"new_binding"`
}
type nonceLink struct {
	EventID         string `json:"event_id"`
	Network         string `json:"network"`
	Source          string `json:"source"`
	StatementDigest string `json:"statement_digest"`
	TrustDigest     string `json:"trust_digest"`
}

func trustDigest(t policy.Trust) string {
	t.Keys = append([]policy.Key{}, t.Keys...)
	t.AllowedTags = append([]string{}, t.AllowedTags...)
	sort.Slice(t.Keys, func(i, j int) bool { return t.Keys[i].ID < t.Keys[j].ID })
	sort.Strings(t.AllowedTags)
	raw, _ := json.Marshal(t)
	hash := sha256.Sum256(append([]byte("fleetwatch.trust.v1\n"), raw...))
	return hex.EncodeToString(hash[:])
}
func nonceScope(e policy.Envelope) []byte {
	raw, _ := json.Marshal([]string{"fleetwatch.approval-nonce-scope.v1", e.Statement.NetworkHash, e.Statement.Owner})
	hash := sha256.Sum256(raw)
	return hash[:]
}

// BindApproval derives observation fields from a persisted event, re-verifies
// against the supplied current trust configuration, and atomically consumes the
// owner/network nonce. This records an offline association, never chain approval.
func (s *Store) BindApproval(network, source, eventID string, e policy.Envelope, t policy.Trust) (BindResult, error) {
	return s.bindApproval(network, source, eventID, e, t, nil)
}
func (s *Store) bindApproval(network, source, eventID string, e policy.Envelope, t policy.Trust, beforeCommit func() error) (BindResult, error) {
	var result BindResult
	if err := e.Validate(); err != nil {
		return result, err
	}
	if err := t.Validate(); err != nil {
		return result, err
	}
	rawID, err := hex.DecodeString(eventID)
	if err != nil || len(rawID) != 32 || hex.EncodeToString(rawID) != eventID {
		return result, ErrApprovalEventUnavailable
	}
	err = s.db.Update(func(tx *bolt.Tx) error {
		states := tx.Bucket(stateBucket)
		if states == nil {
			return ErrApprovalEventUnavailable
		}
		raw := states.Get(scope(network, source))
		if raw == nil {
			return ErrApprovalEventUnavailable
		}
		var state State
		if err := decodeState(raw, &state); err != nil {
			return err
		}
		if state.Journal.Network != network || state.Journal.Source != source {
			return errors.New("approval state scope mismatch")
		}
		observed := policy.Observation{SchemaVersion: 1, NetworkPassphrase: network, Source: source, Completeness: "PARTIAL"}
		found := false
		for _, event := range state.Report.Events {
			if event.ID != eventID {
				continue
			}
			if event.Finding.Code != "UNAPPROVED_REFERENCE_CHANGE" || event.Finding.Authorization != "UNVERIFIED" {
				return ErrApprovalEventUnavailable
			}
			f := event.Finding
			observed.Owner = f.Owner
			observed.TagHex = f.TagHex
			observed.FromHash = f.OldHash
			observed.ToHash = f.NewHash
			observed.Ledger = event.Ledger
			found = true
			break
		}
		if !found {
			return ErrApprovalEventUnavailable
		}
		if state.Report.SequenceCoverage == "COMPLETE_INPUT_SEQUENCE" && len(state.Report.Issues) == 0 {
			observed.Completeness = "COMPLETE"
		}
		if err := observed.Validate(); err != nil {
			return ErrApprovalEventUnavailable
		}
		verified, err := policy.Verify(e, t, observed)
		if err != nil {
			return err
		}
		result.Verification = verified
		if verified.VerificationStatus != "VERIFIED" {
			return nil
		}
		bindings, err := tx.CreateBucketIfNotExists(approvalBucket)
		if err != nil {
			return err
		}
		bindings, err = bindings.CreateBucketIfNotExists(scope(network, source))
		if err != nil {
			return err
		}
		nonces, err := tx.CreateBucketIfNotExists(nonceBucket)
		if err != nil {
			return err
		}
		nonces, err = nonces.CreateBucketIfNotExists(nonceScope(e))
		if err != nil {
			return err
		}
		link := nonceLink{EventID: eventID, Network: network, Source: source, StatementDigest: verified.StatementDigest, TrustDigest: trustDigest(t)}
		nonce := []byte(e.Statement.Nonce)
		previousNonce := nonces.Get(nonce)
		previousEvent := bindings.Get([]byte(eventID))
		if previousNonce != nil {
			if len(previousNonce) > 2048 {
				return errors.New("stored approval nonce exceeds local limit")
			}
			var old nonceLink
			if err := json.Unmarshal(previousNonce, &old); err != nil {
				return errors.New("invalid stored approval nonce")
			}
			if old != link {
				return ErrApprovalNonceUsed
			}
			if previousEvent == nil {
				return errors.New("approval nonce has no binding")
			}
		}
		if previousEvent != nil {
			if len(previousEvent) > 256<<10 {
				return errors.New("stored approval binding exceeds local limit")
			}
			var old ApprovalBinding
			if err := json.Unmarshal(previousEvent, &old); err != nil {
				return errors.New("invalid stored approval binding")
			}
			if old.SchemaVersion != 1 || old.EventID != eventID || old.Network != network || old.Source != source || old.StatementDigest != link.StatementDigest || old.TrustDigest != link.TrustDigest || old.Envelope.Statement.Nonce != e.Statement.Nonce {
				return ErrApprovalEventBound
			}
			if previousNonce == nil {
				return errors.New("approval binding has no nonce")
			}
			if _, err := time.Parse(time.RFC3339, old.BoundAt); err != nil {
				return errors.New("invalid stored binding timestamp")
			}
			checked, err := policy.Verify(old.Envelope, old.Trust, observed)
			if err != nil || checked.VerificationStatus != "VERIFIED" || checked.StatementDigest != old.StatementDigest || trustDigest(old.Trust) != old.TrustDigest || old.Verification != checked {
				return errors.New("stored approval binding validation failed")
			}
			result.Binding = &old
			return nil
		}
		if bindings.Stats().KeyN >= 10000 || nonces.Stats().KeyN >= 10000 {
			return errors.New("approval binding limit reached")
		}
		binding := ApprovalBinding{SchemaVersion: 1, EventID: eventID, Network: network, Source: source, StatementDigest: link.StatementDigest, TrustDigest: link.TrustDigest, BoundAt: time.Now().UTC().Format(time.RFC3339), Envelope: e, Trust: t, Verification: verified}
		data, err := json.Marshal(binding)
		if err != nil {
			return err
		}
		if len(data) > 256<<10 {
			return errors.New("approval binding exceeds local size limit")
		}
		encodedLink, err := json.Marshal(link)
		if err != nil {
			return err
		}
		if err := bindings.Put([]byte(eventID), data); err != nil {
			return err
		}
		if err := nonces.Put(nonce, encodedLink); err != nil {
			return err
		}
		if beforeCommit != nil {
			if err := beforeCommit(); err != nil {
				return err
			}
		}
		result.Binding = &binding
		result.NewBinding = true
		return nil
	})
	if err != nil {
		result.Binding = nil
		result.NewBinding = false
	}
	return result, err
}
