package config

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func watchJSON() string {
	return `{"schema_version":1,"rpc_url":"https://rpc.example.com","baseline":"baseline.json","state":"state.db","stream":"watch","to_ledger":101,"poll_interval_seconds":1,"max_attempts":3,"max_duration_seconds":60}`
}
func TestWatchConfigResolvesPathsAndStrictShape(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "watch.json")
	if err := os.WriteFile(path, []byte(watchJSON()), 0600); err != nil {
		t.Fatal(err)
	}
	w, err := LoadWatch(path)
	if err != nil || w.Baseline != filepath.Join(dir, "baseline.json") || w.State != filepath.Join(dir, "state.db") || w.Options().Validate() != nil {
		t.Fatal(w, err)
	}
	for _, raw := range []string{`null`, watchJSON() + `{}`, strings.Replace(watchJSON(), `"schema_version":1`, `"schema_version":1,"schema_version":1`, 1), strings.Replace(watchJSON(), `"schema_version":1`, `"schema_version":1,"SCHEMA_VERSION":1`, 1), strings.Replace(watchJSON(), `"schema_version":1`, `"schema_version":1,"token":"forbidden"`, 1), strings.Repeat(" ", MaxWatchConfigBytes+1)} {
		if err := os.WriteFile(path, []byte(raw), 0600); err != nil {
			t.Fatal(err)
		}
		if _, err := LoadWatch(path); err == nil {
			t.Fatal("invalid config accepted")
		}
	}
}
func TestWatchConfigRejectsInvalidValuesAndCollidingPaths(t *testing.T) {
	var original map[string]any
	if err := json.Unmarshal([]byte(watchJSON()), &original); err != nil {
		t.Fatal(err)
	}
	for _, bad := range []struct {
		key   string
		value any
	}{{"schema_version", 2}, {"rpc_url", "http://rpc.example.com"}, {"rpc_url", "https://token@rpc.example.com"}, {"rpc_url", "https://rpc.example.com?secret=x"}, {"state", "baseline.json"}, {"state", "watch.json"}, {"baseline", " "}, {"state", "bad\npath"}, {"stream", "../watch"}, {"to_ledger", 0}, {"to_ledger", 4294967296}, {"poll_interval_seconds", 0}, {"poll_interval_seconds", 61}, {"max_attempts", 121}, {"max_attempts", -1}, {"max_duration_seconds", 0}, {"max_duration_seconds", 3601}, {"max_duration_seconds", 9223372036854775807}, {"max_attempts", nil}} {
		values := map[string]any{}
		for k, v := range original {
			values[k] = v
		}
		values[bad.key] = bad.value
		raw, _ := json.Marshal(values)
		path := filepath.Join(t.TempDir(), "watch.json")
		if err := os.WriteFile(path, raw, 0600); err != nil {
			t.Fatal(err)
		}
		if _, err := LoadWatch(path); err == nil {
			t.Fatal(bad.key, bad.value)
		}
	}
}
