package fleetwatch

import (
	"errors"
	"sort"
	"strings"
)

type FleetQuery struct {
	Network string `json:"network"`
	Ledger  uint32 `json:"ledger"`
	Owner   string `json:"owner"`
	TagHex  string `json:"tag_hex"`
}

func (q FleetQuery) Validate() error {
	if !validText(q.Network, MaxIdentifierBytes) || q.Ledger == 0 || !validIdentifier(q.Owner) || !validTag(q.TagHex) {
		return errors.New("fleet query requires exact network, positive ledger, owner and byte-exact tag hex")
	}
	return nil
}

type FleetObservation struct {
	Ledger          uint32   `json:"ledger"`
	SnapshotDigest  string   `json:"snapshot_digest"`
	ReferenceStatus string   `json:"reference_status"`
	WasmHash        *string  `json:"wasm_hash"`
	Members         []string `json:"members"`
}

// MembershipChange describes only contracts explicitly present in adjacent
// supplied snapshots. Absence is not evidence of on-chain deletion.
type MembershipChange struct {
	Joined        []string `json:"joined"`
	Left          []string `json:"left"`
	Retained      []string `json:"retained"`
	ObservedUnion []string `json:"observed_union"`
}

type FleetReport struct {
	SchemaVersion int `json:"schema_version"`
	FleetQuery
	Source             string            `json:"source"`
	StateFound         bool              `json:"state_found"`
	SnapshotStatus     string            `json:"snapshot_status"`
	ProcessedThrough   uint32            `json:"processed_through"`
	SequenceCoverage   string            `json:"sequence_coverage"`
	InputIssues        []ReplayIssue     `json:"input_issues"`
	FleetScope         string            `json:"fleet_scope"`
	MembershipCoverage string            `json:"membership_coverage"`
	EvaluationStatus   string            `json:"evaluation_status"`
	FinalVerdict       string            `json:"final_verdict"`
	Current            *FleetObservation `json:"current"`
	Previous           *FleetObservation `json:"previous"`
	MembershipChange   *MembershipChange `json:"membership_change"`
	RelatedEvents      []ReplayEvent     `json:"related_events"`
}

// DescribeFleet is a pure query over exact supplied snapshots. No nearest-state
// interpolation, fleet discovery or source authentication is performed.
func DescribeFleet(q FleetQuery, before, after *Snapshot) (FleetReport, error) {
	if err := q.Validate(); err != nil {
		return FleetReport{}, err
	}
	q.TagHex = strings.ToLower(q.TagHex)
	out := FleetReport{SchemaVersion: 1, FleetQuery: q, SnapshotStatus: "NOT_RETAINED",
		SequenceCoverage: "UNKNOWN", InputIssues: []ReplayIssue{}, FleetScope: "OBSERVED_SNAPSHOT_MEMBERS",
		MembershipCoverage: "UNKNOWN", EvaluationStatus: "UNVERIFIED_INPUT", FinalVerdict: "UNDETERMINED", RelatedEvents: []ReplayEvent{}}
	if after == nil {
		if before != nil {
			return FleetReport{}, errors.New("previous snapshot without requested snapshot")
		}
		return out, nil
	}
	observe := func(s Snapshot) (*FleetObservation, error) {
		if err := s.Validate(); err != nil {
			return nil, err
		}
		if s.Network != q.Network {
			return nil, errors.New("fleet snapshot network mismatch")
		}
		digest, err := SnapshotDigest(s)
		if err != nil {
			return nil, err
		}
		o := &FleetObservation{Ledger: s.Ledger, SnapshotDigest: digest, ReferenceStatus: "NOT_OBSERVED", Members: []string{}}
		for _, ref := range s.References {
			if ref.Owner == q.Owner && strings.EqualFold(ref.TagHex, q.TagHex) {
				hash := strings.ToLower(ref.WasmHash)
				o.WasmHash = &hash
				o.ReferenceStatus = "OBSERVED"
			}
		}
		for _, c := range s.Contracts {
			if c.Executable.Kind == "external_ref" && c.Executable.Owner == q.Owner && strings.EqualFold(c.Executable.TagHex, q.TagHex) {
				o.Members = append(o.Members, c.ID)
			}
		}
		sort.Strings(o.Members)
		return o, nil
	}
	if after.Ledger != q.Ledger {
		return FleetReport{}, errors.New("fleet query requires exact snapshot ledger")
	}
	var err error
	out.Current, err = observe(*after)
	if err != nil {
		return FleetReport{}, err
	}
	out.SnapshotStatus = "RETAINED"
	if before == nil {
		return out, nil
	}
	if uint64(before.Ledger)+1 != uint64(q.Ledger) {
		return FleetReport{}, errors.New("fleet transition requires adjacent snapshots")
	}
	out.Previous, err = observe(*before)
	if err != nil {
		return FleetReport{}, err
	}
	change := &MembershipChange{Joined: []string{}, Left: []string{}, Retained: []string{}, ObservedUnion: []string{}}
	old, current := map[string]bool{}, map[string]bool{}
	for _, id := range out.Previous.Members {
		old[id] = true
	}
	for _, id := range out.Current.Members {
		current[id] = true
		change.ObservedUnion = append(change.ObservedUnion, id)
		if old[id] {
			change.Retained = append(change.Retained, id)
		} else {
			change.Joined = append(change.Joined, id)
		}
	}
	for _, id := range out.Previous.Members {
		if !current[id] {
			change.Left = append(change.Left, id)
			change.ObservedUnion = append(change.ObservedUnion, id)
		}
	}
	sort.Strings(change.ObservedUnion)
	out.MembershipChange = change
	return out, nil
}
