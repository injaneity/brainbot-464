package workflow

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"orchestrator/types"
	"os"

	"github.com/google/uuid"
)

// sendGenerationRequest sends request to generation service
func (r *Runner) sendGenerationRequest(ctx context.Context) error {
	r.stateManager.SetState(types.StateSending)
	r.stateManager.AddLog("Sending to generation service...")

	results := r.stateManager.GetDedupResults()

	// Find presigned URL from first new article and collect article URLs
	var presignedURL string
	var articleURLs []string
	for _, res := range results {
		if res.Status == "new" {
			// Get presigned URL from first new article
			if presignedURL == "" && res.PresignedURL != "" {
				presignedURL = res.PresignedURL
			}
			// Collect article URLs
			if res.Article != nil {
				articleURLs = append(articleURLs, res.Article.URL)
			}
		}
	}

	if presignedURL == "" {
		r.stateManager.AddLog("No presigned URL available for new articles. Workflow complete.")
		r.stateManager.SetState(types.StateComplete)
		return nil
	}

	reqUUID := uuid.New().String()

	requestBody := map[string]interface{}{
		"uuid":          reqUUID,
		"presigned_url": presignedURL,
	}

	// Add article URLs if available (optional field in schema)
	if len(articleURLs) > 0 {
		requestBody["article_urls"] = articleURLs
	}

	jsonData, err := json.Marshal(requestBody)
	if err != nil {
		return fmt.Errorf("marshal request: %w", err)
	}

	// Use Docker internal DNS for generation service
	generationURL := getEnvOrDefault("GENERATION_SERVICE_URL", "http://generation-service:8000")
	url := fmt.Sprintf("%s/generate", generationURL)

	resp, err := http.Post(url, "application/json", bytes.NewBuffer(jsonData))
	if err != nil {
		return fmt.Errorf("failed to send request: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusAccepted {
		body, _ := io.ReadAll(resp.Body)
		return fmt.Errorf("generation service returned %d: %s", resp.StatusCode, string(body))
	}

	r.stateManager.SetGenerationUUID(reqUUID)
	r.stateManager.SetState(types.StateWaiting)
	r.stateManager.AddLog(fmt.Sprintf("Generation request sent with UUID: %s", reqUUID))
	return nil
}

func getEnvOrDefault(key, defaultVal string) string {
	if val := os.Getenv(key); val != "" {
		return val
	}
	return defaultVal
}
