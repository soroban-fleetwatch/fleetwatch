package source

import (
	"bytes"
	"context"
	"encoding/hex"
	"encoding/json"
	"errors"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"sync/atomic"
	"time"

	protocol "github.com/stellar/go-stellar-sdk/protocols/rpc"
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

type transientError struct{ delay time.Duration }

func (e *transientError) Error() string { return "RPC temporarily unavailable" }

func (r *HTTPRPC) call(ctx context.Context, method string, params any, target any) error {
	for attempt := 0; attempt < 3; attempt++ {
		if err := ctx.Err(); err != nil {
			return err
		}
		err := r.callOnce(ctx, method, params, target)
		var transient *transientError
		if !errors.As(err, &transient) || attempt == 2 {
			return err
		}
		delay := time.Duration(100*(1<<attempt)) * time.Millisecond
		if transient.delay > delay {
			delay = transient.delay
		}
		timer := time.NewTimer(delay)
		select {
		case <-ctx.Done():
			timer.Stop()
			return ctx.Err()
		case <-timer.C:
		}
	}
	return errors.New("RPC retry limit reached")
}
func (r *HTTPRPC) callOnce(ctx context.Context, method string, params any, target any) error {
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
	if response.StatusCode == 429 || response.StatusCode == 502 || response.StatusCode == 503 || response.StatusCode == 504 {
		delay := time.Duration(0)
		if retry := response.Header.Get("Retry-After"); retry != "" {
			seconds, err := strconv.Atoi(retry)
			if err == nil {
				if seconds < 0 || seconds > 2 {
					return errors.New("RPC retry interval outside local budget")
				}
				delay = time.Duration(seconds) * time.Second
			} else if at, err := http.ParseTime(retry); err == nil {
				delay = time.Until(at)
			} else {
				return errors.New("invalid RPC retry interval")
			}
			if delay > 2*time.Second || delay < 0 {
				return errors.New("RPC retry interval outside local budget")
			}
		}
		return &transientError{delay: delay}
	}
	if response.StatusCode != http.StatusOK {
		return errors.New("RPC returned non-success HTTP status")
	}
	return decodeRPCResponse(response.Body, id, target)
}
func (r *HTTPRPC) Network(ctx context.Context) (Network, error) {
	var n Network
	err := r.call(ctx, "getNetwork", struct{}{}, &n)
	if err != nil {
		return Network{}, err
	}
	return n, nil
}
func (r *HTTPRPC) Entries(ctx context.Context, keys []string) (Entries, error) {
	var e Entries
	err := r.call(ctx, "getLedgerEntries", struct {
		Keys []string `json:"keys"`
	}{keys}, &e)
	if err != nil {
		return Entries{}, err
	}
	return e, nil
}

func (r *HTTPRPC) Head(ctx context.Context) (Head, error) {
	var h Head
	err := r.call(ctx, "getLatestLedger", struct{}{}, &h)
	if err != nil {
		return Head{}, err
	}
	return h, nil
}

func (r *HTTPRPC) Ledgers(ctx context.Context, request protocol.GetLedgersRequest) (protocol.GetLedgersResponse, error) {
	var result protocol.GetLedgersResponse
	err := r.call(ctx, protocol.GetLedgersMethodName, request, &result)
	if err != nil {
		return protocol.GetLedgersResponse{}, err
	}
	return result, nil
}

// Transaction retrieves an endpoint claim for a canonical transaction hash.
// Callers must separately validate its envelope/result and any chain inclusion.
func (r *HTTPRPC) Transaction(ctx context.Context, hash string) (protocol.GetTransactionResponse, error) {
	raw, err := hex.DecodeString(hash)
	if err != nil || len(raw) != 32 || hash != strings.ToLower(hash) {
		return protocol.GetTransactionResponse{}, errors.New("invalid transaction hash")
	}
	var result protocol.GetTransactionResponse
	if err := r.call(ctx, protocol.GetTransactionMethodName, protocol.GetTransactionRequest{Hash: hash}, &result); err != nil {
		return protocol.GetTransactionResponse{}, err
	}
	return result, nil
}
