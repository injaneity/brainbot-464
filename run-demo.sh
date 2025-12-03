#!/bin/bash

# BrainBot Demo Runner (Refactored for Client-Server Architecture)
# Usage: ./run-demo.sh

set -e

# Colors
GREEN='\033[0;32m'
BLUE='\033[0;34m'
YELLOW='\033[1;33m'
RED='\033[0;31m'
NC='\033[0m'

echo -e "${BLUE}━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━${NC}"
echo -e "${BLUE}┃      🤖 BrainBot Demo Runner          ┃${NC}"
echo -e "${BLUE}━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━${NC}"
echo ""

# Check for Docker
if ! command -v docker &> /dev/null || ! command -v docker compose &> /dev/null; then
    echo -e "${RED}Docker or Docker Compose not found!${NC}"
    echo -e "${YELLOW}Please install Docker Desktop or Docker Engine with Docker Compose${NC}"
    exit 1
fi

echo ""

SERVICES_STARTED=false

cleanup() {
    echo ""
    echo -e "${YELLOW}Cleaning up services...${NC}"
    if [ "$SERVICES_STARTED" = true ]; then
        docker compose down
    fi
    echo -e "${GREEN}Cleanup complete${NC}"
}

# Only cleanup on INT/TERM, not on normal exit (to support detach)
trap cleanup INT TERM

# Check credentials
CREATION_ENV_FILE="creation_service/.secrets/youtube.env"
if [ ! -f "$CREATION_ENV_FILE" ]; then
    echo -e "${RED}Missing $CREATION_ENV_FILE${NC}"
    echo -e "${YELLOW}Run: creation_service/scripts/setup_creation_service_credentials.sh${NC}"
    exit 1
fi

GEN_ENV_FILE="generation_service/.env"

if [ ! -f "$GEN_ENV_FILE" ]; then
    echo -e "${RED}Missing $GEN_ENV_FILE${NC}"
    echo -e "${YELLOW}Create it with GOOGLE_API_KEY and FAL_KEY, e.g.:${NC}"
    echo "GOOGLE_API_KEY=your-gemini-api-key"
    echo "FAL_KEY=your-fal-key"
    exit 1
fi

if ! grep -q '^GOOGLE_API_KEY=' "$GEN_ENV_FILE" || ! grep -q '^FAL_KEY=' "$GEN_ENV_FILE"; then
    echo -e "${YELLOW}Warning: GOOGLE_API_KEY or FAL_KEY not set in $GEN_ENV_FILE${NC}"
    read -p "Continue? (y/n) " -n 1 -r
    echo
    [[ ! $REPLY =~ ^[Yy]$ ]] && exit 1
fi

    exit 1
fi

ROOT_ENV_FILE=".env"
if [ ! -f "$ROOT_ENV_FILE" ]; then
    echo -e "${RED}Missing root .env file${NC}"
    echo -e "${YELLOW}Please create .env with S3 and Redis configuration (see .env.example)${NC}"
    exit 1
fi

if ! grep -q '^S3_BUCKET=' "$ROOT_ENV_FILE"; then
    echo -e "${YELLOW}Warning: S3_BUCKET not set in .env${NC}"
    read -p "Continue? (y/n) " -n 1 -r
    echo
    [[ ! $REPLY =~ ^[Yy]$ ]] && exit 1
fi

set -a
source "$CREATION_ENV_FILE"
set +a

# Check if orchestrator is already running
ORCHESTRATOR_RUNNING=$(docker ps -q -f name=brainbot-orchestrator 2>/dev/null)

if [ -n "$ORCHESTRATOR_RUNNING" ]; then
    echo -e "${GREEN}✓ Orchestrator already running${NC}"
else
    echo -e "${BLUE}Building and starting services...${NC}"
    docker compose up -d --build
    SERVICES_STARTED=true
    echo ""

    wait_for_service() {
        local url=$1 name=$2 max=60 attempt=0
        echo -e "${YELLOW}Waiting for $name...${NC}"
        while [ $attempt -lt $max ]; do
            curl -s "$url" >/dev/null 2>&1 && { echo -e "${GREEN}✓ $name ready${NC}"; return 0; }
            ((attempt++)); sleep 2
        done
        echo -e "${RED}✗ $name timeout${NC}"; return 1
    }

    wait_for_service "http://localhost:8090" "Kafka UI" || exit 1
    wait_for_service "http://localhost:8000/api/v2/heartbeat" "ChromaDB" || exit 1
    wait_for_service "http://localhost:8002/health" "Generation" || true
    wait_for_service "http://localhost:8080/api/health" "API" || exit 1
    wait_for_service "http://localhost:8081/health" "Orchestrator" || exit 1
fi

# Run the TUI client
export ORCHESTRATOR_URL=http://localhost:8081

echo -e "${BLUE}Building TUI client...${NC}"
go build -o bin/demo-client demo/main.go

echo -e "${GREEN}Starting TUI client...${NC}"
echo ""

EXIT_CODE=0
./bin/demo-client --url="$ORCHESTRATOR_URL" || EXIT_CODE=$?

if [ $EXIT_CODE -eq 10 ]; then
    echo -e "${YELLOW}Shutdown requested...${NC}"
    docker compose down
    echo -e "${GREEN}Services stopped${NC}"
    exit 0
fi

# After TUI exits, services remain running (no automatic cleanup)
echo ""
echo -e "${GREEN}TUI client exited${NC}"
echo -e "${YELLOW}Orchestrator is still running in the background${NC}"
echo -e "${YELLOW}Run this script again to reconnect, or use 'docker compose down' to stop all services${NC}"
