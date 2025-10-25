package main

import (
	"brainbot/deduplication"
	"brainbot/rssfeeds"
	"brainbot/types"
	"encoding/json"
	"log"
	"os"
	"time"

	"github.com/joho/godotenv"
)

func main() {
	// Initialize logging
	log.SetOutput(os.Stderr)
	log.Println("=== BrainBot RSS Feed Deduplicator ===")

	// Load environment variables from .env if present (non-fatal if missing)
	if err := godotenv.Load(); err == nil {
		log.Println("Loaded environment variables from .env")
	}

	// Step 1: Fetch RSS feed using existing rssfeeds package
	feedURL := rssfeeds.ResolveFeedURL(rssfeeds.DefaultFeedPreset)
	log.Printf("Fetching RSS feed: %s", feedURL)

	articles, err := rssfeeds.FetchFeed(feedURL, rssfeeds.DefaultCount)
	if err != nil {
		log.Fatalf("Failed to fetch articles: %v", err)
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

	// Step 3: Process articles through deduplication
	log.Println("Processing articles for deduplication...")
	results := processArticles(articles, deduplicator)

	// Step 4: Display results
	displayResults(results, articles)

	log.Println("=== Processing Complete ===")
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

	// Output detailed results as JSON to stdout
	output := ProcessingOutput{
		ProcessedAt: time.Now(),
		Results:     results,
	}

	encoder := json.NewEncoder(os.Stdout)
	encoder.SetIndent("", "  ")
	if err := encoder.Encode(output); err != nil {
		log.Fatalf("Failed to encode JSON output: %v", err)
	}
}

// ArticleResult represents the processing result for a single article
type ArticleResult struct {
	Article             *types.Article                     `json:"article"`
	Status              string                             `json:"status"` // "new", "duplicate", "failed", "error"
	DeduplicationResult *deduplication.DeduplicationResult `json:"deduplication_result,omitempty"`
	Error               string                             `json:"error,omitempty"`
}

// ProcessingOutput is the complete output structure
type ProcessingOutput struct {
	ProcessedAt time.Time       `json:"processed_at"`
	Results     []ArticleResult `json:"results"`
}
