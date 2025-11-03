#!/bin/bash

# Test script to simulate the generation service webhook response
# Useful for testing the demo without running the full generation service

WEBHOOK_URL="${1:-http://localhost:9999/webhook}"
UUID="${2:-test-uuid-12345}"

echo "Sending test webhook to: $WEBHOOK_URL"
echo "UUID: $UUID"

curl -X POST "$WEBHOOK_URL" \
  -H "Content-Type: application/json" \
  -d "{
    \"uuid\": \"$UUID\",
    \"voiceover\": \"This is a test voiceover generated for demonstration purposes. In a real scenario, this would contain the full generated script based on the article content. The generation service processes articles through AI models to create engaging narration suitable for video content.\",
    \"subtitle_timestamps\": [
      {\"start\": 0.0, \"end\": 2.5, \"text\": \"This is a test voiceover\"},
      {\"start\": 2.5, \"end\": 5.0, \"text\": \"generated for demonstration\"},
      {\"start\": 5.0, \"end\": 8.0, \"text\": \"purposes.\"}
    ],
    \"resource_timestamps\": {
      \"resource1\": {\"start\": 1.0, \"end\": 3.0},
      \"resource2\": {\"start\": 5.5, \"end\": 7.5}
    },
    \"status\": \"success\",
    \"timings\": {
      \"total\": 45.2,
      \"generation\": 30.5,
      \"audio\": 12.3,
      \"subtitles\": 2.4
    }
  }"

echo -e "\n\nTest webhook sent!"
