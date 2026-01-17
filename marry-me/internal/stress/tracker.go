package stress

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"sync"
	"time"

	"github.com/rs/zerolog/log"

	"marry-me/internal/broker"
	"marry-me/internal/config"
	"marry-me/internal/event"
)

// Tracker tracks stress levels based on event completion rates
type Tracker struct {
	broker      broker.Broker
	cfg         *config.Config
	stats       Stats
	mu          sync.RWMutex
	stopCh      chan struct{}
	wg          sync.WaitGroup
	startTime   time.Time
}

// Stats holds tracking statistics
type Stats struct {
	EventsCompleted int64 `json:"events_completed"`
	EventsExpired   int64 `json:"events_expired"`
	TotalEvents     int64 `json:"total_events"`

	// By priority
	HighPriorityCompleted   int64 `json:"high_priority_completed"`
	HighPriorityExpired     int64 `json:"high_priority_expired"`
	MediumPriorityCompleted int64 `json:"medium_priority_completed"`
	MediumPriorityExpired   int64 `json:"medium_priority_expired"`
	LowPriorityCompleted    int64 `json:"low_priority_completed"`
	LowPriorityExpired      int64 `json:"low_priority_expired"`

	// By team (event type's primary team)
	TeamStats map[string]TeamStats `json:"team_stats"`
}

// TeamStats holds per-team statistics
type TeamStats struct {
	Completed int64 `json:"completed"`
	Expired   int64 `json:"expired"`
}

// Report is the final stress report
type Report struct {
	TotalEvents     int64   `json:"total_events"`
	EventsCompleted int64   `json:"events_completed"`
	EventsExpired   int64   `json:"events_expired"`
	StressLevel     float64 `json:"stress_level"`
	Duration        string  `json:"duration"`
	Timestamp       string  `json:"timestamp"`

	ByPriority struct {
		High   PriorityStats `json:"high"`
		Medium PriorityStats `json:"medium"`
		Low    PriorityStats `json:"low"`
	} `json:"by_priority"`

	ByTeam map[string]TeamReport `json:"by_team"`
}

// PriorityStats holds statistics for a priority level
type PriorityStats struct {
	Completed int64   `json:"completed"`
	Expired   int64   `json:"expired"`
	Total     int64   `json:"total"`
	StressLevel float64 `json:"stress_level"`
}

// TeamReport holds statistics for a team
type TeamReport struct {
	Completed   int64   `json:"completed"`
	Expired     int64   `json:"expired"`
	Total       int64   `json:"total"`
	StressLevel float64 `json:"stress_level"`
}

// New creates a new Tracker instance
func New(b broker.Broker, cfg *config.Config) *Tracker {
	return &Tracker{
		broker:    b,
		cfg:       cfg,
		stopCh:    make(chan struct{}),
		startTime: time.Now(),
		stats: Stats{
			TeamStats: make(map[string]TeamStats),
		},
	}
}

// Start begins tracking stress from results queue
func (t *Tracker) Start(ctx context.Context) error {
	log.Info().Msg("Stress tracker starting...")

	t.startTime = time.Now()

	// Subscribe to results queue
	err := t.broker.Subscribe(ctx, t.cfg.ResultsQueue, t.handleResult)
	if err != nil {
		return fmt.Errorf("failed to subscribe to results: %w", err)
	}

	t.wg.Add(1)
	go func() {
		defer t.wg.Done()
		select {
		case <-ctx.Done():
			log.Info().Msg("Stress tracker stopping due to context cancellation")
		case <-t.stopCh:
			log.Info().Msg("Stress tracker stopping due to stop signal")
		}
	}()

	log.Info().
		Str("queue", t.cfg.ResultsQueue).
		Msg("Stress tracker started")

	return nil
}

// Stop gracefully stops the tracker
func (t *Tracker) Stop() {
	close(t.stopCh)
	t.wg.Wait()
	log.Info().Msg("Stress tracker stopped")
}

// handleResult processes a result event
func (t *Tracker) handleResult(ctx context.Context, e *event.Event) error {
	t.mu.Lock()
	defer t.mu.Unlock()

	t.stats.TotalEvents++

	// Get primary team for this event type
	teams := event.GetTeamsForEvent(e.Type)
	primaryTeam := ""
	if len(teams) > 0 {
		primaryTeam = string(teams[0])
	}

	// Update team stats
	if primaryTeam != "" {
		teamStats := t.stats.TeamStats[primaryTeam]
		if e.Status == event.StatusCompleted {
			teamStats.Completed++
		} else {
			teamStats.Expired++
		}
		t.stats.TeamStats[primaryTeam] = teamStats
	}

	// Update overall and priority stats
	switch e.Status {
	case event.StatusCompleted:
		t.stats.EventsCompleted++
		switch e.Priority {
		case event.PriorityHigh:
			t.stats.HighPriorityCompleted++
		case event.PriorityMedium:
			t.stats.MediumPriorityCompleted++
		case event.PriorityLow:
			t.stats.LowPriorityCompleted++
		}

	default: // Expired or other
		t.stats.EventsExpired++
		switch e.Priority {
		case event.PriorityHigh:
			t.stats.HighPriorityExpired++
		case event.PriorityMedium:
			t.stats.MediumPriorityExpired++
		case event.PriorityLow:
			t.stats.LowPriorityExpired++
		}
	}

	log.Debug().
		Str("event_id", e.ID).
		Str("status", e.Status.String()).
		Str("priority", e.Priority.String()).
		Int64("total_completed", t.stats.EventsCompleted).
		Int64("total_expired", t.stats.EventsExpired).
		Msg("Result recorded")

	return nil
}

// RecordCompleted manually records a completed event
func (t *Tracker) RecordCompleted(e *event.Event) {
	e.Status = event.StatusCompleted
	t.handleResult(context.Background(), e)
}

// RecordExpired manually records an expired event
func (t *Tracker) RecordExpired(e *event.Event) {
	e.Status = event.StatusExpired
	t.handleResult(context.Background(), e)
}

// GetStats returns a copy of current statistics
func (t *Tracker) GetStats() Stats {
	t.mu.RLock()
	defer t.mu.RUnlock()

	// Deep copy team stats
	statsCopy := t.stats
	statsCopy.TeamStats = make(map[string]TeamStats)
	for k, v := range t.stats.TeamStats {
		statsCopy.TeamStats[k] = v
	}

	return statsCopy
}

// CalculateStress calculates the stress level (0.0 - 1.0)
func (t *Tracker) CalculateStress() float64 {
	t.mu.RLock()
	defer t.mu.RUnlock()

	if t.stats.TotalEvents == 0 {
		return 0.0
	}

	return float64(t.stats.EventsExpired) / float64(t.stats.TotalEvents)
}

// GenerateReport creates a comprehensive stress report
func (t *Tracker) GenerateReport() Report {
	t.mu.RLock()
	defer t.mu.RUnlock()

	report := Report{
		TotalEvents:     t.stats.TotalEvents,
		EventsCompleted: t.stats.EventsCompleted,
		EventsExpired:   t.stats.EventsExpired,
		StressLevel:     t.calculateStressLocked(),
		Duration:        time.Since(t.startTime).Round(time.Second).String(),
		Timestamp:       time.Now().Format(time.RFC3339),
		ByTeam:          make(map[string]TeamReport),
	}

	// Priority stats
	report.ByPriority.High = t.calcPriorityStats(
		t.stats.HighPriorityCompleted,
		t.stats.HighPriorityExpired,
	)
	report.ByPriority.Medium = t.calcPriorityStats(
		t.stats.MediumPriorityCompleted,
		t.stats.MediumPriorityExpired,
	)
	report.ByPriority.Low = t.calcPriorityStats(
		t.stats.LowPriorityCompleted,
		t.stats.LowPriorityExpired,
	)

	// Team stats
	for team, stats := range t.stats.TeamStats {
		total := stats.Completed + stats.Expired
		stressLevel := 0.0
		if total > 0 {
			stressLevel = float64(stats.Expired) / float64(total)
		}
		report.ByTeam[team] = TeamReport{
			Completed:   stats.Completed,
			Expired:     stats.Expired,
			Total:       total,
			StressLevel: stressLevel,
		}
	}

	return report
}

func (t *Tracker) calcPriorityStats(completed, expired int64) PriorityStats {
	total := completed + expired
	stressLevel := 0.0
	if total > 0 {
		stressLevel = float64(expired) / float64(total)
	}
	return PriorityStats{
		Completed:   completed,
		Expired:     expired,
		Total:       total,
		StressLevel: stressLevel,
	}
}

func (t *Tracker) calculateStressLocked() float64 {
	if t.stats.TotalEvents == 0 {
		return 0.0
	}
	return float64(t.stats.EventsExpired) / float64(t.stats.TotalEvents)
}

// WriteReportToFile writes the report to a JSON file
func (t *Tracker) WriteReportToFile(filename string) error {
	report := t.GenerateReport()

	data, err := json.MarshalIndent(report, "", "  ")
	if err != nil {
		return fmt.Errorf("failed to marshal report: %w", err)
	}

	err = os.WriteFile(filename, data, 0644)
	if err != nil {
		return fmt.Errorf("failed to write report: %w", err)
	}

	log.Info().
		Str("file", filename).
		Float64("stress_level", report.StressLevel).
		Msg("Report written to file")

	return nil
}

// PrintReport prints a summary report to stdout
func (t *Tracker) PrintReport() {
	report := t.GenerateReport()

	fmt.Println("\n========================================")
	fmt.Println("           STRESS REPORT")
	fmt.Println("========================================")
	fmt.Printf("Duration: %s\n", report.Duration)
	fmt.Printf("Timestamp: %s\n\n", report.Timestamp)

	fmt.Println("OVERALL:")
	fmt.Printf("  Total Events:  %d\n", report.TotalEvents)
	fmt.Printf("  Completed:     %d\n", report.EventsCompleted)
	fmt.Printf("  Expired:       %d\n", report.EventsExpired)
	fmt.Printf("  Stress Level:  %.2f%%\n\n", report.StressLevel*100)

	fmt.Println("BY PRIORITY:")
	fmt.Printf("  High:   %d/%d completed (%.2f%% stress)\n",
		report.ByPriority.High.Completed,
		report.ByPriority.High.Total,
		report.ByPriority.High.StressLevel*100)
	fmt.Printf("  Medium: %d/%d completed (%.2f%% stress)\n",
		report.ByPriority.Medium.Completed,
		report.ByPriority.Medium.Total,
		report.ByPriority.Medium.StressLevel*100)
	fmt.Printf("  Low:    %d/%d completed (%.2f%% stress)\n\n",
		report.ByPriority.Low.Completed,
		report.ByPriority.Low.Total,
		report.ByPriority.Low.StressLevel*100)

	if len(report.ByTeam) > 0 {
		fmt.Println("BY TEAM:")
		for team, stats := range report.ByTeam {
			fmt.Printf("  %-12s: %d/%d completed (%.2f%% stress)\n",
				team,
				stats.Completed,
				stats.Total,
				stats.StressLevel*100)
		}
	}

	fmt.Println("========================================")
}
