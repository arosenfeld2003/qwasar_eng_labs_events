package event

import (
	"encoding/json"
	"testing"
	"time"
)

func TestPriorityParsing(t *testing.T) {
	tests := []struct {
		input    string
		expected Priority
	}{
		{"high", PriorityHigh},
		{"medium", PriorityMedium},
		{"low", PriorityLow},
		{"unknown", PriorityLow},
		{"", PriorityLow},
	}

	for _, tt := range tests {
		t.Run(tt.input, func(t *testing.T) {
			result := ParsePriority(tt.input)
			if result != tt.expected {
				t.Errorf("ParsePriority(%q) = %v, want %v", tt.input, result, tt.expected)
			}
		})
	}
}

func TestPriorityString(t *testing.T) {
	tests := []struct {
		priority Priority
		expected string
	}{
		{PriorityHigh, "high"},
		{PriorityMedium, "medium"},
		{PriorityLow, "low"},
	}

	for _, tt := range tests {
		t.Run(tt.expected, func(t *testing.T) {
			result := tt.priority.String()
			if result != tt.expected {
				t.Errorf("Priority.String() = %q, want %q", result, tt.expected)
			}
		})
	}
}

func TestEventUnmarshalJSON(t *testing.T) {
	jsonData := `{
		"id": "evt-001",
		"type": "meal_service",
		"priority": "high",
		"timestamp": 1234567890,
		"description": "Main course service"
	}`

	var e Event
	err := json.Unmarshal([]byte(jsonData), &e)
	if err != nil {
		t.Fatalf("failed to unmarshal: %v", err)
	}

	if e.ID != "evt-001" {
		t.Errorf("ID = %q, want %q", e.ID, "evt-001")
	}
	if e.Type != EventMealService {
		t.Errorf("Type = %q, want %q", e.Type, EventMealService)
	}
	if e.Priority != PriorityHigh {
		t.Errorf("Priority = %v, want %v", e.Priority, PriorityHigh)
	}
	if e.Timestamp != 1234567890 {
		t.Errorf("Timestamp = %d, want %d", e.Timestamp, 1234567890)
	}
	if e.Description != "Main course service" {
		t.Errorf("Description = %q, want %q", e.Description, "Main course service")
	}
	if e.Status != StatusPending {
		t.Errorf("Status = %v, want %v", e.Status, StatusPending)
	}
}

func TestEventMarshalJSON(t *testing.T) {
	e := Event{
		ID:          "evt-002",
		Type:        EventPhotoSession,
		Priority:    PriorityMedium,
		Timestamp:   1234567890,
		Description: "Bridal photos",
	}

	data, err := json.Marshal(e)
	if err != nil {
		t.Fatalf("failed to marshal: %v", err)
	}

	var result map[string]interface{}
	if err := json.Unmarshal(data, &result); err != nil {
		t.Fatalf("failed to unmarshal result: %v", err)
	}

	if result["id"] != "evt-002" {
		t.Errorf("id = %v, want %v", result["id"], "evt-002")
	}
	if result["priority"] != "medium" {
		t.Errorf("priority = %v, want %v", result["priority"], "medium")
	}
}

func TestParseDataset(t *testing.T) {
	datasetJSON := `{
		"events": [
			{"id": "evt-001", "type": "meal_service", "priority": "high", "timestamp": 1},
			{"id": "evt-002", "type": "photo_session", "priority": "low", "timestamp": 2}
		]
	}`

	dataset, err := ParseDataset([]byte(datasetJSON))
	if err != nil {
		t.Fatalf("failed to parse dataset: %v", err)
	}

	if len(dataset.Events) != 2 {
		t.Errorf("len(Events) = %d, want %d", len(dataset.Events), 2)
	}

	if dataset.Events[0].ID != "evt-001" {
		t.Errorf("Events[0].ID = %q, want %q", dataset.Events[0].ID, "evt-001")
	}
	if dataset.Events[1].Type != EventPhotoSession {
		t.Errorf("Events[1].Type = %q, want %q", dataset.Events[1].Type, EventPhotoSession)
	}
}

func TestGetTeamsForEvent(t *testing.T) {
	// Single team event
	teams := GetTeamsForEvent(EventMealService)
	if len(teams) != 1 || teams[0] != TeamCatering {
		t.Errorf("GetTeamsForEvent(meal_service) = %v, want [catering]", teams)
	}

	// Multi-team event
	teams = GetTeamsForEvent(EventFirstDance)
	if len(teams) != 2 {
		t.Errorf("GetTeamsForEvent(first_dance) = %v, want 2 teams", teams)
	}

	// Unknown event
	teams = GetTeamsForEvent(EventType("unknown"))
	if teams != nil {
		t.Errorf("GetTeamsForEvent(unknown) = %v, want nil", teams)
	}
}

func TestIsValidEventType(t *testing.T) {
	if !IsValidEventType(EventMealService) {
		t.Error("meal_service should be valid")
	}
	if IsValidEventType(EventType("invalid")) {
		t.Error("invalid should not be valid")
	}
}

func TestDeadlineDuration(t *testing.T) {
	tests := []struct {
		priority Priority
		expected time.Duration
	}{
		{PriorityHigh, 5 * time.Second},
		{PriorityMedium, 10 * time.Second},
		{PriorityLow, 20 * time.Second},
	}

	for _, tt := range tests {
		t.Run(tt.priority.String(), func(t *testing.T) {
			result := DeadlineDuration(tt.priority)
			if result != tt.expected {
				t.Errorf("DeadlineDuration(%v) = %v, want %v", tt.priority, result, tt.expected)
			}
		})
	}
}

func TestEventSetDeadline(t *testing.T) {
	e := &Event{
		Priority:   PriorityHigh,
		ReceivedAt: time.Now(),
	}
	e.SetDeadline()

	expected := e.ReceivedAt.Add(5 * time.Second)
	if !e.Deadline.Equal(expected) {
		t.Errorf("Deadline = %v, want %v", e.Deadline, expected)
	}
}

func TestEventIsExpired(t *testing.T) {
	// Not expired
	e := &Event{
		Deadline: time.Now().Add(10 * time.Second),
	}
	if e.IsExpired() {
		t.Error("event should not be expired")
	}

	// Expired
	e.Deadline = time.Now().Add(-1 * time.Second)
	if !e.IsExpired() {
		t.Error("event should be expired")
	}
}

func TestEventClone(t *testing.T) {
	original := &Event{
		ID:          "evt-001",
		Type:        EventMealService,
		Priority:    PriorityHigh,
		Timestamp:   12345,
		Description: "Test",
		ReceivedAt:  time.Now(),
		Status:      StatusProcessing,
	}
	original.SetDeadline()

	clone := original.Clone()

	if clone.ID != original.ID {
		t.Errorf("Clone ID = %q, want %q", clone.ID, original.ID)
	}
	if clone.Type != original.Type {
		t.Errorf("Clone Type = %q, want %q", clone.Type, original.Type)
	}

	// Modify clone should not affect original
	clone.ID = "modified"
	if original.ID == "modified" {
		t.Error("modifying clone affected original")
	}
}
