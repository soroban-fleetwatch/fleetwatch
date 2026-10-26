// Package alert dispatches persisted offline findings. Delivery is at least
// once: receivers must deduplicate by alert ID after ambiguous acknowledgments.
package alert

import (
	"context"
	"errors"
	"time"

	"github.com/soroban-fleetwatch/fleetwatch/internal/store"
)

type Sender interface {
	Send(context.Context, store.Alert) error
}
type Queue interface {
	Claim(string, string, string, time.Time, time.Duration) (*store.Alert, error)
	Resolve(string, string, string, string, bool, bool, time.Time) error
}

type contextQueue interface {
	ClaimContext(context.Context, string, string, string, time.Time, time.Duration) (*store.Alert, error)
}
type PermanentError struct{}

func (PermanentError) Error() string { return "permanent delivery failure" }

type Result struct {
	Attempted int `json:"attempted"`
	Delivered int `json:"delivered"`
	Failed    int `json:"failed"`
}

// Dispatch runs one bounded batch. It never embeds transport errors or secrets
// in durable state. A failed acknowledgement leaves a lease for recovery.
func Dispatch(ctx context.Context, q Queue, s Sender, network, source, sink string, limit int) (Result, error) {
	var result Result
	if limit < 1 || limit > 50 {
		return result, errors.New("delivery batch requires 1-50 alerts")
	}
	for i := 0; i < limit; i++ {
		if err := ctx.Err(); err != nil {
			return result, err
		}
		var item *store.Alert
		var err error
		if queue, ok := q.(contextQueue); ok {
			item, err = queue.ClaimContext(ctx, network, source, sink, time.Now(), 30*time.Second)
		} else {
			item, err = q.Claim(network, source, sink, time.Now(), 30*time.Second)
		}
		if err != nil {
			return result, err
		}
		if item == nil {
			return result, nil
		}
		sendCtx, cancel := context.WithTimeout(ctx, 15*time.Second)
		sendErr := s.Send(sendCtx, *item)
		cancel()
		result.Attempted++
		permanent := false
		var terminal PermanentError
		if errors.As(sendErr, &terminal) {
			permanent = true
		}
		if err := q.Resolve(network, source, item.ID, item.LeaseToken, sendErr == nil, permanent, time.Now()); err != nil {
			return result, err
		}
		if sendErr == nil {
			result.Delivered++
		} else {
			result.Failed++
		}
	}
	return result, nil
}
