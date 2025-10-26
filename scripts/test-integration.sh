#!/usr/bin/env bash
set -euo pipefail

# Simple integration test runner using docker-compose.test.yml
# - brings up chroma + redis (redis-stack includes RedisBloom)
# - waits for redis to respond to PING
# - sets REDIS_ADDR and runs `go test ./...` (you can change which tests run)
# - tears down the compose setup on exit

ROOT_DIR="$( cd "$( dirname "${BASH_SOURCE[0]}" )/.." && pwd )"
COMPOSE_FILE="$ROOT_DIR/docker-compose.test.yml"

echo "Starting test services via docker-compose..."
docker compose -f "$COMPOSE_FILE" up -d

function teardown() {
  echo "Tearing down test services..."
  docker compose -f "$COMPOSE_FILE" down -v --remove-orphans
}
trap teardown EXIT

echo "Waiting for Redis to be ready..."
# Wait until redis-cli PING returns PONG inside the redis container
until docker compose -f "$COMPOSE_FILE" exec -T redis redis-cli PING 2>/dev/null | grep -q PONG; do
  echo -n "."
  sleep 1
done
echo "\nRedis ready. Running tests..."

# Export environment variable expected by the integration tests
export REDIS_ADDR="127.0.0.1:6379"

# Run tests (adjust package or -run filter as needed)
go test ./... -v

echo "Integration tests finished. Cleaning up..."
