# BrainBot Utilities

Demo and utility scripts for the BrainBot project.

## Files

### run-demo.sh

Complete demo runner that orchestrates all BrainBot services.

**Usage:**

```bash
./run-demo.sh
```

**What it does:**

1. Starts ChromaDB (port 8000)
2. Starts Generation Service (port 8002)
3. Starts Deduplication API (port 8080)
4. Starts Creation Service (port 8081)
5. Runs the demo workflow

**Features:**

- Automatic service startup
- Health checks for all services
- Cleanup on exit
- Service log locations

### download_background.ps1

PowerShell script for downloading YouTube videos to use as background footage.

**Usage:**

```powershell
.\download_background.ps1
```

Edit the `$videoUrl` variable in the script to change the source video.

**Requirements:**

- PowerShell
- yt-dlp (installed automatically if missing)

## Notes

These utilities are meant for development, testing, and demo purposes across all BrainBot microservices.
