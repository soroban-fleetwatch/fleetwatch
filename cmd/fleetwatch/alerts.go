package main

import (
	"encoding/json"
	"flag"
	"fmt"
	"io"
	"os"

	"github.com/soroban-fleetwatch/fleetwatch/internal/store"
)

func alerts(args []string, stdout, stderr io.Writer) int {
	fs := flag.NewFlagSet("alerts", flag.ContinueOnError)
	fs.SetOutput(stderr)
	path := fs.String("state", "", "existing local replay database")
	network := fs.String("network", "", "exact journal network")
	source := fs.String("source", "synthetic", "synthetic or user_snapshot")
	if err := fs.Parse(args); err != nil {
		return 3
	}
	if *path == "" || *network == "" || (*source != "synthetic" && *source != "user_snapshot") || fs.NArg() != 0 {
		fmt.Fprintln(stderr, "alerts requires --state FILE --network TEXT [--source synthetic|user_snapshot]")
		return 3
	}
	if _, err := os.Stat(*path); err != nil {
		fmt.Fprintln(stderr, "state file unavailable")
		return 5
	}
	state, err := store.Open(*path)
	if err != nil {
		fmt.Fprintln(stderr, "state unavailable:", err)
		return 5
	}
	defer state.Close()
	rows, err := state.Alerts(*network, *source)
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
