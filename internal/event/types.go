package event

import (
	"encoding/json"
	"fmt"
	"os"
	"time"
)

// Priority represents event urgency level.
type Priority string

const (
	PriorityHigh   Priority = "High"
	PriorityMedium Priority = "Medium"
	PriorityLow    Priority = "Low"
)

// DeadlineDuration returns the deadline duration for a priority level.
func (p Priority) DeadlineDuration() time.Duration {
	switch p {
	case PriorityHigh:
		return 5 * time.Second
	case PriorityMedium:
		return 10 * time.Second
	case PriorityLow:
		return 20 * time.Second
	default:
		return 10 * time.Second
	}
}

// IsValid checks if the priority is a known value.
func (p Priority) IsValid() bool {
	switch p {
	case PriorityHigh, PriorityMedium, PriorityLow:
		return true
	default:
		return false
	}
}

// Status represents the processing state of an event.
type Status string

const (
	StatusPending    Status = "Pending"
	StatusProcessing Status = "Processing"
	StatusCompleted  Status = "Completed"
	StatusExpired    Status = "Expired"
)

// EventType represents the category of an event.
type EventType string

// Catering team event types
const (
	TypeMealService    EventType = "meal_service"
	TypeCakeDelivery   EventType = "cake_delivery"
	TypeBarSetup       EventType = "bar_setup"
	TypeDessertService EventType = "dessert_service"
)

// Decoration team event types
const (
	TypeTableSetup           EventType = "table_setup"
	TypeFlowerArrangement    EventType = "flower_arrangement"
	TypeLightingSetup        EventType = "lighting_setup"
	TypeCenterpiecePlacement EventType = "centerpiece_placement"
)

// Photography team event types
const (
	TypePhotoSession   EventType = "photo_session"
	TypeGroupPhoto     EventType = "group_photo"
	TypeCandidCapture  EventType = "candid_capture"
	TypeVideoRecording EventType = "video_recording"
)

// Music team event types
const (
	TypeBandSetup       EventType = "band_setup"
	TypeDJSetup         EventType = "dj_setup"
	TypeFirstDance      EventType = "first_dance"
	TypeLivePerformance EventType = "live_performance"
)

// Coordinator team event types
const (
	TypeGuestArrival   EventType = "guest_arrival"
	TypeCeremonyStart  EventType = "ceremony_start"
	TypeReceptionStart EventType = "reception_start"
	TypeScheduleUpdate EventType = "schedule_update"
)

// Floral team event types
const (
	TypeBouquetDelivery EventType = "bouquet_delivery"
	TypeFloralSetup     EventType = "floral_setup"
	TypePetalScatter    EventType = "petal_scatter"
)

// Venue team event types
const (
	TypeVenuePrep          EventType = "venue_prep"
	TypeSeatingArrangement EventType = "seating_arrangement"
	TypeCleanup            EventType = "cleanup"
)

// Transport team event types
const (
	TypeGuestPickup     EventType = "guest_pickup"
	TypeBridalTransport EventType = "bridal_transport"
	TypeVendorDelivery  EventType = "vendor_delivery"
)

// Legacy/Dataset event types (for backward compatibility)
const (
	TypeAccident    EventType = "accident"
	TypeBadFood     EventType = "bad_food"
	TypeBrawl       EventType = "brawl"
	TypeBride       EventType = "bride"
	TypeBrokenItems EventType = "broken_itens" // Note: typo preserved from dataset
	TypeDirtyFloor  EventType = "dirty_floor"
	TypeDirtyTable  EventType = "dirty_table"
	TypeFeelingIll  EventType = "feeling_ill"
	TypeGroom       EventType = "groom"
	TypeMusic       EventType = "music"
	TypeNotOnList   EventType = "not_on_list"
)

// Team represents a team that handles events.
type Team string

const (
	TeamCatering    Team = "catering"
	TeamDecoration  Team = "decoration"
	TeamPhotography Team = "photography"
	TeamMusic       Team = "music"
	TeamCoordinator Team = "coordinator"
	TeamFloral      Team = "floral"
	TeamVenue       Team = "venue"
	TeamTransport   Team = "transport"
	TeamSecurity    Team = "security"  // Handles legacy incident types
	TeamMedical     Team = "medical"   // Handles health-related incidents
	TeamUnknown     Team = "unknown"
)

// eventTeamMapping maps event types to their responsible teams.
var eventTeamMapping = map[EventType]Team{
	// Catering
	TypeMealService:    TeamCatering,
	TypeCakeDelivery:   TeamCatering,
	TypeBarSetup:       TeamCatering,
	TypeDessertService: TeamCatering,
	TypeBadFood:        TeamCatering,

	// Decoration
	TypeTableSetup:           TeamDecoration,
	TypeFlowerArrangement:    TeamDecoration,
	TypeLightingSetup:        TeamDecoration,
	TypeCenterpiecePlacement: TeamDecoration,
	TypeDirtyTable:           TeamDecoration,

	// Photography
	TypePhotoSession:   TeamPhotography,
	TypeGroupPhoto:     TeamPhotography,
	TypeCandidCapture:  TeamPhotography,
	TypeVideoRecording: TeamPhotography,

	// Music
	TypeBandSetup:       TeamMusic,
	TypeDJSetup:         TeamMusic,
	TypeFirstDance:      TeamMusic,
	TypeLivePerformance: TeamMusic,
	TypeMusic:           TeamMusic,

	// Coordinator
	TypeGuestArrival:   TeamCoordinator,
	TypeCeremonyStart:  TeamCoordinator,
	TypeReceptionStart: TeamCoordinator,
	TypeScheduleUpdate: TeamCoordinator,
	TypeBride:          TeamCoordinator,
	TypeGroom:          TeamCoordinator,
	TypeNotOnList:      TeamCoordinator,

	// Floral
	TypeBouquetDelivery: TeamFloral,
	TypeFloralSetup:     TeamFloral,
	TypePetalScatter:    TeamFloral,

	// Venue
	TypeVenuePrep:          TeamVenue,
	TypeSeatingArrangement: TeamVenue,
	TypeCleanup:            TeamVenue,
	TypeDirtyFloor:         TeamVenue,
	TypeBrokenItems:        TeamVenue,

	// Transport
	TypeGuestPickup:     TeamTransport,
	TypeBridalTransport: TeamTransport,
	TypeVendorDelivery:  TeamTransport,

	// Security
	TypeBrawl:    TeamSecurity,
	TypeAccident: TeamSecurity,

	// Medical
	TypeFeelingIll: TeamMedical,
}

// GetTeam returns the team responsible for handling an event type.
func GetTeam(eventType EventType) Team {
	if team, ok := eventTeamMapping[eventType]; ok {
		return team
	}
	return TeamUnknown
}

// IsValidEventType checks if the event type is known.
func IsValidEventType(eventType EventType) bool {
	_, ok := eventTeamMapping[eventType]
	return ok
}

// AllEventTypes returns all known event types.
func AllEventTypes() []EventType {
	types := make([]EventType, 0, len(eventTeamMapping))
	for t := range eventTeamMapping {
		types = append(types, t)
	}
	return types
}

// Event represents a wedding event to be processed.
type Event struct {
	ID          int       `json:"id"`
	Type        EventType `json:"event_type"`
	Priority    Priority  `json:"priority"`
	Description string    `json:"description"`
	Timestamp   string    `json:"timestamp"`
	ReceivedAt  time.Time `json:"received_at,omitempty"`
	Deadline    time.Time `json:"deadline,omitempty"`
	Status      Status    `json:"status,omitempty"`
}

// SetReceived marks the event as received and calculates its deadline.
func (e *Event) SetReceived(receivedAt time.Time) {
	e.ReceivedAt = receivedAt
	e.Deadline = receivedAt.Add(e.Priority.DeadlineDuration())
	e.Status = StatusPending
}

// IsExpired checks if the event has passed its deadline.
func (e *Event) IsExpired(now time.Time) bool {
	return now.After(e.Deadline)
}

// Team returns the team responsible for this event.
func (e *Event) Team() Team {
	return GetTeam(e.Type)
}

// Validate checks if the event has valid fields.
func (e *Event) Validate() error {
	if e.ID <= 0 {
		return fmt.Errorf("invalid event ID: %d", e.ID)
	}
	if !IsValidEventType(e.Type) {
		return fmt.Errorf("unknown event type: %s", e.Type)
	}
	if !e.Priority.IsValid() {
		return fmt.Errorf("invalid priority: %s", e.Priority)
	}
	return nil
}

// LoadFromFile loads events from a JSON file.
func LoadFromFile(path string) ([]Event, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("read file: %w", err)
	}

	var events []Event
	if err := json.Unmarshal(data, &events); err != nil {
		return nil, fmt.Errorf("parse json: %w", err)
	}

	return events, nil
}

// LoadFromBytes parses events from JSON bytes.
func LoadFromBytes(data []byte) ([]Event, error) {
	var events []Event
	if err := json.Unmarshal(data, &events); err != nil {
		return nil, fmt.Errorf("parse json: %w", err)
	}
	return events, nil
}
