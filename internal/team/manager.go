package team

import (
	"context"
	"fmt"
	"log"
	"sync"
	"time"

	"github.com/arosenfeld2003/qwasar_eng_labs_events/internal/broker"
	"github.com/arosenfeld2003/qwasar_eng_labs_events/internal/event"
	"github.com/arosenfeld2003/qwasar_eng_labs_events/internal/worker"
)

// Manager orchestrates worker pools per team.
type Manager struct {
	broker         broker.Broker
	workersPerTeam int
	speed          float64

	mu    sync.Mutex
	teams map[event.Team]*teamState
}

// teamState holds runtime state for a single team.
type teamState struct {
	team    event.Team
	workers []*worker.Worker
	cancel  context.CancelFunc
}

// New creates a new team manager. speed is the simulation speed multiplier
// (e.g. 5.0 means all worker timers run 5x faster).
func New(b broker.Broker, workersPerTeam int, speed float64) *Manager {
	if speed <= 0 {
		speed = 1.0
	}
	return &Manager{
		broker:         b,
		workersPerTeam: workersPerTeam,
		speed:          speed,
		teams:          make(map[event.Team]*teamState),
	}
}

// Setup initializes workers for all teams.
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
		event.TeamSecurity,
		event.TeamMedical,
	}

	for _, t := range teamsToSetup {
		if err := m.setupTeam(ctx, t); err != nil {
			return fmt.Errorf("setup team %s: %w", t, err)
		}
	}
	return nil
}

func (m *Manager) setupTeam(ctx context.Context, t event.Team) error {
	tctx, cancel := context.WithCancel(ctx)

	// Scale work/idle durations by speed multiplier.
	routine := worker.Routine{
		Work: time.Duration(float64(20*time.Second) / m.speed),
		Idle: time.Duration(float64(5*time.Second) / m.speed),
	}

	workers := make([]*worker.Worker, m.workersPerTeam)
	for i := 0; i < m.workersPerTeam; i++ {
		w := worker.New(i, t, m.broker, routine)
		// Scale processing time by speed multiplier.
		w.ProcessingTime = time.Duration(float64(3*time.Second) / m.speed)
		workers[i] = w
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

	m.mu.Lock()
	m.teams[t] = ts
	m.mu.Unlock()

	return nil
}

// Shutdown cancels all team worker goroutines.
func (m *Manager) Shutdown(ctx context.Context) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	for _, ts := range m.teams {
		ts.cancel()
	}
	return nil
}
