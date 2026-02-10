package organizer

import (
	"container/heap"
	"context"
	"encoding/json"
	"testing"
	"time"

	"github.com/arosenfeld2003/qwasar_eng_labs_events/internal/broker"
	"github.com/arosenfeld2003/qwasar_eng_labs_events/internal/event"
)

// helpers ------------------------------------------------------------------

func makeEvent(id int, typ event.EventType, pri event.Priority) event.Event {
	return event.Event{
		ID:       id,
		Type:     typ,
		Priority: pri,
	}
}

func mustJSON(t *testing.T, v interface{}) []byte {
	t.Helper()
	data, err := json.Marshal(v)
	if err != nil {
		t.Fatal(err)
	}
	return data
}

func publishEvent(t *testing.T, ctx context.Context, mb *broker.MockBroker, ev event.Event) {
	t.Helper()
	if err := mb.Publish(ctx, broker.Message{
		Body:        mustJSON(t, ev),
		ContentType: "application/json",
	}, broker.PublishOptions{RoutingKey: QueueValidated}); err != nil {
		t.Fatal(err)
	}
}

// startRun launches o.Run in a goroutine and waits for the organizer's
// subscription to be registered in the mock broker before returning.
func startRun(t *testing.T, ctx context.Context, mb *broker.MockBroker, o *Organizer) {
	t.Helper()
	go o.Run(ctx)
	// Allow the goroutine to reach Subscribe and register its channel.
	// The mock broker's Subscribe is synchronous, so a brief runtime yield
	// is sufficient to ensure the channel exists before we publish.
	time.Sleep(20 * time.Millisecond)
}

// waitStats polls until the predicate is true or the timeout expires.
func waitStats(o *Organizer, pred func(Stats) bool, timeout time.Duration) bool {
	deadline := time.Now().Add(timeout)
	for time.Now().Before(deadline) {
		if pred(o.Stats()) {
			return true
		}
		time.Sleep(5 * time.Millisecond)
	}
	return false
}

// tests --------------------------------------------------------------------

func TestSetupDeclaresQueuesAndExchange(t *testing.T) {
	mb := broker.NewMockBroker()
	o := New(mb)
	ctx := context.Background()

	if err := o.Setup(ctx); err != nil {
		t.Fatal(err)
	}

	// Verify by publishing to exchange with team routing keys and checking
	// that the mock broker resolves bindings. We subscribe to a team queue
	// and publish through the exchange.
	sub, err := mb.Subscribe(ctx, broker.ConsumeOptions{Queue: TeamQueue(event.TeamCatering)})
	if err != nil {
		t.Fatal(err)
	}
	defer sub.Cancel()

	if err := mb.Publish(ctx, broker.Message{Body: []byte("test")}, broker.PublishOptions{
		Exchange:   ExchangeOrganized,
		RoutingKey: string(event.TeamCatering),
	}); err != nil {
		t.Fatal(err)
	}

	select {
	case d := <-sub.Deliveries:
		if string(d.Body) != "test" {
			t.Errorf("body = %q", string(d.Body))
		}
	case <-time.After(200 * time.Millisecond):
		t.Fatal("timeout waiting for delivery through exchange binding")
	}
}

func TestRoutesToCorrectTeam(t *testing.T) {
	mb := broker.NewMockBroker()
	o := New(mb)
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	if err := o.Setup(ctx); err != nil {
		t.Fatal(err)
	}

	// Subscribe to catering team queue.
	sub, err := mb.Subscribe(ctx, broker.ConsumeOptions{Queue: TeamQueue(event.TeamCatering)})
	if err != nil {
		t.Fatal(err)
	}
	defer sub.Cancel()

	startRun(t, ctx, mb, o)

	ev := makeEvent(1, event.TypeMealService, event.PriorityMedium)
	publishEvent(t, ctx, mb, ev)

	select {
	case d := <-sub.Deliveries:
		var got event.Event
		if err := json.Unmarshal(d.Body, &got); err != nil {
			t.Fatal(err)
		}
		if got.ID != 1 {
			t.Errorf("event ID = %d, want 1", got.ID)
		}
	case <-time.After(500 * time.Millisecond):
		t.Fatal("timeout waiting for routed event")
	}

	if !waitStats(o, func(s Stats) bool { return s.Routed >= 1 }, 500*time.Millisecond) {
		t.Errorf("Stats.Routed = %d, want >= 1", o.Stats().Routed)
	}
}

func TestRoutesToSecurityTeam(t *testing.T) {
	mb := broker.NewMockBroker()
	o := New(mb)
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	if err := o.Setup(ctx); err != nil {
		t.Fatal(err)
	}

	sub, err := mb.Subscribe(ctx, broker.ConsumeOptions{Queue: TeamQueue(event.TeamSecurity)})
	if err != nil {
		t.Fatal(err)
	}
	defer sub.Cancel()

	startRun(t, ctx, mb, o)

	publishEvent(t, ctx, mb, makeEvent(100, event.TypeAccident, event.PriorityMedium))

	select {
	case d := <-sub.Deliveries:
		var got event.Event
		if err := json.Unmarshal(d.Body, &got); err != nil {
			t.Fatal(err)
		}
		if got.ID != 100 {
			t.Errorf("event ID = %d, want 100", got.ID)
		}
	case <-time.After(500 * time.Millisecond):
		t.Fatal("timeout waiting for security team event")
	}
}

func TestPublishExpiredDirectly(t *testing.T) {
	mb := broker.NewMockBroker()
	o := New(mb)
	ctx := context.Background()

	if err := o.Setup(ctx); err != nil {
		t.Fatal(err)
	}

	// Subscribe to results queue to receive expired events.
	sub, err := mb.Subscribe(ctx, broker.ConsumeOptions{Queue: QueueResults})
	if err != nil {
		t.Fatal(err)
	}
	defer sub.Cancel()

	// Directly call publishExpired to test the expired path.
	ev := makeEvent(50, event.TypeBrawl, event.PriorityHigh)
	ev.SetReceived(time.Now().Add(-10 * time.Second))

	o.publishExpired(ctx, &ev)

	select {
	case d := <-sub.Deliveries:
		var got event.Event
		if err := json.Unmarshal(d.Body, &got); err != nil {
			t.Fatal(err)
		}
		if got.ID != 50 {
			t.Errorf("event ID = %d, want 50", got.ID)
		}
		if got.Status != event.StatusExpired {
			t.Errorf("status = %q, want %q", got.Status, event.StatusExpired)
		}
	case <-time.After(200 * time.Millisecond):
		t.Fatal("timeout waiting for expired event on results queue")
	}

	if o.Stats().Expired != 1 {
		t.Errorf("Stats.Expired = %d, want 1", o.Stats().Expired)
	}
}

func TestPriorityQueueOrdering(t *testing.T) {
	var pq priorityQueue
	heap.Init(&pq)

	low := &event.Event{ID: 1, Priority: event.PriorityLow}
	high := &event.Event{ID: 2, Priority: event.PriorityHigh}
	med := &event.Event{ID: 3, Priority: event.PriorityMedium}

	heap.Push(&pq, low)
	heap.Push(&pq, high)
	heap.Push(&pq, med)

	first := heap.Pop(&pq).(*event.Event)
	second := heap.Pop(&pq).(*event.Event)
	third := heap.Pop(&pq).(*event.Event)

	if first.Priority != event.PriorityHigh {
		t.Errorf("first = %s, want High", first.Priority)
	}
	if second.Priority != event.PriorityMedium {
		t.Errorf("second = %s, want Medium", second.Priority)
	}
	if third.Priority != event.PriorityLow {
		t.Errorf("third = %s, want Low", third.Priority)
	}
}

func TestStatsTracking(t *testing.T) {
	mb := broker.NewMockBroker()
	o := New(mb)
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	if err := o.Setup(ctx); err != nil {
		t.Fatal(err)
	}

	startRun(t, ctx, mb, o)

	// Publish multiple events to different teams.
	publishEvent(t, ctx, mb, makeEvent(1, event.TypeMealService, event.PriorityHigh))
	publishEvent(t, ctx, mb, makeEvent(2, event.TypeBandSetup, event.PriorityMedium))
	publishEvent(t, ctx, mb, makeEvent(3, event.TypePhotoSession, event.PriorityLow))

	if !waitStats(o, func(s Stats) bool { return s.Routed >= 3 }, time.Second) {
		t.Errorf("Stats.Routed = %d, want >= 3", o.Stats().Routed)
	}
}

func TestUnknownEventTypeHandledGracefully(t *testing.T) {
	mb := broker.NewMockBroker()
	o := New(mb)
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	if err := o.Setup(ctx); err != nil {
		t.Fatal(err)
	}

	startRun(t, ctx, mb, o)

	// Publish an event with an unknown type.
	ev := event.Event{ID: 42, Type: "totally_unknown", Priority: event.PriorityHigh}
	publishEvent(t, ctx, mb, ev)

	// Give the organizer time to process.
	time.Sleep(100 * time.Millisecond)

	// The event should not be routed.
	s := o.Stats()
	if s.Routed != 0 {
		t.Errorf("Stats.Routed = %d, want 0 for unknown event type", s.Routed)
	}
}

func TestMultipleEventsRoutedToDifferentTeams(t *testing.T) {
	mb := broker.NewMockBroker()
	o := New(mb)
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	if err := o.Setup(ctx); err != nil {
		t.Fatal(err)
	}

	// Subscribe to multiple team queues.
	subCatering, _ := mb.Subscribe(ctx, broker.ConsumeOptions{Queue: TeamQueue(event.TeamCatering)})
	defer subCatering.Cancel()
	subMusic, _ := mb.Subscribe(ctx, broker.ConsumeOptions{Queue: TeamQueue(event.TeamMusic)})
	defer subMusic.Cancel()

	startRun(t, ctx, mb, o)

	publishEvent(t, ctx, mb, makeEvent(10, event.TypeCakeDelivery, event.PriorityHigh))
	publishEvent(t, ctx, mb, makeEvent(20, event.TypeDJSetup, event.PriorityLow))

	// Verify catering received its event.
	select {
	case d := <-subCatering.Deliveries:
		var got event.Event
		json.Unmarshal(d.Body, &got)
		if got.ID != 10 {
			t.Errorf("catering event ID = %d, want 10", got.ID)
		}
	case <-time.After(500 * time.Millisecond):
		t.Fatal("timeout waiting for catering event")
	}

	// Verify music received its event.
	select {
	case d := <-subMusic.Deliveries:
		var got event.Event
		json.Unmarshal(d.Body, &got)
		if got.ID != 20 {
			t.Errorf("music event ID = %d, want 20", got.ID)
		}
	case <-time.After(500 * time.Millisecond):
		t.Fatal("timeout waiting for music event")
	}
}

func TestTeamQueueNaming(t *testing.T) {
	tests := []struct {
		team event.Team
		want string
	}{
		{event.TeamCatering, "team.catering"},
		{event.TeamDecoration, "team.decoration"},
		{event.TeamPhotography, "team.photography"},
		{event.TeamMusic, "team.music"},
		{event.TeamCoordinator, "team.coordinator"},
		{event.TeamSecurity, "team.security"},
	}
	for _, tt := range tests {
		if got := TeamQueue(tt.team); got != tt.want {
			t.Errorf("TeamQueue(%s) = %q, want %q", tt.team, got, tt.want)
		}
	}
}
