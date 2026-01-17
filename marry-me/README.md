# Marry-Me

A wedding event simulation system built in Go that processes events through a distributed pipeline using RabbitMQ, tracking stress levels based on event completion rates.

## Features

- Event-driven architecture with RabbitMQ message broker
- Priority-based event processing (High, Medium, Low)
- Multiple team queues with worker pools
- Worker routine cycles (Standard, Intermittent, Concentrated)
- Real-time stress tracking and reporting
- Docker-based development environment

## Quick Start

### Prerequisites

- Go 1.21 or later
- Docker and Docker Compose
- Make (optional, for convenience commands)

### Setup

```bash
# Clone the repository
git clone <repository-url>
cd marry-me

# Run the setup script
./scripts/setup.sh

# Or manually:
docker-compose -f docker/docker-compose.yml up -d rabbitmq
go mod download
go build -o bin/marry-me ./cmd/marry-me
```

### Running

```bash
# Run with default dataset
make run

# Run with a specific dataset
./bin/marry-me --dataset=datasets/dataset_1.json --verbose

# Run all datasets
make run-all
```

### Configuration

| Flag | Default | Description |
|------|---------|-------------|
| `--dataset` | `datasets/dataset_1.json` | Path to dataset file |
| `--rabbitmq-url` | `amqp://guest:guest@localhost:5672/` | RabbitMQ URL |
| `--workers` | `3` | Workers per team |
| `--verbose` | `false` | Enable verbose logging |
| `--speed` | `1.0` | Simulation speed multiplier |
| `--report` | `` | Output file for JSON report |

Environment variables `RABBITMQ_URL` and `DATASET_PATH` can also be used.

## Project Structure

```
marry-me/
├── cmd/marry-me/          # Application entry point
├── internal/
│   ├── broker/            # RabbitMQ broker implementation
│   ├── coordinator/       # Event validation & forwarding
│   ├── organizer/         # Event routing & priority queue
│   ├── team/              # Team manager with worker pool
│   ├── worker/            # Worker with routine cycles
│   ├── stress/            # Stress tracking & reporting
│   ├── event/             # Event types and parsing
│   ├── config/            # Configuration management
│   └── logger/            # Logging setup
├── docker/                # Docker configuration
├── scripts/               # Utility scripts
├── datasets/              # Event datasets
└── docs/                  # Documentation
```

## Dataset Format

Events are stored in JSON format:

```json
{
  "events": [
    {
      "id": "evt-001",
      "type": "meal_service",
      "priority": "high",
      "timestamp": 1234567890,
      "description": "Main course service"
    }
  ]
}
```

## Stress Calculation

Stress level = (Expired Events) / (Total Events)

Events expire if not processed before their deadline:
- High priority: 5 seconds
- Medium priority: 10 seconds
- Low priority: 20 seconds

## Development

### Running Tests

```bash
# All tests
make test

# Short tests only (no integration)
make test-short

# With coverage
make test-coverage
```

### Linting

```bash
make lint
```

### Docker

```bash
# Start RabbitMQ
make up

# Stop all
make down

# Run with Docker Compose
make docker-run

# View RabbitMQ logs
make logs
```

### RabbitMQ Management

- URL: http://localhost:15672
- Username: `guest`
- Password: `guest`

## Architecture

See [docs/architecture.md](docs/architecture.md) for detailed architecture documentation.

## License

MIT
