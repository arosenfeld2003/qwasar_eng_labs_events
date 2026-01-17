package worker

import (
	"context"
	"sync"
	"testing"
	"time"

	"marry-me/internal/config"
	"marry-me/internal/event"
)

// MockProvider implements EventProvider for testing
type MockProvider struct {
	mu         sync.Mutex
	events     []*event.Event
	completed  []*event.Event
}

func NewMockProvider() *MockProvider {
	return &MockProvider{
		events:    make([]*event.Event, 0),
		completed: make([]*event.Event, 0),
	}
}

func (m *MockProvider) AddEvent(e *event.Event) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.events = append(m.events, e)
}

func (m *MockProvider) RequestEvent() (*event.Event, bool) {
	m.mu.Lock()
	defer m.mu.Unlock()

	if len(m.events) == 0 {
		return nil, false
	}

	e := m.events[0]
	m.events = m.events[1:]
	return e, true
}

func (m *MockProvider) ReportCompletion(ctx context.Context, e *event.Event, success bool) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.completed = append(m.completed, e)
}

func (m *MockProvider) GetCompleted() []*event.Event {
	m.mu.Lock()
	defer m.mu.Unlock()
	return m.completed
}

func TestRoutine_StateTransitions(t *testing.T) {
	cfg := &config.Config{
		StandardRoutine: config.RoutineConfig{
			WorkDuration: 100 * time.Millisecond,
			IdleDuration: 50 * time.Millisecond,
		},
	}

	r := NewRoutine(config.RoutineStandard, cfg)

	// Initially should be working
	if r.State() != StateWorking {
		t.Errorf("Initial state = %v, want working", r.State())
	}

	// Wait for work period to end
	time.Sleep(110 * time.Millisecond)

	// Should now be idle
	if r.State() != StateIdle {
		t.Errorf("After work period = %v, want idle", r.State())
	}

	// Wait for idle period to end
	time.Sleep(60 * time.Millisecond)

	// Should be working again
	if r.State() != StateWorking {
		t.Errorf("After full cycle = %v, want working", r.State())
	}
}

func TestRoutine_IsIdle(t *testing.T) {
	cfg := &config.Config{
		StandardRoutine: config.RoutineConfig{
			WorkDuration: 50 * time.Millisecond,
			IdleDuration: 50 * time.Millisecond,
		},
	}

	r := NewRoutine(config.RoutineStandard, cfg)

	if r.IsIdle() {
		t.Error("Should not be idle initially")
	}

	time.Sleep(60 * time.Millisecond)

	if !r.IsIdle() {
		t.Error("Should be idle after work period")
	}
}

func TestRoutine_CycleDuration(t *testing.T) {
	cfg := &config.Config{
		StandardRoutine: config.RoutineConfig{
			WorkDuration: 20 * time.Second,
			IdleDuration: 5 * time.Second,
		},
	}

	r := NewRoutine(config.RoutineStandard, cfg)

	expected := 25 * time.Second
	if r.CycleDuration() != expected {
		t.Errorf("CycleDuration = %v, want %v", r.CycleDuration(), expected)
	}
}

func TestRoutine_WithOffset(t *testing.T) {
	cfg := &config.Config{
		StandardRoutine: config.RoutineConfig{
			WorkDuration: 100 * time.Millisecond,
			IdleDuration: 100 * time.Millisecond,
		},
	}

	// Create routine with offset that puts it in idle phase
	r := NewRoutineWithOffset(config.RoutineStandard, cfg, 120*time.Millisecond)

	// Should be in idle phase due to offset
	if r.State() != StateIdle {
		t.Errorf("State with offset = %v, want idle", r.State())
	}
}

func TestWorker_ProcessesEvent(t *testing.T) {
	cfg := &config.Config{
		EventHandlingTime: 50 * time.Millisecond,
		StandardRoutine: config.RoutineConfig{
			WorkDuration: 20 * time.Millisecond, // Short work period so we quickly get to idle
			IdleDuration: 500 * time.Millisecond,
		},
	}

	provider := NewMockProvider()

	e := &event.Event{
		ID:         "test-001",
		Type:       event.EventMealService,
		Priority:   event.PriorityHigh,
		ReceivedAt: time.Now(),
	}
	e.SetDeadline()

	worker := New("test-worker", provider, config.RoutineStandard, cfg)

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	err := worker.Start(ctx)
	if err != nil {
		t.Fatalf("Start failed: %v", err)
	}

	// Wait for worker to enter idle state
	time.Sleep(30 * time.Millisecond)

	// Add event
	provider.AddEvent(e)

	// Wait for processing
	time.Sleep(200 * time.Millisecond)

	worker.Stop()

	completed := provider.GetCompleted()
	if len(completed) != 1 {
		t.Errorf("Completed count = %d, want 1", len(completed))
	}
}

func TestWorker_ID(t *testing.T) {
	cfg := &config.Config{}
	provider := NewMockProvider()

	worker := New("my-worker-id", provider, config.RoutineStandard, cfg)

	if worker.ID() != "my-worker-id" {
		t.Errorf("ID = %q, want %q", worker.ID(), "my-worker-id")
	}
}

func TestWorker_IsIdle(t *testing.T) {
	cfg := &config.Config{
		StandardRoutine: config.RoutineConfig{
			WorkDuration: 10 * time.Millisecond,
			IdleDuration: 100 * time.Millisecond,
		},
	}

	provider := NewMockProvider()
	worker := New("test-worker", provider, config.RoutineStandard, cfg)

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	worker.Start(ctx)

	// Initially in work phase
	if worker.IsIdle() {
		t.Error("Should not be idle in work phase")
	}

	// Wait for idle phase
	time.Sleep(20 * time.Millisecond)

	if !worker.IsIdle() {
		t.Error("Should be idle in idle phase")
	}

	worker.Stop()
}

func TestWorker_StopsDuringProcessing(t *testing.T) {
	cfg := &config.Config{
		EventHandlingTime: 500 * time.Millisecond, // Long processing
		StandardRoutine: config.RoutineConfig{
			WorkDuration: 10 * time.Millisecond,
			IdleDuration: 1 * time.Second,
		},
	}

	provider := NewMockProvider()
	e := &event.Event{
		ID:         "test-001",
		Type:       event.EventMealService,
		ReceivedAt: time.Now(),
	}
	e.SetDeadline()

	worker := New("test-worker", provider, config.RoutineStandard, cfg)

	ctx, cancel := context.WithCancel(context.Background())

	worker.Start(ctx)
	time.Sleep(20 * time.Millisecond) // Enter idle

	provider.AddEvent(e)
	time.Sleep(100 * time.Millisecond) // Start processing

	// Cancel context
	cancel()

	// Stop should not hang
	done := make(chan bool)
	go func() {
		worker.Stop()
		done <- true
	}()

	select {
	case <-done:
		// Good, stopped successfully
	case <-time.After(2 * time.Second):
		t.Error("Worker.Stop() timed out")
	}
}

func TestDefaultWorker_GetRoutineType(t *testing.T) {
	cfg := &config.Config{}
	provider := NewMockProvider()

	w := New("test", provider, config.RoutineIntermittent, cfg)
	dw := w.(*DefaultWorker)

	if dw.GetRoutineType() != config.RoutineIntermittent {
		t.Errorf("GetRoutineType = %v, want intermittent", dw.GetRoutineType())
	}
}
