package main

import (
	"context"
	"encoding/json"
	"flag"
	"fmt"
	"io"
	"time"

	"github.com/soroban-fleetwatch/fleetwatch/internal/history"
	"github.com/soroban-fleetwatch/fleetwatch/internal/store"
)

type checkpointView struct {
	SchemaVersion int                          `json:"schema_version"`
	Network       string                       `json:"network"`
	Stream        string                       `json:"stream"`
	Found         bool                         `json:"found"`
	Coverage      string                       `json:"coverage"`
	Report        *history.Reconstruction      `json:"report"`
	LastAttempt   *store.ReconstructionAttempt `json:"last_attempt"`
}

func checkpoint(args []string, stdout, stderr io.Writer) int {
	fs := flag.NewFlagSet("checkpoint", flag.ContinueOnError)
	fs.SetOutput(stderr)
	path := fs.String("state", "", "existing database (read only)")
	network := fs.String("network", "", "exact persisted network")
	stream := fs.String("stream", "", "operator-assigned stream ID")
	if code, ok := parseArgs(fs, args); !ok {
		return code
	}
	if *path == "" || store.ValidateReconstructionScope(*network, *stream) != nil || fs.NArg() != 0 {
		fmt.Fprintln(stderr, "checkpoint requires --state FILE --network TEXT --stream ID")
		return 3
	}
	s, err := store.OpenReadOnly(*path)
	if err != nil {
		fmt.Fprintln(stderr, "checkpoint database unavailable or locked")
		return 5
	}
	defer s.Close()
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	state, found, err := s.LoadReconstruction(ctx, *network, *stream)
	if err != nil {
		fmt.Fprintln(stderr, "checkpoint validation failed")
		return 5
	}
	out := checkpointView{SchemaVersion: 1, Network: *network, Stream: *stream, Found: found, Coverage: "RETAINED_PREFIX_ONLY"}
	out.LastAttempt, err = s.LastReconstructionAttempt(ctx, *network, *stream)
	if err != nil {
		fmt.Fprintln(stderr, "checkpoint attempt validation failed")
		return 5
	}
	if found && out.LastAttempt != nil && (out.LastAttempt.BaselineDigest != state.Report.BaselineDigest || out.LastAttempt.BaselineHeaderHash != state.Baseline.HeaderHash || out.LastAttempt.BaselineLedger != state.Baseline.Snapshot.Ledger || out.LastAttempt.RetainedThrough != state.Report.ProcessedThrough) {
		fmt.Fprintln(stderr, "checkpoint/attempt mismatch")
		return 5
	}
	if found && state.AlertsEnabled && out.LastAttempt == nil {
		fmt.Fprintln(stderr, "checkpoint attempt missing")
		return 5
	}
	if !found && out.LastAttempt != nil && (out.LastAttempt.Status != "PARTIAL" || out.LastAttempt.RetainedThrough != out.LastAttempt.BaselineLedger) {
		fmt.Fprintln(stderr, "attempt refers to missing checkpoint")
		return 5
	}
	if found {
		out.Report = &state.Report
	}
	enc := json.NewEncoder(stdout)
	enc.SetIndent("", "  ")
	if err := enc.Encode(out); err != nil {
		fmt.Fprintln(stderr, "checkpoint output failed")
		return 5
	}
	if !found {
		return 4
	}
	return 0
}
