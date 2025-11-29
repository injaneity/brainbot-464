package client

import (
	"brainbot/ingestion_service/rssfeeds"
	"brainbot/ingestion_service/types"
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"time"
)

// Client represents the BrainBot application client
type Client struct {
	baseURL    string
	httpClient *http.Client
}

// NewClient creates a new application client
func NewClient(baseURL string) *Client {
	if baseURL == "" {
		baseURL = "http://localhost:8080"
	}
	return &Client{
		baseURL:    baseURL,
		httpClient: &http.Client{Timeout: 30 * time.Second},
	}
}

// ArticleResult represents the processing result for a single article
type ArticleResult struct {
	Article             *types.Article       `json:"article"`
	Status              string               `json:"status"` // "new", "duplicate", "failed", "error"
	DeduplicationResult *DeduplicationResult `json:"deduplication_result,omitempty"`
	Error               string               `json:"error,omitempty"`
}

// DeduplicationResult contains the result of deduplication check
type DeduplicationResult struct {
	IsDuplicate     bool      `json:"is_duplicate"`
	MatchingID      string    `json:"matching_id,omitempty"`
	SimilarityScore float32   `json:"similarity_score,omitempty"`
	CheckedAt       time.Time `json:"checked_at"`
}

// FetchArticles fetches articles from RSS feed
func (c *Client) FetchArticles(ctx context.Context, feedPreset string, count int) ([]*types.Article, error) {
	if feedPreset == "" {
		feedPreset = rssfeeds.DefaultFeedPreset
	}
	if count == 0 {
		count = rssfeeds.DefaultCount
	}

	feedURL := rssfeeds.ResolveFeedURL(feedPreset)
	articles, err := rssfeeds.FetchFeed(feedURL, count)
	if err != nil {
		return nil, fmt.Errorf("failed to fetch articles: %w", err)
	}

	// Extract full content for all articles
	rssfeeds.ExtractAllContent(articles)

	return articles, nil
}

// CheckDuplicate checks if an article is a duplicate via the API
func (c *Client) CheckDuplicate(ctx context.Context, article *types.Article) (*DeduplicationResult, error) {
	url := fmt.Sprintf("%s/api/deduplication/check", c.baseURL)

	payload := map[string]interface{}{
		"article": article,
	}

	jsonData, err := json.Marshal(payload)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal request: %w", err)
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, url, bytes.NewBuffer(jsonData))
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %w", err)
	}
	req.Header.Set("Content-Type", "application/json")

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("failed to send request: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return nil, fmt.Errorf("API returned %d: %s", resp.StatusCode, string(body))
	}

	var result DeduplicationResult
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return nil, fmt.Errorf("failed to decode response: %w", err)
	}

	return &result, nil
}

// AddArticle adds an article to the deduplication database via the API
func (c *Client) AddArticle(ctx context.Context, article *types.Article) error {
	url := fmt.Sprintf("%s/api/deduplication/add", c.baseURL)

	payload := map[string]interface{}{
		"article": article,
	}

	jsonData, err := json.Marshal(payload)
	if err != nil {
		return fmt.Errorf("failed to marshal request: %w", err)
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, url, bytes.NewBuffer(jsonData))
	if err != nil {
		return fmt.Errorf("failed to create request: %w", err)
	}
	req.Header.Set("Content-Type", "application/json")

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return fmt.Errorf("failed to send request: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return fmt.Errorf("API returned %d: %s", resp.StatusCode, string(body))
	}

	return nil
}

// ProcessArticle processes an article (checks for duplicates and adds if new) via the API
func (c *Client) ProcessArticle(ctx context.Context, article *types.Article) (*ArticleResult, error) {
	url := fmt.Sprintf("%s/api/deduplication/process", c.baseURL)

	payload := map[string]interface{}{
		"article": article,
	}

	jsonData, err := json.Marshal(payload)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal request: %w", err)
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, url, bytes.NewBuffer(jsonData))
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %w", err)
	}
	req.Header.Set("Content-Type", "application/json")

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("failed to send request: %w", err)
	}
	defer resp.Body.Close()

	body, _ := io.ReadAll(resp.Body)

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("API returned %d: %s", resp.StatusCode, string(body))
	}

	var result struct {
		Status              string               `json:"status"`
		DeduplicationResult *DeduplicationResult `json:"deduplication_result,omitempty"`
		Error               string               `json:"error,omitempty"`
	}

	if err := json.Unmarshal(body, &result); err != nil {
		return nil, fmt.Errorf("failed to decode response: %w", err)
	}

	return &ArticleResult{
		Article:             article,
		Status:              result.Status,
		DeduplicationResult: result.DeduplicationResult,
		Error:               result.Error,
	}, nil
}

// ProcessArticles processes multiple articles
func (c *Client) ProcessArticles(ctx context.Context, articles []*types.Article) ([]ArticleResult, error) {
	results := make([]ArticleResult, 0, len(articles))

	for _, article := range articles {
		// Skip articles that failed extraction
		if article.ExtractionError != "" {
			results = append(results, ArticleResult{
				Article: article,
				Status:  "failed",
				Error:   article.ExtractionError,
			})
			continue
		}

		result, err := c.ProcessArticle(ctx, article)
		if err != nil {
			results = append(results, ArticleResult{
				Article: article,
				Status:  "error",
				Error:   err.Error(),
			})
			continue
		}

		results = append(results, *result)
	}

	return results, nil
}

// ClearCache clears the deduplication cache via the API
func (c *Client) ClearCache(ctx context.Context) error {
	url := fmt.Sprintf("%s/api/deduplication/clear", c.baseURL)

	req, err := http.NewRequestWithContext(ctx, http.MethodDelete, url, nil)
	if err != nil {
		return fmt.Errorf("failed to create request: %w", err)
	}

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return fmt.Errorf("failed to send request: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return fmt.Errorf("API returned %d: %s", resp.StatusCode, string(body))
	}

	return nil
}

// GetCount gets the number of documents in the deduplication database via the API
func (c *Client) GetCount(ctx context.Context) (int, error) {
	url := fmt.Sprintf("%s/api/deduplication/count", c.baseURL)

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return 0, fmt.Errorf("failed to create request: %w", err)
	}

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return 0, fmt.Errorf("failed to send request: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return 0, fmt.Errorf("API returned %d: %s", resp.StatusCode, string(body))
	}

	var result struct {
		Count int `json:"count"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return 0, fmt.Errorf("failed to decode response: %w", err)
	}

	return result.Count, nil
}

// TriggerRSSRefresh triggers the RSS refresh orchestrator via the API
func (c *Client) TriggerRSSRefresh(ctx context.Context) error {
	url := fmt.Sprintf("%s/api/rss/refresh", c.baseURL)

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, url, nil)
	if err != nil {
		return fmt.Errorf("failed to create request: %w", err)
	}

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return fmt.Errorf("failed to send request: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusAccepted {
		body, _ := io.ReadAll(resp.Body)
		return fmt.Errorf("API returned %d: %s", resp.StatusCode, string(body))
	}

	return nil
}

// GetEnvOrDefault returns the value of an environment variable or a default value
func GetEnvOrDefault(key, defaultVal string) string {
	if val := os.Getenv(key); val != "" {
		return val
	}
	return defaultVal
}
