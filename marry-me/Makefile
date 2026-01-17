.PHONY: all build test clean up down run run-all logs help lint fmt

# Default target
all: build

# Build the application
build:
	@echo "Building marry-me..."
	@mkdir -p bin
	go build -o bin/marry-me ./cmd/marry-me

# Run tests
test:
	@echo "Running tests..."
	go test -v ./...

# Run tests with coverage
test-coverage:
	@echo "Running tests with coverage..."
	go test -coverprofile=coverage.out ./...
	go tool cover -html=coverage.out -o coverage.html
	@echo "Coverage report: coverage.html"

# Run short tests only (skip integration tests)
test-short:
	@echo "Running short tests..."
	go test -short ./...

# Clean build artifacts
clean:
	@echo "Cleaning..."
	rm -rf bin/
	rm -f coverage.out coverage.html

# Start RabbitMQ container
up:
	@echo "Starting RabbitMQ..."
	docker-compose -f docker/docker-compose.yml up -d rabbitmq

# Stop all containers
down:
	@echo "Stopping containers..."
	docker-compose -f docker/docker-compose.yml down

# Build and run with default dataset
run: build
	@echo "Running with dataset_1.json..."
	./bin/marry-me --dataset=datasets/dataset_1.json --verbose

# Run all datasets
run-all: build
	@for i in 1 2 3 4 5; do \
		echo ""; \
		echo "========================================"; \
		echo "=== Running Dataset $$i ==="; \
		echo "========================================"; \
		./bin/marry-me --dataset=datasets/dataset_$$i.json; \
	done

# View RabbitMQ logs
logs:
	docker-compose -f docker/docker-compose.yml logs -f rabbitmq

# Run with Docker
docker-run:
	docker-compose -f docker/docker-compose.yml up --build

# Lint the code
lint:
	@echo "Linting..."
	@which golangci-lint > /dev/null || (echo "Installing golangci-lint..." && go install github.com/golangci/golangci-lint/cmd/golangci-lint@latest)
	golangci-lint run ./...

# Format the code
fmt:
	@echo "Formatting..."
	go fmt ./...

# Download dependencies
deps:
	@echo "Downloading dependencies..."
	go mod download
	go mod tidy

# Generate mocks (if using mockgen)
mocks:
	@echo "Generating mocks..."
	@which mockgen > /dev/null || go install github.com/golang/mock/mockgen@latest
	# Add mockgen commands here as needed

# Show help
help:
	@echo "Marry-Me Makefile"
	@echo ""
	@echo "Usage:"
	@echo "  make build        Build the application"
	@echo "  make test         Run all tests"
	@echo "  make test-short   Run short tests only"
	@echo "  make test-coverage Run tests with coverage report"
	@echo "  make clean        Remove build artifacts"
	@echo "  make up           Start RabbitMQ container"
	@echo "  make down         Stop all containers"
	@echo "  make run          Build and run with dataset_1"
	@echo "  make run-all      Run all 5 datasets"
	@echo "  make logs         View RabbitMQ logs"
	@echo "  make docker-run   Run with Docker Compose"
	@echo "  make lint         Run linter"
	@echo "  make fmt          Format code"
	@echo "  make deps         Download dependencies"
	@echo "  make help         Show this help"
