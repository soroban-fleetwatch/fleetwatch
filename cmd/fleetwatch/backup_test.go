package main

import (
	"bytes"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"os"
	"path/filepath"
	"testing"

	"github.com/soroban-fleetwatch/fleetwatch/internal/store"
)

func TestCLIBackupRestoreAndOutputFailure(t *testing.T) {
	dir := t.TempDir()
	source := filepath.Join(dir, "source.db")
	destination := filepath.Join(dir, "backup.db")
	var out, errs bytes.Buffer
	if code := run([]string{"replay", "--journal", fixture("replay-transient.json"), "--state", source}, &out, &errs); code != 2 {
		t.Fatal(code, errs.String())
	}
	out.Reset()
	errs.Reset()
	if code := run([]string{"backup", "--state", source, "--output", destination}, &out, &errs); code != 0 {
		t.Fatal(code, errs.String())
	}
	var receipt store.BackupReceipt
	if err := json.Unmarshal(out.Bytes(), &receipt); err != nil {
		t.Fatal(err)
	}
	raw, err := os.ReadFile(destination)
	if err != nil {
		t.Fatal(err)
	}
	hash := sha256.Sum256(raw)
	if receipt.SHA256 != hex.EncodeToString(hash[:]) || receipt.Bytes != int64(len(raw)) {
		t.Fatalf("%+v", receipt)
	}
	out.Reset()
	if code := run([]string{"doctor", "--state", destination, "--network", "TESTNET-SYNTHETIC"}, &out, &errs); code != 0 {
		t.Fatal(code, errs.String())
	}
	before := append([]byte{}, raw...)
	out.Reset()
	if code := run([]string{"backup", "--state", source, "--output", destination}, &out, &errs); code != 5 || out.Len() != 0 {
		t.Fatal("overwrite not rejected", code)
	}
	raw, err = os.ReadFile(destination)
	if err != nil || !bytes.Equal(raw, before) {
		t.Fatal("destination changed", err)
	}
	outputFailure := filepath.Join(dir, "receipt-failed.db")
	if code := run([]string{"backup", "--state", source, "--output", outputFailure}, reportFailWriter{}, &errs); code != 5 {
		t.Fatal(code)
	}
	if _, err := os.Stat(outputFailure); err != nil {
		t.Fatal("lost saved backup after output failure", err)
	}
	for _, args := range [][]string{{"backup"}, {"backup", "--state", source}, {"backup", "--state", source, "--output", destination, "extra"}} {
		out.Reset()
		if code := run(args, &out, &errs); code != 3 || out.Len() != 0 {
			t.Fatal(code)
		}
	}
	missing := filepath.Join(dir, "missing.db")
	out.Reset()
	if code := run([]string{"backup", "--state", missing, "--output", filepath.Join(dir, "unused.db")}, &out, &errs); code != 5 || out.Len() != 0 {
		t.Fatal(code)
	}
	if _, err := os.Stat(missing); !os.IsNotExist(err) {
		t.Fatal("backup created source database")
	}
}
