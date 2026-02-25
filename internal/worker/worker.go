package worker

import (
    "context"
    "encoding/json"
    "log"
    "time"

    "github.com/arosenfeld2003/qwasar_eng_labs_events/internal/broker"
    "github.com/arosenfeld2003/qwasar_eng_labs_events/internal/event"
    "github.com/arosenfeld2003/qwasar_eng_labs_events/internal/organizer"
    "github.com/arosenfeld2003/qwasar_eng_labs_events/internal/stress"
)

// RoutineType identifies common worker routines.
type RoutineType string

const (
    RoutineStandard     RoutineType = "standard"
    RoutineIntermittent RoutineType = "intermittent"
    RoutineConcentrated RoutineType = "concentrated"
)

// Routine defines durations for a worker's work/idle cycle.
type Routine struct {
    Work time.Duration
    Idle time.Duration
}

// Predefined routines (useful defaults for the simulation).
var DefaultRoutines = map[RoutineType]Routine{
    RoutineStandard:     {Work: 20 * time.Second, Idle: 5 * time.Second},
    RoutineIntermittent: {Work: 5 * time.Second, Idle: 5 * time.Second},
    RoutineConcentrated: {Work: 60 * time.Second, Idle: 60 * time.Second},
}

// Worker processes events for a single team following a routine cycle.
type Worker struct {
    ID             int
    Team           event.Team
    Broker         broker.Broker
    Routine        Routine
    ProcessingTime time.Duration // handling time per event
}

// New constructs a worker with the given parameters.
func New(id int, t event.Team, b broker.Broker, r Routine) *Worker {
    return &Worker{ID: id, Team: t, Broker: b, Routine: r, ProcessingTime: 3 * time.Second}
}

// Run subscribes to the team's queue and processes events. It accepts
// events only during the idle phase of its routine. Processed results are
// published to the results queue (organizer.QueueResults).
func (w *Worker) Run(ctx context.Context) error {
    sub, err := w.Broker.Subscribe(ctx, broker.ConsumeOptions{Queue: organizer.TeamQueue(w.Team), AutoAck: true})
    if err != nil {
        return err
    }
    defer sub.Cancel()

    // state channel toggles idle availability
    idleCh := make(chan bool, 1)
    go func() {
        // Start in work phase (unavailable) per README semantics.
        for {
            select {
            case <-ctx.Done():
                return
            default:
            }
            // Work phase
            idleCh <- false
            select {
            case <-time.After(w.Routine.Work):
            case <-ctx.Done():
                return
            }

            // Idle phase
            idleCh <- true
            select {
            case <-time.After(w.Routine.Idle):
            case <-ctx.Done():
                return
            }
        }
    }()

    // Current availability state
    idle := false
    // non-blocking drain of idleCh to update state when toggles occur
    updateIdle := func() {
        for {
            select {
            case s := <-idleCh:
                idle = s
            default:
                return
            }
        }
    }

    for {
        select {
        case <-ctx.Done():
            return ctx.Err()
        case s := <-idleCh:
            idle = s
        case d, ok := <-sub.Deliveries:
            if !ok {
                return nil
            }
            // Make sure we have the latest state before accepting
            updateIdle()
            if !idle {
                // Not accepting events while working.
                continue
            }

            var ev event.Event
            if err := json.Unmarshal(d.Body, &ev); err != nil {
                log.Printf("worker: unmarshal event: %v", err)
                continue
            }

            // Process in a goroutine so that the worker can keep toggling
            // availability and other deliveries can be observed/dropped.
            go func(e event.Event) {
                // Mark processing
                e.Status = event.StatusProcessing

                // Simulate handling time
                select {
                case <-time.After(w.ProcessingTime):
                case <-ctx.Done():
                    return
                }

                now := time.Now()
                var out []byte
                // Check expiration
                if e.IsExpired(now) {
                    e.Status = event.StatusExpired
                    // publish wrapper with expired status
                    re := stress.ResultEvent{Event: e}
                    b, _ := json.Marshal(re)
                    out = b
                } else {
                    e.Status = event.StatusCompleted
                    re := stress.ResultEvent{Event: e, CompletedAt: now}
                    b, _ := json.Marshal(re)
                    out = b
                }

                _ = w.Broker.Publish(context.Background(), broker.Message{Body: out, ContentType: "application/json"}, broker.PublishOptions{RoutingKey: organizer.QueueResults})
            }(ev)
        }
    }
}
