package main

import (
	"bytes"
	"encoding/json"
	"os"
	"path/filepath"
	"testing"

	"github.com/soroban-fleetwatch/fleetwatch/internal/config"
	"github.com/soroban-fleetwatch/fleetwatch/internal/history"
	"github.com/soroban-fleetwatch/fleetwatch/internal/monitor"
)

func watchConfig(t *testing.T) (string, config.Watch) {
	t.Helper()
	dir := t.TempDir()
	raw, err := os.ReadFile(fixture("reconstruction-baseline.json"))
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, "baseline.json"), raw, 0600); err != nil {
		t.Fatal(err)
	}
	w := config.Watch{SchemaVersion: 1, RPCURL: "https://rpc.example.com", Baseline: "baseline.json", State: "state.db", Stream: "watch", ToLedger: 100, PollIntervalSeconds: 1, MaxAttempts: 1, MaxDurationSeconds: 60}
	path := filepath.Join(dir, "watch.json")
	writeWatchConfig(t, path, w)
	return path, w
}
func writeWatchConfig(t *testing.T, path string, w config.Watch) {
	t.Helper()
	raw, _ := json.Marshal(w)
	if err := os.WriteFile(path, raw, 0600); err != nil {
		t.Fatal(err)
	}
}
func TestCLIWatchCommitOutputFailureAndColdRestart(t *testing.T) {
	path, _ := watchConfig(t)
	r := &ingestionRPC{}
	factory := func(string) (history.RPC, error) { return r, nil }
	var out, errs bytes.Buffer
	if code := watchWithRPC([]string{"--config", path}, reportFailWriter{}, &errs, factory); code != 5 {
		t.Fatal(code, errs.String())
	}
	if code := watchWithRPC([]string{"--config", path}, &out, &errs, factory); code != 0 {
		t.Fatal(code, errs.String())
	}
	var e monitor.Event
	if err := json.Unmarshal(out.Bytes(), &e); err != nil || e.Status != "ALREADY_RETAINED" || e.SourceQuery != "NOT_QUERIED" || e.ObservedEvents != nil || r.calls != 1 {
		t.Fatal(e, err, r.calls)
	}
	state := filepath.Join(filepath.Dir(path), "state.db")
	out.Reset()
	if code := run([]string{"doctor", "--state", state, "--network", "SYNTHETIC-NETWORK", "--source", "rpc_ledger_meta", "--stream", "watch"}, &out, &errs); code != 0 {
		t.Fatal(code, errs.String())
	}
}
func TestCLIWatchCompleteAndAttemptLimit(t *testing.T) {
	for _, fail := range []bool{false, true} {
		t.Run(map[bool]string{false: "complete", true: "limit"}[fail], func(t *testing.T) {
			path, _ := watchConfig(t)
			r := &ingestionRPC{fail: fail}
			var out, errs bytes.Buffer
			code := watchWithRPC([]string{"--config", path}, &out, &errs, func(string) (history.RPC, error) { return r, nil })
			want := 2
			if fail {
				want = 4
			}
			if code != want {
				t.Fatal(code, errs.String())
			}
			var e monitor.Event
			if err := json.Unmarshal(out.Bytes(), &e); err != nil {
				t.Fatal(err)
			}
			if fail && (e.Code != "ATTEMPT_LIMIT" || e.Status != "STOPPED" || e.PartialAttempts != 1) {
				t.Fatal(e)
			}
			if !fail && (e.Status != "COMPLETE" || e.ObservedEvents == nil || *e.ObservedEvents != 2) {
				t.Fatal(e)
			}
		})
	}
}
func TestCLIWatchInvalidConfigDoesNotCreateState(t *testing.T) {
	path, w := watchConfig(t)
	state := filepath.Join(filepath.Dir(path), "state.db")
	var out, errs bytes.Buffer
	for _, args := range [][]string{{"watch"}, {"watch", "--config", path, "extra"}, {"watch", "--config", filepath.Join(t.TempDir(), "missing.json")}} {
		if code := run(args, &out, &errs); code != 3 {
			t.Fatal(args, code)
		}
	}
	w.ToLedger = 116
	writeWatchConfig(t, path, w)
	if code := run([]string{"watch", "--config", path}, &out, &errs); code != 3 {
		t.Fatal(code)
	}
	if _, err := os.Stat(state); !os.IsNotExist(err) {
		t.Fatal("invalid config created state", err)
	}
}
