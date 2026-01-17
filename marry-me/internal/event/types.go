package event

// Priority represents the urgency level of an event
type Priority int

const (
	PriorityLow Priority = iota
	PriorityMedium
	PriorityHigh
)

// String returns the string representation of Priority
func (p Priority) String() string {
	switch p {
	case PriorityHigh:
		return "high"
	case PriorityMedium:
		return "medium"
	case PriorityLow:
		return "low"
	default:
		return "unknown"
	}
}

// ParsePriority converts a string to Priority
func ParsePriority(s string) Priority {
	switch s {
	case "high":
		return PriorityHigh
	case "medium":
		return PriorityMedium
	case "low":
		return PriorityLow
	default:
		return PriorityLow
	}
}

// TeamName represents a team that can handle events
type TeamName string

const (
	TeamCatering    TeamName = "catering"
	TeamDecoration  TeamName = "decoration"
	TeamPhotography TeamName = "photography"
	TeamMusic       TeamName = "music"
	TeamCoordinator TeamName = "coordinator"
	TeamFloral      TeamName = "floral"
	TeamVenue       TeamName = "venue"
	TeamTransport   TeamName = "transport"
)

// AllTeams returns all available team names
func AllTeams() []TeamName {
	return []TeamName{
		TeamCatering,
		TeamDecoration,
		TeamPhotography,
		TeamMusic,
		TeamCoordinator,
		TeamFloral,
		TeamVenue,
		TeamTransport,
	}
}

// EventType represents the type of wedding event
type EventType string

const (
	// Catering events
	EventMealService    EventType = "meal_service"
	EventCakeDelivery   EventType = "cake_delivery"
	EventBarSetup       EventType = "bar_setup"
	EventDessertService EventType = "dessert_service"

	// Decoration events
	EventTableSetup      EventType = "table_setup"
	EventFlowerArrangement EventType = "flower_arrangement"
	EventLightingSetup   EventType = "lighting_setup"
	EventCenterpiecePlacement EventType = "centerpiece_placement"

	// Photography events
	EventPhotoSession   EventType = "photo_session"
	EventGroupPhoto     EventType = "group_photo"
	EventCandidCapture  EventType = "candid_capture"
	EventVideoRecording EventType = "video_recording"

	// Music events
	EventBandSetup      EventType = "band_setup"
	EventDJSetup        EventType = "dj_setup"
	EventFirstDance     EventType = "first_dance"
	EventLivePerformance EventType = "live_performance"

	// Coordinator events
	EventGuestArrival   EventType = "guest_arrival"
	EventCeremonyStart  EventType = "ceremony_start"
	EventReceptionStart EventType = "reception_start"
	EventScheduleUpdate EventType = "schedule_update"

	// Floral events
	EventBouquetDelivery EventType = "bouquet_delivery"
	EventFloralSetup     EventType = "floral_setup"
	EventPetalScatter    EventType = "petal_scatter"

	// Venue events
	EventVenuePrep      EventType = "venue_prep"
	EventSeatingArrangement EventType = "seating_arrangement"
	EventCleanup        EventType = "cleanup"

	// Transport events
	EventGuestPickup    EventType = "guest_pickup"
	EventBridalTransport EventType = "bridal_transport"
	EventVendorDelivery EventType = "vendor_delivery"
)

// EventTypeToTeams maps event types to the teams that can handle them
var EventTypeToTeams = map[EventType][]TeamName{
	// Catering events
	EventMealService:    {TeamCatering},
	EventCakeDelivery:   {TeamCatering},
	EventBarSetup:       {TeamCatering, TeamVenue},
	EventDessertService: {TeamCatering},

	// Decoration events
	EventTableSetup:          {TeamDecoration, TeamVenue},
	EventFlowerArrangement:   {TeamDecoration, TeamFloral},
	EventLightingSetup:       {TeamDecoration},
	EventCenterpiecePlacement: {TeamDecoration, TeamFloral},

	// Photography events
	EventPhotoSession:   {TeamPhotography},
	EventGroupPhoto:     {TeamPhotography},
	EventCandidCapture:  {TeamPhotography},
	EventVideoRecording: {TeamPhotography},

	// Music events
	EventBandSetup:       {TeamMusic},
	EventDJSetup:         {TeamMusic},
	EventFirstDance:      {TeamMusic, TeamPhotography},
	EventLivePerformance: {TeamMusic},

	// Coordinator events
	EventGuestArrival:   {TeamCoordinator},
	EventCeremonyStart:  {TeamCoordinator},
	EventReceptionStart: {TeamCoordinator},
	EventScheduleUpdate: {TeamCoordinator},

	// Floral events
	EventBouquetDelivery: {TeamFloral},
	EventFloralSetup:     {TeamFloral, TeamDecoration},
	EventPetalScatter:    {TeamFloral},

	// Venue events
	EventVenuePrep:          {TeamVenue},
	EventSeatingArrangement: {TeamVenue, TeamCoordinator},
	EventCleanup:            {TeamVenue},

	// Transport events
	EventGuestPickup:    {TeamTransport},
	EventBridalTransport: {TeamTransport, TeamCoordinator},
	EventVendorDelivery: {TeamTransport},
}

// GetTeamsForEvent returns the teams that can handle the given event type
func GetTeamsForEvent(eventType EventType) []TeamName {
	teams, ok := EventTypeToTeams[eventType]
	if !ok {
		return nil
	}
	return teams
}

// IsValidEventType checks if the event type is known
func IsValidEventType(eventType EventType) bool {
	_, ok := EventTypeToTeams[eventType]
	return ok
}
