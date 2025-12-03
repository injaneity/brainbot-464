package app

type SubtitleTimestamp struct {
	Text  string  `json:"text"`
	Start float64 `json:"start"`
	End   float64 `json:"end"`
}

type VideoInput struct {
	UUID               string              `json:"uuid"`
	Voiceover          string              `json:"voiceover"`
	SubtitleTimestamps []SubtitleTimestamp `json:"subtitle_timestamps"`
	ResourceTimestamps map[string]any      `json:"resource_timestamps"`
	Status             string              `json:"status"`
	Error              *string             `json:"error"`
	Title              string              `json:"title,omitempty"`
	SourceURL          string              `json:"source_url,omitempty"`
}

type VideoMetadata struct {
	Title       string
	Description string
	Tags        []string
	CategoryID  string
}

type ProcessVideoRequest struct {
	VideoInput
}

type ProcessVideoResponse struct {
	Success bool   `json:"success"`
	Message string `json:"message"`
	VideoID string `json:"video_id,omitempty"`
	Error   string `json:"error,omitempty"`
}
