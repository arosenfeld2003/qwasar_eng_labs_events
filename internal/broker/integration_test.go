package broker

import (
	"context"
	"fmt"
	"os"
	"testing"
	"time"
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

// connectOrSkip creates a RabbitMQ connection, skipping if unavailable.
func connectOrSkip(t *testing.T) *RabbitMQ {
	t.Helper()
	url := rabbitmqURL(t)
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	rmq, err := NewRabbitMQ(ctx, RabbitMQConfig{
		URL:            url,
		ConnectionName: fmt.Sprintf("test-%s", t.Name()),
	})
	if err != nil {
		t.Skipf("RabbitMQ not available at %s: %v", url, err)
	}
	t.Cleanup(func() { rmq.Close() })
	return rmq
}

func TestRabbitMQConnect(t *testing.T) {
	rmq := connectOrSkip(t)
	if err := rmq.Close(); err != nil {
		t.Fatalf("close: %v", err)
	}
}

func TestRabbitMQDeclareExchange(t *testing.T) {
	rmq := connectOrSkip(t)
	ctx := context.Background()

	name := fmt.Sprintf("test-exchange-%d", time.Now().UnixNano())
	err := rmq.DeclareExchange(ctx, name, ExchangeOptions{
		Kind:       "direct",
		AutoDelete: true,
	})
	if err != nil {
		t.Fatalf("declare exchange: %v", err)
	}
}

func TestRabbitMQDeclareQueue(t *testing.T) {
	rmq := connectOrSkip(t)
	ctx := context.Background()

	name := fmt.Sprintf("test-queue-%d", time.Now().UnixNano())
	info, err := rmq.DeclareQueue(ctx, name, QueueOptions{
		AutoDelete: true,
	})
	if err != nil {
		t.Fatalf("declare queue: %v", err)
	}
	if info.Name != name {
		t.Errorf("expected queue name %s, got %s", name, info.Name)
	}
}

func TestRabbitMQPublishSubscribe(t *testing.T) {
	rmq := connectOrSkip(t)
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	queue := fmt.Sprintf("test-pubsub-%d", time.Now().UnixNano())
	if _, err := rmq.DeclareQueue(ctx, queue, QueueOptions{AutoDelete: true}); err != nil {
		t.Fatalf("declare queue: %v", err)
	}

	sub, err := rmq.Subscribe(ctx, ConsumeOptions{
		Queue:   queue,
		AutoAck: true,
	})
	if err != nil {
		t.Fatalf("subscribe: %v", err)
	}
	defer func() { _ = sub.Cancel() }()

	body := []byte(`{"event":"test"}`)
	err = rmq.Publish(ctx, Message{
		Body:        body,
		ContentType: "application/json",
	}, PublishOptions{RoutingKey: queue})
	if err != nil {
		t.Fatalf("publish: %v", err)
	}

	select {
	case d := <-sub.Deliveries:
		if string(d.Body) != string(body) {
			t.Errorf("expected body %q, got %q", body, d.Body)
		}
		if d.ContentType != "application/json" {
			t.Errorf("expected content type application/json, got %s", d.ContentType)
		}
	case <-ctx.Done():
		t.Fatal("timed out waiting for delivery")
	}
}

func TestRabbitMQExchangeRouting(t *testing.T) {
	rmq := connectOrSkip(t)
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	ts := time.Now().UnixNano()
	exchange := fmt.Sprintf("test-routing-ex-%d", ts)
	queue1 := fmt.Sprintf("test-routing-q1-%d", ts)
	queue2 := fmt.Sprintf("test-routing-q2-%d", ts)

	if err := rmq.DeclareExchange(ctx, exchange, ExchangeOptions{Kind: "direct", AutoDelete: true}); err != nil {
		t.Fatalf("declare exchange: %v", err)
	}
	if _, err := rmq.DeclareQueue(ctx, queue1, QueueOptions{AutoDelete: true}); err != nil {
		t.Fatalf("declare queue1: %v", err)
	}
	if _, err := rmq.DeclareQueue(ctx, queue2, QueueOptions{AutoDelete: true}); err != nil {
		t.Fatalf("declare queue2: %v", err)
	}
	if err := rmq.BindQueue(ctx, queue1, exchange, "catering", nil); err != nil {
		t.Fatalf("bind queue1: %v", err)
	}
	if err := rmq.BindQueue(ctx, queue2, exchange, "security", nil); err != nil {
		t.Fatalf("bind queue2: %v", err)
	}

	sub1, err := rmq.Subscribe(ctx, ConsumeOptions{Queue: queue1, AutoAck: true})
	if err != nil {
		t.Fatalf("subscribe q1: %v", err)
	}
	defer func() { _ = sub1.Cancel() }()

	sub2, err := rmq.Subscribe(ctx, ConsumeOptions{Queue: queue2, AutoAck: true})
	if err != nil {
		t.Fatalf("subscribe q2: %v", err)
	}
	defer func() { _ = sub2.Cancel() }()

	// Publish to catering routing key
	if err := rmq.Publish(ctx, Message{Body: []byte("food-event")}, PublishOptions{Exchange: exchange, RoutingKey: "catering"}); err != nil {
		t.Fatal(err)
	}
	// Publish to security routing key
	if err := rmq.Publish(ctx, Message{Body: []byte("security-event")}, PublishOptions{Exchange: exchange, RoutingKey: "security"}); err != nil {
		t.Fatal(err)
	}

	// queue1 should get catering
	select {
	case d := <-sub1.Deliveries:
		if string(d.Body) != "food-event" {
			t.Errorf("queue1: expected food-event, got %q", d.Body)
		}
	case <-ctx.Done():
		t.Fatal("timed out waiting for queue1 delivery")
	}

	// queue2 should get security
	select {
	case d := <-sub2.Deliveries:
		if string(d.Body) != "security-event" {
			t.Errorf("queue2: expected security-event, got %q", d.Body)
		}
	case <-ctx.Done():
		t.Fatal("timed out waiting for queue2 delivery")
	}
}

func TestRabbitMQManualAck(t *testing.T) {
	rmq := connectOrSkip(t)
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	queue := fmt.Sprintf("test-ack-%d", time.Now().UnixNano())
	if _, err := rmq.DeclareQueue(ctx, queue, QueueOptions{AutoDelete: true}); err != nil {
		t.Fatal(err)
	}

	sub, err := rmq.Subscribe(ctx, ConsumeOptions{
		Queue:   queue,
		AutoAck: false,
	})
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = sub.Cancel() }()

	if err := rmq.Publish(ctx, Message{Body: []byte("ack-me")}, PublishOptions{RoutingKey: queue}); err != nil {
		t.Fatal(err)
	}

	select {
	case d := <-sub.Deliveries:
		if err := d.Ack(false); err != nil {
			t.Fatalf("ack: %v", err)
		}
	case <-ctx.Done():
		t.Fatal("timed out")
	}
}

func TestRabbitMQPrefetch(t *testing.T) {
	url := rabbitmqURL(t)
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	rmq, err := NewRabbitMQ(ctx, RabbitMQConfig{
		URL:            url,
		ConnectionName: "test-prefetch",
		PrefetchCount:  10,
	})
	if err != nil {
		t.Skipf("RabbitMQ not available: %v", err)
	}
	defer rmq.Close()

	queue := fmt.Sprintf("test-prefetch-%d", time.Now().UnixNano())
	if _, err := rmq.DeclareQueue(ctx, queue, QueueOptions{AutoDelete: true}); err != nil {
		t.Fatal(err)
	}

	sub, err := rmq.Subscribe(ctx, ConsumeOptions{Queue: queue, AutoAck: true})
	if err != nil {
		t.Fatalf("subscribe with prefetch: %v", err)
	}
	_ = sub.Cancel()
}
