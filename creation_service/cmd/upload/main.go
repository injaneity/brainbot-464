package main

import (
	"flag"
	"fmt"
	"log"
	"os"
	"path/filepath"
	"strings"

	"brainbot/creation_service/app"
	"brainbot/creation_service/app/services"
)

func main() {
	videoPath := flag.String("video", "", "Path to the MP4 file to upload")
	title := flag.String("title", "", "Title for the YouTube video (defaults to filename)")
	description := flag.String("description", "", "Description to use (optional)")
	sourceURL := flag.String("source-url", "", "Optional source URL to append to the description")
	tagsFlag := flag.String("tags", "tech news,AI,technology", "Comma-separated list of tags")
	categoryID := flag.String("category-id", "28", "YouTube category ID (default: 28 - Science & Technology)")

	flag.Parse()

	if *videoPath == "" {
		flag.Usage()
		log.Fatal("--video is required")
	}

	if err := ensureFileExists(*videoPath); err != nil {
		log.Fatalf("invalid video path: %v", err)
	}

	titleVal := strings.TrimSpace(*title)
	if titleVal == "" {
		filename := filepath.Base(*videoPath)
		titleVal = strings.TrimSuffix(filename, filepath.Ext(filename))
	}

	descVal := strings.TrimSpace(*description)
	if descVal == "" {
		descVal = defaultDescription(titleVal, *sourceURL)
	}

	tags := parseTags(*tagsFlag)
	if len(tags) == 0 {
		tags = []string{"tech", "AI", "shorts"}
	}

	uploader, err := services.NewUploader()
	if err != nil {
		log.Fatalf("failed to initialize uploader: %v", err)
	}

	metadata := app.VideoMetadata{
		Title:       titleVal,
		Description: descVal,
		Tags:        tags,
		CategoryID:  *categoryID,
	}

	videoID, err := uploader.UploadVideo(*videoPath, metadata)
	if err != nil {
		log.Fatalf("upload failed: %v", err)
	}

	log.Printf("Uploaded successfully! https://youtube.com/watch?v=%s", videoID)
}

func ensureFileExists(path string) error {
	info, err := os.Stat(path)
	if err != nil {
		return err
	}
	if info.IsDir() {
		return fmt.Errorf("path is a directory, expected file: %s", path)
	}
	return nil
}

func parseTags(raw string) []string {
	split := strings.Split(raw, ",")
	var tags []string
	for _, tag := range split {
		clean := strings.TrimSpace(tag)
		if clean != "" {
			tags = append(tags, clean)
		}
	}
	return tags
}

func defaultDescription(title, source string) string {
	builder := strings.Builder{}
	builder.WriteString(title)
	builder.WriteString("\n\n")
	if strings.TrimSpace(source) != "" {
		builder.WriteString("Source: ")
		builder.WriteString(source)
		builder.WriteString("\n\n")
	}
	builder.WriteString("Follow for daily tech updates!\n")
	builder.WriteString("#tech #ai #technology #shorts")
	return builder.String()
}
