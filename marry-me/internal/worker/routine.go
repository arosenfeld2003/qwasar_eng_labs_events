package worker

import (
	"time"

	"marry-me/internal/config"
)

// RoutineState represents the current state of a worker's routine
type RoutineState int

const (
	StateWorking RoutineState = iota
	StateIdle
)

// String returns the string representation of RoutineState
func (s RoutineState) String() string {
	switch s {
	case StateWorking:
		return "working"
	case StateIdle:
		return "idle"
	default:
		return "unknown"
	}
}

// Routine manages the work/idle cycle for a worker
type Routine struct {
	routineType   config.RoutineType
	cfg           config.RoutineConfig
	state         RoutineState
	stateStart    time.Time
	cycleStart    time.Time
}

// NewRoutine creates a new routine with the specified type
func NewRoutine(routineType config.RoutineType, cfg *config.Config) *Routine {
	routineCfg := cfg.GetRoutineConfig(routineType)
	now := time.Now()

	return &Routine{
		routineType: routineType,
		cfg:         routineCfg,
		state:       StateWorking,
		stateStart:  now,
		cycleStart:  now,
	}
}

// NewRoutineWithOffset creates a routine with a time offset for staggered starts
func NewRoutineWithOffset(routineType config.RoutineType, cfg *config.Config, offset time.Duration) *Routine {
	r := NewRoutine(routineType, cfg)

	// Adjust cycle start to simulate offset
	r.cycleStart = r.cycleStart.Add(-offset)
	r.stateStart = r.stateStart.Add(-offset)

	// Recalculate initial state based on offset
	r.updateState()

	return r
}

// Type returns the routine type
func (r *Routine) Type() config.RoutineType {
	return r.routineType
}

// State returns the current state
func (r *Routine) State() RoutineState {
	r.updateState()
	return r.state
}

// IsIdle returns true if the routine is in idle state
func (r *Routine) IsIdle() bool {
	return r.State() == StateIdle
}

// CanWork returns true if the routine can currently accept work
func (r *Routine) CanWork() bool {
	return r.State() == StateIdle
}

// TimeInCurrentState returns how long the routine has been in its current state
func (r *Routine) TimeInCurrentState() time.Duration {
	r.updateState()
	return time.Since(r.stateStart)
}

// TimeUntilNextState returns the time until the next state transition
func (r *Routine) TimeUntilNextState() time.Duration {
	r.updateState()

	var stateDuration time.Duration
	if r.state == StateWorking {
		stateDuration = r.cfg.WorkDuration
	} else {
		stateDuration = r.cfg.IdleDuration
	}

	elapsed := time.Since(r.stateStart)
	remaining := stateDuration - elapsed

	if remaining < 0 {
		return 0
	}
	return remaining
}

// updateState updates the routine state based on elapsed time
func (r *Routine) updateState() {
	cycleDuration := r.cfg.WorkDuration + r.cfg.IdleDuration
	elapsed := time.Since(r.cycleStart)

	// Calculate position within current cycle
	cyclePosition := elapsed % cycleDuration

	previousState := r.state

	if cyclePosition < r.cfg.WorkDuration {
		r.state = StateWorking
		if previousState != StateWorking {
			r.stateStart = r.cycleStart.Add((elapsed / cycleDuration) * cycleDuration)
		}
	} else {
		r.state = StateIdle
		if previousState != StateIdle {
			r.stateStart = r.cycleStart.Add((elapsed/cycleDuration)*cycleDuration + r.cfg.WorkDuration)
		}
	}
}

// WorkDuration returns the configured work duration
func (r *Routine) WorkDuration() time.Duration {
	return r.cfg.WorkDuration
}

// IdleDuration returns the configured idle duration
func (r *Routine) IdleDuration() time.Duration {
	return r.cfg.IdleDuration
}

// CycleDuration returns the total cycle duration (work + idle)
func (r *Routine) CycleDuration() time.Duration {
	return r.cfg.WorkDuration + r.cfg.IdleDuration
}
