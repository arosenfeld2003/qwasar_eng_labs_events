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

func TestMockBrokerDeclareExchangeErrors(t *testing.T) {
	ctx := context.Background()
	b := NewMockBroker()

	if err := b.DeclareExchange(ctx, "", ExchangeOptions{}); err == nil {
		t.Fatal("expected error for empty exchange name")
	}

	b.Close()
	if err := b.DeclareExchange(ctx, "events", ExchangeOptions{}); err == nil {
		t.Fatal("expected error on closed broker")
	}
}

func TestMockBrokerDeclareQueueErrors(t *testing.T) {
	ctx := context.Background()
	b := NewMockBroker()

	if _, err := b.DeclareQueue(ctx, "", QueueOptions{}); err == nil {
		t.Fatal("expected error for empty queue name")
	}

	b.Close()
	if _, err := b.DeclareQueue(ctx, "q1", QueueOptions{}); err == nil {
		t.Fatal("expected error on closed broker")
	}
}

func TestMockBrokerDeclareQueueReturnsInfo(t *testing.T) {
	ctx := context.Background()
	b := NewMockBroker()

	info, err := b.DeclareQueue(ctx, "my-queue", QueueOptions{Durable: true})
	if err != nil {
		t.Fatalf("declare queue: %v", err)
	}
	if info.Name != "my-queue" {
		t.Errorf("expected queue name my-queue, got %s", info.Name)
	}
}

func TestMockBrokerBindQueueErrors(t *testing.T) {
	ctx := context.Background()
	b := NewMockBroker()

	if err := b.BindQueue(ctx, "q", "", "key", nil); err == nil {
		t.Fatal("expected error for empty exchange")
	}
	if err := b.BindQueue(ctx, "q", "ex", "", nil); err == nil {
		t.Fatal("expected error for empty routing key")
	}

	b.Close()
	if err := b.BindQueue(ctx, "q", "ex", "key", nil); err == nil {
		t.Fatal("expected error on closed broker")
	}
}

func TestMockBrokerBindQueueDuplicateIsIdempotent(t *testing.T) {
	ctx := context.Background()
	b := NewMockBroker()

	if err := b.BindQueue(ctx, "q1", "ex", "key", nil); err != nil {
		t.Fatal(err)
	}
	if err := b.BindQueue(ctx, "q1", "ex", "key", nil); err != nil {
		t.Fatal(err)
	}
}

func TestMockBrokerPublishOnClosedBroker(t *testing.T) {
	b := NewMockBroker()
	b.Close()

	err := b.Publish(context.Background(), Message{Body: []byte("x")}, PublishOptions{RoutingKey: "q"})
	if err == nil {
		t.Fatal("expected error on closed broker")
	}
}

func TestMockBrokerSubscribeErrors(t *testing.T) {
	ctx := context.Background()

	b := NewMockBroker()
	if _, err := b.Subscribe(ctx, ConsumeOptions{Queue: ""}); err == nil {
		t.Fatal("expected error for empty queue")
	}

	b.Close()
	if _, err := b.Subscribe(ctx, ConsumeOptions{Queue: "q1"}); err == nil {
		t.Fatal("expected error on closed broker")
	}
}

func TestMockBrokerPublished(t *testing.T) {
	ctx := context.Background()
	b := NewMockBroker()

	if len(b.Published()) != 0 {
		t.Fatal("expected no published messages initially")
	}

	msg := Message{Body: []byte("event-1"), ContentType: "application/json"}
	opts := PublishOptions{Exchange: "ex", RoutingKey: "key"}
	if err := b.Publish(ctx, msg, opts); err != nil {
		t.Fatal(err)
	}

	pubs := b.Published()
	if len(pubs) != 1 {
		t.Fatalf("expected 1 published message, got %d", len(pubs))
	}
	if string(pubs[0].Message.Body) != "event-1" {
		t.Errorf("unexpected body: %q", string(pubs[0].Message.Body))
	}
	if pubs[0].Options.Exchange != "ex" {
		t.Errorf("unexpected exchange: %s", pubs[0].Options.Exchange)
	}
	if pubs[0].Options.RoutingKey != "key" {
		t.Errorf("unexpected routing key: %s", pubs[0].Options.RoutingKey)
	}
}

func TestMockBrokerMultipleSubscribers(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	b := NewMockBroker()

	sub1, err := b.Subscribe(ctx, ConsumeOptions{Queue: "q1"})
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = sub1.Cancel() }()

	sub2, err := b.Subscribe(ctx, ConsumeOptions{Queue: "q1"})
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = sub2.Cancel() }()

	if err := b.Publish(ctx, Message{Body: []byte("fan-out")}, PublishOptions{RoutingKey: "q1"}); err != nil {
		t.Fatal(err)
	}

	d1 := receiveDelivery(t, sub1.Deliveries)
	d2 := receiveDelivery(t, sub2.Deliveries)

	if string(d1.Body) != "fan-out" || string(d2.Body) != "fan-out" {
		t.Error("both subscribers should receive the message")
	}
}

func TestMockBrokerPublishNoSubscribers(t *testing.T) {
	ctx := context.Background()
	b := NewMockBroker()

	err := b.Publish(ctx, Message{Body: []byte("nobody listening")}, PublishOptions{RoutingKey: "q"})
	if err != nil {
		t.Fatalf("publish should succeed even without subscribers: %v", err)
	}

	if len(b.Published()) != 1 {
		t.Error("message should still be recorded in Published()")
	}
}

func TestMockBrokerPublishNoRoutingKey(t *testing.T) {
	ctx := context.Background()
	b := NewMockBroker()

	err := b.Publish(ctx, Message{Body: []byte("x")}, PublishOptions{})
	if err != nil {
		t.Fatalf("publish with no routing key should succeed: %v", err)
	}
}

func TestMockBrokerCancelSubscription(t *testing.T) {
	ctx := context.Background()
	b := NewMockBroker()

	sub, err := b.Subscribe(ctx, ConsumeOptions{Queue: "q1"})
	if err != nil {
		t.Fatal(err)
	}

	if err := sub.Cancel(); err != nil {
		t.Fatalf("cancel: %v", err)
	}

	// Double cancel should be safe
	if err := sub.Cancel(); err != nil {
		t.Fatalf("double cancel: %v", err)
	}
}

func TestMockBrokerContextCancellation(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	b := NewMockBroker()

	sub, err := b.Subscribe(ctx, ConsumeOptions{Queue: "q1"})
	if err != nil {
		t.Fatal(err)
	}

	cancel()
	// Wait for the goroutine to close the channel
	time.Sleep(50 * time.Millisecond)

	// Channel should be closed, reads should return zero value
	select {
	case _, ok := <-sub.Deliveries:
		if ok {
			t.Error("expected channel to be closed after context cancel")
		}
	case <-time.After(200 * time.Millisecond):
		t.Fatal("timed out waiting for channel close")
	}
}

func TestMockBrokerCloseIdempotent(t *testing.T) {
	b := NewMockBroker()

	if err := b.Close(); err != nil {
		t.Fatal(err)
	}
	if err := b.Close(); err != nil {
		t.Fatal("second Close should not error")
	}
}

func TestMockBrokerCloseClosesSubscriberChannels(t *testing.T) {
	ctx := context.Background()
	b := NewMockBroker()

	sub, err := b.Subscribe(ctx, ConsumeOptions{Queue: "q1"})
	if err != nil {
		t.Fatal(err)
	}

	b.Close()

	select {
	case _, ok := <-sub.Deliveries:
		if ok {
			t.Error("expected delivery channel to be closed after broker Close()")
		}
	case <-time.After(200 * time.Millisecond):
		t.Fatal("timed out waiting for delivery channel close")
	}
}

func TestMockBrokerDeliveryCallbacks(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	b := NewMockBroker()
	sub, err := b.Subscribe(ctx, ConsumeOptions{Queue: "q1"})
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = sub.Cancel() }()

	b.Publish(ctx, Message{Body: []byte("test")}, PublishOptions{RoutingKey: "q1"})
	d := receiveDelivery(t, sub.Deliveries)

	if err := d.Ack(false); err != nil {
		t.Errorf("Ack should not error: %v", err)
	}
	if err := d.Nack(false); err != nil {
		t.Errorf("Nack should not error: %v", err)
	}
	if err := d.Reject(false); err != nil {
		t.Errorf("Reject should not error: %v", err)
	}
}

func TestMockBrokerDeliveryMetadata(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	b := NewMockBroker()
	b.DeclareExchange(ctx, "events", ExchangeOptions{})
	b.DeclareQueue(ctx, "q1", QueueOptions{})
	b.BindQueue(ctx, "q1", "events", "rk", nil)

	sub, err := b.Subscribe(ctx, ConsumeOptions{Queue: "q1"})
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = sub.Cancel() }()

	b.Publish(ctx, Message{Body: []byte("meta")}, PublishOptions{Exchange: "events", RoutingKey: "rk"})
	d := receiveDelivery(t, sub.Deliveries)

	if d.Exchange != "events" {
		t.Errorf("expected exchange 'events', got %q", d.Exchange)
	}
	if d.RoutingKey != "rk" {
		t.Errorf("expected routing key 'rk', got %q", d.RoutingKey)
	}
}

func TestMockBrokerPublishToUnboundExchange(t *testing.T) {
	ctx := context.Background()
	b := NewMockBroker()

	b.DeclareExchange(ctx, "orphan", ExchangeOptions{})
	sub, _ := b.Subscribe(ctx, ConsumeOptions{Queue: "q1"})
	defer func() { _ = sub.Cancel() }()

	err := b.Publish(ctx, Message{Body: []byte("lost")}, PublishOptions{Exchange: "orphan", RoutingKey: "x"})
	if err != nil {
		t.Fatalf("publish should succeed even if no bindings match: %v", err)
	}

	// No delivery expected
	select {
	case d := <-sub.Deliveries:
		t.Fatalf("unexpected delivery: %q", string(d.Body))
	case <-time.After(50 * time.Millisecond):
		// expected
	}
}

func TestMockBrokerInterfaceCompliance(t *testing.T) {
	var _ Broker = (*MockBroker)(nil)
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
