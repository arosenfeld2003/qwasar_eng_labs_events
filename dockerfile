# ====== Builder stage ======
FROM golang:1.22-alpine AS builder

WORKDIR /app

# Install certs (often needed for HTTPS calls)
RUN apk add --no-cache ca-certificates

# Copy go module files
COPY go.mod go.sum ./
RUN go mod download

# Copy the rest of the source
COPY . .

# Build the binary (assumes main.go in project root)
RUN CGO_ENABLED=0 GOOS=linux GOARCH=amd64 go build -o marry-me .

# ====== Runtime stage ======
FROM alpine:3.20 AS runner

# Install certs and curl for healthcheck
RUN apk add --no-cache ca-certificates curl && \
    update-ca-certificates

# Create non-root user
RUN addgroup -S app && adduser -S -G app app

WORKDIR /app

# Copy compiled binary from builder
COPY --from=builder /app/marry-me /app/marry-me

# Set ownership
RUN chown -R app:app /app

USER app

# Internal app port
EXPOSE 8080

# Healthcheck – expects GET /health to return 200
HEALTHCHECK --interval=30s --timeout=5s --retries=3 \
  CMD curl -fsS http://localhost:8080/health || exit 1

CMD ["/app/marry-me"]
