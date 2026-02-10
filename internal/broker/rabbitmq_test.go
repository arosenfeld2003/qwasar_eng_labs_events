package broker

import (
	"context"
	"testing"
)

func TestNewRabbitMQEmptyURL(t *testing.T) {
	_, err := NewRabbitMQ(context.Background(), RabbitMQConfig{})
	if err == nil {
		t.Fatal("expected error for empty URL")
	}
}

func TestNewRabbitMQInvalidURL(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping dial test in short mode")
	}

	_, err := NewRabbitMQ(context.Background(), RabbitMQConfig{
		URL:            "amqp://invalid:5672",
		ConnectionName: "test",
	})
	if err == nil {
		t.Fatal("expected error for unreachable host")
	}
}

func TestToTable(t *testing.T) {
	if got := toTable(nil); got != nil {
		t.Errorf("toTable(nil) = %v, want nil", got)
	}
	if got := toTable(map[string]interface{}{}); got != nil {
		t.Errorf("toTable(empty) = %v, want nil", got)
	}

	input := map[string]interface{}{"key": "value", "num": 42}
	result := toTable(input)
	if result == nil {
		t.Fatal("expected non-nil result")
	}
	if result["key"] != "value" {
		t.Errorf("expected key=value, got %v", result["key"])
	}
	if result["num"] != 42 {
		t.Errorf("expected num=42, got %v", result["num"])
	}
}

func TestFromTable(t *testing.T) {
	if got := fromTable(nil); got != nil {
		t.Errorf("fromTable(nil) = %v, want nil", got)
	}

	input := map[string]interface{}{"a": "b", "c": 3}
	result := fromTable(input)
	if result == nil {
		t.Fatal("expected non-nil result")
	}
	if result["a"] != "b" {
		t.Errorf("expected a=b, got %v", result["a"])
	}
	if result["c"] != 3 {
		t.Errorf("expected c=3, got %v", result["c"])
	}
}

func TestCloseChannelNil(t *testing.T) {
	// Should not panic
	closeChannel(nil)
}

func TestRabbitMQCloseWithoutConnection(t *testing.T) {
	r := &RabbitMQ{}
	if err := r.Close(); err != nil {
		t.Fatalf("Close with nil conn should not error: %v", err)
	}
}

func TestRabbitMQDoubleClose(t *testing.T) {
	r := &RabbitMQ{}
	if err := r.Close(); err != nil {
		t.Fatalf("first Close should not error: %v", err)
	}
	if err := r.Close(); err != nil {
		t.Fatalf("second Close should not error: %v", err)
	}
}

func TestRabbitMQOpenChannelWhenClosed(t *testing.T) {
	r := &RabbitMQ{closed: true}
	_, err := r.openChannel()
	if err == nil {
		t.Fatal("expected error when connection is closed")
	}
}

func TestRabbitMQOperationsWhenClosed(t *testing.T) {
	ctx := context.Background()
	r := &RabbitMQ{closed: true}

	if _, err := r.DeclareQueue(ctx, "q", QueueOptions{}); err == nil {
		t.Fatal("expected error from DeclareQueue on closed connection")
	}
	if err := r.DeclareExchange(ctx, "ex", ExchangeOptions{}); err == nil {
		t.Fatal("expected error from DeclareExchange on closed connection")
	}
	if err := r.BindQueue(ctx, "q", "ex", "rk", nil); err == nil {
		t.Fatal("expected error from BindQueue on closed connection")
	}
	if err := r.Publish(ctx, Message{}, PublishOptions{}); err == nil {
		t.Fatal("expected error from Publish on closed connection")
	}
	if _, err := r.Subscribe(ctx, ConsumeOptions{Queue: "q"}); err == nil {
		t.Fatal("expected error from Subscribe on closed connection")
	}
}
