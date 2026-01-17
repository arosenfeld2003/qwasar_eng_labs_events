# Marry-Me Architecture

## Overview

Marry-Me is a wedding event simulation system that processes events through a multi-stage pipeline using RabbitMQ for message passing. The system tracks "stress levels" based on the ratio of events that expire before being processed.

## System Components

```
┌─────────────┐     ┌─────────────┐     ┌─────────────┐     ┌─────────────┐
│   Dataset   │────>│ Coordinator │────>│  Organizer  │────>│    Teams    │
│   (JSON)    │     │ (Validate)  │     │  (Route)    │     │  (Process)  │
└─────────────┘     └─────────────┘     └─────────────┘     └─────────────┘
                           │                   │                   │
                           │                   │                   │
                           v                   v                   v
                    ┌─────────────────────────────────────────────────────┐
                    │                    RabbitMQ                          │
                    │  - events.incoming    - team.catering                │
                    │  - events.validated   - team.photography             │
                    │  - events.results     - team.music  (etc.)           │
                    └─────────────────────────────────────────────────────┘
                                            │
                                            v
                                   ┌─────────────┐
                                   │   Stress    │
                                   │   Tracker   │
                                   └─────────────┘
```

## Component Details

### Coordinator (`internal/coordinator/`)
- Receives raw events from the dataset
- Validates event types against known mappings
- Sets `ReceivedAt` timestamp and calculates deadline
- Forwards valid events to the validated queue
- Rejects invalid events with logging

### Organizer (`internal/organizer/`)
- Consumes from validated events queue
- Routes events to appropriate team queues based on event type
- Handles multi-team event types (routes to primary team)
- Detects and reports expired events

### Teams (`internal/team/`)
- Each team has its own queue and worker pool
- Maintains a local priority queue (high > medium > low)
- Assigns events to idle workers
- Periodically checks for and removes expired events

### Workers (`internal/worker/`)
- Operate on work/idle cycles (routines)
- Only accept events during idle phase
- Process events with configurable handling time (default 3s)
- Report completion or expiration to team

### Stress Tracker (`internal/stress/`)
- Consumes from results queue
- Tracks completed vs expired events
- Calculates overall stress level
- Generates detailed reports by priority and team

## Routine Types

Workers operate on different cycles:

| Routine | Work | Idle | Use Case |
|---------|------|------|----------|
| Standard | 20s | 5s | Normal processing |
| Intermittent | 5s | 5s | Frequent availability |
| Concentrated | 60s | 60s | Batch processing |

## Priority & Deadlines

| Priority | Deadline |
|----------|----------|
| High | 5 seconds |
| Medium | 10 seconds |
| Low | 20 seconds |

## Event Types

Events are mapped to teams that can handle them:

- **Catering**: meal_service, cake_delivery, bar_setup, dessert_service
- **Decoration**: table_setup, flower_arrangement, lighting_setup
- **Photography**: photo_session, group_photo, candid_capture, video_recording
- **Music**: band_setup, dj_setup, first_dance, live_performance
- **Coordinator**: guest_arrival, ceremony_start, reception_start
- **Floral**: bouquet_delivery, floral_setup, petal_scatter
- **Venue**: venue_prep, seating_arrangement, cleanup
- **Transport**: guest_pickup, bridal_transport, vendor_delivery

Some events can be handled by multiple teams (e.g., `first_dance` by Music and Photography).

## Data Flow

1. Dataset loaded from JSON file
2. Events grouped by timestamp
3. Ticker-based injection at simulation speed
4. Coordinator validates and forwards
5. Organizer routes to team queues
6. Teams assign to idle workers
7. Workers process and report results
8. Stress tracker records outcomes
9. Final report generated

## Configuration

### CLI Flags
- `--dataset`: Path to dataset JSON file
- `--rabbitmq-url`: RabbitMQ connection URL
- `--workers`: Number of workers per team
- `--verbose`: Enable verbose logging
- `--speed`: Simulation speed multiplier
- `--report`: Output file for stress report

### Environment Variables
- `RABBITMQ_URL`: RabbitMQ connection URL
- `DATASET_PATH`: Path to dataset file
