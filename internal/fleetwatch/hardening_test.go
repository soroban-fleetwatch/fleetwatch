package fleetwatch

import (
	"bytes"
	"encoding/json"
	"io"
	"strings"
	"testing"
)

func TestStrictInputBoundaries(t *testing.T) {
	for _, input := range []string{"null", "{} {}", `{"schema_version":1,"unknown":0}`, `{"schema_version":`, `{"ledger":1,"ledger":2}`, `{"contracts":[{"id":"a","id":"b"}]}`, strings.Repeat("[", 66) + strings.Repeat("]", 66)} {
		var s Snapshot
		if err := decodeStrict(strings.NewReader(input), &s); err == nil {
			t.Errorf("accepted %q", input)
		}
	}
	// Overflow must be detected even on a non-seekable reader and when the
	// bytes beyond the first complete object are all whitespace.
	payload := io.MultiReader(strings.NewReader("{}"), io.LimitReader(repeatedSpace{}, maxInputBytes))
	var s Snapshot
	if err := decodeStrict(payload, &s); err == nil || !strings.Contains(err.Error(), "limit") {
		t.Fatalf("overflow: %v", err)
	}
}

type repeatedSpace struct{}

func (repeatedSpace) Read(p []byte) (int, error) {
	for i := range p {
		p[i] = ' '
	}
	return len(p), nil
}

func TestIdentifierValidationAndBounds(t *testing.T) {
	for _, id := range []string{"", " A", "A/B", "A\nB", "A\x00B", strings.Repeat("x", MaxIdentifierBytes+1)} {
		s := sample(100, hashA)
		s.Contracts[0].ID = id
		if err := s.Validate(); err == nil {
			t.Errorf("invalid contract ID accepted: %q", id)
		}
		s = sample(100, hashA)
		s.Contracts[0].Executable.Owner = id
		if err := s.Validate(); err == nil {
			t.Errorf("invalid owner accepted: %q", id)
		}
	}
	s := sample(100, hashA)
	s.Contracts[0].Executable.TagHex = strings.Repeat("ff", MaxTagBytes+1)
	if err := s.Validate(); err == nil {
		t.Fatal("oversized tag accepted")
	}
	s = sample(100, hashA)
	s.Network = "network\nforged line"
	if err := s.Validate(); err == nil {
		t.Fatal("network control bytes accepted")
	}
	s = sample(100, hashA)
	s.Contracts = make([]Contract, MaxRecords+1)
	if err := s.Validate(); err == nil || !strings.Contains(err.Error(), "limit") {
		t.Fatal("record limit not applied")
	}
	p := Policy{SchemaVersion: 1, ApprovedChanges: []Approval{{Owner: owner, TagHex: tag, FromHash: hashA, ToHash: hashB, Ticket: strings.Repeat("x", MaxTicketBytes+1)}}}
	if err := p.Validate(); err == nil {
		t.Fatal("oversized ticket accepted")
	}
}

func TestReportsNeverClaimVerifiedCoverageOrApproval(t *testing.T) {
	p := Policy{SchemaVersion: 1, ApprovedChanges: []Approval{{Owner: owner, TagHex: tag, FromHash: hashA, ToHash: hashB, Ticket: "LOCAL-1"}}}
	for _, h := range []string{hashA, hashB} {
		r, err := Compare(sample(100, hashA), sample(101, h), p)
		if err != nil {
			t.Fatal(err)
		}
		if r.Coverage != "UNKNOWN" || r.EvaluationStatus != "UNVERIFIED_SNAPSHOTS" {
			t.Fatalf("unsafe assurance: %+v", r)
		}
		for _, f := range r.Findings {
			if f.Authorization != "UNVERIFIED" {
				t.Fatal("unsigned policy authenticated")
			}
		}
	}
}

func TestComparisonDeterminismAndOpaqueTags(t *testing.T) {
	before, after := sample(100, hashA), sample(101, hashB)
	// Two distinct raw byte sequences must remain distinct, even if neither
	// is printable text. Hex casing is representation only.
	before.References = append(before.References, Reference{Owner: owner, TagHex: "ff0062", WasmHash: hashA})
	after.References = append(after.References, Reference{Owner: owner, TagHex: "FF0062", WasmHash: hashB})
	var baseline []byte
	for i := 0; i < 50; i++ {
		r, err := Compare(before, after, Policy{SchemaVersion: 1})
		if err != nil {
			t.Fatal(err)
		}
		data, err := json.Marshal(r)
		if err != nil {
			t.Fatal(err)
		}
		if i == 0 {
			baseline = data
		} else if !bytes.Equal(baseline, data) {
			t.Fatal("nondeterministic report")
		}
		if len(r.Findings) != 2 {
			t.Fatal("distinct tags merged")
		}
	}
}

func FuzzSnapshotDecode(f *testing.F) {
	f.Add([]byte(`{"schema_version":1,"network":"TESTNET","ledger":1}`))
	f.Add([]byte(`null`))
	f.Fuzz(func(t *testing.T, data []byte) {
		if len(data) > 1<<16 {
			t.Skip()
		}
		var s Snapshot
		if decodeStrict(bytes.NewReader(data), &s) == nil {
			_ = s.Validate()
		}
	})
}
