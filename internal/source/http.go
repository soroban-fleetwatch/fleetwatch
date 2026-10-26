package source

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"net/url"
	"sync/atomic"
	"time"
)

// HTTPRPC uses HTTPS with ordinary certificate verification, bounded replies,
// per-request cancellation and no cross-host redirects or URL credentials.
type HTTPRPC struct {
	endpoint string
	client   *http.Client
	next     atomic.Uint64
}

func NewHTTPRPC(endpoint string) (*HTTPRPC, error) {
	u, err := url.Parse(endpoint)
	if err != nil || u.Scheme != "https" || u.Hostname() == "" || u.User != nil || u.RawQuery != "" || u.Fragment != "" {
		return nil, errors.New("RPC endpoint must be HTTPS without credentials, query, or fragment")
	}
	return &HTTPRPC{endpoint: endpoint, client: &http.Client{Timeout: 20 * time.Second, CheckRedirect: func(*http.Request, []*http.Request) error { return errors.New("RPC redirects are disabled") }}}, nil
}
func (r *HTTPRPC) call(ctx context.Context, method string, params any, target any) error {
	id := r.next.Add(1)
	body, err := json.Marshal(struct {
		JSONRPC string `json:"jsonrpc"`
		ID      uint64 `json:"id"`
		Method  string `json:"method"`
		Params  any    `json:"params"`
	}{"2.0", id, method, params})
	if err != nil {
		return err
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, r.endpoint, bytes.NewReader(body))
	if err != nil {
		return errors.New("invalid RPC request")
	}
	req.Header.Set("Content-Type", "application/json")
	response, err := r.client.Do(req)
	if err != nil {
		return errors.New("RPC transport failed or timed out")
	}
	defer response.Body.Close()
	if response.StatusCode != http.StatusOK {
		return errors.New("RPC returned non-success HTTP status")
	}
	data, err := io.ReadAll(io.LimitReader(response.Body, (25<<20)+1))
	if err != nil {
		return errors.New("RPC response read failed")
	}
	if len(data) > 25<<20 {
		return errors.New("RPC response exceeds local size limit")
	}
	var envelope struct {
		JSONRPC string          `json:"jsonrpc"`
		ID      uint64          `json:"id"`
		Result  json.RawMessage `json:"result"`
		Error   json.RawMessage `json:"error"`
	}
	if err := json.Unmarshal(data, &envelope); err != nil {
		return errors.New("invalid RPC JSON response")
	}
	if envelope.JSONRPC != "2.0" || envelope.ID != id || (len(envelope.Error) > 0 && string(envelope.Error) != "null") || len(envelope.Result) == 0 || string(envelope.Result) == "null" {
		return errors.New("RPC error or mismatched response envelope")
	}
	if err := json.Unmarshal(envelope.Result, target); err != nil {
		return errors.New("invalid RPC result")
	}
	return nil
}
func (r *HTTPRPC) Network(ctx context.Context) (Network, error) {
	var n Network
	err := r.call(ctx, "getNetwork", struct{}{}, &n)
	return n, err
}
func (r *HTTPRPC) Entries(ctx context.Context, keys []string) (Entries, error) {
	var e Entries
	err := r.call(ctx, "getLedgerEntries", struct {
		Keys []string `json:"keys"`
	}{keys}, &e)
	return e, err
}
