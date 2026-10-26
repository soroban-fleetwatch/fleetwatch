package source

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	protocol "github.com/stellar/go-stellar-sdk/protocols/rpc"
)

func TestHTTPRPCGetLedgersWire(t *testing.T) {
	server := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var req struct {
			ID     uint64                     `json:"id"`
			Method string                     `json:"method"`
			Params protocol.GetLedgersRequest `json:"params"`
		}
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			t.Error(err)
			return
		}
		if req.Method != "getLedgers" || req.Params.StartLedger != 100 || req.Params.Pagination == nil || req.Params.Pagination.Limit != 1 || req.Params.Pagination.Cursor != "" {
			t.Errorf("wrong wire request: %+v", req)
		}
		_ = json.NewEncoder(w).Encode(map[string]any{"jsonrpc": "2.0", "id": req.ID, "result": protocol.GetLedgersResponse{OldestLedger: 100, LatestLedger: 101, Ledgers: []protocol.LedgerInfo{{Sequence: 100, LedgerCloseTime: 1700000000, Hash: "synthetic", LedgerHeader: "header", LedgerMetadata: "metadata"}}}})
	}))
	defer server.Close()
	rpc, err := NewHTTPRPC(server.URL)
	if err != nil {
		t.Fatal(err)
	}
	rpc.client = server.Client()
	r, err := rpc.Ledgers(context.Background(), protocol.GetLedgersRequest{StartLedger: 100, Pagination: &protocol.LedgerPaginationOptions{Limit: 1}})
	if err != nil || len(r.Ledgers) != 1 || r.Ledgers[0].LedgerCloseTime != 1700000000 {
		t.Fatal(r, err)
	}
}
