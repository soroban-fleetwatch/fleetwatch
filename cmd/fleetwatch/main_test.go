package main

import (
	"bytes"
	"encoding/json"
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

func TestCLIReplayDurableRepeat(t *testing.T) {
	path := filepath.Join(t.TempDir(), "state.db")
	var first string
	for i := 0; i < 2; i++ {
		var out, errs bytes.Buffer
		code := run([]string{"replay", "--journal", fixture("replay-transient.json"), "--state", path}, &out, &errs)
		if code != 2 || errs.Len() != 0 {
			t.Fatalf("code=%d err=%s", code, errs.String())
		}
		if i == 0 {
			first = out.String()
		} else if out.String() != first {
			t.Fatal("durable repeated replay changed output")
		}
	}
}

func TestCLIListsOutboxWithoutDelivering(t *testing.T) {
	path := filepath.Join(t.TempDir(), "state.db")
	var out, errs bytes.Buffer
	if code := run([]string{"replay", "--journal", fixture("replay-transient.json"), "--state", path}, &out, &errs); code != 2 {
		t.Fatalf("code=%d %s", code, errs.String())
	}
	out.Reset()
	errs.Reset()
	if code := run([]string{"alerts", "--state", path, "--network", "TESTNET-SYNTHETIC"}, &out, &errs); code != 0 || strings.Count(out.String(), `"status": "PENDING"`) != 2 || strings.Contains(out.String(), "lease_token") {
		t.Fatalf("code=%d out=%s err=%s", code, out.String(), errs.String())
	}
}

func TestCLIDeliveryRejectsMissingKeyWithoutSending(t *testing.T) {
	const name = "FLEETWATCH_TEST_MISSING_KEY"
	t.Setenv(name, "")
	var out, errs bytes.Buffer
	code := run([]string{"deliver", "--state", "not-opened.db", "--network", "synthetic", "--webhook-url", "https://hooks.example.com", "--allow-host", "hooks.example.com", "--hmac-key-env", name}, &out, &errs)
	if code != 3 || out.Len() != 0 || !strings.Contains(errs.String(), "signing key") {
		t.Fatalf("code=%d output=%s error=%s", code, out.String(), errs.String())
	}
}

func TestCLISignedApprovalVerificationAndUnsignedRejection(t *testing.T) {
	var out, errs bytes.Buffer
	args := []string{"verify-approval", "--approval", fixture("signed-approval.json"), "--trust", fixture("approval-trust.json"), "--observation", fixture("approval-observation.json")}
	code := run(args, &out, &errs)
	if code != 0 || !strings.Contains(out.String(), `"verification_status": "VERIFIED"`) || !strings.Contains(out.String(), `"final_verdict": "UNDETERMINED"`) {
		t.Fatalf("code=%d out=%s error=%s", code, out.String(), errs.String())
	}
	args[2] = fixture("approved-policy.json")
	out.Reset()
	errs.Reset()
	if code := run(args, &out, &errs); code != 3 || out.Len() != 0 {
		t.Fatalf("unsigned policy used as signature: code=%d out=%s", code, out.String())
	}
}

func TestCLIBindsPersistedApprovalAndRejectsNonceReuse(t *testing.T) {
	path := filepath.Join(t.TempDir(), "state.db")
	var out, errs bytes.Buffer
	if code := run([]string{"replay", "--journal", fixture("approval-journal.json"), "--state", path}, &out, &errs); code != 2 {
		t.Fatalf("replay=%d %s", code, errs.String())
	}
	var replay struct {
		Network string `json:"network"`
		Events  []struct {
			ID string `json:"id"`
		} `json:"events"`
	}
	if err := json.Unmarshal(out.Bytes(), &replay); err != nil || len(replay.Events) != 3 {
		t.Fatalf("%s %v", out.String(), err)
	}
	args := []string{"bind-approval", "--state", path, "--network", replay.Network, "--event-id", replay.Events[0].ID, "--approval", fixture("signed-approval.json"), "--trust", fixture("approval-trust.json")}
	for i := 0; i < 2; i++ {
		out.Reset()
		errs.Reset()
		code := run(args, &out, &errs)
		var result struct {
			New          bool `json:"new_binding"`
			Verification struct {
				Final string `json:"final_verdict"`
			} `json:"verification"`
		}
		if err := json.Unmarshal(out.Bytes(), &result); err != nil || code != 0 || result.New != (i == 0) || result.Verification.Final != "UNDETERMINED" {
			t.Fatalf("code=%d out=%s err=%s parse=%v", code, out.String(), errs.String(), err)
		}
	}
	args[6] = replay.Events[2].ID
	out.Reset()
	errs.Reset()
	if code := run(args, &out, &errs); code != 2 || out.Len() != 0 || !strings.Contains(errs.String(), "nonce") {
		t.Fatalf("reuse code=%d out=%s err=%s", code, out.String(), errs.String())
	}
}
