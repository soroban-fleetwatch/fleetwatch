//go:build js && wasm

// FleetWatch's browser edition is a read-only, foreground current-state viewer.
// It uses the same bounded RPC/XDR capture as the native CLI. It does not run
// historical ingestion, sign transactions, infer chain safety or send alerts.
package main

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"syscall/js"
	"time"

	"github.com/soroban-fleetwatch/fleetwatch/internal/source"
)

const endpoint = "https://soroban-testnet.stellar.org"
const network = "Test SDF Network ; September 2015"

func main() {
	callback := js.FuncOf(func(_ js.Value, args []js.Value) any {
		input := ""
		if len(args) == 1 && args[0].Type() == js.TypeString {
			input = args[0].String()
		}
		executor := js.FuncOf(func(_ js.Value, promises []js.Value) any {
			resolve, reject := promises[0], promises[1]
			go func() {
				if len(input) > 8192 {
					reject.Invoke("Watchlist exceeds 8192 bytes")
					return
				}
				ids := strings.Fields(input)
				if err := source.ValidateWatchlist(ids); err != nil {
					reject.Invoke(err.Error())
					return
				}
				rpc, err := source.NewHTTPRPC(endpoint)
				if err != nil {
					reject.Invoke("RPC configuration failed")
					return
				}
				ctx, cancel := context.WithTimeout(context.Background(), 45*time.Second)
				defer cancel()
				capture, err := source.CaptureCurrent(ctx, rpc, network, ids)
				if err != nil {
					reject.Invoke(fmt.Sprintf("Capture unavailable: %s", err))
					return
				}
				raw, err := json.Marshal(capture)
				if err != nil {
					reject.Invoke("Capture encoding failed")
					return
				}
				resolve.Invoke(string(raw))
			}()
			return nil
		})
		promise := js.Global().Get("Promise").New(executor)
		executor.Release()
		return promise
	})
	js.Global().Set("fleetwatchCapture", callback)
	js.Global().Call("dispatchEvent", js.Global().Get("Event").New("fleetwatch-ready"))
	select {}
}
