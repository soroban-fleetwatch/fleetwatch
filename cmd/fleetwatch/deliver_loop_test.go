package main

import (
	"bytes"
	"os"
	"path/filepath"
	"testing"
)

func TestDeliveryLoopCLIInvalidOptionsAndUnavailableKeyDoNotCreateState(t *testing.T) {
	path := filepath.Join(t.TempDir(), "missing.db")
	var out, errs bytes.Buffer
	base := []string{"deliver-loop", "--state", path, "--network", "SYNTHETIC-NETWORK", "--stream", "pilot", "--webhook-url", "https://hooks.example.invalid/fleetwatch", "--allow-host", "hooks.example.invalid", "--hmac-key-env", "FLEETWATCH_TEST_MISSING_LOOP_KEY"}
	t.Setenv("FLEETWATCH_TEST_MISSING_LOOP_KEY", "")
	for _, options := range [][]string{nil, {"--batch-size", "0"}, {"--batch-size", "6"}, {"--poll-interval-seconds", "0"}, {"--poll-interval-seconds", "61"}, {"--stream", "invalid stream"}, {"--check"}} {
		args := append(append([]string{}, base...), options...)
		if code := run(args, &out, &errs); code != 3 {
			t.Fatal("invalid/missing-key options accepted", options, code)
		}
	}
	if _, err := os.Stat(path); !os.IsNotExist(err) {
		t.Fatal("invalid invocation created state")
	}
	if code := run([]string{"deliver-loop"}, &out, &errs); code != 3 {
		t.Fatal(code)
	}
}
