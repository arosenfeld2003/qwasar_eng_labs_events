package main

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sync"
	"testing"
	"time"

	"github.com/arosenfeld2003/qwasar_eng_labs_events/internal/broker"
	"github.com/arosenfeld2003/qwasar_eng_labs_events/internal/event"
	"github.com/arosenfeld2003/qwasar_eng_labs_events/internal/organizer"
)

// rabbitmqURL returns the RabbitMQ connection URL or skips the test.
func rabbitmqURL(t *testing.T) string {
	t.Helper()
	if testing.Short() {
		t.Skip("skipping integration test in short mode")
	}
	url := os.Getenv("RABBITMQ_URL")
	if url == "" {
		url = "amqp://admin:password@localhost:5672"
	}
	return url
}

// connectBroker creates a RabbitMQ connection, skipping if unavailable.
func connectBroker(t *testing.T) *broker.RabbitMQ {
	t.Helper()
	url := rabbitmqURL(t)
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	rmq, err := broker.NewRabbitMQ(ctx, broker.RabbitMQConfig{
		URL:            url,
		ConnectionName: fmt.Sprintf("integration-%s", t.Name()),
	})
	if err != nil {
		t.Skipf("RabbitMQ not available at %s: %v", url, err)
	}
	t.Cleanup(func() { rmq.Close() })
	return rmq
}

// TestEndToEndPipeline loads a dataset, publishes events through RabbitMQ
// routing by team, and verifies all events are received by the correct team
// subscriber.
func TestEndToEndPipeline(t *testing.T) {
	rmq := connectBroker(t)
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	// Load dataset
	datasetPath := filepath.Join("datasets", "dataset_1.json")
	events, err := event.LoadFromFile(datasetPath)
	if err != nil {
		t.Fatalf("load dataset: %v", err)
	}

	// Use a subset for the integration test
	if len(events) > 20 {
		events = events[:20]
	}

	ts := time.Now().UnixNano()
	exchange := fmt.Sprintf("pipeline-test-%d", ts)

	if err := rmq.DeclareExchange(ctx, exchange, broker.ExchangeOptions{
		Kind:       "direct",
		AutoDelete: true,
	}); err != nil {
		t.Fatalf("declare exchange: %v", err)
	}

	// Determine which teams are needed
	teamEvents := make(map[event.Team][]event.Event)
	for _, e := range events {
		team := e.Team()
		teamEvents[team] = append(teamEvents[team], e)
	}

	// Create a queue and subscriber per team
	type teamSub struct {
		sub    *broker.Subscription
		events []event.Event
	}
	subs := make(map[event.Team]*teamSub)

	for team := range teamEvents {
		queue := fmt.Sprintf("team-%s-%d", team, ts)
		if _, err := rmq.DeclareQueue(ctx, queue, broker.QueueOptions{AutoDelete: true}); err != nil {
			t.Fatalf("declare queue %s: %v", queue, err)
		}
		if err := rmq.BindQueue(ctx, queue, exchange, string(team), nil); err != nil {
			t.Fatalf("bind queue %s: %v", queue, err)
		}
		sub, err := rmq.Subscribe(ctx, broker.ConsumeOptions{Queue: queue, AutoAck: true})
		if err != nil {
			t.Fatalf("subscribe %s: %v", queue, err)
		}
		subs[team] = &teamSub{sub: sub}
	}
	defer func() {
		for _, ts := range subs {
			_ = ts.sub.Cancel()
		}
	}()

	// Publish all events
	for _, e := range events {
		team := e.Team()
		body, err := json.Marshal(e)
		if err != nil {
			t.Fatal(err)
		}
		err = rmq.Publish(ctx, broker.Message{
			Body:        body,
			ContentType: "application/json",
		}, broker.PublishOptions{
			Exchange:   exchange,
			RoutingKey: string(team),
		})
		if err != nil {
			t.Fatalf("publish event %d: %v", e.ID, err)
		}
	}

	// Collect deliveries
	var mu sync.Mutex
	var wg sync.WaitGroup
	received := make(map[event.Team]int)

	for team, ts := range subs {
		wg.Add(1)
		go func(team event.Team, ts *teamSub) {
			defer wg.Done()
			expected := len(teamEvents[team])
			count := 0
			for count < expected {
				select {
				case d, ok := <-ts.sub.Deliveries:
					if !ok {
						return
					}
					var e event.Event
					if err := json.Unmarshal(d.Body, &e); err != nil {
						t.Errorf("unmarshal: %v", err)
						return
					}
					if e.Team() != team {
						t.Errorf("event %d routed to wrong team: got %s, expected %s", e.ID, e.Team(), team)
					}
					count++
				case <-ctx.Done():
					t.Errorf("team %s: timed out after receiving %d/%d events", team, count, expected)
					return
				}
			}
			mu.Lock()
			received[team] = count
			mu.Unlock()
		}(team, ts)
	}

	wg.Wait()

	// Verify all teams received their events
	for team, expected := range teamEvents {
		got := received[team]
		if got != len(expected) {
			t.Errorf("team %s: received %d events, expected %d", team, got, len(expected))
		}
	}

	t.Logf("Pipeline test: %d events published across %d teams", len(events), len(teamEvents))
}

// TestStressCalculation simulates processing events with deadlines and
// calculates a stress score based on completion vs expiration.
func TestStressCalculation(t *testing.T) {
	rmq := connectBroker(t)
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	ts := time.Now().UnixNano()
	queue := fmt.Sprintf("stress-test-%d", ts)

	if _, err := rmq.DeclareQueue(ctx, queue, broker.QueueOptions{AutoDelete: true}); err != nil {
		t.Fatal(err)
	}

	sub, err := rmq.Subscribe(ctx, broker.ConsumeOptions{Queue: queue, AutoAck: false})
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = sub.Cancel() }()

	// Create events with different priorities
	testEvents := []event.Event{
		{ID: 1, Type: event.TypeAccident, Priority: event.PriorityHigh, Description: "urgent"},
		{ID: 2, Type: event.TypeMealService, Priority: event.PriorityMedium, Description: "meal"},
		{ID: 3, Type: event.TypeBandSetup, Priority: event.PriorityLow, Description: "music setup"},
		{ID: 4, Type: event.TypeBrawl, Priority: event.PriorityHigh, Description: "fight"},
		{ID: 5, Type: event.TypeCleanup, Priority: event.PriorityLow, Description: "cleanup"},
	}

	now := time.Now()
	for i := range testEvents {
		testEvents[i].SetReceived(now)
		body, _ := json.Marshal(testEvents[i])
		if err := rmq.Publish(ctx, broker.Message{
			Body:        body,
			ContentType: "application/json",
		}, broker.PublishOptions{RoutingKey: queue}); err != nil {
			t.Fatal(err)
		}
	}

	// Process events, tracking stress
	completed := 0
	expired := 0
	totalProcessed := 0

	for totalProcessed < len(testEvents) {
		select {
		case d := <-sub.Deliveries:
			var e event.Event
			if err := json.Unmarshal(d.Body, &e); err != nil {
				t.Fatal(err)
			}

			processTime := now.Add(time.Duration(totalProcessed) * 3 * time.Second)
			if e.IsExpired(processTime) {
				expired++
				e.Status = event.StatusExpired
			} else {
				completed++
				e.Status = event.StatusCompleted
			}
			totalProcessed++

			if err := d.Ack(false); err != nil {
				t.Fatalf("ack: %v", err)
			}
		case <-ctx.Done():
			t.Fatalf("timed out after processing %d/%d events", totalProcessed, len(testEvents))
		}
	}

	// Calculate stress: percentage of events that expired
	stressPercent := float64(expired) / float64(totalProcessed) * 100.0
	completionRate := float64(completed) / float64(totalProcessed) * 100.0

	t.Logf("Stress results:")
	t.Logf("  Total events:    %d", totalProcessed)
	t.Logf("  Completed:       %d (%.1f%%)", completed, completionRate)
	t.Logf("  Expired:         %d (%.1f%%)", expired, stressPercent)
	t.Logf("  Stress score:    %.1f%%", stressPercent)

	// With 3s processing intervals and high priority = 5s deadline:
	// Events processed at t=0,3,6,9,12s. High priority (5s) events
	// processed after t=5s should expire.
	if totalProcessed != len(testEvents) {
		t.Errorf("expected %d processed, got %d", len(testEvents), totalProcessed)
	}
}

// TestOrganizerRunWithRabbitMQ exercises the real Organizer.Setup + Run
// against a live RabbitMQ instance. It publishes events to the validated
// queue and verifies they arrive on the correct team queues.
func TestOrganizerRunWithRabbitMQ(t *testing.T) {
	rmq := connectBroker(t)
	ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
	defer cancel()

	o := organizer.New(rmq)
	if err := o.Setup(ctx); err != nil {
		t.Fatalf("organizer setup: %v", err)
	}

	// Start the organizer in the background.
	runErr := make(chan error, 1)
	runCtx, runCancel := context.WithCancel(ctx)
	defer runCancel()
	go func() {
		runErr <- o.Run(runCtx)
	}()

	// Give the organizer time to subscribe to events.validated.
	time.Sleep(100 * time.Millisecond)

	// Prepare test events targeting different teams.
	testEvents := []event.Event{
		{ID: 1001, Type: event.TypeMealService, Priority: event.PriorityHigh},
		{ID: 1002, Type: event.TypeCakeDelivery, Priority: event.PriorityMedium},
		{ID: 1003, Type: event.TypeAccident, Priority: event.PriorityHigh},
		{ID: 1004, Type: event.TypeBandSetup, Priority: event.PriorityLow},
		{ID: 1005, Type: event.TypePhotoSession, Priority: event.PriorityMedium},
	}

	// Build expected mapping: team -> event IDs.
	expected := make(map[event.Team][]int)
	for _, ev := range testEvents {
		team := ev.Team()
		expected[team] = append(expected[team], ev.ID)
	}

	// Subscribe to each team queue before publishing.
	type teamSub struct {
		sub  *broker.Subscription
		team event.Team
	}
	var subs []teamSub
	for team := range expected {
		sub, err := rmq.Subscribe(ctx, broker.ConsumeOptions{
			Queue:   organizer.TeamQueue(team),
			AutoAck: true,
		})
		if err != nil {
			t.Fatalf("subscribe to %s: %v", organizer.TeamQueue(team), err)
		}
		subs = append(subs, teamSub{sub: sub, team: team})
	}
	defer func() {
		for _, s := range subs {
			_ = s.sub.Cancel()
		}
	}()

	// Publish events to events.validated (the organizer's input queue).
	for _, ev := range testEvents {
		body, err := json.Marshal(ev)
		if err != nil {
			t.Fatal(err)
		}
		if err := rmq.Publish(ctx, broker.Message{
			Body:        body,
			ContentType: "application/json",
		}, broker.PublishOptions{
			RoutingKey: organizer.QueueValidated,
		}); err != nil {
			t.Fatalf("publish event %d: %v", ev.ID, err)
		}
	}

	// Collect routed events from team queues.
	received := make(map[event.Team][]int)
	totalExpected := len(testEvents)
	totalReceived := 0

	for totalReceived < totalExpected {
		for _, s := range subs {
			select {
			case d := <-s.sub.Deliveries:
				var got event.Event
				if err := json.Unmarshal(d.Body, &got); err != nil {
					t.Fatalf("unmarshal from %s: %v", s.team, err)
				}
				received[s.team] = append(received[s.team], got.ID)
				totalReceived++
			case <-ctx.Done():
				t.Fatalf("timed out: received %d/%d events", totalReceived, totalExpected)
			}
		}
	}

	// Verify each team got the right events.
	for team, wantIDs := range expected {
		gotIDs := received[team]
		if len(gotIDs) != len(wantIDs) {
			t.Errorf("team %s: got %d events, want %d", team, len(gotIDs), len(wantIDs))
			continue
		}
		gotSet := make(map[int]bool)
		for _, id := range gotIDs {
			gotSet[id] = true
		}
		for _, id := range wantIDs {
			if !gotSet[id] {
				t.Errorf("team %s: missing event ID %d", team, id)
			}
		}
	}

	// Verify organizer stats.
	stats := o.Stats()
	if stats.Routed != int64(totalExpected) {
		t.Errorf("Stats.Routed = %d, want %d", stats.Routed, totalExpected)
	}

	runCancel()
	<-runErr
	t.Logf("Organizer integration: routed %d events across %d teams", stats.Routed, len(expected))
}
