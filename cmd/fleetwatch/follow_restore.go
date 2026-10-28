package main

import (
	"context"
	"encoding/hex"
	"encoding/json"
	"flag"
	"fmt"
	"io"
	"os"
	"os/signal"
	"strings"
	"syscall"
	"time"

	"github.com/soroban-fleetwatch/fleetwatch/internal/follow"
)

func restoreFollowArchive(args []string, stdout, stderr io.Writer) int {
	fs := flag.NewFlagSet("restore-follow-archive", flag.ContinueOnError)
	fs.SetOutput(stderr)
	input := fs.String("input", "", "existing gzip evidence archive")
	output := fs.String("output", "", "NEW follow database path")
	expected := fs.String("sha256", "", "expected compressed archive SHA256 from trusted receipt")
	if code, ok := parseArgs(fs, args); !ok {
		return code
	}
	if fs.NArg() != 0 || *input == "" || *output == "" {
		fmt.Fprintln(stderr, "restore-follow-archive requires --input FILE --output NEW_DB --sha256 EXPECTED_DIGEST")
		return 3
	}
	raw, err := hex.DecodeString(*expected)
	if err != nil || len(raw) != 32 || *expected != strings.ToLower(*expected) {
		fmt.Fprintln(stderr, "expected canonical SHA256 required from the archive handoff")
		return 3
	}
	info, err := os.Lstat(*input)
	if err != nil || !info.Mode().IsRegular() || info.Size() == 0 || info.Size() > follow.MaxArchiveBytes {
		fmt.Fprintln(stderr, "archive must be an existing regular file within the size limit")
		return 3
	}
	f, err := os.Open(*input)
	if err != nil {
		return 5
	}
	defer f.Close()
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()
	ctx, cancel := context.WithTimeout(ctx, 10*time.Minute)
	defer cancel()
	receipt, err := follow.RestoreArchive(ctx, f, *output, *expected)
	if err != nil {
		fmt.Fprintln(stderr, "archive restore failed; inspect destination before retrying; source was not replaced")
		return 5
	}
	if json.NewEncoder(stdout).Encode(receipt) != nil {
		fmt.Fprintln(stderr, "restored database saved but receipt output failed; inspect destination before retrying")
		return 5
	}
	return 0
}
