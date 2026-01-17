package broker

import (
	"context"

	"marry-me/internal/event"
)

// EventHandler is a callback function for processing events
type EventHandler func(ctx context.Context, e *event.Event) error

// Broker defines the interface for message broker operations
type Broker interface {
	// Publish sends an event to the specified routing key
	Publish(ctx context.Context, routingKey string, e *event.Event) error

	// Subscribe starts consuming events from a queue
	Subscribe(ctx context.Context, queue string, handler EventHandler) error

	// DeclareQueue creates a queue if it doesn't exist
	DeclareQueue(ctx context.Context, name string) error

	// BindQueue binds a queue to an exchange with a routing key
	BindQueue(ctx context.Context, queue, exchange, routingKey string) error

	// Close closes the broker connection
	Close() error
}

// BrokerStats holds statistics about broker operations
type BrokerStats struct {
	MessagesPublished int64
	MessagesConsumed  int64
	Errors            int64
}
