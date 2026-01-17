package event

import (
	"encoding/json"
	"fmt"
	"time"
)

// Event represents a wedding event to be processed
type Event struct {
	ID          string    `json:"id"`
	Type        EventType `json:"type"`
	Priority    Priority  `json:"priority"`
	Timestamp   int64     `json:"timestamp"`
	Description string    `json:"description,omitempty"`
	ReceivedAt  time.Time `json:"-"`
	Deadline    time.Time `json:"-"`
	Status      Status    `json:"-"`
}

// Status represents the processing status of an event
type Status int

const (
	StatusPending Status = iota
	StatusProcessing
	StatusCompleted
	StatusExpired
)

// String returns the string representation of Status
func (s Status) String() string {
	switch s {
	case StatusPending:
		return "pending"
	case StatusProcessing:
		return "processing"
	case StatusCompleted:
		return "completed"
	case StatusExpired:
		return "expired"
	default:
		return "unknown"
	}
}

// eventJSON is used for custom JSON unmarshaling
type eventJSON struct {
	ID          string `json:"id"`
	Type        string `json:"type"`
	Priority    string `json:"priority"`
	Timestamp   int64  `json:"timestamp"`
	Description string `json:"description,omitempty"`
}

// UnmarshalJSON implements custom JSON unmarshaling for Event
func (e *Event) UnmarshalJSON(data []byte) error {
	var ej eventJSON
	if err := json.Unmarshal(data, &ej); err != nil {
		return fmt.Errorf("failed to unmarshal event: %w", err)
	}

	e.ID = ej.ID
	e.Type = EventType(ej.Type)
	e.Priority = ParsePriority(ej.Priority)
	e.Timestamp = ej.Timestamp
	e.Description = ej.Description
	e.Status = StatusPending

	return nil
}

// MarshalJSON implements custom JSON marshaling for Event
func (e Event) MarshalJSON() ([]byte, error) {
	return json.Marshal(eventJSON{
		ID:          e.ID,
		Type:        string(e.Type),
		Priority:    e.Priority.String(),
		Timestamp:   e.Timestamp,
		Description: e.Description,
	})
}

// Dataset represents the structure of the input dataset file
type Dataset struct {
	Events []Event `json:"events"`
}

// ParseDataset parses a JSON dataset into events
func ParseDataset(data []byte) (*Dataset, error) {
	var dataset Dataset
	if err := json.Unmarshal(data, &dataset); err != nil {
		return nil, fmt.Errorf("failed to parse dataset: %w", err)
	}
	return &dataset, nil
}

// DeadlineDuration returns the deadline duration based on priority
func DeadlineDuration(p Priority) time.Duration {
	switch p {
	case PriorityHigh:
		return 5 * time.Second
	case PriorityMedium:
		return 10 * time.Second
	case PriorityLow:
		return 20 * time.Second
	default:
		return 20 * time.Second
	}
}

// SetDeadline sets the deadline based on priority and received time
func (e *Event) SetDeadline() {
	e.Deadline = e.ReceivedAt.Add(DeadlineDuration(e.Priority))
}

// IsExpired checks if the event has passed its deadline
func (e *Event) IsExpired() bool {
	return time.Now().After(e.Deadline)
}

// Clone creates a copy of the event
func (e *Event) Clone() *Event {
	return &Event{
		ID:          e.ID,
		Type:        e.Type,
		Priority:    e.Priority,
		Timestamp:   e.Timestamp,
		Description: e.Description,
		ReceivedAt:  e.ReceivedAt,
		Deadline:    e.Deadline,
		Status:      e.Status,
	}
}
