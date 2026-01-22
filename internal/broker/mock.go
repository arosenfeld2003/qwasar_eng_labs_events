package broker

import (
	"context"
	"errors"
	"sync"
	"time"
)

// PublishedMessage captures mock publish calls for assertions.
type PublishedMessage struct {
	Message Message
	Options PublishOptions
	Time    time.Time
}

// MockBroker is an in-memory Broker implementation for tests.
type MockBroker struct {
	mu         sync.Mutex
	queues     map[string][]chan Delivery
	bindings   map[string]map[string][]string
	exchanges  map[string]ExchangeOptions
	queueDefs  map[string]QueueOptions
	published  []PublishedMessage
	closed     bool
}

// NewMockBroker constructs an in-memory broker.
func NewMockBroker() *MockBroker {
	return &MockBroker{
		queues:    map[string][]chan Delivery{},
		bindings:  map[string]map[string][]string{},
		exchanges: map[string]ExchangeOptions{},
		queueDefs: map[string]QueueOptions{},
	}
}

// Published returns a snapshot of published messages.
func (m *MockBroker) Published() []PublishedMessage {
	m.mu.Lock()
	defer m.mu.Unlock()

	out := make([]PublishedMessage, len(m.published))
	copy(out, m.published)
	return out
}

// DeclareExchange stores exchange metadata.
func (m *MockBroker) DeclareExchange(_ context.Context, name string, opts ExchangeOptions) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	if m.closed {
		return errors.New("mock broker is closed")
	}
	if name == "" {
		return errors.New("exchange name is required")
	}
	m.exchanges[name] = opts
	return nil
}

// DeclareQueue stores queue metadata.
func (m *MockBroker) DeclareQueue(_ context.Context, name string, opts QueueOptions) (QueueInfo, error) {
	m.mu.Lock()
	defer m.mu.Unlock()

	if m.closed {
		return QueueInfo{}, errors.New("mock broker is closed")
	}
	if name == "" {
		return QueueInfo{}, errors.New("queue name is required")
	}
	m.queueDefs[name] = opts
	return QueueInfo{Name: name}, nil
}

// BindQueue records exchange routing bindings.
func (m *MockBroker) BindQueue(_ context.Context, queue, exchange, routingKey string, _ map[string]interface{}) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	if m.closed {
		return errors.New("mock broker is closed")
	}
	if exchange == "" {
		return errors.New("exchange name is required")
	}
	if routingKey == "" {
		return errors.New("routing key is required")
	}
	if _, ok := m.bindings[exchange]; !ok {
		m.bindings[exchange] = map[string][]string{}
	}
	m.bindings[exchange][routingKey] = appendUnique(m.bindings[exchange][routingKey], queue)
	return nil
}

// Publish sends a message to all bound subscriptions.
func (m *MockBroker) Publish(_ context.Context, msg Message, opts PublishOptions) error {
	m.mu.Lock()
	if m.closed {
		m.mu.Unlock()
		return errors.New("mock broker is closed")
	}

	m.published = append(m.published, PublishedMessage{Message: msg, Options: opts, Time: time.Now()})
	targets := m.resolveTargetsLocked(opts)
	subscribers := collectSubscribersLocked(m.queues, targets)
	m.mu.Unlock()

	delivery := Delivery{
		Message:    msg,
		Exchange:   opts.Exchange,
		RoutingKey: opts.RoutingKey,
		Ack: func(bool) error {
			return nil
		},
		Nack: func(bool) error {
			return nil
		},
		Reject: func(bool) error {
			return nil
		},
	}

	for _, ch := range subscribers {
		select {
		case ch <- delivery:
		default:
			// Drop when buffer is full to keep tests deterministic.
		}
	}
	return nil
}

// Subscribe registers a consumer for a queue.
func (m *MockBroker) Subscribe(ctx context.Context, opts ConsumeOptions) (*Subscription, error) {
	if opts.Queue == "" {
		return nil, errors.New("queue name is required")
	}

	m.mu.Lock()
	if m.closed {
		m.mu.Unlock()
		return nil, errors.New("mock broker is closed")
	}
	ch := make(chan Delivery, 128)
	m.queues[opts.Queue] = append(m.queues[opts.Queue], ch)
	m.mu.Unlock()

	var cancelOnce sync.Once
	cancel := func() error {
		cancelOnce.Do(func() {
			m.removeSubscriber(opts.Queue, ch)
			close(ch)
		})
		return nil
	}

	go func() {
		<-ctx.Done()
		_ = cancel()
	}()

	return &Subscription{Deliveries: ch, Cancel: cancel}, nil
}

// Close shuts down the mock broker.
func (m *MockBroker) Close() error {
	m.mu.Lock()
	if m.closed {
		m.mu.Unlock()
		return nil
	}
	m.closed = true
	queues := m.queues
	m.queues = map[string][]chan Delivery{}
	m.mu.Unlock()

	for _, subs := range queues {
		for _, ch := range subs {
			close(ch)
		}
	}
	return nil
}

func (m *MockBroker) resolveTargetsLocked(opts PublishOptions) []string {
	if opts.Exchange == "" {
		if opts.RoutingKey == "" {
			return nil
		}
		return []string{opts.RoutingKey}
	}
	bindings := m.bindings[opts.Exchange]
	if len(bindings) == 0 {
		return nil
	}
	return bindings[opts.RoutingKey]
}

func collectSubscribersLocked(queues map[string][]chan Delivery, targets []string) []chan Delivery {
	var subs []chan Delivery
	for _, queue := range targets {
		subs = append(subs, queues[queue]...)
	}
	return subs
}

func (m *MockBroker) removeSubscriber(queue string, ch chan Delivery) {
	m.mu.Lock()
	defer m.mu.Unlock()

	subs := m.queues[queue]
	for idx, candidate := range subs {
		if candidate == ch {
			m.queues[queue] = append(subs[:idx], subs[idx+1:]...)
			return
		}
	}
}

func appendUnique(values []string, value string) []string {
	for _, existing := range values {
		if existing == value {
			return values
		}
	}
	return append(values, value)
}

var _ Broker = (*MockBroker)(nil)
