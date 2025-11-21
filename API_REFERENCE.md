# Deduplication Service API Reference

## Base URL

```
http://localhost:8080/api/deduplication
```

## Endpoints

### 1. Check for Duplicate

Check if an article is a duplicate without adding it to the database.

**Endpoint:** `POST /check`

**Request Body:**

```json
{
  "article": {
    "id": "string",
    "title": "string",
    "url": "string",
    "summary": "string",
    "full_content": "string",
    "full_content_text": "string",
    "published_at": "2025-01-01T00:00:00Z",
    "fetched_at": "2025-01-01T00:00:00Z"
  }
}
```

**Response (200 OK):**

```json
{
  "is_duplicate": false,
  "matching_id": "",
  "similarity_score": 0.0,
  "checked_at": "2025-01-01T00:00:00Z"
}
```

**Response (Duplicate Found):**

```json
{
  "is_duplicate": true,
  "matching_id": "abc123",
  "similarity_score": 0.96,
  "checked_at": "2025-01-01T00:00:00Z"
}
```

**Error Response (400):**

```json
{
  "error": "invalid request body"
}
```

**Error Response (500):**

```json
{
  "error": "failed to check duplicates: connection error"
}
```

---

### 2. Add Article

Add an article to the deduplication database without checking for duplicates.

**Endpoint:** `POST /add`

**Request Body:**

```json
{
  "article": {
    "id": "string",
    "title": "string",
    "url": "string",
    "summary": "string",
    "full_content_text": "string",
    "published_at": "2025-01-01T00:00:00Z",
    "fetched_at": "2025-01-01T00:00:00Z",
    "author": "string",
    "categories": ["string"]
  }
}
```

**Response (200 OK):**

```json
{
  "status": "added",
  "article_id": "abc123"
}
```

**Error Response (400):**

```json
{
  "error": "invalid request body"
}
```

**Error Response (500):**

```json
{
  "error": "failed to add article: no content to embed"
}
```

---

### 3. Process Article

Check for duplicates and add to database if new (recommended endpoint).

**Endpoint:** `POST /process`

**Request Body:**

```json
{
  "article": {
    "id": "string",
    "title": "string",
    "url": "string",
    "summary": "string",
    "full_content_text": "string",
    "published_at": "2025-01-01T00:00:00Z",
    "fetched_at": "2025-01-01T00:00:00Z"
  }
}
```

**Response (200 OK - New Article):**

```json
{
  "status": "new",
  "deduplication_result": {
    "is_duplicate": false,
    "matching_id": "",
    "similarity_score": 0.0,
    "checked_at": "2025-01-01T00:00:00Z"
  }
}
```

**Response (200 OK - Duplicate):**

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

**Response (500 - Error):**

```json
{
  "status": "error",
  "error": "failed to process article"
}
```

---

### 4. Clear Cache

Clear all documents from the ChromaDB collection.

**Endpoint:** `DELETE /clear`

**Request Body:** None

**Response (200 OK):**

```json
{
  "status": "cleared"
}
```

**Error Response (500):**

```json
{
  "error": "failed to connect to ChromaDB"
}
```

---

### 5. Get Count

Get the number of documents in the deduplication database.

**Endpoint:** `GET /count`

**Request Body:** None

**Response (200 OK):**

```json
{
  "count": 42
}
```

**Error Response (500):**

```json
{
  "error": "failed to get count"
}
```

---

## Article Object Schema

```typescript
interface Article {
  id: string; // Unique identifier (required)
  title: string; // Article title (required)
  url: string; // Source URL (required)
  summary: string; // Brief summary
  excerpt: string; // Short excerpt
  full_content: string; // Full HTML content
  full_content_text: string; // Full text content (preferred for embeddings)
  published_at: string; // ISO 8601 timestamp
  fetched_at: string; // ISO 8601 timestamp
  author: string; // Article author
  categories: string[]; // Categories/tags
  images: Image[]; // Associated images
  extraction_error: string; // Error message if extraction failed
}

interface Image {
  url: string;
  alt: string;
  caption: string;
}
```

## Deduplication Logic

### Similarity Threshold

- Default: `0.95` (95% similarity)
- Configurable via environment: `SIMILARITY_THRESHOLD`

### Embedding Model

- Default: `sentence-transformers/all-MiniLM-L6-v2`
- Configurable via environment: `EMBEDDING_MODEL`

### Content Priority for Embeddings

1. `full_content_text` (preferred)
2. `full_content`
3. `summary`
4. `title`

### TTL (Time To Live)

- Default: 24 hours
- Articles older than TTL are automatically removed during duplicate checks

## Error Codes

| Status Code | Meaning                                               |
| ----------- | ----------------------------------------------------- |
| 200         | Success                                               |
| 400         | Bad Request (invalid JSON or missing required fields) |
| 500         | Internal Server Error (database or service error)     |

## Rate Limiting

Currently no rate limiting is implemented. Consider adding in production:

- Requests per minute per IP
- Burst limits
- Quota management

## Usage Examples

### Using cURL

```bash
# Check if article is duplicate
curl -X POST http://localhost:8080/api/deduplication/check \
  -H "Content-Type: application/json" \
  -d @article.json

# Process article (recommended)
curl -X POST http://localhost:8080/api/deduplication/process \
  -H "Content-Type: application/json" \
  -d '{
    "article": {
      "id": "test-article-1",
      "title": "Sample Article",
      "url": "https://example.com/article",
      "full_content_text": "This is the full content of the article...",
      "published_at": "2025-01-01T00:00:00Z",
      "fetched_at": "2025-01-01T00:00:00Z"
    }
  }'

# Get count
curl http://localhost:8080/api/deduplication/count

# Clear cache
curl -X DELETE http://localhost:8080/api/deduplication/clear
```

### Using Go Client

```go
import (
    "brainbot/app"
    "context"
)

// Create client
client := app.NewClient("http://localhost:8080")

// Process article
result, err := client.ProcessArticle(context.Background(), article)
if err != nil {
    log.Fatal(err)
}

if result.Status == "duplicate" {
    log.Printf("Duplicate found: %s (%.2f%% similar)",
        result.DeduplicationResult.MatchingID,
        result.DeduplicationResult.SimilarityScore*100)
} else {
    log.Println("New article added")
}

// Get count
count, err := client.GetCount(context.Background())
if err != nil {
    log.Fatal(err)
}
log.Printf("Total articles: %d", count)
```

### Using JavaScript/TypeScript

```javascript
// Check for duplicate
const response = await fetch(
  "http://localhost:8080/api/deduplication/process",
  {
    method: "POST",
    headers: {
      "Content-Type": "application/json",
    },
    body: JSON.stringify({
      article: {
        id: "article-123",
        title: "My Article",
        url: "https://example.com",
        full_content_text: "Article content...",
        published_at: new Date().toISOString(),
        fetched_at: new Date().toISOString(),
      },
    }),
  }
);

const result = await response.json();
console.log("Status:", result.status);
if (result.status === "duplicate") {
  console.log("Similarity:", result.deduplication_result.similarity_score);
}
```

## Configuration

### Environment Variables

```bash
# ChromaDB Configuration
CHROMA_HOST=localhost
CHROMA_PORT=8000
CHROMA_COLLECTION=brainbot_articles

# Deduplication Settings (future)
SIMILARITY_THRESHOLD=0.95
MAX_SEARCH_RESULTS=5
EMBEDDING_MODEL=sentence-transformers/all-MiniLM-L6-v2
```

## Security Considerations

**Current Status:** No authentication/authorization

**Recommended for Production:**

1. Add API key authentication
2. Implement rate limiting
3. Add request validation
4. Enable CORS with whitelist
5. Add audit logging
6. Encrypt sensitive data
7. Use HTTPS only

## Monitoring & Health

Check overall API health:

```bash
curl http://localhost:8080/api/health
```

Response:

```json
{
  "status": "ok"
}
```

## Support & Troubleshooting

### Common Issues

**"Failed to initialize deduplicator"**

- Check ChromaDB is running: `curl http://localhost:8000/api/v1/heartbeat`
- Verify CHROMA_HOST and CHROMA_PORT environment variables

**"No content to embed"**

- Ensure article has at least one of: full_content_text, full_content, summary, or title
- Check article.extraction_error field

**"Similarity score always 1.0"**

- May indicate embedding model issues
- Check ChromaDB logs for errors

### Debug Mode

Enable verbose logging:

```bash
LOG_LEVEL=debug go run main.go
```
