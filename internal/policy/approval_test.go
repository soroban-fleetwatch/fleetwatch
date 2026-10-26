package policy

import (
	"crypto/ed25519"
	"crypto/rand"
	"encoding/base64"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/stellar/go-stellar-sdk/strkey"
)

func fixture(t *testing.T) (Envelope, Trust, Observation, ed25519.PrivateKey) {
	t.Helper()
	public, private, err := ed25519.GenerateKey(rand.Reader)
	if err != nil {
		t.Fatal(err)
	}
	owner, err := strkey.Encode(strkey.VersionByteContract, make([]byte, 32))
	if err != nil {
		t.Fatal(err)
	}
	o := Observation{SchemaVersion: 1, NetworkPassphrase: "TESTNET-SYNTHETIC", Owner: owner, TagHex: "ff0061", FromHash: strings.Repeat("a", 64), ToHash: strings.Repeat("b", 64), Ledger: 101, Completeness: "COMPLETE", Source: "synthetic"}
	statement := Statement{SchemaVersion: 1, NetworkHash: NetworkHash(o.NetworkPassphrase), Owner: o.Owner, TagHex: o.TagHex, FromHash: o.FromHash, ToHash: o.ToHash, Ticket: "TEST-ONLY-1", ReviewedArtifactDigest: o.ToHash, ValidFrom: 100, ValidUntil: 300, Nonce: strings.Repeat("c", 64)}
	envelope := Envelope{SchemaVersion: 1, Statement: statement, Signatures: []Signature{{KeyID: KeyID(public)}}}
	sign(t, &envelope, private)
	trust := Trust{SchemaVersion: 1, NetworkHash: statement.NetworkHash, Owner: o.Owner, AllowedTags: []string{o.TagHex}, Threshold: 1, Keys: []Key{{ID: KeyID(public), PublicKey: base64.StdEncoding.EncodeToString(public), ValidFrom: 1, ValidUntil: 1000}}}
	return envelope, trust, o, private
}
func sign(t *testing.T, e *Envelope, key ed25519.PrivateKey) {
	t.Helper()
	raw, err := SigningBytes(e.Statement)
	if err != nil {
		t.Fatal(err)
	}
	public := key.Public().(ed25519.PublicKey)
	e.Signatures = []Signature{{KeyID: KeyID(public), Value: base64.StdEncoding.EncodeToString(ed25519.Sign(key, raw))}}
}
func verify(t *testing.T, e Envelope, trust Trust, o Observation) Result {
	t.Helper()
	r, err := Verify(e, trust, o)
	if err != nil {
		t.Fatal(err)
	}
	return r
}
func TestValidApprovalNeverAuthenticatesChain(t *testing.T) {
	e, trust, o, _ := fixture(t)
	for _, source := range []string{"synthetic", "user_snapshot", "rpc_current"} {
		o.Source = source
		r := verify(t, e, trust, o)
		if r.VerificationStatus != "VERIFIED" || r.PolicyMatch != "YES" || r.SignatureVerified != "YES" || r.SignerTrusted != "YES" || r.ApprovalValidForLedger != "YES" || r.FinalVerdict != "UNDETERMINED" || r.TrustedSignatures != 1 {
			t.Fatalf("%+v", r)
		}
	}
}
func TestSignatureBindsEveryStatementField(t *testing.T) {
	original, trust, o, _ := fixture(t)
	mutations := []func(*Statement){func(s *Statement) { s.NetworkHash = strings.Repeat("d", 64) }, func(s *Statement) {
		s.Owner, _ = strkey.Encode(strkey.VersionByteContract, []byte(strings.Repeat("x", 32)))
	}, func(s *Statement) { s.TagHex = "ff0062" }, func(s *Statement) { s.FromHash = strings.Repeat("d", 64) }, func(s *Statement) { s.ToHash = strings.Repeat("d", 64); s.ReviewedArtifactDigest = s.ToHash }, func(s *Statement) { s.Ticket = "CHANGED" }, func(s *Statement) { s.ValidFrom++ }, func(s *Statement) { s.ValidUntil-- }, func(s *Statement) { s.Nonce = strings.Repeat("d", 64) }}
	for i, mutate := range mutations {
		e := original
		mutate(&e.Statement)
		r := verify(t, e, trust, o)
		if r.SignatureVerified != "NO" || r.VerificationStatus == "VERIFIED" {
			t.Fatalf("field %d not bound: %+v", i, r)
		}
	}
}
func TestWrongTransitionNetworkAndTrustScope(t *testing.T) {
	e, trust, o, _ := fixture(t)
	wrong := o
	wrong.NetworkPassphrase = "wrong network"
	if r := verify(t, e, trust, wrong); r.PolicyMatch != "NO" || r.Reason != "TRANSITION_MISMATCH" {
		t.Fatalf("%+v", r)
	}
	wrong = o
	wrong.FromHash, wrong.ToHash = wrong.ToHash, wrong.FromHash
	if r := verify(t, e, trust, wrong); r.PolicyMatch != "NO" {
		t.Fatal("reverse transition approved")
	}
	trust.AllowedTags = []string{"00"}
	if r := verify(t, e, trust, o); r.SignerTrusted != "NO" || r.Reason != "TRUST_SCOPE_MISMATCH" {
		t.Fatalf("%+v", r)
	}
}
func TestExpiredFutureAndIncompleteObservation(t *testing.T) {
	e, trust, o, _ := fixture(t)
	for _, ledger := range []uint32{99, 301} {
		o.Ledger = ledger
		r := verify(t, e, trust, o)
		if r.ApprovalValidForLedger != "NO" || r.VerificationStatus == "VERIFIED" {
			t.Fatalf("%+v", r)
		}
	}
	o.Ledger = 101
	for _, coverage := range []string{"PARTIAL", "UNKNOWN"} {
		o.Completeness = coverage
		r := verify(t, e, trust, o)
		if r.VerificationStatus != "UNDETERMINED" || r.SignatureVerified != "YES" || r.FinalVerdict != "UNDETERMINED" {
			t.Fatalf("%+v", r)
		}
	}
}
func TestRevocationAndKeyRotationAtLedgerBoundary(t *testing.T) {
	e, trust, o, _ := fixture(t)
	trust.Keys[0].RevokedAt = 101
	o.Ledger = 100
	if r := verify(t, e, trust, o); r.VerificationStatus != "VERIFIED" {
		t.Fatalf("before revocation: %+v", r)
	}
	o.Ledger = 101
	if r := verify(t, e, trust, o); r.SignatureVerified != "YES" || r.SignerTrusted != "NO" {
		t.Fatalf("revoked: %+v", r)
	}
	trust.Keys[0].RevokedAt = 0
	trust.Keys[0].ValidUntil = 100
	public, private, _ := ed25519.GenerateKey(rand.Reader)
	trust.Keys = append(trust.Keys, Key{ID: KeyID(public), PublicKey: base64.StdEncoding.EncodeToString(public), ValidFrom: 101, ValidUntil: 1000})
	if r := verify(t, e, trust, o); r.SignerTrusted != "NO" {
		t.Fatal("retired key accepted")
	}
	sign(t, &e, private)
	if r := verify(t, e, trust, o); r.VerificationStatus != "VERIFIED" {
		t.Fatalf("new key rejected: %+v", r)
	}
}
func TestThresholdRequiresDistinctActiveTrustedKeys(t *testing.T) {
	e, trust, o, _ := fixture(t)
	original := e.Signatures[0]
	public, private, _ := ed25519.GenerateKey(rand.Reader)
	trust.Keys = append(trust.Keys, Key{ID: KeyID(public), PublicKey: base64.StdEncoding.EncodeToString(public), ValidFrom: 1, ValidUntil: 1000})
	trust.Threshold = 2
	if r := verify(t, e, trust, o); r.VerificationStatus == "VERIFIED" {
		t.Fatal("single signer met threshold 2")
	}
	e.Signatures = append(e.Signatures, original)
	if err := e.Validate(); err == nil {
		t.Fatal("duplicate signer accepted")
	}
	sign(t, &e, private)
	e.Signatures = append(e.Signatures, original)
	if r := verify(t, e, trust, o); r.TrustedSignatures != 2 || r.VerificationStatus != "VERIFIED" {
		t.Fatalf("%+v", r)
	}
	trust.Keys[1].RevokedAt = 101
	if r := verify(t, e, trust, o); r.SignerTrusted != "NO" {
		t.Fatal("revoked signer counted")
	}
}
func TestUnknownSignerAndForgedSignature(t *testing.T) {
	e, trust, o, _ := fixture(t)
	_, private, _ := ed25519.GenerateKey(rand.Reader)
	sign(t, &e, private)
	if r := verify(t, e, trust, o); r.VerificationStatus == "VERIFIED" {
		t.Fatal("unknown signer accepted")
	}
	e, trust, o, _ = fixture(t)
	raw, _ := base64.StdEncoding.DecodeString(e.Signatures[0].Value)
	raw[0] ^= 1
	e.Signatures[0].Value = base64.StdEncoding.EncodeToString(raw)
	if r := verify(t, e, trust, o); r.SignatureVerified != "NO" {
		t.Fatal("forged signature accepted")
	}
}
func TestValidationRejectsAmbiguousAndUnsignedInputs(t *testing.T) {
	e, trust, o, _ := fixture(t)
	duplicate := trust
	duplicate.Keys = append(append([]Key{}, trust.Keys...), trust.Keys[0])
	if err := duplicate.Validate(); err == nil {
		t.Fatal("duplicate trust key accepted")
	}
	bad := e
	bad.Statement.TagHex = "FF0061"
	if err := bad.Validate(); err == nil {
		t.Fatal("noncanonical tag accepted")
	}
	bad = e
	bad.Statement.ReviewedArtifactDigest = strings.Repeat("e", 64)
	if err := bad.Validate(); err == nil {
		t.Fatal("reviewed hash mismatch accepted")
	}
	o.Source = "verified_ledger"
	if err := o.Validate(); err == nil {
		t.Fatal("fabricated trusted source accepted")
	}
	dir := t.TempDir()
	path := filepath.Join(dir, "approval.json")
	for _, raw := range []string{`{"schema_version":1,"approved_changes":[]}`, `{"schema_version":1,"Schema_Version":2}`, `null`} {
		os.WriteFile(path, []byte(raw), 0600)
		if _, err := LoadEnvelope(path); err == nil {
			t.Fatalf("ambiguous/unsigned envelope accepted: %s", raw)
		}
	}
	raw, _ := json.Marshal(e)
	os.WriteFile(path, raw, 0600)
	if loaded, err := LoadEnvelope(path); err != nil || loaded.Statement.Nonce != e.Statement.Nonce {
		t.Fatal("valid envelope failed round trip", err)
	}
}

func TestPublicSyntheticSigningVector(t *testing.T) {
	dir := filepath.Join("..", "..", "testdata")
	envelope, err := LoadEnvelope(filepath.Join(dir, "signed-approval.json"))
	if err != nil {
		t.Fatal(err)
	}
	trust, err := LoadTrust(filepath.Join(dir, "approval-trust.json"))
	if err != nil {
		t.Fatal(err)
	}
	observation, err := LoadObservation(filepath.Join(dir, "approval-observation.json"))
	if err != nil {
		t.Fatal(err)
	}
	data, err := os.ReadFile(filepath.Join(dir, "approval-signing-vector.json"))
	if err != nil {
		t.Fatal(err)
	}
	var vector struct {
		Bytes     string `json:"signing_bytes_base64"`
		Public    string `json:"public_key_base64"`
		Signature string `json:"signature_base64"`
	}
	if err := json.Unmarshal(data, &vector); err != nil {
		t.Fatal(err)
	}
	payload, err := SigningBytes(envelope.Statement)
	if err != nil || base64.StdEncoding.EncodeToString(payload) != vector.Bytes {
		t.Fatal("canonical signing bytes changed", err)
	}
	public, err := base64.StdEncoding.DecodeString(vector.Public)
	if err != nil {
		t.Fatal(err)
	}
	signature, _ := base64.StdEncoding.DecodeString(vector.Signature)
	if !ed25519.Verify(public, payload, signature) {
		t.Fatal("published vector does not verify")
	}
	if r := verify(t, envelope, trust, observation); r.VerificationStatus != "VERIFIED" || r.FinalVerdict != "UNDETERMINED" {
		t.Fatalf("%+v", r)
	}
}
