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
	"time"

	"github.com/soroban-fleetwatch/fleetwatch/internal/follow"
	"github.com/soroban-fleetwatch/fleetwatch/internal/history"
	"github.com/soroban-fleetwatch/fleetwatch/internal/jsoninput"
)

func followArchive(args []string, stdout, stderr io.Writer, verify bool) int {
	name := "follow-archive"
	if verify {
		name = "verify-follow-archive"
	}
	fs := flag.NewFlagSet(name, flag.ContinueOnError)
	fs.SetOutput(stderr)
	var config, output, input string
	if verify {
		fs.StringVar(&input, "input", "", "existing gzip evidence archive")
	} else {
		fs.StringVar(&config, "config", "", "stopped follow configuration")
		fs.StringVar(&output, "output", "", "new evidence archive path")
	}
	if code, ok := parseArgs(fs, args); !ok {
		return code
	}
	if fs.NArg() != 0 || (verify && input == "") || (!verify && (config == "" || output == "")) {
		fmt.Fprintln(stderr, "follow-archive requires --config FILE --output NEW_FILE; verify-follow-archive requires --input FILE")
		return 3
	}
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()
	ctx, cancel := context.WithTimeout(ctx, 10*time.Minute)
	defer cancel()
	var receipt follow.ArchiveReceipt
	var err error
	if verify {
		info, statErr := os.Lstat(input)
		if statErr != nil || !info.Mode().IsRegular() || info.Size() == 0 || info.Size() > follow.MaxArchiveBytes {
			fmt.Fprintln(stderr, "archive must be an existing regular file within the size limit")
			return 3
		}
		file, openErr := os.Open(input)
		if openErr != nil {
			return 5
		}
		defer file.Close()
		receipt, err = follow.VerifyArchive(ctx, file)
		if err != nil {
			fmt.Fprintln(stderr, "archive verification failed; no state created or modified")
			return 4
		}
	} else {
		cfg, loadErr := follow.LoadConfig(config)
		if loadErr != nil {
			return 3
		}
		var baseline history.Baseline
		if jsoninput.File(cfg.Baseline, &baseline) != nil || history.ValidateBaseline(baseline) != nil {
			return 3
		}
		if _, err := os.Stat(cfg.State); err != nil {
			fmt.Fprintln(stderr, "archive requires existing stopped follow state")
			return 5
		}
		loadCtx, loadCancel := context.WithTimeout(ctx, 2*time.Minute)
		s, openErr := follow.OpenExisting(loadCtx, cfg.State, baseline, cfg.Limits())
		loadCancel()
		if openErr != nil {
			fmt.Fprintln(stderr, "follow state unavailable or failed validation; stop the service first")
			return 5
		}
		defer s.Close()
		receipt, err = s.ExportArchive(ctx, output)
		if err != nil {
			fmt.Fprintln(stderr, "archive export failed; inspect destination before retrying")
			return 5
		}
	}
	if json.NewEncoder(stdout).Encode(receipt) != nil {
		fmt.Fprintln(stderr, "archive receipt output failed; inspect destination before retrying")
		return 5
	}
	return 0
}
