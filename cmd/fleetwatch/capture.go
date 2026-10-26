package main

import (
	"bufio"
	"context"
	"encoding/json"
	"flag"
	"fmt"
	"io"
	"os"
	"time"

	"github.com/soroban-fleetwatch/fleetwatch/internal/source"
)

func capture(args []string, stdout, stderr io.Writer) int {
	fs := flag.NewFlagSet("capture", flag.ContinueOnError)
	fs.SetOutput(stderr)
	endpoint := fs.String("rpc-url", "", "HTTPS Stellar RPC endpoint")
	network := fs.String("network-passphrase", "", "exact expected network passphrase")
	file := fs.String("contracts-file", "", "watchlist: one canonical contract address per line, maximum 50")
	if err := fs.Parse(args); err != nil {
		return 3
	}
	if *endpoint == "" || *network == "" || *file == "" || fs.NArg() != 0 {
		fmt.Fprintln(stderr, "capture requires --rpc-url, --network-passphrase and --contracts-file")
		return 3
	}
	rpc, err := source.NewHTTPRPC(*endpoint)
	if err != nil {
		fmt.Fprintln(stderr, err)
		return 3
	}
	f, err := os.Open(*file)
	if err != nil {
		fmt.Fprintln(stderr, "cannot open contracts file")
		return 3
	}
	defer f.Close()
	scanner := bufio.NewScanner(io.LimitReader(f, 8193))
	scanner.Buffer(make([]byte, 256), 256)
	ids := []string{}
	size := 0
	for scanner.Scan() {
		line := scanner.Text()
		size += len(line) + 1
		if line != "" {
			ids = append(ids, line)
		}
		if len(ids) > source.MaxWatchlist || size > 8192 {
			fmt.Fprintln(stderr, "watchlist exceeds local bounds")
			return 3
		}
	}
	if scanner.Err() != nil || size > 8192 {
		fmt.Fprintln(stderr, "invalid or oversized watchlist")
		return 3
	}
	if err := source.ValidateWatchlist(ids); err != nil {
		fmt.Fprintln(stderr, err)
		return 3
	}
	ctx, cancel := context.WithTimeout(context.Background(), 45*time.Second)
	defer cancel()
	result, err := source.CaptureCurrent(ctx, rpc, *network, ids)
	if err != nil {
		fmt.Fprintln(stderr, "capture unavailable:", err)
		return 4
	}
	encoder := json.NewEncoder(stdout)
	encoder.SetIndent("", "  ")
	if err := encoder.Encode(result); err != nil {
		fmt.Fprintln(stderr, "capture output failed")
		return 5
	}
	if result.Completeness != "COMPLETE" || len(result.Errors) > 0 {
		return 4
	}
	return 0
}
