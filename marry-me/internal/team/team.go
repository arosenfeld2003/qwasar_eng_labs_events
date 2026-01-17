package team

import (
	"context"
	"fmt"
	"sync"
	"time"

	"github.com/rs/zerolog/log"

	"marry-me/internal/broker"
	"marry-me/internal/config"
	"marry-me/internal/event"
	"marry-me/internal/organizer"
	"marry-me/internal/worker"
)

// Team manages a pool of workers for a specific team
type Team struct {
	name        event.TeamName
	broker      broker.Broker
	cfg         *config.Config
	queue       *organizer.PriorityQueue
	workers     []worker.Worker
	stats       Stats
	mu          sync.RWMutex
	stopCh      chan struct{}
	wg          sync.WaitGroup
	eventCh     chan *event.Event
}

// Stats holds team statistics
type Stats struct {
	EventsReceived  int64
	EventsCompleted int64
	EventsExpired   int64
	WorkersActive   int64
}

// New creates a new Team instance
func New(name event.TeamName, b broker.Broker, cfg *config.Config) *Team {
	return &Team{
		name:    name,
		broker:  b,
		cfg:     cfg,
		queue:   organizer.NewPriorityQueue(),
		stopCh:  make(chan struct{}),
		eventCh: make(chan *event.Event, 100),
	}
}

// Name returns the team name
func (t *Team) Name() event.TeamName {
	return t.name
}

// Start begins the team's event processing
func (t *Team) Start(ctx context.Context) error {
	log.Info().
		Str("team", string(t.name)).
		Int("workers", t.cfg.WorkersPerTeam).
		Msg("Team starting...")

	// Create workers
	for i := 0; i < t.cfg.WorkersPerTeam; i++ {
		workerID := fmt.Sprintf("%s-worker-%d", t.name, i)
		routineType := t.getRoutineType(i)
		w := worker.New(workerID, t, routineType, t.cfg)
		t.workers = append(t.workers, w)
	}

	// Start workers
	for _, w := range t.workers {
		if err := w.Start(ctx); err != nil {
			return fmt.Errorf("failed to start worker %s: %w", w.ID(), err)
		}
	}

	// Subscribe to team queue
	queueName := config.GetTeamQueue(string(t.name))
	err := t.broker.Subscribe(ctx, queueName, t.handleIncomingEvent)
	if err != nil {
		return fmt.Errorf("failed to subscribe to queue %s: %w", queueName, err)
	}

	// Start deadline checker
	t.wg.Add(1)
	go t.checkDeadlines(ctx)

	// Start event dispatcher
	t.wg.Add(1)
	go t.dispatchEvents(ctx)

	log.Info().
		Str("team", string(t.name)).
		Str("queue", queueName).
		Msg("Team started")

	return nil
}

// Stop gracefully stops the team
func (t *Team) Stop() {
	close(t.stopCh)

	// Stop all workers
	for _, w := range t.workers {
		w.Stop()
	}

	t.wg.Wait()
	log.Info().
		Str("team", string(t.name)).
		Msg("Team stopped")
}

// handleIncomingEvent receives events from the broker
func (t *Team) handleIncomingEvent(ctx context.Context, e *event.Event) error {
	t.mu.Lock()
	t.stats.EventsReceived++
	t.mu.Unlock()

	// Check if already expired
	if e.IsExpired() {
		t.handleExpiredEvent(ctx, e)
		return nil
	}

	// Add to priority queue
	t.queue.Push(e)

	log.Debug().
		Str("team", string(t.name)).
		Str("event_id", e.ID).
		Int("queue_size", t.queue.Len()).
		Msg("Event added to team queue")

	return nil
}

// RequestEvent is called by workers to get the next event
func (t *Team) RequestEvent() (*event.Event, bool) {
	e, ok := t.queue.Pop()
	if !ok {
		return nil, false
	}

	// Double-check expiration
	if e.IsExpired() {
		go func() {
			ctx := context.Background()
			t.handleExpiredEvent(ctx, e)
		}()
		return nil, false
	}

	e.Status = event.StatusProcessing
	return e, true
}

// ReportCompletion is called by workers when they finish processing
func (t *Team) ReportCompletion(ctx context.Context, e *event.Event, success bool) {
	if success {
		t.mu.Lock()
		t.stats.EventsCompleted++
		t.mu.Unlock()

		e.Status = event.StatusCompleted
	} else {
		e.Status = event.StatusExpired
	}

	// Publish to results queue
	err := t.broker.Publish(ctx, t.cfg.ResultsQueue, e)
	if err != nil {
		log.Error().
			Err(err).
			Str("team", string(t.name)).
			Str("event_id", e.ID).
			Msg("Failed to publish completion result")
	}

	log.Debug().
		Str("team", string(t.name)).
		Str("event_id", e.ID).
		Str("status", e.Status.String()).
		Msg("Event processing reported")
}

// handleExpiredEvent handles events that have passed their deadline
func (t *Team) handleExpiredEvent(ctx context.Context, e *event.Event) {
	t.mu.Lock()
	t.stats.EventsExpired++
	t.mu.Unlock()

	e.Status = event.StatusExpired

	err := t.broker.Publish(ctx, t.cfg.ResultsQueue, e)
	if err != nil {
		log.Error().
			Err(err).
			Str("team", string(t.name)).
			Str("event_id", e.ID).
			Msg("Failed to publish expired event")
	}

	log.Debug().
		Str("team", string(t.name)).
		Str("event_id", e.ID).
		Msg("Event expired in team queue")
}

// checkDeadlines periodically removes expired events from the queue
func (t *Team) checkDeadlines(ctx context.Context) {
	defer t.wg.Done()

	ticker := time.NewTicker(1 * time.Second)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return
		case <-t.stopCh:
			return
		case <-ticker.C:
			expired := t.queue.RemoveExpired()
			for _, e := range expired {
				t.handleExpiredEvent(ctx, e)
			}
		}
	}
}

// dispatchEvents notifies workers when events are available
func (t *Team) dispatchEvents(ctx context.Context) {
	defer t.wg.Done()

	ticker := time.NewTicker(100 * time.Millisecond)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return
		case <-t.stopCh:
			return
		case <-ticker.C:
			// Workers pull events on their own schedule
			// This goroutine is mainly for housekeeping
		}
	}
}

// getRoutineType assigns a routine type to a worker based on index
func (t *Team) getRoutineType(index int) config.RoutineType {
	// Distribute routine types among workers
	switch index % 3 {
	case 0:
		return config.RoutineStandard
	case 1:
		return config.RoutineIntermittent
	default:
		return config.RoutineConcentrated
	}
}

// GetStats returns a copy of the current statistics
func (t *Team) GetStats() Stats {
	t.mu.RLock()
	defer t.mu.RUnlock()

	stats := t.stats
	stats.WorkersActive = int64(t.countActiveWorkers())
	return stats
}

// countActiveWorkers counts workers currently processing events
func (t *Team) countActiveWorkers() int {
	count := 0
	for _, w := range t.workers {
		if !w.IsIdle() {
			count++
		}
	}
	return count
}

// QueueSize returns the current queue size
func (t *Team) QueueSize() int {
	return t.queue.Len()
}
