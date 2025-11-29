# API Server Reference

Base URL: `http://localhost:8080`

All endpoints are available through the main API server on port 8080 (configurable via `PORT` environment variable).

## Health Check

### GET /api/health

Check if the API server is running.

**Response:**

```json
{
  "status": "ok"
}
```

---

## Articles

### POST /api/articles

Utility endpoint to accept an article and return normalized identifiers.

**Request:**

```json
{
  "id": "string",
  "title": "string",
  "url": "string",
  "summary": "string",
  "full_content": "string",
  "published_at": "2025-01-01T00:00:00Z"
}
```

**Response:**

```json
{
  "id": "normalized-id",
  "title": "Article Title",
  "url": "https://example.com/article"
}
```

---

## ChromaDB Access

Direct access to ChromaDB collection for inspection and debugging.

### GET /api/chroma/articles

List documents stored in the ChromaDB collection.

**Query Parameters:**

- `limit` (optional): Maximum number of documents to return (default: 10)
- `offset` (optional): Number of documents to skip (default: 0)

**Example:**

```bash
curl "http://localhost:8080/api/chroma/articles?limit=20&offset=0"
```

**Response:**

```json
{
  "documents": [
    {
      "id": "article-123",
      "content": "Article text...",
      "metadata": {
        "title": "Article Title",
        "url": "https://example.com",
        "added_at": "2025-01-01T00:00:00Z"
      }
    }
  ],
  "count": 20,
  "offset": 0
}
```

---

## Deduplication Service

Vector-based duplicate detection and article management.

### POST /api/deduplication/check

Check if an article is a duplicate without adding it to the database.

**Request:**

```json
{
  "article": {
    "id": "string",
    "title": "string",
    "url": "string",
    "full_content_text": "string",
    "published_at": "2025-01-01T00:00:00Z",
    "fetched_at": "2025-01-01T00:00:00Z"
  }
}
```

**Response:**

```json
{
  "is_duplicate": false,
  "matching_id": "",
  "similarity_score": 0.0,
  "checked_at": "2025-01-01T00:00:00Z"
}
```

### POST /api/deduplication/add

Add an article to the deduplication database without checking for duplicates.

**Request:** Same as `/check`

**Response:**

```json
{
  "status": "added",
  "article_id": "abc123"
}
```

### POST /api/deduplication/process

**Recommended endpoint** - Check for duplicates and add to database if new.

**Request:** Same as `/check`

**Response (New):**

```json
{
  "status": "new",
  "deduplication_result": {
    "is_duplicate": false,
    "similarity_score": 0.0,
    "checked_at": "2025-01-01T00:00:00Z"
  }
}
```

**Response (Duplicate):**

```json
{
  "status": "duplicate",
  "deduplication_result": {
    "is_duplicate": true,
    "matching_id": "abc123",
    "similarity_score": 0.97,
    "checked_at": "2025-01-01T00:00:00Z"
  }
}
```

### DELETE /api/deduplication/clear

Clear all documents from the ChromaDB collection.

**Response:**

```json
{
  "status": "cleared"
}
```

### GET /api/deduplication/count

Get the number of documents in the deduplication database.

**Response:**

```json
{
  "count": 42
}
```

---

## RSS Orchestrator

Trigger background orchestration runs for RSS feed processing.

### POST /api/rss/refresh

Trigger a background orchestrator run that:

1. Fetches RSS feed articles
2. Extracts full content
3. Deduplicates against ChromaDB
4. (Optional) Uploads to S3

**Request Body:** None

**Response:**

```json
{
  "status": "triggered",
  "message": "RSS refresh started in background"
}
```

**Background Process:**
The orchestrator runs asynchronously. Check logs for progress and results.

---

## Configuration

### Environment Variables

```bash
# Server
PORT=8080

# ChromaDB
CHROMA_HOST=localhost
CHROMA_PORT=8000
CHROMA_COLLECTION=brainbot_articles

# RSS
RSS_FEED_PRESET=st  # or cna, hn, tr

# S3 (optional)
S3_BUCKET=your-bucket
S3_REGION=us-east-1
S3_PREFIX=articles/

# Embeddings (choose one)
COHERE_API_KEY=your-key
# OR
OPENAI_API_KEY=your-key
OPENAI_ORG_ID=your-org  # optional
```

---

## Error Responses

All endpoints return standard error responses:

```json
{
  "error": "error message here"
}
```

**Status Codes:**

- `200` - Success
- `400` - Bad Request (invalid input)
- `500` - Internal Server Error

---

## Usage Examples

### cURL

```bash
# Health check
curl http://localhost:8080/api/health

# Get article count
curl http://localhost:8080/api/deduplication/count

# Process an article
curl -X POST http://localhost:8080/api/deduplication/process \
  -H "Content-Type: application/json" \
  -d '{
    "article": {
      "id": "test-1",
      "title": "Test Article",
      "url": "https://example.com",
      "full_content_text": "Article content here...",
      "published_at": "2025-01-01T00:00:00Z",
      "fetched_at": "2025-01-01T00:00:00Z"
    }
  }'

# Trigger RSS refresh
curl -X POST http://localhost:8080/api/rss/refresh

# List ChromaDB articles
curl "http://localhost:8080/api/chroma/articles?limit=5"

# Clear deduplication cache
curl -X DELETE http://localhost:8080/api/deduplication/clear
```

### Go Client

```go
import "brainbot/app"

client := app.NewClient("http://localhost:8080")

// Process article
result, err := client.ProcessArticle(ctx, article)

// Get count
count, err := client.GetCount(ctx)

// Clear cache
err = client.ClearCache(ctx)

// Trigger RSS refresh
err = client.TriggerRSSRefresh(ctx)
```

---

## Architecture

The API server acts as a unified gateway to all services:

```
Client → API Server (port 8080)
            ├─► Health Check
            ├─► Articles Management
            ├─► ChromaDB Access
            ├─► Deduplication Service → ChromaDB (port 8000)
            └─► RSS Orchestrator
```

All deduplication operations are handled through REST endpoints, making the service fully isolated and independently scalable.
