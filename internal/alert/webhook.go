package alert

import (
	"bytes"
	"context"
	"crypto/hmac"
	"crypto/sha256"
	"crypto/tls"
	"encoding/hex"
	"encoding/json"
	"errors"
	"io"
	"net"
	"net/http"
	"net/netip"
	"net/url"
	"strconv"
	"strings"
	"time"

	"github.com/soroban-fleetwatch/fleetwatch/internal/store"
)

type resolver func(context.Context, string) ([]netip.Addr, error)
type dialer func(context.Context, string, string) (net.Conn, error)
type Webhook struct {
	endpoint string
	key      []byte
	client   *http.Client
	sinkID   string
}

// NewWebhook constructs a direct HTTPS sender with DNS validation and IP-pinned
// dialing. Proxy routing is intentionally unsupported: a proxy can resolve the
// target differently and defeat this sender's private-address checks.
func NewWebhook(endpoint string, allowedHosts []string, key []byte) (*Webhook, error) {
	resolve := func(ctx context.Context, host string) ([]netip.Addr, error) {
		ips, err := net.DefaultResolver.LookupNetIP(ctx, "ip", host)
		return ips, err
	}
	d := &net.Dialer{Timeout: 5 * time.Second}
	sender, err := newWebhook(endpoint, allowedHosts, key, resolve, d.DialContext)
	if err != nil {
		return nil, err
	}
	req, err := http.NewRequest(http.MethodPost, sender.endpoint, nil)
	if err != nil {
		sender.Close()
		return nil, err
	}
	proxy, err := http.ProxyFromEnvironment(req)
	if err != nil || proxy != nil {
		sender.Close()
		return nil, errors.New("proxy-backed webhook routing is unsupported by DNS pinning policy")
	}
	return sender, nil
}
func newWebhook(endpoint string, allowedHosts []string, key []byte, resolve resolver, dial dialer) (*Webhook, error) {
	u, err := url.Parse(endpoint)
	if err != nil || u.Scheme != "https" || u.Hostname() == "" || u.User != nil || u.RawQuery != "" || u.ForceQuery || u.Fragment != "" || (u.Port() != "" && u.Port() != "443") || len(endpoint) > 2048 {
		return nil, errors.New("webhook requires HTTPS on port 443 without credentials, query, or fragment")
	}
	host := strings.ToLower(u.Hostname())
	if !validHostname(host) {
		return nil, errors.New("webhook requires a canonical public DNS hostname")
	}
	permitted := false
	for _, allowed := range allowedHosts {
		if validHostname(allowed) && allowed == host {
			permitted = true
		}
	}
	if !permitted {
		return nil, errors.New("webhook hostname is not explicitly allowed")
	}
	if len(key) < 32 || len(key) > 4096 {
		return nil, errors.New("webhook signing key must contain 32-4096 bytes")
	}
	u.Host = host
	endpoint = u.String()
	transport := &http.Transport{
		Proxy:                  nil,
		TLSClientConfig:        &tls.Config{MinVersion: tls.VersionTLS12},
		TLSHandshakeTimeout:    5 * time.Second,
		ResponseHeaderTimeout:  10 * time.Second,
		MaxResponseHeaderBytes: 16 << 10,
		IdleConnTimeout:        30 * time.Second,
		DialContext: func(ctx context.Context, network, address string) (net.Conn, error) {
			target, port, err := net.SplitHostPort(address)
			if err != nil || target != host || port != "443" {
				return nil, PermanentError{}
			}
			ips, err := resolve(ctx, host)
			if err != nil {
				return nil, errors.New("webhook DNS lookup failed")
			}
			if len(ips) == 0 || len(ips) > 32 {
				return nil, PermanentError{}
			}
			for _, ip := range ips {
				if !publicAddress(ip) {
					return nil, PermanentError{}
				}
			}
			// Dial only an address returned by this checked resolution. Do not pass
			// the hostname back into a second resolver (DNS rebinding protection).
			return dial(ctx, network, net.JoinHostPort(ips[0].Unmap().String(), "443"))
		},
	}
	client := &http.Client{Transport: transport, Timeout: 15 * time.Second, CheckRedirect: func(*http.Request, []*http.Request) error { return PermanentError{} }}
	sum := sha256.Sum256([]byte(endpoint))
	return &Webhook{endpoint: endpoint, key: append([]byte(nil), key...), client: client, sinkID: "webhook-sha256:" + hex.EncodeToString(sum[:])}, nil
}
func validHostname(host string) bool {
	if len(host) == 0 || len(host) > 253 || host != strings.ToLower(host) || !strings.Contains(host, ".") || strings.HasSuffix(host, ".") {
		return false
	}
	if _, err := netip.ParseAddr(host); err == nil {
		return false
	}
	for _, label := range strings.Split(host, ".") {
		if len(label) == 0 || len(label) > 63 || label[0] == '-' || label[len(label)-1] == '-' {
			return false
		}
		for _, c := range label {
			if !(c >= 'a' && c <= 'z' || c >= '0' && c <= '9' || c == '-') {
				return false
			}
		}
	}
	return true
}

var deniedRanges = []netip.Prefix{
	netip.MustParsePrefix("0.0.0.0/8"), netip.MustParsePrefix("100.64.0.0/10"), netip.MustParsePrefix("192.0.0.0/24"), netip.MustParsePrefix("192.0.2.0/24"), netip.MustParsePrefix("192.88.99.0/24"), netip.MustParsePrefix("198.18.0.0/15"), netip.MustParsePrefix("198.51.100.0/24"), netip.MustParsePrefix("203.0.113.0/24"), netip.MustParsePrefix("240.0.0.0/4"), netip.MustParsePrefix("2001::/23"), netip.MustParsePrefix("2001:db8::/32"), netip.MustParsePrefix("2002::/16"), netip.MustParsePrefix("64:ff9b::/96"), netip.MustParsePrefix("64:ff9b:1::/48"), netip.MustParsePrefix("100::/64"), netip.MustParsePrefix("3fff::/20"),
}

func publicAddress(ip netip.Addr) bool {
	if !ip.IsValid() || ip.Zone() != "" {
		return false
	}
	ip = ip.Unmap()
	if ip.Is6() && !netip.MustParsePrefix("2000::/3").Contains(ip) {
		return false
	}
	if !ip.IsGlobalUnicast() || ip.IsPrivate() || ip.IsLoopback() || ip.IsLinkLocalUnicast() {
		return false
	}
	for _, prefix := range deniedRanges {
		if prefix.Contains(ip) {
			return false
		}
	}
	return true
}
func (w *Webhook) SinkID() string { return w.sinkID }
func (w *Webhook) Close()         { w.client.CloseIdleConnections() }
func (w *Webhook) Send(ctx context.Context, item store.Alert) error {
	if store.ValidateAlertPayload(item.ID, item.Payload) != nil {
		return PermanentError{}
	}
	body, err := json.Marshal(struct {
		SchemaVersion int                `json:"schema_version"`
		ID            string             `json:"id"`
		Payload       store.AlertPayload `json:"payload"`
	}{1, item.ID, item.Payload})
	if err != nil || len(body) > 1<<20 {
		return PermanentError{}
	}
	timestamp := strconv.FormatInt(time.Now().Unix(), 10)
	mac := hmac.New(sha256.New, w.key)
	mac.Write([]byte("v1\n" + timestamp + "\n" + item.ID + "\n"))
	mac.Write(body)
	request, err := http.NewRequestWithContext(ctx, http.MethodPost, w.endpoint, bytes.NewReader(body))
	if err != nil {
		return PermanentError{}
	}
	request.Header.Set("Content-Type", "application/json")
	request.Header.Set("Idempotency-Key", item.ID)
	request.Header.Set("X-FleetWatch-Timestamp", timestamp)
	request.Header.Set("X-FleetWatch-Signature", "v1="+hex.EncodeToString(mac.Sum(nil)))
	response, err := w.client.Do(request)
	if err != nil {
		var permanent PermanentError
		var certificate *tls.CertificateVerificationError
		if errors.As(err, &permanent) || errors.As(err, &certificate) {
			return PermanentError{}
		}
		return errors.New("webhook transport failed")
	}
	defer response.Body.Close()
	data, err := io.ReadAll(io.LimitReader(response.Body, (64<<10)+1))
	if err != nil || len(data) > 64<<10 {
		return errors.New("webhook response outside local limits")
	}
	if response.StatusCode >= 200 && response.StatusCode < 300 {
		return nil
	}
	if response.StatusCode == 408 || response.StatusCode == 429 || response.StatusCode >= 500 {
		return errors.New("webhook temporarily unavailable")
	}
	return PermanentError{}
}
