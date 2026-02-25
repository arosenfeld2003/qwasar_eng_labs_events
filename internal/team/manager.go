package team

import (
	"container/heap"
	"context"
	"encoding/json"
	"fmt"
	"log"
	"sync"
	"time"

	"github.com/arosenfeld2003/qwasar_eng_labs_events/internal/broker"
	"github.com/arosenfeld2003/qwasar_eng_labs_events/internal/event"
	"github.com/arosenfeld2003/qwasar_eng_labs_events/internal/organizer"
	"github.com/arosenfeld2003/qwasar_eng_labs_events/internal/worker"
)

// Stats tracks statistics for a team.
type Stats struct {
	Pending   int64
	Completed int64
	Expired   int64
}

// Manager orchestrates worker pools and event distribution per team.
type Manager struct {
	broker          broker.Broker
	workersPerTeam  int
	expirationCheck time.Duration

	mu      sync.Mutex // guards the following fields
	teams   map[event.Team]*teamState
	stats   map[event.Team]Stats
	ready   sync.WaitGroup // signals when all intake subscriptions are ready
}

// teamState holds runtime state for a single team.
type teamState struct {
	team    event.Team
	workers []*worker.Worker
	pq      priorityQueue
	cancel  context.CancelFunc
}

// priorityQueue orders events by priority (high > medium > low).
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
	old[n-1] = nil
	*pq = old[:n-1]
	return item
}

// New creates a new team manager.
func New(b broker.Broker, workersPerTeam int) *Manager {
	return &Manager{
		broker:          b,
		workersPerTeam:  workersPerTeam,
		expirationCheck: 1 * time.Second,
		teams:           make(map[event.Team]*teamState),
		stats:           make(map[event.Team]Stats),
	}
}

// Setup initializes workers for all teams and sets up subscriptions.
func (m *Manager) Setup(ctx context.Context) error {
	teamsToSetup := []event.Team{
		event.TeamCatering,
		event.TeamDecoration,
		event.TeamPhotography,
		event.TeamMusic,
		event.TeamCoordinator,
		event.TeamFloral,
		event.TeamVenue,
		event.TeamTransport,
	}

	// Wait for all intake subscriptions to be ready
	m.ready.Add(len(teamsToSetup))

	for _, t := range teamsToSetup {
		if err := m.setupTeam(ctx, t); err != nil {
			return fmt.Errorf("setup team %s: %w", t, err)
		}
	}

	// Block until all intake goroutines are subscribed
	m.ready.Wait()
	return nil
}

func (m *Manager) setupTeam(ctx context.Context, t event.Team) error {
	tctx, cancel := context.WithCancel(ctx)

	// Create workers for this team
	workers := make([]*worker.Worker, m.workersPerTeam)
	for i := 0; i < m.workersPerTeam; i++ {
		w := worker.New(i, t, m.broker, worker.DefaultRoutines[worker.RoutineStandard])
		workers[i] = w
		// Start each worker in a goroutine
		go func(w *worker.Worker) {
			if err := w.Run(tctx); err != nil && err != context.Canceled {
				log.Printf("worker error for team %s: %v", t, err)
			}
		}(w)
	}

	ts := &teamState{
		team:    t,
		workers: workers,
		cancel:  cancel,
	}
	heap.Init(&ts.pq)

	m.mu.Lock()
	m.teams[t] = ts
	m.stats[t] = Stats{}
	m.mu.Unlock()

	// Start event intake for this team
	go m.intakeEvents(tctx, t)

	// Start expiration checker for this team
	go m.checkExpiration(tctx, t)

	return nil
}

// intakeEvents subscribes to a team's queue and buffers events.
func (m *Manager) intakeEvents(ctx context.Context, t event.Team) {
	sub, err := m.broker.Subscribe(ctx, broker.ConsumeOptions{
		Queue:   organizer.TeamQueue(t),
		AutoAck: true,
	})
	if err != nil {
		log.Printf("team %s: subscribe error: %v", t, err)
		m.ready.Done()
		return
	}
	defer sub.Cancel()

	// Signal that subscription is ready
	m.ready.Done()

	for {
		select {
		case <-ctx.Done():
			return
		case d, ok := <-sub.Deliveries:
			if !ok {
				return
			}
			var ev event.Event
			if err := json.Unmarshal(d.Body, &ev); err != nil {
				log.Printf("team %s: unmarshal error: %v", t, err)
				continue
			}

			m.mu.Lock()
			ts := m.teams[t]
			if ts != nil {
				heap.Push(&ts.pq, &ev)
				s := m.stats[t]
				s.Pending++
				m.stats[t] = s
			}
			m.mu.Unlock()
		}
	}
}

// checkExpiration periodically checks for and removes expired events.
func (m *Manager) checkExpiration(ctx context.Context, t event.Team) {
	ticker := time.NewTicker(m.expirationCheck)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			now := time.Now()
			m.mu.Lock()
			ts := m.teams[t]
			if ts == nil {
				m.mu.Unlock()
				continue
			}

			// Drain expired events from the front of the queue
			for ts.pq.Len() > 0 {
				front := ts.pq[0]
				if !front.IsExpired(now) {
					break
				}
				_ = heap.Pop(&ts.pq)

				// Publish expired event to results queue
				front.Status = event.StatusExpired
				data, _ := json.Marshal(front)
				_ = m.broker.Publish(ctx, broker.Message{Body: data, ContentType: "application/json"},
					broker.PublishOptions{RoutingKey: organizer.QueueResults})

				// Update stats
				s := m.stats[t]
				s.Pending--
				s.Expired++
				m.stats[t] = s
			}
			m.mu.Unlock()
		}
	}
}

// Stats returns a snapshot of statistics for a team.
func (m *Manager) Stats(t event.Team) Stats {
	m.mu.Lock()
	defer m.mu.Unlock()
	return m.stats[t]
}

// AllStats returns stats for all teams.
func (m *Manager) AllStats() map[event.Team]Stats {
	m.mu.Lock()
	defer m.mu.Unlock()
	out := make(map[event.Team]Stats, len(m.stats))
	for k, v := range m.stats {
		out[k] = v
	}
	return out
}

// Shutdown gracefully stops all team managers.
func (m *Manager) Shutdown(ctx context.Context) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	for _, ts := range m.teams {
		ts.cancel()
	}
	return nil
}
