package main

import (
    "encoding/json"
    "fmt"
    "log"
    "math/rand"
    "os"
    "path/filepath"
    "sync"
    "time"
    
    "github.com/yourteam/brainbot/video"
)

func runVideoUploader() {
    log.Println("🤖 Video Uploader Starting...")
    
    uploader, err := video.NewUploader("service-account.json")
    if err != nil {
        log.Fatalf("❌ Failed to initialize YouTube uploader: %v", err)
    }
    log.Println("✅ YouTube client initialized")
    
    backgrounds, err := getBackgroundVideos("backgrounds")
    if err != nil {
        log.Fatalf("❌ Failed to load background videos: %v", err)
    }
    log.Printf("📹 Found %d background videos", len(backgrounds))
    
    jsonFiles, err := filepath.Glob("input/*.json")
    if err != nil {
        log.Fatalf("❌ Failed to read input directory: %v", err)
    }
    
    if len(jsonFiles) == 0 {
        log.Println("⚠️  No JSON files found in input/ directory")
        return
    }
    
    log.Printf("📄 Found %d videos to process", len(jsonFiles))
    
    var wg sync.WaitGroup
    semaphore := make(chan struct{}, 2)
    
    for i, jsonFile := range jsonFiles {
        wg.Add(1)
        
        go func(idx int, file string) {
            defer wg.Done()
            
            semaphore <- struct{}{}
            defer func() { <-semaphore }()
            
            if err := processVideo(file, backgrounds, uploader, idx+1, len(jsonFiles)); err != nil {
                log.Printf("❌ Failed to process %s: %v", file, err)
            }
            
            if idx < len(jsonFiles)-1 {
                time.Sleep(2 * time.Minute)
            }
        }(i, jsonFile)
    }
    
    wg.Wait()
    log.Println("🎉 All videos processed!")
}

func processVideo(jsonFile string, backgrounds []string, uploader *video.Uploader, current, total int) error {
    log.Printf("🎬 [%d/%d] Processing: %s", current, total, filepath.Base(jsonFile))
    
    data, err := os.ReadFile(jsonFile)
    if err != nil {
        return fmt.Errorf("failed to read JSON: %w", err)
    }
    
    var input video.VideoInput
    if err := json.Unmarshal(data, &input); err != nil {
        return fmt.Errorf("failed to parse JSON: %w", err)
    }
    
    if input.Status != "success" {
        return fmt.Errorf("input status is not success: %s", input.Status)
    }
    
    backgroundVideo := backgrounds[rand.Intn(len(backgrounds))]
    log.Printf("  🎨 Using background: %s", filepath.Base(backgroundVideo))
    
    // Keep video in output/ folder (not /tmp)
    outputPath := fmt.Sprintf("output/%s.mp4", input.UUID)
    log.Printf("  🎥 Creating video...")
    if err := video.CreateVideo(input, backgroundVideo, outputPath); err != nil {
        return fmt.Errorf("video creation failed: %w", err)
    }
    log.Printf("  ✅ Video created: %s", outputPath)
    
    articleTitle := generateTitleFromSubtitles(input.SubtitleTimestamps)
    metadata := video.GenerateMetadata(input, articleTitle, "https://example.com/article")
    
    log.Printf("  📤 Uploading to YouTube...")
    videoID, err := uploader.UploadVideo(outputPath, metadata)
    if err != nil {
        return fmt.Errorf("upload failed: %w", err)
    }
    
    log.Printf("  🎉 SUCCESS! Video ID: %s", videoID)
    
    // Only delete input JSON, keep the video
    os.Remove(jsonFile)
    // os.Remove(outputPath)  this is to remove the actual .mp4 file, but ill comment out for now
    
    return nil
}

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

func generateTitleFromSubtitles(timestamps []video.SubtitleTimestamp) string {
    title := ""
    wordCount := 0
    maxWords := 10
    
    for _, ts := range timestamps {
        title += ts.Text + " "
        wordCount++
        if wordCount >= maxWords {
            break
        }
    }
    
    if len(title) > 100 {
        title = title[:97] + "..."
    }
    
    return title
}