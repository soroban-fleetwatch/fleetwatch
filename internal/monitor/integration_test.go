package monitor

import (
	"context"
	"errors"
	"path/filepath"
	"testing"
	"time"

	"github.com/soroban-fleetwatch/fleetwatch/internal/history"
	"github.com/soroban-fleetwatch/fleetwatch/internal/ingest"
	"github.com/soroban-fleetwatch/fleetwatch/internal/jsoninput"
	"github.com/soroban-fleetwatch/fleetwatch/internal/source"
	"github.com/soroban-fleetwatch/fleetwatch/internal/store"
	protocol "github.com/stellar/go-stellar-sdk/protocols/rpc"
)

type fixtureRPC struct {
	infos    map[uint32]protocol.LedgerInfo
	networks int
	requests []uint32
	repair   bool
}

func (r *fixtureRPC) Network(context.Context) (source.Network, error) {
	r.networks++
	return source.Network{Passphrase: "SYNTHETIC-NETWORK", ProtocolVersion: 29}, nil
}
func (r *fixtureRPC) Ledgers(_ context.Context, req protocol.GetLedgersRequest) (protocol.GetLedgersResponse, error) {
	r.requests = append(r.requests, req.StartLedger)
	out := protocol.GetLedgersResponse{OldestLedger: 100, LatestLedger: 101}
	if req.StartLedger == 101 && (!r.repair || r.networks == 1) {
		return out, nil
	}
	out.Ledgers = []protocol.LedgerInfo{r.infos[req.StartLedger]}
	return out, nil
}
func fixtureData(t *testing.T) (history.Baseline, *fixtureRPC) {
	t.Helper()
	var b history.Baseline
	if err := jsoninput.File("../../testdata/reconstruction-baseline.json", &b); err != nil {
		t.Fatal(err)
	}
	r := &fixtureRPC{infos: map[uint32]protocol.LedgerInfo{}}
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
func TestWatchRealIngestionRetryAndColdNoop(t *testing.T) {
	b, r := fixtureData(t)
	r.repair = true
	path := filepath.Join(t.TempDir(), "state.db")
	s, err := store.Open(path)
	if err != nil {
		t.Fatal(err)
	}
	ctx := context.Background()
	events := []Event{}
	last, err := Run(ctx, scope(), options(), func(ctx context.Context) (ingest.Result, error) { return ingest.Run(ctx, r, s, "watch", b, 101) }, func(e Event) error { events = append(events, e); return nil })
	if err != nil || last.Status != "COMPLETE" || last.Attempts != 2 || last.FetchedLedgers != 2 || last.AcceptedLedgers != 2 || *last.ObservedEvents != 3 || len(events) != 2 || *events[0].ProcessedThrough != 100 {
		t.Fatal(last, err)
	}
	if len(r.requests) != 3 || r.requests[0] != 100 || r.requests[1] != 101 || r.requests[2] != 101 {
		t.Fatal(r.requests)
	}
	alerts, err := s.ReconstructionAlerts(ctx, b.Snapshot.Network, "watch")
	if err != nil || len(alerts) != 4 {
		t.Fatal(len(alerts), err)
	}
	s.Close()
	s, err = store.Open(path)
	if err != nil {
		t.Fatal(err)
	}
	defer s.Close()
	last, err = Run(ctx, scope(), options(), func(ctx context.Context) (ingest.Result, error) { return ingest.Run(ctx, nil, s, "watch", b, 101) }, func(Event) error { return nil })
	if err != nil || last.Status != "ALREADY_RETAINED" || last.FetchedLedgers != 0 || last.ObservedEvents != nil {
		t.Fatal(last, err)
	}
}
func TestWatchCancelledWaitRetainsPrefixAndCanResume(t *testing.T) {
	b, r := fixtureData(t)
	path := filepath.Join(t.TempDir(), "state.db")
	s, err := store.Open(path)
	if err != nil {
		t.Fatal(err)
	}
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	last, err := Run(ctx, scope(), options(), func(ctx context.Context) (ingest.Result, error) { return ingest.Run(ctx, r, s, "watch", b, 101) }, func(e Event) error {
		if e.Status == "RETRYING" {
			cancel()
		}
		return nil
	})
	if !errors.Is(err, context.Canceled) || last.Code != "CANCELLED" || *last.ProcessedThrough != 100 {
		t.Fatal(last, err)
	}
	s.Close()
	s, err = store.Open(path)
	if err != nil {
		t.Fatal(err)
	}
	defer s.Close()
	h, err := s.ReconstructionHealth(context.Background(), b.Snapshot.Network, "watch", time.Now())
	if err != nil || h.ProcessedThrough != 100 || h.LastAttempt.Status != "PARTIAL" {
		t.Fatal(h, err)
	}
	r.repair = true
	r.networks = 1
	last, err = Run(context.Background(), scope(), options(), func(ctx context.Context) (ingest.Result, error) { return ingest.Run(ctx, r, s, "watch", b, 101) }, func(Event) error { return nil })
	if err != nil || last.Status != "COMPLETE" || last.Attempts != 1 || last.AcceptedLedgers != 1 {
		t.Fatal(last, err)
	}
}
