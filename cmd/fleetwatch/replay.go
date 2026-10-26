package main

import (
	"encoding/json"
	"flag"
	"fmt"
	"io"

	"github.com/soroban-fleetwatch/fleetwatch/internal/fleetwatch"
)

func replay(args []string, stdout, stderr io.Writer) int {
	fs := flag.NewFlagSet("replay", flag.ContinueOnError)
	fs.SetOutput(stderr)
	input := fs.String("journal", "", "bounded offline per-ledger snapshot journal JSON")
	if err := fs.Parse(args); err != nil {
		return 3
	}
	if *input == "" || fs.NArg() != 0 {
		fmt.Fprintln(stderr, "replay requires --journal FILE")
		return 3
	}
	j, err := fleetwatch.LoadJournal(*input)
	if err != nil {
		fmt.Fprintln(stderr, "invalid journal:", err)
		return 3
	}
	result, err := fleetwatch.ReplayJournal(j)
	if err != nil {
		fmt.Fprintln(stderr, "invalid journal:", err)
		return 3
	}
	enc := json.NewEncoder(stdout)
	enc.SetIndent("", "  ")
	if err := enc.Encode(result); err != nil {
		fmt.Fprintln(stderr, "replay output failed")
		return 5
	}
	if len(result.Issues) > 0 {
		return 4
	}
	if len(result.Events) > 0 {
		return 2
	}
	return 0
}
