package main

import (
	"context"
	"encoding/json"
	"flag"
	"fmt"
	"io"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/soroban-fleetwatch/fleetwatch/internal/follow"
	"github.com/soroban-fleetwatch/fleetwatch/internal/history"
	"github.com/soroban-fleetwatch/fleetwatch/internal/jsoninput"
)

func followCompact(args []string, stdout, stderr io.Writer) int {
	fs := flag.NewFlagSet("follow-compact", flag.ContinueOnError)
	fs.SetOutput(stderr)
	config := fs.String("config", "", "stopped follow configuration")
	output := fs.String("output", "", "NEW compacted database path")
	if fs.Parse(args) != nil || fs.NArg() != 0 || *config == "" || *output == "" {
		fmt.Fprintln(stderr, "follow-compact requires --config FILE --output NEW_DB")
		return 3
	}
	cfg, err := follow.LoadConfig(*config)
	if err != nil {
		return 3
	}
	var baseline history.Baseline
	if jsoninput.File(cfg.Baseline, &baseline) != nil || history.ValidateBaseline(baseline) != nil {
		return 3
	}
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()
	ctx, cancel := context.WithTimeout(ctx, 10*time.Minute)
	defer cancel()
	receipt, err := follow.CompactState(ctx, cfg.State, baseline, cfg.Limits(), *output)
	if err != nil {
		fmt.Fprintln(stderr, "follow compaction failed; stop the service and inspect destination before retrying")
		return 5
	}
	if json.NewEncoder(stdout).Encode(receipt) != nil {
		fmt.Fprintln(stderr, "compacted database saved but receipt output failed; inspect destination before retrying")
		return 5
	}
	return 0
}
