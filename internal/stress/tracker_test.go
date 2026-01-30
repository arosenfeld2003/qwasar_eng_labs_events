package stress

import (
	"context"
	"encoding/json"
	"testing"
	"time"

	"github.com/arosenfeld2003/qwasar_eng_labs_events/internal/broker"
	"github.com/arosenfeld2003/qwasar_eng_labs_events/internal/event"
)

func makeEvent(id int, et event.EventType, p event.Priority, s event.Status) event.Event {
	e := event.Event{
		ID:       id,
		Type:     et,
		Priority: p,
		Status:   s,
	}
	now := time.Now()
	e.SetReceived(now.Add(-10 * time.Second))
	e.Status = s // restore after SetReceived resets to Pending
	return e
}

func TestNewTracker(t *testing.T) {
	tr := New()
	if tr == nil {
		t.Fatal("expected non-nil tracker")
	}
	if tr.StressLevel() != 0 {
		t.Fatal("expected 0 stress with no events")
	}
}

func TestRecordCompleted(t *testing.T) {
	tr := New()
	e := makeEvent(1, event.TypeMealService, event.PriorityHigh, event.StatusCompleted)
	tr.Record(ResultEvent{Event: e, CompletedAt: time.Now()})

	if tr.StressLevel() != 0 {
		t.Fatalf("expected 0 stress, got %f", tr.StressLevel())
	}

	r := tr.Report()
	if r.TotalEvents != 1 {
		t.Fatalf("expected 1 event, got %d", r.TotalEvents)
	}
	if r.Completed != 1 {
		t.Fatalf("expected 1 completed, got %d", r.Completed)
	}
	if r.Expired != 0 {
		t.Fatalf("expected 0 expired, got %d", r.Expired)
	}
}

func TestRecordExpired(t *testing.T) {
	tr := New()
	e := makeEvent(1, event.TypeBrawl, event.PriorityHigh, event.StatusExpired)
	tr.Record(ResultEvent{Event: e})

	if tr.StressLevel() != 1.0 {
		t.Fatalf("expected 1.0 stress, got %f", tr.StressLevel())
	}
}

func TestStressLevelMixed(t *testing.T) {
	tr := New()

	// 3 completed, 1 expired -> stress = 0.25
	for i := 1; i <= 3; i++ {
		e := makeEvent(i, event.TypeMealService, event.PriorityMedium, event.StatusCompleted)
		tr.Record(ResultEvent{Event: e, CompletedAt: time.Now()})
	}
	e := makeEvent(4, event.TypeBrawl, event.PriorityHigh, event.StatusExpired)
	tr.Record(ResultEvent{Event: e})

	got := tr.StressLevel()
	if got != 0.25 {
		t.Fatalf("expected 0.25 stress, got %f", got)
	}
}

func TestIgnoresNonTerminalStatus(t *testing.T) {
	tr := New()
	e := makeEvent(1, event.TypeMealService, event.PriorityHigh, event.StatusPending)
	tr.Record(ResultEvent{Event: e})

	r := tr.Report()
	if r.TotalEvents != 0 {
		t.Fatalf("expected 0 counted events for Pending status, got %d", r.TotalEvents)
	}
}

func TestConcurrentRecords(t *testing.T) {
	tr := New()
	done := make(chan struct{})

	for i := 0; i < 100; i++ {
		go func(id int) {
			status := event.StatusCompleted
			if id%4 == 0 {
				status = event.StatusExpired
			}
			e := makeEvent(id, event.TypeMealService, event.PriorityMedium, status)
			tr.Record(ResultEvent{Event: e, CompletedAt: time.Now()})
			done <- struct{}{}
		}(i)
	}

	for i := 0; i < 100; i++ {
		<-done
	}

	r := tr.Report()
	if r.TotalEvents != 100 {
		t.Fatalf("expected 100 events, got %d", r.TotalEvents)
	}
	if r.Expired != 25 {
		t.Fatalf("expected 25 expired, got %d", r.Expired)
	}
}

func TestReportByPriority(t *testing.T) {
	tr := New()
	tr.Record(ResultEvent{Event: makeEvent(1, event.TypeMealService, event.PriorityHigh, event.StatusCompleted), CompletedAt: time.Now()})
	tr.Record(ResultEvent{Event: makeEvent(2, event.TypeBrawl, event.PriorityHigh, event.StatusExpired)})
	tr.Record(ResultEvent{Event: makeEvent(3, event.TypeTableSetup, event.PriorityLow, event.StatusCompleted), CompletedAt: time.Now()})

	r := tr.Report()

	high, ok := r.ByPriority["High"]
	if !ok {
		t.Fatal("expected High priority stats")
	}
	if high.Total != 2 || high.Completed != 1 || high.Expired != 1 {
		t.Fatalf("High: got total=%d completed=%d expired=%d", high.Total, high.Completed, high.Expired)
	}
	if high.Stress != 0.5 {
		t.Fatalf("expected High stress 0.5, got %f", high.Stress)
	}

	low, ok := r.ByPriority["Low"]
	if !ok {
		t.Fatal("expected Low priority stats")
	}
	if low.Total != 1 || low.Completed != 1 || low.Expired != 0 {
		t.Fatalf("Low: got total=%d completed=%d expired=%d", low.Total, low.Completed, low.Expired)
	}
	if low.Stress != 0 {
		t.Fatalf("expected Low stress 0, got %f", low.Stress)
	}
}

func TestReportByTeam(t *testing.T) {
	tr := New()
	tr.Record(ResultEvent{Event: makeEvent(1, event.TypeMealService, event.PriorityHigh, event.StatusCompleted), CompletedAt: time.Now()})
	tr.Record(ResultEvent{Event: makeEvent(2, event.TypeCakeDelivery, event.PriorityMedium, event.StatusExpired)})
	tr.Record(ResultEvent{Event: makeEvent(3, event.TypeBrawl, event.PriorityHigh, event.StatusExpired)})

	r := tr.Report()

	catering, ok := r.ByTeam["catering"]
	if !ok {
		t.Fatal("expected catering team stats")
	}
	if catering.Total != 2 || catering.Completed != 1 || catering.Expired != 1 {
		t.Fatalf("catering: got total=%d completed=%d expired=%d", catering.Total, catering.Completed, catering.Expired)
	}
	if catering.Stress != 0.5 {
		t.Fatalf("expected catering stress 0.5, got %f", catering.Stress)
	}

	security, ok := r.ByTeam["security"]
	if !ok {
		t.Fatal("expected security team stats")
	}
	if security.Total != 1 || security.Expired != 1 {
		t.Fatalf("security: got total=%d expired=%d", security.Total, security.Expired)
	}
	if security.Stress != 1.0 {
		t.Fatalf("expected security stress 1.0, got %f", security.Stress)
	}
}

func TestReportDurations(t *testing.T) {
	tr := New()
	now := time.Now()
	e := event.Event{
		ID:       1,
		Type:     event.TypeMealService,
		Priority: event.PriorityHigh,
		Status:   event.StatusCompleted,
	}
	e.SetReceived(now.Add(-3 * time.Second))
	e.Status = event.StatusCompleted

	tr.Record(ResultEvent{Event: e, CompletedAt: now})

	r := tr.Report()
	if len(r.Durations) != 1 {
		t.Fatalf("expected 1 duration entry, got %d", len(r.Durations))
	}
	d := r.Durations[0]
	if d.EventID != 1 {
		t.Fatalf("expected event ID 1, got %d", d.EventID)
	}
	if d.Duration < 2*time.Second || d.Duration > 4*time.Second {
		t.Fatalf("expected duration ~3s, got %s", d.DurationS)
	}
	if d.Team != "catering" {
		t.Fatalf("expected team catering, got %s", d.Team)
	}
}

func TestReportJSON(t *testing.T) {
	tr := New()
	tr.Record(ResultEvent{Event: makeEvent(1, event.TypeMealService, event.PriorityHigh, event.StatusCompleted), CompletedAt: time.Now()})
	tr.Record(ResultEvent{Event: makeEvent(2, event.TypeBrawl, event.PriorityHigh, event.StatusExpired)})

	r := tr.Report()
	data, err := r.JSON()
	if err != nil {
		t.Fatalf("JSON marshal failed: %v", err)
	}

	var parsed Report
	if err := json.Unmarshal(data, &parsed); err != nil {
		t.Fatalf("JSON unmarshal failed: %v", err)
	}
	if parsed.TotalEvents != 2 {
		t.Fatalf("expected 2 total events in JSON, got %d", parsed.TotalEvents)
	}
	if parsed.OverallStress != 0.5 {
		t.Fatalf("expected 0.5 overall stress in JSON, got %f", parsed.OverallStress)
	}
}

func TestConsumeFromBroker(t *testing.T) {
	mb := broker.NewMockBroker()
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	queueName := "events.results"
	_, err := mb.DeclareQueue(ctx, queueName, broker.QueueOptions{})
	if err != nil {
		t.Fatalf("declare queue: %v", err)
	}

	tr := New()

	// Publish two result events
	e1 := makeEvent(1, event.TypeMealService, event.PriorityHigh, event.StatusCompleted)
	re1 := ResultEvent{Event: e1, CompletedAt: time.Now()}
	body1, _ := json.Marshal(re1)

	e2 := makeEvent(2, event.TypeBrawl, event.PriorityHigh, event.StatusExpired)
	re2 := ResultEvent{Event: e2}
	body2, _ := json.Marshal(re2)

	// Start consuming in background
	consumeDone := make(chan error, 1)
	go func() {
		consumeDone <- tr.Consume(ctx, mb, queueName)
	}()

	// Give consumer time to subscribe
	time.Sleep(50 * time.Millisecond)

	err = mb.Publish(ctx, broker.Message{Body: body1, ContentType: "application/json"}, broker.PublishOptions{RoutingKey: queueName})
	if err != nil {
		t.Fatalf("publish: %v", err)
	}
	err = mb.Publish(ctx, broker.Message{Body: body2, ContentType: "application/json"}, broker.PublishOptions{RoutingKey: queueName})
	if err != nil {
		t.Fatalf("publish: %v", err)
	}

	// Wait for messages to be processed
	time.Sleep(100 * time.Millisecond)
	cancel()

	<-consumeDone

	r := tr.Report()
	if r.TotalEvents != 2 {
		t.Fatalf("expected 2 events, got %d", r.TotalEvents)
	}
	if r.Completed != 1 || r.Expired != 1 {
		t.Fatalf("expected 1 completed and 1 expired, got %d/%d", r.Completed, r.Expired)
	}
	if r.OverallStress != 0.5 {
		t.Fatalf("expected 0.5 stress, got %f", r.OverallStress)
	}
}
