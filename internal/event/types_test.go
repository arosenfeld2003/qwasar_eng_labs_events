package event

import (
	"os"
	"path/filepath"
	"testing"
	"time"
)

func TestPriorityDeadlineDuration(t *testing.T) {
	tests := []struct {
		priority Priority
		want     time.Duration
	}{
		{PriorityHigh, 5 * time.Second},
		{PriorityMedium, 10 * time.Second},
		{PriorityLow, 20 * time.Second},
		{Priority("invalid"), 10 * time.Second}, // defaults to medium
	}

	for _, tt := range tests {
		t.Run(string(tt.priority), func(t *testing.T) {
			got := tt.priority.DeadlineDuration()
			if got != tt.want {
				t.Errorf("DeadlineDuration() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestPriorityIsValid(t *testing.T) {
	tests := []struct {
		priority Priority
		want     bool
	}{
		{PriorityHigh, true},
		{PriorityMedium, true},
		{PriorityLow, true},
		{Priority("invalid"), false},
		{Priority(""), false},
	}

	for _, tt := range tests {
		t.Run(string(tt.priority), func(t *testing.T) {
			if got := tt.priority.IsValid(); got != tt.want {
				t.Errorf("IsValid() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestGetTeam(t *testing.T) {
	tests := []struct {
		eventType EventType
		want      Team
	}{
		// Catering
		{TypeMealService, TeamCatering},
		{TypeBadFood, TeamCatering},

		// Decoration
		{TypeTableSetup, TeamDecoration},
		{TypeDirtyTable, TeamDecoration},

		// Photography
		{TypePhotoSession, TeamPhotography},

		// Music
		{TypeBandSetup, TeamMusic},
		{TypeMusic, TeamMusic},

		// Coordinator
		{TypeGuestArrival, TeamCoordinator},
		{TypeBride, TeamCoordinator},
		{TypeGroom, TeamCoordinator},

		// Floral
		{TypeBouquetDelivery, TeamFloral},

		// Venue
		{TypeVenuePrep, TeamVenue},
		{TypeDirtyFloor, TeamVenue},
		{TypeBrokenItems, TeamVenue},

		// Transport
		{TypeGuestPickup, TeamTransport},

		// Security
		{TypeBrawl, TeamSecurity},
		{TypeAccident, TeamSecurity},

		// Medical
		{TypeFeelingIll, TeamMedical},

		// Unknown
		{EventType("unknown_type"), TeamUnknown},
	}

	for _, tt := range tests {
		t.Run(string(tt.eventType), func(t *testing.T) {
			if got := GetTeam(tt.eventType); got != tt.want {
				t.Errorf("GetTeam(%s) = %v, want %v", tt.eventType, got, tt.want)
			}
		})
	}
}

func TestIsValidEventType(t *testing.T) {
	if !IsValidEventType(TypeMealService) {
		t.Error("expected meal_service to be valid")
	}
	if !IsValidEventType(TypeAccident) {
		t.Error("expected accident to be valid")
	}
	if IsValidEventType(EventType("not_a_real_type")) {
		t.Error("expected unknown type to be invalid")
	}
}

func TestAllEventTypes(t *testing.T) {
	types := AllEventTypes()
	if len(types) == 0 {
		t.Error("expected at least some event types")
	}

	// Verify some expected types are present
	found := make(map[EventType]bool)
	for _, et := range types {
		found[et] = true
	}

	expectedTypes := []EventType{
		TypeMealService,
		TypeAccident,
		TypeBrawl,
		TypePhotoSession,
	}

	for _, et := range expectedTypes {
		if !found[et] {
			t.Errorf("expected %s to be in AllEventTypes()", et)
		}
	}
}

func TestEventSetReceived(t *testing.T) {
	e := Event{
		ID:       1,
		Type:     TypeAccident,
		Priority: PriorityHigh,
	}

	now := time.Now()
	e.SetReceived(now)

	if e.ReceivedAt != now {
		t.Errorf("ReceivedAt = %v, want %v", e.ReceivedAt, now)
	}
	if e.Status != StatusPending {
		t.Errorf("Status = %v, want %v", e.Status, StatusPending)
	}

	expectedDeadline := now.Add(5 * time.Second)
	if e.Deadline != expectedDeadline {
		t.Errorf("Deadline = %v, want %v", e.Deadline, expectedDeadline)
	}
}

func TestEventIsExpired(t *testing.T) {
	now := time.Now()
	e := Event{
		ID:       1,
		Type:     TypeAccident,
		Priority: PriorityHigh,
	}
	e.SetReceived(now)

	// Not expired yet
	if e.IsExpired(now) {
		t.Error("event should not be expired at receive time")
	}

	// Still not expired 4 seconds later
	if e.IsExpired(now.Add(4 * time.Second)) {
		t.Error("event should not be expired after 4 seconds (high priority has 5s deadline)")
	}

	// Expired after 6 seconds
	if !e.IsExpired(now.Add(6 * time.Second)) {
		t.Error("event should be expired after 6 seconds")
	}
}

func TestEventTeam(t *testing.T) {
	e := Event{Type: TypeBrawl}
	if e.Team() != TeamSecurity {
		t.Errorf("Team() = %v, want %v", e.Team(), TeamSecurity)
	}

	e.Type = TypeMealService
	if e.Team() != TeamCatering {
		t.Errorf("Team() = %v, want %v", e.Team(), TeamCatering)
	}
}

func TestEventValidate(t *testing.T) {
	tests := []struct {
		name    string
		event   Event
		wantErr bool
	}{
		{
			name: "valid event",
			event: Event{
				ID:       1,
				Type:     TypeAccident,
				Priority: PriorityHigh,
			},
			wantErr: false,
		},
		{
			name: "invalid ID",
			event: Event{
				ID:       0,
				Type:     TypeAccident,
				Priority: PriorityHigh,
			},
			wantErr: true,
		},
		{
			name: "invalid type",
			event: Event{
				ID:       1,
				Type:     EventType("invalid"),
				Priority: PriorityHigh,
			},
			wantErr: true,
		},
		{
			name: "invalid priority",
			event: Event{
				ID:       1,
				Type:     TypeAccident,
				Priority: Priority("invalid"),
			},
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := tt.event.Validate()
			if (err != nil) != tt.wantErr {
				t.Errorf("Validate() error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}

func TestLoadFromBytes(t *testing.T) {
	jsonData := `[
		{"id": 1, "event_type": "accident", "priority": "High", "description": "Test", "timestamp": "01:00"},
		{"id": 2, "event_type": "brawl", "priority": "Low", "description": "Test 2", "timestamp": "02:00"}
	]`

	events, err := LoadFromBytes([]byte(jsonData))
	if err != nil {
		t.Fatalf("LoadFromBytes() error = %v", err)
	}

	if len(events) != 2 {
		t.Fatalf("expected 2 events, got %d", len(events))
	}

	if events[0].ID != 1 || events[0].Type != TypeAccident || events[0].Priority != PriorityHigh {
		t.Errorf("first event mismatch: %+v", events[0])
	}

	if events[1].ID != 2 || events[1].Type != TypeBrawl || events[1].Priority != PriorityLow {
		t.Errorf("second event mismatch: %+v", events[1])
	}
}

func TestLoadFromBytesInvalidJSON(t *testing.T) {
	_, err := LoadFromBytes([]byte("not json"))
	if err == nil {
		t.Error("expected error for invalid JSON")
	}
}

func TestLoadFromFile(t *testing.T) {
	// Create temp file with test data
	tmpDir := t.TempDir()
	tmpFile := filepath.Join(tmpDir, "test_events.json")

	jsonData := `[{"id": 1, "event_type": "accident", "priority": "High", "description": "Test", "timestamp": "01:00"}]`
	if err := os.WriteFile(tmpFile, []byte(jsonData), 0644); err != nil {
		t.Fatalf("failed to write temp file: %v", err)
	}

	events, err := LoadFromFile(tmpFile)
	if err != nil {
		t.Fatalf("LoadFromFile() error = %v", err)
	}

	if len(events) != 1 {
		t.Fatalf("expected 1 event, got %d", len(events))
	}

	if events[0].ID != 1 {
		t.Errorf("expected ID 1, got %d", events[0].ID)
	}
}

func TestLoadFromFileNotFound(t *testing.T) {
	_, err := LoadFromFile("/nonexistent/path/file.json")
	if err == nil {
		t.Error("expected error for nonexistent file")
	}
}
