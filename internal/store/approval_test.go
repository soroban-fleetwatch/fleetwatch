package store

import (
	"crypto/ed25519"
	"crypto/rand"
	"encoding/base64"
	"encoding/json"
	"errors"
	bolt "go.etcd.io/bbolt"
	"os"
	"os/exec"
	"path/filepath"
	"sync"
	"testing"

	"github.com/soroban-fleetwatch/fleetwatch/internal/fleetwatch"
	"github.com/soroban-fleetwatch/fleetwatch/internal/policy"
)

func publicBindFixture(t *testing.T) (fleetwatch.Journal, policy.Envelope, policy.Trust) {
	t.Helper()
	dir := filepath.Join("..", "..", "testdata")
	e, err := policy.LoadEnvelope(filepath.Join(dir, "signed-approval.json"))
	if err != nil {
		t.Fatal(err)
	}
	trust, err := policy.LoadTrust(filepath.Join(dir, "approval-trust.json"))
	if err != nil {
		t.Fatal(err)
	}
	o, err := policy.LoadObservation(filepath.Join(dir, "approval-observation.json"))
	if err != nil {
		t.Fatal(err)
	}
	j := fleetwatch.Journal{SchemaVersion: 1, Source: "synthetic", Network: o.NetworkPassphrase, FromLedger: 100, ToLedger: 103}
	for i, h := range []string{o.FromHash, o.ToHash, o.FromHash, o.ToHash} {
		j.Snapshots = append(j.Snapshots, fleetwatch.Snapshot{SchemaVersion: 1, Network: j.Network, Ledger: uint32(100 + i), Contracts: []fleetwatch.Contract{{ID: "SYNTHETIC_INSTANCE", Executable: fleetwatch.Executable{Kind: "external_ref", Owner: o.Owner, TagHex: o.TagHex}}}, References: []fleetwatch.Reference{{Owner: o.Owner, TagHex: o.TagHex, WasmHash: h}}})
	}
	return j, e, trust
}
func generatedBindFixture(t *testing.T) (fleetwatch.Journal, policy.Envelope, policy.Trust, ed25519.PrivateKey) {
	t.Helper()
	j, e, trust := publicBindFixture(t)
	pub, key, err := ed25519.GenerateKey(rand.Reader)
	if err != nil {
		t.Fatal(err)
	}
	trust.Keys[0].ID = policy.KeyID(pub)
	trust.Keys[0].PublicKey = base64.StdEncoding.EncodeToString(pub)
	resign(t, &e, key)
	return j, e, trust, key
}
func resign(t *testing.T, e *policy.Envelope, key ed25519.PrivateKey) {
	t.Helper()
	raw, err := policy.SigningBytes(e.Statement)
	if err != nil {
		t.Fatal(err)
	}
	e.Signatures = []policy.Signature{{KeyID: policy.KeyID(key.Public().(ed25519.PublicKey)), Value: base64.StdEncoding.EncodeToString(ed25519.Sign(key, raw))}}
}
func replayForBinding(t *testing.T, s *Store, j fleetwatch.Journal) []fleetwatch.ReplayEvent {
	t.Helper()
	r, err := s.Replay(j)
	if err != nil {
		t.Fatal(err)
	}
	return r.Events
}
func TestApprovalBindingPersistsAndIsIdempotent(t *testing.T) {
	path := filepath.Join(t.TempDir(), "state.db")
	s := openTest(t, path)
	j, e, trust := publicBindFixture(t)
	events := replayForBinding(t, s, j)
	r, err := s.BindApproval(j.Network, j.Source, events[0].ID, e, trust)
	if err != nil || !r.NewBinding || r.Binding == nil || r.Verification.FinalVerdict != "UNDETERMINED" {
		t.Fatalf("%+v %v", r, err)
	}
	original := r.Binding.BoundAt
	s.Close()
	s = openTest(t, path)
	defer s.Close()
	r, err = s.BindApproval(j.Network, j.Source, events[0].ID, e, trust)
	if err != nil || r.NewBinding || r.Binding == nil || r.Binding.BoundAt != original {
		t.Fatalf("idempotence: %+v %v", r, err)
	}
	state, _, _ := s.Load(j.Network, j.Source)
	if state.Report.Events[0].Finding.Authorization != "UNVERIFIED" || state.Report.Events[0].Finding.Severity != "critical" {
		t.Fatal("binding downgraded unverified chain event")
	}
	rows, _ := s.Alerts(j.Network, j.Source)
	for _, a := range rows {
		if a.Payload.Event != nil && a.Payload.Event.Finding.Authorization != "UNVERIFIED" {
			t.Fatal("binding changed outbox authorization")
		}
	}
}
func TestApprovalNonceCannotBindAnotherIncident(t *testing.T) {
	s := openTest(t, filepath.Join(t.TempDir(), "state.db"))
	defer s.Close()
	j, e, trust, key := generatedBindFixture(t)
	events := replayForBinding(t, s, j)
	if _, err := s.BindApproval(j.Network, j.Source, events[0].ID, e, trust); err != nil {
		t.Fatal(err)
	}
	if _, err := s.BindApproval(j.Network, j.Source, events[2].ID, e, trust); !errors.Is(err, ErrApprovalNonceUsed) {
		t.Fatal("nonce reused for later A->B", err)
	}
	e.Statement.Nonce = "dddddddddddddddddddddddddddddddddddddddddddddddddddddddddddddddd"
	resign(t, &e, key)
	if _, err := s.BindApproval(j.Network, j.Source, events[0].ID, e, trust); !errors.Is(err, ErrApprovalEventBound) {
		t.Fatal("binding silently replaced", err)
	}
	if r, err := s.BindApproval(j.Network, j.Source, events[2].ID, e, trust); err != nil || !r.NewBinding {
		t.Fatalf("failed attempt consumed new nonce: %+v %v", r, err)
	}
}
func TestFailedOrPartialVerificationDoesNotConsumeNonce(t *testing.T) {
	s := openTest(t, filepath.Join(t.TempDir(), "state.db"))
	defer s.Close()
	j, e, trust := publicBindFixture(t)
	events := replayForBinding(t, s, j)
	forged := e
	forged.Statement.Ticket = "TAMPERED"
	r, err := s.BindApproval(j.Network, j.Source, events[0].ID, forged, trust)
	if err != nil || r.Binding != nil || r.NewBinding || r.Verification.VerificationStatus != "REJECTED" {
		t.Fatalf("%+v %v", r, err)
	}
	gap := j
	gap.ToLedger = 104
	replayForBinding(t, s, gap)
	r, err = s.BindApproval(j.Network, j.Source, events[0].ID, e, trust)
	if err != nil || r.Binding != nil || r.Verification.VerificationStatus != "UNDETERMINED" {
		t.Fatalf("partial: %+v %v", r, err)
	}
	replayForBinding(t, s, j)
	if r, err := s.BindApproval(j.Network, j.Source, events[0].ID, e, trust); err != nil || !r.NewBinding {
		t.Fatalf("nonce consumed by rejected attempt: %+v %v", r, err)
	}
}
func TestBindingRechecksCurrentRevocation(t *testing.T) {
	s := openTest(t, filepath.Join(t.TempDir(), "state.db"))
	defer s.Close()
	j, e, trust := publicBindFixture(t)
	events := replayForBinding(t, s, j)
	if _, err := s.BindApproval(j.Network, j.Source, events[0].ID, e, trust); err != nil {
		t.Fatal(err)
	}
	trust.Keys[0].RevokedAt = 101
	r, err := s.BindApproval(j.Network, j.Source, events[0].ID, e, trust)
	if err != nil || r.Binding != nil || r.NewBinding || r.Verification.SignerTrusted != "NO" {
		t.Fatalf("revoked binding reaccepted: %+v %v", r, err)
	}
}
func TestApprovalBindingRollsBackBothWrites(t *testing.T) {
	s := openTest(t, filepath.Join(t.TempDir(), "state.db"))
	defer s.Close()
	j, e, trust := publicBindFixture(t)
	events := replayForBinding(t, s, j)
	fail := errors.New("pre-commit fault")
	r, err := s.bindApproval(j.Network, j.Source, events[0].ID, e, trust, func() error { return fail })
	if !errors.Is(err, fail) || r.Binding != nil || r.NewBinding {
		t.Fatalf("%+v %v", r, err)
	}
	if r, err := s.BindApproval(j.Network, j.Source, events[2].ID, e, trust); err != nil || !r.NewBinding {
		t.Fatalf("nonce survived aborted transaction: %+v %v", r, err)
	}
}
func TestApprovalNonceIsSharedAcrossSourceScopes(t *testing.T) {
	s := openTest(t, filepath.Join(t.TempDir(), "state.db"))
	defer s.Close()
	j, e, trust := publicBindFixture(t)
	events := replayForBinding(t, s, j)
	if _, err := s.BindApproval(j.Network, j.Source, events[0].ID, e, trust); err != nil {
		t.Fatal(err)
	}
	j.Source = "user_snapshot"
	other := replayForBinding(t, s, j)
	if _, err := s.BindApproval(j.Network, j.Source, other[0].ID, e, trust); !errors.Is(err, ErrApprovalNonceUsed) {
		t.Fatal("nonce reused across source scopes", err)
	}
}
func TestConcurrentApprovalClaimsHaveOneWinner(t *testing.T) {
	s := openTest(t, filepath.Join(t.TempDir(), "state.db"))
	defer s.Close()
	j, e, trust := publicBindFixture(t)
	events := replayForBinding(t, s, j)
	var wg sync.WaitGroup
	results := make(chan error, 2)
	for _, id := range []string{events[0].ID, events[2].ID} {
		wg.Add(1)
		go func(id string) {
			defer wg.Done()
			_, err := s.BindApproval(j.Network, j.Source, id, e, trust)
			results <- err
		}(id)
	}
	wg.Wait()
	close(results)
	success, conflict := 0, 0
	for err := range results {
		if err == nil {
			success++
		} else if errors.Is(err, ErrApprovalNonceUsed) {
			conflict++
		} else {
			t.Fatal(err)
		}
	}
	if success != 1 || conflict != 1 {
		t.Fatalf("success=%d conflict=%d", success, conflict)
	}
}
func TestApprovalProcessCrashBeforeCommit(t *testing.T) {
	j, e, trust := publicBindFixture(t)
	if path := os.Getenv("FLEETWATCH_APPROVAL_CRASH_STATE"); path != "" {
		s := openTest(t, path)
		r, err := fleetwatch.ReplayJournal(j)
		if err != nil {
			t.Fatal(err)
		}
		_, _ = s.bindApproval(j.Network, j.Source, r.Events[0].ID, e, trust, func() error { os.Exit(23); return nil })
		t.Fatal("crash hook not reached")
	}
	path := filepath.Join(t.TempDir(), "state.db")
	s := openTest(t, path)
	events := replayForBinding(t, s, j)
	s.Close()
	child := exec.Command(os.Args[0], "-test.run=^TestApprovalProcessCrashBeforeCommit$")
	child.Env = append(os.Environ(), "FLEETWATCH_APPROVAL_CRASH_STATE="+path)
	err := child.Run()
	var exit *exec.ExitError
	if !errors.As(err, &exit) || exit.ExitCode() != 23 {
		t.Fatalf("crash result: %v", err)
	}
	s = openTest(t, path)
	defer s.Close()
	if r, err := s.BindApproval(j.Network, j.Source, events[2].ID, e, trust); err != nil || !r.NewBinding {
		t.Fatalf("nonce or binding survived crash: %+v %v", r, err)
	}
}

func TestBindingRejectsCorruptedRecordedVerdict(t *testing.T) {
	s := openTest(t, filepath.Join(t.TempDir(), "state.db"))
	defer s.Close()
	j, e, trust := publicBindFixture(t)
	events := replayForBinding(t, s, j)
	if _, err := s.BindApproval(j.Network, j.Source, events[0].ID, e, trust); err != nil {
		t.Fatal(err)
	}
	err := s.db.Update(func(tx *bolt.Tx) error {
		bucket := tx.Bucket(approvalBucket).Bucket(scope(j.Network, j.Source))
		raw := bucket.Get([]byte(events[0].ID))
		var binding ApprovalBinding
		if err := json.Unmarshal(raw, &binding); err != nil {
			return err
		}
		binding.Verification.FinalVerdict = "APPROVED"
		data, err := json.Marshal(binding)
		if err != nil {
			return err
		}
		return bucket.Put([]byte(events[0].ID), data)
	})
	if err != nil {
		t.Fatal(err)
	}
	r, err := s.BindApproval(j.Network, j.Source, events[0].ID, e, trust)
	if err == nil || r.Binding != nil || r.NewBinding {
		t.Fatal("corrupted stored verdict returned as valid")
	}
}

func TestApprovalTrustSnapshotIsOrderIndependentButImmutable(t *testing.T) {
	s := openTest(t, filepath.Join(t.TempDir(), "state.db"))
	defer s.Close()
	j, e, trust := publicBindFixture(t)
	events := replayForBinding(t, s, j)
	public, _, err := ed25519.GenerateKey(rand.Reader)
	if err != nil {
		t.Fatal(err)
	}
	trust.Keys = append(trust.Keys, policy.Key{ID: policy.KeyID(public), PublicKey: base64.StdEncoding.EncodeToString(public), ValidFrom: 1, ValidUntil: 1000})
	trust.AllowedTags = append(trust.AllowedTags, "00")
	if _, err := s.BindApproval(j.Network, j.Source, events[0].ID, e, trust); err != nil {
		t.Fatal(err)
	}
	trust.Keys[0], trust.Keys[1] = trust.Keys[1], trust.Keys[0]
	trust.AllowedTags[0], trust.AllowedTags[1] = trust.AllowedTags[1], trust.AllowedTags[0]
	r, err := s.BindApproval(j.Network, j.Source, events[0].ID, e, trust)
	if err != nil || r.NewBinding || r.Binding == nil {
		t.Fatalf("order changed trust identity: %+v %v", r, err)
	}
	trust.Keys[0].RevokedAt = 101
	if _, err := s.BindApproval(j.Network, j.Source, events[0].ID, e, trust); !errors.Is(err, ErrApprovalNonceUsed) {
		t.Fatal("historical trust snapshot silently replaced", err)
	}
}
func TestApprovalBindingRejectsUnknownAndUnsupportedEvents(t *testing.T) {
	s := openTest(t, filepath.Join(t.TempDir(), "state.db"))
	defer s.Close()
	j, e, trust := publicBindFixture(t)
	if _, err := s.BindApproval(j.Network, j.Source, "invalid", e, trust); !errors.Is(err, ErrApprovalEventUnavailable) {
		t.Fatal(err)
	}
	old := fixture(t)
	events := replayForBinding(t, s, old)
	if _, err := s.BindApproval(old.Network, old.Source, events[0].ID, e, trust); !errors.Is(err, ErrApprovalEventUnavailable) {
		t.Fatal("non-chain owner accepted by approval binding", err)
	}
}
