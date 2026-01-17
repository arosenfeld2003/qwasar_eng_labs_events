package broker

import (
	"context"
	"sync"

	"marry-me/internal/event"
)

// MockBroker is a mock implementation for unit testing
type MockBroker struct {
	mu        sync.Mutex
	published []*event.Event
	handlers  map[string]EventHandler
}

// NewMockBroker creates a new mock broker instance
func NewMockBroker() *MockBroker {
	return &MockBroker{
		published: make([]*event.Event, 0),
		handlers:  make(map[string]EventHandler),
	}
}

// Publish records the published event and optionally calls a handler
func (m *MockBroker) Publish(ctx context.Context, routingKey string, e *event.Event) error {
	m.mu.Lock()
	m.published = append(m.published, e)
	handler := m.handlers[routingKey]
	m.mu.Unlock()

	// If there's a handler for this routing key, call it
	if handler != nil {
		return handler(ctx, e)
	}
	return nil
}

// Subscribe registers a handler for the given queue
func (m *MockBroker) Subscribe(ctx context.Context, queue string, handler EventHandler) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.handlers[queue] = handler
	return nil
}

// DeclareQueue is a no-op for mock
func (m *MockBroker) DeclareQueue(ctx context.Context, name string) error {
	return nil
}

// BindQueue is a no-op for mock
func (m *MockBroker) BindQueue(ctx context.Context, queue, exchange, routingKey string) error {
	return nil
}

// Close is a no-op for mock
func (m *MockBroker) Close() error {
	return nil
}

// GetPublished returns all published events
func (m *MockBroker) GetPublished() []*event.Event {
	m.mu.Lock()
	defer m.mu.Unlock()
	result := make([]*event.Event, len(m.published))
	copy(result, m.published)
	return result
}

// ClearPublished clears the published events list
func (m *MockBroker) ClearPublished() {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.published = make([]*event.Event, 0)
}

// SimulateMessage simulates receiving a message on a queue
func (m *MockBroker) SimulateMessage(ctx context.Context, queue string, e *event.Event) error {
	m.mu.Lock()
	handler := m.handlers[queue]
	m.mu.Unlock()

	if handler != nil {
		return handler(ctx, e)
	}
	return nil
}
