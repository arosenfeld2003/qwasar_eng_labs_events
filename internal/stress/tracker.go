package stress

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"sync"
	"time"

	"github.com/arosenfeld2003/qwasar_eng_labs_events/internal/broker"
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

// Tracker consumes processed events and calculates stress metrics.
type Tracker struct {
	mu        sync.Mutex
	events    []ResultEvent
	completed int
	expired   int
}

// New creates a new Tracker.
func New() *Tracker {
	return &Tracker{}
}

// Record adds a processed event result to the tracker.
func (t *Tracker) Record(re ResultEvent) {
	t.mu.Lock()
	defer t.mu.Unlock()

	t.events = append(t.events, re)
	switch re.Event.Status {
	case event.StatusCompleted:
		t.completed++
	case event.StatusExpired:
		t.expired++
	}
}

// StressLevel returns the current stress ratio: expired / total.
// Returns 0 if no events have been recorded.
func (t *Tracker) StressLevel() float64 {
	t.mu.Lock()
	defer t.mu.Unlock()

	total := t.completed + t.expired
	if total == 0 {
		return 0
	}
	return float64(t.expired) / float64(total)
}

// Report generates a full stress report with breakdowns.
func (t *Tracker) Report() *Report {
	t.mu.Lock()
	defer t.mu.Unlock()

	r := &Report{
		ByPriority: make(map[string]PriorityStats),
		ByTeam:     make(map[string]TeamStats),
	}

	for _, re := range t.events {
		e := re.Event
		if e.Status != event.StatusCompleted && e.Status != event.StatusExpired {
			continue
		}

		r.TotalEvents++
		isCompleted := e.Status == event.StatusCompleted

		if isCompleted {
			r.Completed++
		} else {
			r.Expired++
		}

		// Priority breakdown
		pKey := string(e.Priority)
		ps := r.ByPriority[pKey]
		ps.Total++
		if isCompleted {
			ps.Completed++
		} else {
			ps.Expired++
		}
		r.ByPriority[pKey] = ps

		// Team breakdown
		tKey := string(e.Team())
		ts := r.ByTeam[tKey]
		ts.Total++
		if isCompleted {
			ts.Completed++
		} else {
			ts.Expired++
		}
		r.ByTeam[tKey] = ts

		// Duration
		var dur time.Duration
		if isCompleted && !re.CompletedAt.IsZero() && !e.ReceivedAt.IsZero() {
			dur = re.CompletedAt.Sub(e.ReceivedAt)
		} else if !e.ReceivedAt.IsZero() && !e.Deadline.IsZero() {
			dur = e.Deadline.Sub(e.ReceivedAt)
		}

		r.Durations = append(r.Durations, EventDuration{
			EventID:   e.ID,
			EventType: string(e.Type),
			Priority:  pKey,
			Team:      tKey,
			Status:    string(e.Status),
			Duration:  dur,
			DurationS: dur.String(),
		})
	}

	// Calculate stress ratios
	if r.TotalEvents > 0 {
		r.OverallStress = float64(r.Expired) / float64(r.TotalEvents)
	}
	for k, ps := range r.ByPriority {
		if ps.Total > 0 {
			ps.Stress = float64(ps.Expired) / float64(ps.Total)
		}
		r.ByPriority[k] = ps
	}
	for k, ts := range r.ByTeam {
		if ts.Total > 0 {
			ts.Stress = float64(ts.Expired) / float64(ts.Total)
		}
		r.ByTeam[k] = ts
	}

	return r
}

// parseResultEvent accepts both a ResultEvent wrapper ({"event": {...}})
// and a raw event.Event ({"id": ...}). The organizer publishes raw events
// to the results queue, while team workers may publish ResultEvent wrappers.
func parseResultEvent(data []byte) (ResultEvent, error) {
	// Try the ResultEvent wrapper first.
	var re ResultEvent
	if err := json.Unmarshal(data, &re); err != nil {
		return ResultEvent{}, err
	}
	// If the wrapper produced a valid event, use it.
	if re.Event.ID != 0 {
		return re, nil
	}
	// Fall back to raw event.Event (e.g. from organizer.publishExpired).
	var ev event.Event
	if err := json.Unmarshal(data, &ev); err != nil {
		return ResultEvent{}, err
	}
	if ev.ID == 0 {
		return ResultEvent{}, fmt.Errorf("event has no ID")
	}
	return ResultEvent{Event: ev}, nil
}

// Consume subscribes to the results queue and records events until the
// context is cancelled. It blocks until the context is done.
func (t *Tracker) Consume(ctx context.Context, b broker.Broker, queue string) error {
	sub, err := b.Subscribe(ctx, broker.ConsumeOptions{
		Queue:   queue,
		AutoAck: true,
	})
	if err != nil {
		return fmt.Errorf("subscribe to %s: %w", queue, err)
	}
	defer sub.Cancel()

	for {
		select {
		case <-ctx.Done():
			return ctx.Err()
		case d, ok := <-sub.Deliveries:
			if !ok {
				return nil
			}
			re, err := parseResultEvent(d.Body)
			if err != nil {
				log.Printf("stress: unmarshal error: %v", err)
				continue
			}
			t.Record(re)
		}
	}
}
