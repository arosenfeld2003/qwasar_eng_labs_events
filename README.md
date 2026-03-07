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

- **Priorities**: High (5s deadline), Medium (10s), Low (15s)
- **Processing time**: 3 seconds per event
- **Worker routines**: Standard (20s work / 5s idle), Intermittent (5s/5s), Concentrated (60s/60s)
- **Simulation window**: 6 wedding hours (00:00–06:00); 1 real second = 1 wedding minute
- **10 Teams** with assigned routines:

| Team | Routine | Work / Idle |
|------|---------|-------------|
| Catering | Concentrated | 60s / 60s |
| Coordinator | Concentrated | 60s / 60s |
| Venue | Intermittent | 5s / 5s |
| Medical | Intermittent | 5s / 5s |
| Security | Standard | 20s / 5s |
| Decoration | Standard | 20s / 5s |
| Photography | Standard | 20s / 5s |
| Music | Standard | 20s / 5s |
| Floral | Standard | 20s / 5s |
| Transport | Standard | 20s / 5s |

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
# 1. Start RabbitMQ
docker-compose up -d rabbitmq

# 2. Wait ~10s for RabbitMQ to be ready, then build
make build

# 3. Run — use --speed=5.0 to complete in ~72s instead of 6 minutes
./bin/marry-me \
  --rabbitmq-url=amqp://admin:password@localhost:5672/ \
  --dataset=datasets/dataset_1.json \
  --workers=3 \
  --speed=5.0 \
  --report=report.json
```

**Option B — Run everything in Docker (RabbitMQ + app together)**

```bash
docker-compose up
```

> The app container waits for RabbitMQ to pass its health check before starting.
> Logs from both services stream to your terminal.

### RabbitMQ Management UI

While the simulation runs, open the management console to watch queues fill and drain live:

- **URL**: http://localhost:15672
- **Username**: `admin`
- **Password**: `password`

> Use port **15672** for the browser UI. Port 5672 is the AMQP wire protocol.

### CLI Flags

```
--dataset       Path to event JSON file (default: datasets/dataset_1.json)
--rabbitmq-url  AMQP connection URL (default: amqp://guest:guest@localhost:5672/)
--workers       Workers per team (default: 3)
--speed         Simulation speed multiplier (default: 1.0)
--report        Output path for JSON stress report
--verbose       Enable verbose logging
```

### Simulation Timing

The wedding spans 6 hours (00:00–06:00). Each real second = 1 wedding minute. Events are released at their scheduled wedding timestamp.

| `--speed` | Real duration | Use case |
|-----------|--------------|----------|
| `1.0`     | 6 min 0s     | Full fidelity |
| `5.0`     | 1 min 12s    | Demo / development |
| `10.0`    | 36s          | Quick smoke test |

## Sample Output

```
2026/03/07 09:45:46 Pipeline ready. Simulating 6-hour wedding over 1m12s (speed 5.0x)...
2026/03/07 09:46:58 Simulation complete.
2026/03/07 09:46:58 All events dispatched: 992 accepted, 0 rejected

========================================
     WEDDING STRESS REPORT
========================================
  Total Events:    713
  Completed:       677
  Expired:         36
  Overall Stress:  5.0%

  By Priority:
    High     249 total | 221 completed |  28 expired | stress 11.2%
    Medium   217 total | 209 completed |   8 expired | stress  3.7%
    Low      247 total | 247 completed |   0 expired | stress  0.0%

  By Team:
    catering      98 total |  86 completed |  12 expired | stress 12.2%
    coordinator  208 total | 184 completed |  24 expired | stress 11.5%
    venue        143 total | 143 completed |   0 expired | stress  0.0%
    security     143 total | 143 completed |   0 expired | stress  0.0%
    medical       38 total |  38 completed |   0 expired | stress  0.0%
    music         42 total |  42 completed |   0 expired | stress  0.0%
    decoration    41 total |  41 completed |   0 expired | stress  0.0%
========================================
```

> Catering and Coordinator (Concentrated routine: 60s/60s) accumulate stress on high-priority events.
> Teams on Intermittent (Venue, Medical) and Standard (Security, others) routines reach 0% stress.

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
