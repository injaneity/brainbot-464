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
func (r *Runner) Run(ctx context.Context, feedPreset string) error {
	_ = godotenv.Load()

	// Step 1: Clear cache
	if err := r.clearCache(ctx); err != nil {
		r.stateManager.SetError(fmt.Errorf("clear cache: %w", err))
		return err
	}

	// Step 2: Fetch articles
	if err := r.fetchArticles(ctx, feedPreset); err != nil {
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

	if r.stateManager.GetState() == types.StateComplete {
		return nil
	}

	// Step 5: Wait for webhook (async - webhook handler will update state)
	r.stateManager.AddLog("Workflow initiated successfully, waiting for generation service callback via Kafka")
	return nil
}

// RunRefresh executes the workflow without clearing the cache
// This allows fetching new articles while keeping the history for deduplication
func (r *Runner) RunRefresh(ctx context.Context, feedPreset string) error {
	_ = godotenv.Load()

	// Skip Step 1: Clear cache

	// Step 2: Fetch articles
	if err := r.fetchArticles(ctx, feedPreset); err != nil {
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

	if r.stateManager.GetState() == types.StateComplete {
		return nil
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
func (r *Runner) fetchArticles(ctx context.Context, feedPreset string) error {
	r.stateManager.SetState(types.StateFetching)

	client := r.stateManager.GetIngestionClient()
	var allArticles []*types.Article

	presetsToFetch := []string{}
	if feedPreset != "" {
		presetsToFetch = append(presetsToFetch, feedPreset)
	} else {
		// Fetch all
		r.stateManager.AddLog("Fetching all RSS feeds...")
		presets, err := client.GetPresets(ctx)
		if err != nil {
			return err
		}
		for p := range presets {
			presetsToFetch = append(presetsToFetch, p)
		}
	}

	for _, p := range presetsToFetch {
		r.stateManager.AddLog(fmt.Sprintf("Fetching feed: %s...", p))
		articles, err := client.FetchArticles(ctx, p, 0)
		if err != nil {
			r.stateManager.AddLog(fmt.Sprintf("Error fetching %s: %v", p, err))
			continue
		}
		allArticles = append(allArticles, articles...)
	}

	r.stateManager.SetArticles(allArticles)
	r.stateManager.AddLog(fmt.Sprintf("Fetched total %d articles", len(allArticles)))
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
	dupCount := 0
	failCount := 0
	errCount := 0

	for _, res := range results {
		switch res.Status {
		case "new":
			newCount++
		case "duplicate":
			dupCount++
		case "failed":
			failCount++
		case "error":
			errCount++
		}
	}

	r.stateManager.AddLog(fmt.Sprintf("Results: %d new, %d duplicates, %d failed, %d errors", newCount, dupCount, failCount, errCount))
	return nil
}

// sendGenerationRequest is implemented in generation.go
