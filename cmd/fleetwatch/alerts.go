package main

import (
	"context"
	"encoding/json"
	"flag"
	"fmt"
	"io"
	"os"
	"time"

	"github.com/soroban-fleetwatch/fleetwatch/internal/store"
)

func alerts(args []string, stdout, stderr io.Writer) int {
	fs := flag.NewFlagSet("alerts", flag.ContinueOnError)
	fs.SetOutput(stderr)
	path := fs.String("state", "", "existing local replay database")
	network := fs.String("network", "", "exact journal network")
	source := fs.String("source", "synthetic", "synthetic, user_snapshot, rpc_ledger_meta or follow_ledger_meta")
	stream := fs.String("stream", "", "required with ledger-meta sources")
	if err := fs.Parse(args); err != nil {
		return 3
	}
	queueSource, scopeErr := store.AlertSource(*network, *source, *stream)
	if *path == "" || *network == "" || scopeErr != nil || fs.NArg() != 0 {
		fmt.Fprintln(stderr, "alerts requires --state FILE --network TEXT [--source synthetic|user_snapshot|rpc_ledger_meta|follow_ledger_meta] [--stream ID for ledger-meta sources]")
		return 3
	}
	if _, err := os.Stat(*path); err != nil {
		fmt.Fprintln(stderr, "state file unavailable")
		return 5
	}
	state, err := store.OpenReadOnly(*path)
	if err != nil {
		fmt.Fprintln(stderr, "state unavailable:", err)
		return 5
	}
	defer state.Close()
	var rows []store.Alert
	if *source == "follow_ledger_meta" {
		ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
		defer cancel()
		rows, err = state.FollowAlerts(ctx, *network, *stream)
	} else if *source == "rpc_ledger_meta" {
		ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
		defer cancel()
		rows, err = state.ReconstructionAlerts(ctx, *network, *stream)
	} else {
		rows, err = state.Alerts(*network, queueSource)
	}
	if err != nil {
		fmt.Fprintln(stderr, "outbox unavailable")
		return 5
	}
	enc := json.NewEncoder(stdout)
	enc.SetIndent("", "  ")
	if err := enc.Encode(rows); err != nil {
		fmt.Fprintln(stderr, "alerts output failed")
		return 5
	}
	return 0
}
