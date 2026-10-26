package main

import (
	"context"
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"io"
	"math"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/soroban-fleetwatch/fleetwatch/internal/history"
	"github.com/soroban-fleetwatch/fleetwatch/internal/ingest"
	"github.com/soroban-fleetwatch/fleetwatch/internal/jsoninput"
	"github.com/soroban-fleetwatch/fleetwatch/internal/source"
	"github.com/soroban-fleetwatch/fleetwatch/internal/store"
)

func ingestHistory(args []string, stdout, stderr io.Writer) int {
	return ingestHistoryWithRPC(args, stdout, stderr, func(endpoint string) (history.RPC, error) { return source.NewHTTPRPC(endpoint) })
}
func ingestHistoryWithRPC(args []string, stdout, stderr io.Writer, newRPC func(string) (history.RPC, error)) int {
	fs := flag.NewFlagSet("ingest", flag.ContinueOnError)
	fs.SetOutput(stderr)
	endpoint := fs.String("rpc-url", "", "HTTPS Stellar RPC endpoint")
	baseline := fs.String("baseline", "", "operator-asserted baseline JSON")
	path := fs.String("state", "", "local database for durable evidence")
	stream := fs.String("stream", "", "operator-assigned stream ID")
	to := fs.Uint64("to-ledger", 0, "inclusive target within 16 ledgers of baseline")
	if err := fs.Parse(args); err != nil {
		return 3
	}
	var b history.Baseline
	if *path == "" || *baseline == "" || *to > math.MaxUint32 || fs.NArg() != 0 || jsoninput.File(*baseline, &b) != nil || history.ValidateBaseline(b) != nil || store.ValidateReconstructionScope(b.Snapshot.Network, *stream) != nil || *to <= uint64(b.Snapshot.Ledger) || *to-uint64(b.Snapshot.Ledger) > history.MaxLedgers {
		fmt.Fprintln(stderr, "ingest requires --rpc-url HTTPS_URL --baseline FILE --state FILE --stream ID --to-ledger N (within 16 ledgers of baseline)")
		return 3
	}
	rpc, err := newRPC(*endpoint)
	if err != nil {
		fmt.Fprintln(stderr, "invalid ingestion RPC endpoint")
		return 3
	}
	s, err := store.Open(*path)
	if err != nil {
		fmt.Fprintln(stderr, "ingestion database unavailable or locked")
		return 5
	}
	defer s.Close()
	signalCtx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()
	ctx, cancel := context.WithTimeout(signalCtx, 2*time.Minute)
	defer cancel()
	result, err := ingest.Run(ctx, rpc, s, *stream, b, uint32(*to))
	if err != nil && !errors.Is(err, store.ErrReconstructionIncomplete) {
		if errors.Is(err, store.ErrReconstructionCursor) || errors.Is(err, store.ErrReconstructionBaseline) || errors.Is(err, store.ErrReconstructionBound) || errors.Is(err, context.Canceled) || errors.Is(err, context.DeadlineExceeded) {
			fmt.Fprintln(stderr, "ingestion stopped; inspect retained state and replan before retrying")
			return 4
		}
		fmt.Fprintln(stderr, "ingestion failed; inspect retained state before retrying")
		return 5
	}
	enc := json.NewEncoder(stdout)
	enc.SetIndent("", "  ")
	if err := enc.Encode(result); err != nil {
		fmt.Fprintln(stderr, "ingestion output failed; evidence may already be committed; inspect doctor before retrying")
		return 5
	}
	if err != nil {
		return 4
	}
	if result.Report != nil {
		for _, l := range result.Report.Ledgers {
			if len(l.Events) > 0 {
				return 2
			}
		}
	}
	return 0
}
