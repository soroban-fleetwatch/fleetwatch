package main

import (
	"context"
	"encoding/json"
	"flag"
	"fmt"
	"io"
	"time"

	"github.com/soroban-fleetwatch/fleetwatch/internal/store"
)

func backup(args []string, stdout, stderr io.Writer) int {
	fs := flag.NewFlagSet("backup", flag.ContinueOnError)
	fs.SetOutput(stderr)
	path := fs.String("state", "", "existing replay database (read only)")
	destination := fs.String("output", "", "new backup path in an existing directory")
	if err := fs.Parse(args); err != nil {
		return 3
	}
	if *path == "" || *destination == "" || fs.NArg() != 0 {
		fmt.Fprintln(stderr, "backup requires --state FILE --output NEW_FILE")
		return 3
	}
	s, err := store.OpenReadOnly(*path)
	if err != nil {
		fmt.Fprintln(stderr, "backup: existing database unavailable or locked")
		return 5
	}
	defer s.Close()
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Minute)
	defer cancel()
	receipt, err := s.Backup(ctx, *destination)
	if err != nil {
		fmt.Fprintln(stderr, "backup failed; destination may exist, inspect before retrying")
		return 5
	}
	enc := json.NewEncoder(stdout)
	enc.SetIndent("", "  ")
	if err := enc.Encode(receipt); err != nil {
		fmt.Fprintln(stderr, "backup saved but receipt output failed; inspect destination before retrying")
		return 5
	}
	return 0
}
