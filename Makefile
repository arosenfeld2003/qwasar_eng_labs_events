.PHONY: build test test-short clean lint fmt vet run

# Binary output directory
BIN_DIR := bin
BINARY := $(BIN_DIR)/marry-me

# Go parameters
GOCMD := go
GOBUILD := $(GOCMD) build
GOTEST := $(GOCMD) test
GOCLEAN := $(GOCMD) clean
GOVET := $(GOCMD) vet
GOFMT := gofmt

# Build flags
LDFLAGS := -ldflags="-s -w"

# Default target
all: build

# Build the binary
build:
	@mkdir -p $(BIN_DIR)
	$(GOBUILD) $(LDFLAGS) -o $(BINARY) ./cmd/marry-me

# Run all tests
test:
	$(GOTEST) -v -race ./...

# Run tests without integration tests
test-short:
	$(GOTEST) -v -short ./...

# Run tests with coverage
test-coverage:
	$(GOTEST) -v -race -coverprofile=coverage.out ./...
	$(GOCMD) tool cover -html=coverage.out -o coverage.html

# Clean build artifacts
clean:
	$(GOCLEAN)
	rm -rf $(BIN_DIR)
	rm -f coverage.out coverage.html

# Run linter (requires golangci-lint)
lint:
	golangci-lint run ./...

# Format code
fmt:
	$(GOFMT) -s -w .

# Run go vet
vet:
	$(GOVET) ./...

# Run the application
run: build
	$(BINARY)

# Run with a specific dataset
run-dataset: build
	$(BINARY) --dataset=datasets/dataset_1.json

# Download dependencies
deps:
	$(GOCMD) mod download
	$(GOCMD) mod tidy

# Docker targets
docker-build:
	docker build -t marry-me -f docker/Dockerfile .

docker-up:
	docker-compose -f docker/docker-compose.yml up -d

docker-down:
	docker-compose -f docker/docker-compose.yml down

# Help
help:
	@echo "Available targets:"
	@echo "  build          - Build the binary"
	@echo "  test           - Run all tests with race detection"
	@echo "  test-short     - Run tests without integration tests"
	@echo "  test-coverage  - Run tests with coverage report"
	@echo "  clean          - Remove build artifacts"
	@echo "  lint           - Run golangci-lint"
	@echo "  fmt            - Format code"
	@echo "  vet            - Run go vet"
	@echo "  run            - Build and run the application"
	@echo "  run-dataset    - Run with dataset_1.json"
	@echo "  deps           - Download and tidy dependencies"
	@echo "  docker-build   - Build Docker image"
	@echo "  docker-up      - Start Docker containers"
	@echo "  docker-down    - Stop Docker containers"
