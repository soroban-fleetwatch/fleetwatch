package main

import (
	"context"
	"encoding/json"
	"flag"
	"fmt"
	"io"
	"os"
	"os/signal"
	"syscall"

	"github.com/soroban-fleetwatch/fleetwatch/internal/alert"
)

func deliverLoop(args []string, stdout, stderr io.Writer) int {
	fs := flag.NewFlagSet("deliver-loop", flag.ContinueOnError)
	fs.SetOutput(stderr)
	var cfg alert.LoopConfig
	fs.StringVar(&cfg.State, "state", "", "existing imported follow outbox (waits if not created yet)")
	fs.StringVar(&cfg.Network, "network", "", "exact imported network")
	fs.StringVar(&cfg.Stream, "stream", "", "stable follow delivery stream")
	fs.IntVar(&cfg.PollIntervalSeconds, "poll-interval-seconds", 5, "wait between bounded batches, 1-60 seconds")
	fs.IntVar(&cfg.BatchSize, "batch-size", 1, "alerts per bounded batch, 1-5")
	endpoint := fs.String("webhook-url", "", "explicit authorized HTTPS receiver URL")
	host := fs.String("allow-host", "", "exact allowed receiver hostname")
	keyEnv := fs.String("hmac-key-env", "FLEETWATCH_WEBHOOK_KEY", "environment variable holding raw HMAC key")
	proxyMode := fs.String("proxy-mode", "reject", "reject proxy routes or explicitly use pinned-connect")
	check := fs.Bool("check", false, "validate local options/key/transport policy without opening state, DNS or delivery")
	if fs.Parse(args) != nil || fs.NArg() != 0 || *endpoint == "" || *host == "" || !envNamePattern.MatchString(*keyEnv) {
		fmt.Fprintln(stderr, "deliver-loop requires --state FILE --network TEXT --stream ID --webhook-url HTTPS_URL --allow-host HOST")
		return 3
	}
	// Validate local options before looking up a key or constructing transport.
	cfg.Sink = "pending-explicit-receiver"
	if cfg.Validate() != nil {
		fmt.Fprintln(stderr, "invalid follow delivery loop options")
		return 3
	}
	key, exists := os.LookupEnv(*keyEnv)
	if !exists || len(key) < 32 || len(key) > 4096 {
		fmt.Fprintln(stderr, "HMAC signing key unavailable or outside size bounds")
		return 3
	}
	sender, err := alert.NewWebhookWithProxyMode(*endpoint, []string{*host}, []byte(key), *proxyMode)
	if err != nil {
		fmt.Fprintln(stderr, "receiver or transport configuration rejected")
		return 3
	}
	defer sender.Close()
	if *check {
		fmt.Fprintln(stdout, "valid delivery options/key/transport policy; no state, DNS or delivery check performed")
		return 0
	}
	cfg.Sink = sender.SinkID()
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()
	encoder := json.NewEncoder(stdout)
	if err := alert.DeliveryLoop(ctx, cfg, sender, func(s alert.LoopStatus) error { return encoder.Encode(s) }); err != nil {
		fmt.Fprintln(stderr, "delivery loop state, validation or output failed; preserve outbox before retrying")
		return 5
	}
	return 0
}
