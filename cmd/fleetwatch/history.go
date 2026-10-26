package main

import (
	"context"
	"encoding/json"
	"flag"
	"fmt"
	"io"
	"math"
	"time"

	"github.com/soroban-fleetwatch/fleetwatch/internal/history"
	"github.com/soroban-fleetwatch/fleetwatch/internal/source"
)

func captureHistory(args []string, stdout, stderr io.Writer) int {
	fs := flag.NewFlagSet("history", flag.ContinueOnError)
	fs.SetOutput(stderr)
	url := fs.String("rpc-url", "", "HTTPS Stellar RPC endpoint")
	network := fs.String("network-passphrase", "", "exact expected network passphrase")
	from := fs.Uint64("from-ledger", 0, "first ledger (inclusive)")
	to := fs.Uint64("to-ledger", 0, "last ledger (inclusive; at most 16 ledgers)")
	if err := fs.Parse(args); err != nil {
		return 3
	}
	if *from > math.MaxUint32 || *to > math.MaxUint32 || history.ValidateRequest(*network, uint32(*from), uint32(*to)) != nil || fs.NArg() != 0 {
		fmt.Fprintln(stderr, "history requires --rpc-url HTTPS_URL --network-passphrase TEXT --from-ledger N --to-ledger M (maximum 16 ledgers)")
		return 3
	}
	rpc, err := source.NewHTTPRPC(*url)
	if err != nil {
		fmt.Fprintln(stderr, "invalid history RPC endpoint")
		return 3
	}
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Minute)
	defer cancel()
	result, fetchErr := history.Fetch(ctx, rpc, *network, uint32(*from), uint32(*to))
	enc := json.NewEncoder(stdout)
	enc.SetIndent("", "  ")
	if err := enc.Encode(result); err != nil {
		fmt.Fprintln(stderr, "history output failed")
		return 5
	}
	if fetchErr != nil {
		return 4
	}
	return 0
}
