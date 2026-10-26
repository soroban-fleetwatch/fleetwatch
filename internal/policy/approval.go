// Package policy verifies explicit offline approval statements. It does not
// authenticate chain observations or turn an unsigned policy match into approval.
package policy

import (
	"crypto/ed25519"
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"errors"
	"strings"
	"unicode"
	"unicode/utf8"

	"github.com/soroban-fleetwatch/fleetwatch/internal/jsoninput"
	"github.com/stellar/go-stellar-sdk/strkey"
)

type Statement struct {
	SchemaVersion          int    `json:"schema_version"`
	NetworkHash            string `json:"network_passphrase_hash"`
	Owner                  string `json:"owner_contract"`
	TagHex                 string `json:"tag_hex"`
	FromHash               string `json:"from_hash"`
	ToHash                 string `json:"to_hash"`
	Ticket                 string `json:"ticket"`
	ReviewedArtifactDigest string `json:"reviewed_artifact_digest"`
	ValidFrom              uint32 `json:"valid_from_ledger"`
	ValidUntil             uint32 `json:"valid_until_ledger"`
	Nonce                  string `json:"nonce"`
}
type Signature struct {
	KeyID string `json:"key_id"`
	Value string `json:"signature_base64"`
}
type Envelope struct {
	SchemaVersion int         `json:"schema_version"`
	Statement     Statement   `json:"statement"`
	Signatures    []Signature `json:"signatures"`
}
type Key struct {
	ID         string `json:"key_id"`
	PublicKey  string `json:"public_key_base64"`
	ValidFrom  uint32 `json:"valid_from_ledger"`
	ValidUntil uint32 `json:"valid_until_ledger"`
	RevokedAt  uint32 `json:"revoked_at_ledger,omitempty"`
}
type Trust struct {
	SchemaVersion int      `json:"schema_version"`
	NetworkHash   string   `json:"network_passphrase_hash"`
	Owner         string   `json:"owner_contract"`
	AllowedTags   []string `json:"allowed_tags_hex"`
	Threshold     int      `json:"threshold"`
	Keys          []Key    `json:"keys"`
}
type Observation struct {
	SchemaVersion     int    `json:"schema_version"`
	NetworkPassphrase string `json:"network_passphrase"`
	Owner             string `json:"owner_contract"`
	TagHex            string `json:"tag_hex"`
	FromHash          string `json:"from_hash"`
	ToHash            string `json:"to_hash"`
	Ledger            uint32 `json:"ledger"`
	Completeness      string `json:"completeness"`
	Source            string `json:"source"`
}
type Result struct {
	SchemaVersion          int    `json:"schema_version"`
	PolicyMatch            string `json:"policy_match"`
	SignatureVerified      string `json:"signature_verified"`
	SignerTrusted          string `json:"signer_trusted"`
	ApprovalValidForLedger string `json:"approval_valid_for_ledger"`
	VerificationStatus     string `json:"verification_status"`
	FinalVerdict           string `json:"final_verdict"`
	Reason                 string `json:"reason"`
	StatementDigest        string `json:"statement_digest"`
	TrustedSignatures      int    `json:"trusted_signatures"`
	RequiredSignatures     int    `json:"required_signatures"`
	EvidenceDisclaimer     string `json:"evidence_disclaimer"`
}

func KeyID(public []byte) string { sum := sha256.Sum256(public); return hex.EncodeToString(sum[:]) }
func NetworkHash(passphrase string) string {
	sum := sha256.Sum256([]byte(passphrase))
	return hex.EncodeToString(sum[:])
}
func hexBytes(s string, n int) bool {
	if len(s) != n*2 || s != strings.ToLower(s) {
		return false
	}
	_, err := hex.DecodeString(s)
	return err == nil
}
func tag(s string) bool {
	return len(s) > 0 && len(s) <= 2048 && len(s)%2 == 0 && hexBytes(s, len(s)/2)
}
func contract(s string) bool {
	raw, err := strkey.Decode(strkey.VersionByteContract, s)
	return err == nil && len(raw) == 32
}
func text(s string, limit int) bool {
	if len(s) > limit || strings.TrimSpace(s) == "" || !utf8.ValidString(s) {
		return false
	}
	for _, r := range s {
		if unicode.IsControl(r) {
			return false
		}
	}
	return true
}
func (s Statement) Validate() error {
	if s.SchemaVersion != 1 || !hexBytes(s.NetworkHash, 32) || !contract(s.Owner) || !tag(s.TagHex) || !hexBytes(s.FromHash, 32) || !hexBytes(s.ToHash, 32) || s.FromHash == s.ToHash || !text(s.Ticket, 1024) || s.ReviewedArtifactDigest != s.ToHash || s.ValidFrom == 0 || s.ValidUntil < s.ValidFrom || !hexBytes(s.Nonce, 32) {
		return errors.New("invalid signed approval statement")
	}
	return nil
}

// SigningBytes is the versioned canonical encoding for this custom envelope,
// not DSSE or RFC 8785. All fields are fixed-order Go JSON, no maps or floats.
func SigningBytes(s Statement) ([]byte, error) {
	if err := s.Validate(); err != nil {
		return nil, err
	}
	raw, err := json.Marshal(s)
	if err != nil {
		return nil, err
	}
	return append([]byte("fleetwatch.approval.v1\n"), raw...), nil
}
func (e Envelope) Validate() error {
	if e.SchemaVersion != 1 || len(e.Signatures) == 0 || len(e.Signatures) > 32 {
		return errors.New("invalid approval envelope")
	}
	if err := e.Statement.Validate(); err != nil {
		return err
	}
	seen := map[string]bool{}
	for _, sig := range e.Signatures {
		if len(sig.Value) != 88 {
			return errors.New("invalid signature encoding size")
		}
		raw, err := base64.StdEncoding.Strict().DecodeString(sig.Value)
		if !hexBytes(sig.KeyID, 32) || seen[sig.KeyID] || err != nil || len(raw) != ed25519.SignatureSize || base64.StdEncoding.EncodeToString(raw) != sig.Value {
			return errors.New("invalid or duplicate approval signature")
		}
		seen[sig.KeyID] = true
	}
	return nil
}
func (t Trust) Validate() error {
	if t.SchemaVersion != 1 || !hexBytes(t.NetworkHash, 32) || !contract(t.Owner) || len(t.AllowedTags) == 0 || len(t.AllowedTags) > 32 || len(t.Keys) == 0 || len(t.Keys) > 32 || t.Threshold < 1 || t.Threshold > len(t.Keys) {
		return errors.New("invalid approval trust configuration")
	}
	tags := map[string]bool{}
	for _, value := range t.AllowedTags {
		if !tag(value) || tags[value] {
			return errors.New("invalid or duplicate trusted tag")
		}
		tags[value] = true
	}
	seen := map[string]bool{}
	for _, key := range t.Keys {
		if len(key.PublicKey) != 44 {
			return errors.New("invalid public key encoding size")
		}
		raw, err := base64.StdEncoding.Strict().DecodeString(key.PublicKey)
		if err != nil || len(raw) != ed25519.PublicKeySize || base64.StdEncoding.EncodeToString(raw) != key.PublicKey || key.ID != KeyID(raw) || seen[key.ID] || key.ValidFrom == 0 || key.ValidUntil < key.ValidFrom {
			return errors.New("invalid or duplicate trust key")
		}
		seen[key.ID] = true
	}
	return nil
}
func (o Observation) Validate() error {
	if o.SchemaVersion != 1 || !text(o.NetworkPassphrase, 256) || !contract(o.Owner) || !tag(o.TagHex) || !hexBytes(o.FromHash, 32) || !hexBytes(o.ToHash, 32) || o.FromHash == o.ToHash || o.Ledger == 0 || (o.Completeness != "COMPLETE" && o.Completeness != "PARTIAL" && o.Completeness != "UNKNOWN") || (o.Source != "synthetic" && o.Source != "user_snapshot" && o.Source != "rpc_current") {
		return errors.New("invalid approval observation")
	}
	return nil
}
func LoadEnvelope(path string) (Envelope, error) {
	var e Envelope
	if err := jsoninput.File(path, &e); err != nil {
		return e, err
	}
	return e, e.Validate()
}
func LoadTrust(path string) (Trust, error) {
	var t Trust
	if err := jsoninput.File(path, &t); err != nil {
		return t, err
	}
	return t, t.Validate()
}
func LoadObservation(path string) (Observation, error) {
	var o Observation
	if err := jsoninput.File(path, &o); err != nil {
		return o, err
	}
	return o, o.Validate()
}
func Verify(e Envelope, t Trust, o Observation) (Result, error) {
	r := Result{SchemaVersion: 1, PolicyMatch: "NO", SignatureVerified: "NO", SignerTrusted: "NO", ApprovalValidForLedger: "NO", VerificationStatus: "REJECTED", FinalVerdict: "UNDETERMINED", RequiredSignatures: t.Threshold, EvidenceDisclaimer: "Verifies an approval against supplied observation fields and operator-provided trust roots; chain authenticity, nonce consumption and historical completeness are not verified. No comparison severity or outbox authorization is changed."}
	for _, err := range []error{e.Validate(), t.Validate(), o.Validate()} {
		if err != nil {
			return r, err
		}
	}
	payload, _ := SigningBytes(e.Statement)
	r.StatementDigest = KeyID(payload)
	s := e.Statement
	if s.NetworkHash == NetworkHash(o.NetworkPassphrase) && s.Owner == o.Owner && s.TagHex == o.TagHex && s.FromHash == o.FromHash && s.ToHash == o.ToHash {
		r.PolicyMatch = "YES"
	}
	if o.Ledger >= s.ValidFrom && o.Ledger <= s.ValidUntil {
		r.ApprovalValidForLedger = "YES"
	}
	scopeOK := t.NetworkHash == s.NetworkHash && t.Owner == s.Owner
	allowed := false
	for _, value := range t.AllowedTags {
		if value == s.TagHex {
			allowed = true
		}
	}
	known := map[string]Key{}
	for _, key := range t.Keys {
		known[key.ID] = key
	}
	cryptoCount := 0
	for _, sig := range e.Signatures {
		key, ok := known[sig.KeyID]
		if !ok {
			continue
		}
		public, _ := base64.StdEncoding.DecodeString(key.PublicKey)
		value, _ := base64.StdEncoding.DecodeString(sig.Value)
		if !ed25519.Verify(public, payload, value) {
			continue
		}
		cryptoCount++
		if scopeOK && allowed && o.Ledger >= key.ValidFrom && o.Ledger <= key.ValidUntil && (key.RevokedAt == 0 || o.Ledger < key.RevokedAt) {
			r.TrustedSignatures++
		}
	}
	if cryptoCount >= t.Threshold {
		r.SignatureVerified = "YES"
	}
	if r.TrustedSignatures >= t.Threshold {
		r.SignerTrusted = "YES"
	}
	switch {
	case r.PolicyMatch != "YES":
		r.Reason = "TRANSITION_MISMATCH"
	case !scopeOK || !allowed:
		r.Reason = "TRUST_SCOPE_MISMATCH"
	case r.SignatureVerified != "YES":
		r.Reason = "SIGNATURE_THRESHOLD_NOT_MET"
	case r.SignerTrusted != "YES":
		r.Reason = "TRUSTED_THRESHOLD_NOT_MET"
	case r.ApprovalValidForLedger != "YES":
		r.Reason = "OUTSIDE_APPROVAL_LEDGER_WINDOW"
	case o.Completeness != "COMPLETE":
		r.VerificationStatus = "UNDETERMINED"
		r.Reason = "OBSERVATION_INCOMPLETE"
	default:
		r.VerificationStatus = "VERIFIED"
		r.Reason = "APPROVAL_VERIFIED_FOR_SUPPLIED_FIELDS"
	}
	return r, nil
}
