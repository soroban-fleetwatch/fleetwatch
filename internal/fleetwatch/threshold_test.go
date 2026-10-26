package fleetwatch

import "testing"

func TestThresholdsFailClosed(t *testing.T) {
	for _, threshold := range []string{"", "HIGH", "invalid"} {
		if !ShouldFail(Report{}, threshold) {
			t.Errorf("unknown threshold %q accepted", threshold)
		}
	}
	r := Report{Findings: []Finding{{Severity: "future-severity"}}}
	if !ShouldFail(r, "high") {
		t.Fatal("unknown severity silently ignored")
	}
	if ShouldFail(r, "never") {
		t.Fatal("explicit never must disable threshold enforcement")
	}
}
