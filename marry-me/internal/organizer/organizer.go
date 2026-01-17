package organizer

import (
	"context"
	"sync"
	"time"

	"github.com/rs/zerolog/log"

	"marry-me/internal/broker"
	"marry-me/internal/config"
	"marry-me/internal/event"
)

// Organizer routes validated events to appropriate team queues
type Organizer struct {
	broker          broker.Broker
	cfg             *config.Config
	stats           Stats
	mu              sync.RWMutex
	stopCh          chan struct{}
	wg              sync.WaitGroup
	deadlineChecker *time.Ticker
}

// Stats holds organizer statistics
type Stats struct {
	EventsRouted    int64
	EventsExpired   int64
	MultiTeamEvents int64
}

// New creates a new Organizer instance
func New(b broker.Broker, cfg *config.Config) *Organizer {
	return &Organizer{
		broker: b,
		cfg:    cfg,
		stopCh: make(chan struct{}),
	}
}

// Start begins routing validated events
func (o *Organizer) Start(ctx context.Context) error {
	log.Info().Msg("Organizer starting...")

	// Subscribe to validated events queue
	err := o.broker.Subscribe(ctx, o.cfg.ValidatedQueue, o.handleEvent)
	if err != nil {
		return err
	}

	o.wg.Add(1)
	go func() {
		defer o.wg.Done()
		select {
		case <-ctx.Done():
			log.Info().Msg("Organizer stopping due to context cancellation")
		case <-o.stopCh:
			log.Info().Msg("Organizer stopping due to stop signal")
		}
	}()

	log.Info().
		Str("queue", o.cfg.ValidatedQueue).
		Msg("Organizer started")

	return nil
}

// Stop gracefully stops the organizer
func (o *Organizer) Stop() {
	close(o.stopCh)
	if o.deadlineChecker != nil {
		o.deadlineChecker.Stop()
	}
	o.wg.Wait()
	log.Info().Msg("Organizer stopped")
}

// handleEvent processes a validated event and routes it to appropriate team(s)
func (o *Organizer) handleEvent(ctx context.Context, e *event.Event) error {
	// Check if already expired
	if e.IsExpired() {
		o.handleExpiredEvent(ctx, e)
		return nil
	}

	// Get teams that can handle this event type
	teams := event.GetTeamsForEvent(e.Type)
	if len(teams) == 0 {
		log.Warn().
			Str("event_id", e.ID).
			Str("event_type", string(e.Type)).
			Msg("No teams found for event type")
		return nil
	}

	// Track multi-team events
	if len(teams) > 1 {
		o.mu.Lock()
		o.stats.MultiTeamEvents++
		o.mu.Unlock()
	}

	// Route to primary team (first in list)
	// In a more sophisticated implementation, we could route to multiple teams
	// or use load balancing
	primaryTeam := teams[0]
	teamQueue := config.GetTeamQueue(string(primaryTeam))

	err := o.broker.Publish(ctx, teamQueue, e)
	if err != nil {
		log.Error().
			Err(err).
			Str("event_id", e.ID).
			Str("team", string(primaryTeam)).
			Msg("Failed to route event to team")
		return err
	}

	o.mu.Lock()
	o.stats.EventsRouted++
	o.mu.Unlock()

	log.Debug().
		Str("event_id", e.ID).
		Str("event_type", string(e.Type)).
		Str("team", string(primaryTeam)).
		Str("priority", e.Priority.String()).
		Msg("Event routed to team")

	return nil
}

// handleExpiredEvent handles events that have passed their deadline
func (o *Organizer) handleExpiredEvent(ctx context.Context, e *event.Event) {
	o.mu.Lock()
	o.stats.EventsExpired++
	o.mu.Unlock()

	e.Status = event.StatusExpired

	// Publish to results queue for stress tracking
	err := o.broker.Publish(ctx, o.cfg.ResultsQueue, e)
	if err != nil {
		log.Error().
			Err(err).
			Str("event_id", e.ID).
			Msg("Failed to publish expired event to results")
	}

	log.Debug().
		Str("event_id", e.ID).
		Str("event_type", string(e.Type)).
		Str("priority", e.Priority.String()).
		Msg("Event expired before processing")
}

// GetStats returns a copy of the current statistics
func (o *Organizer) GetStats() Stats {
	o.mu.RLock()
	defer o.mu.RUnlock()
	return o.stats
}
