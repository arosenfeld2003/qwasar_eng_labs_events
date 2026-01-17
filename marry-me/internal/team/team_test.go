package team

import (
	"context"
	"testing"
	"time"

	"marry-me/internal/broker"
	"marry-me/internal/config"
	"marry-me/internal/event"
)

func TestTeam_RequestEvent(t *testing.T) {
	mockBroker := broker.NewMockBroker()
	cfg := &config.Config{
		WorkersPerTeam:    1,
		EventHandlingTime: 100 * time.Millisecond,
		ResultsQueue:      "events.results",
		StandardRoutine: config.RoutineConfig{
			WorkDuration: 1 * time.Second,
			IdleDuration: 100 * time.Millisecond,
		},
	}

	team := New(event.TeamCatering, mockBroker, cfg)

	// Add event directly to queue (bypassing broker subscription)
	e := &event.Event{
		ID:         "test-001",
		Type:       event.EventMealService,
		Priority:   event.PriorityHigh,
		ReceivedAt: time.Now(),
	}
	e.SetDeadline()
	team.queue.Push(e)

	// Request event
	retrieved, ok := team.RequestEvent()
	if !ok {
		t.Fatal("RequestEvent should return true")
	}
	if retrieved.ID != e.ID {
		t.Errorf("Retrieved ID = %q, want %q", retrieved.ID, e.ID)
	}
	if retrieved.Status != event.StatusProcessing {
		t.Errorf("Status = %v, want processing", retrieved.Status)
	}
}

func TestTeam_RequestEvent_Empty(t *testing.T) {
	mockBroker := broker.NewMockBroker()
	cfg := &config.Config{
		WorkersPerTeam: 1,
		ResultsQueue:   "events.results",
	}

	team := New(event.TeamCatering, mockBroker, cfg)

	_, ok := team.RequestEvent()
	if ok {
		t.Error("RequestEvent on empty queue should return false")
	}
}

func TestTeam_RequestEvent_ExpiredSkipped(t *testing.T) {
	mockBroker := broker.NewMockBroker()
	cfg := &config.Config{
		WorkersPerTeam: 1,
		ResultsQueue:   "events.results",
	}

	team := New(event.TeamCatering, mockBroker, cfg)

	// Add expired event
	e := &event.Event{
		ID:         "test-expired",
		Type:       event.EventMealService,
		Priority:   event.PriorityHigh,
		ReceivedAt: time.Now().Add(-10 * time.Second),
		Deadline:   time.Now().Add(-5 * time.Second),
	}
	team.queue.Push(e)

	// Request should skip expired event
	_, ok := team.RequestEvent()
	if ok {
		t.Error("RequestEvent should skip expired events")
	}

	// Give async handler time to run
	time.Sleep(50 * time.Millisecond)

	// Check that expired stats were updated
	stats := team.GetStats()
	if stats.EventsExpired != 1 {
		t.Errorf("EventsExpired = %d, want 1", stats.EventsExpired)
	}
}

func TestTeam_ReportCompletion(t *testing.T) {
	mockBroker := broker.NewMockBroker()
	cfg := &config.Config{
		WorkersPerTeam: 1,
		ResultsQueue:   "events.results",
	}

	team := New(event.TeamCatering, mockBroker, cfg)

	ctx := context.Background()
	e := &event.Event{
		ID:     "test-001",
		Type:   event.EventMealService,
		Status: event.StatusProcessing,
	}

	team.ReportCompletion(ctx, e, true)

	stats := team.GetStats()
	if stats.EventsCompleted != 1 {
		t.Errorf("EventsCompleted = %d, want 1", stats.EventsCompleted)
	}

	if e.Status != event.StatusCompleted {
		t.Errorf("Event status = %v, want completed", e.Status)
	}
}

func TestTeam_ReportCompletion_Failure(t *testing.T) {
	mockBroker := broker.NewMockBroker()
	cfg := &config.Config{
		WorkersPerTeam: 1,
		ResultsQueue:   "events.results",
	}

	team := New(event.TeamCatering, mockBroker, cfg)

	ctx := context.Background()
	e := &event.Event{
		ID:     "test-001",
		Type:   event.EventMealService,
		Status: event.StatusProcessing,
	}

	team.ReportCompletion(ctx, e, false)

	stats := team.GetStats()
	if stats.EventsCompleted != 0 {
		t.Errorf("EventsCompleted = %d, want 0", stats.EventsCompleted)
	}

	if e.Status != event.StatusExpired {
		t.Errorf("Event status = %v, want expired", e.Status)
	}
}

func TestTeam_HandleIncomingEvent(t *testing.T) {
	mockBroker := broker.NewMockBroker()
	cfg := &config.Config{
		WorkersPerTeam: 1,
		ResultsQueue:   "events.results",
	}

	team := New(event.TeamCatering, mockBroker, cfg)

	ctx := context.Background()
	e := &event.Event{
		ID:         "test-001",
		Type:       event.EventMealService,
		Priority:   event.PriorityHigh,
		ReceivedAt: time.Now(),
	}
	e.SetDeadline()

	err := team.handleIncomingEvent(ctx, e)
	if err != nil {
		t.Fatalf("handleIncomingEvent failed: %v", err)
	}

	if team.QueueSize() != 1 {
		t.Errorf("Queue size = %d, want 1", team.QueueSize())
	}

	stats := team.GetStats()
	if stats.EventsReceived != 1 {
		t.Errorf("EventsReceived = %d, want 1", stats.EventsReceived)
	}
}

func TestTeam_HandleIncomingEvent_Expired(t *testing.T) {
	mockBroker := broker.NewMockBroker()
	cfg := &config.Config{
		WorkersPerTeam: 1,
		ResultsQueue:   "events.results",
	}

	team := New(event.TeamCatering, mockBroker, cfg)

	ctx := context.Background()
	e := &event.Event{
		ID:         "test-expired",
		Type:       event.EventMealService,
		Priority:   event.PriorityHigh,
		ReceivedAt: time.Now().Add(-10 * time.Second),
		Deadline:   time.Now().Add(-5 * time.Second),
	}

	err := team.handleIncomingEvent(ctx, e)
	if err != nil {
		t.Fatalf("handleIncomingEvent failed: %v", err)
	}

	// Expired events should not be added to queue
	if team.QueueSize() != 0 {
		t.Errorf("Queue size = %d, want 0", team.QueueSize())
	}

	stats := team.GetStats()
	if stats.EventsExpired != 1 {
		t.Errorf("EventsExpired = %d, want 1", stats.EventsExpired)
	}
}

func TestTeam_GetRoutineType(t *testing.T) {
	mockBroker := broker.NewMockBroker()
	cfg := &config.Config{}

	team := New(event.TeamCatering, mockBroker, cfg)

	if team.getRoutineType(0) != config.RoutineStandard {
		t.Error("Index 0 should be Standard routine")
	}
	if team.getRoutineType(1) != config.RoutineIntermittent {
		t.Error("Index 1 should be Intermittent routine")
	}
	if team.getRoutineType(2) != config.RoutineConcentrated {
		t.Error("Index 2 should be Concentrated routine")
	}
	if team.getRoutineType(3) != config.RoutineStandard {
		t.Error("Index 3 should wrap to Standard routine")
	}
}
