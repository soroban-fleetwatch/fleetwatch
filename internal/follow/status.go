package follow

import (
	"bytes"
	"context"
	"fmt"
	"net/http"
	"sync"
	"time"

	"github.com/soroban-fleetwatch/fleetwatch/internal/jsoninput"
	bolt "go.etcd.io/bbolt"
)

// Cycle is durable operational metadata, never a chain verdict. A fresh
// process starts unready and must perform its own successful source check.
type Cycle struct {
	Code          string  `json:"code"`
	At            int64   `json:"at_unix"`
	SuccessfulAt  int64   `json:"last_successful_cycle_unix"`
	Polls         uint64  `json:"polls"`
	GapEpisodes   uint64  `json:"gap_episodes"`
	EndpointHead  *uint32 `json:"endpoint_head"`
	HighWaterHead uint32  `json:"high_water_endpoint_head"`
}

var cycleCodes = map[string]bool{"STARTING": true, "IDLE": true, "FOLLOWING": true, "SOURCE_UNAVAILABLE": true, "EVIDENCE_REJECTED": true, "CAPACITY_REACHED": true, "STATE_FAILURE": true, "STOPPED": true}

func (s *Store) Cycle() (Cycle, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	out := Cycle{Code: "STARTING"}
	err := s.db.View(func(tx *bolt.Tx) error {
		raw := tx.Bucket(rootName).Get([]byte("cycle"))
		if raw == nil {
			return nil
		}
		if len(raw) > 4096 || jsoninput.Decode(bytes.NewReader(raw), &out) != nil || !cycleCodes[out.Code] || out.At < 0 || out.SuccessfulAt < 0 || out.SuccessfulAt > out.At {
			return ErrState
		}
		return nil
	})
	return out, err
}
func (s *Store) SaveCycle(ctx context.Context, c Cycle) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	if !cycleCodes[c.Code] || c.At <= 0 || c.SuccessfulAt < 0 || c.SuccessfulAt > c.At {
		return ErrState
	}
	return s.db.Update(func(tx *bolt.Tx) error {
		if ctx.Err() != nil {
			return ctx.Err()
		}
		return tx.Bucket(rootName).Put([]byte("cycle"), encode(c))
	})
}

type Status struct {
	SchemaVersion          int     `json:"schema_version"`
	Mode                   string  `json:"mode"`
	Code                   string  `json:"code"`
	Ready                  bool    `json:"ready"`
	ProcessedThrough       uint32  `json:"processed_through"`
	EndpointHead           *uint32 `json:"endpoint_head"`
	EndpointLedgerLag      *uint32 `json:"endpoint_ledger_lag"`
	RetainedLedgers        int     `json:"retained_ledgers"`
	RetainedPayloadBytes   int64   `json:"retained_payload_bytes"`
	PendingFindings        int     `json:"pending_findings"`
	Polls                  uint64  `json:"polls"`
	GapEpisodes            uint64  `json:"gap_episodes"`
	LastSuccessfulCycle    int64   `json:"last_successful_cycle_unix"`
	SourceAuthentication   string  `json:"source_authentication"`
	BaselineAuthentication string  `json:"baseline_authentication"`
	MetadataCommitment     string  `json:"metadata_commitment"`
	MembershipCoverage     string  `json:"membership_coverage"`
	HistoryCompleteness    string  `json:"history_completeness"`
	FinalVerdict           string  `json:"final_verdict"`
}

func status(head Head, c Cycle) Status {
	out := Status{SchemaVersion: 1, Mode: "EXPERIMENTAL_READ_ONLY_FOLLOW", Code: c.Code, ProcessedThrough: head.Projection.Snapshot.Ledger, EndpointHead: c.EndpointHead, RetainedLedgers: head.RetainedLedgers, RetainedPayloadBytes: head.RetainedPayloadBytes, PendingFindings: head.PendingFindings, Polls: c.Polls, GapEpisodes: c.GapEpisodes, LastSuccessfulCycle: c.SuccessfulAt, SourceAuthentication: "UNVERIFIED", BaselineAuthentication: "UNVERIFIED", MetadataCommitment: "NOT_VERIFIED", MembershipCoverage: "UNKNOWN", HistoryCompleteness: "UNKNOWN", FinalVerdict: "UNDETERMINED"}
	if c.EndpointHead != nil && *c.EndpointHead >= out.ProcessedThrough && (c.Code == "FOLLOWING" || c.Code == "IDLE") {
		n := *c.EndpointHead - out.ProcessedThrough
		out.EndpointLedgerLag = &n
	}
	return out
}

type Health struct {
	mu      sync.Mutex
	current Status
	config  Config
}

func NewHealth(head Head, cfg Config) *Health {
	return &Health{current: status(head, Cycle{Code: "STARTING"}), config: cfg}
}
func (h *Health) Update(head Head, c Cycle) {
	h.mu.Lock()
	defer h.mu.Unlock()
	h.current = status(head, c)
}
func (h *Health) Snapshot(now time.Time) Status {
	h.mu.Lock()
	defer h.mu.Unlock()
	out := h.current
	// A lag is an endpoint claim. Readiness means only that this ingestion
	// process recently checked its source and meets the configured lag bound.
	age := now.Unix() - out.LastSuccessfulCycle
	if age<0 || age>int64(30+3*h.config.PollIntervalSeconds){out.EndpointHead=nil;out.EndpointLedgerLag=nil}
	out.Ready = (out.Code == "IDLE" || out.Code == "FOLLOWING") && out.EndpointLedgerLag != nil && *out.EndpointLedgerLag <= h.config.MaxEndpointLag && age >= 0 && age <= int64(30+3*h.config.PollIntervalSeconds) && out.RetainedLedgers < h.config.MaxRetainedLedgers && out.RetainedPayloadBytes < h.config.MaxRetainedPayloadBytes && out.PendingFindings < 10000
	return out
}
func (h *Health) Handler() http.Handler {
	mux := http.NewServeMux()
	mux.HandleFunc("/livez", func(w http.ResponseWriter, r *http.Request) {
		if r.Method != "GET" {
			http.Error(w, "method not allowed", 405)
			return
		}
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte("{\"live\":true}\n"))
	})
	for _, path := range []string{"/readyz", "/status"} {
		mux.HandleFunc(path, func(w http.ResponseWriter, r *http.Request) {
			if r.Method != "GET" {
				http.Error(w, "method not allowed", 405)
				return
			}
			s := h.Snapshot(time.Now())
			w.Header().Set("Content-Type", "application/json")
			if r.URL.Path == "/readyz" && !s.Ready {
				w.WriteHeader(503)
			}
			_, _ = w.Write(append(encode(s), '\n'))
		})
	}
	mux.HandleFunc("/metrics", func(w http.ResponseWriter, r *http.Request) {
		if r.Method != "GET" {
			http.Error(w, "method not allowed", 405)
			return
		}
		s := h.Snapshot(time.Now())
		ready := 0
		if s.Ready {
			ready = 1
		}
		w.Header().Set("Content-Type", "text/plain; version=0.0.4; charset=utf-8")
		fmt.Fprintf(w, "fleetwatch_follow_ready %d\nfleetwatch_follow_processed_ledger %d\nfleetwatch_follow_retained_ledgers %d\nfleetwatch_follow_retained_payload_bytes %d\nfleetwatch_follow_pending_findings %d\nfleetwatch_follow_polls_total %d\nfleetwatch_follow_gap_episodes_total %d\nfleetwatch_follow_last_successful_cycle_unix %d\n", ready, s.ProcessedThrough, s.RetainedLedgers, s.RetainedPayloadBytes, s.PendingFindings, s.Polls, s.GapEpisodes, s.LastSuccessfulCycle)
		if s.EndpointLedgerLag != nil {
			fmt.Fprintf(w, "fleetwatch_follow_endpoint_ledger_lag %d\n", *s.EndpointLedgerLag)
		}
	})
	return mux
}
