package broker

import (
	"context"
	"testing"
	"time"

	"marry-me/internal/config"
	"marry-me/internal/event"
)

// Note: These tests require a running RabbitMQ instance.
// For unit testing without RabbitMQ, use mock implementations.

func TestRabbitMQ_Integration(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping integration test in short mode")
	}

	cfg := &config.Config{
		RabbitMQURL:  "amqp://guest:guest@localhost:5672/",
		ExchangeName: "marry-me-test",
	}

	rmq, err := NewRabbitMQ(cfg)
	if err != nil {
		t.Skipf("Skipping test - RabbitMQ not available: %v", err)
	}
	defer rmq.Close()

	ctx := context.Background()
	testQueue := "test-queue-" + time.Now().Format("20060102150405")

	// Test DeclareQueue
	t.Run("DeclareQueue", func(t *testing.T) {
		err := rmq.DeclareQueue(ctx, testQueue)
		if err != nil {
			t.Fatalf("DeclareQueue failed: %v", err)
		}
	})

	// Test BindQueue
	t.Run("BindQueue", func(t *testing.T) {
		err := rmq.BindQueue(ctx, testQueue, cfg.ExchangeName, testQueue)
		if err != nil {
			t.Fatalf("BindQueue failed: %v", err)
		}
	})

	// Test Publish and Subscribe
	t.Run("PublishSubscribe", func(t *testing.T) {
		testEvent := &event.Event{
			ID:        "test-001",
			Type:      event.EventMealService,
			Priority:  event.PriorityHigh,
			Timestamp: time.Now().Unix(),
		}

		received := make(chan *event.Event, 1)

		err := rmq.Subscribe(ctx, testQueue, func(ctx context.Context, e *event.Event) error {
			received <- e
			return nil
		})
		if err != nil {
			t.Fatalf("Subscribe failed: %v", err)
		}

		// Give consumer time to start
		time.Sleep(100 * time.Millisecond)

		err = rmq.Publish(ctx, testQueue, testEvent)
		if err != nil {
			t.Fatalf("Publish failed: %v", err)
		}

		select {
		case e := <-received:
			if e.ID != testEvent.ID {
				t.Errorf("Received event ID = %q, want %q", e.ID, testEvent.ID)
			}
		case <-time.After(5 * time.Second):
			t.Error("Timeout waiting for event")
		}
	})
}

func TestMockBroker(t *testing.T) {
	mock := NewMockBroker()
	ctx := context.Background()

	received := make(chan *event.Event, 1)
	mock.Subscribe(ctx, "test-queue", func(ctx context.Context, e *event.Event) error {
		received <- e
		return nil
	})

	testEvent := &event.Event{
		ID:   "test-001",
		Type: event.EventMealService,
	}

	mock.Publish(ctx, "test-queue", testEvent)

	select {
	case e := <-received:
		if e.ID != testEvent.ID {
			t.Errorf("Received event ID = %q, want %q", e.ID, testEvent.ID)
		}
	case <-time.After(time.Second):
		t.Error("Timeout waiting for event")
	}

	if len(mock.GetPublished()) != 1 {
		t.Errorf("Published count = %d, want 1", len(mock.GetPublished()))
	}
}
