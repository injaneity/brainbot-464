package orchestrator

import (
	"brainbot/app"
	"brainbot/common"
	"brainbot/types"
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"log"
	"os"
	"regexp"
	"strings"
	"time"

	"github.com/joho/godotenv"
)

// RunOnce executes a single end-to-end cycle: fetch RSS, extract, deduplicate, optional S3 upload, summary.
func RunOnce(ctx context.Context) error {
	// Initialize logging
	log.SetOutput(os.Stderr)
	log.Println("=== BrainBot Orchestrator ===")

	// Load environment variables from .env if present (non-fatal if missing)
	_ = godotenv.Load()

	// Create app client
	apiURL := app.GetEnvOrDefault("API_URL", "http://localhost:8080")
	appClient := app.NewClient(apiURL)

	// Step 1: Fetch RSS feed using app client
	log.Println("Fetching RSS feed...")
	articles, err := appClient.FetchArticles(ctx, "", 0)
	if err != nil {
		return fmt.Errorf("failed to fetch articles: %w", err)
	}
	log.Printf("Fetched %d articles from feed", len(articles))

	successCount := 0
	for _, article := range articles {
		if article.ExtractionError == "" {
			successCount++
		}
	}
	log.Printf("Successfully extracted %d/%d articles", successCount, len(articles))

	// Step 2: Initialize optional S3 client (uploads are skipped if not configured)
	s3Client, s3Bucket, s3Prefix := initializeS3()

	// Step 3: Process articles through deduplication via API
	log.Println("Processing articles for deduplication...")
	results, err := appClient.ProcessArticles(ctx, articles)
	if err != nil {
		return fmt.Errorf("failed to process articles: %w", err)
	}

	// Log results
	for i, r := range results {
		if r.Status == "duplicate" && r.DeduplicationResult != nil {
			log.Printf("  [%d/%d] 🔄 DUPLICATE DETECTED (%.2f%% similar to %s)",
				i+1, len(results),
				r.DeduplicationResult.SimilarityScore*100,
				r.DeduplicationResult.MatchingID)
		} else if r.Status == "new" {
			log.Printf("  [%d/%d] ✅ NEW ARTICLE - Added to database", i+1, len(results))
		} else if r.Status == "failed" {
			log.Printf("  [%d/%d] ⚠️  Skipping - extraction failed: %s", i+1, len(results), r.Error)
		} else if r.Status == "error" {
			log.Printf("  [%d/%d] ❌ Error during deduplication: %v", i+1, len(results), r.Error)
		}
	}

	// Step 3b: Upload non-duplicate articles to S3 (without images) if configured
	if s3Client != nil && s3Bucket != "" {
		log.Printf("Uploading new articles to S3 bucket %q with prefix %q", s3Bucket, s3Prefix)
		uploaded := 0
		for i, r := range results {
			if r.Status != "new" || r.Article == nil {
				continue
			}
			// Short timeout per upload
			uctx, cancel := context.WithTimeout(ctx, 30*time.Second)
			err := uploadArticleToS3(uctx, s3Client, s3Bucket, s3Prefix, r.Article)
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

	// Step 4: Display results (summary only)
	displayResults(results, articles)

	log.Println("=== Orchestrator Run Complete ===")
	return nil
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

func displayResults(results []app.ArticleResult, articles []*types.Article) {
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
}
