package main

import (
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

type transportFunc func(*http.Request) (*http.Response, error)

func (f transportFunc) RoundTrip(r *http.Request) (*http.Response, error) { return f(r) }

func TestBridgeOnlyForwardsAllowedRPCToFixedTestnet(t *testing.T) {
	body := `{"jsonrpc":"2.0","id":1,"method":"getNetwork","params":{}}`
	called := 0
	client := &http.Client{Transport: transportFunc(func(r *http.Request) (*http.Response, error) {
		called++
		got, _ := io.ReadAll(r.Body)
		if r.URL.String() != upstream || string(got) != body || r.Header.Get("Authorization") != "" {
			t.Fatal("request destination, bytes or headers changed")
		}
		return &http.Response{StatusCode: 200, Body: io.NopCloser(strings.NewReader(`{"jsonrpc":"2.0","id":1,"result":{}}`))}, nil
	})}
	h := handler(client)
	r := httptest.NewRequest("POST", "/", strings.NewReader(body))
	r.Header.Set("Authorization", "local-only")
	w := httptest.NewRecorder()
	h.ServeHTTP(w, r)
	if w.Code != 200 || called != 1 {
		t.Fatal("valid fixed Testnet call failed")
	}
	for _, bad := range []string{
		`{"jsonrpc":"2.0","id":1,"method":"arbitrary"}`,
		`{"jsonrpc":"2.0","id":1,"method":"getNetwork","method":"sendTransaction"}`,
		`{"jsonrpc":"2.0","id":null,"method":"getNetwork"}`,
		`[{"jsonrpc":"2.0","id":1,"method":"getNetwork"}]`,
	} {
		w = httptest.NewRecorder()
		h.ServeHTTP(w, httptest.NewRequest("POST", "/", strings.NewReader(bad)))
		if w.Code != 400 || called != 1 {
			t.Fatal("invalid RPC reached upstream")
		}
	}
	r = httptest.NewRequest("POST", "/", strings.NewReader(body))
	r.Header.Set("Origin", "https://untrusted.example")
	w = httptest.NewRecorder()
	h.ServeHTTP(w, r)
	if w.Code != 403 || called != 1 {
		t.Fatal("browser-origin request reached upstream")
	}
}
