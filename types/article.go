package types

import (
	"crypto/sha256"
	"encoding/hex"
	"time"
)

// Article represents a single article with metadata and extracted content
type Article struct {
	ID              string    `json:"id"`
	Title           string    `json:"title"`
	URL             string    `json:"url"`
	PublishedAt     time.Time `json:"published_at"`
	FetchedAt       time.Time `json:"fetched_at"`
	Summary         string    `json:"summary"`
	Author          string    `json:"author,omitempty"`
	Categories      []string  `json:"categories,omitempty"`
	FullContent     string    `json:"full_content"`
	FullContentText string    `json:"full_content_text"`
	Excerpt         string    `json:"excerpt,omitempty"`
	ImageURL        string    `json:"image_url,omitempty"`
	ExtractionError string    `json:"extraction_error,omitempty"`
}

// FeedResult is the top-level wrapper for JSON output
type FeedResult struct {
	FeedURL      string     `json:"feed_url"`
	FetchedAt    time.Time  `json:"fetched_at"`
	ArticleCount int        `json:"article_count"`
	Articles     []*Article `json:"articles"`
}

// GenerateID creates a unique ID from URL
func GenerateID(url string) string {
	hash := sha256.Sum256([]byte(url))
	return hex.EncodeToString(hash[:])[:16]
}
