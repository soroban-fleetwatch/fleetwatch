package follow

import (
	"encoding/json"
	"github.com/soroban-fleetwatch/fleetwatch/internal/history"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func TestFollowConfigurationAndLocalHealth(t *testing.T) {
	dir := t.TempDir()
	cfg := config()
	path := filepath.Join(dir, "follow.json")
	raw, _ := json.Marshal(cfg)
	if err := os.WriteFile(path, raw, 0600); err != nil {
		t.Fatal(err)
	}
	loaded, err := LoadConfig(path)
	if err != nil || loaded.State != filepath.Join(dir, "state.db") {
		t.Fatal(loaded, err)
	}
	for _, bad := range []string{
		strings.Replace(string(raw), `"head_delay_ledgers":0,`, "", 1),
		strings.Replace(string(raw), `"head_delay_ledgers":0`, `"head_delay_ledgers":null`, 1),
		strings.Replace(string(raw), `"schema_version":1`, `"schema_version":1,"schema_version":1`, 1),
		strings.Replace(string(raw), `127.0.0.1:0`, `0.0.0.0:9789`, 1),
		strings.Replace(string(raw), `"batch_size":8`, `"batch_size":17`, 1),
		strings.Replace(string(raw), `"state":"state.db"`, `"state":"baseline.json"`, 1),
	} {
		os.WriteFile(path, []byte(bad), 0600)
		if _, err := LoadConfig(path); err == nil {
			t.Fatal("unsafe/missing follow field accepted", bad)
		}
	}
	b, _ := chain(t)
	p, err := history.NewProjection(b)
	if err != nil {
		t.Fatal(err)
	}
	h := NewHealth(Head{Projection: p}, cfg)
	for _, route := range []string{"/readyz", "/status", "/livez", "/metrics"} {
		w := httptest.NewRecorder()
		h.Handler().ServeHTTP(w, httptest.NewRequest("GET", route, nil))
		if route == "/readyz" && w.Code != 503 {
			t.Fatal("startup ready")
		}
		if route != "/readyz" && w.Code != 200 {
			t.Fatal(route, w.Code)
		}
	}
	n := p.Snapshot.Ledger
	h.Update(Head{Projection: p}, Cycle{Code: "IDLE", At: time.Now().Unix(), SuccessfulAt: time.Now().Unix(), EndpointHead: &n})
	w := httptest.NewRecorder()
	h.Handler().ServeHTTP(w, httptest.NewRequest("GET", "/readyz", nil))
	if w.Code != 200 || !strings.Contains(w.Body.String(), `"final_verdict":"UNDETERMINED"`) {
		t.Fatal(w.Code, w.Body.String())
	}
	w = httptest.NewRecorder()
	h.Handler().ServeHTTP(w, httptest.NewRequest("POST", "/status", nil))
	if w.Code != 405 {
		t.Fatal("health accepted mutation")
	}
}
