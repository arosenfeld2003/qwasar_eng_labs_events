#!/usr/bin/env bash
set -euo pipefail

echo "[marry-me] Checking Docker installation..."
if ! command -v docker >/dev/null 2>&1; then
  echo "Error: docker is not installed or not in PATH." >&2
  exit 1
fi

# Support both docker compose v2 and legacy docker-compose
if command -v docker-compose >/dev/null 2>&1; then
  COMPOSE_CMD="docker-compose"
elif docker compose version >/dev/null 2>&1; then
  COMPOSE_CMD="docker compose"
else
  echo "Error: docker compose is not available (neither 'docker compose' nor 'docker-compose')." >&2
  exit 1
fi

echo "[marry-me] Building images..."
$COMPOSE_CMD build

echo "[marry-me] Starting services (RabbitMQ + app)..."
$COMPOSE_CMD up -d

echo "[marry-me] Services started."
echo
echo "RabbitMQ AMQP:        localhost:5672"
echo "RabbitMQ Management:  http://localhost:15672  (user: marryme, pass: marryme_password)"
echo "marry-me app:         http://localhost:8080"

