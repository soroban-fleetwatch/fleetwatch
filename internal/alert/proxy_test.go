package alert

import (
	"context"
	"crypto/hmac"
	"crypto/sha256"
	"crypto/tls"
	"crypto/x509"
	"encoding/base64"
	"encoding/hex"
	"errors"
	"io"
	"log"
	"net"
	"net/http"
	"net/http/httptest"
	"net/netip"
	"net/url"
	"os"
	"os/exec"
	"strings"
	"sync/atomic"
	"testing"
	"time"
)

func testConnectProxy(t *testing.T, handler http.Handler, secure bool) *httptest.Server {
	t.Helper()
	s := httptest.NewUnstartedServer(handler)
	s.Config.ErrorLog = log.New(io.Discard, "", 0)
	if secure {
		s.StartTLS()
	} else {
		s.Start()
	}
	t.Cleanup(s.Close)
	return s
}

func localProxyDial(t *testing.T, address string) dialer {
	t.Helper()
	d := &net.Dialer{Timeout: time.Second}
	return func(ctx context.Context, network, target string) (net.Conn, error) {
		if target != address {
			t.Error("unexpected proxy route; test refused non-fixture dial")
			return nil, errors.New("non-fixture route")
		}
		return d.DialContext(ctx, network, address)
	}
}

func TestPinnedProxyPreservesNumericAuthorityTLSAndHMAC(t *testing.T) {
	for _, secure := range []bool{false, true} {
		t.Run(map[bool]string{false: "http", true: "https"}[secure], func(t *testing.T) {
			var received, connects atomic.Int32
			receiver, roots := localHook(t, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				body, _ := io.ReadAll(r.Body)
				if r.Host != hookHost || r.TLS.ServerName != hookHost || r.Header.Get("Proxy-Authorization") != "" || r.Header.Get("Idempotency-Key") != hookAlert().ID {
					t.Error("receiver identity or credential separation failed")
				}
				mac := hmac.New(sha256.New, hookKey())
				mac.Write([]byte("v1\n" + r.Header.Get("X-FleetWatch-Timestamp") + "\n" + hookAlert().ID + "\n"))
				mac.Write(body)
				signature, err := hex.DecodeString(strings.TrimPrefix(r.Header.Get("X-FleetWatch-Signature"), "v1="))
				if err != nil || !hmac.Equal(signature, mac.Sum(nil)) {
					t.Error("HMAC changed through tunnel")
				}
				received.Add(1)
				w.WriteHeader(204)
			}))
			proxy := testConnectProxy(t, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				connects.Add(1)
				if r.Method != "CONNECT" || r.RequestURI != "8.8.8.8:443" || r.Host != "8.8.8.8:443" || r.Header.Get("Proxy-Authorization") != "Basic "+base64.StdEncoding.EncodeToString([]byte("fixture-user:fixture-secret")) {
					t.Error("proxy received unchecked authority or invalid auth")
					w.WriteHeader(403)
					return
				}
				upstream, err := net.DialTimeout("tcp", receiver.Listener.Addr().String(), time.Second)
				if err != nil {
					t.Error(err)
					return
				}
				defer upstream.Close()
				conn, rw, err := w.(http.Hijacker).Hijack()
				if err != nil {
					t.Error(err)
					return
				}
				defer conn.Close()
				conn.SetDeadline(time.Now().Add(5 * time.Second))
				upstream.SetDeadline(time.Now().Add(5 * time.Second))
				rw.WriteString("HTTP/1.1 200 Connection Established\r\n\r\n")
				rw.Flush()
				done := make(chan struct{})
				go func() { _, _ = io.Copy(upstream, rw); upstream.Close(); close(done) }()
				_, _ = io.Copy(conn, upstream)
				conn.Close()
				<-done
			}), secure)
			selected, _ := url.Parse(proxy.URL)
			selected.User = url.UserPassword("fixture-user", "fixture-secret")
			var proxyTLS *tls.Config
			if secure {
				proxyRoots := x509.NewCertPool()
				proxyRoots.AddCert(proxy.Certificate())
				proxyTLS = &tls.Config{RootCAs: proxyRoots}
			}
			d := &net.Dialer{Timeout: time.Second}
			sender, err := newProxyWebhook("https://"+hookHost+"/ingest", []string{hookHost}, hookKey(),
				func(context.Context, string) ([]netip.Addr, error) {
					return []netip.Addr{netip.MustParseAddr("8.8.8.8")}, nil
				},
				func(ctx context.Context, network, address string) (net.Conn, error) {
					if address != selected.Host {
						t.Error("dial bypassed selected proxy", address)
						return nil, errors.New("unexpected route")
					}
					return d.DialContext(ctx, network, address)
				}, selected, proxyTLS)
			if err != nil {
				t.Fatal(err)
			}
			t.Cleanup(sender.Close)
			sender.client.Transport.(*http.Transport).TLSClientConfig.RootCAs = roots
			if err := sender.Send(context.Background(), hookAlert()); err != nil || received.Load() != 1 || connects.Load() != 1 {
				t.Fatal("pinned proxy delivery failed", err, received.Load(), connects.Load())
			}
			untrustedTarget, err := newProxyWebhook("https://"+hookHost+"/ingest", []string{hookHost}, hookKey(),
				func(context.Context, string) ([]netip.Addr, error) {
					return []netip.Addr{netip.MustParseAddr("8.8.8.8")}, nil
				},
				localProxyDial(t, selected.Host), selected, proxyTLS)
			if err != nil {
				t.Fatal(err)
			}
			t.Cleanup(untrustedTarget.Close)
			var permanent PermanentError
			if err := untrustedTarget.Send(context.Background(), hookAlert()); !errors.As(err, &permanent) || received.Load() != 1 || connects.Load() != 2 {
				t.Fatal("target TLS verification was weakened by proxy mode", err)
			}
		})
	}
}

func TestPinnedProxyRejectsPrivateMixedDNSAndDeniedTunnel(t *testing.T) {
	var connects atomic.Int32
	proxy := testConnectProxy(t, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { connects.Add(1); w.WriteHeader(403) }), false)
	selected, _ := url.Parse(proxy.URL)
	for _, addresses := range [][]netip.Addr{{netip.MustParseAddr("127.0.0.1")}, {netip.MustParseAddr("8.8.8.8"), netip.MustParseAddr("10.0.0.1")}, {netip.MustParseAddr("8.8.8.8")}} {
		before := connects.Load()
		sender, err := newProxyWebhook("https://"+hookHost+"/ingest", []string{hookHost}, hookKey(), func(context.Context, string) ([]netip.Addr, error) { return addresses, nil }, localProxyDial(t, selected.Host), selected, nil)
		if err != nil {
			t.Fatal(err)
		}
		err = sender.Send(context.Background(), hookAlert())
		sender.Close()
		var permanent PermanentError
		if !errors.As(err, &permanent) {
			t.Fatal("unsafe DNS or denied CONNECT accepted", err)
		}
		if addresses[0].IsLoopback() || len(addresses) == 2 {
			if connects.Load() != before {
				t.Fatal("unsafe target reached proxy")
			}
		} else if connects.Load() != before+1 {
			t.Fatal("denied proxy route retried or bypassed")
		}
	}
}

func TestPinnedProxyHandshakeBoundsCancellationAndUntrustedTLS(t *testing.T) {
	for _, mode := range []string{"large-header", "redirect", "untrusted-tls", "cancel"} {
		t.Run(mode, func(t *testing.T) {
			proxy := testConnectProxy(t, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				switch mode {
				case "large-header":
					w.Header().Set("X-Large", strings.Repeat("x", 17<<10))
					w.WriteHeader(200)
				case "redirect":
					w.Header().Set("Location", "https://elsewhere.example.invalid")
					w.WriteHeader(302)
				case "cancel":
					<-r.Context().Done()
				default:
					w.WriteHeader(200)
				}
			}), mode == "untrusted-tls")
			selected, _ := url.Parse(proxy.URL)
			ctx, cancel := context.WithTimeout(context.Background(), 100*time.Millisecond)
			defer cancel()
			conn, err := proxyTunnel(ctx, "tcp", "8.8.8.8:443", selected, localProxyDial(t, selected.Host), nil)
			if conn != nil {
				conn.Close()
			}
			if err == nil {
				t.Fatal("invalid proxy response/cert or cancelled handshake accepted")
			}
		})
	}
	if _, err := NewWebhookWithProxyMode("https://"+hookHost, []string{hookHost}, hookKey(), "unsafe"); err == nil {
		t.Fatal("unknown proxy mode accepted")
	}
	for _, raw := range []string{"socks5://proxy.example", "http://proxy.example/path", "http://proxy.example?secret=value", "http://proxy.example:0"} {
		selected, err := url.Parse(raw)
		if err == nil && validateProxy(selected) == nil {
			t.Fatal("unsupported proxy accepted", raw)
		}
	}
}

func TestProxyModeConstructorSelection(t *testing.T) {
	// These subprocesses construct only; they never resolve, dial or send. Go's
	// environment-proxy cache requires isolation to test route selection.
	mode := os.Getenv("FLEETWATCH_PROXY_CONSTRUCTOR_TEST")
	if mode != "" {
		sender, err := NewWebhook("https://"+hookHost, []string{hookHost}, hookKey())
		if mode == "selected" && err == nil {
			sender.Close()
			t.Fatal("default constructor accepted selected proxy")
		}
		if mode == "excluded" && err != nil {
			t.Fatal("environment exclusion not respected", err)
		}
		if sender != nil {
			sender.Close()
		}
		sender, err = NewWebhookWithProxyMode("https://"+hookHost, []string{hookHost}, hookKey(), "pinned-connect")
		if err != nil {
			t.Fatal("explicit mode rejected valid route configuration", err)
		}
		sender.Close()
		return
	}
	for _, selected := range []string{"selected", "excluded"} {
		cmd := exec.Command(os.Args[0], "-test.run=^TestProxyModeConstructorSelection$")
		for _, entry := range os.Environ() {
			name, _, _ := strings.Cut(entry, "=")
			switch strings.ToUpper(name) {
			case "HTTP_PROXY", "HTTPS_PROXY", "NO_PROXY", "FLEETWATCH_PROXY_CONSTRUCTOR_TEST":
			default:
				cmd.Env = append(cmd.Env, entry)
			}
		}
		noProxy := ""
		if selected == "excluded" {
			noProxy = hookHost
		}
		cmd.Env = append(cmd.Env, "FLEETWATCH_PROXY_CONSTRUCTOR_TEST="+selected, "HTTPS_PROXY=http://127.0.0.1:9", "HTTP_PROXY=http://127.0.0.1:9", "NO_PROXY="+noProxy)
		if output, err := cmd.CombinedOutput(); err != nil {
			t.Fatalf("constructor-only subprocess failed: %v %s", err, output)
		}
	}
}
