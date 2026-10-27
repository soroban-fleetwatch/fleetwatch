package follow

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/soroban-fleetwatch/fleetwatch/internal/history"
	"github.com/soroban-fleetwatch/fleetwatch/internal/jsoninput"
	"github.com/soroban-fleetwatch/fleetwatch/internal/source"
	protocol "github.com/stellar/go-stellar-sdk/protocols/rpc"
	"github.com/stellar/go-stellar-sdk/xdr"
	bolt "go.etcd.io/bbolt"
)

// Synthetic typed XDR only: two watched members, A->B at 100 and B->A at 117.
func chain(t testing.TB) (history.Baseline, []history.Evidence) {
	t.Helper()
	var b history.Baseline
	var first history.Capture
	if jsoninput.File("../../testdata/reconstruction-baseline.json", &b) != nil || jsoninput.File("../../testdata/reconstruction-capture.json", &first) != nil {
		t.Fatal("fixtures unavailable")
	}
	var template xdr.LedgerCloseMeta
	if err := xdr.SafeUnmarshalBase64(first.Ledgers[0].MetadataXDR, &template); err != nil {
		t.Fatal(err)
	}
	previousRaw, _ := hex.DecodeString(b.HeaderHash)
	var previous xdr.Hash
	copy(previous[:], previousRaw)
	all := []history.Evidence{}
	for seq := uint32(100); seq < 140; seq++ {
		header := xdr.LedgerHeader{LedgerVersion: 29, LedgerSeq: xdr.Uint32(seq), PreviousLedgerHash: previous, ScpValue: xdr.StellarValue{CloseTime: xdr.TimePoint(1700000000 + seq*5)}}
		raw, _ := header.MarshalBinary()
		hash := sha256.Sum256(raw)
		entry := xdr.LedgerHeaderHistoryEntry{Hash: xdr.Hash(hash), Header: header}
		meta := xdr.LedgerCloseMeta{V: 2, V2: &xdr.LedgerCloseMetaV2{LedgerHeader: entry, TxSet: xdr.GeneralizedTransactionSet{V: 1, V1TxSet: &xdr.TransactionSetV1{PreviousLedgerHash: previous}}, TxProcessing: []xdr.TransactionResultMetaV1{}}}
		if seq == 100 || seq == 117 {
			tx := template.V2.TxProcessing[0]
			apply := *tx.TxApplyProcessing.V4
			ops := apply.Operations
			if seq == 100 {
				apply.Operations = ops[:1]
			} else {
				apply.Operations = ops[1:]
				tx.Result.TransactionHash = xdr.Hash{2}
			}
			tx.TxApplyProcessing = xdr.TransactionMeta{V: 4, V4: &apply}
			meta.V2.TxProcessing = []xdr.TransactionResultMetaV1{tx}
		}
		h, _ := xdr.MarshalBase64(entry)
		m, _ := xdr.MarshalBase64(meta)
		e, err := history.Decode(protocol.LedgerInfo{Sequence: seq, Hash: hex.EncodeToString(hash[:]), LedgerCloseTime: int64(header.ScpValue.CloseTime), LedgerHeader: h, LedgerMetadata: m})
		if err != nil {
			t.Fatal(err)
		}
		all = append(all, e)
		previous = xdr.Hash(hash)
	}
	return b, all
}
func batch(network string, all []history.Evidence) history.Capture {
	c := evidenceCapture(network, all[0])
	c.ToLedger = all[len(all)-1].Sequence
	c.ProcessedThrough = c.ToLedger
	c.Ledgers = all
	return c
}
func config() Config {
	return Config{SchemaVersion: 1, RPCURL: "https://rpc.example.invalid", Baseline: "baseline.json", State: "state.db", Listen: "127.0.0.1:0", PollIntervalSeconds: 1, BatchSize: 8, HeadDelayLedgers: 0, MaxEndpointLag: 2, MaxRetainedLedgers: 100, MaxRetainedPayloadBytes: 64 << 20}
}
func openState(t testing.TB, path string, b history.Baseline) *Store {
	t.Helper()
	s, err := Open(context.Background(), path, b, config().Limits())
	if err != nil {
		t.Fatal(err)
	}
	return s
}
func eventBytes(t testing.TB, s *Store) []byte {
	t.Helper()
	out := []byte{}
	err := s.db.View(func(tx *bolt.Tx) error {
		return tx.Bucket(rootName).Bucket(findingName).ForEach(func(k, v []byte) error { out = append(out, v...); return nil })
	})
	if err != nil {
		t.Fatal(err)
	}
	return out
}

func TestFollowBeyondSixteenColdReplayAndBatchStableIDs(t *testing.T) {
	b, all := chain(t)
	var expected []byte
	for _, size := range []int{1, 8, 16} {
		path := filepath.Join(t.TempDir(), "follow.db")
		s := openState(t, path, b)
		for i := 0; i < len(all); i += size {
			end := min(i+size, len(all))
			r, err := s.Commit(context.Background(), batch(b.Snapshot.Network, all[i:end]))
			if err != nil || r.ProcessedThrough != all[end-1].Sequence {
				t.Fatal(r, err)
			}
		}
		head := s.Head()
		if head.RetainedLedgers != 40 || head.Projection.Snapshot.Ledger != 139 || head.PendingFindings != 2 || !strings.EqualFold(head.Projection.Snapshot.References[0].WasmHash, b.Snapshot.References[0].WasmHash) {
			t.Fatal("follow lost a transition across batch boundary", head)
		}
		events := eventBytes(t, s)
		if expected == nil {
			expected = events
		} else if string(events) != string(expected) {
			t.Fatal("event identities depend on batch partition")
		}
		s.Close()
		before, _ := os.ReadFile(path)
		s = openState(t, path, b)
		after, _ := os.ReadFile(path)
		if string(before) != string(after) || string(eventBytes(t, s)) != string(events) {
			t.Fatal("cold validation mutated/duplicated retained state")
		}
		s.Close()
	}
}

func TestFollowCapacityRollbackAndCorruption(t *testing.T) {
	b, all := chain(t)
	path := filepath.Join(t.TempDir(), "state.db")
	s := openState(t, path, b)
	s.limits.MaxLedgers = 1
	if _, err := s.Commit(context.Background(), batch(b.Snapshot.Network, all[:2])); !errors.Is(err, ErrCapacity) {
		t.Fatal(err)
	}
	if s.Head().RetainedLedgers != 0 || s.Head().PendingFindings != 0 {
		t.Fatal("capacity published partial transaction")
	}
	s.limits = config().Limits()
	if _, err := s.Commit(context.Background(), batch(b.Snapshot.Network, all[:2])); err != nil {
		t.Fatal(err)
	}
	prior := s.Head()
	s.limits.MaxPayloadBytes = prior.RetainedPayloadBytes + 1 // Inject a tight remaining payload budget.
	if _, err := s.Commit(context.Background(), batch(b.Snapshot.Network, all[2:3])); !errors.Is(err, ErrCapacity) {
		t.Fatal(err)
	}
	if s.Head().Projection.Snapshot.Ledger != 101 {
		t.Fatal("byte capacity advanced cursor")
	}
	s.limits = config().Limits()
	err := s.db.Update(func(tx *bolt.Tx) error {
		return tx.Bucket(rootName).Bucket(findingName).Put([]byte("orphan"), []byte(`{}`))
	})
	if err != nil {
		t.Fatal(err)
	}
	s.Close()
	before, _ := os.ReadFile(path)
	if got, err := Open(context.Background(), path, b, config().Limits()); err == nil {
		got.Close()
		t.Fatal("orphan queue evidence accepted")
	}
	after, _ := os.ReadFile(path)
	if string(before) != string(after) {
		t.Fatal("failed validation modified database")
	}
}

func TestFollowTransactionRollbackAndExclusiveWriter(t *testing.T) {
	b, all := chain(t)
	path := filepath.Join(t.TempDir(), "state.db")
	s := openState(t, path, b)
	defer s.Close()
	if second, err := Open(context.Background(), path, b, config().Limits()); err == nil {
		second.Close()
		t.Fatal("second writer acquired lock")
	}
	if _, err := s.commit(context.Background(), batch(b.Snapshot.Network, all[:8]), func() error { return errors.New("injected commit interruption") }); err == nil {
		t.Fatal("commit hook did not fail")
	}
	if s.Head().RetainedLedgers != 0 || len(eventBytes(t, s)) != 0 {
		t.Fatal("rollback lost atomic evidence/findings")
	}
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	if _, err := s.Commit(ctx, batch(b.Snapshot.Network, all[:8])); !errors.Is(err, context.Canceled) {
		t.Fatal(err)
	}
}

func TestFollowRefusesTruncatedOrForeignState(t *testing.T) {
	b, _ := chain(t)
	dir := t.TempDir()
	for _, name := range []string{"empty", "foreign"} {
		path := filepath.Join(dir, name+".db")
		if name == "empty" {
			if err := os.WriteFile(path, nil, 0600); err != nil {
				t.Fatal(err)
			}
		} else {
			db, err := bolt.Open(path, 0600, nil)
			if err != nil {
				t.Fatal(err)
			}
			if err := db.Update(func(tx *bolt.Tx) error { _, err := tx.CreateBucket([]byte("other-application")); return err }); err != nil {
				t.Fatal(err)
			}
			db.Close()
		}
		before, _ := os.ReadFile(path)
		if s, err := Open(context.Background(), path, b, config().Limits()); err == nil {
			s.Close()
			t.Fatal("truncated/foreign DB was reinitialized")
		}
		after, _ := os.ReadFile(path)
		if string(before) != string(after) {
			t.Fatal("rejected DB was changed")
		}
	}
}

func TestFollowCrashHelper(t *testing.T) {
	path := os.Getenv("FLEETWATCH_FOLLOW_CRASH_DB")
	if path == "" {
		t.Skip("subprocess invoked by parent")
	}
	b, all := chain(t)
	s := openState(t, path, b)
	_, _ = s.commit(context.Background(), batch(b.Snapshot.Network, all[:8]), func() error { os.Exit(29); return nil })
	os.Exit(30)
}
func TestFollowProcessCrashBeforeCommit(t *testing.T) {
	b, all := chain(t)
	path := filepath.Join(t.TempDir(), "state.db")
	s := openState(t, path, b)
	s.Close()
	cmd := exec.Command(os.Args[0], "-test.run=^TestFollowCrashHelper$")
	cmd.Env = append(os.Environ(), "FLEETWATCH_FOLLOW_CRASH_DB="+path)
	err := cmd.Run()
	var code *exec.ExitError
	if !errors.As(err, &code) || code.ExitCode() != 29 {
		t.Fatal("crash helper did not execute", err)
	}
	s = openState(t, path, b)
	defer s.Close()
	if s.Head().RetainedLedgers != 0 || s.Head().PendingFindings != 0 {
		t.Fatal("uncommitted ledger survived process exit")
	}
	if _, err := s.Commit(context.Background(), batch(b.Snapshot.Network, all[:8])); err != nil || s.Head().PendingFindings != 1 {
		t.Fatal("cold resume failed", err)
	}
}

type fakeRPC struct {
	baseline     history.Baseline
	all          []history.Evidence
	missing      uint32
	fail         bool
	head         uint32
	wrongNetwork bool
	oldHead      bool
}

func (r *fakeRPC) Network(context.Context) (source.Network, error) {
	if r.fail {
		return source.Network{}, errors.New("private upstream detail")
	}
	if r.wrongNetwork {
		return source.Network{Passphrase: "wrong", ProtocolVersion: 29}, nil
	}
	return source.Network{Passphrase: r.baseline.Snapshot.Network, ProtocolVersion: 29}, nil
}
func (r *fakeRPC) Head(context.Context) (source.Head, error) {
	at := time.Now().Unix()
	if r.oldHead {
		at -= 121
	}
	return source.Head{Sequence: r.head, ProtocolVersion: 29, CloseTime: at}, nil
}
func (r *fakeRPC) Ledgers(_ context.Context, req protocol.GetLedgersRequest) (protocol.GetLedgersResponse, error) {
	wanted := req.StartLedger
	if wanted == r.missing {
		wanted++
	}
	out := protocol.GetLedgersResponse{OldestLedger: 100, LatestLedger: r.head}
	for _, e := range r.all {
		if e.Sequence == wanted {
			out.Ledgers = []protocol.LedgerInfo{{Sequence: e.Sequence, Hash: e.Hash, LedgerCloseTime: e.CloseTime, LedgerHeader: e.HeaderXDR, LedgerMetadata: e.MetadataXDR}}
			break
		}
	}
	return out, nil
}

func TestFollowMissingLedgerRecoveryAndReadiness(t *testing.T) {
	b, all := chain(t)
	s := openState(t, filepath.Join(t.TempDir(), "state.db"), b)
	defer s.Close()
	cfg := config()
	rpc := &fakeRPC{baseline: b, all: all, head: 139, missing: 103}
	health := NewHealth(s.Head(), cfg)
	if health.Snapshot(time.Now()).Ready {
		t.Fatal("new process ready without source check")
	}
	c, err := Poll(context.Background(), rpc, s, cfg, Cycle{Code: "STARTING"}, time.Now())
	if err != nil || c.Code != "SOURCE_UNAVAILABLE" || c.GapEpisodes != 1 || s.Head().Projection.Snapshot.Ledger != 102 {
		t.Fatal(c, err, s.Head())
	}
	if err := s.SaveCycle(context.Background(), c); err != nil {
		t.Fatal(err)
	}
	health.Update(s.Head(), c)
	if health.Snapshot(time.Now()).Ready {
		t.Fatal("partial history ready")
	}
	c, err = Poll(context.Background(), rpc, s, cfg, c, time.Now())
	if err != nil || c.GapEpisodes != 1 {
		t.Fatal("poll spam counted repeated gap episodes", c, err)
	}
	rpc.missing = 0
	for s.Head().Projection.Snapshot.Ledger < 139 {
		c, err = Poll(context.Background(), rpc, s, cfg, c, time.Now())
		if err != nil {
			t.Fatal(err)
		}
	}
	health.Update(s.Head(), c)
	if !health.Snapshot(time.Now()).Ready || health.Snapshot(time.Now().Add(time.Hour)).Ready {
		t.Fatal("readiness did not track lag/freshness")
	}
	rpc.fail = true
	c, err = Poll(context.Background(), rpc, s, cfg, c, time.Now())
	if err != nil || c.GapEpisodes != 2 {
		t.Fatal(c, err)
	}
	rpc.fail = false
	rpc.head = 138
	c, err = Poll(context.Background(), rpc, s, cfg, c, time.Now())
	if !errors.Is(err, ErrEvidence) {
		t.Fatal("endpoint regression after outage accepted", c, err)
	}
}

func TestFollowRejectsWrongNetworkStaleHeadAndCrossBatchLink(t *testing.T) {
	b, all := chain(t)
	for _, mode := range []string{"network", "stale", "metadata", "link"} {
		t.Run(mode, func(t *testing.T) {
			s := openState(t, filepath.Join(t.TempDir(), "state.db"), b)
			defer s.Close()
			rpc := &fakeRPC{baseline: b, all: append([]history.Evidence{}, all...), head: 139}
			if mode == "network" {
				rpc.wrongNetwork = true
			}
			if mode == "stale" {
				rpc.oldHead = true
			}
			if mode == "metadata" {
				rpc.all[0].MetadataXDR = "invalid"
			}
			through := uint32(99)
			if mode == "link" {
				if _, err := s.Commit(context.Background(), batch(b.Snapshot.Network, all[:8])); err != nil {
					t.Fatal(err)
				}
				through = 107
				e := all[8]
				var header xdr.LedgerHeaderHistoryEntry
				var meta xdr.LedgerCloseMeta
				xdr.SafeUnmarshalBase64(e.HeaderXDR, &header)
				xdr.SafeUnmarshalBase64(e.MetadataXDR, &meta)
				header.Header.PreviousLedgerHash = xdr.Hash{99}
				raw, _ := header.Header.MarshalBinary()
				hash := sha256.Sum256(raw)
				header.Hash = xdr.Hash(hash)
				meta.V2.LedgerHeader = header
				h, _ := xdr.MarshalBase64(header)
				m, _ := xdr.MarshalBase64(meta)
				e, err := history.Decode(protocol.LedgerInfo{Sequence: e.Sequence, Hash: hex.EncodeToString(hash[:]), LedgerCloseTime: e.CloseTime, LedgerHeader: h, LedgerMetadata: m})
				if err != nil {
					t.Fatal(err)
				}
				rpc.all = []history.Evidence{e}
				rpc.head = 108
			}
			cycle, err := Poll(context.Background(), rpc, s, config(), Cycle{Code: "STARTING"}, time.Now())
			if !errors.Is(err, ErrEvidence) || cycle.Code != "EVIDENCE_REJECTED" || s.Head().Projection.Snapshot.Ledger != through {
				t.Fatal("rejected evidence advanced cursor", cycle, err)
			}
		})
	}
}

func TestFollowRunGracefulCancellationAndOutputFailure(t *testing.T) {
	b, all := chain(t)
	for _, failOutput := range []bool{false, true} {
		s := openState(t, filepath.Join(t.TempDir(), "state.db"), b)
		cfg := config()
		rpc := &fakeRPC{baseline: b, all: all, head: 99}
		health := NewHealth(s.Head(), cfg)
		ctx, cancel := context.WithCancel(context.Background())
		calls := 0
		err := Run(ctx, rpc, s, cfg, health, func(status Status) error {
			calls++
			if status.FinalVerdict != "UNDETERMINED" {
				t.Fatal("follow changed assurance")
			}
			if failOutput {
				return errors.New("output unavailable")
			}
			cancel()
			return nil
		})
		cancel()
		if calls != 1 || (err != nil) != failOutput {
			t.Fatal(calls, err)
		}
		if !failOutput {
			cycle, err := s.Cycle()
			if err != nil || cycle.Code != "STOPPED" || health.Snapshot(time.Now()).Ready {
				t.Fatal("shutdown did not persist not-ready", cycle, err)
			}
		}
		s.Close()
	}
}
