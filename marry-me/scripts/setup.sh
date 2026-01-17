#!/bin/bash

# Marry-Me Setup Script
# One-command local development setup

set -e

echo "========================================"
echo "  Marry-Me Development Setup"
echo "========================================"
echo ""

# Check prerequisites
echo "Checking prerequisites..."

# Check Go
if ! command -v go &> /dev/null; then
    echo "Error: Go is not installed. Please install Go 1.21 or later."
    exit 1
fi
GO_VERSION=$(go version | awk '{print $3}' | sed 's/go//')
echo "  Go: $GO_VERSION"

# Check Docker
if ! command -v docker &> /dev/null; then
    echo "Error: Docker is not installed. Please install Docker."
    exit 1
fi
echo "  Docker: $(docker --version | awk '{print $3}' | sed 's/,//')"

# Check Docker Compose
if ! command -v docker-compose &> /dev/null; then
    echo "Error: Docker Compose is not installed."
    exit 1
fi
echo "  Docker Compose: $(docker-compose --version | awk '{print $4}')"

echo ""

# Download dependencies
echo "Downloading Go dependencies..."
go mod download
go mod tidy
echo "  Dependencies downloaded."

echo ""

# Start RabbitMQ
echo "Starting RabbitMQ..."
docker-compose -f docker/docker-compose.yml up -d rabbitmq

# Wait for RabbitMQ to be healthy
echo "Waiting for RabbitMQ to be ready..."
MAX_ATTEMPTS=30
ATTEMPT=0
while [ $ATTEMPT -lt $MAX_ATTEMPTS ]; do
    if docker exec marry-me-rabbitmq rabbitmq-diagnostics -q ping &> /dev/null; then
        echo "  RabbitMQ is ready!"
        break
    fi
    ATTEMPT=$((ATTEMPT + 1))
    echo "  Waiting... (attempt $ATTEMPT/$MAX_ATTEMPTS)"
    sleep 2
done

if [ $ATTEMPT -eq $MAX_ATTEMPTS ]; then
    echo "Error: RabbitMQ did not become ready in time."
    exit 1
fi

echo ""

# Build the application
echo "Building the application..."
mkdir -p bin
go build -o bin/marry-me ./cmd/marry-me
echo "  Built: bin/marry-me"

echo ""

# Run tests
echo "Running tests..."
if go test -short ./... ; then
    echo "  All tests passed!"
else
    echo "  Warning: Some tests failed. Check output above."
fi

echo ""
echo "========================================"
echo "  Setup Complete!"
echo "========================================"
echo ""
echo "Quick start:"
echo "  make run          - Run with dataset_1.json"
echo "  make run-all      - Run all datasets"
echo "  make test         - Run tests"
echo ""
echo "RabbitMQ Management UI:"
echo "  URL: http://localhost:15672"
echo "  Username: guest"
echo "  Password: guest"
echo ""
