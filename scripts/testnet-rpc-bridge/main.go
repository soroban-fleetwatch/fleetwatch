// Developer-only loopback adapter for Stellar CLI's non-proxy-aware RPC client.
// All outbound traffic still uses the managed proxy and ordinary TLS checks.
package main

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/soroban-fleetwatch/fleetwatch/internal/jsoninput"
)

const upstream = "https://soroban-testnet.stellar.org"
const maxBytes = 25 << 20

var methods = map[string]bool{
	"getNetwork": true, "getHealth": true, "getVersionInfo": true,
	"getLatestLedger": true, "getLedgerEntries": true, "getTransaction": true,
	"getFeeStats": true, "simulateTransaction": true, "sendTransaction": true,
}

func handler(client *http.Client) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost || r.URL.Path != "/" || r.URL.RawQuery != "" || r.Header.Get("Origin") != "" {
			http.Error(w, "unsupported bridge request", http.StatusForbidden)
			return
		}
		body, err := io.ReadAll(io.LimitReader(r.Body, maxBytes+1))
		if err != nil || len(body) > maxBytes {
			http.Error(w, "invalid bridge body", http.StatusRequestEntityTooLarge)
			return
		}
		var request struct {
			JSONRPC string          `json:"jsonrpc"`
			Method  string          `json:"method"`
			ID      json.RawMessage `json:"id"`
			Params  json.RawMessage `json:"params"`
		}
		if jsoninput.Decode(bytes.NewReader(body), &request) != nil || request.JSONRPC != "2.0" || !methods[request.Method] || len(request.ID) == 0 || string(request.ID) == "null" {
			http.Error(w, "unsupported bridge RPC", http.StatusBadRequest)
			return
		}
		req, err := http.NewRequestWithContext(r.Context(), http.MethodPost, upstream, bytes.NewReader(body))
		if err != nil {
			http.Error(w, "bridge request failed", http.StatusBadGateway)
			return
		}
		req.Header.Set("Content-Type", "application/json")
		response, err := client.Do(req)
		if err != nil {
			http.Error(w, "Testnet transport failed", http.StatusBadGateway)
			return
		}
		defer response.Body.Close()
		result, err := io.ReadAll(io.LimitReader(response.Body, maxBytes+1))
		if err != nil || len(result) > maxBytes || response.StatusCode != http.StatusOK {
			http.Error(w, "Testnet response unavailable", http.StatusBadGateway)
			return
		}
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write(result)
	})
}

func main() {
	if len(os.Args) != 1 {
		fmt.Fprintln(os.Stderr, "testnet-rpc-bridge takes no arguments; destination is fixed to Testnet")
		os.Exit(3)
	}
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()
	ctx, cancel := context.WithTimeout(ctx, 15*time.Minute)
	defer cancel()
	listener, err := net.Listen("tcp4", "127.0.0.1:0")
	if err != nil {
		fmt.Fprintln(os.Stderr, "cannot bind Testnet bridge")
		os.Exit(5)
	}
	client := &http.Client{Timeout: 30 * time.Second, CheckRedirect: func(*http.Request, []*http.Request) error { return http.ErrUseLastResponse }}
	server := &http.Server{Handler: handler(client), ReadHeaderTimeout: 5 * time.Second, ReadTimeout: 30 * time.Second, WriteTimeout: 45 * time.Second, MaxHeaderBytes: 8192}
	go func() {
		<-ctx.Done()
		_ = server.Close()
	}()
	fmt.Printf("http://%s\n", listener.Addr())
	if err := server.Serve(listener); err != nil && err != http.ErrServerClosed {
		fmt.Fprintln(os.Stderr, "Testnet bridge stopped unexpectedly")
		os.Exit(5)
	}
}
