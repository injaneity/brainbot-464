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
	webhookPort := r.stateManager.GetWebhookPort()

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
		return fmt.Errorf("no new articles to send for generation")
	}

	reqUUID := uuid.New().String()

	// IMPORTANT: Use Docker internal DNS for webhook callback
	// When running in Docker, "orchestrator" is the service name
	webhookURL := fmt.Sprintf("http://orchestrator:%s/webhook", webhookPort)

	requestBody := map[string]interface{}{
		"uuid":        reqUUID,
		"articles":    articleTexts,
		"webhook_url": webhookURL,
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
