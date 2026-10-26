package source

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"sync/atomic"
	"testing"
	"time"

	"github.com/stellar/go-stellar-sdk/xdr"
)

func TestReadOnlyRPCRetries(t *testing.T) {
	for _, tc := range []struct {
		name    string
		status  int
		retry   string
		recover bool
		want    int
	}{{"recover", 503, "", true, 2}, {"bounded", 429, "", false, 3}, {"denied", 403, "", false, 1}, {"long-delay", 503, "60", false, 1}} {
		t.Run(tc.name, func(t *testing.T) {
			var calls atomic.Int32
			s := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				n := calls.Add(1)
				if !tc.recover || n == 1 {
					if tc.retry != "" {
						w.Header().Set("Retry-After", tc.retry)
					}
					w.WriteHeader(tc.status)
					return
				}
				var request struct {
					ID uint64 `json:"id"`
				}
				_ = json.NewDecoder(r.Body).Decode(&request)
				_ = json.NewEncoder(w).Encode(map[string]any{"jsonrpc": "2.0", "id": request.ID, "result": Network{Passphrase: "test", ProtocolVersion: 28}})
			}))
			defer s.Close()
			rpc, _ := NewHTTPRPC(s.URL)
			rpc.client = s.Client()
			_, err := rpc.Network(context.Background())
			if (err == nil) != tc.recover || int(calls.Load()) != tc.want {
				t.Fatalf("calls=%d err=%v", calls.Load(), err)
			}
		})
	}
}
func TestRetryWaitIsCancellable(t *testing.T) {
	var calls atomic.Int32
	s := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { calls.Add(1); w.WriteHeader(503) }))
	defer s.Close()
	rpc, _ := NewHTTPRPC(s.URL)
	rpc.client = s.Client()
	ctx, cancel := context.WithTimeout(context.Background(), 50*time.Millisecond)
	defer cancel()
	_, err := rpc.Network(ctx)
	if !errors.Is(err, context.DeadlineExceeded) || calls.Load() != 1 {
		t.Fatalf("calls=%d err=%v", calls.Load(), err)
	}
}

type headRPC struct {
	*mockRPC
	head Head
	err  error
}

func (r headRPC) Head(context.Context) (Head, error) { return r.head, r.err }
func TestCaptureRejectsStaleInvalidAndUnavailableHead(t *testing.T) {
	now := time.Now().Unix()
	for _, h := range []Head{{100, 28, now - 120}, {100, 28, now + 120}, {0, 28, now}, {100, 27, now}, {100, 29, now}, {100, 28, 0}} {
		m := mock()
		if _, err := CaptureCurrent(context.Background(), headRPC{mockRPC: m, head: h}, "test network", []string{address(1)}); err == nil || m.calls != 0 {
			t.Fatalf("head=%+v err=%v entry calls=%d", h, err, m.calls)
		}
	}
	m := mock()
	if _, err := CaptureCurrent(context.Background(), headRPC{mockRPC: m, err: errors.New("offline")}, "test network", []string{address(1)}); err == nil {
		t.Fatal("missing head accepted")
	}
}
func TestCaptureDetectsRegressingEntryHeight(t *testing.T) {
	e := instance(t, address(1), xdr.ContractExecutable{Type: xdr.ContractExecutableTypeContractExecutableStellarAsset})
	m := mock(Entries{Entries: []Entry{e}, LatestLedger: 99})
	c, err := CaptureCurrent(context.Background(), m, "test network", []string{address(1)})
	if err != nil || c.Completeness != "PARTIAL" || len(c.Errors) == 0 {
		t.Fatalf("capture=%+v err=%v", c, err)
	}
}
