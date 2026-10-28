package follow

import (
	"context"
	"os"
	"path/filepath"
	"testing"

	"github.com/soroban-fleetwatch/fleetwatch/internal/store"
)

func TestFollowBackupReplayAndFindingPages(t *testing.T) {
	b, all := chain(t)
	dir := t.TempDir()
	path := filepath.Join(dir, "original.db")
	s := openState(t, path, b)
	for start := 0; start < 32; start += 8 {
		if _, err := s.Commit(context.Background(), batch(b.Snapshot.Network, all[start:start+8])); err != nil {
			t.Fatal(err)
		}
	}
	page, more, err := s.Findings(context.Background(), "", 1)
	if err != nil || !more || len(page) != 1 {
		t.Fatal(page, more, err)
	}
	next, more, err := s.Findings(context.Background(), page[0].Event.ID, 1)
	if err != nil || more || len(next) != 1 || next[0].Event.ID == page[0].Event.ID {
		t.Fatal(next, more, err)
	}
	s.Close()
	before, _ := os.ReadFile(path)
	if legacy, err := store.OpenReadOnly(path); err == nil {
		legacy.Close()
		t.Fatal("legacy health reader adopted follow state")
	}
	reader, err := store.OpenBackupReadOnly(path)
	if err != nil {
		t.Fatal(err)
	}
	backup := filepath.Join(dir, "backup.db")
	receipt, err := reader.Backup(context.Background(), backup)
	reader.Close()
	if err != nil || receipt.Bytes == 0 {
		t.Fatal(receipt, err)
	}
	s = openState(t, backup, b)
	defer s.Close()
	if s.Head().Projection.Snapshot.Ledger != 131 || s.Head().PendingFindings != 2 {
		t.Fatal("backup lost follow state")
	}
	after, _ := os.ReadFile(path)
	if string(before) != string(after) {
		t.Fatal("backup modified source database")
	}
}
