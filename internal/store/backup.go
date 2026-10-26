package store

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"io"
	"os"
	"path/filepath"

	bolt "go.etcd.io/bbolt"
)

const MaxBackupBytes int64 = 512 << 20

type BackupReceipt struct {
	SchemaVersion      int    `json:"schema_version"`
	Bytes              int64  `json:"bytes"`
	SHA256             string `json:"sha256"`
	StorageFormat      string `json:"storage_format"`
	SemanticValidation string `json:"semantic_validation"`
}

// Backup publishes a transaction-consistent copy of all buckets to a NEW path.
// The containing directory must already exist on a local filesystem supporting
// hard links and fsync. It does not semantically validate application records.
func (s *Store) Backup(ctx context.Context, destination string) (BackupReceipt, error) {
	return s.backup(ctx, destination, MaxBackupBytes, nil)
}

// hook permits pre-publication fault injection in tests only.
func (s *Store) backup(ctx context.Context, destination string, limit int64, hook func() error) (BackupReceipt, error) {
	var result BackupReceipt
	if err := ctx.Err(); err != nil {
		return result, err
	}
	if destination == "" || limit <= 0 {
		return result, errors.New("invalid backup destination or limit")
	}
	if _, err := os.Lstat(destination); err == nil {
		return result, errors.New("backup destination already exists")
	} else if !os.IsNotExist(err) {
		return result, err
	}
	parent := filepath.Dir(destination)
	f, err := os.CreateTemp(parent, ".fleetwatch-backup-*")
	if err != nil {
		return result, err
	}
	temporary := f.Name()
	defer os.Remove(temporary)
	defer f.Close()
	hash := sha256.New()
	w := &backupWriter{ctx: ctx, writer: io.MultiWriter(f, hash), remaining: limit}
	var size int64
	err = s.db.View(func(tx *bolt.Tx) error {
		if tx.Size() > limit {
			return errors.New("backup exceeds local byte limit")
		}
		var err error
		size, err = tx.WriteTo(w)
		return err
	})
	if err != nil {
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
	if err := ctx.Err(); err != nil {
		return result, err
	}
	// Link atomically exposes the complete fsynced file without replacing any
	// existing path, including a symlink introduced after the initial check.
	if err := os.Link(temporary, destination); err != nil {
		return result, err
	}
	// After publication, failures must not delete the destination: the operator
	// may already have observed it. Report uncertainty and allow inspection.
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
	return BackupReceipt{SchemaVersion: 1, Bytes: size, SHA256: hex.EncodeToString(hash.Sum(nil)),
		StorageFormat: "BBOLT_TRANSACTION_SNAPSHOT", SemanticValidation: "NOT_PERFORMED"}, nil
}

type backupWriter struct {
	ctx       context.Context
	writer    io.Writer
	remaining int64
}

func (w *backupWriter) Write(p []byte) (int, error) {
	if err := w.ctx.Err(); err != nil {
		return 0, err
	}
	if int64(len(p)) > w.remaining {
		return 0, errors.New("backup byte limit exceeded")
	}
	n, err := w.writer.Write(p)
	w.remaining -= int64(n)
	return n, err
}
