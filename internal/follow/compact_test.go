package follow

import (
	"bytes"
	"context"
	"errors"
	"os"
	"path/filepath"
	"reflect"
	"testing"
	"time"

	bolt "go.etcd.io/bbolt"
)

func compactFixture(t testing.TB) (string, *Store) {
	t.Helper()
	b, all := chain(t)
	path := filepath.Join(t.TempDir(), "source.db")
	s := openState(t, path, b)
	for i := 0; i < 24; i += 8 {
		if _, err := s.Commit(context.Background(), batch(b.Snapshot.Network, all[i:i+8])); err != nil {
			t.Fatal(err)
		}
	}
	if err := s.SaveCycle(context.Background(), Cycle{Code: "STOPPED", At: 100, SuccessfulAt: 90, Polls: 71, GapEpisodes: 4, HighWaterHead: 150}); err != nil {
		t.Fatal(err)
	}
	if err := s.db.Update(func(tx *bolt.Tx) error {
		audit, err := tx.CreateBucket([]byte("operator-data"))
		if err != nil {
			return err
		}
		if err := audit.SetSequence(99); err != nil {
			return err
		}
		if err := audit.Put([]byte("empty-value"), []byte{}); err != nil {
			return err
		}
		if err := audit.Put([]byte("binary"), []byte{0, 255, 1}); err != nil {
			return err
		}
		child, err := audit.CreateBucket([]byte("empty-bucket"))
		if err != nil {
			return err
		}
		return child.SetSequence(7)
	}); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { s.Close() })
	return path, s
}

func TestFollowCompactionPreservesAllStateAndContinues(t *testing.T) {
	b, all := chain(t)
	path, s := compactFixture(t)
	head := s.Head()
	cycle, _ := s.Cycle()
	events := eventBytes(t, s)
	logical, err := compactDigest(context.Background(), s.db)
	if err != nil {
		t.Fatal(err)
	}
	s.Close()
	before, _ := os.ReadFile(path)
	destination := filepath.Join(t.TempDir(), "compacted.db")
	receipt, err := CompactState(context.Background(), path, b, config().Limits(), destination)
	if err != nil {
		t.Fatal(err)
	}
	after, _ := os.ReadFile(path)
	physical, _ := os.ReadFile(destination)
	info, _ := os.Stat(destination)
	if !bytes.Equal(before, after) || receipt.StateSHA256 != archiveDigest(physical) || receipt.LogicalSHA256 != logical || receipt.SourceModified || receipt.OperationalHistory != "PRESERVED" || receipt.FinalVerdict != "UNDETERMINED" || info.Mode().Perm() != 0600 {
		t.Fatal("unsafe compaction", receipt)
	}
	if receipt.CompactedBytes >= receipt.SourceBytes {
		t.Fatal("test fixture did not reclaim physical allocation", receipt)
	}
	reopened := openState(t, destination, b)
	defer reopened.Close()
	newCycle, _ := reopened.Cycle()
	if !reflect.DeepEqual(head, reopened.Head()) || !reflect.DeepEqual(cycle, newCycle) || !bytes.Equal(events, eventBytes(t, reopened)) || NewHealth(reopened.Head(), config()).Snapshot(time.Now()).Ready {
		t.Fatal("evidence/counters/readiness changed")
	}
	source := openState(t, path, b)
	defer source.Close()
	for i := 24; i < 40; i += 8 {
		for _, store := range []*Store{source, reopened} {
			if _, err := store.Commit(context.Background(), batch(b.Snapshot.Network, all[i:i+8])); err != nil {
				t.Fatal(err)
			}
		}
	}
	if !reflect.DeepEqual(source.Head(), reopened.Head()) || !bytes.Equal(eventBytes(t, source), eventBytes(t, reopened)) {
		t.Fatal("continued ingestion differs")
	}
}

func TestFollowCompactionChunkingAndCancellation(t *testing.T) {
	_, s := compactFixture(t)
	for _, budget := range []int64{1, 1024, 16 << 20} {
		dst, err := bolt.Open(filepath.Join(t.TempDir(), "copy.db"), 0600, nil)
		if err != nil {
			t.Fatal(err)
		}
		expected, err := copyCompact(context.Background(), s.db, dst, budget)
		if err != nil {
			t.Fatal(err)
		}
		actual, err := compactDigest(context.Background(), dst)
		if err != nil || expected != actual {
			t.Fatal("chunk boundary lost values/buckets/sequences", err)
		}
		dst.Close()
	}
	ctx, cancel := context.WithCancel(context.Background())
	_, err := walkCompact(ctx, s.db, func(_ [][]byte, _, _ []byte, _ uint64) error { cancel(); return nil })
	if !errors.Is(err, context.Canceled) {
		t.Fatal("walk ignored cancellation", err)
	}
}

func TestFollowCompactionRejectsBadStateAndPublicationFailures(t *testing.T) {
	b, _ := chain(t)
	path, s := compactFixture(t)
	s.Close()
	dir := t.TempDir()
	destination := filepath.Join(dir, "copy.db")
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	if _, err := CompactState(ctx, path, b, config().Limits(), destination); !errors.Is(err, context.Canceled) {
		t.Fatal(err)
	}
	if _, err := CompactState(context.Background(), filepath.Join(dir, "missing.db"), b, config().Limits(), destination); err == nil {
		t.Fatal("missing source accepted")
	}
	if _, err := compactState(context.Background(), path, b, config().Limits(), destination, func() error { return errors.New("interrupted") }); err == nil {
		t.Fatal("publication error ignored")
	}
	entries, _ := os.ReadDir(dir)
	if len(entries) != 0 {
		t.Fatal("failed compaction leaked output/staging", entries)
	}
	if _, err := compactState(context.Background(), path, b, config().Limits(), destination, func() error { return os.WriteFile(destination, []byte("preserve"), 0600) }); err == nil {
		t.Fatal("racing target replaced")
	}
	if _, err := CompactState(context.Background(), path, b, config().Limits(), destination); err == nil {
		t.Fatal("existing output replaced")
	}
	if err := os.Symlink(destination, filepath.Join(dir, "link.db")); err != nil {
		t.Fatal(err)
	}
	if _, err := CompactState(context.Background(), path, b, config().Limits(), filepath.Join(dir, "link.db")); err == nil {
		t.Fatal("symlink output replaced")
	}
	prior, _ := os.ReadFile(destination)
	entries, _ = os.ReadDir(dir)
	if string(prior) != "preserve" || len(entries) != 2 {
		t.Fatal("failed publication lost existing data or leaked staging")
	}
}

func TestFollowCompactionRejectsCorruptStateAndExcessiveNesting(t *testing.T) {
	b, _ := chain(t)
	for _, mode := range []string{"corrupt", "nesting"} {
		t.Run(mode, func(t *testing.T) {
			path, s := compactFixture(t)
			if err := s.db.Update(func(tx *bolt.Tx) error {
				if mode == "corrupt" {
					return tx.Bucket(rootName).Put([]byte("head"), []byte(`{}`))
				}
				bucket, err := tx.CreateBucket([]byte("too-deep"))
				if err != nil {
					return err
				}
				for i := 0; i < 64; i++ {
					bucket, err = bucket.CreateBucket([]byte("nested"))
					if err != nil {
						return err
					}
				}
				return nil
			}); err != nil {
				t.Fatal(err)
			}
			s.Close()
			before, _ := os.ReadFile(path)
			dir := t.TempDir()
			if _, err := CompactState(context.Background(), path, b, config().Limits(), filepath.Join(dir, "copy.db")); err == nil {
				t.Fatal("invalid state/nesting accepted")
			}
			after, _ := os.ReadFile(path)
			entries, _ := os.ReadDir(dir)
			if !bytes.Equal(before, after) || len(entries) != 0 {
				t.Fatal("rejected source was modified or staging leaked")
			}
		})
	}
}
