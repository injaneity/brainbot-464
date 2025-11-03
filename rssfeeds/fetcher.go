package rssfeeds

import (
	"fmt"
	"time"

	types "brainbot/types"

	"github.com/mmcdole/gofeed"
)

// FetchFeed retrieves and parses an RSS/Atom feed, returning article metadata
func FetchFeed(feedURL string, maxCount int) ([]*types.Article, error) {
	parser := gofeed.NewParser()
	feed, err := parser.ParseURL(feedURL)
	if err != nil {
		return nil, fmt.Errorf("failed to fetch feed: %w", err)
	}

	count := min(len(feed.Items), maxCount)
	articles := make([]*types.Article, 0, count)

	for i := 0; i < count; i++ {
		item := feed.Items[i]
		// Always use a generated hash as the article ID (not the URL or GUID)
		var id string
		if item.Link != "" {
			id = GenerateID(item.Link)
		} else if item.GUID != "" {
			// Fallback: hash the GUID if link is missing
			id = GenerateID(item.GUID)
		} else if item.Title != "" {
			// Last resort: hash the title
			id = GenerateID(item.Title)
		}

		// Parse published date
		var publishedAt time.Time
		if item.PublishedParsed != nil {
			publishedAt = *item.PublishedParsed
		} else if item.UpdatedParsed != nil {
			publishedAt = *item.UpdatedParsed
		}

		// Extract author
		author := ""
		if item.Author != nil {
			author = item.Author.Name
		}

		// Extract categories
		categories := make([]string, len(item.Categories))
		copy(categories, item.Categories)

		// Get description/summary
		summary := item.Description
		if summary == "" {
			summary = item.Content
		}

		article := &types.Article{
			ID:          id,
			Title:       item.Title,
			URL:         item.Link,
			PublishedAt: publishedAt,
			FetchedAt:   time.Now(),
			Summary:     summary,
			Author:      author,
			Categories:  categories,
		}

		// Extract image if available
		if item.Image != nil {
			article.ImageURL = item.Image.URL
		}

		articles = append(articles, article)
	}

	return articles, nil
}

func min(a, b int) int {
	if a < b {
		return a
	}
	return b
}
