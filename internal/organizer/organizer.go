package organizer

import (
	"container/heap"
	"context"
	"encoding/json"
	"fmt"
	"log"
	"sync"
	"sync/atomic"
	"time"

	"github.com/arosenfeld2003/qwasar_eng_labs_events/internal/broker"
	"github.com/arosenfeld2003/qwasar_eng_labs_events/internal/event"
)

// Queue / exchange names used by the organizer.
const (
	QueueValidated    = "events.validated"
	ExchangeOrganized = "events.organized"
	QueueResults      = "events.results"
)

// allTeams lists every team that gets a dedicated queue.
var allTeams = []event.Team{
	event.TeamCatering,
	event.TeamDecoration,
	event.TeamPhotography,
	event.TeamMusic,
	event.TeamCoordinator,
	event.TeamFloral,
	event.TeamVenue,
	event.TeamTransport,
	event.TeamSecurity,
	event.TeamMedical,
}

// TeamQueue returns the queue name for a given team (e.g. "team.catering").
func TeamQueue(t event.Team) string {
	return "team." + string(t)
}

// ---- priority queue (container/heap) ------------------------------------

// priorityQueue orders events so that the highest priority is dequeued first.
// Order: High (index 0) > Medium (1) > Low (2).
type priorityQueue []*event.Event

func priorityIndex(p event.Priority) int {
	switch p {
	case event.PriorityHigh:
		return 0
	case event.PriorityMedium:
		return 1
	case event.PriorityLow:
		return 2
	default:
		return 3
	}
}

func (pq priorityQueue) Len() int { return len(pq) }

func (pq priorityQueue) Less(i, j int) bool {
	return priorityIndex(pq[i].Priority) < priorityIndex(pq[j].Priority)
}

func (pq priorityQueue) Swap(i, j int) { pq[i], pq[j] = pq[j], pq[i] }

func (pq *priorityQueue) Push(x interface{}) {
	*pq = append(*pq, x.(*event.Event))
}

func (pq *priorityQueue) Pop() interface{} {
	old := *pq
	n := len(old)
	item := old[n-1]
	old[n-1] = nil // avoid memory leak
	*pq = old[:n-1]
	return item
}

// ---- Stats --------------------------------------------------------------

// Stats holds organizer processing counters.
type Stats struct {
	Routed  int64
	Expired int64
}

// ---- Organizer ----------------------------------------------------------

// Organizer subscribes to validated events, sorts them by priority, and
// routes each event to the appropriate team queue.
type Organizer struct {
	broker  broker.Broker
	routed  atomic.Int64
	expired atomic.Int64
	mu      sync.Mutex // guards pq
	pq     priorityQueue
}

// New creates a new Organizer backed by the given broker.
func New(b broker.Broker) *Organizer {
	o := &Organizer{broker: b}
	heap.Init(&o.pq)
	return o
}

// Setup declares the exchange, team queues, bindings, and results queue.
func (o *Organizer) Setup(ctx context.Context) error {
	// Declare the organised exchange (direct routing).
	if err := o.broker.DeclareExchange(ctx, ExchangeOrganized, broker.ExchangeOptions{
		Kind:    "direct",
		Durable: true,
	}); err != nil {
		return fmt.Errorf("declare exchange: %w", err)
	}

	// Declare a queue for every team and bind it to the exchange.
	for _, team := range allTeams {
		qName := TeamQueue(team)
		if _, err := o.broker.DeclareQueue(ctx, qName, broker.QueueOptions{Durable: true}); err != nil {
			return fmt.Errorf("declare queue %s: %w", qName, err)
		}
		if err := o.broker.BindQueue(ctx, qName, ExchangeOrganized, string(team), nil); err != nil {
			return fmt.Errorf("bind queue %s: %w", qName, err)
		}
	}

	// Declare the validated input queue.
	if _, err := o.broker.DeclareQueue(ctx, QueueValidated, broker.QueueOptions{Durable: true}); err != nil {
		return fmt.Errorf("declare validated queue: %w", err)
	}

	// Declare the results queue.
	if _, err := o.broker.DeclareQueue(ctx, QueueResults, broker.QueueOptions{Durable: true}); err != nil {
		return fmt.Errorf("declare results queue: %w", err)
	}

	return nil
}

// Run subscribes to the validated queue and processes events until ctx is
// cancelled. Each event is pushed into the priority queue and then the queue
// is drained in priority order to team queues.
func (o *Organizer) Run(ctx context.Context) error {
	sub, err := o.broker.Subscribe(ctx, broker.ConsumeOptions{
		Queue:   QueueValidated,
		AutoAck: true,
	})
	if err != nil {
		return fmt.Errorf("subscribe to %s: %w", QueueValidated, err)
	}

	for {
		select {
		case <-ctx.Done():
			_ = sub.Cancel()
			return ctx.Err()
		case d, ok := <-sub.Deliveries:
			if !ok {
				return nil
			}
			o.handleDelivery(ctx, d)
		}
	}
}

func (o *Organizer) handleDelivery(ctx context.Context, d broker.Delivery) {
	var ev event.Event
	if err := json.Unmarshal(d.Body, &ev); err != nil {
		log.Printf("organizer: unmarshal error: %v", err)
		return
	}

	ev.SetReceived(time.Now())

	team := ev.Team()
	if team == event.TeamUnknown {
		log.Printf("organizer: unknown team for event %d (type %s)", ev.ID, ev.Type)
		return
	}

	// Push into priority queue, drain under lock, then route outside the lock
	// so that broker.Publish (potential network I/O) does not hold the mutex.
	o.mu.Lock()
	heap.Push(&o.pq, &ev)
	batch := o.drainLocked()
	o.mu.Unlock()

	for _, e := range batch {
		o.routeEvent(ctx, e)
	}
}

// drainLocked pops all events from the priority queue and returns them.
// Caller must hold o.mu.
func (o *Organizer) drainLocked() []*event.Event {
	var batch []*event.Event
	for o.pq.Len() > 0 {
		batch = append(batch, heap.Pop(&o.pq).(*event.Event))
	}
	return batch
}

func (o *Organizer) routeEvent(ctx context.Context, ev *event.Event) {
	data, err := json.Marshal(ev)
	if err != nil {
		log.Printf("organizer: marshal error: %v", err)
		return
	}
	team := ev.Team()
	if err := o.broker.Publish(ctx, broker.Message{
		Body:        data,
		ContentType: "application/json",
	}, broker.PublishOptions{
		Exchange:   ExchangeOrganized,
		RoutingKey: string(team),
	}); err != nil {
		log.Printf("organizer: publish to team %s: %v", team, err)
		return
	}
	o.routed.Add(1)
}

func (o *Organizer) publishExpired(ctx context.Context, ev *event.Event) {
	ev.Status = event.StatusExpired
	data, err := json.Marshal(ev)
	if err != nil {
		log.Printf("organizer: marshal expired event: %v", err)
		return
	}
	if err := o.broker.Publish(ctx, broker.Message{
		Body:        data,
		ContentType: "application/json",
	}, broker.PublishOptions{
		RoutingKey: QueueResults,
	}); err != nil {
		log.Printf("organizer: publish expired event: %v", err)
		return
	}
	o.expired.Add(1)
}

// Stats returns a snapshot of the organizer's processing counters.
func (o *Organizer) Stats() Stats {
	return Stats{
		Routed:  o.routed.Load(),
		Expired: o.expired.Load(),
	}
}
