package coordinator

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"sort"
	"sync/atomic"
	"time"

	"github.com/arosenfeld2003/qwasar_eng_labs_events/internal/broker"
	"github.com/arosenfeld2003/qwasar_eng_labs_events/internal/event"
	"github.com/arosenfeld2003/qwasar_eng_labs_events/internal/organizer"
)

// Stats holds coordinator processing counters.
type Stats struct {
	Accepted int64
	Rejected int64
}

// Coordinator validates incoming events and publishes valid ones
// to the events.validated queue for the organizer to consume.
type Coordinator struct {
	broker   broker.Broker
	accepted atomic.Int64
	rejected atomic.Int64
}

// New creates a new Coordinator backed by the given broker.
func New(b broker.Broker) *Coordinator {
	return &Coordinator{broker: b}
}

// Setup declares the validated queue used by the organizer.
func (c *Coordinator) Setup(ctx context.Context) error {
	if _, err := c.broker.DeclareQueue(ctx, organizer.QueueValidated, broker.QueueOptions{Durable: true}); err != nil {
		return fmt.Errorf("declare validated queue: %w", err)
	}
	return nil
}

// parseTimestamp converts a wedding timestamp ("HH:MM") to a simulation duration.
// The wedding is 6 hours long; 1 wedding minute = 1 real second at speed 1.0.
// e.g. "05:37" → 5h37m = 337 minutes = 337 seconds simulation time.
func parseTimestamp(ts string) (time.Duration, error) {
	var h, m int
	if _, err := fmt.Sscanf(ts, "%d:%d", &h, &m); err != nil {
		return 0, fmt.Errorf("invalid timestamp %q: %w", ts, err)
	}
	return time.Duration(h*60+m) * time.Second, nil
}

// Ingest validates events and publishes them to the validated queue according
// to their wedding timestamp, compressed by the speed multiplier.
// Events with unparseable timestamps are published immediately.
func (c *Coordinator) Ingest(ctx context.Context, events []event.Event, speed float64) error {
	if speed <= 0 {
		speed = 1.0
	}

	// Sort by timestamp so we can schedule them in order.
	sorted := make([]event.Event, len(events))
	copy(sorted, events)
	sort.Slice(sorted, func(i, j int) bool {
		ti, errI := parseTimestamp(sorted[i].Timestamp)
		tj, errJ := parseTimestamp(sorted[j].Timestamp)
		if errI != nil || errJ != nil {
			return false
		}
		return ti < tj
	})

	start := time.Now()

	for i := range sorted {
		ev := &sorted[i]

		if err := ev.Validate(); err != nil {
			log.Printf("coordinator: rejected event %d: %v", ev.ID, err)
			c.rejected.Add(1)
			continue
		}

		// Wait until the event's scheduled simulation time.
		if simDelay, err := parseTimestamp(ev.Timestamp); err == nil {
			realDelay := time.Duration(float64(simDelay) / speed)
			elapsed := time.Since(start)
			if remaining := realDelay - elapsed; remaining > 0 {
				select {
				case <-time.After(remaining):
				case <-ctx.Done():
					return ctx.Err()
				}
			}
		}

		data, err := json.Marshal(ev)
		if err != nil {
			log.Printf("coordinator: marshal error for event %d: %v", ev.ID, err)
			c.rejected.Add(1)
			continue
		}

		if err := c.broker.Publish(ctx, broker.Message{
			Body:        data,
			ContentType: "application/json",
		}, broker.PublishOptions{
			RoutingKey: organizer.QueueValidated,
		}); err != nil {
			return fmt.Errorf("publish event %d: %w", ev.ID, err)
		}

		c.accepted.Add(1)
	}
	return nil
}

// Stats returns a snapshot of the coordinator's counters.
func (c *Coordinator) Stats() Stats {
	return Stats{
		Accepted: c.accepted.Load(),
		Rejected: c.rejected.Load(),
	}
}
