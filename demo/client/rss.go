package client

import (
	"brainbot/ingestion_service/rssfeeds"
	"brainbot/ingestion_service/types"
	"context"
	"fmt"
	"net/http"
)

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

// TriggerRSSRefresh triggers the RSS refresh orchestrator via the API
func (c *Client) TriggerRSSRefresh(ctx context.Context) error {
	return c.doJSONRequest(ctx, http.MethodPost, "/api/rss/refresh", nil, nil)
}
