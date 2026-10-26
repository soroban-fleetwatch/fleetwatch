package main

import (
	"context"
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"io"
	"time"

	"github.com/soroban-fleetwatch/fleetwatch/internal/history"
	"github.com/soroban-fleetwatch/fleetwatch/internal/jsoninput"
	"github.com/soroban-fleetwatch/fleetwatch/internal/store"
)

func reconstruct(args []string, stdout, stderr io.Writer) int {
	fs := flag.NewFlagSet("reconstruct", flag.ContinueOnError)
	fs.SetOutput(stderr)
	baseline := fs.String("baseline", "", "operator-asserted watched baseline JSON")
	capture := fs.String("capture", "", "raw history capture JSON (not normalized changes)")
	statePath := fs.String("state", "", "optional local database for retained reconstruction evidence")
	stream := fs.String("stream", "", "operator-assigned stream ID (required with --state)")
	if err := fs.Parse(args); err != nil {
		return 3
	}
	if *baseline == "" || *capture == "" || (*statePath == "") != (*stream == "") || fs.NArg() != 0 {
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
	var report history.Reconstruction
	var reconstructionErr error
	if *statePath == "" {
		report, reconstructionErr = history.Reconstruct(ctx, b, c)
	} else {
		if store.ValidateReconstructionScope(b.Snapshot.Network, *stream) != nil {
			fmt.Fprintln(stderr, "invalid reconstruction stream ID")
			return 3
		}
		s, err := store.Open(*statePath)
		if err != nil {
			fmt.Fprintln(stderr, "reconstruction database unavailable or locked")
			return 5
		}
		defer s.Close()
		report, reconstructionErr = s.Reconstruct(ctx, *stream, b, c)
		if reconstructionErr != nil && !errors.Is(reconstructionErr, store.ErrReconstructionIncomplete) {
			if errors.Is(reconstructionErr, store.ErrConflict) || errors.Is(reconstructionErr, store.ErrReconstructionBaseline) || errors.Is(reconstructionErr, store.ErrReconstructionBound) {
				fmt.Fprintln(stderr, reconstructionErr)
				return 4
			}
			fmt.Fprintln(stderr, "checkpoint write failed; inspect retained state before retrying")
			return 5
		}
	}
	enc := json.NewEncoder(stdout)
	enc.SetIndent("", "  ")
	if err := enc.Encode(report); err != nil {
		fmt.Fprintln(stderr, "reconstruction output failed; a requested checkpoint may already be committed")
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
