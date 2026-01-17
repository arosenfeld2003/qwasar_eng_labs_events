# Build stage
FROM golang:1.21-alpine AS builder

WORKDIR /build

# Copy go mod files first for better caching
COPY go.mod go.sum ./
RUN go mod download

# Copy source code
COPY . .

# Build the binary
RUN CGO_ENABLED=0 GOOS=linux go build -o marry-me ./cmd/marry-me

# Runtime stage
FROM alpine:3.18

WORKDIR /app

# Install ca-certificates for HTTPS
RUN apk --no-cache add ca-certificates

# Copy binary from builder
COPY --from=builder /build/marry-me .

# Create datasets directory
RUN mkdir -p /app/datasets

ENTRYPOINT ["./marry-me"]
