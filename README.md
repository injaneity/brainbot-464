# BrainBot

Ingest RSS feeds, extract full article content, deduplicate using a vector database, and optionally upload sanitized JSON to S3. Ships as an HTTP API with a one-shot background "orchestrator" run on startup.

## Features

- RSS/Atom fetching with feed presets (CNA, Straits Times, Hacker News, Tech Review)
- Full-content extraction via Mozilla Readability (worker pool of 5)
- Deduplication with ChromaDB (local Docker) using embeddings (Cohere or OpenAI)
- Optional S3 uploads (sanitized JSON, no images)
- HTTP API to trigger refresh and inspect stored vectors
- Graceful failure handling; extraction errors are tracked on the article

## Quick start

1. Start ChromaDB (required for deduplication):

```bash
docker compose up -d
```

2. Provide an embeddings provider (pick one):

- Cohere: set COHERE_API_KEY
- OpenAI: set OPENAI_API_KEY (optional OPENAI_ORG_ID)

3. Build and run the API:

```bash
go mod download
go build -o brainbot .
./brainbot
```

On startup, one orchestrator run fetches a preset feed, extracts, deduplicates, and uploads (if S3 is configured). You can trigger additional runs via the API.

## API endpoints

- GET /api/health — basic health check
- POST /api/rss/refresh — trigger a background orchestrator run
- GET /api/chroma/articles?limit=&offset= — list stored documents from the Chroma collection
- POST /articles — accept a posted Article and echo normalized identifiers (utility endpoint)

## Components overview

- rssfeeds/
  - config.go — feed presets and defaults
  - fetcher.go — fetch/parse feeds (gofeed)
  - extractor.go — readability extraction with worker pool
  - utils.go — ID generation (sha256 short hash)
- deduplication/
  - embeddings.go — Cohere or OpenAI embeddings client (selected by env)
  - chroma.go — minimal Chroma v2 REST wrapper (collections, add/query/get/list/update/delete)
  - deduplicator.go — similarity search, TTL metadata maintenance, duplicate detection
- orchestrator/
  - orchestrator.go — end-to-end run: fetch → extract → deduplicate → optional S3 upload → summary
- api/
  - server.go — route registration (Gin)
  - healthcontroller.go — health endpoint
  - rsscontroller.go — trigger refresh (POST)
  - chromacontroller.go — read/list documents from Chroma
  - articlescontroller.go — utility POST for articles
- common/
  - s3.go — thin wrapper over AWS SDK v2 S3 client
- types/
  - article.go — Article and FeedResult models

## Local development

Prereqs

- Go 1.24+
- Docker (for ChromaDB)

Run

```bash
docker compose up -d
export COHERE_API_KEY=...
go build -o brainbot .
./brainbot
```

Quick checks

```bash
curl -s http://localhost:8080/api/health
curl -s -X POST http://localhost:8080/api/rss/refresh
curl -s "http://localhost:8080/api/chroma/articles?limit=10"
```

## Docker Compose

The repository provides a simple `docker-compose.yml` that starts a local
ChromaDB instance used by the application for vector storage. If you need a
multi-container development environment that includes Redis (with RedisBloom)
for integration testing, see `docker-compose.test.yml` (added for CI/local
tests).

To run the (minimal) Chroma service defined in `docker-compose.yml`:

```bash
docker compose up -d
```

If you want to run the full application in Docker Compose, add an `app` service
to the compose file (or create a separate compose override) that builds and
runs the `brainbot` binary and depends on the `chroma` service.

To run the integration test harness (Chroma + Redis Stack including RedisBloom):

```bash
./scripts/test-integration.sh
```

To stop and clean up (compose or test harness):

```bash
docker compose down
docker compose -f docker-compose.test.yml down -v --remove-orphans
```
