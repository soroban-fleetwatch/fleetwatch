package fleetwatch

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"sort"
	"strings"
)

const MaxJournalSnapshots = 1000

type Journal struct {
	SchemaVersion int        `json:"schema_version"`
	Source        string     `json:"source"`
	Network       string     `json:"network"`
	FromLedger    uint32     `json:"from_ledger"`
	ToLedger      uint32     `json:"to_ledger"`
	Snapshots     []Snapshot `json:"snapshots"`
}
type ReplayIssue struct {
	Code       string `json:"code"`
	FromLedger uint32 `json:"from_ledger"`
	ToLedger   uint32 `json:"to_ledger"`
}
type ReplayEvent struct {
	ID      string  `json:"id"`
	Ledger  uint32  `json:"ledger"`
	Finding Finding `json:"finding"`
}
type ReplayReport struct {
	SchemaVersion    int           `json:"schema_version"`
	Network          string        `json:"network"`
	Source           string        `json:"source"`
	SequenceCoverage string        `json:"sequence_coverage"`
	EvaluationStatus string        `json:"evaluation_status"`
	ProcessedThrough uint32        `json:"processed_through"`
	Events           []ReplayEvent `json:"events"`
	Issues           []ReplayIssue `json:"issues"`
}

func LoadJournal(path string) (Journal, error) {
	var j Journal
	err := readStrict(path, &j)
	return j, err
}

// ReplayJournal compares ordered per-ledger snapshots, not transaction deltas.
// It stops at the first missing or conflicting observation, retaining only
// the contiguous prefix. No database checkpoint or chain completeness is implied.
func ReplayJournal(j Journal) (ReplayReport, error) {
	out := ReplayReport{SchemaVersion: 1, Network: j.Network, Source: j.Source, SequenceCoverage: "PARTIAL", EvaluationStatus: "UNVERIFIED_INPUT", Events: []ReplayEvent{}, Issues: []ReplayIssue{}}
	if j.SchemaVersion != 1 || (j.Source != "synthetic" && j.Source != "user_snapshot") || !validText(j.Network, MaxIdentifierBytes) || j.FromLedger == 0 || j.ToLedger < j.FromLedger || uint64(j.ToLedger)-uint64(j.FromLedger) >= MaxJournalSnapshots || len(j.Snapshots) > MaxJournalSnapshots {
		return out, errors.New("invalid or oversized replay journal")
	}
	// Validate the complete input before emitting any events.
	var previousLedger uint32
	for i, s := range j.Snapshots {
		if err := s.Validate(); err != nil {
			return out, fmt.Errorf("snapshots[%d]: %w", i, err)
		}
		if s.Network != j.Network || s.Ledger < j.FromLedger || s.Ledger > j.ToLedger || (i > 0 && s.Ledger < previousLedger) {
			return out, errors.New("journal contains wrong network, out-of-range or out-of-order snapshot")
		}
		previousLedger = s.Ledger
	}
	expected := uint64(j.FromLedger)
	var before *Snapshot
	var lastDigest string
	for _, input := range j.Snapshots {
		s := canonicalSnapshot(input)
		digest := snapshotDigest(s)
		if before != nil && s.Ledger == before.Ledger {
			if digest != lastDigest {
				if s.Ledger == j.FromLedger {
					out.ProcessedThrough = 0
				} else {
					out.ProcessedThrough = s.Ledger - 1
				}
				for len(out.Events) > 0 && out.Events[len(out.Events)-1].Ledger == s.Ledger {
					out.Events = out.Events[:len(out.Events)-1]
				}
				out.Issues = append(out.Issues, ReplayIssue{Code: "SOURCE_DIVERGENCE", FromLedger: s.Ledger, ToLedger: s.Ledger})
				return out, nil
			}
			continue // Byte-equivalent state is idempotent, independent of list order.
		}
		if uint64(s.Ledger) != expected {
			out.Issues = append(out.Issues, ReplayIssue{Code: "OBSERVATION_GAP", FromLedger: uint32(expected), ToLedger: s.Ledger - 1})
			return out, nil
		}
		if before != nil {
			report, err := Compare(*before, s, Policy{SchemaVersion: 1})
			if err != nil {
				return out, err
			}
			for _, f := range report.Findings {
				if len(out.Events) >= MaxRecords {
					return out, errors.New("replay exceeds event limit")
				}
				// Identity binds the observation pair, source kind and complete finding.
				// No wall clock enters identity; replaying the journal gives the same IDs.
				payload, _ := json.Marshal(struct {
					Domain, Source, Network, Before, After string
					Ledger                                 uint32
					Finding                                Finding
				}{"fleetwatch.snapshot-event.v1", j.Source, j.Network, lastDigest, digest, s.Ledger, f})
				sum := sha256.Sum256(payload)
				out.Events = append(out.Events, ReplayEvent{ID: hex.EncodeToString(sum[:]), Ledger: s.Ledger, Finding: f})
			}
		}
		out.ProcessedThrough = s.Ledger
		before = &s
		lastDigest = digest
		expected = uint64(s.Ledger) + 1
	}
	if expected <= uint64(j.ToLedger) {
		out.Issues = append(out.Issues, ReplayIssue{Code: "OBSERVATION_GAP", FromLedger: uint32(expected), ToLedger: j.ToLedger})
	} else {
		out.SequenceCoverage = "COMPLETE_INPUT_SEQUENCE"
	}
	return out, nil
}
func canonicalSnapshot(input Snapshot) Snapshot {
	s := input
	s.Contracts = append([]Contract{}, input.Contracts...)
	s.References = append([]Reference{}, input.References...)
	for i := range s.Contracts {
		s.Contracts[i].Executable.TagHex = strings.ToLower(s.Contracts[i].Executable.TagHex)
		s.Contracts[i].Executable.WasmHash = strings.ToLower(s.Contracts[i].Executable.WasmHash)
	}
	for i := range s.References {
		s.References[i].TagHex = strings.ToLower(s.References[i].TagHex)
		s.References[i].WasmHash = strings.ToLower(s.References[i].WasmHash)
	}
	sort.Slice(s.Contracts, func(i, j int) bool { return s.Contracts[i].ID < s.Contracts[j].ID })
	sort.Slice(s.References, func(i, j int) bool {
		return refKey(s.References[i].Owner, s.References[i].TagHex) < refKey(s.References[j].Owner, s.References[j].TagHex)
	})
	return s
}
func snapshotDigest(s Snapshot) string {
	data, _ := json.Marshal(s)
	sum := sha256.Sum256(data)
	return hex.EncodeToString(sum[:])
}

// SnapshotDigest returns the canonical v1 observation digest used by replay.
func SnapshotDigest(s Snapshot) (string, error) {
	if err := s.Validate(); err != nil {
		return "", err
	}
	return snapshotDigest(canonicalSnapshot(s)), nil
}
