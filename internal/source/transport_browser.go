//go:build js && wasm

package source

import "net/http"

// The browser owns TLS, CORS and networking. Go's default native transport has
// dial hooks that bypass Fetch, so use an empty transport and explicit Fetch
// options. Redirects must fail before the browser follows them.
type browserTransport struct{ transport http.Transport }

func (t *browserTransport) RoundTrip(req *http.Request) (*http.Response, error) {
	r := req.Clone(req.Context())
	r.Header.Set("js.fetch:mode", "cors")
	r.Header.Set("js.fetch:credentials", "omit")
	r.Header.Set("js.fetch:redirect", "error")
	return t.transport.RoundTrip(r)
}

func rpcTransport() http.RoundTripper { return &browserTransport{} }
