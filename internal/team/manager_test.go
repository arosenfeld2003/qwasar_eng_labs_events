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
	// Use Security (Standard routine: 20s/5s) — Catering is Concentrated (60s/60s).
	_, _ = mb.DeclareQueue(ctx, organizer.TeamQueue(event.TeamSecurity), broker.QueueOptions{Durable: true})
	_, _ = mb.DeclareQueue(ctx, organizer.QueueResults, broker.QueueOptions{Durable: true})

	resSub, err := mb.Subscribe(ctx, broker.ConsumeOptions{Queue: organizer.QueueResults, AutoAck: true})
	if err != nil {
		t.Fatalf("subscribe results: %v", err)
	}

	// speed=50: Security work=400ms (20s/50), idle=100ms (5s/50), processing=60ms.
	m := New(mb, 1, 50.0)
	if err := m.Setup(ctx); err != nil {
		t.Fatalf("setup error: %v", err)
	}

	// Wait 450ms to be 50ms into the idle phase (work=400ms).
	time.Sleep(450 * time.Millisecond)

	// Publish an event with a generous deadline.
	now := time.Now()
	ev := makeEvent(1, event.TypeBrawl, event.PriorityLow)
	ev.ReceivedAt = now
	ev.Deadline = now.Add(10 * time.Second)
	b, _ := json.Marshal(ev)
	_ = mb.Publish(ctx, broker.Message{Body: b, ContentType: "application/json"},
		broker.PublishOptions{RoutingKey: organizer.TeamQueue(event.TeamSecurity)})

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

	m := New(mb, 1, 10.0)
	if err := m.Setup(ctx); err != nil {
		t.Fatalf("setup error: %v", err)
	}

	m.mu.Lock()
	catering := m.teams[event.TeamCatering]
	venue := m.teams[event.TeamVenue]
	security := m.teams[event.TeamSecurity]
	m.mu.Unlock()

	wantProcessing := time.Duration(float64(3*time.Second) / 10.0)

	// Catering: Concentrated (60s/60s) scaled by speed=10
	if w := catering.workers[0]; w.ProcessingTime != wantProcessing {
		t.Fatalf("catering processing time: got %v, want %v", w.ProcessingTime, wantProcessing)
	}
	if w := catering.workers[0]; w.Routine.Work != time.Duration(float64(60*time.Second)/10.0) {
		t.Fatalf("catering work: got %v, want 6s", catering.workers[0].Routine.Work)
	}

	// Venue: Intermittent (5s/5s) scaled by speed=10
	if w := venue.workers[0]; w.Routine.Work != time.Duration(float64(5*time.Second)/10.0) {
		t.Fatalf("venue work: got %v, want 500ms", w.Routine.Work)
	}

	// Security: Standard (20s/5s) scaled by speed=10
	if w := security.workers[0]; w.Routine.Work != time.Duration(float64(20*time.Second)/10.0) {
		t.Fatalf("security work: got %v, want 2s", w.Routine.Work)
	}
	_ = wantProcessing
}
