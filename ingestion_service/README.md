# Ingestion Service

RSS feed ingestion and content deduplication service.

## Overview

The ingestion service handles:
- Fetching articles from RSS/Atom feeds
- Extracting full article content
- Deduplicating content using vector embeddings
- Storing articles in ChromaDB for future comparison

## Setup

### Prerequisites

- Go 1.24+
- ChromaDB running (Docker or local)
- Cohere or OpenAI API key for embeddings

### Environment Variables

```bash
# ChromaDB connection
CHROMA_HOST=localhost
CHROMA_PORT=8000
CHROMA_COLLECTION=brainbot_articles

# Redis (for Bloom Filter)
REDIS_ADDR=localhost:6379
REDIS_PASSWORD=
REDIS_DB=0

# S3 Storage
S3_BUCKET=your-bucket-name
S3_REGION=us-east-1
S3_PREFIX=articles/
AWS_ACCESS_KEY_ID=your_access_key
AWS_SECRET_ACCESS_KEY=your_secret_key

# Embeddings API (choose one)
COHERE_API_KEY=your_cohere_key
# OR
OPENAI_API_KEY=your_openai_key
OPENAI_ORG_ID=your_org  # optional

# RSS Feed preset
RSS_FEED_PRESET=st  # Options: cna, st, hn, tr

# Server
PORT=8080
```

### Build & Run

```bash
# Build
go build -o ingestion-server main.go

# Run
./ingestion-server

# Or run directly
go run main.go
```

## API Endpoints

### Health Check
```bash
GET /api/health
```

### Deduplication

**Check if content is duplicate:**
```bash
POST /api/deduplication/check
Content-Type: application/json

{
  "text": "Article content to check..."
}
```

**Add article to database:**
```bash
POST /api/deduplication/add
Content-Type: application/json

{
  "article": {
    "id": "unique-id",
    "title": "Article Title",
    "content": "Full article content..."
  }
}
```

**Process article (check + add if new):**
```bash
POST /api/deduplication/process
Content-Type: application/json

{
  "article": {
    "id": "unique-id",
    "title": "Article Title",
    "content": "Full article content...",
    "url": "https://example.com/article"
  }
}
```

**Get article count:**
```bash
GET /api/deduplication/count
```

**Clear database:**
```bash
DELETE /api/deduplication/clear
```

## Configuration

### RSS Feed Presets

Set `RSS_FEED_PRESET` environment variable:

- `cna` - Channel NewsAsia
- `st` - Straits Times
- `hn` - Hacker News
- `tr` - MIT Technology Review

### Deduplication Settings

Edit `deduplication/deduplicator.go`:

```go
const (
    SimilarityThreshold = 0.85  // 85% similarity threshold
    MaxSearchResults    = 5     // Number of similar articles to check
)
```

## Architecture

```
┌─────────────┐
│  RSS Feed   │
└──────┬──────┘
       ↓
┌─────────────┐
│  Fetcher    │  Fetch & parse RSS/Atom
└──────┬──────┘
       ↓
┌─────────────┐
│  Extractor  │  Extract full content
└──────┬──────┘
       ↓
┌─────────────┐      ┌─────────────┐
│Deduplicator │ ───→ │ Redis Bloom │  Exact match check
└──────┬──────┘      └─────────────┘
       │
       │ (If not exact match)
       ↓
┌─────────────┐
│  ChromaDB   │  Vector similarity check
└──────┬──────┘
       │
       │ (If new or similar)
       ↓
┌─────────────┐
│     S3      │  Store/Append content
└─────────────┘
```

## Development

### Project Structure

```
ingestion_service/
├── api/
│   ├── server.go                  # Router setup
│   ├── healthcontroller.go        # Health endpoint
│   └── deduplicationcontroller.go # Deduplication endpoints
├── deduplication/
│   ├── deduplicator.go           # Core logic
│   ├── embeddings.go             # Cohere/OpenAI clients
│   └── chroma.go                 # ChromaDB client
├── rssfeeds/
│   ├── fetcher.go                # RSS parsing
│   ├── extractor.go              # Content extraction
│   └── config.go                 # Feed presets
├── types/
│   └── article.go                # Article data model
└── main.go                       # Entry point
```

### Testing

```bash
# Run tests
go test ./...

# Test with specific feed
export RSS_FEED_PRESET=hn
go run main.go
```

## Docker

### Build

```bash
docker build -t brainbot-ingestion .
```

### Run

```bash
docker run -p 8080:8080 \
  -e CHROMA_HOST=chromadb \
  -e COHERE_API_KEY=your_key \
  brainbot-ingestion
```

## Troubleshooting

**ChromaDB connection errors:**
- Verify ChromaDB is running: `curl http://localhost:8000/api/v1/heartbeat`
- Check `CHROMA_HOST` and `CHROMA_PORT` environment variables

**Embedding errors:**
- Verify API key is set: `echo $COHERE_API_KEY` or `echo $OPENAI_API_KEY`
- Check API key has proper permissions

**RSS feed errors:**
- Some feeds may require user agent headers
- Check feed URL is accessible: `curl -I <feed-url>`
