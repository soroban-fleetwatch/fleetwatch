package alert

import (
	"bufio"
	"bytes"
	"context"
	"crypto/tls"
	"encoding/base64"
	"errors"
	"net"
	"net/http"
	"net/netip"
	"net/url"
	"strconv"
	"time"
)

// newProxyWebhook reuses the normal target DNS/public-IP validator. Its dial
// callback can connect only via the selected proxy, using a numeric CONNECT
// authority. It does not disguise that authority with the receiver hostname.
// A proxy policy denial never causes direct dialing or hostname fallback.
func newProxyWebhook(endpoint string, hosts []string, key []byte, resolve resolver, dialProxy dialer, proxy *url.URL, proxyTLS *tls.Config) (*Webhook, error) {
	if validateProxy(proxy) != nil {
		return nil, errors.New("invalid selected webhook proxy")
	}
	// Freeze operator-selected URL/credentials; requests cannot change routing.
	selected := *proxy
	return newWebhook(endpoint, hosts, key, resolve, func(ctx context.Context, network, target string) (net.Conn, error) {
		return proxyTunnel(ctx, network, target, &selected, dialProxy, proxyTLS)
	})
}

func validateProxy(proxy *url.URL) error {
	if proxy == nil || (proxy.Scheme != "http" && proxy.Scheme != "https") || proxy.Hostname() == "" || (proxy.Path != "" && proxy.Path != "/") || proxy.RawQuery != "" || proxy.ForceQuery || proxy.Fragment != "" || proxy.Opaque != "" || len(proxy.String()) > 4096 {
		return errors.New("unsupported proxy URL")
	}
	if port := proxy.Port(); port != "" {
		n, err := strconv.Atoi(port)
		if err != nil || n < 1 || n > 65535 {
			return errors.New("invalid proxy port")
		}
	}
	return nil
}

type bufferedTunnel struct {
	net.Conn
	reader *bufio.Reader
}

func (c *bufferedTunnel) Read(p []byte) (int, error) { return c.reader.Read(p) }

func proxyTunnel(ctx context.Context, network, target string, proxy *url.URL, dialProxy dialer, proxyTLS *tls.Config) (net.Conn, error) {
	// Defense in depth even though newWebhook calls us only after DNS checking.
	host, port, err := net.SplitHostPort(target)
	ip, ipErr := netip.ParseAddr(host)
	if err != nil || ipErr != nil || port != "443" || !publicAddress(ip) {
		return nil, PermanentError{}
	}
	proxyPort := proxy.Port()
	if proxyPort == "" {
		proxyPort = "80"
		if proxy.Scheme == "https" {
			proxyPort = "443"
		}
	}
	conn, err := dialProxy(ctx, network, net.JoinHostPort(proxy.Hostname(), proxyPort))
	if err != nil {
		return nil, errors.New("webhook proxy connection failed")
	}
	keep := false
	defer func() {
		if !keep {
			conn.Close()
		}
	}()
	baseConn := conn
	stopCancel := context.AfterFunc(ctx, func() { baseConn.Close() })
	defer stopCancel()
	deadline := time.Now().Add(10 * time.Second)
	if limit, ok := ctx.Deadline(); ok && limit.Before(deadline) {
		deadline = limit
	}
	if err := conn.SetDeadline(deadline); err != nil {
		return nil, err
	}
	if proxy.Scheme == "https" {
		config := &tls.Config{MinVersion: tls.VersionTLS12, ServerName: proxy.Hostname()}
		if proxyTLS != nil {
			config = proxyTLS.Clone() // isolated test trust, never provided by CLI
			config.MinVersion = tls.VersionTLS12
			config.ServerName = proxy.Hostname()
		}
		config.InsecureSkipVerify = false
		config.NextProtos = []string{"http/1.1"}
		tlsConn := tls.Client(conn, config)
		if err := tlsConn.HandshakeContext(ctx); err != nil {
			return nil, err
		}
		conn = tlsConn
	}
	// Both CONNECT request target and Host are the checked numeric authority.
	req := &http.Request{Method: http.MethodConnect, URL: &url.URL{Host: target}, Host: target, Header: make(http.Header)}
	if proxy.User != nil {
		password, _ := proxy.User.Password()
		token := base64.StdEncoding.EncodeToString([]byte(proxy.User.Username() + ":" + password))
		req.Header.Set("Proxy-Authorization", "Basic "+token)
	}
	if err := req.Write(conn); err != nil {
		return nil, errors.New("webhook proxy CONNECT write failed")
	}
	reader := bufio.NewReaderSize(conn, 4096)
	header := make([]byte, 0, 1024)
	for len(header) <= 16<<10 {
		b, err := reader.ReadByte()
		if err != nil {
			return nil, errors.New("webhook proxy CONNECT response failed")
		}
		header = append(header, b)
		if bytes.HasSuffix(header, []byte("\r\n\r\n")) {
			break
		}
	}
	if len(header) > 16<<10 || !bytes.HasSuffix(header, []byte("\r\n\r\n")) {
		return nil, PermanentError{}
	}
	response, err := http.ReadResponse(bufio.NewReader(bytes.NewReader(header)), req)
	if err != nil {
		return nil, PermanentError{}
	}
	if response.StatusCode != http.StatusOK {
		if response.StatusCode >= 500 || response.StatusCode == 408 || response.StatusCode == 429 {
			return nil, errors.New("webhook proxy temporarily unavailable")
		}
		return nil, PermanentError{}
	}
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	// Prevent a cancelled handshake from closing a successfully handed-off socket.
	if !stopCancel() && ctx.Err() != nil {
		return nil, ctx.Err()
	}
	if err := conn.SetDeadline(time.Time{}); err != nil {
		return nil, err
	}
	keep = true
	// Preserve bytes already buffered beyond the CONNECT header for target TLS.
	return &bufferedTunnel{Conn: conn, reader: reader}, nil
}
