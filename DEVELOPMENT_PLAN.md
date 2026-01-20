# Marry-Me Development Plan

## Overview

Rebuild the Marry-Me wedding event simulation system **from scratch** as a collaborative learning project with 3 developers. The existing implementation on `main` branch will be preserved as a reference. The system processes wedding events through a distributed pipeline using RabbitMQ, tracking stress levels based on event completion rates.

## Branch Strategy

1. **Main development branch:** `dev/marry-me-rebuild`
2. **Keep existing code** on `main` as reference (do not delete)
3. Each developer works on feature branches off `dev/marry-me-rebuild`:
   - `feature/infrastructure` (Developer A)
   - `feature/pipeline` (Developer B)
   - `feature/processing` (Developer C)

## GitHub Project Board

Project board named **"Marry-Me Rebuild"** with columns:
- **Backlog** - All unstarted issues
- **In Progress** - Currently being worked on
- **In Review** - PRs submitted, awaiting review
- **Done** - Completed and merged

---

## Development Phases & Work Distribution

### Phase 1: Foundation (Week 1)
All developers can work in parallel.

| Developer | Component | Files to Create |
|-----------|-----------|-----------------|
| A | Project setup + Broker | `go.mod`, `internal/broker/` |
| B | Event types + Models | `internal/event/` |
| C | Config + Logging + Docker | `internal/config/`, `internal/logger/`, `docker/` |

### Phase 2: Core Pipeline (Week 2)
Sequential dependencies: A's broker needed by B and C.

| Developer | Component | Dependencies |
|-----------|-----------|--------------|
| A | Coordinator | Broker, Events |
| B | Organizer + Priority Queue | Broker, Events, Coordinator interface |
| C | Team Manager | Broker, Events |

### Phase 3: Processing & Integration (Week 3)

| Developer | Component | Dependencies |
|-----------|-----------|--------------|
| A | Worker + Routines | Team interface |
| B | Stress Tracker | Broker, Events |
| C | Main app + CLI | All components |

### Phase 4: Testing & Polish (Week 4)
- All: Unit tests for owned components
- Integration tests with RabbitMQ
- Documentation updates

---

## Directory Structure

```
marry-me/
├── cmd/
│   └── marry-me/
│       └── main.go
├── internal/
│   ├── broker/
│   │   ├── broker.go
│   │   ├── rabbitmq.go
│   │   └── mock.go
│   ├── coordinator/
│   │   └── coordinator.go
│   ├── organizer/
│   │   └── organizer.go
│   ├── team/
│   │   └── team.go
│   ├── worker/
│   │   └── worker.go
│   ├── stress/
│   │   └── tracker.go
│   ├── event/
│   │   └── types.go
│   ├── config/
│   │   └── config.go
│   └── logger/
│       └── logger.go
├── datasets/
│   └── dataset_1.json
├── docker/
│   ├── Dockerfile
│   └── docker-compose.yml
├── scripts/
│   └── setup.sh
├── docs/
├── .github/
│   └── workflows/
│       └── ci.yml
├── go.mod
├── go.sum
├── Makefile
└── README.md
```

---

## Critical Files

| File | Purpose |
|------|---------|
| `cmd/marry-me/main.go` | Application entry point |
| `internal/broker/rabbitmq.go` | RabbitMQ implementation |
| `internal/event/types.go` | Event definitions |
| `internal/coordinator/coordinator.go` | Event validation |
| `internal/organizer/organizer.go` | Event routing |
| `internal/team/team.go` | Team management |
| `internal/worker/worker.go` | Worker implementation |
| `internal/stress/tracker.go` | Stress tracking |
| `docker/docker-compose.yml` | Container orchestration |
| `.github/workflows/ci.yml` | CI pipeline |

---

## Verification Plan

1. **Unit Tests**: `make test-short` - all pass
2. **Integration Tests**: `make test` with RabbitMQ running
3. **Manual Test**:
   ```bash
   docker-compose up -d rabbitmq
   ./bin/marry-me --dataset=datasets/dataset_1.json --verbose
   ```
4. **Verify stress report** output shows completed/expired breakdown
5. **Check RabbitMQ UI** at http://localhost:15672 for queue activity

---

## Issue Tracking

All work is tracked through GitHub Issues linked to the "Marry-Me Rebuild" project board. See the repository issues for the full breakdown of tasks by developer.

### Issue Summary by Developer

**Developer A (Infrastructure & Core):**
- Issue #1: Project Setup and Go Module Initialization
- Issue #2: RabbitMQ Broker Abstraction
- Issue #3: Coordinator Component
- Issue #4: Worker Implementation with Routines

**Developer B (Pipeline Components):**
- Issue #5: Event Types and Data Models
- Issue #6: Organizer and Priority Queue
- Issue #7: Stress Tracker Component

**Developer C (Processing & Integration):**
- Issue #8: Configuration and CLI Flags
- Issue #9: Logger Setup with Zerolog
- Issue #10: Team Manager Component
- Issue #11: Docker and Docker Compose Setup
- Issue #12: Main Application Entry Point

**All Developers (Testing & CI/CD):**
- Issue #13: Unit Test Suite
- Issue #14: Integration Tests
- Issue #15: GitHub Actions CI Pipeline
- Issue #16: Sample Datasets
