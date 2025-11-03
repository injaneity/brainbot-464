#!/bin/bash

# Quick run script for BrainBot Terminal UI Demo

set -e

# Colors
GREEN='\033[0;32m'
BLUE='\033[0;34m'
YELLOW='\033[1;33m'
RED='\033[0;31m'
NC='\033[0m'

# Track if we started the generation service
GEN_SERVICE_STARTED=false
GEN_SERVICE_PID=""

# Cleanup function
cleanup() {
    if [ "$GEN_SERVICE_STARTED" = true ] && [ -n "$GEN_SERVICE_PID" ]; then
        echo -e "\n${YELLOW}Stopping Generation Service (PID: $GEN_SERVICE_PID)...${NC}"
        kill $GEN_SERVICE_PID 2>/dev/null || true
        echo -e "${GREEN}Generation Service stopped${NC}"
    fi
}

# Set trap to call cleanup on exit
trap cleanup EXIT INT TERM

echo -e "${BLUE}🤖 BrainBot Demo Launcher${NC}\n"

# Check if services are running
echo -e "${YELLOW}Checking prerequisites...${NC}"

MISSING_SERVICES=0

# Check ChromaDB
if ! curl -s http://localhost:8000/api/v1/heartbeat > /dev/null 2>&1; then
    echo -e "${RED}✗ ChromaDB is not running on port 8000${NC}"
    echo -e "  Start with: ${BLUE}docker run -d -p 8000:8000 chromadb/chroma${NC}"
    MISSING_SERVICES=1
else
    echo -e "${GREEN}✓ ChromaDB is running${NC}"
fi

# Check Generation Service
if ! curl -s http://localhost:8002/health > /dev/null 2>&1; then
    echo -e "${YELLOW}⚠ Generation Service is not running on port 8002${NC}"
    echo -e "${BLUE}Starting Generation Service...${NC}"
    
    # Check if Python virtual environment exists
    if [ ! -d "generation_service/venv" ]; then
        echo -e "${YELLOW}  Creating Python virtual environment...${NC}"
        cd generation_service
        python3 -m venv venv
        source venv/bin/activate
        pip install -q -r requirements.txt
        cd ..
    fi
    
    # Start the generation service in the background on port 8002
    cd generation_service
    source venv/bin/activate
    PORT=8002 nohup python -m app.main > /tmp/generation_service.log 2>&1 &
    GEN_SERVICE_PID=$!
    GEN_SERVICE_STARTED=true
    cd ..
    
    # Wait for service to start
    echo -e "${YELLOW}  Waiting for Generation Service to start...${NC}"
    for _i in {1..30}; do
        if curl -s http://localhost:8002/health > /dev/null 2>&1; then
            echo -e "${GREEN}✓ Generation Service is running (PID: $GEN_SERVICE_PID)${NC}"
            break
        fi
        sleep 1
    done
    
    if ! curl -s http://localhost:8002/health > /dev/null 2>&1; then
        echo -e "${RED}✗ Failed to start Generation Service${NC}"
        echo -e "  Check logs: tail -f /tmp/generation_service.log"
        MISSING_SERVICES=1
    fi
else
    echo -e "${GREEN}✓ Generation Service is running${NC}"
fi

# Check webhook port
if lsof -Pi :9999 -sTCP:LISTEN -t >/dev/null 2>&1; then
    echo -e "${RED}✗ Port 9999 is already in use${NC}"
    echo -e "  Free the port or set WEBHOOK_PORT environment variable"
    MISSING_SERVICES=1
else
    echo -e "${GREEN}✓ Webhook port 9999 is available${NC}"
fi

echo ""

if [ $MISSING_SERVICES -eq 1 ]; then
    echo -e "${RED}Some required services are not running.${NC}"
    echo -e "${YELLOW}Do you want to continue anyway? (y/N)${NC}"
    read -r response
    if [[ ! "$response" =~ ^([yY][eE][sS]|[yY])$ ]]; then
        echo "Exiting..."
        exit 1
    fi
fi

# Load .env if it exists
if [ -f .env ]; then
    echo -e "${GREEN}Loading .env configuration...${NC}"
    export $(cat .env | grep -v '^#' | xargs)
fi

# Run the demo
echo -e "\n${GREEN}Starting demo...${NC}\n"
echo -e "${BLUE}━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━${NC}\n"

# Rebuild to ensure latest code
go build -o demo_bin ./cmd/demo 2>/dev/null

./demo_bin

echo -e "\n${BLUE}━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━${NC}"
echo -e "${GREEN}Demo finished!${NC}"
