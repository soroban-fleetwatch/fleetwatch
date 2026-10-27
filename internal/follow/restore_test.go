package follow

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
)

func archiveDigest(raw []byte) string {
	h := sha256.Sum256(raw)
	return hex.EncodeToString(h[:])
}

func TestFollowRestoreRoundTripAndContinueOriginalHistory(t *testing.T) {
	b, all := chain(t)
	for _, count := range []int{0, 24} {
		dir := t.TempDir()
		s := openState(t, filepath.Join(dir, "source.db"), b)
		for i := 0; i < count; i += 8 {
			if _, err := s.Commit(context.Background(), batch(b.Snapshot.Network, all[i:i+8])); err != nil {
				t.Fatal(err)
			}
		}
		archive := filepath.Join(dir, "archive.gz")
		if err := s.SaveCycle(context.Background(), Cycle{Code: "SOURCE_UNAVAILABLE", At: 1, Polls: 99, GapEpisodes: 7, HighWaterHead: 150}); err != nil {
			t.Fatal(err)
		}
		if _, err := s.ExportArchive(context.Background(), archive); err != nil {
			t.Fatal(err)
		}
		raw, _ := os.ReadFile(archive)
		destination := filepath.Join(dir, "restored.db")
		receipt, err := RestoreArchive(context.Background(), bytes.NewReader(raw), destination, archiveDigest(raw))
		if err != nil {
			t.Fatal(err)
		}
		if receipt.OperationalMetadata != "RESET_NOT_ARCHIVED" || !reflect.DeepEqual(receipt.Baseline, b) || receipt.Archive.RetainedLedgers != count || receipt.Archive.FinalVerdict != "UNDETERMINED" {
			t.Fatal(receipt)
		}
		physical, _ := os.ReadFile(destination)
		if receipt.StateSHA256 != archiveDigest(physical) {
			t.Fatal("state digest mismatch")
		}
		info, _ := os.Stat(destination)
		if info.Mode().Perm() != 0600 {
			t.Fatal("state file is not private")
		}
		restored := openState(t, destination, receipt.Baseline)
		if !reflect.DeepEqual(s.Head(), restored.Head()) || !bytes.Equal(eventBytes(t, s), eventBytes(t, restored)) {
			t.Fatal("restore lost projection, original provenance or finding identity")
		}
		cycle, err := restored.Cycle()
		if err != nil || cycle.Code != "STARTING" || cycle.Polls != 0 || cycle.GapEpisodes != 0 || cycle.HighWaterHead != 0 {
			t.Fatal("restore invented operational history", cycle, err)
		}
		if NewHealth(restored.Head(), config()).Snapshot(time.Now()).Ready {
			t.Fatal("offline restore claimed readiness")
		}
		for i := count; i < len(all); i += 8 {
			for _, state := range []*Store{s, restored} {
				if _, err := state.Commit(context.Background(), batch(b.Snapshot.Network, all[i:i+8])); err != nil {
					t.Fatal("resume failed", err)
				}
			}
		}
		if !reflect.DeepEqual(s.Head(), restored.Head()) || !bytes.Equal(eventBytes(t, s), eventBytes(t, restored)) || restored.Head().PendingFindings != 2 {
			t.Fatal("continued history or events differ")
		}
		s.Close()
		restored.Close()
		entries, _ := os.ReadDir(dir)
		if len(entries) != 3 {
			t.Fatal("restore leaked staging data", entries)
		}
	}
}

func TestFollowRestoreRejectsInvalidInputAndCleansStaging(t *testing.T) {
	_, _, raw := archiveFixture(t)
	dir := t.TempDir()
	path := filepath.Join(dir, "restored.db")
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	if _, err := RestoreArchive(ctx, bytes.NewReader(raw), path, archiveDigest(raw)); !errors.Is(err, context.Canceled) {
		t.Fatal(err)
	}
	for _, test := range []struct {
		raw    []byte
		digest string
	}{
		{raw, "invalid"}, {raw, "0000000000000000000000000000000000000000000000000000000000000000"},
		{raw[:len(raw)-1], archiveDigest(raw[:len(raw)-1])}, {[]byte("invalid"), archiveDigest([]byte("invalid"))},
	} {
		if _, err := RestoreArchive(context.Background(), bytes.NewReader(test.raw), path, test.digest); err == nil {
			t.Fatal("invalid archive/digest accepted")
		}
		entries, _ := os.ReadDir(dir)
		if len(entries) != 0 {
			t.Fatal("failed restore left published/temporary files", entries)
		}
	}
	if _, err := restoreArchive(context.Background(), bytes.NewReader(raw), path, archiveDigest(raw), func() error { return errors.New("interrupted") }); err == nil {
		t.Fatal("prepublication fault ignored")
	}
	if _, err := os.Stat(path); !os.IsNotExist(err) {
		t.Fatal("failed restore published state")
	}
	if _, err := restoreArchive(context.Background(), bytes.NewReader(raw), path, archiveDigest(raw), func() error { return os.WriteFile(path, []byte("preserve"), 0600) }); err == nil {
		t.Fatal("racing destination overwritten")
	}
	if _, err := RestoreArchive(context.Background(), bytes.NewReader(raw), path, archiveDigest(raw)); err == nil {
		t.Fatal("existing destination accepted")
	}
	if err := os.Symlink(path, filepath.Join(dir, "link.db")); err != nil {
		t.Fatal(err)
	}
	if _, err := RestoreArchive(context.Background(), bytes.NewReader(raw), filepath.Join(dir, "link.db"), archiveDigest(raw)); err == nil {
		t.Fatal("symlink accepted")
	}
	prior, _ := os.ReadFile(path)
	entries, _ := os.ReadDir(dir)
	if string(prior) != "preserve" || len(entries) != 2 {
		t.Fatal("existing destination lost or temporary data leaked")
	}
}
