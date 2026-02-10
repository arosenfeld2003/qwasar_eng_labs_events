# Marry-Me Wedding Event System

Go + RabbitMQ event-driven wedding simulation.

## Quick Commands

```bash
go test -short ./...          # Run unit tests (no RabbitMQ needed)
go test -v -race ./...        # Run all tests with race detection
go test -coverprofile=coverage.out ./...  # Coverage report
```

## Project Structure

- `internal/broker/` - Message broker interface + RabbitMQ impl + mock
- `internal/event/` - Event types, priorities, team routing, models
- `main.go` - HTTP health-check server (placeholder entry point)
- `datasets/` - JSON test data files

## Conventions

- Go 1.22, module `github.com/arosenfeld2003/qwasar_eng_labs_events`
- Table-driven tests using `testing` stdlib
- Mock broker (`internal/broker/mock.go`) for tests — no RabbitMQ required
- String-based enums for Priority, Status, EventType, Team
- Tests tagged `-short` skip integration tests requiring external services
