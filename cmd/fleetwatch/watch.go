package main

import (
	"context"
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"io"
	"os"
	"os/signal"
	"syscall"

	"github.com/soroban-fleetwatch/fleetwatch/internal/config"
	"github.com/soroban-fleetwatch/fleetwatch/internal/history"
	"github.com/soroban-fleetwatch/fleetwatch/internal/ingest"
	"github.com/soroban-fleetwatch/fleetwatch/internal/jsoninput"
	"github.com/soroban-fleetwatch/fleetwatch/internal/monitor"
	"github.com/soroban-fleetwatch/fleetwatch/internal/source"
	"github.com/soroban-fleetwatch/fleetwatch/internal/store"
)

func watch(args []string, stdout, stderr io.Writer) int {
	return watchWithRPC(args, stdout, stderr, func(endpoint string) (history.RPC, error) { return source.NewHTTPRPC(endpoint) })
}
func watchWithRPC(args []string, stdout, stderr io.Writer, newRPC func(string) (history.RPC, error)) int {
	fs := flag.NewFlagSet("watch", flag.ContinueOnError)
	fs.SetOutput(stderr)
	path := fs.String("config", "", "bounded watch JSON configuration")
	if code, ok := parseArgs(fs, args); !ok {
		return code
	}
	if *path == "" || fs.NArg() != 0 {
		fmt.Fprintln(stderr, "watch requires --config FILE")
		return 3
	}
	cfg, err := config.LoadWatch(*path)
	if err != nil {
		fmt.Fprintln(stderr, "invalid watch configuration")
		return 3
	}
	var b history.Baseline
	if jsoninput.File(cfg.Baseline, &b) != nil || history.ValidateBaseline(b) != nil || store.ValidateReconstructionScope(b.Snapshot.Network, cfg.Stream) != nil || uint64(cfg.ToLedger) <= uint64(b.Snapshot.Ledger) || uint64(cfg.ToLedger)-uint64(b.Snapshot.Ledger) > history.MaxLedgers {
		fmt.Fprintln(stderr, "invalid watch baseline, scope or bounded target")
		return 3
	}
	rpc, err := newRPC(cfg.RPCURL)
	if err != nil {
		fmt.Fprintln(stderr, "invalid watch RPC endpoint")
		return 3
	}
	s, err := store.Open(cfg.State)
	if err != nil {
		fmt.Fprintln(stderr, "watch database unavailable or locked")
		return 5
	}
	defer s.Close()
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()
	enc := json.NewEncoder(stdout)
	last, err := monitor.Run(ctx, monitor.Scope{Network: b.Snapshot.Network, Stream: cfg.Stream, ToLedger: cfg.ToLedger}, cfg.Options(), func(ctx context.Context) (ingest.Result, error) {
		return ingest.Run(ctx, rpc, s, cfg.Stream, b, cfg.ToLedger)
	}, func(e monitor.Event) error { return enc.Encode(e) })
	if err != nil {
		if errors.Is(err, monitor.ErrOutput) {
			fmt.Fprintln(stderr, "watch output failed; progress may already be committed; inspect doctor before restarting")
			return 5
		}
		if errors.Is(err, monitor.ErrAttemptLimit) || errors.Is(err, store.ErrReconstructionIncomplete) || errors.Is(err, context.Canceled) || errors.Is(err, context.DeadlineExceeded) {
			fmt.Fprintln(stderr, "watch stopped before completion; inspect retained state and progress codes")
			return 4
		}
		fmt.Fprintln(stderr, "watch state validation or storage failed; inspect retained state before restarting")
		return 5
	}
	if last.ObservedEvents != nil && *last.ObservedEvents > 0 {
		return 2
	}
	return 0
}
