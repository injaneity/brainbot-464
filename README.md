# BrainBot

Microservice-based RSS feed ingestion and AI content generation platform. Fetches RSS feeds, extracts full article content, deduplicates using vector embeddings, and generates AI-powered audio scripts with subtitles.

## Features

- **RSS Feed Processing**: Fetch and parse RSS/Atom feeds with presets (CNA, Straits Times, Hacker News, Tech Review)
- **Content Extraction**: Full-text extraction via Mozilla Readability with parallel worker pools
- **Vector-Based Deduplication**: ChromaDB-powered similarity detection using embeddings (Cohere or OpenAI)
- **Microservice Architecture**: Fully isolated services communicating via REST APIs
- **AI Content Generation**: Generate scripts, audio (TTS), and subtitles from articles
- **S3 Storage**: Optional cloud storage for processed articles
- **Interactive Demo**: Terminal UI showcasing end-to-end integration
- **Docker Support**: Containerized services for easy deployment

## Architecture

```
┌──────────────┐     ┌──────────────────┐     ┌─────────────┐
│   API Server │────▶│  Deduplication   │────▶│  ChromaDB   │
│  (port 8080) │     │    Service       │     │ (port 8000) │
└──────┬───────┘     └──────────────────┘     └─────────────┘
       │
       ├─► RSS Orchestrator
       ├─► Articles Management
       └─► Health Checks

┌──────────────────┐
│  Generation      │
│  Service         │
│  (port 8002)     │
└──────────────────┘
```

## 🚀 Quick Start

### Prerequisites

- Go 1.25+
- Docker & Docker Compose
- Python 3.9+ (for generation service)
- API Keys:
  - Google API Key (for Gemini)
  - Fal.ai API Key (for TTS)
  - Cohere or OpenAI API Key (for embeddings)

### One-Command Demo

The fastest way to see everything in action:

```bash
./run-demo.sh
```

This automatically:

1. ✅ Starts ChromaDB (if not running)
2. ✅ Starts Generation Service (if not running)
3. ✅ Starts API Server (if not running)
4. ✅ Launches the interactive demo
5. 🧹 Cleans up all services on exit

**Press `d` when the demo starts to begin the workflow!**

### Manual Setup

If you prefer manual control:

#### 1. Start ChromaDB

```bash
docker-compose up -d chroma

# Verify it's running
docker ps | grep chroma
```

#### 2. Configure Environment

```bash
# Required: Embeddings provider (choose one)
export COHERE_API_KEY=your_cohere_key
# OR
export OPENAI_API_KEY=your_openai_key

# Optional: S3 storage
export S3_BUCKET=your-bucket
export S3_REGION=us-east-1
export AWS_ACCESS_KEY_ID=your_key
export AWS_SECRET_ACCESS_KEY=your_secret

# Optional: Generation service credentials
export GOOGLE_API_KEY=your_gemini_key
export FAL_KEY=your_fal_key
```

#### 3. Start API Server

```bash
go build -o brainbot main.go
./brainbot
```

The API server will be available at `http://localhost:8080`

#### 4. (Optional) Start Generation Service

```bash
cd generation_service
python -m venv venv
source venv/bin/activate
pip install -r requirements.txt

# Set custom port to avoid conflict with ChromaDB
PORT=8002 python -m app.main
```

#### 5. Run the Demo

```bash
export API_URL=http://localhost:8080
export GENERATION_SERVICE_URL=http://localhost:8002
go run cmd/demo/main.go
```

## API Endpoints

Full API documentation: [api/API_REFERENCE.md](api/API_REFERENCE.md)

### Quick Reference

```bash
# Health check
curl http://localhost:8080/api/health

# Trigger RSS refresh
curl -X POST http://localhost:8080/api/rss/refresh

# Get article count
curl http://localhost:8080/api/deduplication/count

# Process an article
curl -X POST http://localhost:8080/api/deduplication/process \
  -H "Content-Type: application/json" \
  -d '{"article": {...}}'

# List ChromaDB articles
curl "http://localhost:8080/api/chroma/articles?limit=10"

# Clear deduplication cache
curl -X DELETE http://localhost:8080/api/deduplication/clear
```

## Services

### API Server (Main)

**Port:** 8080 (default)  
**Purpose:** Unified gateway for all operations

Endpoints:

- `/api/health` - Health check
- `/api/articles` - Article management
- `/api/chroma/*` - ChromaDB access
- `/api/deduplication/*` - Deduplication service
- `/api/rss/refresh` - RSS orchestrator

**Documentation:** [api/API_REFERENCE.md](api/API_REFERENCE.md)

### Generation Service

**Port:** 8002 (configurable)  
**Purpose:** AI-powered script, audio, and subtitle generation

Endpoints:

- `/health` - Health check
- `/generate` - Generate content from articles

**Documentation:** [generation_service/README.md](generation_service/README.md)

### ChromaDB

**Port:** 8000  
**Purpose:** Vector database for deduplication

Managed via Docker Compose. See [docker-compose.yml](docker-compose.yml)

## Configuration

### Environment Variables

```bash
# API Server
PORT=8080
CHROMA_HOST=localhost
CHROMA_PORT=8000
CHROMA_COLLECTION=brainbot_articles

# Embeddings (choose one)
COHERE_API_KEY=your_key
# OR
OPENAI_API_KEY=your_key
OPENAI_ORG_ID=your_org  # optional

# RSS Feed
RSS_FEED_PRESET=st  # cna, st, hn, tr

# S3 (optional)
S3_BUCKET=your-bucket
S3_REGION=us-east-1
S3_PREFIX=articles/

# Generation Service
GOOGLE_API_KEY=your_gemini_key
FAL_KEY=your_fal_key
WEBHOOK_URL=your_webhook_url

# Demo
API_URL=http://localhost:8080
GENERATION_SERVICE_URL=http://localhost:8002
WEBHOOK_PORT=9999
```

## Project Structure

```
brainbot-464/
├── api/                      # API controllers & routing
│   ├── server.go             # Server setup & route registration
│   ├── healthcontroller.go   # Health endpoint
│   ├── articlescontroller.go # Articles management
│   ├── chromacontroller.go   # ChromaDB access
│   ├── deduplicationcontroller.go  # Deduplication endpoints
│   ├── rsscontroller.go      # RSS orchestrator trigger
│   └── API_REFERENCE.md      # API documentation
├── app/                      # Application client library
│   └── app.go                # HTTP client for all services
├── cmd/demo/                 # Interactive demo
│   └── main.go               # Terminal UI demo
├── deduplication/            # Deduplication service
│   ├── deduplicator.go       # Core deduplication logic
│   ├── embeddings.go         # Embeddings API clients
│   └── chroma.go             # ChromaDB REST wrapper
├── generation_service/       # AI generation microservice
│   ├── app/
│   │   ├── main.py           # FastAPI server
│   │   ├── services/         # Gemini & Fal.ai clients
│   │   └── api/schemas.py    # Request/response models
│   └── README.md             # Generation service docs
├── orchestrator/             # End-to-end pipeline
│   └── orchestrator.go       # Fetch → Extract → Dedupe → S3
├── rssfeeds/                 # RSS fetching & extraction
│   ├── fetcher.go            # RSS/Atom parsing
│   ├── extractor.go          # Content extraction
│   └── config.go             # Feed presets
├── types/                    # Shared data models
│   └── article.go            # Article struct
├── common/                   # Shared utilities
│   └── s3.go                 # S3 client wrapper
├── main.go                   # API server entry point
├── docker-compose.yml        # Docker services
├── run-demo.sh              # One-command demo script
└── README.md                 # This file
```

## Interactive Demo

The demo showcases the complete integration:

1. Fetches RSS feed articles
2. Extracts full content
3. Deduplicates against ChromaDB
4. Sends new articles to generation service
5. Receives results via webhook
6. Displays generated audio & subtitles

```bash
./run-demo.sh
# Press 'd' to start the demo workflow
# Press 'q' to quit
```

See [demo/README.md](demo/README.md) for details.

## Docker Deployment

### Using Docker Compose

```bash
# Create .env file with your API keys
cp .env.example .env
# Edit .env with your credentials

# Start all services
docker-compose up -d --build

# Check status
docker-compose ps

# View logs
docker-compose logs -f app

# Stop services
docker-compose down

# Stop and remove volumes
docker-compose down -v
```

### Production Deployment

See individual service documentation:

- API Server: Build with `go build -o brainbot main.go`
- Generation Service: [generation_service/README.md](generation_service/README.md)
- ChromaDB: Use official Docker image

## Development

### Run Tests

```bash
# Go tests
go test ./...

# Python tests (generation service)
cd generation_service
pytest app/tests/
```

### Build Binaries

```bash
# API server
go build -o brainbot main.go

# Demo
go build -o demo_bin cmd/demo/main.go
```

### Local Development

```bash
# Start dependencies
docker-compose up -d chroma

# Run API server
go run main.go

# Run demo (in another terminal)
go run cmd/demo/main.go
```

## Troubleshooting

### ChromaDB Issues

**Container won't start:**

```bash
# Check if port 8000 is in use
lsof -i :8000

# Remove existing container
docker stop brainbot-chroma && docker rm brainbot-chroma

# Start fresh
docker-compose up -d chroma
```

**Connection errors:**

```bash
# Test ChromaDB connectivity
curl http://localhost:8000/

# Should return 404 (ChromaDB is running but endpoint doesn't exist)
```

### Generation Service Issues

**Port conflict:**

```bash
# Check what's using the port
lsof -i :8002

# Use a different port
PORT=8003 python -m app.main
```

**API key errors:**

```bash
# Verify environment variables are set
echo $GOOGLE_API_KEY
echo $FAL_KEY

# Check generation service logs
cat generation_service.log
```

### API Server Issues

**Build errors:**

```bash
# Clean and rebuild
go clean
go mod tidy
go mod download
go build -o brainbot main.go
```

**Can't connect to ChromaDB:**

```bash
# Verify ChromaDB is running
docker ps | grep chroma

# Check environment variables
echo $CHROMA_HOST
echo $CHROMA_PORT
```

### Demo Issues

**Connection refused:**

```bash
# Ensure API server is running
curl http://localhost:8080/api/health

# Check environment variables
echo $API_URL
echo $GENERATION_SERVICE_URL
```

## Key Differences from Old Architecture

### Before (Monolithic)

```go
// Demo directly imported deduplication
import "brainbot/deduplication"

deduplicator, _ := deduplication.NewDeduplicator(config)
result, _ := deduplicator.ProcessArticle(article)
```

### After (Microservices)

```go
// Demo uses HTTP client
import "brainbot/app"

client := app.NewClient("http://localhost:8080")
result, _ := client.ProcessArticle(ctx, article)
```

**Benefits:**

- Loose coupling between services
- Network-based communication
- Independent scaling
- Clear service boundaries
- Easier testing with API mocks

## Contributing

1. Fork the repository
2. Create a feature branch
3. Make your changes
4. Add tests
5. Submit a pull request

## License

See [LICENSE](LICENSE) file for details.

## Support

For issues or questions:

- Open a GitHub issue
- Check documentation in each service directory
- Review API reference docs
