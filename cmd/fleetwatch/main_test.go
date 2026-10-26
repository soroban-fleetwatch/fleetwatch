package main

import (
	"bytes"
	"path/filepath"
	"strings"
	"testing"
)

func fixture(n string) string { return filepath.Join("..", "..", "testdata", n) }
func TestCLIUnapprovedUpgradeFails(t *testing.T) {
	var out, errs bytes.Buffer
	code := run([]string{"compare", "--before", fixture("before.json"), "--after", fixture("after.json")}, &out, &errs)
	if code != 2 || !strings.Contains(out.String(), "CONTRACT_DEMO_A") || !strings.Contains(out.String(), "CRITICAL") {
		t.Fatalf("code=%d stdout=%s stderr=%s", code, out.String(), errs.String())
	}
}
func TestCLIApprovedUpgradePasses(t *testing.T) {
	var out, errs bytes.Buffer
	code := run([]string{"compare", "--before", fixture("before.json"), "--after", fixture("after.json"), "--policy", fixture("approved-policy.json"), "--format", "json"}, &out, &errs)
	if code != 0 || !strings.Contains(out.String(), "POLICY_MATCHED_REFERENCE_CHANGE") {
		t.Fatalf("code=%d out=%s stderr=%s", code, out.String(), errs.String())
	}
}
func TestCLIValidateAndInvalidFlags(t *testing.T) {
	var out, errs bytes.Buffer
	if code := run([]string{"validate", "--snapshot", fixture("before.json")}, &out, &errs); code != 0 {
		t.Fatalf("validate exit=%d stderr=%s", code, errs.String())
	}
	if code := run([]string{"compare", "--format", "xml"}, &out, &errs); code != 3 {
		t.Fatalf("expected invalid usage, got %d", code)
	}
}

func TestCLIRejectsUnknownThreshold(t *testing.T) {
	var out, errs bytes.Buffer
	code := run([]string{"compare", "--before", fixture("before.json"), "--after", fixture("after.json"), "--fail-on", "invalid"}, &out, &errs)
	if code != 3 || !strings.Contains(errs.String(), "--fail-on") || out.Len() != 0 {
		t.Fatalf("code=%d out=%s stderr=%s", code, out.String(), errs.String())
	}
}

func TestCaptureRejectsInvalidConfiguration(t *testing.T) {
	for _, args := range [][]string{{"capture"}, {"capture", "--rpc-url", "http://example.com", "--network-passphrase", "test", "--contracts-file", "missing"}} {
		var out, errs bytes.Buffer
		if code := run(args, &out, &errs); code != 3 || out.Len() != 0 {
			t.Fatalf("code=%d out=%s errors=%s", code, out.String(), errs.String())
		}
	}
}

func TestCLIReplayTransientAndGap(t *testing.T) {
	for _, tc := range []struct {
		file   string
		exit   int
		marker string
	}{{"replay-transient.json", 2, "COMPLETE_INPUT_SEQUENCE"}, {"replay-gap.json", 4, "OBSERVATION_GAP"}} {
		var out, errs bytes.Buffer
		code := run([]string{"replay", "--journal", fixture(tc.file)}, &out, &errs)
		if code != tc.exit || !strings.Contains(out.String(), tc.marker) || errs.Len() != 0 {
			t.Fatalf("code=%d output=%s errors=%s", code, out.String(), errs.String())
		}
	}
}
