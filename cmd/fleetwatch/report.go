package main

import (
	"encoding/json"
	"flag"
	"fmt"
	"io"
	"math"

	"github.com/soroban-fleetwatch/fleetwatch/internal/fleetwatch"
	"github.com/soroban-fleetwatch/fleetwatch/internal/store"
)

func reportFleet(args []string, stdout, stderr io.Writer) int {
	fs := flag.NewFlagSet("report", flag.ContinueOnError)
	fs.SetOutput(stderr)
	path := fs.String("state", "", "existing local replay database (read only)")
	network := fs.String("network", "", "exact journal network")
	source := fs.String("source", "synthetic", "synthetic or user_snapshot")
	ledger := fs.Uint64("ledger", 0, "exact retained ledger")
	owner := fs.String("owner", "", "exact executable reference owner")
	tag := fs.String("tag-hex", "", "raw tag bytes in hex")
	if code, ok := parseArgs(fs, args); !ok {
		return code
	}
	q := fleetwatch.FleetQuery{Network: *network, Ledger: uint32(*ledger), Owner: *owner, TagHex: *tag}
	if *path == "" || *ledger > math.MaxUint32 || q.Validate() != nil || (*source != "synthetic" && *source != "user_snapshot") || fs.NArg() != 0 {
		fmt.Fprintln(stderr, "report requires --state FILE --network TEXT --ledger N --owner ID --tag-hex HEX [--source synthetic|user_snapshot]")
		return 3
	}
	s, err := store.OpenReadOnly(*path)
	if err != nil {
		fmt.Fprintln(stderr, "report: existing database unavailable or locked")
		return 5
	}
	defer s.Close()
	r, err := s.ReportFleet(q, *source)
	if err != nil {
		fmt.Fprintln(stderr, "report: retained replay state validation failed")
		return 5
	}
	enc := json.NewEncoder(stdout)
	enc.SetIndent("", "  ")
	if err := enc.Encode(r); err != nil {
		fmt.Fprintln(stderr, "report output failed")
		return 5
	}
	if r.Current == nil || r.Current.ReferenceStatus != "OBSERVED" || r.SequenceCoverage != "COMPLETE_INPUT_SEQUENCE" {
		return 4
	}
	return 0
}
