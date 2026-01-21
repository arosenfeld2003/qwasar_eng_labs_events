package broker

import (
	"context"
	"testing"
	"time"
)

func TestMockBrokerPublishSubscribeDefaultExchange(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	broker := NewMockBroker()
	sub, err := broker.Subscribe(ctx, ConsumeOptions{Queue: "team.security"})
	if err != nil {
		t.Fatalf("subscribe: %v", err)
	}
	defer func() { _ = sub.Cancel() }()

	msg := Message{Body: []byte("hello")}
	if err := broker.Publish(ctx, msg, PublishOptions{RoutingKey: "team.security"}); err != nil {
		t.Fatalf("publish: %v", err)
	}

	delivery := receiveDelivery(t, sub.Deliveries)
	if string(delivery.Body) != "hello" {
		t.Fatalf("expected body hello, got %q", string(delivery.Body))
	}
}

func TestMockBrokerPublishSubscribeWithBinding(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	broker := NewMockBroker()
	if err := broker.DeclareExchange(ctx, "events", ExchangeOptions{}); err != nil {
		t.Fatalf("declare exchange: %v", err)
	}
	if _, err := broker.DeclareQueue(ctx, "team.security", QueueOptions{}); err != nil {
		t.Fatalf("declare queue: %v", err)
	}
	if err := broker.BindQueue(ctx, "team.security", "events", "security", nil); err != nil {
		t.Fatalf("bind queue: %v", err)
	}

	sub, err := broker.Subscribe(ctx, ConsumeOptions{Queue: "team.security"})
	if err != nil {
		t.Fatalf("subscribe: %v", err)
	}
	defer func() { _ = sub.Cancel() }()

	msg := Message{Body: []byte("incident")}
	if err := broker.Publish(ctx, msg, PublishOptions{Exchange: "events", RoutingKey: "security"}); err != nil {
		t.Fatalf("publish: %v", err)
	}

	delivery := receiveDelivery(t, sub.Deliveries)
	if string(delivery.Body) != "incident" {
		t.Fatalf("expected body incident, got %q", string(delivery.Body))
	}
}

func receiveDelivery(t *testing.T, deliveries <-chan Delivery) Delivery {
	t.Helper()

	select {
	case d := <-deliveries:
		return d
	case <-time.After(200 * time.Millisecond):
		t.Fatal("timed out waiting for delivery")
		return Delivery{}
	}
}
