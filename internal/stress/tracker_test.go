package stress

import (
	"testing"
	"time"

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
