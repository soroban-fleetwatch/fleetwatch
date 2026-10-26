package source

import (
	"context"
	"encoding/json"
	"github.com/stellar/go-stellar-sdk/xdr"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"
)

func TestHTTPRPCRequestsAndResponseGuards(t *testing.T) {
	for _, mode := range []string{"ok", "id", "version", "error", "null", "http", "malformed"} {
		t.Run(mode, func(t *testing.T) {
			server := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				if r.Method != "POST" || r.Header.Get("Content-Type") != "application/json" {
					t.Error("invalid request")
				}
				var req struct {
					ID     uint64 `json:"id"`
					Method string `json:"method"`
				}
				if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
					t.Error(err)
				}
				if req.Method != "getNetwork" {
					t.Error("wrong RPC method")
				}
				id := req.ID
				version := "2.0"
				var result any = Network{Passphrase: "test network", ProtocolVersion: 28}
				var rpcError any
				switch mode {
				case "id":
					id++
				case "version":
					version = "1.0"
				case "error":
					rpcError = map[string]any{"code": -1, "message": "secret should not escape"}
				case "null":
					result = nil
				case "http":
					w.WriteHeader(503)
					return
				case "malformed":
					_, _ = w.Write([]byte("not json"))
					return
				}
				_ = json.NewEncoder(w).Encode(map[string]any{"jsonrpc": version, "id": id, "result": result, "error": rpcError})
			}))
			defer server.Close()
			rpc, err := NewHTTPRPC(server.URL)
			if err != nil {
				t.Fatal(err)
			}
			rpc.client = server.Client()
			_, err = rpc.Network(context.Background())
			if (err == nil) != (mode == "ok") {
				t.Fatalf("mode=%s err=%v", mode, err)
			}
			if err != nil && strings.Contains(err.Error(), "secret") {
				t.Fatal("upstream error leaked")
			}
		})
	}
}
func TestRPCURLAndCancellation(t *testing.T) {
	for _, url := range []string{"http://example.com", "https://user:secret@example.com", "https://example.com?token=secret", "https://example.com#fragment", "nonsense"} {
		if _, err := NewHTTPRPC(url); err == nil {
			t.Fatal("unsafe endpoint accepted")
		}
	}
	server := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		select {
		case <-r.Context().Done():
		case <-time.After(100 * time.Millisecond):
		}
	}))
	defer server.Close()
	rpc, _ := NewHTTPRPC(server.URL)
	rpc.client = server.Client()
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Millisecond)
	defer cancel()
	if _, err := rpc.Network(ctx); err == nil {
		t.Fatal("request ignored timeout")
	}
}

func TestHTTPRPCCaptureFunctionalRequest(t *testing.T) {
	entry := instance(t, address(1), xdr.ContractExecutable{Type: xdr.ContractExecutableTypeContractExecutableStellarAsset})
	server := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var req struct {
			ID     uint64 `json:"id"`
			Method string `json:"method"`
			Params struct {
				Keys []string `json:"keys"`
			} `json:"params"`
		}
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			t.Error(err)
			return
		}
		var result any
		switch req.Method {
		case "getNetwork":
			result = Network{Passphrase: "test network", ProtocolVersion: 28}
		case "getLedgerEntries":
			if len(req.Params.Keys) != 1 || req.Params.Keys[0] != entry.Key {
				t.Error("wrong requested instance key")
			}
			result = Entries{Entries: []Entry{entry}, LatestLedger: 100}
		default:
			t.Error("unexpected method")
			w.WriteHeader(400)
			return
		}
		_ = json.NewEncoder(w).Encode(map[string]any{"jsonrpc": "2.0", "id": req.ID, "result": result})
	}))
	defer server.Close()
	rpc, _ := NewHTTPRPC(server.URL)
	rpc.client = server.Client()
	c, err := CaptureCurrent(context.Background(), rpc, "test network", []string{address(1)})
	if err != nil || len(c.Snapshot.Contracts) != 1 || c.Snapshot.Contracts[0].Executable.Kind != "stellar_asset" || len(c.Evidence) != 1 {
		t.Fatalf("capture=%+v err=%v", c, err)
	}
}
