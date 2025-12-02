package client

import (
	"brainbot/ingestion_service/types"
	"context"
	"net/http"
)

// CheckDuplicate checks if an article is a duplicate via the API
func (c *Client) CheckDuplicate(ctx context.Context, article *types.Article) (*DeduplicationResult, error) {
	payload := map[string]interface{}{
		"article": article,
	}

	var result DeduplicationResult
	if err := c.doJSONRequest(ctx, http.MethodPost, "/api/deduplication/check", payload, &result); err != nil {
		return nil, err
	}

	return &result, nil
}

// AddArticle adds an article to the deduplication database via the API
func (c *Client) AddArticle(ctx context.Context, article *types.Article) error {
	payload := map[string]interface{}{
		"article": article,
	}

	return c.doJSONRequest(ctx, http.MethodPost, "/api/deduplication/add", payload, nil)
}

// ProcessArticle processes an article (checks for duplicates and adds if new) via the API
func (c *Client) ProcessArticle(ctx context.Context, article *types.Article) (*ArticleResult, error) {
	payload := map[string]interface{}{
		"article": article,
	}

	var result struct {
		Status              string               `json:"status"`
		DeduplicationResult *DeduplicationResult `json:"deduplication_result,omitempty"`
		Error               string               `json:"error,omitempty"`
	}

	if err := c.doJSONRequest(ctx, http.MethodPost, "/api/deduplication/process", payload, &result); err != nil {
		return nil, err
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
	return c.doJSONRequest(ctx, http.MethodDelete, "/api/deduplication/clear", nil, nil)
}

// GetCount gets the number of documents in the deduplication database via the API
func (c *Client) GetCount(ctx context.Context) (int, error) {
	var result struct {
		Count int `json:"count"`
	}

	if err := c.doJSONRequest(ctx, http.MethodGet, "/api/deduplication/count", nil, &result); err != nil {
		return 0, err
	}

	return result.Count, nil
}
