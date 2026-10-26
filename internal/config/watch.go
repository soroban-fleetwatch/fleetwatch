// Package config loads bounded nonsecret operator configuration.
package config

import (
	"bytes"
	"errors"
	"io"
	"os"
	"path/filepath"
	"strings"
	"time"
	"unicode"
	"unicode/utf8"

	"github.com/soroban-fleetwatch/fleetwatch/internal/jsoninput"
	"github.com/soroban-fleetwatch/fleetwatch/internal/monitor"
	"github.com/soroban-fleetwatch/fleetwatch/internal/source"
)

const MaxWatchConfigBytes = 16 << 10

type Watch struct {
	SchemaVersion       int    `json:"schema_version"`
	RPCURL              string `json:"rpc_url"`
	Baseline            string `json:"baseline"`
	State               string `json:"state"`
	Stream              string `json:"stream"`
	ToLedger            uint32 `json:"to_ledger"`
	PollIntervalSeconds int    `json:"poll_interval_seconds"`
	MaxAttempts         int    `json:"max_attempts"`
	MaxDurationSeconds  int    `json:"max_duration_seconds"`
}

func validPath(s string) bool {
	if strings.TrimSpace(s) == "" || len(s) > 4096 || !utf8.ValidString(s) {
		return false
	}
	for _, c := range s {
		if unicode.IsControl(c) {
			return false
		}
	}
	return true
}
func (w Watch) Options() monitor.Options {
	return monitor.Options{PollInterval: time.Duration(w.PollIntervalSeconds) * time.Second, MaxAttempts: w.MaxAttempts, MaxDuration: time.Duration(w.MaxDurationSeconds) * time.Second}
}
func LoadWatch(path string) (Watch, error) {
	fail := func() (Watch, error) { return Watch{}, errors.New("invalid bounded watch configuration") }
	f, err := os.Open(path)
	if err != nil {
		return fail()
	}
	defer f.Close()
	raw, err := io.ReadAll(io.LimitReader(f, MaxWatchConfigBytes+1))
	if err != nil || len(raw) > MaxWatchConfigBytes {
		return fail()
	}
	var w Watch
	if jsoninput.Decode(bytes.NewReader(raw), &w) != nil || w.SchemaVersion != 1 || !validPath(w.Baseline) || !validPath(w.State) || w.ToLedger == 0 || w.PollIntervalSeconds < 1 || w.PollIntervalSeconds > 60 || w.MaxDurationSeconds < 1 || w.MaxDurationSeconds > 3600 || w.Options().Validate() != nil {
		return fail()
	}
	if _, err := source.NewHTTPRPC(w.RPCURL); err != nil {
		return fail()
	}
	if len(w.Stream) < 1 || len(w.Stream) > 128 {
		return fail()
	}
	for _, c := range w.Stream {
		if !(c >= 'a' && c <= 'z' || c >= 'A' && c <= 'Z' || c >= '0' && c <= '9' || c == '.' || c == '_' || c == '-') {
			return fail()
		}
	}
	configPath, err := filepath.Abs(path)
	if err != nil {
		return fail()
	}
	dir := filepath.Dir(configPath)
	if !filepath.IsAbs(w.Baseline) {
		w.Baseline = filepath.Join(dir, w.Baseline)
	}
	if !filepath.IsAbs(w.State) {
		w.State = filepath.Join(dir, w.State)
	}
	w.Baseline = filepath.Clean(w.Baseline)
	w.State = filepath.Clean(w.State)
	if w.State == w.Baseline || w.State == configPath {
		return fail()
	}
	return w, nil
}
