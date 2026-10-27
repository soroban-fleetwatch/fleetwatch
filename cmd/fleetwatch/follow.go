package main

import (
	"context"
	"encoding/hex"
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"io"
	"net"
	"net/http"
	"os"
	"os/signal"
	"path/filepath"
	"strings"
	"syscall"
	"time"

	"github.com/soroban-fleetwatch/fleetwatch/internal/follow"
	"github.com/soroban-fleetwatch/fleetwatch/internal/followdelivery"
	"github.com/soroban-fleetwatch/fleetwatch/internal/history"
	"github.com/soroban-fleetwatch/fleetwatch/internal/jsoninput"
	"github.com/soroban-fleetwatch/fleetwatch/internal/source"
	"github.com/soroban-fleetwatch/fleetwatch/internal/store"
)

func followService(args []string, stdout, stderr io.Writer) int {
	fs := flag.NewFlagSet("follow", flag.ContinueOnError)
	fs.SetOutput(stderr)
	path := fs.String("config", "", "follow configuration JSON")
	check := fs.Bool("check", false, "validate configuration and baseline without opening state or querying RPC")
	deliveryState := fs.String("delivery-state", "", "optional separate outbox database for automatic sync (no dispatch)")
	deliveryStream := fs.String("delivery-stream", "", "stable stream required with --delivery-state")
	if fs.Parse(args) != nil || *path == "" || fs.NArg() != 0 {
		fmt.Fprintln(stderr, "follow requires --config FILE [--check]")
		return 3
	}
	cfg, err := follow.LoadConfig(*path)
	if err != nil {
		fmt.Fprintln(stderr, "invalid follow configuration")
		return 3
	}
	var baseline history.Baseline
	if jsoninput.File(cfg.Baseline, &baseline) != nil || history.ValidateBaseline(baseline) != nil {
		fmt.Fprintln(stderr, "invalid follow baseline")
		return 3
	}
	if (*deliveryState == "") != (*deliveryStream == "") {
		fmt.Fprintln(stderr, "automatic sync requires both --delivery-state and --delivery-stream")
		return 3
	}
	if *deliveryState != "" {
		abs, err := filepath.Abs(*deliveryState)
		configAbs, configErr := filepath.Abs(*path)
		if err != nil || configErr != nil || abs == cfg.State || abs == cfg.Baseline || abs == configAbs || store.ValidateReconstructionScope(baseline.Snapshot.Network, *deliveryStream) != nil {
			fmt.Fprintln(stderr, "automatic sync requires a separate outbox path and valid stream")
			return 3
		}
		*deliveryState = abs
	}
	if *check {
		fmt.Fprintln(stdout, "valid experimental follow configuration and asserted baseline; no state/source check performed")
		return 0
	}
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()
	loadCtx, cancel := context.WithTimeout(ctx, 2*time.Minute)
	state, err := follow.Open(loadCtx, cfg.State, baseline, cfg.Limits())
	cancel()
	if err != nil {
		fmt.Fprintln(stderr, "follow state unavailable, over capacity or failed retained-evidence validation")
		if errors.Is(err, follow.ErrCapacity) {
			return 6
		}
		return 5
	}
	defer state.Close()
	listener, err := net.Listen("tcp4", cfg.Listen)
	if err != nil {
		fmt.Fprintln(stderr, "follow health listener unavailable")
		return 5
	}
	health := follow.NewHealth(state.Head(), cfg)
	server := &http.Server{Handler: health.Handler(), ReadHeaderTimeout: 5 * time.Second, ReadTimeout: 10 * time.Second, WriteTimeout: 10 * time.Second, IdleTimeout: 30 * time.Second, MaxHeaderBytes: 8192}
	serverErrors := make(chan error, 1)
	go func() {
		err := server.Serve(listener)
		if err != nil && !errors.Is(err, http.ErrServerClosed) {
			serverErrors <- err
			stop()
		}
	}()
	defer server.Close()
	// Only the loopback address is printed; no provider URLs, keys or watched IDs.
	fmt.Fprintf(stderr, "experimental follow health listening on %s\n", listener.Addr())
	rpc, err := source.NewHTTPRPC(cfg.RPCURL)
	if err != nil {
		return 3
	}
	enc := json.NewEncoder(stdout)
	var syncOutbox func(context.Context) error
	if *deliveryState != "" {
		syncOutbox = func(ctx context.Context) error {
			_, err := followdelivery.SyncFile(ctx, state, *deliveryState, baseline, *deliveryStream)
			return err
		}
	}
	err = follow.RunWithSync(ctx, rpc, state, cfg, health, func(s follow.Status) error { return enc.Encode(s) }, syncOutbox)
	select {
	case <-serverErrors:
		fmt.Fprintln(stderr, "follow health listener failed")
		return 5
	default:
	}
	if errors.Is(err, follow.ErrCapacity) {
		fmt.Fprintln(stderr, "follow retention capacity reached; stop and archive/inspect before restarting")
		return 6
	}
	if errors.Is(err, follow.ErrEvidence) {
		fmt.Fprintln(stderr, "follow source or projection evidence rejected; inspect before restarting")
		return 4
	}
	if errors.Is(err, follow.ErrSync) {
		fmt.Fprintln(stderr, "follow outbox sync failed; retained native findings require recovery before restart")
		return 5
	}
	if err != nil {
		fmt.Fprintln(stderr, "follow storage, validation or output failed")
		return 5
	}
	return 0
}

func followInspect(args []string, stdout, stderr io.Writer) int {
	fs := flag.NewFlagSet("follow-inspect", flag.ContinueOnError)
	fs.SetOutput(stderr)
	path := fs.String("config", "", "follow configuration JSON")
	after := fs.String("after-id", "", "exclusive stable finding ID cursor")
	limit := fs.Int("limit", 20, "finding page size 1-100")
	if fs.Parse(args) != nil || *path == "" || *limit < 1 || *limit > 100 || fs.NArg() != 0 {
		fmt.Fprintln(stderr, "follow-inspect requires --config FILE [--limit N --after-id ID]")
		return 3
	}
	if *after != "" {
		raw, err := hex.DecodeString(*after)
		if err != nil || len(raw) != 32 || *after != strings.ToLower(*after) {
			fmt.Fprintln(stderr, "invalid finding cursor")
			return 3
		}
	}
	cfg, err := follow.LoadConfig(*path)
	if err != nil {
		return 3
	}
	var baseline history.Baseline
	if jsoninput.File(cfg.Baseline, &baseline) != nil || history.ValidateBaseline(baseline) != nil {
		return 3
	}
	if _, err := os.Stat(cfg.State); err != nil {
		fmt.Fprintln(stderr, "follow state does not exist")
		return 5
	}
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Minute)
	defer cancel()
	s, err := follow.Open(ctx, cfg.State, baseline, cfg.Limits())
	if err != nil {
		fmt.Fprintln(stderr, "follow state unavailable or failed validation; stop the service before inspection")
		return 5
	}
	defer s.Close()
	cycle, err := s.Cycle()
	if err != nil {
		return 5
	}
	rows, more, err := s.Findings(ctx, *after, *limit)
	if err != nil {
		fmt.Fprintln(stderr, "invalid follow journal state")
		return 5
	}
	next := ""
	if more && len(rows) > 0 {
		next = rows[len(rows)-1].Event.ID
	}
	if json.NewEncoder(stdout).Encode(struct {
		Head      follow.Head       `json:"head"`
		Cycle     follow.Cycle      `json:"cycle"`
		Findings  []follow.Finding  `json:"findings"`
		Next      string            `json:"next_after_id"`
		Offline   bool              `json:"offline"`
		Assurance map[string]string `json:"assurance"`
	}{s.Head(), cycle, rows, next, true, map[string]string{"baseline_authentication": "UNVERIFIED", "source_authentication": "UNVERIFIED", "metadata_commitment": "NOT_VERIFIED", "membership_coverage": "UNKNOWN", "history_completeness": "UNKNOWN", "final_verdict": "UNDETERMINED"}}) != nil {
		return 5
	}
	return 0
}
