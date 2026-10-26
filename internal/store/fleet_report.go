package store

import (
	"errors"
	"strings"

	"github.com/soroban-fleetwatch/fleetwatch/internal/fleetwatch"
)

// ReportFleet reads the validated retained prefix, never an unprocessed suffix.
// Load copies its decoded state from one read transaction, keeping the snapshot
// and related findings consistent even if another goroutine later replays.
func (s *Store) ReportFleet(q fleetwatch.FleetQuery, source string) (fleetwatch.FleetReport, error) {
	if err := q.Validate(); err != nil {
		return fleetwatch.FleetReport{}, err
	}
	if source != "synthetic" && source != "user_snapshot" {
		return fleetwatch.FleetReport{}, errors.New("unsupported fleet report source")
	}
	state, found, err := s.Load(q.Network, source)
	if err != nil {
		return fleetwatch.FleetReport{}, err
	}
	var before, after *fleetwatch.Snapshot
	if found {
		if err := validateHealthCoverage(state); err != nil {
			return fleetwatch.FleetReport{}, err
		}
		for i := range state.Journal.Snapshots {
			snapshot := &state.Journal.Snapshots[i]
			if snapshot.Ledger == q.Ledger {
				after = snapshot
			}
			if uint64(snapshot.Ledger)+1 == uint64(q.Ledger) {
				before = snapshot
			}
		}
	}
	if after == nil {
		before = nil
	}
	out, err := fleetwatch.DescribeFleet(q, before, after)
	if err != nil {
		return fleetwatch.FleetReport{}, err
	}
	out.Source = source
	out.StateFound = found
	if !found {
		return out, nil
	}
	out.ProcessedThrough = state.Report.ProcessedThrough
	out.SequenceCoverage = state.Report.SequenceCoverage
	out.InputIssues = append(out.InputIssues, state.Report.Issues...)
	if after == nil {
		return out, nil
	}
	members := map[string]bool{}
	for _, o := range []*fleetwatch.FleetObservation{out.Previous, out.Current} {
		if o != nil {
			for _, id := range o.Members {
				members[id] = true
			}
		}
	}
	for _, event := range state.Report.Events {
		f := event.Finding
		if event.Ledger == q.Ledger && ((f.Owner == q.Owner && strings.EqualFold(f.TagHex, q.TagHex)) || (f.ContractID != "" && members[f.ContractID])) {
			out.RelatedEvents = append(out.RelatedEvents, event)
		}
	}
	return out, nil
}
