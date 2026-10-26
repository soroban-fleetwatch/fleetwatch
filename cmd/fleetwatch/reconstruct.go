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

func reconstruct(args []string, stdout, stderr io.Writer) int {
	fs := flag.NewFlagSet("reconstruct", flag.ContinueOnError)
	fs.SetOutput(stderr)
	baseline := fs.String("baseline", "", "operator-asserted watched baseline JSON")
	capture := fs.String("capture", "", "raw history capture JSON (not normalized changes)")
	if err := fs.Parse(args); err != nil {
		return 3
	}
	if *baseline == "" || *capture == "" || fs.NArg() != 0 {
		fmt.Fprintln(stderr, "reconstruct requires --baseline FILE --capture FILE")
		return 3
	}
	var b history.Baseline
	var c history.Capture
	if jsoninput.File(*baseline, &b) != nil || history.ValidateBaseline(b) != nil || jsoninput.File(*capture, &c) != nil || history.ValidateCaptureEnvelope(c) != nil {
		fmt.Fprintln(stderr, "invalid reconstruction baseline or capture")
		return 3
	}
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	report, reconstructionErr := history.Reconstruct(ctx, b, c)
	enc := json.NewEncoder(stdout)
	enc.SetIndent("", "  ")
	if err := enc.Encode(report); err != nil {
		fmt.Fprintln(stderr, "reconstruction output failed")
		return 5
	}
	if reconstructionErr != nil {
		return 4
	}
	for _, ledger := range report.Ledgers {
		if len(ledger.Events) > 0 {
			return 2
		}
	}
	return 0
}
