package source

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"reflect"
	"strings"
	"testing"

	protocol "github.com/stellar/go-stellar-sdk/protocols/rpc"
)

func TestRPCJSONRejectsAmbiguousEnvelopeAndResult(t *testing.T) {
	good := `{"jsonrpc":"2.0","id":1,"result":{"passphrase":"network","protocolVersion":29}}`
	for _, raw := range []string{
		strings.Replace(good, `"id":1`, `"id":1,"id":1`, 1),
		strings.Replace(good, `"id":1`, `"id":2,"ID":1`, 1),
		strings.Replace(good, `"result":`, `"result":{},"result":`, 1),
		strings.Replace(good, `"passphrase":"network"`, `"passphrase":"wrong","passphrase":"network"`, 1),
		strings.Replace(good, `"passphrase":"network"`, `"Passphrase":"wrong","passphrase":"network"`, 1),
		strings.Replace(good, `"protocolVersion":29`, `"protocolVersion":28,"protocolVersion":29`, 1),
		strings.Replace(good, `"result":`, `"extension":{"key":1,"key":2},"result":`, 1),
		strings.Replace(good, `"id":1`, `"ID":1`, 1),
		strings.Replace(good, `"jsonrpc":`, `"JSONRPC":`, 1),
		strings.Replace(good, `"result":`, `"RESULT":`, 1),
		strings.Replace(good, `"result":`, `"ERROR":{"code":1},"result":`, 1),
		strings.Replace(good, `"result":`, `"error":{"code":1,"message":"sensitive upstream"},"result":`, 1),
		strings.Replace(good, `"id":1`, `"id":null`, 1),
		strings.Replace(good, `"id":1`, `"id":"1"`, 1),
		strings.Replace(good, `"result":{"passphrase":"network","protocolVersion":29}`, `"result":null`, 1),
		good + `{}`, `null`, `[]`,
		strings.Replace(good, `"passphrase":"network"`, `"passphrase":"`+string([]byte{255})+`"`, 1),
		strings.Replace(good, `"result":`, `"extension":`+strings.Repeat("[", 66)+strings.Repeat("]", 66)+`,"result":`, 1),
	} {
		var out Network
		err := decodeRPCResponse(strings.NewReader(raw), 1, &out)
		if err == nil {
			t.Fatalf("ambiguous RPC response accepted: %q", raw)
		}
		if strings.Contains(err.Error(), "sensitive") {
			t.Fatal("RPC error details leaked")
		}
	}
}
func TestRPCJSONAllowsAdditiveFieldsAndNullError(t *testing.T) {
	raw := `{"jsonrpc":"2.0","id":7,"result":{"passphrase":"network","protocolVersion":29,"friendbotUrl":"https://example.invalid","providerExtension":{"ready":true}},"error":null,"providerTrace":{"version":2}}`
	var out Network
	if err := decodeRPCResponse(strings.NewReader(raw), 7, &out); err != nil || out.Passphrase != "network" || out.ProtocolVersion != 29 {
		t.Fatal(out, err)
	}
	raw = `{"jsonrpc":"2.0","id":7,"result":{"passphrase":"network","protocolVersion":29},"error": null }`
	if err := decodeRPCResponse(strings.NewReader(raw), 7, &out); err != nil {
		t.Fatal(err)
	}
}
func TestRPCJSONBoundsReadAndPropagatesFailure(t *testing.T) {
	oversized := io.MultiReader(strings.NewReader(`{"jsonrpc":"2.0","id":1,"result":{},"padding":"`), io.LimitReader(repeatByte('x'), 25<<20), strings.NewReader(`"}`))
	var out Network
	if err := decodeRPCResponse(oversized, 1, &out); err == nil {
		t.Fatal("oversized RPC document accepted")
	}
	if err := decodeRPCResponse(failingRPCReader{}, 1, &out); err == nil || strings.Contains(err.Error(), "sensitive") {
		t.Fatal("reader error leaked or accepted", err)
	}
}

type repeatByte byte

func (b repeatByte) Read(p []byte) (int, error) {
	for i := range p {
		p[i] = byte(b)
	}
	return len(p), nil
}

type failingRPCReader struct{}

func (failingRPCReader) Read([]byte) (int, error) { return 0, fmt.Errorf("sensitive read failure") }

func TestHTTPRPCDecodeFailureReturnsZeroValues(t *testing.T) {
	server := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var req struct {
			ID     uint64 `json:"id"`
			Method string `json:"method"`
		}
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			t.Error(err)
			return
		}
		result := `{"passphrase":"sensitive partially decoded","protocolVersion":"invalid"}`
		switch req.Method {
		case "getLedgerEntries":
			result = `{"entries":[{"key":"sensitive"}],"latestLedger":"invalid"}`
		case "getLatestLedger":
			result = `{"sequence":100,"protocolVersion":"invalid"}`
		case "getLedgers":
			result = `{"ledgers":[{"sequence":100}],"latestLedger":"invalid"}`
		}
		_, _ = fmt.Fprintf(w, `{"jsonrpc":"2.0","id":%d,"result":%s}`, req.ID, result)
	}))
	defer server.Close()
	rpc, err := NewHTTPRPC(server.URL)
	if err != nil {
		t.Fatal(err)
	}
	rpc.client = server.Client()
	ctx := context.Background()
	if v, err := rpc.Network(ctx); err == nil || v != (Network{}) {
		t.Fatal(v, err)
	}
	if v, err := rpc.Entries(ctx, nil); err == nil || !reflect.DeepEqual(v, Entries{}) {
		t.Fatal(v, err)
	}
	if v, err := rpc.Head(ctx); err == nil || v != (Head{}) {
		t.Fatal(v, err)
	}
	if v, err := rpc.Ledgers(ctx, protocol.GetLedgersRequest{StartLedger: 100}); err == nil || !reflect.DeepEqual(v, protocol.GetLedgersResponse{}) {
		t.Fatal(v, err)
	}
}
func FuzzRPCJSONResponse(f *testing.F) {
	for _, raw := range []string{`{"jsonrpc":"2.0","id":1,"result":{"passphrase":"network","protocolVersion":29}}`, `{"jsonrpc":"2.0","id":1,"result":{"passphrase":"a","Passphrase":"b"}}`, `{"jsonrpc":"2.0","id":1,"result":{},"error":null}`, `null`, `{"id":1,"id":2}`} {
		f.Add([]byte(raw))
	}
	f.Fuzz(func(t *testing.T, data []byte) {
		if len(data) > 1<<20 {
			t.Skip()
		}
		var out Network
		if err := decodeRPCResponse(bytes.NewReader(data), 1, &out); err == nil {
			var envelope map[string]json.RawMessage
			if err := json.Unmarshal(data, &envelope); err != nil {
				t.Fatal("accepted invalid JSON")
			}
			var version string
			var id uint64
			if json.Unmarshal(envelope["jsonrpc"], &version) != nil || json.Unmarshal(envelope["id"], &id) != nil || version != "2.0" || id != 1 {
				t.Fatal("accepted mismatched protocol/ID")
			}
		}
	})
}
func BenchmarkRPCJSONMetadataSizedResponse(b *testing.B) {
	raw := []byte(`{"jsonrpc":"2.0","id":1,"result":{"passphrase":"network","protocolVersion":29},"providerPadding":"` + strings.Repeat("A", 2<<20) + `"}`)
	b.ReportAllocs()
	b.SetBytes(int64(len(raw)))
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		var out Network
		if err := decodeRPCResponse(bytes.NewReader(raw), 1, &out); err != nil {
			b.Fatal(err)
		}
	}
}
