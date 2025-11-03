#!/bin/bash

# Stop all demo services

echo "Stopping BrainBot Demo Services..."

# Stop Generation Service
if [ -f generation_service.pid ]; then
    PID=$(cat generation_service.pid)
    if kill -0 $PID 2>/dev/null; then
        kill $PID
        echo "✓ Stopped Generation Service (PID: $PID)"
    fi
    rm generation_service.pid
fi

# Stop ChromaDB
if docker ps | grep -q brainbot-chroma; then
    docker stop brainbot-chroma >/dev/null 2>&1
    docker rm brainbot-chroma >/dev/null 2>&1
    echo "✓ Stopped ChromaDB"
fi

echo "All services stopped."
