package types

import "time"

// Article represents a single article with metadata and extracted content
// This is imported from the ingestion service types
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
