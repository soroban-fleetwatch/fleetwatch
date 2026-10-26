package main

import (
	"context"
	"encoding/json"
	"flag"
	"fmt"
	"io"
	"os"
	"regexp"
	"time"

	"github.com/soroban-fleetwatch/fleetwatch/internal/alert"
	"github.com/soroban-fleetwatch/fleetwatch/internal/store"
)

var envNamePattern = regexp.MustCompile(`^[A-Za-z_][A-Za-z0-9_]*$`)

func deliver(args []string, stdout, stderr io.Writer) int {
	fs := flag.NewFlagSet("deliver", flag.ContinueOnError)
	fs.SetOutput(stderr)
	path := fs.String("state", "", "existing replay database")
	network := fs.String("network", "", "exact journal network")
	source := fs.String("source", "synthetic", "synthetic, user_snapshot or rpc_ledger_meta")
	stream := fs.String("stream", "", "required with --source rpc_ledger_meta")
	endpoint := fs.String("webhook-url", "", "HTTPS receiver URL")
	host := fs.String("allow-host", "", "exact lowercase allowed receiver hostname")
	keyEnv := fs.String("hmac-key-env", "FLEETWATCH_WEBHOOK_KEY", "environment variable holding raw HMAC key (32-4096 bytes)")
	limit := fs.Int("limit", 10, "maximum alerts this batch, 1-50")
	if err := fs.Parse(args); err != nil {
		return 3
	}
	queueSource, scopeErr := store.AlertSource(*network, *source, *stream)
	if *path == "" || *network == "" || *endpoint == "" || *host == "" || scopeErr != nil || !envNamePattern.MatchString(*keyEnv) || *limit < 1 || *limit > 50 || fs.NArg() != 0 {
		fmt.Fprintln(stderr, "deliver requires --state FILE --network TEXT --webhook-url HTTPS_URL --allow-host HOST and valid batch/key options")
		return 3
	}
	key, exists := os.LookupEnv(*keyEnv)
	if !exists || len(key) < 32 || len(key) > 4096 {
		fmt.Fprintln(stderr, "HMAC signing key is unavailable or outside size bounds")
		return 3
	}
	sender, err := alert.NewWebhook(*endpoint, []string{*host}, []byte(key))
	if err != nil {
		fmt.Fprintln(stderr, err)
		return 3
	}
	defer sender.Close()
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
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Minute)
	defer cancel()
	result, err := alert.Dispatch(ctx, state, sender, *network, queueSource, sender.SinkID(), *limit)
	if encodeErr := json.NewEncoder(stdout).Encode(result); encodeErr != nil {
		fmt.Fprintln(stderr, "delivery report output failed")
		return 5
	}
	if err != nil {
		fmt.Fprintln(stderr, "delivery batch interrupted; inspect durable outbox before retrying")
		return 4
	}
	if result.Failed > 0 {
		return 4
	}
	return 0
}
