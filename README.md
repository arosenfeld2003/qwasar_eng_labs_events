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

## Prerequisites

- **Go 1.22+** -- [install](https://go.dev/doc/install)
- **Docker & Docker Compose** -- for RabbitMQ
- **Make** -- build automation (ships with macOS/Linux)

## Step-by-Step: Build, Test, and Demo

### Step 1: Install Dependencies

```bash
# Download Go modules
make deps

# Verify everything compiles
go build ./...
```

### Step 2: Run Unit Tests (no RabbitMQ needed)

Tests use an in-memory mock broker, so no external services are required.

```bash
# Quick unit tests (recommended first step)
make test-short

# Full test suite with race detection
make test

# Generate HTML coverage report
make test-coverage
open coverage.html
```

### Step 3: Start RabbitMQ

```bash
# Start RabbitMQ in the background (with management UI)
docker-compose up -d rabbitmq

# Verify it's healthy (wait ~15s for startup)
docker-compose ps
```

Once running, the RabbitMQ management UI is available at http://localhost:15672
(login: `admin` / `password`).

### Step 4: Build the Application

```bash
# Build the binary to ./bin/marry-me
make build

# Verify the binary was created
ls -la bin/marry-me
```

### Step 5: Run the Simulation

```bash
# Run with default settings (dataset_1, 3 workers per team)
./bin/marry-me

# Run with a specific dataset and save the report
./bin/marry-me --dataset=datasets/dataset_1.json --workers=3 --report=report.json

# Run with verbose logging to see per-event details
./bin/marry-me --dataset=datasets/dataset_2.json --verbose

# Try different worker counts to see how stress changes
./bin/marry-me --workers=1 --report=stress_1worker.json
./bin/marry-me --workers=5 --report=stress_5workers.json
```

Press `Ctrl+C` at any time to stop the simulation and see the stress report.

### Step 6: View the Report

```bash
# Pretty-print the JSON report
cat report.json | python3 -m json.tool

# Or just check the console output -- the stress report prints automatically
```

### Full Docker Demo (alternative)

If you prefer to run everything in containers:

```bash
# Build and start both RabbitMQ and the app
docker-compose up --build

# Stop everything
docker-compose down
```

### All Makefile Targets

```bash
make build          # Build binary to ./bin/marry-me
make test           # Run all tests with race detection
make test-short     # Run unit tests only (no RabbitMQ needed)
make test-coverage  # Run tests + generate coverage.html
make run            # Build and run the application
make run-dataset    # Build and run with datasets/dataset_1.json
make clean          # Remove build artifacts
make fmt            # Format all Go code
make vet            # Run go vet static analysis
make lint           # Run golangci-lint (requires install)
make deps           # Download and tidy Go modules
make docker-build   # Build Docker image
make docker-up      # Start all Docker containers
make docker-down    # Stop all Docker containers
make help           # Show all available targets
```

### CLI Flags Reference

```
Flag              Default                                  Description
--dataset         datasets/dataset_1.json                  Path to event JSON file
--rabbitmq-url    amqp://guest:guest@localhost:5672/        RabbitMQ connection URL
--workers         3                                        Number of workers per team
--speed           1.0                                      Simulation speed multiplier
--report          (none)                                   Output path for JSON stress report
--verbose         false                                    Enable verbose logging
```

Environment variables `DATASET_PATH` and `RABBITMQ_URL` can be used as
alternatives to the `--dataset` and `--rabbitmq-url` flags.

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
