#!/bin/bash

# Quick start script for the BrainBot demo
# This script starts all required services in the background

set -e

# Colors for output
GREEN='\033[0;32m'
BLUE='\033[0;34m'
YELLOW='\033[1;33m'
NC='\033[0m' # No Color

echo -e "${BLUE}🤖 Starting BrainBot Demo Services${NC}"
echo ""

# Function to check if a port is in use
check_port() {
    lsof -Pi :$1 -sTCP:LISTEN -t >/dev/null 2>&1
}

# Start ChromaDB if not running
if ! check_port 8000; then
    echo -e "${YELLOW}Starting ChromaDB...${NC}"
    docker run -d -p 8000:8000 --name brainbot-chroma chromadb/chroma
    echo -e "${GREEN}ChromaDB started on port 8000${NC}"
    sleep 2
else
    echo -e "${GREEN}ChromaDB already running on port 8000${NC}"
fi

# Start Generation Service if not running
if ! check_port 8001; then
    echo -e "${YELLOW}Starting Generation Service...${NC}"
    cd generation_service
    # Check if virtual environment exists
    if [ ! -d "venv" ]; then
        echo "Creating virtual environment..."
        python3 -m venv venv
    fi
    source venv/bin/activate
    pip install -q -r requirements.txt
    nohup python -m app.main > ../generation_service.log 2>&1 &
    echo $! > ../generation_service.pid
    cd ..
    echo -e "${GREEN}Generation Service started on port 8001${NC}"
    echo "  Logs: generation_service.log"
    echo "  PID: $(cat generation_service.pid)"
    sleep 2
else
    echo -e "${GREEN}Generation Service already running on port 8001${NC}"
fi

echo ""
echo -e "${GREEN}All services ready!${NC}"
echo ""
echo "Now run the demo:"
echo "  go run ./cmd/demo/main.go"
echo ""
echo "To stop services:"
echo "  docker stop brainbot-chroma && docker rm brainbot-chroma"
echo "  kill \$(cat generation_service.pid) && rm generation_service.pid"
