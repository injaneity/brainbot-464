#!/bin/bash

# BrainBot Docker Compose Demo Runner
# This script orchestrates the entire BrainBot demo using Docker containers

set -e

# Colors
GREEN='\033[0;32m'
BLUE='\033[0;34m'
YELLOW='\033[1;33m'
RED='\033[0;31m'
NC='\033[0m'

# Track if we started services
SERVICES_STARTED=false

# Cleanup function
cleanup() {
    echo ""
    echo -e "${YELLOW}Cleaning up services...${NC}"

    if [ "$SERVICES_STARTED" = true ]; then
        echo -e "${YELLOW}Stopping Docker services...${NC}"
        docker-compose down
    fi

    echo -e "${GREEN}Cleanup complete${NC}"
}

# Set trap to call cleanup on exit
trap cleanup EXIT INT TERM

echo -e "${BLUE}╔════════════════════════════════════════╗${NC}"
echo -e "${BLUE}║   🤖 BrainBot Docker Demo Runner     ║${NC}"
echo -e "${BLUE}╚════════════════════════════════════════╝${NC}"
echo ""

# Check for required environment variables
CREATION_ENV_FILE="creation_service/.secrets/youtube.env"
if [ ! -f "$CREATION_ENV_FILE" ]; then
    echo -e "${RED}Missing $CREATION_ENV_FILE${NC}"
    echo -e "${YELLOW}Run creation_service/scripts/setup_creation_service_credentials.sh first to generate OAuth env vars.${NC}"
    exit 1
fi

# Check for generation service API keys
if [ -z "$GEMINI_API_KEY" ] || [ -z "$FAL_KEY" ]; then
    echo -e "${YELLOW}Warning: GEMINI_API_KEY and/or FAL_KEY not set in environment${NC}"
    echo -e "${YELLOW}Generation service will not function without these keys${NC}"
    echo -e "${YELLOW}Set them in your environment or create a .env file${NC}"
    echo ""
    read -p "Continue anyway? (y/n) " -n 1 -r
    echo
    if [[ ! $REPLY =~ ^[Yy]$ ]]; then
        exit 1
    fi
fi

# Load environment variables from creation service
set -a
# shellcheck disable=SC1090
source "$CREATION_ENV_FILE"
set +a

echo -e "${BLUE}Step 1/3: Building Docker images...${NC}"
docker-compose build
echo -e "${GREEN}✓ Images built successfully${NC}"
echo ""

echo -e "${BLUE}Step 2/3: Starting all services...${NC}"
docker-compose up -d
SERVICES_STARTED=true
echo -e "${GREEN}✓ Services started${NC}"
echo ""

# Function to wait for a service to be ready
wait_for_service() {
    local url=$1
    local name=$2
    local max_attempts=60
    local attempt=0

    echo -e "${YELLOW}Waiting for $name to be ready...${NC}"
    while [ $attempt -lt $max_attempts ]; do
        if curl -s "$url" >/dev/null 2>&1; then
            echo -e "${GREEN}$name is ready!${NC}"
            return 0
        fi
        attempt=$((attempt + 1))
        sleep 2
    done

    echo -e "${RED}$name failed to start (timeout after ${max_attempts}s)${NC}"
    echo -e "${YELLOW}Check logs with: docker-compose logs $name${NC}"
    return 1
}

echo -e "${BLUE}Step 3/3: Waiting for services to be ready...${NC}"

# Wait for infrastructure services
if wait_for_service "http://localhost:8090" "Kafka UI"; then
    echo -e "${GREEN}✓ Kafka is ready${NC}"
else
    echo -e "${RED}✗ Kafka failed to start${NC}"
    echo -e "${YELLOW}Check logs with: docker-compose logs kafka${NC}"
    exit 1
fi

if wait_for_service "http://localhost:8000/api/v1/heartbeat" "ChromaDB"; then
    echo -e "${GREEN}✓ ChromaDB is ready${NC}"
else
    echo -e "${RED}✗ ChromaDB failed to start${NC}"
    echo -e "${YELLOW}Check logs with: docker-compose logs chromadb${NC}"
    exit 1
fi

# Wait for application services
if wait_for_service "http://localhost:8002/health" "Generation Service"; then
    echo -e "${GREEN}✓ Generation Service is ready${NC}"
else
    echo -e "${YELLOW}⚠ Generation Service may not be ready (continuing anyway)${NC}"
fi

if wait_for_service "http://localhost:8080/api/health" "API Service"; then
    echo -e "${GREEN}✓ API Service is ready${NC}"
else
    echo -e "${RED}✗ API Service failed to start${NC}"
    echo -e "${YELLOW}Check logs with: docker-compose logs ingestion-service${NC}"
    exit 1
fi

echo -e "${GREEN}✓ Creation Service running in Kafka consumer mode${NC}"
echo ""

# Display service status
echo -e "${BLUE}═══════════════════════════════════════${NC}"
echo -e "${GREEN}All Services Ready!${NC}"
echo -e "  Kafka:              http://localhost:9093 ✓"
echo -e "  Kafka UI:           http://localhost:8090 ✓"
echo -e "  ChromaDB:           http://localhost:8000 ✓"
echo -e "  Generation Service: http://localhost:8002 ✓"
echo -e "  Ingestion Service:  http://localhost:8080 ✓"
echo -e "  Creation Service:   Kafka Consumer Mode ✓"
echo -e "${BLUE}═══════════════════════════════════════${NC}"
echo ""

# Show logs information
echo -e "${YELLOW}View Service Logs:${NC}"
echo -e "  All services:       docker-compose logs -f"
echo -e "  Ingestion Service:  docker-compose logs -f ingestion-service"
echo -e "  Generation Service: docker-compose logs -f generation-service"
echo -e "  Creation Service:   docker-compose logs -f creation-service"
echo -e "  Kafka:              docker-compose logs -f kafka"
echo -e "  ChromaDB:           docker-compose logs -f chromadb"
echo ""

echo -e "${GREEN}Architecture:${NC}"
echo -e "  RSS Feed → API Service → Generation Service → Kafka → Creation Service → YouTube"
echo ""

# Export environment variables for the demo
export API_URL=http://localhost:8080
export WEBHOOK_PORT=9999
export GENERATION_SERVICE_URL=http://localhost:8002

# Run the demo (this will block until demo exits)
echo -e "${GREEN}Starting demo client...${NC}"
echo -e "${YELLOW}Press 'd' to start the demo workflow${NC}"
echo -e "${YELLOW}Press 'q' or Ctrl+C to quit${NC}"
echo ""

go run demo/main.go

# Cleanup will be called automatically by the trap
