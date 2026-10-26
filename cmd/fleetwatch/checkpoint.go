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
	SchemaVersion int                     `json:"schema_version"`
	Network       string                  `json:"network"`
	Stream        string                  `json:"stream"`
	Found         bool                    `json:"found"`
	Coverage      string                  `json:"coverage"`
	Report        *history.Reconstruction `json:"report"`
}

func checkpoint(args []string, stdout, stderr io.Writer) int {
	fs := flag.NewFlagSet("checkpoint", flag.ContinueOnError)
	fs.SetOutput(stderr)
	path := fs.String("state", "", "existing database (read only)")
	network := fs.String("network", "", "exact persisted network")
	stream := fs.String("stream", "", "operator-assigned stream ID")
	if err := fs.Parse(args); err != nil {
		return 3
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
