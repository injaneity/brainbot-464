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
/*
Brainrot Video Content Mill - Video Generation & Upload Service

ARCHITECTURE OVERVIEW:
======================
This service is part of a multi-component system that creates short-form video content:

1. RSS Feed Aggregation (handled by other team) 
   └─> Scrapes interesting articles from configured RSS feeds

2. Script Generation (handled by other team)
   └─> Converts articles into narrated scripts with timestamps
   └─> Generates voiceover audio files
   └─> Outputs: JSON with voiceover URL + subtitle timestamps

3. Video Generation & Upload (THIS SERVICE)
   └─> Receives JSON from script generation
   └─> Downloads voiceover audio
   └─> Overlays subtitles on background video
   └─> Uploads final video to YouTube as Short
   └─> Cleans up temporary files

OPERATIONAL MODES:
==================
- API Mode (default): HTTP server accepts JSON payloads for real-time processing
- Batch Mode: Processes multiple JSON files from input/ directory

WORKFLOW:
=========
1. Receive video generation request (via API or file)
2. Validate input JSON structure and status
3. Select random background video from pool
4. Download voiceover audio from provided URL
5. Generate SRT subtitle file with timestamps
6. Compose final video using FFmpeg:
   - Background video (9:16 aspect ratio)
   - Voiceover audio overlay
   - Subtitle overlay with styling
   - Max duration: 3 minutes
   - End padding: 0.5 seconds
7. Upload to YouTube with auto-generated metadata
8. Clean up temporary files

API ENDPOINTS:
==============
- POST /api/process-video : Process single video from JSON payload
- GET  /health           : Health check endpoint

CONFIGURATION:
==============
All constants defined in config/constants.go:
- Video dimensions, codecs, bitrates
- Max concurrent processing
- Batch delays
- Directory paths
*/

package main

import (
	"flag"
	"log"
	"net/http"
	"os"

	"brainbot/api"
	"brainbot/config"
	"brainbot/processor"
)

const (
	// ServiceAccountPath is the path to YouTube API service account credentials
	ServiceAccountPath = "service-account.json"
	
	// DefaultAPIPort is the default port for the HTTP API server
	DefaultAPIPort = ":8080"
)

func main() {
	// Command-line flags
	batchMode := flag.Bool("batch", false, "Run in batch mode (process files from input/ directory)")
	apiPort := flag.String("port", DefaultAPIPort, "API server port (e.g., :8080)")
	flag.Parse()

	log.Println("🤖 Brainrot Video Content Mill - Starting...")

	// Initialize video processor
	proc, err := processor.NewVideoProcessor(ServiceAccountPath, config.BackgroundsDir)
	if err != nil {
		log.Fatalf("❌ Failed to initialize processor: %v", err)
	}

	if *batchMode {
		// Batch mode: Process all files in input/ directory
		log.Println("📁 Running in BATCH mode")
		if err := proc.ProcessFromDirectory(config.InputDir); err != nil {
			log.Fatalf("❌ Batch processing failed: %v", err)
		}
		os.Exit(0)
	}

	// API mode: Start HTTP server
	log.Println("🌐 Running in API mode")
	
	apiServer := api.NewServer(proc)
	mux := apiServer.SetupRoutes()

	log.Printf("🚀 API Server listening on %s", *apiPort)
	log.Println("📌 Endpoints:")
	log.Println("   POST /api/process-video  - Process video from JSON")
	log.Println("   GET  /health             - Health check")

	if err := http.ListenAndServe(*apiPort, mux); err != nil {
		log.Fatalf("❌ Server failed: %v", err)
	}
}


//make it own microservice with api endpoint, let them handle api calling


