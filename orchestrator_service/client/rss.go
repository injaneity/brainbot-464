package client

import (
	"context"
	"fmt"
	"orchestrator/types"

	"brainbot/ingestion_service/rssfeeds"
)

// FetchArticles fetches articles from RSS feed
func (c *IngestionClient) FetchArticles(ctx context.Context, feedPreset string, count int) ([]*types.Article, error) {
	if feedPreset == "" {
		feedPreset = rssfeeds.DefaultFeedPreset
	}
	if count == 0 {
		count = rssfeeds.DefaultCount
	}

	feedURL := rssfeeds.ResolveFeedURL(feedPreset)
	ingestArticles, err := rssfeeds.FetchFeed(feedURL, count)
	if err != nil {
		return nil, fmt.Errorf("failed to fetch articles: %w", err)
	}

	// Extract full content for all articles
	rssfeeds.ExtractAllContent(ingestArticles)

	// Convert ingestion service articles to orchestrator types
	articles := make([]*types.Article, len(ingestArticles))
	for i, a := range ingestArticles {
		articles[i] = &types.Article{
			ID:              a.ID,
			Title:           a.Title,
			URL:             a.URL,
			PublishedAt:     a.PublishedAt,
			FetchedAt:       a.FetchedAt,
			Summary:         a.Summary,
			Author:          a.Author,
			Categories:      a.Categories,
			FullContent:     a.FullContent,
			FullContentText: a.FullContentText,
			Excerpt:         a.Excerpt,
			ImageURL:        a.ImageURL,
			ExtractionError: a.ExtractionError,
		}
	}

	return articles, nil
}
