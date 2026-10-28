package store

import (
	"bytes"
	"encoding/json"
	"errors"
	"os"
	"time"
	"unicode/utf8"

	"github.com/soroban-fleetwatch/fleetwatch/internal/fleetwatch"
	"github.com/soroban-fleetwatch/fleetwatch/internal/stateformat"
	bolt "go.etcd.io/bbolt"
)

// OpenReadOnly requires an existing recognized database and never initializes it.
// bbolt holds a shared file lock: an active writer may prevent this open.
func OpenReadOnly(path string) (*Store, error) {
	return openReadOnly(path, false)
}

// OpenBackupReadOnly recognizes either engine for transaction-consistent
// copying only. It does not make legacy replay/health APIs adopt follow state.
func OpenBackupReadOnly(path string) (*Store, error) { return openReadOnly(path, true) }

func openReadOnly(path string, allowFollow bool) (*Store, error) {
	info, err := os.Lstat(path)
	if err != nil {
		return nil, err
	}
	if !info.Mode().IsRegular() || info.Size() == 0 {
		return nil, errors.New("state path must be an existing nonempty regular database")
	}
	db, err := bolt.Open(path, 0600, &bolt.Options{ReadOnly: true, Timeout: time.Second})
	if err != nil {
		return nil, err
	}
	if err := db.View(func(tx *bolt.Tx) error {
		if tx.Bucket(stateBucket) == nil && !(allowFollow && tx.Bucket([]byte(stateformat.Follow)) != nil) {
			return errors.New("unrecognized replay database")
		}
		return nil
	}); err != nil {
		db.Close()
		return nil, err
	}
	return &Store{db: db}, nil
}

type OutboxHealth struct {
	Total              int `json:"total"`
	Pending            int `json:"pending"`
	Due                int `json:"due"`
	Scheduled          int `json:"scheduled"`
	Inflight           int `json:"inflight"`
	ExpiredLeases      int `json:"expired_leases"`
	Delivered          int `json:"delivered"`
	Dead               int `json:"dead"`
	ExhaustedUnsettled int `json:"exhausted_unsettled"`
}

type Health struct {
	SchemaVersion    int                      `json:"schema_version"`
	Network          string                   `json:"network"`
	Source           string                   `json:"source"`
	CheckedAt        string                   `json:"checked_at"`
	Status           string                   `json:"status"`
	Reasons          []string                 `json:"reasons"`
	StateFound       bool                     `json:"state_found"`
	ProcessedThrough uint32                   `json:"processed_through"`
	SequenceCoverage string                   `json:"sequence_coverage"`
	EvaluationStatus string                   `json:"evaluation_status"`
	ChainHistory     string                   `json:"chain_history"`
	FinalVerdict     string                   `json:"final_verdict"`
	LedgerLag        *uint32                  `json:"ledger_lag"`
	Events           int                      `json:"events"`
	Issues           []fleetwatch.ReplayIssue `json:"issues"`
	Outbox           OutboxHealth             `json:"outbox"`
}

// Health inspects one consistent read transaction; it does not claim leases,
// contact sources, or infer source freshness from an offline input sequence.
func (s *Store) Health(network, source string, now time.Time) (Health, error) {
	out := Health{SchemaVersion: 1, Network: network, Source: source,
		CheckedAt: now.UTC().Format(time.RFC3339Nano), Status: "NO_STATE",
		Reasons: []string{}, SequenceCoverage: "UNKNOWN", EvaluationStatus: "UNVERIFIED_INPUT",
		ChainHistory: "UNKNOWN", FinalVerdict: "UNDETERMINED", Issues: []fleetwatch.ReplayIssue{}}
	if network == "" || len(network) > fleetwatch.MaxIdentifierBytes || !utf8.ValidString(network) || (source != "synthetic" && source != "user_snapshot") || now.Unix() <= 0 {
		return Health{}, errors.New("invalid health scope or clock")
	}
	err := s.db.View(func(tx *bolt.Tx) error {
		root := tx.Bucket(stateBucket)
		if root == nil {
			return errors.New("missing replay namespace")
		}
		var state State
		raw := root.Get(scope(network, source))
		if raw != nil {
			if err := decodeState(raw, &state); err != nil {
				return err
			}
			if state.Journal.Network != network || state.Journal.Source != source {
				return errors.New("stored replay scope mismatch")
			}
			if err := validateHealthCoverage(state); err != nil {
				return err
			}
			out.StateFound = true
			out.ProcessedThrough = state.Report.ProcessedThrough
			out.SequenceCoverage = state.Report.SequenceCoverage
			out.Events = len(state.Report.Events)
			out.Issues = append(out.Issues, state.Report.Issues...)
			out.Status = "OK_OFFLINE"
		}
		b, err := queue(tx, network, source, false)
		if err != nil {
			return err
		}
		expected := make(map[string][]byte, len(state.Report.Events))
		for _, event := range state.Report.Events {
			expected[event.ID], _ = json.Marshal(event)
		}
		issuesFound := make(map[string]bool)
		if b != nil {
			if err := b.ForEach(func(key, data []byte) error {
				if out.Outbox.Total >= fleetwatch.MaxRecords || !out.StateFound {
					return errors.New("outbox exceeds limit or lacks replay state")
				}
				a, err := decodeAlert(data, string(key), network, source)
				if err != nil {
					return err
				}
				if a.Payload.Event != nil {
					actual, _ := json.Marshal(a.Payload.Event)
					if !bytes.Equal(expected[a.ID], actual) {
						return errors.New("outbox event differs from retained replay")
					}
					delete(expected, a.ID)
				} else {
					i := *a.Payload.Issue
					if i.FromLedger < state.Journal.FromLedger || i.ToLedger < i.FromLedger || uint64(i.ToLedger)-uint64(state.Journal.FromLedger) >= fleetwatch.MaxJournalSnapshots ||
						(i.Code != "OBSERVATION_GAP" && !(i.Code == "SOURCE_DIVERGENCE" && i.FromLedger == i.ToLedger)) || a.ID != issueAlertID(network, source, i) {
						return errors.New("invalid retained issue alert")
					}
					issuesFound[a.ID] = true
				}
				countOutboxHealth(&out.Outbox, a, now)
				return nil
			}); err != nil {
				return err
			}
		}
		if len(expected) != 0 {
			return errors.New("retained replay event missing from outbox")
		}
		for _, issue := range state.Report.Issues {
			if !issuesFound[issueAlertID(network, source, issue)] {
				return errors.New("current replay issue missing from outbox")
			}
		}
		return nil
	})
	if err != nil {
		return Health{}, err // never return a partial success report after corruption
	}
	if !out.StateFound {
		out.Reasons = append(out.Reasons, "NO_REPLAY_STATE")
		return out, nil
	}
	for _, check := range []struct {
		failed bool
		code   string
	}{
		{out.SequenceCoverage != "COMPLETE_INPUT_SEQUENCE", "INPUT_SEQUENCE_INCOMPLETE"},
		{out.Outbox.Dead > 0, "DELIVERY_DEAD_LETTERS"},
		{out.Outbox.ExpiredLeases > 0, "DELIVERY_EXPIRED_LEASES"},
		{out.Outbox.ExhaustedUnsettled > 0, "DELIVERY_ATTEMPTS_EXHAUSTED"},
	} {
		if check.failed {
			out.Status = "NEEDS_ATTENTION"
			out.Reasons = append(out.Reasons, check.code)
		}
	}
	return out, nil
}

// The stored report can describe a failed extension beyond the retained prefix.
// Such a diagnostic must start immediately after the cursor (or at baseline).
func validateHealthCoverage(s State) error {
	r := s.Report
	if r.SequenceCoverage == "COMPLETE_INPUT_SEQUENCE" && len(r.Issues) == 0 && r.ProcessedThrough == s.Journal.ToLedger {
		return nil
	}
	if r.SequenceCoverage == "PARTIAL" && len(r.Issues) == 1 {
		issue := r.Issues[0]
		from := uint64(r.ProcessedThrough) + 1
		if r.ProcessedThrough == 0 {
			from = uint64(s.Journal.FromLedger)
		}
		if uint64(issue.FromLedger) == from && issue.ToLedger >= issue.FromLedger && uint64(issue.ToLedger)-uint64(s.Journal.FromLedger) < fleetwatch.MaxJournalSnapshots &&
			(issue.Code == "OBSERVATION_GAP" || (issue.Code == "SOURCE_DIVERGENCE" && issue.FromLedger == issue.ToLedger)) {
			return nil
		}
	}
	return errors.New("invalid stored sequence coverage diagnostic")
}
