package main

import (
	"encoding/json"
	"log"
	"math/rand"
	"os"
	"path/filepath"

	"brainbot/creation_service/app"
	"brainbot/creation_service/app/config"
	"brainbot/creation_service/app/services"
)

// Test video generation WITHOUT uploading to YouTube
// Run from creation_service directory: go run scripts/test_video_generation.go
func main() {
	log.Println("🧪 Testing Video Generation (No Upload)")

	// Read your test JSON file from inputs/ folder
	jsonFile := "inputs/output_format.txt" // JSON data in .txt file
	data, err := os.ReadFile(jsonFile)
	if err != nil {
		log.Fatalf("❌ Failed to read JSON: %v", err)
	}

	var input app.VideoInput
	if err := json.Unmarshal(data, &input); err != nil {
		log.Fatalf("❌ Failed to parse JSON: %v", err)
	}

	log.Printf("✅ Loaded video input: UUID=%s", input.UUID)

	// Get background videos
	backgrounds, err := filepath.Glob(filepath.Join(config.BackgroundsDir, "*.mp4"))
	if err != nil {
		log.Fatalf("❌ Failed to find backgrounds: %v", err)
	}

	if len(backgrounds) == 0 {
		log.Fatalf("❌ No background videos found in %s/ - please add .mp4 files!", config.BackgroundsDir)
	}

	log.Printf("📹 Found %d background videos", len(backgrounds))

	// Pick random background
	backgroundVideo := backgrounds[rand.Intn(len(backgrounds))]
	log.Printf("🎨 Using background: %s", filepath.Base(backgroundVideo))

	// Create output directory if needed
	os.MkdirAll(config.OutputDir, 0755)

	// Generate video
	outputPath := filepath.Join(config.OutputDir, input.UUID+".mp4")
	log.Printf("🎥 Creating video: %s", outputPath)

	if err := services.CreateVideo(input, backgroundVideo, outputPath); err != nil {
		log.Fatalf("❌ Video creation failed: %v", err)
	}

	log.Printf("🎉 SUCCESS! Video created at: %s", outputPath)
	log.Println("▶️  You can now play this video to verify it works!")
}
