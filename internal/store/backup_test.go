package store

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"os"
	"path/filepath"
	"reflect"
	"testing"
	"time"

	bolt "go.etcd.io/bbolt"
)

func TestBackupRestorePreservesCursorOutboxReceiptsAndNonce(t *testing.T) {
	dir := t.TempDir()
	s := openTest(t, filepath.Join(dir, "source.db"))
	j, envelope, trust := publicBindFixture(t)
	events := replayForBinding(t, s, j)
	binding, err := s.BindApproval(j.Network, j.Source, events[0].ID, envelope, trust)
	if err != nil || !binding.NewBinding {
		t.Fatal(err)
	}
	now := time.Unix(1000, 0)
	var inflight *Alert
	for _, status := range []string{"DELIVERED", "DEAD", "INFLIGHT"} {
		a, err := s.Claim(j.Network, j.Source, "local-test-sink", now, time.Minute)
		if err != nil || a == nil {
			t.Fatal(err)
		}
		if status != "INFLIGHT" {
			if err := s.Resolve(j.Network, j.Source, a.ID, a.LeaseToken, status == "DELIVERED", status == "DEAD", now); err != nil {
				t.Fatal(err)
			}
		} else {
			inflight = a
		}
	}
	other := j
	other.Source = "user_snapshot"
	if _, err := s.Replay(other); err != nil {
		t.Fatal(err)
	}
	a, err := s.Claim(other.Network, other.Source, "local-test-sink", now, time.Minute)
	if err != nil || a == nil {
		t.Fatal(err)
	}
	if err := s.Resolve(other.Network, other.Source, a.ID, a.LeaseToken, false, false, now); err != nil {
		t.Fatal(err)
	}
	state, _, err := s.Load(j.Network, j.Source)
	if err != nil {
		t.Fatal(err)
	}
	alerts, err := s.Alerts(j.Network, j.Source)
	if err != nil {
		t.Fatal(err)
	}
	retries, err := s.Alerts(other.Network, other.Source)
	if err != nil {
		t.Fatal(err)
	}
	if err := s.db.Update(func(tx *bolt.Tx) error {
		b, err := tx.CreateBucket([]byte("future.namespace"))
		if err != nil {
			return err
		}
		return b.Put([]byte("key"), []byte("preserve opaque future data"))
	}); err != nil {
		t.Fatal(err)
	}
	path := filepath.Join(dir, "restored.db")
	receipt, err := s.Backup(context.Background(), path)
	if err != nil {
		t.Fatal(err)
	}
	s.Close()
	raw, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	digest := sha256.Sum256(raw)
	if receipt.SHA256 != hex.EncodeToString(digest[:]) || receipt.Bytes != int64(len(raw)) || receipt.SemanticValidation != "NOT_PERFORMED" {
		t.Fatalf("%+v", receipt)
	}
	info, err := os.Stat(path)
	if err != nil || info.Mode().Perm() != 0600 {
		t.Fatal("backup permission", err)
	}
	restored := openTest(t, path)
	defer restored.Close()
	if err := restored.db.View(func(tx *bolt.Tx) error {
		b := tx.Bucket([]byte("future.namespace"))
		if b == nil || string(b.Get([]byte("key"))) != "preserve opaque future data" {
			return errors.New("future namespace lost")
		}
		return nil
	}); err != nil {
		t.Fatal(err)
	}
	got, found, err := restored.Load(j.Network, j.Source)
	if err != nil || !found || !reflect.DeepEqual(state, got) {
		t.Fatal("state mismatch", err)
	}
	gotAlerts, err := restored.Alerts(j.Network, j.Source)
	if err != nil || !reflect.DeepEqual(alerts, gotAlerts) {
		t.Fatal("delivery state mismatch", err)
	}
	gotRetries, err := restored.Alerts(other.Network, other.Source)
	if err != nil || !reflect.DeepEqual(retries, gotRetries) {
		t.Fatal("retry state mismatch", err)
	}
	repeated, err := restored.BindApproval(j.Network, j.Source, events[0].ID, envelope, trust)
	if err != nil || repeated.NewBinding || !reflect.DeepEqual(binding.Binding, repeated.Binding) {
		t.Fatal("receipt not preserved", err)
	}
	if _, err := restored.BindApproval(j.Network, j.Source, events[2].ID, envelope, trust); !errors.Is(err, ErrApprovalNonceUsed) {
		t.Fatal("nonce reservation lost", err)
	}
	otherEvents := replayForBinding(t, restored, other)
	if _, err := restored.BindApproval(other.Network, other.Source, otherEvents[2].ID, envelope, trust); !errors.Is(err, ErrApprovalNonceUsed) {
		t.Fatal("cross-source nonce reservation lost", err)
	}
	if _, err := restored.Replay(j); err != nil {
		t.Fatal("replay after restore", err)
	}
	rows, err := restored.Alerts(j.Network, j.Source)
	if err != nil || !reflect.DeepEqual(alerts, rows) {
		t.Fatal("replay reset restored delivery state", err)
	}
	if err := restored.Resolve(j.Network, j.Source, inflight.ID, inflight.LeaseToken, true, false, now); err != nil {
		t.Fatal("restored fencing token or deadline lost", err)
	}
}

func TestBackupReadOnlyAndSnapshotIsolation(t *testing.T) {
	dir := t.TempDir()
	source := filepath.Join(dir, "source.db")
	s := openTest(t, source)
	j := fixture(t)
	if _, err := s.Replay(baseline(j)); err != nil {
		t.Fatal(err)
	}
	// Commit an extension after the snapshot has been copied but before it is
	// published: the backup must retain its own coherent earlier checkpoint.
	path := filepath.Join(dir, "snapshot.db")
	if _, err := s.backup(context.Background(), path, MaxBackupBytes, func() error { _, err := s.Replay(j); return err }); err != nil {
		t.Fatal(err)
	}
	r, err := OpenReadOnly(path)
	if err != nil {
		t.Fatal(err)
	}
	state, found, err := r.Load(j.Network, j.Source)
	if err != nil || !found || state.Report.ProcessedThrough != 100 || len(state.Report.Events) != 0 {
		t.Fatalf("%+v %v", state, err)
	}
	r.Close()
	s.Close()
	before, err := os.ReadFile(source)
	if err != nil {
		t.Fatal(err)
	}
	r, err = OpenReadOnly(source)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := r.Backup(context.Background(), filepath.Join(dir, "readonly.db")); err != nil {
		t.Fatal(err)
	}
	r.Close()
	after, err := os.ReadFile(source)
	if err != nil || !bytes.Equal(before, after) {
		t.Fatal("backup changed source bytes", err)
	}
}

func TestBackupRefusesOverwriteAndCleansFailedCopies(t *testing.T) {
	dir := t.TempDir()
	source := filepath.Join(dir, "source.db")
	s := openTest(t, source)
	defer s.Close()
	j := fixture(t)
	if _, err := s.Replay(j); err != nil {
		t.Fatal(err)
	}
	for _, kind := range []string{"existing", "symlink", "directory", "race", "injected", "limit", "cancel"} {
		t.Run(kind, func(t *testing.T) {
			path := filepath.Join(dir, kind+".db")
			limit := MaxBackupBytes
			ctx := context.Background()
			var hook func() error
			sentinel := []byte("preserve this file")
			switch kind {
			case "existing":
				if err := os.WriteFile(path, sentinel, 0600); err != nil {
					t.Fatal(err)
				}
			case "symlink":
				if err := os.Symlink(source, path); err != nil {
					t.Fatal(err)
				}
			case "directory":
				if err := os.Mkdir(path, 0700); err != nil {
					t.Fatal(err)
				}
			case "race":
				hook = func() error { return os.WriteFile(path, sentinel, 0600) }
			case "injected":
				hook = func() error { return errors.New("abort before publication") }
			case "limit":
				limit = 1
			case "cancel":
				var cancel context.CancelFunc
				ctx, cancel = context.WithCancel(ctx)
				hook = func() error { cancel(); return nil }
				defer cancel()
			}
			if receipt, err := s.backup(ctx, path, limit, hook); err == nil || receipt.SchemaVersion != 0 {
				t.Fatalf("unexpected success: %+v %v", receipt, err)
			}
			if kind == "existing" || kind == "race" {
				raw, err := os.ReadFile(path)
				if err != nil || !bytes.Equal(raw, sentinel) {
					t.Fatal("overwrote existing destination", err)
				}
			}
			if kind == "injected" || kind == "limit" || kind == "cancel" {
				if _, err := os.Stat(path); !os.IsNotExist(err) {
					t.Fatal("published failed copy")
				}
			}
			left, err := filepath.Glob(filepath.Join(dir, ".fleetwatch-backup-*"))
			if err != nil || len(left) != 0 {
				t.Fatal("temporary files leaked", left, err)
			}
		})
	}
	before, err := os.ReadFile(source)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := s.Backup(context.Background(), source); err == nil {
		t.Fatal("overwrote source database")
	}
	after, err := os.ReadFile(source)
	if err != nil || !bytes.Equal(before, after) {
		t.Fatal("source changed", err)
	}
}

func TestBackupWriterCancellationAndBound(t *testing.T) {
	var buf bytes.Buffer
	ctx, cancel := context.WithCancel(context.Background())
	w := &backupWriter{ctx: ctx, writer: &buf, remaining: 2}
	if n, err := w.Write([]byte("abc")); n != 0 || err == nil || buf.Len() != 0 {
		t.Fatal("size bound failed")
	}
	if n, err := w.Write([]byte("ab")); n != 2 || err != nil {
		t.Fatal(n, err)
	}
	cancel()
	if n, err := w.Write([]byte("c")); n != 0 || !errors.Is(err, context.Canceled) {
		t.Fatal(n, err)
	}
}

func TestBackupOlderHistoryDoesNotKnowLaterNonceConsumption(t *testing.T) {
	dir := t.TempDir()
	s := openTest(t, filepath.Join(dir, "current.db"))
	j, envelope, trust := publicBindFixture(t)
	events := replayForBinding(t, s, j)
	older := filepath.Join(dir, "older.db")
	if _, err := s.Backup(context.Background(), older); err != nil {
		t.Fatal(err)
	}
	if _, err := s.BindApproval(j.Network, j.Source, events[0].ID, envelope, trust); err != nil {
		t.Fatal(err)
	}
	s.Close()
	restored := openTest(t, older)
	defer restored.Close()
	// The snapshot predates consumption. No local-only format can discover the
	// later reservation from a newer database history without reconciliation.
	r, err := restored.BindApproval(j.Network, j.Source, events[2].ID, envelope, trust)
	if err != nil || !r.NewBinding {
		t.Fatal("unexpected global nonce guarantee", err)
	}
}
