package coordinator

import (
	"context"
	"sync"
	"time"

	"github.com/rs/zerolog/log"

	"marry-me/internal/broker"
	"marry-me/internal/config"
	"marry-me/internal/event"
)

// Coordinator validates incoming events and forwards them for processing
type Coordinator struct {
	broker   broker.Broker
	cfg      *config.Config
	stats    Stats
	mu       sync.RWMutex
	stopCh   chan struct{}
	wg       sync.WaitGroup
}

// Stats holds coordinator statistics
type Stats struct {
	EventsReceived  int64
	EventsValidated int64
	EventsRejected  int64
}

// New creates a new Coordinator instance
func New(b broker.Broker, cfg *config.Config) *Coordinator {
	return &Coordinator{
		broker: b,
		cfg:    cfg,
		stopCh: make(chan struct{}),
	}
}

// Start begins processing incoming events
func (c *Coordinator) Start(ctx context.Context) error {
	log.Info().Msg("Coordinator starting...")

	// Subscribe to incoming events queue
	err := c.broker.Subscribe(ctx, c.cfg.IncomingQueue, c.handleEvent)
	if err != nil {
		return err
	}

	c.wg.Add(1)
	go func() {
		defer c.wg.Done()
		select {
		case <-ctx.Done():
			log.Info().Msg("Coordinator stopping due to context cancellation")
		case <-c.stopCh:
			log.Info().Msg("Coordinator stopping due to stop signal")
		}
	}()

	log.Info().
		Str("queue", c.cfg.IncomingQueue).
		Msg("Coordinator started")

	return nil
}

// Stop gracefully stops the coordinator
func (c *Coordinator) Stop() {
	close(c.stopCh)
	c.wg.Wait()
	log.Info().Msg("Coordinator stopped")
}

// handleEvent processes a single incoming event
func (c *Coordinator) handleEvent(ctx context.Context, e *event.Event) error {
	c.mu.Lock()
	c.stats.EventsReceived++
	c.mu.Unlock()

	// Set received timestamp
	e.ReceivedAt = time.Now()

	// Validate event type
	if !event.IsValidEventType(e.Type) {
		c.mu.Lock()
		c.stats.EventsRejected++
		c.mu.Unlock()

		log.Warn().
			Str("event_id", e.ID).
			Str("event_type", string(e.Type)).
			Msg("Invalid event type - rejected")

		return nil // Don't requeue invalid events
	}

	// Check if event can be handled by any team
	teams := event.GetTeamsForEvent(e.Type)
	if len(teams) == 0 {
		c.mu.Lock()
		c.stats.EventsRejected++
		c.mu.Unlock()

		log.Warn().
			Str("event_id", e.ID).
			Str("event_type", string(e.Type)).
			Msg("No team can handle event - rejected")

		return nil
	}

	// Set deadline based on priority
	e.SetDeadline()

	// Publish to validated queue
	err := c.broker.Publish(ctx, c.cfg.ValidatedQueue, e)
	if err != nil {
		log.Error().
			Err(err).
			Str("event_id", e.ID).
			Msg("Failed to publish validated event")
		return err
	}

	c.mu.Lock()
	c.stats.EventsValidated++
	c.mu.Unlock()

	log.Debug().
		Str("event_id", e.ID).
		Str("event_type", string(e.Type)).
		Str("priority", e.Priority.String()).
		Time("deadline", e.Deadline).
		Msg("Event validated and forwarded")

	return nil
}

// InjectEvent directly injects an event into the system (for simulation)
func (c *Coordinator) InjectEvent(ctx context.Context, e *event.Event) error {
	return c.handleEvent(ctx, e)
}

// InjectEvents injects multiple events (for batch simulation)
func (c *Coordinator) InjectEvents(ctx context.Context, events []*event.Event) error {
	for _, e := range events {
		if err := c.InjectEvent(ctx, e); err != nil {
			return err
		}
	}
	return nil
}

// GetStats returns a copy of the current statistics
func (c *Coordinator) GetStats() Stats {
	c.mu.RLock()
	defer c.mu.RUnlock()
	return c.stats
}
