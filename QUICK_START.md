# Quick Start Guide: Refactored Architecture

## Prerequisites

Before running the application, ensure you have:

- Go 1.25+ installed
- Docker and Docker Compose installed
- (Optional) AWS credentials configured for S3 support

## 🚀 Easiest Way: One-Command Demo

Run everything with a single command:

```bash
./run-demo.sh
```

This script will:

1. ✅ Start ChromaDB (if not running)
2. ✅ Start Generation Service (if not running)
3. ✅ Start API Server (if not running)
4. ✅ Launch the demo automatically
5. 🧹 Clean up all services when you exit

**That's it!** Press `d` when the demo starts, and everything will work.

---

## Manual Setup (Advanced)

If you prefer to start services individually or need more control:

## Running the Application

### 1. Start ChromaDB (Required)

ChromaDB is the vector database used for deduplication. Start it using Docker Compose:

```bash
# Start ChromaDB in the background
docker-compose up -d chroma

# Verify ChromaDB is running
curl http://localhost:8000/api/v1/heartbeat

# Expected response: {"nanosecond heartbeat": ...}
```

**Alternative: Using Docker directly**

```bash
docker run -d \
  --name brainbot-chroma \
  -p 8000:8000 \
  -v $(pwd)/chroma_data:/chroma/chroma \
  -e IS_PERSISTENT=TRUE \
  -e ANONYMIZED_TELEMETRY=FALSE \
  chromadb/chroma:latest
```

**Check ChromaDB status:**

```bash
# View logs
docker logs brainbot-chroma

# Check if container is running
docker ps | grep chroma
```

### 2. (Optional) Configure S3 for Article Storage

If you want to store articles in S3, configure your AWS credentials:

```bash
# Set up AWS credentials (if not already configured)
aws configure

# Or set environment variables
export AWS_ACCESS_KEY_ID=your_access_key
export AWS_SECRET_ACCESS_KEY=your_secret_key
export AWS_REGION=us-east-1

# Configure S3 bucket for the application
export S3_BUCKET=your-bucket-name
export S3_PREFIX=articles/
export S3_REGION=us-east-1

# Optional: Use LocalStack for local S3 testing
docker run -d \
  --name localstack \
  -p 4566:4566 \
  -e SERVICES=s3 \
  localstack/localstack:latest

# Configure for LocalStack
export S3_BUCKET=test-bucket
export S3_USE_PATH_STYLE=true
export AWS_ENDPOINT_URL=http://localhost:4566
```

**Note:** If S3 is not configured, the application will work fine but won't upload articles to S3.

### 3. Start the Main API Server

The main server now includes all deduplication endpoints:

```bash
# Start the API server on port 8080 (default)
go run main.go

# Or use custom port
PORT=8090 go run main.go
```

The server exposes:

- `/api/health` - Health check
- `/api/articles` - Article management
- `/api/chroma/articles` - ChromaDB direct access
- `/api/deduplication/*` - Deduplication service endpoints
- `/api/rss/refresh` - RSS orchestrator trigger

### 4. Run the Demo

The demo now uses the app client to communicate with the API:

```bash
# Make sure API server is running first!
API_URL=http://localhost:8080 go run cmd/demo/main.go
```

**Demo Flow:**

1. Press `d` to start the demo
2. Fetches articles from RSS feed
3. Sends articles to deduplication API
4. Displays results
5. Sends new articles to generation service

### Quick Start Script (All-in-One)

**Deprecated:** Use `./run-demo.sh` instead (see top of guide).

For manual control, you can use the provided scripts:

```bash
# Start infrastructure services only
./demo/start-services.sh

# In another terminal, start the API server
go run main.go

# In a third terminal, run the demo
go run cmd/demo/main.go
```

**To stop all services:**

```bash
./demo/stop-services.sh
```

### 5. Test Deduplication Endpoints

```bash
# Check count
curl http://localhost:8080/api/deduplication/count

# Clear cache
curl -X DELETE http://localhost:8080/api/deduplication/clear

# Process an article
curl -X POST http://localhost:8080/api/deduplication/process \
  -H "Content-Type: application/json" \
  -d '{
    "article": {
      "id": "test-123",
      "title": "Test Article",
      "url": "https://example.com/test",
      "summary": "This is a test article",
      "published_at": "2025-01-01T00:00:00Z",
      "fetched_at": "2025-01-01T00:00:00Z"
    }
  }'
```

## Configuration

### Environment Variables

```bash
# API Server Configuration
export PORT=8080
export CHROMA_HOST=localhost
export CHROMA_PORT=8000
export CHROMA_COLLECTION=brainbot_articles

# Demo Configuration
export API_URL=http://localhost:8080
export WEBHOOK_PORT=9999
export GENERATION_SERVICE_URL=http://localhost:8001

# Optional: S3 Configuration (comment out to disable)
export S3_BUCKET=your-bucket
export S3_REGION=us-east-1
export S3_PREFIX=articles/

# Optional: For LocalStack (local S3 testing)
# export S3_USE_PATH_STYLE=true
# export AWS_ENDPOINT_URL=http://localhost:4566
```

### Complete Setup Example

**Terminal 1: Start Infrastructure**

```bash
# Start ChromaDB
docker-compose up -d chroma

# Verify ChromaDB is ready
curl http://localhost:8000/api/v1/heartbeat

# (Optional) Start LocalStack for S3
docker run -d -p 4566:4566 --name localstack \
  -e SERVICES=s3 localstack/localstack:latest

# (Optional) Create S3 bucket in LocalStack
aws --endpoint-url=http://localhost:4566 s3 mb s3://test-bucket
```

**Terminal 2: Start API Server**

```bash
# Load environment variables
export CHROMA_HOST=localhost
export CHROMA_PORT=8000

# (Optional) Configure S3
export S3_BUCKET=test-bucket
export S3_USE_PATH_STYLE=true

# Start API server
go run main.go
```

**Terminal 3: Run Demo**

```bash
# Configure demo
export API_URL=http://localhost:8080

# Run demo
go run cmd/demo/main.go

# Press 'd' to start the demo
```

### Cleanup

```bash
# Stop ChromaDB
docker-compose down

# Or manually stop and remove
docker stop brainbot-chroma && docker rm brainbot-chroma

# Stop LocalStack (if using)
docker stop localstack && docker rm localstack

# Clean up data
rm -rf chroma_data/
```

## Architecture Overview

```
┌─────────────────┐
│   Demo Client   │
│  (cmd/demo)     │
└────────┬────────┘
         │ HTTP
         │ (app.Client)
         ▼
┌─────────────────┐     ┌──────────────────┐
│   API Server    │────▶│  Deduplication   │
│   (main.go)     │     │    Service       │
│                 │     │  (/api/dedup)    │
└─────────────────┘     └────────┬─────────┘
         │                       │
         │                       ▼
         │              ┌─────────────────┐
         │              │    ChromaDB     │
         │              │  (Vector Store) │
         │              └─────────────────┘
         │
         ▼
┌─────────────────┐
│  Orchestrator   │
│  (/api/rss)     │
└─────────────────┘
```

## Key Differences from Old Architecture

### Old Way (Direct Dependencies)

```go
// Demo had direct access to deduplication
import "brainbot/deduplication"

deduplicator, _ := deduplication.NewDeduplicator(config)
result, _ := deduplicator.ProcessArticle(article)
```

### New Way (API-Based)

```go
// Demo uses app client
import "brainbot/app"

appClient := app.NewClient("http://localhost:8080")
result, _ := appClient.ProcessArticle(ctx, article)
```

## Benefits

1. **Loose Coupling**: Demo doesn't need to know about ChromaDB or embeddings
2. **Network-Based**: Services can run on different machines
3. **Easy Testing**: Mock the API instead of the database
4. **Independent Scaling**: Scale deduplication service separately
5. **Clear Boundaries**: Each service has defined responsibilities

## Troubleshooting

### ChromaDB Issues

**ChromaDB not starting:**

```bash
# Check if port 8000 is already in use
lsof -i :8000

# Stop any existing ChromaDB containers
docker stop brainbot-chroma && docker rm brainbot-chroma

# Check Docker logs
docker logs brainbot-chroma

# Restart ChromaDB
docker-compose up -d chroma
```

**ChromaDB connection errors:**

```bash
# Test ChromaDB connection
curl http://localhost:8000/api/v1/heartbeat

# Verify ChromaDB is accessible from the app
docker exec brainbot-chroma python -c "import socket; s=socket.socket(); s.connect(('localhost',8000)); print('OK')"
```

### S3 Issues

**S3 upload failures:**

```bash
# Verify AWS credentials
aws sts get-caller-identity

# Test S3 access
aws s3 ls s3://your-bucket-name/

# Check bucket permissions
aws s3api get-bucket-policy --bucket your-bucket-name

# Disable S3 (run without S3)
unset S3_BUCKET
go run main.go
```

**Using LocalStack for local testing:**

```bash
# Create bucket in LocalStack
aws --endpoint-url=http://localhost:4566 s3 mb s3://test-bucket

# List buckets
aws --endpoint-url=http://localhost:4566 s3 ls

# Configure app to use LocalStack
export S3_BUCKET=test-bucket
export S3_USE_PATH_STYLE=true
export AWS_ENDPOINT_URL=http://localhost:4566
```

### Demo can't connect to API

```bash
# Check if API server is running
curl http://localhost:8080/api/health

# Verify API_URL in demo
export API_URL=http://localhost:8080
```

### Deduplication returns errors

```bash
# Check ChromaDB is running
docker ps | grep chromadb

# Test ChromaDB connection
curl http://localhost:8000/api/v1/heartbeat

# Check deduplication endpoint
curl http://localhost:8080/api/deduplication/count
```

### Build errors

```bash
# Ensure all dependencies are downloaded
go mod tidy
go mod download

# Build main server
go build -o brainbot main.go

# Build demo
go build -o demo_bin cmd/demo/main.go
```

## What Changed in Each File

### `api/deduplicationcontroller.go` (NEW)

- Added 5 new endpoints for deduplication service
- Handles check, add, process, clear, count operations

### `api/server.go`

- Registered new deduplication routes

### `app/app.go` (NEW)

- Created application client layer
- Provides HTTP-based methods for all operations

### `cmd/demo/main.go`

- Removed direct deduplication imports
- Uses `app.Client` for all operations
- Cleaner, more focused code

### `orchestrator/orchestrator.go`

- Removed direct deduplication imports
- Uses `app.Client` for processing
- Simpler implementation

## Future Enhancements

1. Add authentication tokens for API access
2. Implement request/response logging
3. Add circuit breakers for resilience
4. Create Docker Compose for all services
5. Add health checks for dependencies
6. Implement request retries with backoff
