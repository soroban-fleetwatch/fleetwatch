package store

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"os"
	"os/exec"
	"path/filepath"
	"reflect"
	"strings"
	"sync"
	"testing"

	"github.com/soroban-fleetwatch/fleetwatch/internal/history"
	"github.com/soroban-fleetwatch/fleetwatch/internal/jsoninput"
	"github.com/soroban-fleetwatch/fleetwatch/internal/source"
	protocol "github.com/stellar/go-stellar-sdk/protocols/rpc"
	"github.com/stellar/go-stellar-sdk/xdr"
	bolt "go.etcd.io/bbolt"
)

func reconstructionFixture(t testing.TB) (history.Baseline, history.Capture) {
	t.Helper()
	var b history.Baseline
	var c history.Capture
	if err := jsoninput.File("../../testdata/reconstruction-baseline.json", &b); err != nil {
		t.Fatal(err)
	}
	if err := jsoninput.File("../../testdata/reconstruction-capture.json", &c); err != nil {
		t.Fatal(err)
	}
	return b, c
}
func replaceReconstructionMeta(t testing.TB, e history.Evidence, meta xdr.LedgerCloseMeta) history.Evidence {
	t.Helper()
	encoded, err := xdr.MarshalBase64(meta)
	if err != nil {
		t.Fatal(err)
	}
	out, err := history.Decode(protocol.LedgerInfo{Sequence: e.Sequence, Hash: e.Hash, LedgerCloseTime: e.CloseTime, LedgerHeader: e.HeaderXDR, LedgerMetadata: encoded})
	if err != nil {
		t.Fatal(err)
	}
	return out
}
func reconstructionInstance(t testing.TB, b history.Baseline, next bool, cache bool) xdr.LedgerEntry {
	t.Helper()
	contract := b.Snapshot.Contracts[0]
	key, err := source.InstanceKey(contract.ID)
	if err != nil {
		t.Fatal(err)
	}
	refKey, err := source.ReferenceKey(contract.Executable.Owner, contract.Executable.TagHex)
	if err != nil {
		t.Fatal(err)
	}
	tag, _ := hex.DecodeString(contract.Executable.TagHex)
	ex := xdr.ContractExecutable{Type: xdr.ContractExecutableTypeContractExecutableExternalRef, ExternalRef: &xdr.ContractExecutableExternalRef{ExecutableOwner: refKey.ContractData.Contract, Tag: xdr.ScString(string(tag))}}
	if next {
		if cache {
			ex.ExternalRef.Tag = xdr.ScString(string([]byte{0xff, 0, 0x61, 0}))
		} else {
			h := xdr.Hash{9}
			ex = xdr.ContractExecutable{Type: xdr.ContractExecutableTypeContractExecutableWasm, WasmHash: &h}
		}
	}
	return xdr.LedgerEntry{LastModifiedLedgerSeq: 100, Data: xdr.LedgerEntryData{Type: xdr.LedgerEntryTypeContractData, ContractData: &xdr.ContractDataEntry{Contract: key.ContractData.Contract, Key: key.ContractData.Key, Durability: key.ContractData.Durability, Val: xdr.ScVal{Type: xdr.ScValTypeScvContractInstance, Instance: &xdr.ScContractInstance{Executable: ex}}}}}
}
func nextReconstructionCapture(t testing.TB, b history.Baseline, first history.Capture, cache bool) history.Capture {
	t.Helper()
	raw, _ := hex.DecodeString(first.Ledgers[0].Hash)
	var previous xdr.Hash
	copy(previous[:], raw)
	header := xdr.LedgerHeader{LedgerVersion: 29, LedgerSeq: 101, PreviousLedgerHash: previous, ScpValue: xdr.StellarValue{CloseTime: 1700000105}}
	headerRaw, err := header.MarshalBinary()
	if err != nil {
		t.Fatal(err)
	}
	hash := sha256.Sum256(headerRaw)
	entry := xdr.LedgerHeaderHistoryEntry{Hash: xdr.Hash(hash), Header: header}
	before, after := reconstructionInstance(t, b, false, cache), reconstructionInstance(t, b, true, cache)
	changes := xdr.LedgerEntryChanges{{Type: xdr.LedgerEntryChangeTypeLedgerEntryState, State: &before}, {Type: xdr.LedgerEntryChangeTypeLedgerEntryUpdated, Updated: &after}}
	result := xdr.TransactionResultPair{TransactionHash: xdr.Hash{2}, Result: xdr.TransactionResult{Result: xdr.TransactionResultResult{Code: xdr.TransactionResultCodeTxSuccess, Results: &[]xdr.OperationResult{}}}}
	meta := xdr.LedgerCloseMeta{V: 2, V2: &xdr.LedgerCloseMetaV2{LedgerHeader: entry, TxSet: xdr.GeneralizedTransactionSet{V: 1, V1TxSet: &xdr.TransactionSetV1{PreviousLedgerHash: previous}}, TxProcessing: []xdr.TransactionResultMetaV1{{Result: result, TxApplyProcessing: xdr.TransactionMeta{V: 4, V4: &xdr.TransactionMetaV4{Operations: []xdr.OperationMetaV2{{Changes: changes}}}}}}}}
	headerEncoded, err := xdr.MarshalBase64(entry)
	if err != nil {
		t.Fatal(err)
	}
	metaEncoded, err := xdr.MarshalBase64(meta)
	if err != nil {
		t.Fatal(err)
	}
	e, err := history.Decode(protocol.LedgerInfo{Sequence: 101, Hash: hex.EncodeToString(hash[:]), LedgerCloseTime: 1700000105, LedgerHeader: headerEncoded, LedgerMetadata: metaEncoded})
	if err != nil {
		t.Fatal(err)
	}
	next := first
	next.FromLedger = 101
	next.ToLedger = 101
	next.ProcessedThrough = 101
	next.Ledgers = []history.Evidence{e}
	return next
}

func TestReconstructionColdResumePreservesEvidenceAndFindings(t *testing.T) {
	path := filepath.Join(t.TempDir(), "state.db")
	s := openTest(t, path)
	b, c := reconstructionFixture(t)
	ctx := context.Background()
	first, err := s.Reconstruct(ctx, "operator-stream", b, c)
	if err != nil || first.ProcessedThrough != 100 {
		t.Fatal(first, err)
	}
	s.Close()
	s = openTest(t, path)
	defer s.Close()
	next := nextReconstructionCapture(t, b, c, false)
	r, err := s.Reconstruct(ctx, "operator-stream", b, next)
	if err != nil || r.ProcessedThrough != 101 || len(r.Ledgers) != 2 || len(r.Ledgers[1].Events) != 1 {
		t.Fatal(r, err)
	}
	if !reflect.DeepEqual(first.Ledgers[0], r.Ledgers[0]) {
		t.Fatal("existing event identities changed")
	}
	again, err := s.Reconstruct(ctx, "operator-stream", b, next)
	if err != nil || !reflect.DeepEqual(r, again) {
		t.Fatal("resume not idempotent", err)
	}
	state, found, err := s.LoadReconstruction(ctx, b.Snapshot.Network, "operator-stream")
	if err != nil || !found || len(state.Capture.Ledgers) != 2 || !reflect.DeepEqual(state.Report, r) || state.Baseline.Snapshot.Ledger != 99 {
		t.Fatal(state, found, err)
	}
	if _, found, err := s.Load(b.Snapshot.Network, "synthetic"); err != nil || found {
		t.Fatal("new checkpoint leaked into snapshot replay scope")
	}
	alerts, err := s.Alerts(b.Snapshot.Network, "synthetic")
	if err != nil || len(alerts) != 0 {
		t.Fatal("checkpoint silently queued legacy alerts")
	}
}

func TestReconstructionReplaysUnwatchedCacheAfterBackupRestore(t *testing.T) {
	dir := t.TempDir()
	s := openTest(t, filepath.Join(dir, "source.db"))
	b, c := reconstructionFixture(t)
	ctx := context.Background()
	var meta xdr.LedgerCloseMeta
	if err := xdr.SafeUnmarshalBase64(c.Ledgers[0].MetadataXDR, &meta); err != nil {
		t.Fatal(err)
	}
	key, err := source.ReferenceKey(b.Snapshot.References[0].Owner, "ff006100")
	if err != nil {
		t.Fatal(err)
	}
	value := xdr.ScBytes(make([]byte, 32))
	for i := range value {
		value[i] = 0xcc
	}
	entry := xdr.LedgerEntry{LastModifiedLedgerSeq: 90, Data: xdr.LedgerEntryData{Type: xdr.LedgerEntryTypeContractData, ContractData: &xdr.ContractDataEntry{Contract: key.ContractData.Contract, Key: key.ContractData.Key, Durability: key.ContractData.Durability, Val: xdr.ScVal{Type: xdr.ScValTypeScvBytes, Bytes: &value}}}}
	meta.V2.TxProcessing[0].TxApplyProcessing.V4.Operations[1].Changes = append(meta.V2.TxProcessing[0].TxApplyProcessing.V4.Operations[1].Changes, xdr.LedgerEntryChange{Type: xdr.LedgerEntryChangeTypeLedgerEntryState, State: &entry})
	c.Ledgers[0] = replaceReconstructionMeta(t, c.Ledgers[0], meta)
	r, err := s.Reconstruct(ctx, "cache-stream", b, c)
	if err != nil || len(r.Snapshot.References) != 1 {
		t.Fatal(r, err)
	}
	backup := filepath.Join(dir, "backup.db")
	if _, err := s.Backup(ctx, backup); err != nil {
		t.Fatal(err)
	}
	s.Close()
	restored := openTest(t, backup)
	defer restored.Close()
	r, err = restored.Reconstruct(ctx, "cache-stream", b, nextReconstructionCapture(t, b, c, true))
	if err != nil || r.ProcessedThrough != 101 || len(r.Snapshot.References) != 2 {
		t.Fatal("lost non-active cached reference across restart", r, err)
	}
	found := false
	for _, ref := range r.Snapshot.References {
		if ref.TagHex == "ff006100" && ref.WasmHash == strings.Repeat("cc", 32) {
			found = true
		}
	}
	if !found {
		t.Fatal("wrong restored cache value")
	}
}

func TestReconstructionPrefixGapAndCachedPartialAttempt(t *testing.T) {
	s := openTest(t, filepath.Join(t.TempDir(), "state.db"))
	defer s.Close()
	b, c := reconstructionFixture(t)
	ctx := context.Background()
	next := nextReconstructionCapture(t, b, c, false)
	r, err := s.Reconstruct(ctx, "gap", b, next)
	if !errors.Is(err, ErrReconstructionIncomplete) || r.ProcessedThrough != 99 {
		t.Fatal(r, err)
	}
	if _, found, err := s.LoadReconstruction(ctx, b.Snapshot.Network, "gap"); err != nil || found {
		t.Fatal("unprocessed suffix was retained", err)
	}
	if _, err := s.Reconstruct(ctx, "gap", b, c); err != nil {
		t.Fatal(err)
	}
	partial := c
	partial.ToLedger = 101
	partial.SequenceCoverage = "PARTIAL"
	partial.Errors = []string{"LEDGER_QUERY_FAILED"}
	r, err = s.Reconstruct(ctx, "gap", b, partial)
	if !errors.Is(err, ErrReconstructionIncomplete) || r.ProcessedThrough != 100 {
		t.Fatal(r, err)
	}
	stored, found, err := s.LoadReconstruction(ctx, b.Snapshot.Network, "gap")
	if err != nil || !found || stored.Report.Status != "COMPLETE_CAPTURE_APPLICATION" || stored.Capture.ToLedger != 100 {
		t.Fatal("failed attempt misrepresented as retained evidence", stored, err)
	}
	if _, err := s.Reconstruct(ctx, "gap", b, next); err != nil {
		t.Fatal(err)
	}
	partial = c
	partial.Ledgers = nil
	partial.ProcessedThrough = 0
	partial.SequenceCoverage = "PARTIAL"
	partial.Errors = []string{"LEDGER_QUERY_FAILED"}
	r, err = s.Reconstruct(ctx, "gap", b, partial)
	if !errors.Is(err, ErrReconstructionIncomplete) || r.Errors[0] != "SOURCE_ATTEMPT_INCOMPLETE" || r.ProcessedThrough != 101 {
		t.Fatal("masked current source failure with cached data", r, err)
	}
}

func TestReconstructionRejectsConflictsCorruptionAndBounds(t *testing.T) {
	s := openTest(t, filepath.Join(t.TempDir(), "state.db"))
	defer s.Close()
	b, c := reconstructionFixture(t)
	ctx := context.Background()
	if _, err := s.Reconstruct(ctx, "fixed", b, c); err != nil {
		t.Fatal(err)
	}
	bad := c
	bad.Ledgers = append([]history.Evidence{}, c.Ledgers...)
	bad.Ledgers[0].MetadataSHA256 = strings.Repeat("0", 64)
	if _, err := s.Reconstruct(ctx, "fixed", b, bad); !errors.Is(err, ErrConflict) {
		t.Fatal("conflicting overlap accepted", err)
	}
	changed := b
	changed.HeaderHash = strings.Repeat("1", 64)
	if _, err := s.Reconstruct(ctx, "fixed", changed, c); !errors.Is(err, ErrReconstructionBaseline) {
		t.Fatal(err)
	}
	far := c
	far.FromLedger = 116
	far.ToLedger = 116
	far.ProcessedThrough = 0
	far.Ledgers = nil
	far.SequenceCoverage = "PARTIAL"
	far.Errors = []string{"LEDGER_QUERY_FAILED"}
	if _, err := s.Reconstruct(ctx, "fixed", b, far); !errors.Is(err, ErrReconstructionBound) {
		t.Fatal("cumulative range unbounded", err)
	}
	for _, stream := range []string{"", "bad/stream", "bad stream", strings.Repeat("s", 129)} {
		if ValidateReconstructionScope(b.Snapshot.Network, stream) == nil {
			t.Fatal("invalid stream accepted", stream)
		}
	}
	if err := s.db.Update(func(tx *bolt.Tx) error {
		bucket := tx.Bucket(reconstructionBucket)
		key := reconstructionScope(b.Snapshot.Network, "fixed")
		var state ReconstructionState
		if err := json.Unmarshal(bucket.Get(key), &state); err != nil {
			return err
		}
		state.Report.FinalVerdict = "APPROVED"
		raw, _ := json.Marshal(state)
		return bucket.Put(key, raw)
	}); err != nil {
		t.Fatal(err)
	}
	if state, found, err := s.LoadReconstruction(ctx, b.Snapshot.Network, "fixed"); err == nil || found || state.SchemaVersion != 0 {
		t.Fatal("corrupt verdict trusted")
	}
	if _, err := s.Reconstruct(ctx, "fixed", b, c); err == nil {
		t.Fatal("corrupt prior state ignored")
	}
}

func TestReconstructionAbortCancellationAndScopeIsolation(t *testing.T) {
	s := openTest(t, filepath.Join(t.TempDir(), "state.db"))
	defer s.Close()
	b, c := reconstructionFixture(t)
	ctx := context.Background()
	injected := errors.New("abort")
	if r, err := s.reconstruct(ctx, "abort", b, c, func() error { return injected }); !errors.Is(err, injected) || r.SchemaVersion != 0 {
		t.Fatal(r, err)
	}
	if _, found, err := s.LoadReconstruction(ctx, b.Snapshot.Network, "abort"); err != nil || found {
		t.Fatal("aborted state survived")
	}
	cancelled, cancel := context.WithCancel(ctx)
	if _, err := s.reconstruct(cancelled, "cancel", b, c, func() error { cancel(); return nil }); !errors.Is(err, context.Canceled) {
		t.Fatal(err)
	}
	if _, found, err := s.LoadReconstruction(ctx, b.Snapshot.Network, "cancel"); err != nil || found {
		t.Fatal("cancelled write survived")
	}
	if _, err := s.Reconstruct(ctx, "one", b, c); err != nil {
		t.Fatal(err)
	}
	if _, found, err := s.LoadReconstruction(ctx, b.Snapshot.Network, "two"); err != nil || found {
		t.Fatal("stream isolation failed")
	}
	if _, found, err := s.LoadReconstruction(ctx, "OTHER-NETWORK", "one"); err != nil || found {
		t.Fatal("network isolation failed")
	}
	otherBase, otherCapture := reconstructionFixture(t)
	otherBase.Snapshot.Network = "OTHER-NETWORK"
	otherCapture.Network = "OTHER-NETWORK"
	if r, err := s.Reconstruct(ctx, "one", otherBase, otherCapture); err != nil || r.Network != "OTHER-NETWORK" {
		t.Fatal("separate network scope failed", r, err)
	}
	var wg sync.WaitGroup
	errs := make(chan error, 2)
	next := nextReconstructionCapture(t, b, c, false)
	for i := 0; i < 2; i++ {
		wg.Add(1)
		go func() { defer wg.Done(); _, err := s.Reconstruct(ctx, "one", b, next); errs <- err }()
	}
	wg.Wait()
	close(errs)
	for err := range errs {
		if err != nil {
			t.Fatal(err)
		}
	}
	state, found, err := s.LoadReconstruction(ctx, b.Snapshot.Network, "one")
	if err != nil || !found || len(state.Report.Ledgers) != 2 || len(state.Report.Ledgers[1].Events) != 1 {
		t.Fatal(state, err)
	}
}

func TestReconstructionPartialApplicationCommitsOnlyAcceptedPrefix(t *testing.T) {
	s := openTest(t, filepath.Join(t.TempDir(), "state.db"))
	defer s.Close()
	b, c := reconstructionFixture(t)
	ctx := context.Background()
	validNext := nextReconstructionCapture(t, b, c, false)
	var meta xdr.LedgerCloseMeta
	if err := xdr.SafeUnmarshalBase64(validNext.Ledgers[0].MetadataXDR, &meta); err != nil {
		t.Fatal(err)
	}
	// The new ledger's STATE asserts the post-update executable instead of the
	// retained one; its wire evidence is structurally valid but application fails.
	meta.V2.TxProcessing[0].TxApplyProcessing.V4.Operations[0].Changes[0].State = meta.V2.TxProcessing[0].TxApplyProcessing.V4.Operations[0].Changes[1].Updated
	badNext := replaceReconstructionMeta(t, validNext.Ledgers[0], meta)
	combined := c
	combined.ToLedger = 101
	combined.ProcessedThrough = 101
	combined.Ledgers = append(append([]history.Evidence{}, c.Ledgers...), badNext)
	r, err := s.Reconstruct(ctx, "partial-apply", b, combined)
	if !errors.Is(err, ErrReconstructionIncomplete) || r.ProcessedThrough != 100 || r.Errors[0] != "PREIMAGE_MISMATCH" {
		t.Fatal(r, err)
	}
	stored, found, err := s.LoadReconstruction(ctx, b.Snapshot.Network, "partial-apply")
	if err != nil || !found || len(stored.Capture.Ledgers) != 1 || stored.Capture.ToLedger != 100 || stored.Report.Status != "COMPLETE_CAPTURE_APPLICATION" {
		t.Fatal(stored, err)
	}
	if r, err := s.Reconstruct(ctx, "partial-apply", b, validNext); err != nil || r.ProcessedThrough != 101 {
		t.Fatal("failed ledger poisoned recovery", r, err)
	}
}

func TestReconstructionCrashBeforeCommit(t *testing.T) {
	if path := os.Getenv("FLEETWATCH_TEST_RECONSTRUCTION_CRASH_DB"); path != "" {
		s := openTest(t, path)
		b, c := reconstructionFixture(t)
		_, _ = s.reconstruct(context.Background(), "crash", b, nextReconstructionCapture(t, b, c, false), func() error { os.Exit(23); return nil })
		t.Fatal("crash not reached")
	}
	path := filepath.Join(t.TempDir(), "state.db")
	s := openTest(t, path)
	b, c := reconstructionFixture(t)
	if _, err := s.Reconstruct(context.Background(), "crash", b, c); err != nil {
		t.Fatal(err)
	}
	s.Close()
	child := exec.Command(os.Args[0], "-test.run=^TestReconstructionCrashBeforeCommit$")
	child.Env = append(os.Environ(), "FLEETWATCH_TEST_RECONSTRUCTION_CRASH_DB="+path)
	err := child.Run()
	var status *exec.ExitError
	if !errors.As(err, &status) || status.ExitCode() != 23 {
		t.Fatal(err)
	}
	s = openTest(t, path)
	defer s.Close()
	state, found, err := s.LoadReconstruction(context.Background(), b.Snapshot.Network, "crash")
	if err != nil || !found || state.Report.ProcessedThrough != 100 || len(state.Capture.Ledgers) != 1 {
		t.Fatal("partial crash commit", state, err)
	}
	if r, err := s.Reconstruct(context.Background(), "crash", b, nextReconstructionCapture(t, b, c, false)); err != nil || r.ProcessedThrough != 101 {
		t.Fatal(r, err)
	}
}
