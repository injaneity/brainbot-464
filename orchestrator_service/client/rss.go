package client

import (
	"brainbot/shared/rss"
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"orchestrator/types"
)

type FetchRequest struct {
	FeedPreset string `json:"feed_preset"`
	Count      int    `json:"count"`
}

// FetchArticles fetches articles from RSS feed via ingestion service
func (c *IngestionClient) FetchArticles(ctx context.Context, feedPreset string, count int) ([]*types.Article, error) {
	reqBody := FetchRequest{
		FeedPreset: feedPreset,
		Count:      count,
	}

	jsonBody, err := json.Marshal(reqBody)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal request: %w", err)
	}

	req, err := http.NewRequestWithContext(ctx, "POST", c.baseURL+"/fetch", bytes.NewBuffer(jsonBody))
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
		return nil, fmt.Errorf("ingestion service returned status: %d", resp.StatusCode)
	}

	var articles []*types.Article
	if err := json.NewDecoder(resp.Body).Decode(&articles); err != nil {
		return nil, fmt.Errorf("failed to decode response: %w", err)
	}

	return articles, nil
}

// GetPresets fetches available RSS feed presets
func (c *IngestionClient) GetPresets(ctx context.Context) (map[string]rss.FeedConfig, error) {
	req, err := http.NewRequestWithContext(ctx, "GET", c.baseURL+"/presets", nil)
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %w", err)
	}

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("failed to send request: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("ingestion service returned status: %d", resp.StatusCode)
	}

	var presets map[string]rss.FeedConfig
	if err := json.NewDecoder(resp.Body).Decode(&presets); err != nil {
		return nil, fmt.Errorf("failed to decode response: %w", err)
	}

	return presets, nil
}
