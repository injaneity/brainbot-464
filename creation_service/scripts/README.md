# Creation Service Scripts

Utility scripts for testing the video creation service.

## Files

### test_video_generation.go

Test script for video creation without uploading to YouTube.

**Usage:**

```bash
# From creation_service directory
go run scripts/test_video_generation.go
```

This will:

- Load test data from `inputs/output_format.txt`
- Create video with FFmpeg
- Save to `outputs/` directory
- Skip YouTube upload

**Setup:**
Place your test JSON file at `inputs/output_format.txt` before running.

## Notes

For demo orchestration scripts (run-demo.sh, download_background.ps1), see `utils/` folder in the root directory.
