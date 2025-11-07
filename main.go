package main

import (
	"log"
	"net/http"
	"os"

	"brainbot/api"

	"github.com/joho/godotenv"
)

func main() {
	// Load environment variables from .env if present (non-fatal if missing)
	_ = godotenv.Load()

	addr := ":8080"
	if v := os.Getenv("PORT"); v != "" {
		addr = ":" + v
	}

	r := api.NewRouter()
	log.Printf("Starting API server on %s", addr)
	log.Println("API endpoints available:")
	log.Println("  GET  /api/health")
	log.Println("  POST /api/deduplication/check")
	log.Println("  POST /api/deduplication/add")
	log.Println("  POST /api/deduplication/process")
	log.Println("  GET  /api/deduplication/count")
	log.Println("  DELETE /api/deduplication/clear")
	
	if err := http.ListenAndServe(addr, r); err != nil {
		log.Fatalf("server error: %v", err)
	}
}

/*
	r := api.NewRouter()	feedURL := rssfeeds.ResolveFeedURL(rssfeeds.DefaultFeedPreset)

	log.Printf("Starting API server on %s", addr)	log.Printf("Fetching RSS feed: %s", feedURL)

	if err := http.ListenAndServe(addr, r); err != nil {

		log.Fatalf("server error: %v", err)	articles, err := rssfeeds.FetchFeed(feedURL, rssfeeds.DefaultCount)

	}	if err != nil {

}		log.Fatalf("Failed to fetch articles: %v", err)

	}
	log.Printf("Fetched %d articles from feed", len(articles))

	// Extract full content for all articles
	log.Printf("Extracting full content using %d workers...", rssfeeds.WorkerCount)
	rssfeeds.ExtractAllContent(articles)

	successCount := 0
	for _, article := range articles {
		if article.ExtractionError == "" {
			successCount++
		}
	}
	log.Printf("Successfully extracted %d/%d articles", successCount, len(articles))

	// Step 2: Initialize deduplicator
	log.Println("Initializing deduplication service...")
	deduplicator, err := initializeDeduplicator()
	if err != nil {
		log.Fatalf("Failed to initialize deduplicator: %v", err)
	}
	defer deduplicator.Close()

	log.Println("Deduplicator initialized successfully")
	log.Printf("Similarity threshold: %.2f%%", deduplication.SimilarityThreshold*100)

	// Step 2b: Initialize optional S3 client (uploads are skipped if not configured)
	s3Client, s3Bucket, s3Prefix := initializeS3()

	// Step 3: Process articles through deduplication
	log.Println("Processing articles for deduplication...")
	results := processArticles(articles, deduplicator)

	// Step 3b: Upload non-duplicate articles to S3 (without images) if configured
	if s3Client != nil && s3Bucket != "" {
		log.Printf("Uploading new articles to S3 bucket %q with prefix %q", s3Bucket, s3Prefix)
		uploaded := 0
		for i, r := range results {
			if r.Status != "new" || r.Article == nil {
				continue
			}
			ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
			err := uploadArticleToS3(ctx, s3Client, s3Bucket, s3Prefix, r.Article)
			cancel()
			if err != nil {
				log.Printf("  [%d] S3 upload failed for %s: %v", i+1, r.Article.ID, err)
				continue
			}
			uploaded++
		}
		log.Printf("S3 uploads complete: %d item(s)", uploaded)
	} else {
		log.Printf("S3 not configured; skipping uploads")
	}

	// Step 4: Display results
	displayResults(results, articles) // Summary logs only

	log.Println("=== Processing Complete ===")
}

// initializeS3 returns an S3 client and target bucket/prefix if configured via env.
// Required: S3_BUCKET. Optional: S3_REGION, S3_PROFILE, S3_PREFIX, S3_USE_PATH_STYLE=true
func initializeS3() (*common.S3, string, string) {
	bucket := strings.TrimSpace(os.Getenv("S3_BUCKET"))
	if bucket == "" {
		return nil, "", ""
	}

	cfg := common.S3Config{
		Region:       strings.TrimSpace(os.Getenv("S3_REGION")),
		Profile:      strings.TrimSpace(os.Getenv("S3_PROFILE")),
		UsePathStyle: strings.EqualFold(strings.TrimSpace(os.Getenv("S3_USE_PATH_STYLE")), "true"),
	}
	client, err := common.NewS3(context.Background(), cfg)
	if err != nil {
		log.Printf("Warning: failed to init S3 client: %v (uploads disabled)", err)
		return nil, "", ""
	}

	prefix := strings.TrimSpace(os.Getenv("S3_PREFIX"))
	if prefix != "" {
		prefix = strings.Trim(prefix, "/") + "/"
	}
	return client, bucket, prefix
}

// uploadArticleToS3 writes a sanitized JSON record of the article (without images) to S3.
func uploadArticleToS3(ctx context.Context, s3c *common.S3, bucket, prefix string, a *types.Article) error {
	if a == nil {
		return nil
	}

	// Build sanitized payload (remove images + strip <img> from HTML)
	payload := map[string]interface{}{
		"id":           a.ID,
		"title":        a.Title,
		"url":          a.URL,
		"published_at": a.PublishedAt,
		"fetched_at":   a.FetchedAt,
		"summary":      a.Summary,
		"author":       a.Author,
		"categories":   a.Categories,
		"excerpt":      a.Excerpt,
		"full_content": stripImagesFromHTML(a.FullContent),
		"content_text": a.FullContentText,
	}

	b, err := json.MarshalIndent(payload, "", "  ")
	if err != nil {
		return err
	}

	key := prefix + "articles/" + a.ID + ".json"
	return s3c.Put(ctx, bucket, key, bytes.NewReader(b), "application/json", "public, max-age=300", "")
}

var imgTagRe = regexp.MustCompile(`(?i)<img\b[^>]*>`)

func stripImagesFromHTML(html string) string {
	if strings.TrimSpace(html) == "" {
		return html
	}
	return imgTagRe.ReplaceAllString(html, "")
}

func initializeDeduplicator() (*deduplication.Deduplicator, error) {
	chromaConfig := deduplication.ChromaConfig{
		Host:           "localhost",
		Port:           8000,
		CollectionName: "brainbot_articles",
		EmbeddingModel: "",
	}

	deduplicatorConfig := deduplication.DeduplicatorConfig{
		ChromaConfig:        chromaConfig,
		SimilarityThreshold: 0,
		MaxSearchResults:    0,
	}

	return deduplication.NewDeduplicator(deduplicatorConfig)
}

func processArticles(articles []*types.Article, deduplicator *deduplication.Deduplicator) []ArticleResult {
	results := make([]ArticleResult, 0, len(articles))

	for i, article := range articles {
		log.Printf("[%d/%d] Processing: %s", i+1, len(articles), article.Title)

		// Skip articles that failed extraction
		if article.ExtractionError != "" {
			log.Printf("  ⚠️  Skipping - extraction failed: %s", article.ExtractionError)
			results = append(results, ArticleResult{
				Article: article,
				Status:  "failed",
				Error:   article.ExtractionError,
			})
			continue
		}

		// Process article through deduplicator
		dedupResult, err := deduplicator.ProcessArticle(article)
		if err != nil {
			log.Printf("  ❌ Error during deduplication: %v", err)
			results = append(results, ArticleResult{
				Article: article,
				Status:  "error",
				Error:   err.Error(),
			})
			continue
		}

		// Record result based on whether duplicate was found
		if dedupResult.IsDuplicate {
			log.Printf("  🔄 DUPLICATE DETECTED (%.2f%% similar to %s)",
				dedupResult.SimilarityScore*100,
				dedupResult.MatchingID)
			results = append(results, ArticleResult{
				Article:             article,
				Status:              "duplicate",
				DeduplicationResult: dedupResult,
			})
		} else {
			log.Printf("  ✅ NEW ARTICLE - Added to database")
			results = append(results, ArticleResult{
				Article:             article,
				Status:              "new",
				DeduplicationResult: dedupResult,
			})
		}
	}

	return results
}

func displayResults(results []ArticleResult, articles []*types.Article) {
	totalArticles := len(articles)
	newArticles := 0
	duplicateArticles := 0
	failedArticles := 0

	// Count results by status
	for _, result := range results {
		switch result.Status {
		case "new":
			newArticles++
		case "duplicate":
			duplicateArticles++
		case "failed", "error":
			failedArticles++
		}
	}

	// Print summary to stderr
	log.Println("\n=== Deduplication Summary ===")
	log.Printf("Total Articles:     %d", totalArticles)
	log.Printf("New Articles:       %d", newArticles)
	log.Printf("Duplicate Articles: %d", duplicateArticles)
	log.Printf("Failed Articles:    %d", failedArticles)
	log.Println("=============================")

	// Removed detailed JSON output to avoid printing article metadata
}

// ArticleResult represents the processing result for a single article
type ArticleResult struct {
	Article             *types.Article                     `json:"article"`
	Status              string                             `json:"status"` // "new", "duplicate", "failed", "error"
	DeduplicationResult *deduplication.DeduplicationResult `json:"deduplication_result,omitempty"`
	Error               string                             `json:"error,omitempty"`
}

// Removed detailed JSON output types to avoid printing article metadata
*/
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