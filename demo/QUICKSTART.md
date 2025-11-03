# Quick Start Guide - BrainBot Demo

## 1-Minute Setup

```bash
# 1. Check prerequisites
cd demo
make setup-check

# 2. Start services (if needed)
make start-services

# 3. Run the demo
make demo
```

## What You'll See

```
🤖 BrainBot Integration Demo

⏳ Fetching RSS feed...

📊 Articles fetched: 10
   New: 3 | Duplicates: 7

🌐 Webhook listening on: http://localhost:9999/webhook

📝 Recent Activity:
   [14:32:15] Fetched 10 articles
   [14:32:18] Found 3 new articles
   [14:32:18] Generation request sent with UUID: abc123...
   [14:35:42] Webhook received from generation service!
```

## Common Commands

```bash
# Check services
make setup-check

# Start/stop services
make start-services
make stop-services

# Run demo
make demo

# Test webhook (while demo is running)
make test-webhook

# Clean up
make clean
```

## Troubleshooting

### Services Not Running?
```bash
# Start them automatically
make start-services

# Or manually:
docker run -p 8000:8000 chromadb/chroma
cd generation_service && python -m app.main
```

### Port Already in Use?
Set alternative ports in `.env`:
```
WEBHOOK_PORT=10000
```

### No New Articles?
All articles were duplicates. Options:
1. Try different RSS feed: `RSS_FEED_PRESET=tc`
2. Clear ChromaDB: `docker restart brainbot-chroma`

## Architecture Flow

```
┌─────────────┐
│   Demo UI   │ Press q to quit
└──────┬──────┘
       │
       ├─► 1. Fetch RSS Feed
       │      ↓
       ├─► 2. Extract Content
       │      ↓
       ├─► 3. Deduplicate (ChromaDB)
       │      ↓
       ├─► 4. Send to Generation Service
       │      │
       │      │ Processing...
       │      │
       └─► 5. ◄─── Webhook Result
              (Voiceover, Subtitles, Timings)
```

## Environment Variables

Quick reference of key settings:

| Variable | Default | Purpose |
|----------|---------|---------|
| `WEBHOOK_PORT` | 9999 | Webhook listener port |
| `CHROMA_HOST` | localhost | ChromaDB location |
| `GENERATION_SERVICE_URL` | http://localhost:8001 | Generation service |
| `RSS_FEED_PRESET` | st | Which RSS feed to use |

## Next Steps

- See [README.md](README.md) for full documentation
- Check [.env.example](.env.example) for all config options
- Review the code in `cmd/demo/main.go`
