package broker

import (
	"context"
	"time"
)

// Message represents a broker-agnostic payload.
type Message struct {
	Body          []byte
	ContentType   string
	CorrelationID string
	ReplyTo       string
	Headers       map[string]interface{}
	Timestamp     time.Time
	DeliveryMode  uint8
}

// PublishOptions defines routing metadata for a publish call.
type PublishOptions struct {
	Exchange   string
	RoutingKey string
	Mandatory  bool
	Immediate  bool
}

// ExchangeOptions defines exchange declaration settings.
type ExchangeOptions struct {
	Kind       string
	Durable    bool
	AutoDelete bool
	Internal   bool
	NoWait     bool
	Arguments  map[string]interface{}
}

// QueueOptions defines queue declaration settings.
type QueueOptions struct {
	Durable    bool
	AutoDelete bool
	Exclusive  bool
	NoWait     bool
	Arguments  map[string]interface{}
}

// QueueInfo exposes broker queue metadata.
type QueueInfo struct {
	Name      string
	Messages  int
	Consumers int
}

// ConsumeOptions defines subscription settings.
type ConsumeOptions struct {
	Queue     string
	Consumer  string
	AutoAck   bool
	Exclusive bool
	NoLocal   bool
	NoWait    bool
	Arguments map[string]interface{}
}

// Delivery is a broker-agnostic message delivery.
type Delivery struct {
	Message
	DeliveryTag uint64
	Redelivered bool
	Exchange    string
	RoutingKey  string
	Ack         func(multiple bool) error
	Nack        func(requeue bool) error
	Reject      func(requeue bool) error
}

// Subscription represents an active consumer subscription.
type Subscription struct {
	Deliveries <-chan Delivery
	Cancel     func() error
}

// Broker defines the contract for event transport.
type Broker interface {
	DeclareExchange(ctx context.Context, name string, opts ExchangeOptions) error
	DeclareQueue(ctx context.Context, name string, opts QueueOptions) (QueueInfo, error)
	BindQueue(ctx context.Context, queue, exchange, routingKey string, args map[string]interface{}) error
	Publish(ctx context.Context, msg Message, opts PublishOptions) error
	Subscribe(ctx context.Context, opts ConsumeOptions) (*Subscription, error)
	Close() error
}
