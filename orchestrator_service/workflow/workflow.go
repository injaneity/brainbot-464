package workflow

import (
	"context"
	"fmt"
	"log"
	"orchestrator/state"
	"orchestrator/types"

	"github.com/joho/godotenv"
)

// Runner executes the complete workflow
type Runner struct {
	stateManager *state.Manager
}

// NewRunner creates a new workflow runner
func NewRunner(stateManager *state.Manager) *Runner {
	return &Runner{
		stateManager: stateManager,
	}
}

// Run executes the complete workflow
// This is called either by manual trigger (POST /start) or by cron job
func (r *Runner) Run(ctx context.Context) error {
	_ = godotenv.Load()

	// Step 1: Clear cache
	if err := r.clearCache(ctx); err != nil {
		r.stateManager.SetError(fmt.Errorf("clear cache: %w", err))
		return err
	}

	// Step 2: Fetch articles
	if err := r.fetchArticles(ctx); err != nil {
		r.stateManager.SetError(fmt.Errorf("fetch articles: %w", err))
		return err
	}

	// Step 3: Deduplicate
	if err := r.deduplicateArticles(ctx); err != nil {
		r.stateManager.SetError(fmt.Errorf("deduplicate: %w", err))
		return err
	}

	// Step 4: Send to generation service
	if err := r.sendGenerationRequest(ctx); err != nil {
		r.stateManager.SetError(fmt.Errorf("send generation: %w", err))
		return err
	}

	// Step 5: Wait for webhook (async - webhook handler will update state)
	r.stateManager.AddLog("Workflow initiated successfully, waiting for generation service callback via Kafka")
	return nil
}

// RunRefresh executes the workflow without clearing the cache
// This allows fetching new articles while keeping the history for deduplication
func (r *Runner) RunRefresh(ctx context.Context) error {
	_ = godotenv.Load()

	// Skip Step 1: Clear cache

	// Step 2: Fetch articles
	if err := r.fetchArticles(ctx); err != nil {
		r.stateManager.SetError(fmt.Errorf("fetch articles: %w", err))
		return err
	}

	// Step 3: Deduplicate
	if err := r.deduplicateArticles(ctx); err != nil {
		r.stateManager.SetError(fmt.Errorf("deduplicate: %w", err))
		return err
	}

	// Step 4: Send to generation service
	if err := r.sendGenerationRequest(ctx); err != nil {
		r.stateManager.SetError(fmt.Errorf("send generation: %w", err))
		return err
	}

	// Step 5: Wait for webhook (async - webhook handler will update state)
	r.stateManager.AddLog("Refresh workflow initiated successfully, waiting for generation service callback via Kafka")
	return nil
}

// clearCache clears the ChromaDB cache
func (r *Runner) clearCache(ctx context.Context) error {
	r.stateManager.SetState(types.StateClearing)
	r.stateManager.AddLog("Clearing ChromaDB cache...")

	client := r.stateManager.GetIngestionClient()
	err := client.ClearCache(ctx)
	if err != nil {
		return err
	}

	r.stateManager.AddLog("Cache cleared successfully")
	return nil
}

// fetchArticles fetches RSS articles
func (r *Runner) fetchArticles(ctx context.Context) error {
	r.stateManager.SetState(types.StateFetching)
	r.stateManager.AddLog("Fetching RSS feed...")

	client := r.stateManager.GetIngestionClient()
	articles, err := client.FetchArticles(ctx, "", 0)
	if err != nil {
		return err
	}

	r.stateManager.SetArticles(articles)
	r.stateManager.AddLog(fmt.Sprintf("Fetched %d articles", len(articles)))
	return nil
}

// deduplicateArticles processes articles for deduplication
func (r *Runner) deduplicateArticles(ctx context.Context) error {
	r.stateManager.SetState(types.StateDeduplicating)
	r.stateManager.AddLog("Deduplicating articles...")

	articles := r.stateManager.GetArticles()
	client := r.stateManager.GetIngestionClient()
	results, err := client.ProcessArticles(ctx, articles)
	if err != nil {
		return err
	}

	// Log individual results
	for _, res := range results {
		switch res.Status {
		case "new":
			log.Printf("Article %s: NEW - added to database", res.Article.Title)
		case "duplicate":
			log.Printf("Article %s: DUPLICATE (%.2f%% similar)",
				res.Article.Title, res.DeduplicationResult.SimilarityScore*100)
		case "failed":
			log.Printf("Article %s: FAILED extraction", res.Article.Title)
		case "error":
			log.Printf("Article %s: ERROR - %v", res.Article.Title, res.Error)
		}
	}

	r.stateManager.SetDedupResults(results)

	newCount := 0
	for _, res := range results {
		if res.Status == "new" {
			newCount++
		}
	}

	r.stateManager.AddLog(fmt.Sprintf("Found %d new articles", newCount))
	return nil
}

// sendGenerationRequest is implemented in generation.go
