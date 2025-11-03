package main

import (
	"brainbot/deduplication"
	"brainbot/rssfeeds"
	"brainbot/types"
	"bytes"
	"context"
	"encoding/json"
	"flag"
	"fmt"
	"io"
	"log"
	"net/http"
	"os"
	"os/signal"
	"strings"
	"syscall"
	"time"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"github.com/google/uuid"
	"github.com/joho/godotenv"
)

// Styles
var (
	titleStyle = lipgloss.NewStyle().
			Bold(true).
			Foreground(lipgloss.Color("#7D56F4")).
			MarginTop(1).
			MarginBottom(1)

	statusStyle = lipgloss.NewStyle().
			Foreground(lipgloss.Color("#04B575"))

	errorStyle = lipgloss.NewStyle().
			Foreground(lipgloss.Color("#FF0000"))

	infoStyle = lipgloss.NewStyle().
			Foreground(lipgloss.Color("#626262"))

	boxStyle = lipgloss.NewStyle().
			Border(lipgloss.RoundedBorder()).
			BorderForeground(lipgloss.Color("#874BFD")).
			Padding(1, 2)

	highlightStyle = lipgloss.NewStyle().
			Bold(true).
			Foreground(lipgloss.Color("#FAFAFA")).
			Background(lipgloss.Color("#7D56F4")).
			Padding(0, 1)
)

// Messages for the tea program
type fetchCompleteMsg struct {
	articles []*types.Article
	err      error
}

type cacheClearedMsg struct {
	err error
}

type deduplicationCompleteMsg struct {
	results []ArticleResult
	err     error
}

type generationRequestSentMsg struct {
	uuid string
	err  error
}

type webhookReceivedMsg struct {
	payload WebhookPayload
}

type errorMsg struct {
	err error
}

// ArticleResult represents deduplication results
type ArticleResult struct {
	Article             *types.Article
	Status              string
	DeduplicationResult *deduplication.DeduplicationResult
	Error               string
}

// WebhookPayload represents the generation service response
type WebhookPayload struct {
	UUID                string                   `json:"uuid"`
	Voiceover           string                   `json:"voiceover"`
	SubtitleTimestamps  []map[string]interface{} `json:"subtitle_timestamps"`
	ResourceTimestamps  map[string]interface{}   `json:"resource_timestamps"`
	Status              string                   `json:"status"`
	Error               *string                  `json:"error,omitempty"`
	Timings             map[string]float64       `json:"timings,omitempty"`
}

// Model represents the application state
type model struct {
	state           string // "idle", "fetching", "deduplicating", "sending", "waiting", "complete", "error"
	articles        []*types.Article
	dedupResults    []ArticleResult
	generationUUID  string
	webhookPayload  *WebhookPayload
	webhookServer   *http.Server
	webhookPort     string
	webhookReceived bool
	err             error
	logs            []string
}

func initialModel(webhookPort string, server *http.Server) model {
	return model{
		state:         "idle",
		webhookPort:   webhookPort,
		webhookServer: server,
		logs:          []string{},
	}
}

func (m model) Init() tea.Cmd {
	// Don't start automatically - wait for user input
	return nil
}

func (m model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.KeyMsg:
		switch msg.String() {
		case "ctrl+c", "q":
			if m.webhookServer != nil {
				_ = m.webhookServer.Shutdown(context.Background())
			}
			return m, tea.Quit
		case "d", "D":
			// Start the demo when user presses 'd'
			if m.state == "idle" {
				m.state = "clearing"
				m.addLog("Clearing ChromaDB cache...")
				return m, clearCacheAndFetch()
			}
		}

	case cacheClearedMsg:
		if msg.err != nil {
			m.state = "error"
			m.err = fmt.Errorf("failed to clear cache: %w", msg.err)
			return m, nil
		}
		m.state = "fetching"
		m.addLog("Cache cleared successfully")
		return m, fetchArticles()

	case fetchCompleteMsg:
		if msg.err != nil {
			m.state = "error"
			m.err = msg.err
			return m, nil
		}
		m.articles = msg.articles
		m.state = "deduplicating"
		m.addLog(fmt.Sprintf("Fetched %d articles", len(msg.articles)))
		return m, deduplicateArticles(msg.articles)

	case deduplicationCompleteMsg:
		if msg.err != nil {
			m.state = "error"
			m.err = msg.err
			return m, nil
		}
		m.dedupResults = msg.results
		m.state = "sending"

		// Count new articles
		newCount := 0
		for _, r := range msg.results {
			if r.Status == "new" {
				newCount++
			}
		}
		m.addLog(fmt.Sprintf("Found %d new articles", newCount))

		return m, sendGenerationRequest(m.dedupResults, m.webhookPort)

	case generationRequestSentMsg:
		if msg.err != nil {
			m.state = "error"
			m.err = msg.err
			return m, nil
		}
		m.generationUUID = msg.uuid
		m.state = "waiting"
		m.addLog(fmt.Sprintf("Generation request sent with UUID: %s", msg.uuid))
		return m, nil

	case webhookReceivedMsg:
		m.webhookPayload = &msg.payload
		m.webhookReceived = true
		m.state = "complete"
		m.addLog("Webhook received from generation service!")
		return m, nil

	case errorMsg:
		m.state = "error"
		m.err = msg.err
		return m, nil
	}

	return m, nil
}

func (m *model) addLog(logMsg string) {
	m.logs = append(m.logs, fmt.Sprintf("[%s] %s", time.Now().Format("15:04:05"), logMsg))
	if len(m.logs) > 10 {
		m.logs = m.logs[len(m.logs)-10:]
	}
}

func (m model) View() string {
	var b strings.Builder

	// Title
	b.WriteString(titleStyle.Render("🤖 BrainBot Integration Demo"))
	b.WriteString("\n\n")

	// Current state
	stateText := ""
	switch m.state {
	case "idle":
		stateText = highlightStyle.Render("👋 Ready to start!") + "\n\n" +
			infoStyle.Render("Press 'd' to begin the demo")
	case "clearing":
		stateText = statusStyle.Render("🧹 Clearing ChromaDB cache...")
	case "fetching":
		stateText = statusStyle.Render("⏳ Fetching RSS feed...")
	case "deduplicating":
		stateText = statusStyle.Render("🔍 Deduplicating articles...")
	case "sending":
		stateText = statusStyle.Render("📤 Sending to generation service...")
	case "waiting":
		stateText = statusStyle.Render(fmt.Sprintf("⏰ Waiting for generation service (UUID: %s)...", m.generationUUID))
	case "complete":
		stateText = highlightStyle.Render("✅ COMPLETE")
	case "error":
		stateText = errorStyle.Render(fmt.Sprintf("❌ Error: %v", m.err))
	}
	b.WriteString(stateText)
	b.WriteString("\n\n")

	// Statistics
	if len(m.articles) > 0 {
		stats := fmt.Sprintf("📊 Articles fetched: %d", len(m.articles))
		b.WriteString(infoStyle.Render(stats))
		b.WriteString("\n")
	}

	if len(m.dedupResults) > 0 {
		newCount := 0
		dupCount := 0
		for _, r := range m.dedupResults {
			if r.Status == "new" {
				newCount++
			} else if r.Status == "duplicate" {
				dupCount++
			}
		}
		stats := fmt.Sprintf("   New: %d | Duplicates: %d", newCount, dupCount)
		b.WriteString(infoStyle.Render(stats))
		b.WriteString("\n")
	}

	// Webhook info
	if m.webhookPort != "" && m.state != "error" {
		webhookInfo := fmt.Sprintf("🌐 Webhook listening on: http://localhost:%s/webhook", m.webhookPort)
		b.WriteString(infoStyle.Render(webhookInfo))
		b.WriteString("\n\n")
	}

	// Logs
	if len(m.logs) > 0 {
		b.WriteString(infoStyle.Render("📝 Recent Activity:"))
		b.WriteString("\n")
		for _, logMsg := range m.logs {
			b.WriteString(infoStyle.Render("   " + logMsg))
			b.WriteString("\n")
		}
		b.WriteString("\n")
	}

	// Results
	if m.state == "complete" && m.webhookPayload != nil {
		resultBox := formatWebhookResult(m.webhookPayload)
		b.WriteString(boxStyle.Render(resultBox))
		b.WriteString("\n\n")
	}

	// Help text
	if m.state == "idle" {
		b.WriteString(infoStyle.Render("Press 'd' to start demo | Press 'q' or Ctrl+C to quit"))
	} else if m.state != "complete" {
		b.WriteString(infoStyle.Render("Press 'q' or Ctrl+C to quit"))
	} else {
		b.WriteString(highlightStyle.Render("Press 'q' or Ctrl+C to exit"))
	}

	return b.String()
}

func formatWebhookResult(payload *WebhookPayload) string {
	var b strings.Builder

	b.WriteString(highlightStyle.Render("Generation Service Result"))
	b.WriteString("\n\n")

	b.WriteString(fmt.Sprintf("Status: %s\n", statusStyle.Render(payload.Status)))
	b.WriteString(fmt.Sprintf("UUID: %s\n\n", payload.UUID))

	if payload.Error != nil && *payload.Error != "" {
		b.WriteString(errorStyle.Render(fmt.Sprintf("Error: %s\n\n", *payload.Error)))
	}

	if payload.Voiceover != "" {
		voiceoverPreview := payload.Voiceover
		if len(voiceoverPreview) > 200 {
			voiceoverPreview = voiceoverPreview[:200] + "..."
		}
		b.WriteString(fmt.Sprintf("Voiceover Preview:\n%s\n\n", infoStyle.Render(voiceoverPreview)))
	}

	if len(payload.SubtitleTimestamps) > 0 {
		b.WriteString(fmt.Sprintf("Subtitle Segments: %d\n", len(payload.SubtitleTimestamps)))
	}

	if len(payload.Timings) > 0 {
		b.WriteString("\nTimings:\n")
		for key, val := range payload.Timings {
			b.WriteString(fmt.Sprintf("  %s: %.2fs\n", key, val))
		}
	}

	return b.String()
}

// Tea commands
func clearCacheAndFetch() tea.Cmd {
	return func() tea.Msg {
		err := clearChromaCache()
		return cacheClearedMsg{err: err}
	}
}

func fetchArticles() tea.Cmd {
	return func() tea.Msg {
		// Load environment
		_ = godotenv.Load()

		feedURL := rssfeeds.ResolveFeedURL(rssfeeds.DefaultFeedPreset)
		articles, err := rssfeeds.FetchFeed(feedURL, rssfeeds.DefaultCount)
		if err != nil {
			return fetchCompleteMsg{err: err}
		}

		// Extract content
		rssfeeds.ExtractAllContent(articles)

		return fetchCompleteMsg{articles: articles}
	}
}

func deduplicateArticles(articles []*types.Article) tea.Cmd {
	return func() tea.Msg {
		_ = godotenv.Load()

		// Check if mock mode is enabled (for demo without API keys)
		mockMode := getEnvOrDefault("MOCK_DEDUPLICATION", "false")
		if mockMode == "true" || mockMode == "1" {
			log.Println("⚠️  Mock deduplication mode enabled - all articles marked as NEW")
			// Mock mode: treat all articles as new
			results := make([]ArticleResult, 0, len(articles))
			for _, article := range articles {
				if article.ExtractionError != "" {
					results = append(results, ArticleResult{
						Article: article,
						Status:  "failed",
						Error:   article.ExtractionError,
					})
					log.Printf("Article %s: FAILED extraction", article.Title)
					continue
				}

				results = append(results, ArticleResult{
					Article: article,
					Status:  "new",
					DeduplicationResult: &deduplication.DeduplicationResult{
						IsDuplicate:     false,
						MatchingID:      "",
						SimilarityScore: 0,
						CheckedAt:       time.Now(),
					},
				})
				log.Printf("Article %s: NEW (mock mode)", article.Title)
			}
			return deduplicationCompleteMsg{results: results}
		}

		// Real deduplication mode
		chromaConfig := deduplication.ChromaConfig{
			Host:           getEnvOrDefault("CHROMA_HOST", "localhost"),
			Port:           8000,
			CollectionName: getEnvOrDefault("CHROMA_COLLECTION", "brainbot_articles"),
			EmbeddingModel: "",
		}

		deduplicatorConfig := deduplication.DeduplicatorConfig{
			ChromaConfig:        chromaConfig,
			SimilarityThreshold: 0,
			MaxSearchResults:    0,
		}

		deduplicator, err := deduplication.NewDeduplicator(deduplicatorConfig)
		if err != nil {
			return deduplicationCompleteMsg{err: err}
		}
		defer deduplicator.Close()

		// Process articles
		results := make([]ArticleResult, 0, len(articles))
		for _, article := range articles {
			if article.ExtractionError != "" {
				results = append(results, ArticleResult{
					Article: article,
					Status:  "failed",
					Error:   article.ExtractionError,
				})
				log.Printf("Article %s: FAILED extraction", article.Title)
				continue
			}

			dedupResult, err := deduplicator.ProcessArticle(article)
			if err != nil {
				results = append(results, ArticleResult{
					Article: article,
					Status:  "error",
					Error:   err.Error(),
				})
				log.Printf("Article %s: ERROR - %v", article.Title, err)
				continue
			}

			if dedupResult.IsDuplicate {
				results = append(results, ArticleResult{
					Article:             article,
					Status:              "duplicate",
					DeduplicationResult: dedupResult,
				})
				log.Printf("Article %s: DUPLICATE (%.2f%% similar)", article.Title, dedupResult.SimilarityScore*100)
			} else {
				results = append(results, ArticleResult{
					Article:             article,
					Status:              "new",
					DeduplicationResult: dedupResult,
				})
				log.Printf("Article %s: NEW - added to database", article.Title)
			}
		}

		return deduplicationCompleteMsg{results: results}
	}
}

func sendGenerationRequest(results []ArticleResult, webhookPort string) tea.Cmd {
	return func() tea.Msg {
		_ = godotenv.Load()

		// Collect new articles
		var articleTexts []string
		for _, r := range results {
			if r.Status == "new" && r.Article != nil {
				// Use full content text for generation
				text := r.Article.FullContentText
				if text == "" {
					text = r.Article.Summary
				}
				articleTexts = append(articleTexts, text)
			}
		}

		if len(articleTexts) == 0 {
			return errorMsg{err: fmt.Errorf("no new articles to send for generation")}
		}

		// Generate UUID
		reqUUID := uuid.New().String()

		// Build webhook URL
		webhookURL := fmt.Sprintf("http://localhost:%s/webhook", webhookPort)

		// Build request
		requestBody := map[string]interface{}{
			"uuid":        reqUUID,
			"articles":    articleTexts,
			"webhook_url": webhookURL,
		}

		jsonData, err := json.Marshal(requestBody)
		if err != nil {
			return generationRequestSentMsg{err: err}
		}

		// Send to generation service
		generationURL := getEnvOrDefault("GENERATION_SERVICE_URL", "http://localhost:8001")
		url := fmt.Sprintf("%s/generate", generationURL)

		resp, err := http.Post(url, "application/json", bytes.NewBuffer(jsonData))
		if err != nil {
			return generationRequestSentMsg{err: fmt.Errorf("failed to send request: %w", err)}
		}
		defer resp.Body.Close()

		if resp.StatusCode != http.StatusAccepted {
			body, _ := io.ReadAll(resp.Body)
			return generationRequestSentMsg{err: fmt.Errorf("generation service returned %d: %s", resp.StatusCode, string(body))}
		}

		return generationRequestSentMsg{uuid: reqUUID}
	}
}

func waitForWebhook(received bool) tea.Cmd {
	return func() tea.Msg {
		// This is handled by the webhook server
		return nil
	}
}

func getEnvOrDefault(key, defaultVal string) string {
	if val := os.Getenv(key); val != "" {
		return val
	}
	return defaultVal
}

// clearChromaCache clears all documents from the ChromaDB collection
func clearChromaCache() error {
	chromaConfig := deduplication.ChromaConfig{
		Host:           getEnvOrDefault("CHROMA_HOST", "localhost"),
		Port:           8000,
		CollectionName: getEnvOrDefault("CHROMA_COLLECTION", "brainbot_articles"),
		EmbeddingModel: "",
	}

	// Need to create a full client to get collection ID
	// Use minimal embeddings just to initialize (won't be used for clearing)
	chroma, err := deduplication.NewChromaReadOnly(chromaConfig)
	if err != nil {
		return fmt.Errorf("failed to connect to ChromaDB: %w", err)
	}

	return chroma.ClearCollection()
}

// Webhook server
func startWebhookServer(port string, program *tea.Program) (*http.Server, error) {
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

		// Send message to tea program (will work even if program is nil initially)
		if program != nil {
			program.Send(webhookReceivedMsg{payload: payload})
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

	// Wait a bit for server to start
	time.Sleep(100 * time.Millisecond)

	return server, nil
}

func main() {
	// Parse command-line flags
	clearCache := flag.Bool("clear", true, "Clear ChromaDB cache before starting (default: true)")
	flag.Parse()

	// Load environment
	_ = godotenv.Load()

	// Note: Cache clearing is now done when user presses 'd' if clearCache is true
	_ = clearCache // Store for later use if needed

	// Set up webhook server
	webhookPort := getEnvOrDefault("WEBHOOK_PORT", "9999")

	// Create the model first
	m := initialModel(webhookPort, nil)

	// Create the tea program
	program := tea.NewProgram(m)

	// Start webhook server with the program
	server, err := startWebhookServer(webhookPort, program)
	if err != nil {
		fmt.Printf("Failed to start webhook server: %v\n", err)
		os.Exit(1)
	}

	// Update model with server reference
	m.webhookServer = server

	// Handle graceful shutdown
	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, os.Interrupt, syscall.SIGTERM)

	go func() {
		<-sigChan
		if server != nil {
			ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
			defer cancel()
			_ = server.Shutdown(ctx)
		}
		program.Quit()
	}()

	// Run the program
	if _, err := program.Run(); err != nil {
		fmt.Printf("Error running program: %v\n", err)
		os.Exit(1)
	}

	// Clean up
	if server != nil {
		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		_ = server.Shutdown(ctx)
	}
}
