package types

import "time"

// State represents the orchestrator state machine
type State string

const (
	StateIdle          State = "idle"
	StateClearing      State = "clearing"
	StateFetching      State = "fetching"
	StateDeduplicating State = "deduplicating"
	StateSending       State = "sending"
	StateWaiting       State = "waiting"
	StateComplete      State = "complete"
	StateError         State = "error"
)

// LogEntry represents a single log line with timestamp
type LogEntry struct {
	Timestamp time.Time `json:"timestamp"`
	Message   string    `json:"message"`
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

// StatusResponse is the JSON response for GET /api/status
type StatusResponse struct {
	State          State           `json:"state"`
	Logs           []LogEntry      `json:"logs"`
	ArticleCount   int             `json:"article_count"`
	NewCount       int             `json:"new_count"`
	DuplicateCount int             `json:"duplicate_count"`
	GenerationUUID string          `json:"generation_uuid,omitempty"`
	WebhookPayload *WebhookPayload `json:"webhook_payload,omitempty"`
	Error          string          `json:"error,omitempty"`
}
