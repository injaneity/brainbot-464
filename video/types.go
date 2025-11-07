package video

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
}

type VideoMetadata struct {
    Title       string
    Description string
    Tags        []string
    CategoryID  string
}