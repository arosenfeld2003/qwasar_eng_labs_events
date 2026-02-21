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

	m := New(mb, 2)
	if err := m.Setup(ctx); err != nil {
		t.Fatalf("setup error: %v", err)
	}

	// Verify all 8 teams were initialized
	allStats := m.AllStats()
	if len(allStats) != 8 {
		t.Fatalf("expected 8 teams, got %d", len(allStats))
	}

	// Verify each team has a Stats entry
	teams := []event.Team{
		event.TeamCatering,
		event.TeamDecoration,
		event.TeamPhotography,
		event.TeamMusic,
		event.TeamCoordinator,
		event.TeamFloral,
		event.TeamVenue,
		event.TeamTransport,
	}
	for _, team := range teams {
		if _, ok := allStats[team]; !ok {
			t.Errorf("missing stats for team %s", team)
		}
	}
}

func TestManagerAcceptsEventsInPriorityOrder(t *testing.T) {
	mb := broker.NewMockBroker()
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	// Declare team queue
	_, _ = mb.DeclareQueue(ctx, organizer.TeamQueue(event.TeamCatering), broker.QueueOptions{Durable: true})

	m := New(mb, 1)
	if err := m.Setup(ctx); err != nil {
		t.Fatalf("setup error: %v", err)
	}

	// Publish events in reverse priority order
	now := time.Now()
	evLow := makeEvent(1, event.TypeMealService, event.PriorityLow)
	evLow.ReceivedAt = now
	evLow.Deadline = now.Add(10 * time.Second)

	evHigh := makeEvent(2, event.TypeMealService, event.PriorityHigh)
	evHigh.ReceivedAt = now
	evHigh.Deadline = now.Add(10 * time.Second)

	evMedium := makeEvent(3, event.TypeMealService, event.PriorityMedium)
	evMedium.ReceivedAt = now
	evMedium.Deadline = now.Add(10 * time.Second)

	// Publish in order: Low, High, Medium
	for _, ev := range []event.Event{evLow, evHigh, evMedium} {
		b, _ := json.Marshal(ev)
		_ = mb.Publish(ctx, broker.Message{Body: b, ContentType: "application/json"},
			broker.PublishOptions{RoutingKey: organizer.TeamQueue(event.TeamCatering)})
	}

	// Give time for events to be queued
	time.Sleep(100 * time.Millisecond)

	// Check pending count
	stats := m.Stats(event.TeamCatering)
	if stats.Pending != 3 {
		t.Fatalf("expected 3 pending events, got %d", stats.Pending)
	}
}

func TestManagerChecksExpirationPeriodically(t *testing.T) {
	mb := broker.NewMockBroker()
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	_, _ = mb.DeclareQueue(ctx, organizer.TeamQueue(event.TeamDecoration), broker.QueueOptions{Durable: true})
	_, _ = mb.DeclareQueue(ctx, organizer.QueueResults, broker.QueueOptions{Durable: true})

	resSub, _ := mb.Subscribe(ctx, broker.ConsumeOptions{Queue: organizer.QueueResults, AutoAck: true})

	m := New(mb, 1)
	// Use shorter expiration check for test
	m.expirationCheck = 50 * time.Millisecond
	if err := m.Setup(ctx); err != nil {
		t.Fatalf("setup error: %v", err)
	}

	// Publish an event that will expire quickly
	now := time.Now()
	ev := makeEvent(10, event.TypeTableSetup, event.PriorityHigh)
	ev.ReceivedAt = now
	ev.Deadline = now.Add(50 * time.Millisecond) // Will expire soon
	b, _ := json.Marshal(ev)
	_ = mb.Publish(ctx, broker.Message{Body: b, ContentType: "application/json"},
		broker.PublishOptions{RoutingKey: organizer.TeamQueue(event.TeamDecoration)})

	// Give time for intake and expiration check
	time.Sleep(200 * time.Millisecond)

	// Verify stats: pending should be 0, expired should be 1
	stats := m.Stats(event.TeamDecoration)
	if stats.Pending != 0 {
		t.Fatalf("expected 0 pending after expiration, got %d", stats.Pending)
	}
	if stats.Expired != 1 {
		t.Fatalf("expected 1 expired, got %d", stats.Expired)
	}

	// Verify expired event was published to results queue
	select {
	case d := <-resSub.Deliveries:
		var resultEv event.Event
		if err := json.Unmarshal(d.Body, &resultEv); err != nil {
			t.Fatalf("unmarshal result: %v", err)
		}
		if resultEv.Status != event.StatusExpired {
			t.Fatalf("expected expired status, got %s", resultEv.Status)
		}
	case <-time.After(500 * time.Millisecond):
		t.Fatalf("timeout waiting for expired event on results queue")
	}
}

func TestManagerMultipleTeams(t *testing.T) {
	mb := broker.NewMockBroker()
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	// Declare queues for two teams
	_, _ = mb.DeclareQueue(ctx, organizer.TeamQueue(event.TeamCatering), broker.QueueOptions{Durable: true})
	_, _ = mb.DeclareQueue(ctx, organizer.TeamQueue(event.TeamPhotography), broker.QueueOptions{Durable: true})

	m := New(mb, 1)
	if err := m.Setup(ctx); err != nil {
		t.Fatalf("setup error: %v", err)
	}

	now := time.Now()
	// Publish to catering
	ev1 := makeEvent(1, event.TypeMealService, event.PriorityHigh)
	ev1.ReceivedAt = now
	ev1.Deadline = now.Add(10 * time.Second)
	b1, _ := json.Marshal(ev1)
	_ = mb.Publish(ctx, broker.Message{Body: b1, ContentType: "application/json"},
		broker.PublishOptions{RoutingKey: organizer.TeamQueue(event.TeamCatering)})

	// Publish to photography
	ev2 := makeEvent(2, event.TypePhotoSession, event.PriorityMedium)
	ev2.ReceivedAt = now
	ev2.Deadline = now.Add(10 * time.Second)
	b2, _ := json.Marshal(ev2)
	_ = mb.Publish(ctx, broker.Message{Body: b2, ContentType: "application/json"},
		broker.PublishOptions{RoutingKey: organizer.TeamQueue(event.TeamPhotography)})

	time.Sleep(100 * time.Millisecond)

	cateringStats := m.Stats(event.TeamCatering)
	photoStats := m.Stats(event.TeamPhotography)

	if cateringStats.Pending != 1 {
		t.Fatalf("catering: expected 1 pending, got %d", cateringStats.Pending)
	}
	if photoStats.Pending != 1 {
		t.Fatalf("photography: expected 1 pending, got %d", photoStats.Pending)
	}
}
