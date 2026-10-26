package alert

import (
	"context"
	"errors"
	"path/filepath"
	"testing"
	"time"

	"github.com/soroban-fleetwatch/fleetwatch/internal/fleetwatch"
	"github.com/soroban-fleetwatch/fleetwatch/internal/store"
)

type senderFunc func(context.Context, store.Alert) error

func (f senderFunc) Send(ctx context.Context, a store.Alert) error { return f(ctx, a) }
func prepare(t *testing.T) (*store.Store, fleetwatch.Journal) {
	t.Helper()
	s, err := store.Open(filepath.Join(t.TempDir(), "state.db"))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { s.Close() })
	j, err := fleetwatch.LoadJournal(filepath.Join("..", "..", "testdata", "replay-transient.json"))
	if err != nil {
		t.Fatal(err)
	}
	if _, err := s.Replay(j); err != nil {
		t.Fatal(err)
	}
	return s, j
}
func TestDispatchSuccessDedupAndAssurance(t *testing.T) {
	s, j := prepare(t)
	seen := map[string]bool{}
	sink := senderFunc(func(ctx context.Context, a store.Alert) error {
		if seen[a.ID] {
			t.Fatal("duplicate send after acknowledgement")
		}
		seen[a.ID] = true
		if a.Payload.EvaluationStatus != "UNVERIFIED_INPUT" {
			t.Fatal("assurance changed")
		}
		return nil
	})
	r, err := Dispatch(context.Background(), s, sink, j.Network, j.Source, "test", 50)
	if err != nil || r.Delivered != 2 {
		t.Fatalf("%+v %v", r, err)
	}
	r, err = Dispatch(context.Background(), s, sink, j.Network, j.Source, "test", 50)
	if err != nil || r.Attempted != 0 {
		t.Fatal("acknowledged alert sent again", err)
	}
}
func TestDispatchFailureIsDurableAndRedacted(t *testing.T) {
	s, j := prepare(t)
	sink := senderFunc(func(context.Context, store.Alert) error { return errors.New("credential-must-not-be-saved") })
	r, err := Dispatch(context.Background(), s, sink, j.Network, j.Source, "test", 50)
	if err != nil || r.Failed != 2 {
		t.Fatalf("%+v %v", r, err)
	}
	rows, _ := s.Alerts(j.Network, j.Source)
	for _, a := range rows {
		if a.Status != "PENDING" || a.LastErrorCode != "TRANSIENT_DELIVERY_FAILURE" || a.NextAttempt <= time.Now().Unix() {
			t.Fatalf("%+v", a)
		}
	}
}
func TestPermanentDeliveryFailureAndCancellation(t *testing.T) {
	s, j := prepare(t)
	sink := senderFunc(func(context.Context, store.Alert) error { return PermanentError{} })
	if r, err := Dispatch(context.Background(), s, sink, j.Network, j.Source, "test", 50); err != nil || r.Failed != 2 {
		t.Fatalf("%+v %v", r, err)
	}
	rows, _ := s.Alerts(j.Network, j.Source)
	for _, a := range rows {
		if a.Status != "DEAD" {
			t.Fatal("permanent failure retried")
		}
	}
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	if _, err := Dispatch(ctx, s, sink, j.Network, j.Source, "test", 50); !errors.Is(err, context.Canceled) {
		t.Fatal("cancellation ignored", err)
	}
}

type lostAckQueue struct{ *store.Store }

func (q lostAckQueue) Resolve(string, string, string, string, bool, bool, time.Time) error {
	return errors.New("acknowledgement interrupted")
}
func TestDeliveryWithLostAcknowledgementCanRepeatSameID(t *testing.T) {
	s, j := prepare(t)
	ids := []string{}
	sink := senderFunc(func(ctx context.Context, a store.Alert) error { ids = append(ids, a.ID); return nil })
	if _, err := Dispatch(context.Background(), lostAckQueue{s}, sink, j.Network, j.Source, "test", 1); err == nil || len(ids) != 1 {
		t.Fatal("lost acknowledgement not simulated")
	}
	reclaimed, err := s.Claim(j.Network, j.Source, "test", time.Now().Add(31*time.Second), 30*time.Second)
	if err != nil || reclaimed == nil || reclaimed.ID != ids[0] {
		t.Fatalf("wrong retry: %+v %v", reclaimed, err)
	}
	if err := sink.Send(context.Background(), *reclaimed); err != nil {
		t.Fatal(err)
	}
	if len(ids) != 2 || ids[0] != ids[1] {
		t.Fatal("ambiguous delivery lost stable idempotency key")
	}
}
