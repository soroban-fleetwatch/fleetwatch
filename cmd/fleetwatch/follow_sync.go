package main

import (
	"context"
	"encoding/json"
	"flag"
	"fmt"
	"io"
	"os"
	"os/signal"
	"path/filepath"
	"syscall"
	"time"

	"github.com/soroban-fleetwatch/fleetwatch/internal/follow"
	"github.com/soroban-fleetwatch/fleetwatch/internal/followdelivery"
	"github.com/soroban-fleetwatch/fleetwatch/internal/history"
	"github.com/soroban-fleetwatch/fleetwatch/internal/jsoninput"
	"github.com/soroban-fleetwatch/fleetwatch/internal/store"
)

func followSync(args []string, stdout, stderr io.Writer) int {
	fs := flag.NewFlagSet("follow-sync", flag.ContinueOnError)
	fs.SetOutput(stderr)
	config := fs.String("config", "", "stopped follow configuration")
	destination := fs.String("delivery-state", "", "separate durable delivery outbox database")
	stream := fs.String("stream", "", "stable delivery stream identifier")
	if fs.Parse(args) != nil || fs.NArg() != 0 || *config == "" || *destination == "" || *stream == "" {
		fmt.Fprintln(stderr, "follow-sync requires --config FILE --delivery-state FILE --stream ID")
		return 3
	}
	cfg, err := follow.LoadConfig(*config)
	if err != nil {
		return 3
	}
	var baseline history.Baseline
	if jsoninput.File(cfg.Baseline, &baseline) != nil || history.ValidateBaseline(baseline) != nil || store.ValidateReconstructionScope(baseline.Snapshot.Network, *stream) != nil {
		return 3
	}
	abs, err := filepath.Abs(*destination)
	configAbs, configErr := filepath.Abs(*config)
	if err != nil || configErr != nil || abs == cfg.State || abs == cfg.Baseline || abs == configAbs {
		fmt.Fprintln(stderr, "delivery database must be separate from source/config/baseline")
		return 3
	}
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()
	ctx, cancel := context.WithTimeout(ctx, 2*time.Minute)
	defer cancel()
	source, err := follow.OpenExisting(ctx, cfg.State, baseline, cfg.Limits())
	if err != nil {
		fmt.Fprintln(stderr, "follow state unavailable or failed replay validation; stop follower before sync")
		return 5
	}
	defer source.Close()
	outbox, err := store.Open(abs)
	if err != nil {
		fmt.Fprintln(stderr, "separate delivery state unavailable")
		return 5
	}
	defer outbox.Close()
	receipt, err := followdelivery.Sync(ctx, source, outbox, baseline, *stream)
	if err != nil {
		fmt.Fprintln(stderr, "follow import failed; prior delivery state preserved")
		return 5
	}
	if json.NewEncoder(stdout).Encode(receipt) != nil {
		fmt.Fprintln(stderr, "follow findings imported but receipt output failed; inspect outbox before retrying")
		return 5
	}
	return 0
}
