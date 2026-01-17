package coordinator

import (
	"context"
	"testing"
	"time"

	"marry-me/internal/broker"
	"marry-me/internal/config"
	"marry-me/internal/event"
)

func TestCoordinator_ValidEvent(t *testing.T) {
	mockBroker := broker.NewMockBroker()
	cfg := &config.Config{
		IncomingQueue:  "events.incoming",
		ValidatedQueue: "events.validated",
	}

	coord := New(mockBroker, cfg)

	ctx := context.Background()
	e := &event.Event{
		ID:        "test-001",
		Type:      event.EventMealService,
		Priority:  event.PriorityHigh,
		Timestamp: time.Now().Unix(),
	}

	err := coord.InjectEvent(ctx, e)
	if err != nil {
		t.Fatalf("InjectEvent failed: %v", err)
	}

	stats := coord.GetStats()
	if stats.EventsReceived != 1 {
		t.Errorf("EventsReceived = %d, want 1", stats.EventsReceived)
	}
	if stats.EventsValidated != 1 {
		t.Errorf("EventsValidated = %d, want 1", stats.EventsValidated)
	}
	if stats.EventsRejected != 0 {
		t.Errorf("EventsRejected = %d, want 0", stats.EventsRejected)
	}

	published := mockBroker.GetPublished()
	if len(published) != 1 {
		t.Fatalf("Published count = %d, want 1", len(published))
	}
	if published[0].ID != e.ID {
		t.Errorf("Published event ID = %q, want %q", published[0].ID, e.ID)
	}
}

func TestCoordinator_InvalidEventType(t *testing.T) {
	mockBroker := broker.NewMockBroker()
	cfg := &config.Config{
		IncomingQueue:  "events.incoming",
		ValidatedQueue: "events.validated",
	}

	coord := New(mockBroker, cfg)

	ctx := context.Background()
	e := &event.Event{
		ID:        "test-002",
		Type:      event.EventType("invalid_type"),
		Priority:  event.PriorityMedium,
		Timestamp: time.Now().Unix(),
	}

	err := coord.InjectEvent(ctx, e)
	if err != nil {
		t.Fatalf("InjectEvent failed: %v", err)
	}

	stats := coord.GetStats()
	if stats.EventsReceived != 1 {
		t.Errorf("EventsReceived = %d, want 1", stats.EventsReceived)
	}
	if stats.EventsValidated != 0 {
		t.Errorf("EventsValidated = %d, want 0", stats.EventsValidated)
	}
	if stats.EventsRejected != 1 {
		t.Errorf("EventsRejected = %d, want 1", stats.EventsRejected)
	}

	published := mockBroker.GetPublished()
	if len(published) != 0 {
		t.Errorf("Published count = %d, want 0", len(published))
	}
}

func TestCoordinator_SetsDeadline(t *testing.T) {
	mockBroker := broker.NewMockBroker()
	cfg := &config.Config{
		IncomingQueue:  "events.incoming",
		ValidatedQueue: "events.validated",
	}

	coord := New(mockBroker, cfg)

	ctx := context.Background()
	e := &event.Event{
		ID:        "test-003",
		Type:      event.EventPhotoSession,
		Priority:  event.PriorityHigh,
		Timestamp: time.Now().Unix(),
	}

	beforeInject := time.Now()
	err := coord.InjectEvent(ctx, e)
	if err != nil {
		t.Fatalf("InjectEvent failed: %v", err)
	}

	published := mockBroker.GetPublished()
	if len(published) != 1 {
		t.Fatalf("Published count = %d, want 1", len(published))
	}

	publishedEvent := published[0]
	if publishedEvent.ReceivedAt.Before(beforeInject) {
		t.Error("ReceivedAt should be set")
	}
	if publishedEvent.Deadline.IsZero() {
		t.Error("Deadline should be set")
	}

	// High priority should have 5 second deadline
	expectedDeadline := publishedEvent.ReceivedAt.Add(5 * time.Second)
	if !publishedEvent.Deadline.Equal(expectedDeadline) {
		t.Errorf("Deadline = %v, want %v", publishedEvent.Deadline, expectedDeadline)
	}
}

func TestCoordinator_InjectMultipleEvents(t *testing.T) {
	mockBroker := broker.NewMockBroker()
	cfg := &config.Config{
		IncomingQueue:  "events.incoming",
		ValidatedQueue: "events.validated",
	}

	coord := New(mockBroker, cfg)

	ctx := context.Background()
	events := []*event.Event{
		{ID: "test-001", Type: event.EventMealService, Priority: event.PriorityHigh},
		{ID: "test-002", Type: event.EventPhotoSession, Priority: event.PriorityMedium},
		{ID: "test-003", Type: event.EventBandSetup, Priority: event.PriorityLow},
	}

	err := coord.InjectEvents(ctx, events)
	if err != nil {
		t.Fatalf("InjectEvents failed: %v", err)
	}

	stats := coord.GetStats()
	if stats.EventsReceived != 3 {
		t.Errorf("EventsReceived = %d, want 3", stats.EventsReceived)
	}
	if stats.EventsValidated != 3 {
		t.Errorf("EventsValidated = %d, want 3", stats.EventsValidated)
	}
}

func TestCoordinator_StartStop(t *testing.T) {
	mockBroker := broker.NewMockBroker()
	cfg := &config.Config{
		IncomingQueue:  "events.incoming",
		ValidatedQueue: "events.validated",
	}

	coord := New(mockBroker, cfg)

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	err := coord.Start(ctx)
	if err != nil {
		t.Fatalf("Start failed: %v", err)
	}

	// Give it time to start
	time.Sleep(50 * time.Millisecond)

	coord.Stop()

	// Should not hang
}
