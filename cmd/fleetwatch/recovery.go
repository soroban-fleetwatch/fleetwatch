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

func recovery(args []string, stdout, stderr io.Writer, list bool) int {
	name := "requeue"
	if list {
		name = "recoveries"
	}
	fs := flag.NewFlagSet(name, flag.ContinueOnError)
	fs.SetOutput(stderr)
	path := fs.String("state", "", "existing local database")
	network := fs.String("network", "", "exact network")
	source := fs.String("source", "synthetic", "synthetic, user_snapshot or rpc_ledger_meta")
	stream := fs.String("stream", "", "required for rpc_ledger_meta")
	id := fs.String("alert-id", "", "alert ID")
	var request, reason *string
	var cycle *int
	if !list {
		request = fs.String("request-id", "", "unique operator request ID")
		reason = fs.String("reason", "", "nonsecret operator rationale")
		cycle = fs.Int("expected-cycle", -1, "current recovery cycle (0 initially)")
	}
	if err := fs.Parse(args); err != nil {
		return 3
	}
	queueSource, err := store.AlertSource(*network, *source, *stream)
	if *path == "" || *network == "" || *id == "" || err != nil || fs.NArg() != 0 || !list && (*request == "" || *reason == "" || *cycle < 0) {
		fmt.Fprintln(stderr, name, "requires --state FILE --network TEXT --alert-id ID; requeue also requires --request-id ID --expected-cycle N --reason TEXT")
		return 3
	}
	// Require an existing recognized DB; invalid requests must never create one.
	probe, err := store.OpenReadOnly(*path)
	if err != nil {
		fmt.Fprintln(stderr, "state unavailable:", err)
		return 5
	}
	if list {
		defer probe.Close()
		ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
		defer cancel()
		rows, err := probe.Recoveries(ctx, *network, queueSource, *id)
		if err != nil {
			fmt.Fprintln(stderr, "recovery audit unavailable:", err)
			return 5
		}
		if err := json.NewEncoder(stdout).Encode(rows); err != nil {
			fmt.Fprintln(stderr, "recovery audit output failed")
			return 5
		}
		return 0
	}
	probe.Close()
	s, err := store.Open(*path)
	if err != nil {
		fmt.Fprintln(stderr, "state unavailable:", err)
		return 5
	}
	defer s.Close()
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	result, err := s.Requeue(ctx, *network, queueSource, store.RecoveryRequest{AlertID: *id, RequestID: *request, ExpectedCycle: *cycle, Reason: *reason}, time.Now())
	if err != nil {
		fmt.Fprintln(stderr, err)
		return 5
	}
	if err := json.NewEncoder(stdout).Encode(result); err != nil {
		fmt.Fprintln(stderr, "recovery output failed; retry the identical request ID and parameters to retrieve its receipt")
		return 5
	}
	return 0
}
