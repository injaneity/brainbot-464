# Video Creation Service

Microservice responsible for video creation and YouTube upload for the BrainBot content generation system.

## Features

- **Video Creation**: Combines background videos with voiceover audio and subtitles using FFmpeg
- **Subtitle Overlay**: Generates SRT files and overlays them with custom styling
- **YouTube Upload**: Automated upload to YouTube with generated metadata
- **Async Processing**: Accepts requests and processes videos in the background
- **Batch Mode**: Can process multiple videos from a directory
- **Production Ready**: Comprehensive error handling and logging

## Architecture

This service is part of a multi-microservice architecture:

```
┌─────────────────────┐
│  RSS Feed Service   │
└──────────┬──────────┘
           │
           v
┌─────────────────────┐
│ Deduplication Svc   │
└──────────┬──────────┘
           │
           v
┌─────────────────────┐
│  Generation Svc     │ (Python)
│  - Script Gen       │
│  - TTS              │
│  - Timestamps       │
└──────────┬──────────┘
           │
           v
┌─────────────────────┐
│  Creation Service   │ ← This Service
│  - Video Creation   │
│  - YouTube Upload   │
└─────────────────────┘
```

## Project Structure

```
creation_service/
├── main.go                          # Entry point (API or batch mode)
├── app/
│   ├── types.go                     # Data structures
│   ├── config/
│   │   └── constants.go             # Configuration constants
│   ├── api/
│   │   └── server.go                # HTTP server and handlers
│   └── services/
│       ├── creator.go               # Video creation with FFmpeg
│       ├── uploader.go              # YouTube upload logic
│       └── processor.go             # Orchestration pipeline
├── scripts/
│   ├── README.md                    # Scripts documentation
│   └── test_video_generation.go    # Test video creation
├── inputs/                          # JSON input files for batch mode & testing
├── outputs/                         # Generated video outputs
├── backgroundvids/                  # Background video assets
├── README.md
└── Dockerfile (coming soon)
```

## Setup

### Prerequisites

- Go 1.24 or higher
- FFmpeg installed on system
- YouTube API OAuth client (client ID/secret) and refresh token for the channel you want to upload to
- Background videos in `backgroundvids/` directory

### Install FFmpeg

**macOS:**

```bash
brew install ffmpeg
```

**Ubuntu/Debian:**

```bash
sudo apt-get update
sudo apt-get install ffmpeg
```

**Windows:**
Download from [ffmpeg.org](https://ffmpeg.org/download.html)

### Environment Setup

1. **YouTube OAuth Credentials**

  - In Google Cloud Console, enable the **YouTube Data API v3**
  - Create an **OAuth client ID** (Desktop app) and download the JSON file with the client ID/secret
  - Generate a refresh token for the YouTube channel owner by running the helper script:

    ```bash
    cd creation_service
    python3 scripts/get_refresh_token.py \
      --client-secret scripts/client_secret.json \
      --scopes https://www.googleapis.com/auth/youtube.upload
    ```

    The script opens a browser window for consent and prints the refresh token (store it securely).
  - Export the values as environment variables before running the service:

    ```bash
    export YOUTUBE_CLIENT_ID="your-client-id"
    export YOUTUBE_CLIENT_SECRET="your-client-secret"
    export YOUTUBE_REFRESH_TOKEN="refresh-token-from-consent-flow"
    ```

    To automate the credential setup (including regenerating tokens when needed) you can run:

    ```bash
    scripts/setup_creation_service_credentials.sh \
      --client-secret scripts/client_secret.json
    ```

    The script will run the OAuth helper (if needed) and refresh the `.secrets/youtube.env` file for you. After it finishes, source that env file and start the service manually (API or batch mode) using `go run main.go ...`.

2. **Background Videos**
   - Place background videos (9:16 vertical format, .mp4) in `backgroundvids/` directory
   - Service will randomly select from available videos

3. **Upload-only Smoke Test (optional)**

  If you already have a rendered video (e.g., in `outputs/uuid.mp4`) and just want to validate uploading, run:

  ```bash
  scripts/test_upload.sh \
    --video outputs/demo-001.mp4 \
    --title "Demo upload" \
    --client-secret scripts/client_secret.json
  ```

  The script ensures your OAuth tokens exist, sources the `YOUTUBE_*` env vars, and calls `go run ./creation_service/cmd/upload` to upload that MP4 with the metadata you specify. Provide `--description`, `--source-url`, or custom tags if you need to override the defaults.

## Running the Service

### API Mode (Default)

Start the HTTP server:

```bash
cd creation_service
go run main.go
```

Or specify a custom port:

```bash
go run main.go -port :8081
```

The service will be available at `http://localhost:8081`

### Batch Mode

Process all JSON files from `inputs/` directory:

```bash
go run main.go -batch
```

Place your JSON files (matching the request schema) in the `inputs/` directory, and the service will process them sequentially.

## API Usage

### Health Check

```bash
curl http://localhost:8081/health
```

**Response:**

```json
{
  "status": "healthy"
}
```

### Process Video

```bash
curl -X POST http://localhost:8081/api/process-video \
  -H "Content-Type: application/json" \
  -d '{
    "uuid": "test-123",
    "voiceover": "https://example.com/audio.mp3",
    "subtitle_timestamps": [
      {
        "text": "Hello",
        "start": 0.0,
        "end": 0.5
      },
      {
        "text": "world",
        "start": 0.5,
        "end": 1.0
      }
    ],
    "resource_timestamps": {},
    "status": "success"
  }'
```

**Request Schema:**

```json
{
  "uuid": "string (required)",
  "voiceover": "string (required, URL)",
  "subtitle_timestamps": [
    {
      "text": "string",
      "start": 0.0,
      "end": 0.5
    }
  ],
  "resource_timestamps": {},
  "status": "success" (must be "success")
}
```

**Response (202 Accepted):**

```json
{
  "success": true,
  "message": "Video processing started"
}
```

The video will be processed asynchronously. Check logs for progress:

```
📥 Received video processing request: UUID=test-123
🎨 Using background: background1.mp4
🎥 Creating video...
✅ Video created: output/test-123.mp4
📤 Uploading to YouTube...
✅ Uploaded! https://youtube.com/shorts/dQw4w9WgXcQ
🎉 SUCCESS! Video ID: dQw4w9WgXcQ
```

## Configuration

All configuration is in `app/config/constants.go`:

| Constant              | Value   | Description               |
| --------------------- | ------- | ------------------------- |
| `VideoWidth`          | 1080    | Video width (9:16 format) |
| `VideoHeight`         | 1920    | Video height              |
| `VideoCodec`          | libx264 | H.264 codec               |
| `AudioCodec`          | aac     | AAC audio codec           |
| `AudioBitrate`        | 192k    | Audio quality             |
| `MaxVideoDuration`    | 180.0   | Max 3 minutes             |
| `VideoEndPadding`     | 0.5     | End padding in seconds    |
| `MaxConcurrentVideos` | 3       | Parallel processing limit |

## Subtitle Styling

Subtitles are overlaid with the following style:

- **Font**: Consolas
- **Size**: 32px
- **Color**: White (#FFFFFF)
- **Outline**: Black, 2px
- **Position**: Bottom center
- **Style**: Bold

## Error Handling

The service handles various errors:

- Invalid JSON payload → 400 Bad Request
- Missing required fields → 400 Bad Request
- Download failures → Logged, processing aborted
- FFmpeg errors → Detailed error messages
- Upload failures → Retry logic (future enhancement)

## Deployment

### Docker (Coming Soon)

```bash
docker build -t creation-service .
docker run -p 8081:8081 \
  -v $(pwd)/backgroundvids:/app/backgroundvids \
  -e YOUTUBE_CLIENT_ID \
  -e YOUTUBE_CLIENT_SECRET \
  -e YOUTUBE_REFRESH_TOKEN \
  creation-service
```

### Production Considerations

1. **Scaling**: Run multiple instances behind a load balancer
2. **Storage**: Use shared storage (S3) for background videos and output
3. **Monitoring**: Add Prometheus metrics for video processing times
4. **Webhooks**: Add callback support to notify when upload completes
5. **Queue**: Consider adding a message queue (RabbitMQ/SQS) for better async processing

## Integration with Other Services

The creation service typically receives requests from the generation service:

```
Generation Service                Creation Service
      │                                  │
      │   POST /api/process-video        │
      ├─────────────────────────────────>│
      │                                  │
      │   202 Accepted                   │
      │<─────────────────────────────────┤
      │                                  │
      │                             (async)
      │                                  │
      │                         Create Video
      │                         Upload to YT
      │                                  │
      │   (Future: Webhook callback)     │
      │<─────────────────────────────────┤
```

## Troubleshooting

**Issue: FFmpeg not found**

```
Error: ffmpeg failed: exec: "ffmpeg": executable file not found
```

Solution: Install FFmpeg using package manager

**Issue: OAuth credentials missing**

```
Error: missing required env vars: [YOUTUBE_CLIENT_ID ...]
```

Solution: Export `YOUTUBE_CLIENT_ID`, `YOUTUBE_CLIENT_SECRET`, and `YOUTUBE_REFRESH_TOKEN` before running the service (or provide them via your process manager).

**Issue: No background videos found**

```
Error: no background videos found in backgroundvids
```

Solution: Add .mp4 files to `backgroundvids/` directory

**Issue: YouTube quota exceeded**

```
Error: failed to upload video: quotaExceeded
```

Solution: Check YouTube API quota limits in Google Cloud Console

## Development

### Build

```bash
go build -o creation-service main.go
```

### Run Tests (Coming Soon)

```bash
go test ./...
```

## License

Part of the BrainBot project.
