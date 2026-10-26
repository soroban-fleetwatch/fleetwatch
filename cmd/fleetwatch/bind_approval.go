package main

import (
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"io"
	"os"

	"github.com/soroban-fleetwatch/fleetwatch/internal/policy"
	"github.com/soroban-fleetwatch/fleetwatch/internal/store"
)

func bindApproval(args []string, stdout, stderr io.Writer) int {
	fs := flag.NewFlagSet("bind-approval", flag.ContinueOnError)
	fs.SetOutput(stderr)
	path := fs.String("state", "", "existing replay database")
	network := fs.String("network", "", "exact persisted journal network")
	source := fs.String("source", "synthetic", "synthetic or user_snapshot")
	eventID := fs.String("event-id", "", "exact persisted reference-change event ID")
	approval := fs.String("approval", "", "signed approval envelope JSON")
	trustPath := fs.String("trust", "", "operator-provided public trust configuration JSON")
	if err := fs.Parse(args); err != nil {
		return 3
	}
	if *path == "" || *network == "" || *eventID == "" || *approval == "" || *trustPath == "" || (*source != "synthetic" && *source != "user_snapshot") || fs.NArg() != 0 {
		fmt.Fprintln(stderr, "bind-approval requires --state FILE --network TEXT --event-id ID --approval FILE --trust FILE")
		return 3
	}
	envelope, err := policy.LoadEnvelope(*approval)
	if err != nil {
		fmt.Fprintln(stderr, "invalid approval:", err)
		return 3
	}
	trust, err := policy.LoadTrust(*trustPath)
	if err != nil {
		fmt.Fprintln(stderr, "invalid trust configuration:", err)
		return 3
	}
	if _, err := os.Stat(*path); err != nil {
		fmt.Fprintln(stderr, "state file unavailable")
		return 5
	}
	state, err := store.Open(*path)
	if err != nil {
		fmt.Fprintln(stderr, "state unavailable")
		return 5
	}
	defer state.Close()
	result, err := state.BindApproval(*network, *source, *eventID, envelope, trust)
	if err != nil {
		switch {
		case errors.Is(err, store.ErrApprovalNonceUsed), errors.Is(err, store.ErrApprovalEventBound):
			fmt.Fprintln(stderr, err)
			return 2
		case errors.Is(err, store.ErrApprovalEventUnavailable):
			fmt.Fprintln(stderr, err)
			return 4
		default:
			fmt.Fprintln(stderr, "approval binding failed; retry the exact binding to check persisted state")
			return 5
		}
	}
	enc := json.NewEncoder(stdout)
	enc.SetIndent("", "  ")
	if err := enc.Encode(result); err != nil {
		fmt.Fprintln(stderr, "binding output failed; inspect persisted state before retrying")
		return 5
	}
	if result.Verification.VerificationStatus == "UNDETERMINED" {
		return 4
	}
	if result.Verification.VerificationStatus != "VERIFIED" {
		return 2
	}
	return 0
}
