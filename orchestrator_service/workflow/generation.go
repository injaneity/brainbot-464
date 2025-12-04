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

	// Collect article texts
	var articleTexts []string
	for _, res := range results {
		if res.Status == "new" && res.Article != nil {
			text := res.Article.FullContentText
			if text == "" {
				text = res.Article.Summary
			}
			articleTexts = append(articleTexts, text)
		}
	}

	if len(articleTexts) == 0 {
		r.stateManager.AddLog("No new articles to send for generation. Workflow complete.")
		r.stateManager.SetState(types.StateComplete)
		return nil
	}

	reqUUID := uuid.New().String()

	requestBody := map[string]interface{}{
		"uuid":     reqUUID,
		"articles": articleTexts,
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
