package fleetwatch

import (
	"strings"
	"testing"
)

const hashA = "aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa"
const hashB = "bbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbb"
const tag = "ff0061" // not guaranteed to be UTF-8; preserve original bytes
const owner = "CONTRACT_OWNER"

func sample(ledger uint32, hash string) Snapshot {
	return Snapshot{SchemaVersion: 1, Network: "TESTNET", Ledger: ledger, Contracts: []Contract{
		{ID: "A", Executable: Executable{Kind: "external_ref", Owner: owner, TagHex: tag}},
		{ID: "B", Executable: Executable{Kind: "external_ref", Owner: owner, TagHex: tag}},
	}, References: []Reference{{Owner: owner, TagHex: tag, WasmHash: hash}}}
}

func checkFinding(t *testing.T, report Report, code string) Finding {
	t.Helper()
	for _, f := range report.Findings {
		if f.Code == code {
			return f
		}
	}
	t.Fatalf("missing %q in %#v", code, report.Findings)
	return Finding{}
}

func TestUnapprovedReferenceChangeImpactsFleet(t *testing.T) {
	report, err := Compare(sample(100, hashA), sample(101, hashB), Policy{SchemaVersion: 1})
	if err != nil {
		t.Fatal(err)
	}
	f := checkFinding(t, report, "UNAPPROVED_REFERENCE_CHANGE")
	if f.Severity != "critical" || len(f.AffectedContracts) != 2 || f.AffectedContracts[0] != "A" || f.AffectedContracts[1] != "B" {
		t.Fatalf("unexpected finding: %#v", f)
	}
	if !ShouldFail(report, "high") {
		t.Fatal("must fail for unapproved upgrade")
	}
}
func TestApprovedReferenceChange(t *testing.T) {
	p := Policy{SchemaVersion: 1, ApprovedChanges: []Approval{{Owner: owner, TagHex: tag, FromHash: hashA, ToHash: hashB, Ticket: "CHANGE-123"}}}
	report, err := Compare(sample(100, hashA), sample(101, hashB), p)
	if err != nil {
		t.Fatal(err)
	}
	f := checkFinding(t, report, "POLICY_MATCHED_REFERENCE_CHANGE")
	if f.ApprovalTicket != "CHANGE-123" || ShouldFail(report, "high") {
		t.Fatalf("unexpected approval: %#v", f)
	}
}
func TestApprovalMustMatchExactTransition(t *testing.T) {
	p := Policy{SchemaVersion: 1, ApprovedChanges: []Approval{{Owner: owner, TagHex: tag, FromHash: hashB, ToHash: hashA, Ticket: "WRONG-DIRECTION"}}}
	report, err := Compare(sample(100, hashA), sample(101, hashB), p)
	if err != nil {
		t.Fatal(err)
	}
	checkFinding(t, report, "UNAPPROVED_REFERENCE_CHANGE")
}
func TestUnresolvedReferenceIsObservationGap(t *testing.T) {
	after := sample(101, hashA)
	after.References = nil
	report, err := Compare(sample(100, hashA), after, Policy{SchemaVersion: 1})
	if err != nil {
		t.Fatal(err)
	}
	f := checkFinding(t, report, "REFERENCE_NOT_OBSERVED")
	if f.Severity != "high" {
		t.Fatalf("must not call it confirmed deletion: %#v", f)
	}
	checkFinding(t, report, "UNRESOLVED_EXTERNAL_REFERENCE")
}
func TestContractRepointDetected(t *testing.T) {
	after := sample(101, hashA)
	after.Contracts[0].Executable.TagHex = "01"
	report, err := Compare(sample(100, hashA), after, Policy{SchemaVersion: 1})
	if err != nil {
		t.Fatal(err)
	}
	f := checkFinding(t, report, "CONTRACT_EXECUTABLE_REPOINTED")
	if f.Severity != "high" {
		t.Fatal("expected high risk")
	}
}
func TestDirectWasmUpgrade(t *testing.T) {
	before, after := sample(100, hashA), sample(101, hashA)
	before.Contracts[0].Executable = Executable{Kind: "wasm", WasmHash: hashA}
	after.Contracts[0].Executable = Executable{Kind: "wasm", WasmHash: hashB}
	report, err := Compare(before, after, Policy{SchemaVersion: 1})
	if err != nil {
		t.Fatal(err)
	}
	checkFinding(t, report, "CONTRACT_EXECUTABLE_REPOINTED")
}
func TestMissingContractNotDeletion(t *testing.T) {
	after := sample(101, hashA)
	after.Contracts = after.Contracts[1:]
	report, err := Compare(sample(100, hashA), after, Policy{SchemaVersion: 1})
	if err != nil {
		t.Fatal(err)
	}
	f := checkFinding(t, report, "CONTRACT_NOT_OBSERVED")
	if f.Severity != "high" {
		t.Fatal("should not infer removal")
	}
}
func TestComparisonValidation(t *testing.T) {
	_, err := Compare(sample(100, hashA), sample(101, hashB), Policy{})
	if err == nil {
		t.Fatal("invalid policy accepted")
	}
	_, err = Compare(sample(100, hashA), sample(100, hashB), Policy{SchemaVersion: 1})
	if err == nil {
		t.Fatal("same ledger accepted")
	}
	a := sample(101, hashB)
	a.Network = "PUBLIC"
	_, err = Compare(sample(100, hashA), a, Policy{SchemaVersion: 1})
	if err == nil {
		t.Fatal("cross-network comparison accepted")
	}
}
func TestInvalidAndDuplicateInputs(t *testing.T) {
	s := sample(100, hashA)
	s.Contracts = append(s.Contracts, s.Contracts[0])
	if err := s.Validate(); err == nil {
		t.Fatal("duplicate id allowed")
	}
	s = sample(100, hashA)
	s.References = append(s.References, s.References[0])
	if err := s.Validate(); err == nil {
		t.Fatal("duplicate ref allowed")
	}
	s = sample(100, hashA)
	s.Contracts[0].Executable.TagHex = "zz"
	if err := s.Validate(); err == nil {
		t.Fatal("non-hex tag allowed")
	}
	s = sample(100, hashA)
	s.Contracts[0].Executable.TagHex = "ff0061"
	if err := s.Validate(); err != nil {
		t.Fatalf("opaque tag must be allowed: %v", err)
	}
	p := Policy{SchemaVersion: 1, ApprovedChanges: []Approval{{Owner: owner, TagHex: tag, FromHash: hashA, ToHash: hashB, Ticket: " "}}}
	if err := p.Validate(); err == nil {
		t.Fatal("blank ticket accepted")
	}
}
func TestNoChangeNoFindings(t *testing.T) {
	report, err := Compare(sample(100, hashA), sample(101, hashA), Policy{SchemaVersion: 1})
	if err != nil {
		t.Fatal(err)
	}
	if len(report.Findings) != 0 {
		t.Fatalf("unexpected findings: %+v", report.Findings)
	}
}
func TestCaseInsensitiveHashesAndTags(t *testing.T) {
	a, b := sample(100, hashA), sample(101, strings.ToUpper(hashA))
	b.References[0].TagHex = strings.ToUpper(tag)
	b.Contracts[0].Executable.TagHex = strings.ToUpper(tag)
	report, err := Compare(a, b, Policy{SchemaVersion: 1})
	if err != nil {
		t.Fatal(err)
	}
	if len(report.Findings) != 0 {
		t.Fatalf("unexpected findings: %+v", report.Findings)
	}
}
func TestEmptyFleetValid(t *testing.T) {
	s := Snapshot{SchemaVersion: 1, Network: "TESTNET", Ledger: 123}
	if err := s.Validate(); err != nil {
		t.Fatal(err)
	}
}
