package tui

import (
	"brainbot/demo/client"
	"brainbot/ingestion_service/types"
)

// Messages for the tea program
// These represent events that flow through the Update function

// FetchCompleteMsg is sent when article fetching completes
type FetchCompleteMsg struct {
	Articles []*types.Article
	Err      error
}

// CacheClearedMsg is sent when cache clearing completes
type CacheClearedMsg struct {
	Err error
}

// DeduplicationCompleteMsg is sent when deduplication completes
type DeduplicationCompleteMsg struct {
	Results []client.ArticleResult
	Err     error
}

// GenerationRequestSentMsg is sent when generation request is sent
type GenerationRequestSentMsg struct {
	UUID string
	Err  error
}

// WebhookReceivedMsg is sent when webhook payload is received
type WebhookReceivedMsg struct {
	Payload WebhookPayload
}

// ErrorMsg is sent when an error occurs
type ErrorMsg struct {
	Err error
}

// WebhookPayload represents the generation service response
type WebhookPayload struct {
	UUID               string                   `json:"uuid"`
	Voiceover          string                   `json:"voiceover"`
	SubtitleTimestamps []map[string]interface{} `json:"subtitle_timestamps"`
	ResourceTimestamps map[string]interface{}   `json:"resource_timestamps"`
	Status             string                   `json:"status"`
	Error              *string                  `json:"error,omitempty"`
	Timings            map[string]float64       `json:"timings,omitempty"`
}
