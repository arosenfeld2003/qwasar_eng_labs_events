package organizer

import (
	"context"
	"testing"
	"time"

	"marry-me/internal/broker"
	"marry-me/internal/config"
	"marry-me/internal/event"
)

func TestPriorityQueue_PushPop(t *testing.T) {
	pq := NewPriorityQueue()

	now := time.Now()
	events := []*event.Event{
		{ID: "low", Priority: event.PriorityLow, Deadline: now.Add(20 * time.Second)},
		{ID: "high", Priority: event.PriorityHigh, Deadline: now.Add(5 * time.Second)},
		{ID: "medium", Priority: event.PriorityMedium, Deadline: now.Add(10 * time.Second)},
	}

	for _, e := range events {
		pq.Push(e)
	}

	if pq.Len() != 3 {
		t.Errorf("Len = %d, want 3", pq.Len())
	}

	// Should pop in priority order: high, medium, low
	e, ok := pq.Pop()
	if !ok || e.ID != "high" {
		t.Errorf("First pop = %v, want high", e.ID)
	}

	e, ok = pq.Pop()
	if !ok || e.ID != "medium" {
		t.Errorf("Second pop = %v, want medium", e.ID)
	}

	e, ok = pq.Pop()
	if !ok || e.ID != "low" {
		t.Errorf("Third pop = %v, want low", e.ID)
	}

	_, ok = pq.Pop()
	if ok {
		t.Error("Pop from empty queue should return false")
	}
}

func TestPriorityQueue_SamePriorityDeadlineOrdering(t *testing.T) {
	pq := NewPriorityQueue()

	now := time.Now()
	events := []*event.Event{
		{ID: "later", Priority: event.PriorityHigh, Deadline: now.Add(10 * time.Second)},
		{ID: "earlier", Priority: event.PriorityHigh, Deadline: now.Add(5 * time.Second)},
		{ID: "middle", Priority: event.PriorityHigh, Deadline: now.Add(7 * time.Second)},
	}

	for _, e := range events {
		pq.Push(e)
	}

	// Should pop in deadline order for same priority
	e, _ := pq.Pop()
	if e.ID != "earlier" {
		t.Errorf("First pop = %v, want earlier", e.ID)
	}

	e, _ = pq.Pop()
	if e.ID != "middle" {
		t.Errorf("Second pop = %v, want middle", e.ID)
	}

	e, _ = pq.Pop()
	if e.ID != "later" {
		t.Errorf("Third pop = %v, want later", e.ID)
	}
}

func TestPriorityQueue_Peek(t *testing.T) {
	pq := NewPriorityQueue()

	_, ok := pq.Peek()
	if ok {
		t.Error("Peek on empty queue should return false")
	}

	e := &event.Event{ID: "test", Priority: event.PriorityHigh}
	pq.Push(e)

	peeked, ok := pq.Peek()
	if !ok {
		t.Error("Peek should return true")
	}
	if peeked.ID != "test" {
		t.Errorf("Peek = %v, want test", peeked.ID)
	}

	// Peek should not remove item
	if pq.Len() != 1 {
		t.Error("Peek should not remove item")
	}
}

func TestPriorityQueue_RemoveExpired(t *testing.T) {
	pq := NewPriorityQueue()

	now := time.Now()
	events := []*event.Event{
		{ID: "expired1", Priority: event.PriorityHigh, Deadline: now.Add(-1 * time.Second)},
		{ID: "valid", Priority: event.PriorityMedium, Deadline: now.Add(10 * time.Second)},
		{ID: "expired2", Priority: event.PriorityLow, Deadline: now.Add(-2 * time.Second)},
	}

	for _, e := range events {
		pq.Push(e)
	}

	expired := pq.RemoveExpired()
	if len(expired) != 2 {
		t.Errorf("Expired count = %d, want 2", len(expired))
	}

	if pq.Len() != 1 {
		t.Errorf("Queue length after remove = %d, want 1", pq.Len())
	}

	remaining, _ := pq.Pop()
	if remaining.ID != "valid" {
		t.Errorf("Remaining = %v, want valid", remaining.ID)
	}
}

func TestPriorityQueue_ThreadSafety(t *testing.T) {
	pq := NewPriorityQueue()

	done := make(chan bool)

	// Concurrent pushes
	go func() {
		for i := 0; i < 100; i++ {
			pq.Push(&event.Event{ID: "push"})
		}
		done <- true
	}()

	// Concurrent pops
	go func() {
		for i := 0; i < 50; i++ {
			pq.Pop()
		}
		done <- true
	}()

	// Concurrent peeks
	go func() {
		for i := 0; i < 100; i++ {
			pq.Peek()
		}
		done <- true
	}()

	<-done
	<-done
	<-done

	// Should not panic or deadlock
}

func TestOrganizer_RoutesEvent(t *testing.T) {
	mockBroker := broker.NewMockBroker()
	cfg := &config.Config{
		ValidatedQueue: "events.validated",
		ResultsQueue:   "events.results",
	}

	org := New(mockBroker, cfg)

	ctx := context.Background()
	e := &event.Event{
		ID:         "test-001",
		Type:       event.EventMealService,
		Priority:   event.PriorityHigh,
		ReceivedAt: time.Now(),
	}
	e.SetDeadline()

	// Manually call handleEvent since we can't easily trigger Subscribe
	err := org.handleEvent(ctx, e)
	if err != nil {
		t.Fatalf("handleEvent failed: %v", err)
	}

	stats := org.GetStats()
	if stats.EventsRouted != 1 {
		t.Errorf("EventsRouted = %d, want 1", stats.EventsRouted)
	}
}

func TestOrganizer_ExpiredEvent(t *testing.T) {
	mockBroker := broker.NewMockBroker()
	cfg := &config.Config{
		ValidatedQueue: "events.validated",
		ResultsQueue:   "events.results",
	}

	org := New(mockBroker, cfg)

	ctx := context.Background()
	e := &event.Event{
		ID:         "test-expired",
		Type:       event.EventMealService,
		Priority:   event.PriorityHigh,
		ReceivedAt: time.Now().Add(-10 * time.Second),
		Deadline:   time.Now().Add(-5 * time.Second), // Already expired
	}

	err := org.handleEvent(ctx, e)
	if err != nil {
		t.Fatalf("handleEvent failed: %v", err)
	}

	stats := org.GetStats()
	if stats.EventsExpired != 1 {
		t.Errorf("EventsExpired = %d, want 1", stats.EventsExpired)
	}
	if stats.EventsRouted != 0 {
		t.Errorf("EventsRouted = %d, want 0", stats.EventsRouted)
	}
}

func TestOrganizer_MultiTeamEvent(t *testing.T) {
	mockBroker := broker.NewMockBroker()
	cfg := &config.Config{
		ValidatedQueue: "events.validated",
		ResultsQueue:   "events.results",
	}

	org := New(mockBroker, cfg)

	ctx := context.Background()
	// first_dance is handled by both music and photography teams
	e := &event.Event{
		ID:         "test-multi",
		Type:       event.EventFirstDance,
		Priority:   event.PriorityMedium,
		ReceivedAt: time.Now(),
	}
	e.SetDeadline()

	err := org.handleEvent(ctx, e)
	if err != nil {
		t.Fatalf("handleEvent failed: %v", err)
	}

	stats := org.GetStats()
	if stats.MultiTeamEvents != 1 {
		t.Errorf("MultiTeamEvents = %d, want 1", stats.MultiTeamEvents)
	}
}
