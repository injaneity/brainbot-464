package tui

import (
	"brainbot/demo/client"
	"brainbot/ingestion_service/types"
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
	"time"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/google/uuid"
	"github.com/joho/godotenv"
)

// Tea commands

// ClearCacheAndFetch creates a command to clear the cache
func ClearCacheAndFetch(appClient *client.Client) tea.Cmd {
	return func() tea.Msg {
		ctx := context.Background()
		err := appClient.ClearCache(ctx)
		return CacheClearedMsg{Err: err}
	}
}

// FetchArticles creates a command to fetch RSS articles
func FetchArticles(appClient *client.Client) tea.Cmd {
	return func() tea.Msg {
		_ = godotenv.Load()
		ctx := context.Background()
		articles, err := appClient.FetchArticles(ctx, "", 0)
		if err != nil {
			return FetchCompleteMsg{Err: err}
		}
		return FetchCompleteMsg{Articles: articles}
	}
}

// DeduplicateArticles creates a command to deduplicate articles
func DeduplicateArticles(appClient *client.Client, articles []*types.Article) tea.Cmd {
	return func() tea.Msg {
		_ = godotenv.Load()
		ctx := context.Background()
		results, err := appClient.ProcessArticles(ctx, articles)
		if err != nil {
			return DeduplicationCompleteMsg{Err: err}
		}

		for i, r := range results {
			if r.Status == "new" {
				log.Printf("Article %s: NEW - added to database", r.Article.Title)
			} else if r.Status == "duplicate" {
				log.Printf("Article %s: DUPLICATE (%.2f%% similar)", r.Article.Title, r.DeduplicationResult.SimilarityScore*100)
			} else if r.Status == "failed" {
				log.Printf("Article %s: FAILED extraction", r.Article.Title)
			} else if r.Status == "error" {
				log.Printf("Article %s: ERROR - %v", r.Article.Title, r.Error)
			}
			_ = i
		}

		return DeduplicationCompleteMsg{Results: results}
	}
}

// SendGenerationRequest creates a command to send generation request
func SendGenerationRequest(results []client.ArticleResult, webhookPort string) tea.Cmd {
	return func() tea.Msg {
		_ = godotenv.Load()

		var articleTexts []string
		for _, r := range results {
			if r.Status == "new" && r.Article != nil {
				text := r.Article.FullContentText
				if text == "" {
					text = r.Article.Summary
				}
				articleTexts = append(articleTexts, text)
			}
		}

		if len(articleTexts) == 0 {
			return ErrorMsg{Err: fmt.Errorf("no new articles to send for generation")}
		}

		reqUUID := uuid.New().String()
		webhookURL := fmt.Sprintf("http://localhost:%s/webhook", webhookPort)

		requestBody := map[string]interface{}{
			"uuid":        reqUUID,
			"articles":    articleTexts,
			"webhook_url": webhookURL,
		}

		jsonData, err := json.Marshal(requestBody)
		if err != nil {
			return GenerationRequestSentMsg{Err: err}
		}

		generationURL := getEnvOrDefault("GENERATION_SERVICE_URL", "http://localhost:8001")
		url := fmt.Sprintf("%s/generate", generationURL)

		resp, err := http.Post(url, "application/json", bytes.NewBuffer(jsonData))
		if err != nil {
			return GenerationRequestSentMsg{Err: fmt.Errorf("failed to send request: %w", err)}
		}
		defer resp.Body.Close()

		if resp.StatusCode != http.StatusAccepted {
			body, _ := io.ReadAll(resp.Body)
			return GenerationRequestSentMsg{Err: fmt.Errorf("generation service returned %d: %s", resp.StatusCode, string(body))}
		}

		return GenerationRequestSentMsg{UUID: reqUUID}
	}
}

// Webhook server

// StartWebhookServer starts the webhook server
func StartWebhookServer(port string, program *tea.Program) (*http.Server, error) {
	mux := http.NewServeMux()

	mux.HandleFunc("/webhook", func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
			return
		}

		var payload WebhookPayload
		decoder := json.NewDecoder(r.Body)
		if err := decoder.Decode(&payload); err != nil {
			http.Error(w, "Invalid JSON", http.StatusBadRequest)
			return
		}

		if program != nil {
			program.Send(WebhookReceivedMsg{Payload: payload})
		}

		w.WriteHeader(http.StatusOK)
		w.Write([]byte(`{"status":"received"}`))
	})

	mux.HandleFunc("/health", func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		w.Write([]byte(`{"status":"ok"}`))
	})

	server := &http.Server{
		Addr:    ":" + port,
		Handler: mux,
	}

	go func() {
		if err := server.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			log.Printf("Webhook server error: %v", err)
		}
	}()

	time.Sleep(100 * time.Millisecond)
	return server, nil
}

func getEnvOrDefault(key, defaultVal string) string {
	return client.GetEnvOrDefault(key, defaultVal)
}
