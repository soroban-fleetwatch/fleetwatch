package source

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"reflect"
	"strings"
	"testing"

	protocol "github.com/stellar/go-stellar-sdk/protocols/rpc"
)

func TestTransactionLookupRejectsInvalidHashAndAmbiguousResponse(t *testing.T) {
	for _, duplicate := range []bool{false, true} {
		calls := 0
		server := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			calls++
			var req struct {
				ID     uint64                         `json:"id"`
				Method string                         `json:"method"`
				Params protocol.GetTransactionRequest `json:"params"`
			}
			_ = json.NewDecoder(r.Body).Decode(&req)
			if req.Method != "getTransaction" || req.Params.Hash != strings.Repeat("a", 64) {
				t.Error("incorrect transaction request")
			}
			status := `"status":"SUCCESS"`
			if duplicate {
				status += `,"status":"FAILED"`
			}
			fmt.Fprintf(w, `{"jsonrpc":"2.0","id":%d,"result":{%s,"ledger":123}}`, req.ID, status)
		}))
		rpc, _ := NewHTTPRPC(server.URL)
		rpc.client = server.Client()
		for _, invalid := range []string{"", "bad", strings.Repeat("A", 64), strings.Repeat("g", 64)} {
			if _, err := rpc.Transaction(context.Background(), invalid); err == nil || calls != 0 {
				t.Fatal("invalid transaction hash reached network")
			}
		}
		got, err := rpc.Transaction(context.Background(), strings.Repeat("a", 64))
		if duplicate {
			if err == nil || !reflect.DeepEqual(got, protocol.GetTransactionResponse{}) {
				t.Fatal("ambiguous receipt returned partial data")
			}
		} else if err != nil || got.Status != "SUCCESS" || got.Ledger != 123 {
			t.Fatal(got, err)
		}
		server.Close()
	}
}
