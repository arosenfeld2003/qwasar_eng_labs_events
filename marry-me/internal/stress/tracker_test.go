package stress

import (
	"testing"

	"marry-me/internal/broker"
	"marry-me/internal/config"
	"marry-me/internal/event"
)

func TestTracker_RecordCompleted(t *testing.T) {
	mockBroker := broker.NewMockBroker()
	cfg := &config.Config{
		ResultsQueue: "events.results",
	}

	tracker := New(mockBroker, cfg)

	e := &event.Event{
		ID:       "test-001",
		Type:     event.EventMealService,
		Priority: event.PriorityHigh,
	}

	tracker.RecordCompleted(e)

	stats := tracker.GetStats()
	if stats.EventsCompleted != 1 {
		t.Errorf("EventsCompleted = %d, want 1", stats.EventsCompleted)
	}
	if stats.TotalEvents != 1 {
		t.Errorf("TotalEvents = %d, want 1", stats.TotalEvents)
	}
	if stats.HighPriorityCompleted != 1 {
		t.Errorf("HighPriorityCompleted = %d, want 1", stats.HighPriorityCompleted)
	}
}

func TestTracker_RecordExpired(t *testing.T) {
	mockBroker := broker.NewMockBroker()
	cfg := &config.Config{
		ResultsQueue: "events.results",
	}

	tracker := New(mockBroker, cfg)

	e := &event.Event{
		ID:       "test-001",
		Type:     event.EventMealService,
		Priority: event.PriorityMedium,
	}

	tracker.RecordExpired(e)

	stats := tracker.GetStats()
	if stats.EventsExpired != 1 {
		t.Errorf("EventsExpired = %d, want 1", stats.EventsExpired)
	}
	if stats.TotalEvents != 1 {
		t.Errorf("TotalEvents = %d, want 1", stats.TotalEvents)
	}
	if stats.MediumPriorityExpired != 1 {
		t.Errorf("MediumPriorityExpired = %d, want 1", stats.MediumPriorityExpired)
	}
}

func TestTracker_CalculateStress(t *testing.T) {
	mockBroker := broker.NewMockBroker()
	cfg := &config.Config{
		ResultsQueue: "events.results",
	}

	tracker := New(mockBroker, cfg)

	// No events = no stress
	stress := tracker.CalculateStress()
	if stress != 0.0 {
		t.Errorf("Stress with no events = %f, want 0.0", stress)
	}

	// 2 completed, 2 expired = 50% stress
	tracker.RecordCompleted(&event.Event{ID: "1", Type: event.EventMealService})
	tracker.RecordCompleted(&event.Event{ID: "2", Type: event.EventMealService})
	tracker.RecordExpired(&event.Event{ID: "3", Type: event.EventMealService})
	tracker.RecordExpired(&event.Event{ID: "4", Type: event.EventMealService})

	stress = tracker.CalculateStress()
	if stress != 0.5 {
		t.Errorf("Stress = %f, want 0.5", stress)
	}
}

func TestTracker_TeamStats(t *testing.T) {
	mockBroker := broker.NewMockBroker()
	cfg := &config.Config{
		ResultsQueue: "events.results",
	}

	tracker := New(mockBroker, cfg)

	// Catering events
	tracker.RecordCompleted(&event.Event{ID: "1", Type: event.EventMealService})
	tracker.RecordCompleted(&event.Event{ID: "2", Type: event.EventCakeDelivery})
	tracker.RecordExpired(&event.Event{ID: "3", Type: event.EventBarSetup})

	// Photography events
	tracker.RecordCompleted(&event.Event{ID: "4", Type: event.EventPhotoSession})

	stats := tracker.GetStats()

	cateringStats := stats.TeamStats["catering"]
	if cateringStats.Completed != 2 {
		t.Errorf("Catering completed = %d, want 2", cateringStats.Completed)
	}
	if cateringStats.Expired != 1 {
		t.Errorf("Catering expired = %d, want 1", cateringStats.Expired)
	}

	photoStats := stats.TeamStats["photography"]
	if photoStats.Completed != 1 {
		t.Errorf("Photography completed = %d, want 1", photoStats.Completed)
	}
}

func TestTracker_GenerateReport(t *testing.T) {
	mockBroker := broker.NewMockBroker()
	cfg := &config.Config{
		ResultsQueue: "events.results",
	}

	tracker := New(mockBroker, cfg)

	// Add some events
	tracker.RecordCompleted(&event.Event{ID: "1", Type: event.EventMealService, Priority: event.PriorityHigh})
	tracker.RecordCompleted(&event.Event{ID: "2", Type: event.EventPhotoSession, Priority: event.PriorityMedium})
	tracker.RecordExpired(&event.Event{ID: "3", Type: event.EventBandSetup, Priority: event.PriorityLow})

	report := tracker.GenerateReport()

	if report.TotalEvents != 3 {
		t.Errorf("TotalEvents = %d, want 3", report.TotalEvents)
	}
	if report.EventsCompleted != 2 {
		t.Errorf("EventsCompleted = %d, want 2", report.EventsCompleted)
	}
	if report.EventsExpired != 1 {
		t.Errorf("EventsExpired = %d, want 1", report.EventsExpired)
	}

	// Stress should be 1/3
	expectedStress := 1.0 / 3.0
	if report.StressLevel < expectedStress-0.01 || report.StressLevel > expectedStress+0.01 {
		t.Errorf("StressLevel = %f, want ~%f", report.StressLevel, expectedStress)
	}

	// Check priority breakdown
	if report.ByPriority.High.Completed != 1 {
		t.Errorf("High priority completed = %d, want 1", report.ByPriority.High.Completed)
	}
	if report.ByPriority.Medium.Completed != 1 {
		t.Errorf("Medium priority completed = %d, want 1", report.ByPriority.Medium.Completed)
	}
	if report.ByPriority.Low.Expired != 1 {
		t.Errorf("Low priority expired = %d, want 1", report.ByPriority.Low.Expired)
	}
}

func TestTracker_PriorityStatsCalculation(t *testing.T) {
	mockBroker := broker.NewMockBroker()
	cfg := &config.Config{
		ResultsQueue: "events.results",
	}

	tracker := New(mockBroker, cfg)

	// All low priority
	for i := 0; i < 10; i++ {
		if i < 7 {
			tracker.RecordCompleted(&event.Event{ID: "c", Type: event.EventMealService, Priority: event.PriorityLow})
		} else {
			tracker.RecordExpired(&event.Event{ID: "e", Type: event.EventMealService, Priority: event.PriorityLow})
		}
	}

	report := tracker.GenerateReport()

	// 30% stress for low priority
	if report.ByPriority.Low.Total != 10 {
		t.Errorf("Low priority total = %d, want 10", report.ByPriority.Low.Total)
	}
	if report.ByPriority.Low.StressLevel != 0.3 {
		t.Errorf("Low priority stress = %f, want 0.3", report.ByPriority.Low.StressLevel)
	}
}
