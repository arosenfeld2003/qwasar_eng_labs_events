package worker

import (
	"context"
	"sync"
	"time"

	"github.com/rs/zerolog/log"

	"marry-me/internal/config"
	"marry-me/internal/event"
)

// EventProvider is the interface for getting events to process
type EventProvider interface {
	RequestEvent() (*event.Event, bool)
	ReportCompletion(ctx context.Context, e *event.Event, success bool)
}

// Worker defines the interface for a worker
type Worker interface {
	ID() string
	Start(ctx context.Context) error
	Stop()
	IsIdle() bool
}

// DefaultWorker implements the Worker interface
type DefaultWorker struct {
	id              string
	provider        EventProvider
	routine         *Routine
	cfg             *config.Config
	currentEvent    *event.Event
	mu              sync.RWMutex
	stopCh          chan struct{}
	wg              sync.WaitGroup
	processing      bool
}

// New creates a new worker instance
func New(id string, provider EventProvider, routineType config.RoutineType, cfg *config.Config) Worker {
	return &DefaultWorker{
		id:       id,
		provider: provider,
		routine:  NewRoutine(routineType, cfg),
		cfg:      cfg,
		stopCh:   make(chan struct{}),
	}
}

// NewWithOffset creates a worker with a staggered start offset
func NewWithOffset(id string, provider EventProvider, routineType config.RoutineType, cfg *config.Config, offset time.Duration) Worker {
	return &DefaultWorker{
		id:       id,
		provider: provider,
		routine:  NewRoutineWithOffset(routineType, cfg, offset),
		cfg:      cfg,
		stopCh:   make(chan struct{}),
	}
}

// ID returns the worker ID
func (w *DefaultWorker) ID() string {
	return w.id
}

// Start begins the worker's processing loop
func (w *DefaultWorker) Start(ctx context.Context) error {
	log.Debug().
		Str("worker_id", w.id).
		Str("routine", w.routine.Type().String()).
		Msg("Worker starting")

	w.wg.Add(1)
	go w.run(ctx)

	return nil
}

// Stop gracefully stops the worker
func (w *DefaultWorker) Stop() {
	close(w.stopCh)
	w.wg.Wait()

	log.Debug().
		Str("worker_id", w.id).
		Msg("Worker stopped")
}

// IsIdle returns true if the worker is in idle state and can accept work
func (w *DefaultWorker) IsIdle() bool {
	w.mu.RLock()
	defer w.mu.RUnlock()

	return w.routine.IsIdle() && !w.processing
}

// run is the main worker loop
func (w *DefaultWorker) run(ctx context.Context) {
	defer w.wg.Done()

	// Check for work more frequently than state changes
	ticker := time.NewTicker(100 * time.Millisecond)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			log.Debug().
				Str("worker_id", w.id).
				Msg("Worker stopping due to context cancellation")
			return

		case <-w.stopCh:
			log.Debug().
				Str("worker_id", w.id).
				Msg("Worker stopping due to stop signal")
			return

		case <-ticker.C:
			w.tick(ctx)
		}
	}
}

// tick processes one iteration of the worker loop
func (w *DefaultWorker) tick(ctx context.Context) {
	w.mu.Lock()

	// Only request work during idle phase and not currently processing
	if !w.routine.IsIdle() || w.processing {
		w.mu.Unlock()
		return
	}

	// Try to get an event
	e, ok := w.provider.RequestEvent()
	if !ok {
		w.mu.Unlock()
		return
	}

	w.currentEvent = e
	w.processing = true
	w.mu.Unlock()

	// Process the event
	success := w.processEvent(ctx, e)

	// Report completion
	w.provider.ReportCompletion(ctx, e, success)

	w.mu.Lock()
	w.currentEvent = nil
	w.processing = false
	w.mu.Unlock()
}

// processEvent simulates processing an event
func (w *DefaultWorker) processEvent(ctx context.Context, e *event.Event) bool {
	log.Debug().
		Str("worker_id", w.id).
		Str("event_id", e.ID).
		Str("event_type", string(e.Type)).
		Msg("Processing event")

	processingTime := w.cfg.EventHandlingTime

	select {
	case <-ctx.Done():
		log.Debug().
			Str("worker_id", w.id).
			Str("event_id", e.ID).
			Msg("Event processing cancelled")
		return false

	case <-time.After(processingTime):
		// Check if event expired during processing
		if e.IsExpired() {
			log.Debug().
				Str("worker_id", w.id).
				Str("event_id", e.ID).
				Msg("Event expired during processing")
			return false
		}

		log.Debug().
			Str("worker_id", w.id).
			Str("event_id", e.ID).
			Dur("processing_time", processingTime).
			Msg("Event processed successfully")
		return true
	}
}

// GetCurrentEvent returns the event currently being processed (for monitoring)
func (w *DefaultWorker) GetCurrentEvent() *event.Event {
	w.mu.RLock()
	defer w.mu.RUnlock()
	return w.currentEvent
}

// GetRoutineState returns the current routine state
func (w *DefaultWorker) GetRoutineState() RoutineState {
	w.mu.RLock()
	defer w.mu.RUnlock()
	return w.routine.State()
}

// GetRoutineType returns the worker's routine type
func (w *DefaultWorker) GetRoutineType() config.RoutineType {
	return w.routine.Type()
}
