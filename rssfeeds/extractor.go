package rssfeeds

import (
	"brainbot/types"
	"fmt"
	"log"
	"sync"
	"time"

	readability "github.com/go-shiori/go-readability"
)

const (
	WorkerCount      = 5
	extractorTimeout = 30 * time.Second
)

// ExtractAllContent fetches and extracts full content for all articles using a worker pool
func ExtractAllContent(articles []*types.Article) {
	var wg sync.WaitGroup
	articleChan := make(chan *types.Article, len(articles))

	// Start worker pool
	for i := 0; i < WorkerCount; i++ {
		go func(workerID int) {
			for article := range articleChan {
				if err := extractContent(article); err != nil {
					article.ExtractionError = err.Error()
					log.Printf("[Worker %d] Failed to extract %s: %v", workerID, article.URL, err)
				}
				wg.Done()
			}
		}(i)
	}

	// Queue articles for extraction
	for _, article := range articles {
		wg.Add(1)
		articleChan <- article
	}

	// Wait for all extractions to complete
	wg.Wait()
	close(articleChan)
}

// extractContent fetches and extracts full content for a single article
func extractContent(article *types.Article) error {
	if article.URL == "" {
		return fmt.Errorf("article URL is empty")
	}

	extractedArticle, err := readability.FromURL(article.URL, extractorTimeout)
	if err != nil {
		return fmt.Errorf("readability extraction failed: %w", err)
	}

	// Populate extracted content
	article.FullContent = extractedArticle.Content
	article.FullContentText = extractedArticle.TextContent
	article.Excerpt = extractedArticle.Excerpt

	// Use extracted image if not already set
	if article.ImageURL == "" {
		article.ImageURL = extractedArticle.Image
	}

	// Use extracted metadata if not already set
	if article.Author == "" {
		article.Author = extractedArticle.Byline
	}

	log.Printf("✓ Extracted: %s", article.Title)
	return nil
}
