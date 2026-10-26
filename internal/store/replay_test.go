package store

import (
	"encoding/json"
	"errors"
	bolt "go.etcd.io/bbolt"
	"os"
	"os/exec"
	"path/filepath"
	"reflect"
	"testing"

	"github.com/soroban-fleetwatch/fleetwatch/internal/fleetwatch"
)

func fixture(t *testing.T) fleetwatch.Journal {
	t.Helper()
	j, err := fleetwatch.LoadJournal(filepath.Join("..", "..", "testdata", "replay-transient.json"))
	if err != nil {
		t.Fatal(err)
	}
	return j
}
func baseline(j fleetwatch.Journal) fleetwatch.Journal {
	j.ToLedger = j.FromLedger
	j.Snapshots = j.Snapshots[:1]
	return j
}
func openTest(t *testing.T, path string) *Store {
	t.Helper()
	s, err := Open(path)
	if err != nil {
		t.Fatal(err)
	}
	return s
}
func TestReplayPersistsResumesAndDeduplicates(t *testing.T) {
	path := filepath.Join(t.TempDir(), "state.db")
	j := fixture(t)
	s := openTest(t, path)
	r, err := s.Replay(baseline(j))
	if err != nil || r.ProcessedThrough != 100 {
		t.Fatalf("%+v %v", r, err)
	}
	if err := s.Close(); err != nil {
		t.Fatal(err)
	}
	s = openTest(t, path)
	defer s.Close()
	segment := j
	segment.FromLedger = 101
	segment.Snapshots = segment.Snapshots[1:]
	first, err := s.Replay(segment)
	if err != nil || first.ProcessedThrough != 102 || len(first.Events) != 2 {
		t.Fatalf("%+v %v", first, err)
	}
	again, err := s.Replay(j)
	if err != nil || !reflect.DeepEqual(first, again) {
		t.Fatal("duplicate replay changed stored events", err)
	}
	stored, found, err := s.Load(j.Network, j.Source)
	if err != nil || !found || len(stored.Report.Events) != 2 || len(stored.Journal.Snapshots) != 3 {
		t.Fatalf("%+v found=%v err=%v", stored, found, err)
	}
	info, _ := os.Stat(path)
	if info.Mode().Perm()&0077 != 0 {
		t.Fatal("database accessible to other users")
	}
}
func TestGapAndConflictNeverAdvanceCursor(t *testing.T) {
	s := openTest(t, filepath.Join(t.TempDir(), "state.db"))
	defer s.Close()
	j := fixture(t)
	if _, err := s.Replay(baseline(j)); err != nil {
		t.Fatal(err)
	}
	gap := j
	gap.Snapshots = []fleetwatch.Snapshot{j.Snapshots[0], j.Snapshots[2]}
	r, err := s.Replay(gap)
	if err != nil || r.ProcessedThrough != 100 || len(r.Events) != 0 || len(r.Issues) != 1 {
		t.Fatalf("%+v %v", r, err)
	}
	conflict := baseline(j)
	conflict.Snapshots = append([]fleetwatch.Snapshot{}, conflict.Snapshots...)
	bad := j.Snapshots[1]
	bad.Ledger = 100
	conflict.Snapshots[0] = bad
	if _, err := s.Replay(conflict); !errors.Is(err, ErrConflict) {
		t.Fatal("conflict accepted", err)
	}
	stored, _, err := s.Load(j.Network, j.Source)
	if err != nil || stored.Report.ProcessedThrough != 100 || len(stored.Report.Events) != 0 {
		t.Fatal("conflict changed cursor", err)
	}
	if _, err := s.Replay(j); err != nil {
		t.Fatal("gap repair failed", err)
	}
}
func TestAbortedTransactionRollsBackEventsAndCheckpoint(t *testing.T) {
	s := openTest(t, filepath.Join(t.TempDir(), "state.db"))
	defer s.Close()
	j := fixture(t)
	if _, err := s.Replay(baseline(j)); err != nil {
		t.Fatal(err)
	}
	injected := errors.New("simulated write failure")
	if _, err := s.replay(j, func() error { return injected }); !errors.Is(err, injected) {
		t.Fatal("fault injection failed", err)
	}
	stored, _, err := s.Load(j.Network, j.Source)
	if err != nil || stored.Report.ProcessedThrough != 100 || len(stored.Report.Events) != 0 {
		t.Fatal("transaction partially committed", err)
	}
}
func TestStoreScopeAndFileLock(t *testing.T) {
	path := filepath.Join(t.TempDir(), "state.db")
	s := openTest(t, path)
	defer s.Close()
	j := fixture(t)
	if _, err := s.Replay(j); err != nil {
		t.Fatal(err)
	}
	if _, found, err := s.Load("another network", j.Source); err != nil || found {
		t.Fatal("cross-network state mixed", err)
	}
	if _, found, err := s.Load(j.Network, "user_snapshot"); err != nil || found {
		t.Fatal("source kinds mixed", err)
	}
	if second, err := Open(path); err == nil {
		second.Close()
		t.Fatal("concurrent writable store accepted")
	}
}
func TestProcessCrashBeforeCommit(t *testing.T) {
	path := filepath.Join(t.TempDir(), "state.db")
	j := fixture(t)
	s := openTest(t, path)
	if _, err := s.Replay(baseline(j)); err != nil {
		t.Fatal(err)
	}
	s.Close()
	cmd := exec.Command(os.Args[0], "-test.run=^TestCrashHelper$")
	cmd.Env = append(os.Environ(), "FLEETWATCH_CRASH_HELPER=1", "FLEETWATCH_TEST_STATE="+path)
	if err := cmd.Run(); err == nil {
		t.Fatal("expected child crash")
	} else {
		var exit *exec.ExitError
		if !errors.As(err, &exit) || exit.ExitCode() != 23 {
			t.Fatal(err)
		}
	}
	s = openTest(t, path)
	defer s.Close()
	stored, _, err := s.Load(j.Network, j.Source)
	if err != nil || stored.Report.ProcessedThrough != 100 || len(stored.Report.Events) != 0 {
		t.Fatalf("partial crash commit: %+v %v", stored, err)
	}
	rows, queueErr := s.Alerts(j.Network, j.Source)
	if queueErr != nil || len(rows) != 0 {
		t.Fatal("outbox survived uncommitted process crash", queueErr)
	}
	if r, err := s.Replay(j); err != nil || len(r.Events) != 2 || r.ProcessedThrough != 102 {
		t.Fatalf("restart failed: %+v %v", r, err)
	}
}
func TestCrashHelper(t *testing.T) {
	if os.Getenv("FLEETWATCH_CRASH_HELPER") != "1" {
		t.Skip("subprocess crash helper")
	}
	s := openTest(t, os.Getenv("FLEETWATCH_TEST_STATE"))
	_, _ = s.replay(fixture(t), func() error { os.Exit(23); return nil })
	t.Fatal("crash hook not reached")
}
func TestRejectsSymlinkAndUnknownDatabase(t *testing.T) {
	dir := t.TempDir()
	bad := filepath.Join(dir, "unknown")
	os.WriteFile(bad, []byte("do not overwrite"), 0600)
	if s, err := Open(bad); err == nil {
		s.Close()
		t.Fatal("non-database file accepted")
	}
	data, _ := os.ReadFile(bad)
	if string(data) != "do not overwrite" {
		t.Fatal("existing file changed")
	}
	link := filepath.Join(dir, "link")
	if err := os.Symlink(bad, link); err != nil {
		t.Fatal(err)
	}
	if s, err := Open(link); err == nil {
		s.Close()
		t.Fatal("symlink state accepted")
	}
}
func TestMissingBaselineCanBeRepaired(t *testing.T) {
	s := openTest(t, filepath.Join(t.TempDir(), "state.db"))
	defer s.Close()
	j := fixture(t)
	empty := j
	empty.Snapshots = nil
	r, err := s.Replay(empty)
	if err != nil || r.ProcessedThrough != 0 {
		t.Fatalf("%+v %v", r, err)
	}
	if _, _, err := s.Load(j.Network, j.Source); err != nil {
		t.Fatal(err)
	}
	if r, err := s.Replay(j); err != nil || r.ProcessedThrough != 102 {
		t.Fatalf("%+v %v", r, err)
	}
}
func TestStateRejectsUnsupportedSchema(t *testing.T) {
	var state State
	if err := decodeState([]byte(`{"schema_version":99}`), &state); err == nil {
		t.Fatal("unsupported state accepted")
	}
}

func TestStoredFindingsAreCheckedAgainstObservations(t *testing.T) {
	s := openTest(t, filepath.Join(t.TempDir(), "state.db"))
	defer s.Close()
	j := fixture(t)
	if _, err := s.Replay(j); err != nil {
		t.Fatal(err)
	}
	state, _, err := s.Load(j.Network, j.Source)
	if err != nil {
		t.Fatal(err)
	}
	state.Report.Events[0].ID = "forged-id"
	raw, err := json.Marshal(state)
	if err != nil {
		t.Fatal(err)
	}
	if err := s.db.Update(func(tx *bolt.Tx) error { return tx.Bucket(stateBucket).Put(scope(j.Network, j.Source), raw) }); err != nil {
		t.Fatal(err)
	}
	if _, _, err := s.Load(j.Network, j.Source); err == nil {
		t.Fatal("corrupt stored findings accepted")
	}
	if _, err := s.Replay(j); err == nil {
		t.Fatal("corrupt state silently overwritten")
	}
}
