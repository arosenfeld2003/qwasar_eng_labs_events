package worker

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
	e := event.Event{ID: id, Type: et, Priority: p}
	return e
}

// TestWorkerProcessesEventDuringIdle verifies that an event published during the
// idle phase is processed and reported as completed.
func TestWorkerProcessesEventDuringIdle(t *testing.T) {
	mb := broker.NewMockBroker()
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	_, _ = mb.DeclareQueue(ctx, organizer.TeamQueue(event.TeamCatering), broker.QueueOptions{Durable: true})
	_, _ = mb.DeclareQueue(ctx, organizer.QueueResults, broker.QueueOptions{Durable: true})

	resSub, err := mb.Subscribe(ctx, broker.ConsumeOptions{Queue: organizer.QueueResults, AutoAck: true})
	if err != nil {
		t.Fatalf("subscribe results: %v", err)
	}

	// work=200ms, idle=200ms, processing=50ms
	w := New(1, event.TeamCatering, mb, Routine{Work: 200 * time.Millisecond, Idle: 200 * time.Millisecond})
	w.ProcessingTime = 50 * time.Millisecond
	go w.Run(ctx)

	// Wait until idle phase, then publish.
	time.Sleep(220 * time.Millisecond)

	now := time.Now()
	ev := makeEvent(1, event.TypeMealService, event.PriorityLow)
	ev.ReceivedAt = now
	ev.Deadline = now.Add(5 * time.Second)
	b, _ := json.Marshal(ev)
	_ = mb.Publish(ctx, broker.Message{Body: b, ContentType: "application/json"}, broker.PublishOptions{RoutingKey: organizer.TeamQueue(event.TeamCatering)})

	select {
	case d := <-resSub.Deliveries:
		var wrapper struct {
			Event event.Event `json:"event"`
		}
		if err := json.Unmarshal(d.Body, &wrapper); err != nil {
			t.Fatalf("unmarshal result: %v", err)
		}
		if wrapper.Event.ID != ev.ID {
			t.Fatalf("got event ID %d, want %d", wrapper.Event.ID, ev.ID)
		}
		if wrapper.Event.Status != event.StatusCompleted {
			t.Fatalf("expected completed, got %s", wrapper.Event.Status)
		}
	case <-time.After(600 * time.Millisecond):
		t.Fatal("timeout: expected completed event on results queue")
	}
}

// TestWorkerRepublishesDuringWorkPhase verifies that an event published during the
// work phase is NOT dropped — it is republished and eventually processed.
func TestWorkerRepublishesDuringWorkPhase(t *testing.T) {
	mb := broker.NewMockBroker()
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	_, _ = mb.DeclareQueue(ctx, organizer.TeamQueue(event.TeamCatering), broker.QueueOptions{Durable: true})
	_, _ = mb.DeclareQueue(ctx, organizer.QueueResults, broker.QueueOptions{Durable: true})

	resSub, err := mb.Subscribe(ctx, broker.ConsumeOptions{Queue: organizer.QueueResults, AutoAck: true})
	if err != nil {
		t.Fatalf("subscribe results: %v", err)
	}

	// work=200ms, idle=200ms, processing=50ms
	w := New(1, event.TeamCatering, mb, Routine{Work: 200 * time.Millisecond, Idle: 200 * time.Millisecond})
	w.ProcessingTime = 50 * time.Millisecond
	go w.Run(ctx)
	time.Sleep(10 * time.Millisecond) // allow worker goroutine to subscribe

	// Publish immediately (during work phase) with a long deadline.
	now := time.Now()
	ev := makeEvent(2, event.TypeCakeDelivery, event.PriorityMedium)
	ev.ReceivedAt = now
	ev.Deadline = now.Add(10 * time.Second)
	b, _ := json.Marshal(ev)
	_ = mb.Publish(ctx, broker.Message{Body: b, ContentType: "application/json"}, broker.PublishOptions{RoutingKey: organizer.TeamQueue(event.TeamCatering)})

	// Worker goes idle at ~200ms, processes the republished event, result by ~500ms.
	select {
	case d := <-resSub.Deliveries:
		var wrapper struct {
			Event event.Event `json:"event"`
		}
		if err := json.Unmarshal(d.Body, &wrapper); err != nil {
			t.Fatalf("unmarshal result: %v", err)
		}
		if wrapper.Event.ID != ev.ID {
			t.Fatalf("got event ID %d, want %d", wrapper.Event.ID, ev.ID)
		}
		if wrapper.Event.Status != event.StatusCompleted {
			t.Fatalf("expected completed, got %s", wrapper.Event.Status)
		}
	case <-time.After(800 * time.Millisecond):
		t.Fatal("timeout: event published during work phase was never processed")
	}
}

// TestWorkerReportsExpiredDuringWorkPhase verifies that an event received during
// the work phase that is already past its deadline is reported as expired.
func TestWorkerReportsExpiredDuringWorkPhase(t *testing.T) {
	mb := broker.NewMockBroker()
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	_, _ = mb.DeclareQueue(ctx, organizer.TeamQueue(event.TeamCatering), broker.QueueOptions{Durable: true})
	_, _ = mb.DeclareQueue(ctx, organizer.QueueResults, broker.QueueOptions{Durable: true})

	resSub, err := mb.Subscribe(ctx, broker.ConsumeOptions{Queue: organizer.QueueResults, AutoAck: true})
	if err != nil {
		t.Fatalf("subscribe results: %v", err)
	}

	w := New(1, event.TeamCatering, mb, Routine{Work: 200 * time.Millisecond, Idle: 200 * time.Millisecond})
	w.ProcessingTime = 50 * time.Millisecond
	go w.Run(ctx)
	time.Sleep(10 * time.Millisecond) // allow worker goroutine to subscribe

	// Publish an already-expired event during the work phase.
	past := time.Now().Add(-1 * time.Second)
	ev := makeEvent(3, event.TypeBarSetup, event.PriorityHigh)
	ev.ReceivedAt = past
	ev.Deadline = past.Add(500 * time.Millisecond) // already expired
	b, _ := json.Marshal(ev)
	_ = mb.Publish(ctx, broker.Message{Body: b, ContentType: "application/json"}, broker.PublishOptions{RoutingKey: organizer.TeamQueue(event.TeamCatering)})

	select {
	case d := <-resSub.Deliveries:
		var wrapper struct {
			Event event.Event `json:"event"`
		}
		if err := json.Unmarshal(d.Body, &wrapper); err != nil {
			t.Fatalf("unmarshal result: %v", err)
		}
		if wrapper.Event.Status != event.StatusExpired {
			t.Fatalf("expected expired, got %s", wrapper.Event.Status)
		}
	case <-time.After(400 * time.Millisecond):
		t.Fatal("timeout: expected expired event on results queue")
	}
}

// TestWorkerReportsExpirationDuringProcessing verifies that an event that expires
// while being processed is reported as expired.
func TestWorkerReportsExpirationDuringProcessing(t *testing.T) {
	mb := broker.NewMockBroker()
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	_, _ = mb.DeclareQueue(ctx, organizer.TeamQueue(event.TeamDecoration), broker.QueueOptions{Durable: true})
	_, _ = mb.DeclareQueue(ctx, organizer.QueueResults, broker.QueueOptions{Durable: true})

	resSub, _ := mb.Subscribe(ctx, broker.ConsumeOptions{Queue: organizer.QueueResults, AutoAck: true})

	// routine with short durations so we can test expiration during processing
	w := New(1, event.TeamDecoration, mb, Routine{Work: 50 * time.Millisecond, Idle: 200 * time.Millisecond})
	// processing longer than deadline gap to force expiration
	w.ProcessingTime = 150 * time.Millisecond
	go w.Run(ctx)

	// wait until idle
	time.Sleep(60 * time.Millisecond)

	ev := makeEvent(10, event.TypeTableSetup, event.PriorityHigh)
	now := time.Now()
	ev.ReceivedAt = now
	// short deadline so it will expire while being processed
	ev.Deadline = now.Add(100 * time.Millisecond)
	b, _ := json.Marshal(ev)
	_ = mb.Publish(ctx, broker.Message{Body: b, ContentType: "application/json"}, broker.PublishOptions{RoutingKey: organizer.TeamQueue(event.TeamDecoration)})

	// allow time for processing and publish
	time.Sleep(400 * time.Millisecond)

	// expect one result indicating expiration
	select {
	case d := <-resSub.Deliveries:
		var wrapper map[string]interface{}
		if err := json.Unmarshal(d.Body, &wrapper); err != nil {
			t.Fatalf("unmarshal result: %v", err)
		}
		// event should be marked expired
		if evmap, ok := wrapper["event"].(map[string]interface{}); ok {
			if status, _ := evmap["status"].(string); status != string(event.StatusExpired) {
				t.Fatalf("expected expired status, got %q", status)
			}
		} else {
			t.Fatalf("unexpected result payload")
		}
	default:
		t.Fatalf("expected an expired result, got none")
	}
}
