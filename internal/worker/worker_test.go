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

func TestWorkerAcceptsOnlyDuringIdle(t *testing.T) {
    mb := broker.NewMockBroker()
    ctx, cancel := context.WithCancel(context.Background())
    defer cancel()

    // declare queues
    _, _ = mb.DeclareQueue(ctx, organizer.TeamQueue(event.TeamCatering), broker.QueueOptions{Durable: true})
    _, _ = mb.DeclareQueue(ctx, organizer.QueueResults, broker.QueueOptions{Durable: true})

    // subscribe to results to observe processed events
    resSub, err := mb.Subscribe(ctx, broker.ConsumeOptions{Queue: organizer.QueueResults, AutoAck: true})
    if err != nil {
        t.Fatalf("subscribe results: %v", err)
    }

    // short routine for test: work 200ms, idle 200ms
    w := New(1, event.TeamCatering, mb, Routine{Work: 200 * time.Millisecond, Idle: 200 * time.Millisecond})
    w.ProcessingTime = 100 * time.Millisecond

    go w.Run(ctx)

    // event published during initial work phase -> should be ignored
    ev1 := makeEvent(1, event.TypeMealService, event.PriorityMedium)
    now := time.Now()
    ev1.ReceivedAt = now
    ev1.Deadline = now.Add(5 * time.Second)
    b1, _ := json.Marshal(ev1)
    _ = mb.Publish(ctx, broker.Message{Body: b1, ContentType: "application/json"}, broker.PublishOptions{RoutingKey: organizer.TeamQueue(event.TeamCatering)})

    // wait until idle phase
    time.Sleep(250 * time.Millisecond)

    // publish event during idle -> should be processed
    ev2 := makeEvent(2, event.TypeCakeDelivery, event.PriorityLow)
    now2 := time.Now()
    ev2.ReceivedAt = now2
    ev2.Deadline = now2.Add(5 * time.Second)
    b2, _ := json.Marshal(ev2)
    _ = mb.Publish(ctx, broker.Message{Body: b2, ContentType: "application/json"}, broker.PublishOptions{RoutingKey: organizer.TeamQueue(event.TeamCatering)})

    // allow processing to complete
    time.Sleep(400 * time.Millisecond)

    // read results - expect exactly one processed event (ev2) as a ResultEvent wrapper
    select {
    case d := <-resSub.Deliveries:
        var wrapper struct {
            Event event.Event `json:"event"`
        }
        if err := json.Unmarshal(d.Body, &wrapper); err == nil {
            if wrapper.Event.ID == ev1.ID {
                t.Fatalf("unexpected: processed event that was published during work phase")
            }
            if wrapper.Event.ID != ev2.ID {
                t.Fatalf("got event ID %d, want %d", wrapper.Event.ID, ev2.ID)
            }
        }
    default:
        t.Fatalf("expected 1 processed event, got none")
    }
}

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
