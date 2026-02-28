package coordinator

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"sync/atomic"

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

// Ingest validates and publishes a batch of events to the validated queue.
// Invalid events are logged and skipped.
func (c *Coordinator) Ingest(ctx context.Context, events []event.Event) error {
	for i := range events {
		ev := &events[i]

		if err := ev.Validate(); err != nil {
			log.Printf("coordinator: rejected event %d: %v", ev.ID, err)
			c.rejected.Add(1)
			continue
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
