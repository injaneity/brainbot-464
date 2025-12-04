package services

import (
	"encoding/json"
	"fmt"
	"log"
	"math/rand"
	"os"
	"path/filepath"
	"sync"
	"time"

	"brainbot/creation_service/app"
	"brainbot/creation_service/app/config"
)

// VideoProcessor handles the video creation and upload pipeline
type VideoProcessor struct {
	uploader    *Uploader
	backgrounds []string
	skipUpload  bool
}

// NewVideoProcessor initializes a new video processor
func NewVideoProcessor(backgroundsDir string) (*VideoProcessor, error) {
	// Try to initialize uploader, but allow it to fail for testing
	uploader, err := NewUploader()
	skipUpload := false

	if err != nil {
		log.Printf("YouTube uploader not initialized (missing credentials): %v", err)
		log.Println("Running in VIDEO-ONLY mode (no upload)")
		skipUpload = true
	} else {
		log.Println("YouTube client initialized")
	}

	backgrounds, err := getBackgroundVideos(backgroundsDir)
	if err != nil {
		return nil, fmt.Errorf("failed to load background videos: %w", err)
	}
	log.Printf("Found %d background videos", len(backgrounds))

	return &VideoProcessor{
		uploader:    uploader,
		backgrounds: backgrounds,
		skipUpload:  skipUpload,
	}, nil
}

// ProcessFromDirectory processes all JSON files in the specified directory
func (p *VideoProcessor) ProcessFromDirectory(inputDir string) error {
	// Find both .json and .txt files
	jsonFiles, err := filepath.Glob(filepath.Join(inputDir, "*.json"))
	if err != nil {
		return fmt.Errorf("failed to read JSON files: %w", err)
	}

	txtFiles, err := filepath.Glob(filepath.Join(inputDir, "*.txt"))
	if err != nil {
		return fmt.Errorf("failed to read txt files: %w", err)
	}

	// Combine both file lists
	allFiles := append(jsonFiles, txtFiles...)

	if len(allFiles) == 0 {
		log.Println("No JSON or TXT files found in input/ directory")
		return nil
	}

	log.Printf("Found %d videos to process", len(allFiles))

	var wg sync.WaitGroup
	semaphore := make(chan struct{}, config.MaxConcurrentVideos)

	for i, jsonFile := range allFiles {
		wg.Add(1)

		go func(idx int, file string) {
			defer wg.Done()

			semaphore <- struct{}{}
			defer func() { <-semaphore }()

			if err := p.ProcessSingleVideo(file, idx+1, len(allFiles)); err != nil {
				log.Printf("Failed to process %s: %v", file, err)
			}

			if idx < len(jsonFiles)-1 {
				time.Sleep(config.VideoBatchDelay)
			}
		}(i, jsonFile)
	}

	wg.Wait()
	log.Println("All videos processed!")
	return nil
}

// ProcessSingleVideo processes a single video from JSON input
func (p *VideoProcessor) ProcessSingleVideo(jsonFile string, current, total int) error {
	log.Printf("[%d/%d] Processing: %s", current, total, filepath.Base(jsonFile))

	data, err := os.ReadFile(jsonFile)
	if err != nil {
		return fmt.Errorf("failed to read JSON: %w", err)
	}

	var input app.VideoInput
	if err := json.Unmarshal(data, &input); err != nil {
		return fmt.Errorf("failed to parse JSON: %w", err)
	}

	if input.Status != "success" {
		return fmt.Errorf("input status is not success: %s", input.Status)
	}

	return p.ProcessVideoInput(input, true)
}

// ProcessVideoInput processes a VideoInput struct and optionally deletes the source file
func (p *VideoProcessor) ProcessVideoInput(input app.VideoInput, cleanup bool) error {
	backgroundVideo := p.backgrounds[rand.Intn(len(p.backgrounds))]
	log.Printf("Using background: %s", filepath.Base(backgroundVideo))

	outputPath := filepath.Join(config.OutputDir, fmt.Sprintf("%s.mp4", input.UUID))
	log.Printf("Creating video...")
	if err := CreateVideo(input, backgroundVideo, outputPath); err != nil {
		return fmt.Errorf("video creation failed: %w", err)
	}
	log.Printf("Video created: %s", outputPath)

	// Skip upload if credentials not configured
	if p.skipUpload {
		log.Printf("Skipping YouTube upload (no credentials)")
		log.Printf("SUCCESS! Video saved: %s", outputPath)
		return nil
	}

	// Use title from input, or generate from subtitles as fallback
	articleTitle := input.Title
	if articleTitle == "" {
		articleTitle = generateTitleFromSubtitles(input.SubtitleTimestamps)
	}

	// Use source URL from input, or use default if not provided
	sourceURL := input.SourceURL
	if sourceURL == "" {
		sourceURL = "https://example.com/article"
	}

	metadata := GenerateMetadata(input, articleTitle, sourceURL)

	log.Printf("Uploading to YouTube...")
	videoID, err := p.uploader.UploadVideo(outputPath, metadata)
	if err != nil {
		return fmt.Errorf("upload failed: %w", err)
	}

	log.Printf("SUCCESS! Video ID: %s", videoID)

	// Optional: cleanup can be disabled for API processing
	if cleanup {
		// Only delete input JSON if it came from a file
		// Keep the video file for now (commented out)
		// os.Remove(outputPath)
	}

	return nil
}

// getBackgroundVideos retrieves all background videos from the specified directory
func getBackgroundVideos(dir string) ([]string, error) {
	files, err := filepath.Glob(filepath.Join(dir, "*.mp4"))
	if err != nil {
		return nil, err
	}

	if len(files) == 0 {
		return nil, fmt.Errorf("no background videos found in %s", dir)
	}

	return files, nil
}

// generateTitleFromSubtitles creates a video title from subtitle timestamps
func generateTitleFromSubtitles(timestamps []app.SubtitleTimestamp) string {
	title := ""
	wordCount := 0

	for _, ts := range timestamps {
		title += ts.Text + " "
		wordCount++
		if wordCount >= config.MaxTitleWords {
			break
		}
	}

	if len(title) > config.MaxTitleLength {
		title = title[:config.MaxTitleLength-3] + "..."
	}

	return title
}
