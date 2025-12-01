package tui

import (
	"brainbot/demo/client"
	"brainbot/ingestion_service/types"
	"fmt"
	"net/http"
	"strings"
	"time"

	tea "github.com/charmbracelet/bubbletea"
)

// State represents the application state machine
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

// Model represents the application state
type Model struct {
	State           State
	AppClient       *client.Client
	Articles        []*types.Article
	DedupResults    []client.ArticleResult
	GenerationUUID  string
	WebhookPayload  *WebhookPayload
	WebhookServer   *http.Server
	WebhookPort     string
	WebhookReceived bool
	Err             error
	Logs            []string
}

// NewModel creates a new TUI model
func NewModel(webhookPort string, server *http.Server, appClient *client.Client) Model {
	return Model{
		State:         StateIdle,
		AppClient:     appClient,
		WebhookPort:   webhookPort,
		WebhookServer: server,
		Logs:          make([]string, 0, 10),
	}
}

// Init implements tea.Model interface
func (m Model) Init() tea.Cmd {
	return nil
}

// AddLog adds a log entry and returns a new model (value semantics!)
func (m Model) AddLog(logMsg string) Model {
	logEntry := fmt.Sprintf("[%s] %s", time.Now().Format("15:04:05"), logMsg)
	newLogs := append([]string{}, m.Logs...)
	newLogs = append(newLogs, logEntry)
	if len(newLogs) > 10 {
		newLogs = newLogs[len(newLogs)-10:]
	}
	m.Logs = newLogs
	return m
}

// getStateText returns the appropriate state message
func (m Model) getStateText() string {
	switch m.State {
	case StateIdle:
		return HighlightStyle.Render("👋 Ready to start!") + "\n\n" +
			InfoStyle.Render("Press 'd' to begin the demo")
	case StateClearing:
		return StatusStyle.Render("🧹 Clearing ChromaDB cache...")
	case StateFetching:
		return StatusStyle.Render("⏳ Fetching RSS feed...")
	case StateDeduplicating:
		return StatusStyle.Render("🔍 Deduplicating articles...")
	case StateSending:
		return StatusStyle.Render("📤 Sending to generation service...")
	case StateWaiting:
		return StatusStyle.Render(fmt.Sprintf("⏰ Waiting for generation service (UUID: %s)...", m.GenerationUUID))
	case StateComplete:
		return HighlightStyle.Render("✅ COMPLETE")
	case StateError:
		return ErrorStyle.Render(fmt.Sprintf("❌ Error: %v", m.Err))
	default:
		return ""
	}
}

// countDedupResults counts new and duplicate articles
func (m Model) countDedupResults() (newCount, dupCount int) {
	for _, r := range m.DedupResults {
		switch r.Status {
		case "new":
			newCount++
		case "duplicate":
			dupCount++
		}
	}
	return
}

// formatWebhookResult formats the webhook payload for display
func (m Model) formatWebhookResult() string {
	payload := m.WebhookPayload
	var b strings.Builder

	b.WriteString(HighlightStyle.Render("Generation Service Result"))
	b.WriteString("\n\n")

	b.WriteString(fmt.Sprintf("Status: %s\n", StatusStyle.Render(payload.Status)))
	b.WriteString(fmt.Sprintf("UUID: %s\n\n", payload.UUID))

	if payload.Error != nil && *payload.Error != "" {
		b.WriteString(ErrorStyle.Render(fmt.Sprintf("Error: %s\n\n", *payload.Error)))
	}

	if payload.Voiceover != "" {
		voiceoverPreview := payload.Voiceover
		if len(voiceoverPreview) > 200 {
			voiceoverPreview = voiceoverPreview[:200] + "..."
		}
		b.WriteString(fmt.Sprintf("Voiceover Preview:\n%s\n\n", InfoStyle.Render(voiceoverPreview)))
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
