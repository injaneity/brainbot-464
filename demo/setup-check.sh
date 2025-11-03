#!/bin/bash

# Colors for output
RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
BLUE='\033[0;34m'
NC='\033[0m' # No Color

echo -e "${BLUE}🤖 BrainBot Demo Setup${NC}"
echo ""

# Check if ChromaDB is running
echo -n "Checking ChromaDB (port 8000)... "
if curl -s http://localhost:8000/api/v1/heartbeat > /dev/null 2>&1; then
    echo -e "${GREEN}✓ Running${NC}"
else
    echo -e "${YELLOW}✗ Not running${NC}"
    echo -e "${YELLOW}Please start ChromaDB:${NC}"
    echo "  docker run -p 8000:8000 chromadb/chroma"
    echo ""
fi

# Check if generation service is running
echo -n "Checking Generation Service (port 8001)... "
if curl -s http://localhost:8001/health > /dev/null 2>&1; then
    echo -e "${GREEN}✓ Running${NC}"
else
    echo -e "${YELLOW}✗ Not running${NC}"
    echo -e "${YELLOW}Please start the generation service:${NC}"
    echo "  cd generation_service"
    echo "  pip install -r requirements.txt"
    echo "  python -m app.main"
    echo ""
fi

# Check if webhook port is available
echo -n "Checking webhook port (9999)... "
if lsof -Pi :9999 -sTCP:LISTEN -t >/dev/null 2>&1; then
    echo -e "${YELLOW}✗ Port in use${NC}"
    echo -e "${YELLOW}Port 9999 is already in use. Please free it or set WEBHOOK_PORT env variable.${NC}"
    echo ""
else
    echo -e "${GREEN}✓ Available${NC}"
fi

echo ""
echo -e "${BLUE}Ready to run the demo!${NC}"
echo ""
echo "To build and run:"
echo "  go build -o demo ./cmd/demo"
echo "  ./demo"
echo ""
echo "Or run directly:"
echo "  go run ./cmd/demo/main.go"
echo ""
