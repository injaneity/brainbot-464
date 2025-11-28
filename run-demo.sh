#!/bin/bash

# Complete demo runner with Kafka integration
# This script orchestrates the entire BrainBot demo with Kafka message queue

set -e

# Colors
GREEN='\033[0;32m'
BLUE='\033[0;34m'
YELLOW='\033[1;33m'
RED='\033[0;31m'
NC='\033[0m'

# Track services we started
API_SERVER_STARTED=false
API_SERVER_PID=""
GEN_SERVICE_STARTED=false
GEN_SERVICE_PID=""
CREATION_SERVICE_STARTED=false
CREATION_SERVICE_PID=""
CHROMA_STARTED=false
KAFKA_STARTED=false

# Cleanup function
cleanup() {
    echo ""
    echo -e "${YELLOW}Cleaning up services...${NC}"

    # Kill API server if we started it
    if [ "$API_SERVER_STARTED" = true ] && [ -n "$API_SERVER_PID" ]; then
        echo -e "${YELLOW}Stopping API Server (PID: $API_SERVER_PID)...${NC}"
        kill $API_SERVER_PID 2>/dev/null || true
    fi

    # Kill generation service if we started it
    if [ "$GEN_SERVICE_STARTED" = true ] && [ -n "$GEN_SERVICE_PID" ]; then
        echo -e "${YELLOW}Stopping Generation Service (PID: $GEN_SERVICE_PID)...${NC}"
        kill $GEN_SERVICE_PID 2>/dev/null || true
    fi

    # Kill creation service if we started it
    if [ "$CREATION_SERVICE_STARTED" = true ] && [ -n "$CREATION_SERVICE_PID" ]; then
        echo -e "${YELLOW}Stopping Creation Service (PID: $CREATION_SERVICE_PID)...${NC}"
        kill $CREATION_SERVICE_PID 2>/dev/null || true
    fi

    # Stop ChromaDB if we started it
    if [ "$CHROMA_STARTED" = true ]; then
        echo -e "${YELLOW}Stopping ChromaDB...${NC}"
        docker stop brainbot-chroma 2>/dev/null || true
        docker rm brainbot-chroma 2>/dev/null || true
    fi

    # Stop Kafka if we started it
    if [ "$KAFKA_STARTED" = true ]; then
        echo -e "${YELLOW}Stopping Kafka...${NC}"
        docker-compose -f docker-compose.kafka.yml down
    fi

    echo -e "${GREEN}Cleanup complete${NC}"
}

# Set trap to call cleanup on exit
trap cleanup EXIT INT TERM

echo -e "${BLUE}╔════════════════════════════════════════╗${NC}"
echo -e "${BLUE}║   🤖 BrainBot Kafka Demo Runner      ║${NC}"
echo -e "${BLUE}╚════════════════════════════════════════╝${NC}"
echo ""

# Ensure required binaries are available
if ! command -v ffmpeg >/dev/null 2>&1; then
    echo -e "${RED}Missing dependency: ffmpeg${NC}"
    echo -e "${YELLOW}Install via 'brew install ffmpeg' (macOS) or your distro's package manager, then re-run this script.${NC}"
    exit 1
fi

# Function to check if a port is in use
check_port() {
    lsof -Pi :$1 -sTCP:LISTEN -t >/dev/null 2>&1
}

# Function to wait for a service to be ready
wait_for_service() {
    local url=$1
    local name=$2
    local max_attempts=30
    local attempt=0

    echo -e "${YELLOW}Waiting for $name to be ready...${NC}"
    while [ $attempt -lt $max_attempts ]; do
        if curl -s "$url" >/dev/null 2>&1; then
            echo -e "${GREEN}$name is ready!${NC}"
            return 0
        fi
        attempt=$((attempt + 1))
        sleep 1
    done

    echo -e "${RED}$name failed to start${NC}"
    return 1
}

# Step 1: Start Kafka
echo -e "${BLUE}Step 1/6: Starting Kafka...${NC}"
docker-compose -f docker-compose.kafka.yml up -d
KAFKA_STARTED=true

if wait_for_service "http://localhost:8090" "Kafka UI"; then
    echo -e "${GREEN}✓ Kafka started successfully${NC}"
    echo -e "${GREEN}  Kafka UI: http://localhost:8090${NC}"
else
    echo -e "${RED}✗ Failed to start Kafka${NC}"
    exit 1
fi
echo ""

# Step 2: Start ChromaDB
echo -e "${BLUE}Step 2/6: Starting ChromaDB...${NC}"
if check_port 8000; then
    echo -e "${GREEN}ChromaDB already running on port 8000${NC}"
else
    # Remove existing container if it exists
    docker rm brainbot-chroma 2>/dev/null || true

    # Start new container
    docker run -d \
        --name brainbot-chroma \
        -p 8000:8000 \
        -v "$(pwd)/chroma_data:/chroma/chroma" \
        -e IS_PERSISTENT=TRUE \
        -e ANONYMIZED_TELEMETRY=FALSE \
        chromadb/chroma:latest >/dev/null 2>&1

    CHROMA_STARTED=true

    if wait_for_service "http://localhost:8000/api/v1" "ChromaDB"; then
        echo -e "${GREEN}✓ ChromaDB started successfully${NC}"
    else
        echo -e "${RED}✗ Failed to start ChromaDB${NC}"
        exit 1
    fi
fi
echo ""

# Step 3: Start Generation Service
echo -e "${BLUE}Step 3/6: Starting Generation Service...${NC}"
if check_port 8002; then
    echo -e "${GREEN}Generation Service already running on port 8002${NC}"
else
    cd generation_service
    source venv/bin/activate
    export PORT=8002
    export KAFKA_ENABLED=true
    export KAFKA_BOOTSTRAP_SERVERS=localhost:9093
    nohup python -m app.main > ../generation_service.log 2>&1 &
    GEN_SERVICE_PID=$!
    GEN_SERVICE_STARTED=true
    cd ..

    if wait_for_service "http://localhost:8002/health" "Generation Service"; then
        echo -e "${GREEN}✓ Generation Service started successfully (PID: $GEN_SERVICE_PID)${NC}"
    else
        echo -e "${YELLOW}⚠ Generation Service may not be fully ready (continuing anyway)${NC}"
    fi
fi
echo ""

# Step 4: Start Creation Service (Kafka Consumer Mode)
echo -e "${BLUE}Step 4/6: Starting Creation Service (Kafka Consumer)...${NC}"

CREATION_ENV_FILE="creation_service/.secrets/youtube.env"
if [ -f "$CREATION_ENV_FILE" ]; then
    set -a
    # shellcheck disable=SC1090
    source "$CREATION_ENV_FILE"
    set +a
else
    echo -e "${RED}Missing $CREATION_ENV_FILE${NC}"
    echo -e "${YELLOW}Run creation_service/scripts/setup_creation_service_credentials.sh first to generate OAuth env vars.${NC}"
    exit 1
fi

cd creation_service
export KAFKA_BOOTSTRAP_SERVERS=localhost:9093
export KAFKA_TOPIC_VIDEO_REQUESTS=video-processing-requests
export KAFKA_CONSUMER_GROUP_ID=creation-service-consumer-group
nohup go run main.go -kafka > ../creation_service.log 2>&1 &
CREATION_SERVICE_PID=$!
CREATION_SERVICE_STARTED=true
cd ..

sleep 3
echo -e "${GREEN}✓ Creation Service started in Kafka mode (PID: $CREATION_SERVICE_PID)${NC}"
echo ""

# Step 5: Start API Server
echo -e "${BLUE}Step 5/6: Starting API Server...${NC}"
if check_port 8080; then
    echo -e "${GREEN}API Server already running on port 8080${NC}"
else
    nohup go run main.go > api_server.log 2>&1 &
    API_SERVER_PID=$!
    API_SERVER_STARTED=true

    if wait_for_service "http://localhost:8080/api/health" "API Server"; then
        echo -e "${GREEN}✓ API Server started successfully (PID: $API_SERVER_PID)${NC}"
    else
        echo -e "${RED}✗ Failed to start API Server${NC}"
        echo -e "${YELLOW}Check api_server.log for details${NC}"
        exit 1
    fi
fi
echo ""

# Step 6: Display status
echo -e "${BLUE}Step 6/6: All services ready!${NC}"
echo ""

# Display service status
echo -e "${BLUE}═══════════════════════════════════════${NC}"
echo -e "${GREEN}Services Status:${NC}"
echo -e "  Kafka:              http://localhost:9093 ✓"
echo -e "  Kafka UI:           http://localhost:8090 ✓"
echo -e "  ChromaDB:           http://localhost:8000 ✓"
echo -e "  Generation Service: http://localhost:8002 ✓"
echo -e "  Creation Service:   Kafka Consumer Mode ✓"
echo -e "  API Server:         http://localhost:8080 ✓"
echo -e "${BLUE}═══════════════════════════════════════${NC}"
echo ""

# Show logs locations
echo -e "${YELLOW}Service Logs:${NC}"
echo -e "  API Server:         tail -f api_server.log"
echo -e "  Generation Service: tail -f generation_service.log"
echo -e "  Creation Service:   tail -f creation_service.log"
echo -e "  ChromaDB:           docker logs brainbot-chroma"
echo -e "  Kafka:              docker logs brainbot-kafka"
echo ""

echo -e "${GREEN}Architecture:${NC}"
echo -e "  RSS Feed → API Server → Generation Service → Kafka → Creation Service → YouTube"
echo ""

# Export environment variables for the demo
export API_URL=http://localhost:8080
export WEBHOOK_PORT=9999
export GENERATION_SERVICE_URL=http://localhost:8002

# Run the demo (this will block until demo exits)
echo -e "${GREEN}Starting demo...${NC}"
echo -e "${YELLOW}Press 'd' to start the demo workflow${NC}"
echo -e "${YELLOW}Press 'q' or Ctrl+C to quit${NC}"
echo ""

go run cmd/demo/main.go

# Cleanup will be called automatically by the trap
