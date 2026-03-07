# Marry-Me: Wedding Event Simulation System

An event-driven wedding simulation built with **Go** and **RabbitMQ** that models real-time coordination of wedding events across multiple teams using message queues, priority scheduling, and worker pools.

## Architecture

```
Dataset (JSON)
    |
    v
[Coordinator] -- validates events, rejects invalid ones
    |
    v
[events.validated queue]
    |
    v
[Organizer] -- priority queue sorting, routes by team
    |
    v
[events.organized exchange] -- direct routing
    |
    v
[team.catering] [team.music] [team.venue] ... (10 team queues)
    |               |            |
    v               v            v
[Worker Pool]  [Worker Pool]  [Worker Pool]  (3 workers each)
    |               |            |
    v               v            v
[events.results queue]
    |
    v
[Stress Tracker] -- calculates stress metrics & report
```

## Key Components

| Package | Purpose |
|---------|---------|
| `internal/broker` | Message broker abstraction (RabbitMQ + mock for testing) |
| `internal/event` | Event types, priorities, team routing (10 teams, 30+ event types) |
| `internal/coordinator` | Event validation and ingestion into pipeline |
| `internal/organizer` | Priority queue sorting (High > Medium > Low) and team routing |
| `internal/team` | Worker pool orchestration per team with expiration checking |
| `internal/worker` | Event processing with work/idle routine cycles |
| `internal/stress` | Metrics collection and stress report generation |
| `internal/config` | CLI flags + env var configuration |

## Event Processing Rules

- **Priorities**: High (5s deadline), Medium (10s), Low (20s)
- **Processing time**: 3 seconds per event
- **Worker routines**: Standard (20s work / 5s idle), Intermittent (5s/5s), Concentrated (60s/60s)
- **10 Teams**: Catering, Decoration, Photography, Music, Coordinator, Floral, Venue, Transport, Security, Medical
- **Stress Level** = Expired Events / Total Events

## Quick Start

### Run Tests (no RabbitMQ needed)

```bash
make test-short          # Unit tests with mock broker
make test                # All tests with race detection
make test-coverage       # Generate coverage report
```

### Run the Simulation (requires RabbitMQ)

**Option A — Run locally against Docker RabbitMQ**

```bash
# 1. Start RabbitMQ (uses admin/password credentials)
docker-compose up -d rabbitmq

# 2. Wait ~10s for RabbitMQ to be ready, then build
make build

# 3. Run — RabbitMQ URL must match the docker-compose credentials
./bin/marry-me \
  --rabbitmq-url=amqp://admin:password@localhost:5672/ \
  --dataset=datasets/dataset_1.json \
  --workers=3 \
  --report=report.json
```

**Option B — Run everything in Docker (RabbitMQ + app together)**

```bash
docker-compose up
```

> The app container waits for RabbitMQ to pass its health check before starting.
> Logs from both services stream to your terminal.

### CLI Flags

```
--dataset       Path to event JSON file (default: datasets/dataset_1.json)
--rabbitmq-url  AMQP connection URL (default: amqp://guest:guest@localhost:5672/)
--workers       Workers per team (default: 3)
--speed         Simulation speed multiplier (default: 1.0)
--report        Output path for JSON stress report
--verbose       Enable verbose logging
```

## Sample Output

```
========================================
     WEDDING STRESS REPORT
========================================
  Total Events:    150
  Completed:       112
  Expired:         38
  Overall Stress:  25.3%

  By Priority:
    High       45 total |  28 completed |  17 expired | stress 37.8%
    Medium     55 total |  42 completed |  13 expired | stress 23.6%
    Low        50 total |  42 completed |   8 expired | stress 16.0%

  By Team:
    catering       18 total |  14 completed |   4 expired | stress 22.2%
    music          12 total |   9 completed |   3 expired | stress 25.0%
    ...
========================================
```

## Design Decisions

- **Broker abstraction**: Interface allows swapping RabbitMQ for in-memory mock in tests -- no external dependencies for unit tests
- **Priority queue**: `container/heap` for O(log n) priority-based event ordering
- **Non-blocking I/O**: Mutex guards data structures only, released before network calls
- **Context cancellation**: Graceful shutdown propagated through all goroutines
- **Work/idle state machine**: Workers follow realistic availability cycles per the spec

## Project Structure

```
.
├── cmd/marry-me/          # Application entry point
├── internal/
│   ├── broker/            # Broker interface + RabbitMQ + mock
│   ├── config/            # Configuration management
│   ├── coordinator/       # Event validation & ingestion
│   ├── event/             # Domain models & team routing
│   ├── logger/            # Structured logging (zerolog)
│   ├── organizer/         # Priority queue & team routing
│   ├── stress/            # Metrics & stress report
│   ├── team/              # Worker pool orchestration
│   └── worker/            # Event processing routines
├── datasets/              # 5 JSON event datasets
├── docker-compose.yml     # RabbitMQ + app containers
├── Dockerfile             # Multi-stage build
├── Makefile               # Build automation
└── main.go                # HTTP health-check server
```

## Tech Stack

- **Go 1.22** -- concurrency with goroutines and channels
- **RabbitMQ** (AMQP 0-9-1) -- durable message queues with direct exchange routing
- **zerolog** -- structured JSON logging
- **Docker Compose** -- containerized deployment
- **GitHub Actions** -- CI pipeline
