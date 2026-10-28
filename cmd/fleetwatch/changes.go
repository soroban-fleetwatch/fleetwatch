package main

import (
	"context"
	"encoding/json"
	"flag"
	"fmt"
	"io"
	"time"

	"github.com/soroban-fleetwatch/fleetwatch/internal/history"
	"github.com/soroban-fleetwatch/fleetwatch/internal/jsoninput"
)

func normalizeChanges(args []string, stdout, stderr io.Writer) int {
	fs := flag.NewFlagSet("changes", flag.ContinueOnError)
	fs.SetOutput(stderr)
	path := fs.String("capture", "", "diagnostic history capture JSON")
	if code, ok := parseArgs(fs, args); !ok {
		return code
	}
	if *path == "" || fs.NArg() != 0 {
		fmt.Fprintln(stderr, "changes requires --capture FILE")
		return 3
	}
	var capture history.Capture
	if err := jsoninput.File(*path, &capture); err != nil {
		fmt.Fprintln(stderr, "invalid history capture input")
		return 3
	}
	if err := history.ValidateCaptureEnvelope(capture); err != nil {
		fmt.Fprintln(stderr, "invalid history capture envelope")
		return 3
	}
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	report, normalizeErr := history.Normalize(ctx, capture)
	enc := json.NewEncoder(stdout)
	enc.SetIndent("", "  ")
	if err := enc.Encode(report); err != nil {
		fmt.Fprintln(stderr, "changes output failed")
		return 5
	}
	if normalizeErr != nil {
		return 4
	}
	return 0
}
