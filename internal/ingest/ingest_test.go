package ingest

import (
	"context"
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/soroban-fleetwatch/fleetwatch/internal/history"
	"github.com/soroban-fleetwatch/fleetwatch/internal/jsoninput"
	"github.com/soroban-fleetwatch/fleetwatch/internal/source"
	"github.com/soroban-fleetwatch/fleetwatch/internal/store"
	protocol "github.com/stellar/go-stellar-sdk/protocols/rpc"
)

type fakeRPC struct {
	network source.Network
	infos   map[uint32]protocol.LedgerInfo
	calls   []uint32
	fail    uint32
	hook    func()
	netErr  bool
}

func (r *fakeRPC) Network(context.Context) (source.Network, error) {
	if r.netErr {
		return source.Network{}, errors.New("sensitive upstream URL/token")
	}
	return r.network, nil
}
func (r *fakeRPC) Ledgers(ctx context.Context, req protocol.GetLedgersRequest) (protocol.GetLedgersResponse, error) {
	r.calls = append(r.calls, req.StartLedger)
	if r.hook != nil {
		hook := r.hook
		r.hook = nil
		hook()
	}
	if r.fail == req.StartLedger {
		return protocol.GetLedgersResponse{}, errors.New("sensitive upstream URL/token")
	}
	out := protocol.GetLedgersResponse{OldestLedger: 100, LatestLedger: 200}
	if info, ok := r.infos[req.StartLedger]; ok {
		out.Ledgers = []protocol.LedgerInfo{info}
	}
	return out, nil
}
func fixtures(t *testing.T) (history.Baseline, *fakeRPC) {
	t.Helper()
	var b history.Baseline
	if err := jsoninput.File("../../testdata/reconstruction-baseline.json", &b); err != nil {
		t.Fatal(err)
	}
	r := &fakeRPC{network: source.Network{Passphrase: b.Snapshot.Network, ProtocolVersion: 29}, infos: map[uint32]protocol.LedgerInfo{}}
	for _, name := range []string{"reconstruction-capture.json", "reconstruction-next-capture.json"} {
		var c history.Capture
		if err := jsoninput.File("../../testdata/"+name, &c); err != nil {
			t.Fatal(err)
		}
		for _, e := range c.Ledgers {
			r.infos[e.Sequence] = protocol.LedgerInfo{Sequence: e.Sequence, Hash: e.Hash, LedgerCloseTime: e.CloseTime, LedgerHeader: e.HeaderXDR, LedgerMetadata: e.MetadataXDR}
		}
	}
	return b, r
}
func open(t *testing.T, path string) *store.Store {
	t.Helper()
	s, err := store.Open(path)
	if err != nil {
		t.Fatal(err)
	}
	return s
}
func TestIngestColdResumeNoopAndDedup(t *testing.T) {
	b, r := fixtures(t)
	ctx := context.Background()
	path := filepath.Join(t.TempDir(), "state.db")
	s := open(t, path)
	first, err := Run(ctx, r, s, "stream", b, 100)
	if err != nil || first.Status != "COMPLETE" || first.FetchFrom != 100 || first.FetchedLedgers != 1 || first.ProcessedThrough != 100 || first.Assurance.FinalVerdict != "UNDETERMINED" {
		t.Fatal(first, err)
	}
	s.Close()
	s = open(t, path)
	defer s.Close()
	second, err := Run(ctx, r, s, "stream", b, 101)
	if err != nil || second.StartedThrough != 100 || second.FetchFrom != 101 || second.ProcessedThrough != 101 || len(second.Report.Ledgers) != 2 {
		t.Fatal(second, err)
	}
	before, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	noop, err := Run(ctx, nil, s, "stream", b, 100)
	if err != nil || noop.Status != "ALREADY_RETAINED" || noop.SourceQuery != "NOT_QUERIED" || noop.ProcessedThrough != 101 || noop.Report != nil || noop.LatestAttemptStatus != "COMPLETE" {
		t.Fatal(noop, err)
	}
	after, err := os.ReadFile(path)
	if err != nil || string(before) != string(after) {
		t.Fatal("noop changed DB", err)
	}
	if len(r.calls) != 2 || r.calls[0] != 100 || r.calls[1] != 101 {
		t.Fatal(r.calls)
	}
	alerts, err := s.ReconstructionAlerts(ctx, b.Snapshot.Network, "stream")
	if err != nil || len(alerts) != 3 {
		t.Fatal(len(alerts), err)
	}
}
func TestIngestPartialPrefixFailureAndRepair(t *testing.T) {
	b, r := fixtures(t)
	s := open(t, filepath.Join(t.TempDir(), "state.db"))
	defer s.Close()
	ctx := context.Background()
	r.fail = 101
	partial, err := Run(ctx, r, s, "stream", b, 101)
	if !errors.Is(err, store.ErrReconstructionIncomplete) || partial.Status != "PARTIAL" || partial.FetchedLedgers != 1 || partial.ProcessedThrough != 100 || partial.LatestAttemptStatus != "PARTIAL" || len(partial.FetchErrors) != 1 || partial.FetchErrors[0] != "LEDGER_QUERY_FAILED" {
		t.Fatal(partial, err)
	}
	raw, _ := json.Marshal(partial)
	if strings.Contains(string(raw), "sensitive") {
		t.Fatal("upstream error leaked")
	}
	noop, err := Run(ctx, nil, s, "stream", b, 100)
	if err != nil || noop.Status != "ALREADY_RETAINED" || noop.LatestAttemptStatus != "PARTIAL" {
		t.Fatal(noop, err)
	}
	r.fail = 0
	complete, err := Run(ctx, r, s, "stream", b, 101)
	if err != nil || complete.FetchFrom != 101 || complete.Status != "COMPLETE" || complete.ProcessedThrough != 101 {
		t.Fatal(complete, err)
	}
	alerts, err := s.ReconstructionAlerts(ctx, b.Snapshot.Network, "stream")
	if err != nil || len(alerts) != 4 {
		t.Fatal(len(alerts), err)
	}
}
func TestIngestNetworkFailureRecordsDiagnostic(t *testing.T) {
	b, r := fixtures(t)
	s := open(t, filepath.Join(t.TempDir(), "state.db"))
	defer s.Close()
	r.netErr = true
	result, err := Run(context.Background(), r, s, "stream", b, 100)
	if !errors.Is(err, store.ErrReconstructionIncomplete) || result.ProcessedThrough != 99 || result.FetchedLedgers != 0 || result.FetchErrors[0] != "NETWORK_QUERY_FAILED" {
		t.Fatal(result, err)
	}
	if len(r.calls) != 0 {
		t.Fatal(r.calls)
	}
	a, err := s.LastReconstructionAttempt(context.Background(), b.Snapshot.Network, "stream")
	if err != nil || a.Status != "PARTIAL" {
		t.Fatal(a, err)
	}
}
func TestIngestStaleFetchCannotOverwriteAdvancedCursor(t *testing.T) {
	b, r := fixtures(t)
	s := open(t, filepath.Join(t.TempDir(), "state.db"))
	defer s.Close()
	ctx := context.Background()
	var c history.Capture
	if err := jsoninput.File("../../testdata/reconstruction-capture.json", &c); err != nil {
		t.Fatal(err)
	}
	r.hook = func() {
		if _, err := s.Reconstruct(ctx, "stream", b, c); err != nil {
			t.Fatal(err)
		}
	}
	result, err := Run(ctx, r, s, "stream", b, 101)
	if !errors.Is(err, store.ErrReconstructionCursor) || result.SchemaVersion != 0 {
		t.Fatal(result, err)
	}
	st, found, err := s.LoadReconstruction(ctx, b.Snapshot.Network, "stream")
	if err != nil || !found || st.Report.ProcessedThrough != 100 {
		t.Fatal(found, err)
	}
	a, err := s.LastReconstructionAttempt(ctx, b.Snapshot.Network, "stream")
	if err != nil || a.ToLedger != 100 || a.Status != "COMPLETE" {
		t.Fatal(a, err)
	}
}
func TestIngestCancellationAndInvalidRangeDoNotCommit(t *testing.T) {
	b, r := fixtures(t)
	s := open(t, filepath.Join(t.TempDir(), "state.db"))
	defer s.Close()
	ctx, cancel := context.WithCancel(context.Background())
	r.hook = cancel
	result, err := Run(ctx, r, s, "stream", b, 101)
	if !errors.Is(err, context.Canceled) || result.SchemaVersion != 0 {
		t.Fatal(result, err)
	}
	if _, found, err := s.LoadReconstruction(context.Background(), b.Snapshot.Network, "stream"); err != nil || found {
		t.Fatal(found, err)
	}
	if a, err := s.LastReconstructionAttempt(context.Background(), b.Snapshot.Network, "stream"); err != nil || a != nil {
		t.Fatal(a, err)
	}
	calls := len(r.calls)
	if _, err := Run(context.Background(), r, s, "stream", b, 116); !errors.Is(err, store.ErrReconstructionBound) || len(r.calls) != calls {
		t.Fatal(err)
	}
}

func TestIngestFetchedEvidenceStillRequiresBaselineLink(t *testing.T) {
	b, r := fixtures(t)
	b.HeaderHash = strings.Repeat("0", 63) + "1"
	s := open(t, filepath.Join(t.TempDir(), "state.db"))
	defer s.Close()
	result, err := Run(context.Background(), r, s, "stream", b, 100)
	if !errors.Is(err, store.ErrReconstructionIncomplete) || result.SourceQuery != "COMPLETE_ENDPOINT_SEQUENCE" || result.Status != "PARTIAL" || result.ProcessedThrough != 99 || result.Report.Errors[0] != "BASELINE_HEADER_LINK_MISMATCH" {
		t.Fatal(result, err)
	}
	if _, found, err := s.LoadReconstruction(context.Background(), b.Snapshot.Network, "stream"); err != nil || found {
		t.Fatal("bad baseline link advanced checkpoint", found, err)
	}
}
