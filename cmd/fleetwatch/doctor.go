package main

import (
	"encoding/json"
	"flag"
	"fmt"
	"io"
	"time"
	"unicode/utf8"

	"github.com/soroban-fleetwatch/fleetwatch/internal/fleetwatch"
	"github.com/soroban-fleetwatch/fleetwatch/internal/store"
)

func doctor(args []string, stdout, stderr io.Writer) int {
	fs := flag.NewFlagSet("doctor", flag.ContinueOnError)
	fs.SetOutput(stderr)
	path := fs.String("state", "", "existing offline replay database (read only)")
	network := fs.String("network", "", "exact journal network")
	source := fs.String("source", "synthetic", "synthetic or user_snapshot")
	if err := fs.Parse(args); err != nil {
		return 3
	}
	if *path == "" || *network == "" || len(*network) > fleetwatch.MaxIdentifierBytes || !utf8.ValidString(*network) || (*source != "synthetic" && *source != "user_snapshot") || fs.NArg() != 0 {
		fmt.Fprintln(stderr, "doctor requires --state FILE --network TEXT [--source synthetic|user_snapshot]")
		return 3
	}
	s, err := store.OpenReadOnly(*path)
	if err != nil {
		fmt.Fprintln(stderr, "doctor: existing database unavailable or locked")
		return 5
	}
	defer s.Close()
	health, err := s.Health(*network, *source, time.Now())
	if err != nil {
		fmt.Fprintln(stderr, "doctor: state or outbox validation failed")
		return 5
	}
	enc := json.NewEncoder(stdout)
	enc.SetIndent("", "  ")
	if err := enc.Encode(health); err != nil {
		fmt.Fprintln(stderr, "doctor output failed")
		return 5
	}
	if !health.StateFound || health.SequenceCoverage != "COMPLETE_INPUT_SEQUENCE" {
		return 4
	}
	if health.Status == "NEEDS_ATTENTION" {
		return 2
	}
	return 0
}
