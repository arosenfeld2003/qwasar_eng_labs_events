package stress

import (
	"encoding/json"
	"time"

	"github.com/arosenfeld2003/qwasar_eng_labs_events/internal/event"
)

// ResultEvent is a processed event consumed from the results queue.
type ResultEvent struct {
	Event       event.Event `json:"event"`
	CompletedAt time.Time   `json:"completed_at,omitempty"`
}

// PriorityStats holds stress metrics for a single priority level.
type PriorityStats struct {
	Total     int     `json:"total"`
	Completed int     `json:"completed"`
	Expired   int     `json:"expired"`
	Stress    float64 `json:"stress"`
}

// TeamStats holds stress metrics for a single team.
type TeamStats struct {
	Total     int     `json:"total"`
	Completed int     `json:"completed"`
	Expired   int     `json:"expired"`
	Stress    float64 `json:"stress"`
}

// EventDuration records how long an event took to process.
type EventDuration struct {
	EventID   int           `json:"event_id"`
	EventType string        `json:"event_type"`
	Priority  string        `json:"priority"`
	Team      string        `json:"team"`
	Status    string        `json:"status"`
	Duration  time.Duration `json:"duration_ns"`
	DurationS string        `json:"duration"`
}

// Report is the full stress report with all breakdowns.
type Report struct {
	OverallStress float64                  `json:"overall_stress"`
	TotalEvents   int                      `json:"total_events"`
	Completed     int                      `json:"completed"`
	Expired       int                      `json:"expired"`
	ByPriority    map[string]PriorityStats `json:"by_priority"`
	ByTeam        map[string]TeamStats     `json:"by_team"`
	Durations     []EventDuration          `json:"durations"`
}

// JSON returns the report as a JSON byte slice.
func (r *Report) JSON() ([]byte, error) {
	return json.MarshalIndent(r, "", "  ")
}
