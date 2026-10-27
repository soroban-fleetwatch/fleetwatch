package store

import (
	"bytes"
	"context"
	"os"
	"path/filepath"
	"testing"
	"time"

	bolt "go.etcd.io/bbolt"
)

func foreignUnsyncedFreelist(t testing.TB, path string) {
	t.Helper()
	db, err := bolt.Open(path, 0600, &bolt.Options{NoFreelistSync: true})
	if err != nil {
		t.Fatal(err)
	}
	if err := db.Update(func(tx *bolt.Tx) error {
		b, err := tx.CreateBucket([]byte("foreign-application"))
		if err != nil {
			return err
		}
		return b.Put([]byte("preserve"), []byte("original foreign data"))
	}); err != nil {
		db.Close()
		t.Fatal(err)
	}
	if err := db.Close(); err != nil {
		t.Fatal(err)
	}
}

func TestOpenExistingRejectsForeignBeforeFreelistMutation(t *testing.T) {
	path := filepath.Join(t.TempDir(), "foreign.db")
	foreignUnsyncedFreelist(t, path)
	before, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	if s, err := OpenExisting(path); err == nil {
		s.Close()
		t.Fatal("foreign namespace accepted")
	}
	after, err := os.ReadFile(path)
	if err != nil || !bytes.Equal(before, after) {
		t.Fatal("recognition modified a rejected foreign database", err)
	}
}

func TestOpenExistingRejectsMissingEmptyMalformedAndSwappedFiles(t *testing.T) {
	dir := t.TempDir()
	for _, name := range []string{"missing", "empty", "malformed"} {
		path := filepath.Join(dir, name+".db")
		var before []byte
		if name != "missing" {
			if name == "malformed" {
				before = []byte("unrecognized non-bbolt data")
			}
			if err := os.WriteFile(path, before, 0600); err != nil {
				t.Fatal(err)
			}
		}
		if s, err := OpenExisting(path); err == nil {
			s.Close()
			t.Fatal("invalid state accepted", name)
		}
		after, err := os.ReadFile(path)
		if name == "missing" {
			if !os.IsNotExist(err) {
				t.Fatal("missing state created")
			}
		} else if err != nil || !bytes.Equal(before, after) {
			t.Fatal("rejected state modified", name, err)
		}
	}
	path := filepath.Join(dir, "replaced.db")
	s, err := Open(path)
	if err != nil {
		t.Fatal(err)
	}
	s.Close()
	identity, err := os.Lstat(path)
	if err != nil {
		t.Fatal(err)
	}
	if err := os.Rename(path, filepath.Join(dir, "recognized-original.db")); err != nil {
		t.Fatal(err)
	}
	foreignUnsyncedFreelist(t, path)
	before, _ := os.ReadFile(path)
	if s, err := openStore(path, true, identity); err == nil {
		s.Close()
		t.Fatal("path replacement after recognition accepted")
	}
	if f, err := pinnedStateOpen(identity)(path, os.O_RDWR|os.O_CREATE, 0600); err == nil {
		f.Close()
		t.Fatal("opened descriptor identity replacement accepted")
	}
	after, _ := os.ReadFile(path)
	if !bytes.Equal(before, after) {
		t.Fatal("replaced foreign file modified")
	}
	missing := filepath.Join(dir, "removed-after-recognition.db")
	if f, err := pinnedStateOpen(identity)(missing, os.O_RDWR|os.O_CREATE, 0600); err == nil {
		f.Close()
		t.Fatal("pinned opener created removed state")
	}
	if _, err := os.Stat(missing); !os.IsNotExist(err) {
		t.Fatal("pinned opener created a new file")
	}
}

func TestOpenExistingRetainsWritableFollowAcknowledgementBehavior(t *testing.T) {
	binding, events := followImportFixture(t)
	path := filepath.Join(t.TempDir(), "delivery.db")
	s, err := Open(path)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := s.ImportFollow(context.Background(), binding, events, nil); err != nil {
		t.Fatal(err)
	}
	s.Close()
	s, err = OpenExisting(path)
	if err != nil {
		t.Fatal(err)
	}
	source, _ := AlertSource(binding.Network, "follow_ledger_meta", binding.Stream)
	claimed, err := s.Claim(binding.Network, source, "existing-state-test", time.Now(), time.Minute)
	if err != nil || claimed == nil {
		t.Fatal("recognized state not writable", err)
	}
	if err := s.Resolve(binding.Network, source, claimed.ID, claimed.LeaseToken, true, false, time.Now()); err != nil {
		t.Fatal(err)
	}
	s.Close()
	s, err = OpenReadOnly(path)
	if err != nil {
		t.Fatal(err)
	}
	defer s.Close()
	rows, err := s.FollowAlerts(context.Background(), binding.Network, binding.Stream)
	if err != nil {
		t.Fatal(err)
	}
	for _, row := range rows {
		if row.ID == claimed.ID {
			if row.Status != "DELIVERED" || row.Attempts != 1 {
				t.Fatal("acknowledgement not persisted")
			}
			return
		}
	}
	t.Fatal("acknowledged row missing")
}
