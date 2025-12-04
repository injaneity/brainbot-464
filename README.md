# BrainBot

Automated RSS-to-YouTube pipeline using AI. Fetches articles, generates video content, and publishes to YouTube.

## 🎯 Quick Start

### Docker Mode (Recommended)

```bash
# 1. Set up YouTube credentials
cd creation_service/scripts
./setup_creation_service_credentials.sh
cd ../..

# 2. Create .env file with API keys
cp .env.example .env
# Edit .env and add your GEMINI_API_KEY and FAL_KEY

# 3. Run the demo
./run-demo-docker.sh
```

Press `d` in the demo to start processing!

### Local Development Mode

```bash
./run-demo.sh
```

## 📋 Prerequisites

- Docker & Docker Compose
- Go 1.24+
- Python 3.9+ (for local generation service)
- API Keys:
  - **Google Gemini API** - For script generation
  - **FAL.ai API** - For video/audio generation
  - **YouTube OAuth** - For uploading (see [creation_service/README.md](creation_service/README.md))
  - **Cohere or OpenAI** - For embeddings (deduplication)

## 🏗️ Architecture

```
┌─────────────────────────────────────────────────────────────┐
│                    Infrastructure Layer                      │
│  ┌──────────┐  ┌────────┐  ┌────────┐  ┌─────────────┐    │
│  │Zookeeper │  │ Kafka  │  │Kafka UI│  │  ChromaDB   │    │
│  │  :2181   │  │:9092/93│  │ :8090  │  │   :8000     │    │
│  └──────────┘  └────────┘  └────────┘  └─────────────┘    │
│                                            ┌─────────────┐    │
│                                            │    Redis    │    │
│                                            │    :6379    │    │
│                                            └─────────────┘    │
└─────────────────────────────────────────────────────────────┘
                           ↓
┌─────────────────────────────────────────────────────────────┐
│                   Application Services                       │
│                                                              │
│  📥 ingestion_service (:8080)                               │
│     • Fetches RSS feeds                                     │
│     • Extracts article content                              │
│     • Deduplicates via Redis (Bloom) & ChromaDB             │
│     • Stores content in S3                                  │
│                  ↓                                           │
│  🤖 generation_service (:8002)                              │
│     • Generates video scripts (Gemini)                      │
│     • Creates videos & audio (FAL.ai)                       │
│     • Publishes to Kafka                                    │
│                  ↓                                           │
│  📤 creation_service (Kafka Consumer)                       │
│     • Processes videos with FFmpeg                          │
│     • Uploads to YouTube                                    │
└─────────────────────────────────────────────────────────────┘
```

## 📁 Project Structure

```
brainbot-464/
├── ingestion_service/      # RSS ingestion & deduplication
├── generation_service/     # AI content generation  
├── creation_service/       # Video creation & upload
├── demo/                  # Interactive demo client
├── docker-compose.yml     # Service orchestration
└── run-demo.sh            # Unified demo runner
```

See individual service READMEs for details:
- [ingestion_service/README.md](ingestion_service/README.md)
- [generation_service/README.md](generation_service/README.md)
- [creation_service/README.md](creation_service/README.md)

## 🔧 Setup Instructions

### 1. Clone & Install Dependencies

```bash
git clone <repository-url>
cd brainbot-464

# Go dependencies
go mod download

# Python dependencies (for local mode)
cd generation_service
python -m venv venv
source venv/bin/activate  # or `venv\Scripts\activate` on Windows
pip install -r requirements.txt
cd ..
```

### 2. Configure Environment

Create `.env` file in project root:

```bash
cp .env.example .env
```

Edit `.env` with your API keys:

```env
# Generation Service
GEMINI_API_KEY=your_gemini_api_key
FAL_KEY=your_fal_api_key

# Ingestion Service (choose one)
COHERE_API_KEY=your_cohere_key
# OR
OPENAI_API_KEY=your_openai_key
```

### 3. Set Up YouTube OAuth

```bash
cd creation_service/scripts
./setup_creation_service_credentials.sh --slot 1 --client-secret client_secret.json
```

Run the same command with `--slot 2`, `--slot 3`, etc. (and their corresponding `client_secret_X.json` files) to register additional channels. The script keeps `.secrets/youtube.env` up to date with per-slot secrets plus a `YOUTUBE_ACCOUNT_SLOT` default, so you only need to flip that value to switch accounts.

When you later start the stack via Docker, three creation-service containers spin up automatically, each pinned to one of those slots and listening to a dedicated Kafka topic:

| Service | Kafka topic | Consumer group | YouTube slot | Output dir |
|---------|-------------|----------------|--------------|------------|
| `creation-service-tech` | `video-requests-tech` | `creation-service-tech` | 1 | `creation_service/outputs/tech` |
| `creation-service-finance` | `video-requests-finance` | `creation-service-finance` | 2 | `creation_service/outputs/finance` |
| `creation-service-other` | `video-requests-other` | `creation-service-other` | 3 | `creation_service/outputs/other` |

The generation service classifies each article into one of those topics, so uploads automatically land in the matching YouTube account.

### 4. Start Services

```bash
# Auto-detects and uses Docker if available (recommended)
./run-demo.sh

# Or force local mode
./run-demo.sh --local
```

Press 'd' in the demo to start processing!

## 🎮 Demo Instructions

Once services are running:

1. The demo client will start automatically
2. Press `d` to trigger the RSS processing workflow
3. Watch as articles are:
   - Fetched from RSS feeds
   - Deduplicated against existing content
   - Sent to generation service for AI processing
   - Created into videos and uploaded to YouTube

4. Press `q` to quit

### Demo Features

- **Interactive UI** - Real-time status updates
- **Webhook Server** - Receives generation results
- **Progress Tracking** - See each step in the pipeline
- **Error Handling** - Clear error messages if something fails

## 🐳 Docker Commands

```bash
# Start all services
docker-compose up -d

# View logs (all services)
docker-compose logs -f

# View logs (specific service)
docker-compose logs -f ingestion-service
docker-compose logs -f generation-service
docker-compose logs -f creation-service-tech
docker-compose logs -f creation-service-finance
docker-compose logs -f creation-service-other

# Stop services
docker-compose down

# Rebuild after code changes
docker-compose build
docker-compose up -d

# Reset everything (including data)
docker-compose down -v
```

## 🛠️ Development

### Build Services

```bash
# Ingestion service
cd ingestion_service && go build -o ingestion-server main.go

# Generation service
cd generation_service && pip install -r requirements.txt

# Creation service
cd creation_service && go build -o creation-service main.go

# Demo client
go build -o demo demo/main.go
```

### Run Individual Services Locally

```bash
# Ingestion service
cd ingestion_service && go run main.go

# Generation service
cd generation_service
source venv/bin/activate
export PORT=8002
python -m app.main

# Creation service (Kafka consumer mode)
cd creation_service
source .secrets/youtube.env
export YOUTUBE_ACCOUNT_SLOT=1   # set to 2/3/etc to pick another channel
export KAFKA_BOOTSTRAP_SERVERS=localhost:9093
go run main.go -kafka

# Creation service (API mode)
go run main.go -port :8081

# Demo client
export API_URL=http://localhost:8080
export GENERATION_SERVICE_URL=http://localhost:8002
go run demo/main.go
```

### Run Tests

```bash
# Go tests
go test ./...

# Python tests (generation service)
cd generation_service
pytest app/tests/
```

## 📊 Service Endpoints

| Service | Port | Health Check | Purpose |
|---------|------|--------------|---------|
| Ingestion Service | 8080 | `GET /api/health` | RSS & Deduplication |
| Generation Service | 8002 | `GET /health` | AI Content Generation |
| ChromaDB | 8000 | `GET /api/v1/heartbeat` | Vector Database |
| Kafka UI | 8090 | http://localhost:8090 | Kafka Monitoring |
| Kafka | 9092/9093 | - | Message Queue |

### API Examples

```bash
# Check ingestion service health
curl http://localhost:8080/api/health

# Check deduplication count
curl http://localhost:8080/api/deduplication/count

# Process article for deduplication
curl -X POST http://localhost:8080/api/deduplication/process \
  -H "Content-Type: application/json" \
  -d '{
    "article": {
      "id": "test-123",
      "title": "Test Article",
      "content": "Article content here..."
    }
  }'

# Generate video content
curl -X POST http://localhost:8002/generate \
  -H "Content-Type: application/json" \
  -d '{
    "title": "Test Article",
    "content": "Article content...",
    "url": "https://example.com/article"
  }'
```

## 🔍 Monitoring & Debugging

### View Logs

**Docker Mode:**
```bash
docker-compose logs -f ingestion-service
docker-compose logs -f generation-service
docker-compose logs -f creation-service
docker-compose logs -f kafka
```

**Local Mode:**
```bash
tail -f api_server.log
tail -f generation_service.log
tail -f creation_service.log
```

### Monitor Kafka Messages

Open Kafka UI: http://localhost:8090

Browse topics, view messages, and monitor consumer groups.

### Check Service Health

```bash
# All health checks
curl http://localhost:8080/api/health
curl http://localhost:8002/health
curl http://localhost:8000/api/v1/heartbeat

# Check if ports are in use
lsof -i :8080 :8002 :8000 :9093
```

### Common Issues

**Services not starting:**
```bash
# Check logs
docker-compose logs

# Reset everything
docker-compose down -v
docker-compose up -d
```

**Build failures:**
```bash
# Clean rebuild
docker-compose build --no-cache

# For Go services
go clean && go mod tidy
```

**Connection issues:**
```bash
# Verify services are running
docker-compose ps

# Check Docker network
docker network inspect brainbot-464_brainbot-network
```

**ChromaDB issues:**
```bash
# Restart ChromaDB
docker-compose restart chromadb

# Clear ChromaDB data
docker-compose down
rm -rf chroma_data/*
docker-compose up -d chromadb
```

## 📚 Service Details

### Ingestion Service

Handles RSS feed processing and deduplication:
- Fetches articles from RSS feeds
- Extracts full content using Mozilla Readability
- Deduplicates using vector embeddings (ChromaDB)
- Exposes REST API for article processing

See [ingestion_service/README.md](ingestion_service/README.md) for details.

### Generation Service

AI-powered content generation:
- Generates video scripts using Google Gemini
- Creates videos and audio using FAL.ai
- Generates subtitles/captions
- Publishes results to Kafka

See [generation_service/README.md](generation_service/README.md) for details.

### Creation Service

Video processing and upload:
- Consumes video requests from Kafka
- Processes videos with FFmpeg
- Uploads to YouTube with metadata
- Supports API, Kafka, and batch modes

See [creation_service/README.md](creation_service/README.md) for details.

## 🔐 Security

- **Never commit credentials** - All secrets are in `.env` or `.secrets/` (gitignored)
- **YouTube OAuth tokens** - Stored in `creation_service/.secrets/youtube.env`
- **API keys** - Loaded from environment variables only
- **Docker secrets** - Use environment variables or Docker secrets in production

## 📝 License

See [LICENSE](LICENSE) file for details.
