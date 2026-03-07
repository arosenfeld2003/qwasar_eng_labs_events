package team

import (
	"context"
	"encoding/json"
	"testing"
	"time"

	"github.com/arosenfeld2003/qwasar_eng_labs_events/internal/broker"
	"github.com/arosenfeld2003/qwasar_eng_labs_events/internal/event"
	"github.com/arosenfeld2003/qwasar_eng_labs_events/internal/organizer"
)

func makeEvent(id int, et event.EventType, p event.Priority) event.Event {
	return event.Event{ID: id, Type: et, Priority: p}
}

func TestManagerSetup(t *testing.T) {
	mb := broker.NewMockBroker()
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	m := New(mb, 2, 1.0)
	if err := m.Setup(ctx); err != nil {
		t.Fatalf("setup error: %v", err)
	}

	// Verify all 10 teams were initialized.
	m.mu.Lock()
	got := len(m.teams)
	m.mu.Unlock()
	if got != 10 {
		t.Fatalf("expected 10 teams, got %d", got)
	}
}

func TestManagerWorkersProcessEvents(t *testing.T) {
	mb := broker.NewMockBroker()
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	// Declare queues the organizer would normally set up.
	_, _ = mb.DeclareQueue(ctx, organizer.TeamQueue(event.TeamCatering), broker.QueueOptions{Durable: true})
	_, _ = mb.DeclareQueue(ctx, organizer.QueueResults, broker.QueueOptions{Durable: true})

	resSub, err := mb.Subscribe(ctx, broker.ConsumeOptions{Queue: organizer.QueueResults, AutoAck: true})
	if err != nil {
		t.Fatalf("subscribe results: %v", err)
	}

	// speed=50: work=400ms, idle=100ms, processing=60ms — comfortable windows for testing.
	m := New(mb, 1, 50.0)
	if err := m.Setup(ctx); err != nil {
		t.Fatalf("setup error: %v", err)
	}

	// Wait 450ms to be 50ms into the idle phase (work=400ms).
	time.Sleep(450 * time.Millisecond)

	// Publish an event with a generous deadline.
	now := time.Now()
	ev := makeEvent(1, event.TypeMealService, event.PriorityLow)
	ev.ReceivedAt = now
	ev.Deadline = now.Add(10 * time.Second)
	b, _ := json.Marshal(ev)
	_ = mb.Publish(ctx, broker.Message{Body: b, ContentType: "application/json"},
		broker.PublishOptions{RoutingKey: organizer.TeamQueue(event.TeamCatering)})

	// Allow processing (60ms) plus generous margin.
	select {
	case d := <-resSub.Deliveries:
		var wrapper struct {
			Event event.Event `json:"event"`
		}
		if err := json.Unmarshal(d.Body, &wrapper); err != nil {
			t.Fatalf("unmarshal result: %v", err)
		}
		if wrapper.Event.Status != event.StatusCompleted {
			t.Fatalf("expected completed, got %s", wrapper.Event.Status)
		}
	case <-time.After(500 * time.Millisecond):
		t.Fatal("timeout: no result received")
	}
}

func TestManagerShutdown(t *testing.T) {
	mb := broker.NewMockBroker()
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	m := New(mb, 1, 1.0)
	if err := m.Setup(ctx); err != nil {
		t.Fatalf("setup error: %v", err)
	}

	if err := m.Shutdown(context.Background()); err != nil {
		t.Fatalf("shutdown error: %v", err)
	}
}

func TestManagerSpeedScaling(t *testing.T) {
	mb := broker.NewMockBroker()
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	// speed=10 → work=2s, idle=0.5s, processing=0.3s
	m := New(mb, 1, 10.0)
	if err := m.Setup(ctx); err != nil {
		t.Fatalf("setup error: %v", err)
	}

	m.mu.Lock()
	ts, ok := m.teams[event.TeamCatering]
	m.mu.Unlock()
	if !ok || len(ts.workers) != 1 {
		t.Fatal("expected 1 catering worker")
	}

	w := ts.workers[0]
	wantProcessing := time.Duration(float64(3*time.Second) / 10.0)
	if w.ProcessingTime != wantProcessing {
		t.Fatalf("processing time: got %v, want %v", w.ProcessingTime, wantProcessing)
	}
	wantWork := time.Duration(float64(20*time.Second) / 10.0)
	if w.Routine.Work != wantWork {
		t.Fatalf("work duration: got %v, want %v", w.Routine.Work, wantWork)
	}
}
