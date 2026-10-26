package main

import (
	"bytes"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/soroban-fleetwatch/fleetwatch/internal/history"
)

func TestCLIChangesSyntheticCaptureAndFailures(t *testing.T) {
	args := []string{"changes", "--capture", fixture("history-changes-capture.json")}
	var out, errs bytes.Buffer
	if code := run(args, &out, &errs); code != 0 {
		t.Fatal(code, errs.String())
	}
	var r history.ChangeReport
	if err := json.Unmarshal(out.Bytes(), &r); err != nil || len(r.Ledgers) != 1 || len(r.Ledgers[0].Changes) != 11 || len(r.Ledgers[0].UnorderedEvictions) != 1 || r.FinalVerdict != "UNDETERMINED" || r.StateChanges != "NOT_APPLIED" {
		t.Fatal(r, err)
	}
	path := filepath.Join(t.TempDir(), "normalized.json")
	if err := os.WriteFile(path, out.Bytes(), 0600); err != nil {
		t.Fatal(err)
	}
	out.Reset()
	if code := run([]string{"replay", "--journal", path}, &out, &errs); code != 3 {
		t.Fatal("normalized changes accepted as snapshot journal", code)
	}
	if code := run(args, reportFailWriter{}, &errs); code != 5 {
		t.Fatal(code)
	}
	for _, args := range [][]string{{"changes"}, {"changes", "--capture", "missing.json"}, {"changes", "--capture", fixture("before.json")}, {"changes", "--capture", fixture("history-changes-capture.json"), "extra"}} {
		out.Reset()
		if code := run(args, &out, &errs); code != 3 || out.Len() != 0 {
			t.Fatal(code, out.String())
		}
	}
	raw, err := os.ReadFile(fixture("history-changes-capture.json"))
	if err != nil {
		t.Fatal(err)
	}
	var c history.Capture
	if err := json.Unmarshal(raw, &c); err != nil {
		t.Fatal(err)
	}
	c.Ledgers[0].MetadataSHA256 = strings.Repeat("0", 64)
	encoded, err := json.Marshal(c)
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, encoded, 0600); err != nil {
		t.Fatal(err)
	}
	out.Reset()
	if code := run([]string{"changes", "--capture", path}, &out, &errs); code != 4 || !strings.Contains(out.String(), "INVALID_LEDGER_EVIDENCE") {
		t.Fatal(code, out.String())
	}
	c.SourceAuthentication = "VERIFIED"
	encoded, err = json.Marshal(c)
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, encoded, 0600); err != nil {
		t.Fatal(err)
	}
	out.Reset()
	if code := run([]string{"changes", "--capture", path}, &out, &errs); code != 3 || out.Len() != 0 {
		t.Fatal("accepted inflated assurance", code)
	}
}
