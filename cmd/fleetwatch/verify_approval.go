package main

import (
	"encoding/json"
	"flag"
	"fmt"
	"io"

	"github.com/soroban-fleetwatch/fleetwatch/internal/policy"
)

func verifyApproval(args []string, stdout, stderr io.Writer) int {
	fs := flag.NewFlagSet("verify-approval", flag.ContinueOnError)
	fs.SetOutput(stderr)
	approval := fs.String("approval", "", "versioned signed envelope JSON")
	trustPath := fs.String("trust", "", "operator-provided public trust configuration JSON")
	observation := fs.String("observation", "", "supplied transition observation JSON")
	if err := fs.Parse(args); err != nil {
		return 3
	}
	if *approval == "" || *trustPath == "" || *observation == "" || fs.NArg() != 0 {
		fmt.Fprintln(stderr, "verify-approval requires --approval FILE --trust FILE --observation FILE")
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
	observed, err := policy.LoadObservation(*observation)
	if err != nil {
		fmt.Fprintln(stderr, "invalid observation:", err)
		return 3
	}
	result, err := policy.Verify(envelope, trust, observed)
	if err != nil {
		fmt.Fprintln(stderr, "verification input invalid")
		return 3
	}
	enc := json.NewEncoder(stdout)
	enc.SetIndent("", "  ")
	if err := enc.Encode(result); err != nil {
		fmt.Fprintln(stderr, "verification output failed")
		return 5
	}
	if result.VerificationStatus == "UNDETERMINED" {
		return 4
	}
	if result.VerificationStatus != "VERIFIED" {
		return 2
	}
	return 0
}
