package alert

import (
	"context"
	"crypto/ecdsa"
	"crypto/elliptic"
	"crypto/hmac"
	"crypto/rand"
	"crypto/sha256"
	"crypto/tls"
	"crypto/x509"
	"crypto/x509/pkix"
	"encoding/hex"
	"encoding/json"
	"errors"
	"io"
	"log"
	"math/big"
	"net"
	"net/http"
	"net/http/httptest"
	"net/netip"
	"os"
	"os/exec"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"github.com/soroban-fleetwatch/fleetwatch/internal/fleetwatch"
	"github.com/soroban-fleetwatch/fleetwatch/internal/store"
)

const hookHost = "hooks.example.com"

func hookKey() []byte { return []byte(strings.Repeat("test-only-key-", 4)) }
func hookAlert() store.Alert {
	id := strings.Repeat("a", 64)
	return store.Alert{ID: id, LeaseToken: "must-not-be-sent", Token: "must-not-be-sent", Payload: store.AlertPayload{SchemaVersion: 1, Network: "TESTNET-SYNTHETIC", Source: "synthetic", EvaluationStatus: "UNVERIFIED_INPUT", Event: &fleetwatch.ReplayEvent{ID: id, Ledger: 101, Finding: fleetwatch.Finding{Code: "UNAPPROVED_REFERENCE_CHANGE", Severity: "critical", Authorization: "UNVERIFIED"}}}}
}
func localHook(t *testing.T, handler http.Handler) (*httptest.Server, *x509.CertPool) {
	t.Helper()
	key, err := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	if err != nil {
		t.Fatal(err)
	}
	template := &x509.Certificate{SerialNumber: big.NewInt(1), Subject: pkix.Name{CommonName: hookHost}, DNSNames: []string{hookHost}, NotBefore: time.Now().Add(-time.Minute), NotAfter: time.Now().Add(time.Hour), KeyUsage: x509.KeyUsageDigitalSignature | x509.KeyUsageCertSign, ExtKeyUsage: []x509.ExtKeyUsage{x509.ExtKeyUsageServerAuth}, IsCA: true, BasicConstraintsValid: true}
	der, err := x509.CreateCertificate(rand.Reader, template, template, &key.PublicKey, key)
	if err != nil {
		t.Fatal(err)
	}
	cert, err := x509.ParseCertificate(der)
	if err != nil {
		t.Fatal(err)
	}
	server := httptest.NewUnstartedServer(handler)
	server.Config.ErrorLog = log.New(io.Discard, "", 0)
	server.TLS = &tls.Config{Certificates: []tls.Certificate{{Certificate: [][]byte{der}, PrivateKey: key}}}
	server.StartTLS()
	t.Cleanup(server.Close)
	roots := x509.NewCertPool()
	roots.AddCert(cert)
	return server, roots
}
func testWebhook(t *testing.T, server *httptest.Server, roots *x509.CertPool) *Webhook {
	t.Helper()
	resolve := func(context.Context, string) ([]netip.Addr, error) {
		return []netip.Addr{netip.MustParseAddr("8.8.8.8")}, nil
	}
	dial := func(ctx context.Context, network, address string) (net.Conn, error) {
		if address != "8.8.8.8:443" {
			t.Errorf("unchecked dial target: %s", address)
		}
		var d net.Dialer
		return d.DialContext(ctx, network, server.Listener.Addr().String())
	}
	sender, err := newWebhook("https://"+hookHost+"/ingest", []string{hookHost}, hookKey(), resolve, dial)
	if err != nil {
		t.Fatal(err)
	}
	sender.client.Transport.(*http.Transport).TLSClientConfig.RootCAs = roots
	t.Cleanup(sender.Close)
	return sender
}
func TestWebhookSignsExactPayloadAndDedupID(t *testing.T) {
	server, roots := localHook(t, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != "POST" || r.Host != hookHost || r.Header.Get("Content-Type") != "application/json" {
			t.Error("wrong request")
		}
		data, err := io.ReadAll(r.Body)
		if err != nil {
			t.Error(err)
		}
		var envelope struct {
			SchemaVersion int                `json:"schema_version"`
			ID            string             `json:"id"`
			Payload       store.AlertPayload `json:"payload"`
		}
		if err := json.Unmarshal(data, &envelope); err != nil {
			t.Error(err)
		}
		if envelope.ID != hookAlert().ID || r.Header.Get("Idempotency-Key") != envelope.ID || strings.Contains(string(data), "must-not-be-sent") || envelope.Payload.EvaluationStatus != "UNVERIFIED_INPUT" {
			t.Error("unsafe delivery envelope")
		}
		timestamp := r.Header.Get("X-FleetWatch-Timestamp")
		mac := hmac.New(sha256.New, hookKey())
		mac.Write([]byte("v1\n" + timestamp + "\n" + envelope.ID + "\n"))
		mac.Write(data)
		signature := strings.TrimPrefix(r.Header.Get("X-FleetWatch-Signature"), "v1=")
		actual, err := hex.DecodeString(signature)
		if err != nil || !hmac.Equal(actual, mac.Sum(nil)) {
			t.Error("invalid signature")
		}
		altered := append([]byte{}, data...)
		altered[0] ^= 1
		other := hmac.New(sha256.New, hookKey())
		other.Write([]byte("v1\n" + timestamp + "\n" + envelope.ID + "\n"))
		other.Write(altered)
		if hmac.Equal(actual, other.Sum(nil)) {
			t.Error("tampering not detected")
		}
		w.WriteHeader(204)
	}))
	if err := testWebhook(t, server, roots).Send(context.Background(), hookAlert()); err != nil {
		t.Fatal(err)
	}
}
func TestWebhookResponseClassificationAndRedirects(t *testing.T) {
	for _, code := range []int{200, 204, 301, 400, 401, 403, 408, 429, 500, 503} {
		t.Run(http.StatusText(code), func(t *testing.T) {
			var requests atomic.Int32
			server, roots := localHook(t, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				requests.Add(1)
				if code == 301 {
					w.Header().Set("Location", "https://elsewhere.example.com/steal")
				}
				w.WriteHeader(code)
				_, _ = w.Write([]byte("receiver secret should not leak"))
			}))
			err := testWebhook(t, server, roots).Send(context.Background(), hookAlert())
			var permanent PermanentError
			wantPermanent := code >= 300 && code < 500 && code != 408 && code != 429
			if (err == nil) != (code < 300) || errors.As(err, &permanent) != wantPermanent || requests.Load() != 1 {
				t.Fatalf("code=%d error=%v requests=%d", code, err, requests.Load())
			}
			if err != nil && strings.Contains(err.Error(), "secret") {
				t.Fatal("receiver body leaked")
			}
		})
	}
}
func TestWebhookRejectsUnsafeURLsAndAddresses(t *testing.T) {
	for _, endpoint := range []string{"http://hooks.example.com", "https://hooks.example.com:8443", "https://user:pass@hooks.example.com", "https://hooks.example.com?secret=x", "https://hooks.example.com?", "https://hooks.example.com#fragment", "https://127.0.0.1", "https://localhost", "https://hooks.example.com.", "https://evil.example.com"} {
		if w, err := newWebhook(endpoint, []string{hookHost}, hookKey(), nil, nil); err == nil {
			w.Close()
			t.Fatalf("accepted unsafe URL %s", endpoint)
		}
	}
	for _, address := range []string{"127.0.0.1", "10.0.0.1", "172.16.0.1", "192.168.1.1", "169.254.169.254", "100.64.0.1", "0.0.0.0", "224.0.0.1", "255.255.255.255", "192.0.2.1", "198.18.0.1", "::1", "::ffff:127.0.0.1", "fc00::1", "fe80::1", "2001:db8::1", "64:ff9b::7f00:1", "3fff::1"} {
		if publicAddress(netip.MustParseAddr(address)) {
			t.Errorf("unsafe IP accepted: %s", address)
		}
	}
	for _, address := range []string{"8.8.8.8", "1.1.1.1", "2606:4700:4700::1111"} {
		if !publicAddress(netip.MustParseAddr(address)) {
			t.Errorf("public IP rejected: %s", address)
		}
	}
}
func TestWebhookDNSMixedAnswersFailBeforeDial(t *testing.T) {
	var dialed bool
	resolve := func(context.Context, string) ([]netip.Addr, error) {
		return []netip.Addr{netip.MustParseAddr("8.8.8.8"), netip.MustParseAddr("127.0.0.1")}, nil
	}
	dial := func(context.Context, string, string) (net.Conn, error) {
		dialed = true
		return nil, errors.New("must not dial")
	}
	sender, err := newWebhook("https://"+hookHost, []string{hookHost}, hookKey(), resolve, dial)
	if err != nil {
		t.Fatal(err)
	}
	defer sender.Close()
	var permanent PermanentError
	if err := sender.Send(context.Background(), hookAlert()); !errors.As(err, &permanent) || dialed {
		t.Fatalf("unsafe DNS result error=%v dialed=%v", err, dialed)
	}
}
func TestWebhookTLSVerificationAndResponseBound(t *testing.T) {
	server, roots := localHook(t, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { _, _ = w.Write([]byte(strings.Repeat("x", (64<<10)+1))) }))
	sender := testWebhook(t, server, roots)
	if err := sender.Send(context.Background(), hookAlert()); err == nil {
		t.Fatal("oversized response accepted")
	}
	untrusted := testWebhook(t, server, x509.NewCertPool())
	var permanent PermanentError
	if err := untrusted.Send(context.Background(), hookAlert()); !errors.As(err, &permanent) {
		t.Fatalf("untrusted certificate not rejected: %v", err)
	}
}

func TestWebhookDispatchPersistsAcknowledgements(t *testing.T) {
	state, j := prepare(t)
	var count atomic.Int32
	server, roots := localHook(t, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { count.Add(1); w.WriteHeader(204) }))
	sender := testWebhook(t, server, roots)
	result, err := Dispatch(context.Background(), state, sender, j.Network, j.Source, sender.SinkID(), 50)
	if err != nil || result.Delivered != 2 || count.Load() != 2 {
		t.Fatalf("result=%+v err=%v received=%d", result, err, count.Load())
	}
	again, err := Dispatch(context.Background(), state, sender, j.Network, j.Source, sender.SinkID(), 50)
	if err != nil || again.Attempted != 0 || count.Load() != 2 {
		t.Fatal("acknowledged webhook delivered again", err)
	}
	rows, _ := state.Alerts(j.Network, j.Source)
	for _, a := range rows {
		if a.Status != "DELIVERED" {
			t.Fatal("receiver success not durable")
		}
	}
}
func TestWebhookDNSRebindingRechecksEveryConnection(t *testing.T) {
	var calls atomic.Int32
	var dials atomic.Int32
	resolve := func(context.Context, string) ([]netip.Addr, error) {
		if calls.Add(1) == 1 {
			return []netip.Addr{netip.MustParseAddr("8.8.8.8")}, nil
		}
		return []netip.Addr{netip.MustParseAddr("10.0.0.1")}, nil
	}
	dial := func(context.Context, string, string) (net.Conn, error) {
		dials.Add(1)
		return nil, errors.New("simulated failed public connection")
	}
	sender, err := newWebhook("https://"+hookHost, []string{hookHost}, hookKey(), resolve, dial)
	if err != nil {
		t.Fatal(err)
	}
	defer sender.Close()
	if err := sender.Send(context.Background(), hookAlert()); err == nil {
		t.Fatal("expected failed public connection")
	}
	var permanent PermanentError
	if err := sender.Send(context.Background(), hookAlert()); !errors.As(err, &permanent) || dials.Load() != 1 || calls.Load() != 2 {
		t.Fatalf("rebind error=%v dials=%d lookups=%d", err, dials.Load(), calls.Load())
	}
}
func TestWebhookRejectsInvalidKeyPayloadAndCancellation(t *testing.T) {
	for _, key := range [][]byte{nil, []byte("short"), []byte(strings.Repeat("x", 4097))} {
		if _, err := newWebhook("https://"+hookHost, []string{hookHost}, key, nil, nil); err == nil {
			t.Fatal("invalid signing key accepted")
		}
	}
	server, roots := localHook(t, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { w.WriteHeader(204) }))
	sender := testWebhook(t, server, roots)
	item := hookAlert()
	item.Payload.Event.Finding.Authorization = "APPROVED"
	var permanent PermanentError
	if err := sender.Send(context.Background(), item); !errors.As(err, &permanent) {
		t.Fatal("incompatible assurance accepted")
	}
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	if err := sender.Send(ctx, hookAlert()); err == nil {
		t.Fatal("cancellation ignored")
	}
}

func TestWebhookConfiguredProxyFailsClosed(t *testing.T) {
	if mode := os.Getenv("FLEETWATCH_PROXY_TEST_MODE"); mode != "" {
		sender, err := NewWebhook("https://"+hookHost, []string{hookHost}, hookKey())
		if sender != nil {
			sender.Close()
		}
		if mode == "proxy" && (err == nil || !strings.Contains(err.Error(), "proxy-backed")) {
			t.Fatalf("proxy routing not rejected: %v", err)
		}
		if mode == "direct" && err != nil {
			t.Fatal(err)
		}
		return
	}
	for _, mode := range []string{"proxy", "direct"} {
		proxy := ""
		if mode == "proxy" {
			proxy = "http://proxy.invalid:3128"
		}
		cmd := exec.Command(os.Args[0], "-test.run=^TestWebhookConfiguredProxyFailsClosed$")
		cmd.Env = append(os.Environ(), "FLEETWATCH_PROXY_TEST_MODE="+mode, "HTTPS_PROXY="+proxy, "https_proxy="+proxy, "HTTP_PROXY="+proxy, "http_proxy="+proxy, "NO_PROXY=", "no_proxy=")
		if data, err := cmd.CombinedOutput(); err != nil {
			t.Fatalf("proxy guard %s: %v %s", mode, err, data)
		}
	}
}
