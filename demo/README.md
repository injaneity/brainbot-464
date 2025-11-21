# BrainBot Integration Demo

An interactive terminal UI demo that showcases the complete integration flow between the RSS feed/deduplication service and the generation service.

## Overview

This demo demonstrates the end-to-end pipeline:

1. **Fetch RSS Feed** - Pulls the latest articles from configured RSS feeds
2. **Extract Content** - Extracts full content from article URLs
3. **Deduplicate** - Checks articles against ChromaDB to identify duplicates
4. **Send for Generation** - Sends new articles to the generation service
5. **Webhook Reception** - Receives and displays results from the generation service

## Prerequisites

Before running the demo, ensure you have:

1. **ChromaDB running** on `localhost:8000`
   ```bash
   # If using Docker
   docker run -p 8000:8000 chromadb/chroma
   ```

2. **Generation service running** on `localhost:8001`
   ```bash
   cd generation_service
   pip install -r requirements.txt
   python -m app.main
   ```

3. **Environment variables configured** (optional)
   Create a `.env` file in the root directory:
   ```env
   # RSS Feed Configuration
   RSS_FEED_PRESET=st          # or any other feed preset

   # ChromaDB Configuration
   CHROMA_HOST=localhost
   CHROMA_PORT=8000
   CHROMA_COLLECTION=brainbot_articles

   # Generation Service Configuration
   GENERATION_SERVICE_URL=http://localhost:8001

   # Webhook Configuration
   WEBHOOK_PORT=9999
   ```

## Running the Demo

From the project root directory:

```bash
# Option 1: Using Make (recommended)
cd demo
make demo

# Option 2: Build and run
go build -o demo ./cmd/demo
./demo

# Option 3: Run directly
go run ./cmd/demo/main.go
```

### Available Make Commands

```bash
make help            # Show all available commands
make demo            # Run the interactive demo
make build           # Build the demo binary
make setup-check     # Check if services are running
make test-webhook    # Send a test webhook
make start-services  # Start ChromaDB and Generation Service
make stop-services   # Stop all services
make clean           # Clean build artifacts
```

## What Happens

1. **Initialization**: A temporary webhook server starts on port 9999 (configurable)

2. **RSS Fetch**: The demo fetches the latest articles from your configured RSS feed

3. **Content Extraction**: Full content is extracted from each article URL

4. **Deduplication**: Each article is checked against ChromaDB:
   - New articles are added to the database
   - Duplicates are detected and skipped

5. **Generation Request**: New articles are sent to the generation service with:
   - A unique UUID for tracking
   - Article content text
   - Webhook URL for receiving results

6. **Waiting**: The demo displays a waiting state with the UUID

7. **Webhook Reception**: When the generation service completes processing:
   - It sends results to the webhook
   - The demo displays the results including:
     - Voiceover preview
     - Subtitle segment count
     - Processing timings
     - Any errors

## Interactive Controls

- Press `q` or `Ctrl+C` to quit at any time
- The webhook server will automatically shut down on exit

## Architecture

```
┌─────────────────┐
│   Demo TUI      │
│  (cmd/demo)     │
└────┬────────────┘
     │
     ├─► RSS Feed Fetcher
     │   (rssfeeds package)
     │
     ├─► Content Extractor
     │   (rssfeeds package)
     │
     ├─► Deduplicator
     │   (deduplication package)
     │   └─► ChromaDB
     │
     ├─► Generation Service
     │   (HTTP POST /generate)
     │
     └─► Webhook Server
         (listens on :9999)
         ◄─── Generation Service
              (HTTP POST /webhook)
```

## Troubleshooting

### ChromaDB Connection Issues
```
Error: failed to initialize deduplicator: connection refused
```
- Ensure ChromaDB is running on the configured host and port
- Check `CHROMA_HOST` and `CHROMA_PORT` environment variables

### Generation Service Issues
```
Error: generation service returned 500
```
- Ensure the generation service is running
- Check `GENERATION_SERVICE_URL` environment variable
- Review generation service logs

### Webhook Not Received
- Ensure the webhook port is not in use
- Check firewall settings
- Verify the generation service can reach `localhost:9999`
- If running in containers, ensure proper network configuration

### No New Articles
```
Error: no new articles to send for generation
```
- This means all fetched articles were duplicates
- Try a different RSS feed or clear ChromaDB to reset

## Testing

### Test Webhook Without Generation Service

You can test the demo's webhook functionality without running the full generation service:

```bash
# Start the demo (it will wait at the "waiting" state)
go run ./cmd/demo/main.go

# In another terminal, send a test webhook
./demo/test-webhook.sh

# Or with custom UUID
./demo/test-webhook.sh http://localhost:9999/webhook your-custom-uuid
```

### Manual Testing Steps

1. **Test RSS Fetching Only**: Check if articles are being fetched
   ```bash
   # This should show articles being fetched
   # You can quit (q) before it sends to generation service
   ```

2. **Test Deduplication**: Run the demo multiple times
   - First run: All articles should be "new"
   - Second run: All articles should be "duplicates"

3. **Test Full Flow**: 
   - Ensure generation service is running
   - Run the demo
   - Watch for the webhook response

## Development

The demo uses:
- [Bubble Tea](https://github.com/charmbracelet/bubbletea) - Terminal UI framework
- [Lip Gloss](https://github.com/charmbracelet/lipgloss) - Terminal styling
- Native Go HTTP server for webhook handling

## Configuration Options

| Environment Variable | Default | Description |
|---------------------|---------|-------------|
| `RSS_FEED_PRESET` | `st` | RSS feed to fetch from |
| `CHROMA_HOST` | `localhost` | ChromaDB host |
| `CHROMA_PORT` | `8000` | ChromaDB port |
| `CHROMA_COLLECTION` | `brainbot_articles` | ChromaDB collection name |
| `GENERATION_SERVICE_URL` | `http://localhost:8001` | Generation service URL |
| `WEBHOOK_PORT` | `9999` | Port for webhook server |

## Example Output

```
🤖 BrainBot Integration Demo

✅ COMPLETE

📊 Articles fetched: 10
   New: 3 | Duplicates: 7

🌐 Webhook listening on: http://localhost:9999/webhook

📝 Recent Activity:
   [14:32:15] Fetched 10 articles
   [14:32:18] Found 3 new articles
   [14:32:18] Generation request sent with UUID: 123e4567-e89b-12d3-a456-426614174000
   [14:35:42] Webhook received from generation service!

┌─────────────────────────────────────────────────────────────┐
│                                                             │
│  Generation Service Result                                  │
│                                                             │
│  Status: success                                            │
│  UUID: 123e4567-e89b-12d3-a456-426614174000                │
│                                                             │
│  Voiceover Preview:                                         │
│  Today's news covers breakthrough developments in AI...     │
│                                                             │
│  Subtitle Segments: 42                                      │
│                                                             │
│  Timings:                                                   │
│    total: 204.50s                                          │
│    generation: 120.30s                                     │
│    audio: 75.20s                                           │
│    subtitles: 9.00s                                        │
│                                                             │
└─────────────────────────────────────────────────────────────┘

Press q or Ctrl+C to exit
```
