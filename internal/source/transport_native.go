//go:build !js || !wasm

package source

import "net/http"

// Keep the ordinary Go transport, including the managed environment proxy.
func rpcTransport() http.RoundTripper { return nil }
