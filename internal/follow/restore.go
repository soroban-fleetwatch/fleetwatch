package follow

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"io"
	"os"
	"path/filepath"
	"strings"

	"github.com/soroban-fleetwatch/fleetwatch/internal/history"
)

type RestoreReceipt struct {
	SchemaVersion       int              `json:"schema_version"`
	Archive             ArchiveReceipt   `json:"archive"`
	Baseline            history.Baseline `json:"baseline"`
	StateSHA256         string           `json:"state_sha256"`
	OperationalMetadata string           `json:"operational_metadata"`
}

// RestoreArchive rebuilds a NEW follow DB by replay, keeping original baseline
// provenance and finding IDs. It never rotates the assertion or trims evidence.
// The expected compressed-file digest must come from the operator's handoff.
func RestoreArchive(ctx context.Context, input io.Reader, destination, expectedSHA256 string) (RestoreReceipt, error) {
	return restoreArchive(ctx, input, destination, expectedSHA256, nil)
}

func restoreArchive(ctx context.Context, input io.Reader, destination, expectedSHA256 string, hook func() error) (RestoreReceipt, error) {
	var result RestoreReceipt
	digest, err := hex.DecodeString(expectedSHA256)
	if err != nil || len(digest) != 32 || expectedSHA256 != strings.ToLower(expectedSHA256) || destination == "" {
		return result, errors.New("new destination and canonical expected SHA256 required")
	}
	if ctx.Err() != nil {
		return result, ctx.Err()
	}
	if _, err := os.Lstat(destination); !os.IsNotExist(err) {
		return result, errors.New("restore destination exists or is unavailable")
	}
	parent := filepath.Dir(destination)
	stage, err := os.MkdirTemp(parent, ".fleetwatch-restore-*")
	if err != nil {
		return result, err
	}
	defer os.RemoveAll(stage)
	path := filepath.Join(stage, "follow.db")
	limits := Limits{MaxLedgers: 100000, MaxPayloadBytes: 256 << 20}
	var state *Store
	var baseline history.Baseline
	defer func() {
		if state != nil {
			state.Close()
		}
	}()
	receipt, err := replayArchive(ctx, input, MaxArchiveBytes, MaxArchiveBytes, func(b history.Baseline) error {
		baseline = b
		state, err = Open(ctx, path, b, limits)
		return err
	}, func(r Record) error {
		_, err := state.Commit(ctx, evidenceCapture(baseline.Snapshot.Network, r.Evidence))
		return err
	})
	if err != nil {
		return result, err
	}
	if receipt.ArchiveSHA256 != expectedSHA256 {
		return result, errors.New("archive does not match expected SHA256")
	}
	if err := state.Close(); err != nil {
		state = nil
		return result, err
	}
	state = nil
	// Reopen from disk, replaying and checking the complete journal/head rather
	// than treating successful writes or the archive trailer as recovery proof.
	state, err = OpenExisting(ctx, path, baseline, limits)
	if err != nil {
		return result, err
	}
	head := state.Head()
	if head.Projection.OriginDigest != receipt.BaselineDigest || head.Projection.OriginHeaderHash != receipt.BaselineHeaderHash || head.Projection.Snapshot.Ledger != receipt.ProcessedThrough || head.RetainedLedgers != receipt.RetainedLedgers || head.PendingFindings != receipt.ReconstructedFindings {
		return result, ErrState
	}
	if err := state.Close(); err != nil {
		state = nil
		return result, err
	}
	state = nil
	f, err := os.OpenFile(path, os.O_RDWR, 0)
	if err != nil {
		return result, err
	}
	defer f.Close()
	info, err := f.Stat()
	if err != nil || info.Size() > MaxArchiveBytes {
		return result, errors.New("restored database exceeds physical byte limit")
	}
	hash := sha256.New()
	if _, err := io.Copy(hash, archiveReader{ctx, f}); err != nil {
		return result, err
	}
	if err := f.Sync(); err != nil {
		return result, err
	}
	if err := f.Close(); err != nil {
		return result, err
	}
	if hook != nil {
		if err := hook(); err != nil {
			return result, err
		}
	}
	if ctx.Err() != nil {
		return result, ctx.Err()
	}
	if err := os.Link(path, destination); err != nil {
		return result, err
	}
	// Never remove a published file if directory durability cannot be confirmed.
	dir, err := os.Open(parent)
	if err != nil {
		return result, err
	}
	err = dir.Sync()
	closeErr := dir.Close()
	if err != nil {
		return result, err
	}
	if closeErr != nil {
		return result, closeErr
	}
	return RestoreReceipt{SchemaVersion: 1, Archive: receipt, Baseline: baseline, StateSHA256: hex.EncodeToString(hash.Sum(nil)), OperationalMetadata: "RESET_NOT_ARCHIVED"}, nil
}
