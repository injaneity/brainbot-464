package tui

import (
	"brainbot/shared/types"
	"fmt"
	"strings"

	tea "github.com/charmbracelet/bubbletea"
)

// State represents the application state machine
type State = types.State

const (
	StateIdle          = types.StateIdle
	StateClearing      = types.StateClearing
	StateFetching      = types.StateFetching
	StateDeduplicating = types.StateDeduplicating
	StateSending       = types.StateSending
	StateWaiting       = types.StateWaiting
	StateComplete      = types.StateComplete
	StateError         = types.StateError
)

// LogEntry represents a single log line with timestamp
type LogEntry = types.LogEntry

// WebhookPayload represents the generation service response
type WebhookPayload = types.WebhookPayload

// StatusResponse is the JSON response from orchestrator
type StatusResponse = types.StatusResponse

// Model represents the TUI client state (thin client)
type Model struct {
	// Orchestrator client
	OrchestratorClient *OrchestratorClient

	// Local UI state (synced from orchestrator)
	State          State
	Logs           []LogEntry
	ArticleCount   int
	NewCount       int
	DuplicateCount int
	GenerationUUID string
	WebhookPayload *WebhookPayload
	Err            error

	// Connection status
	Connected bool

	// Exit code for the application
	ExitCode int
}

// NewModel creates a new TUI model
func NewModel(orchestratorURL string) Model {
	return Model{
		OrchestratorClient: NewOrchestratorClient(orchestratorURL),
		State:              StateIdle,
		Logs:               make([]LogEntry, 0),
		Connected:          false,
	}
}

// Init implements tea.Model interface
func (m Model) Init() tea.Cmd {
	// Start polling immediately
	return tea.Batch(
		pollStatus(m.OrchestratorClient),
		tickCmd(),
	)
}

// getStateText returns the appropriate state message
func (m Model) getStateText() string {
	if !m.Connected {
		return ErrorStyle.Render("❌ Not connected to orchestrator")
	}

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
		errMsg := "Unknown error"
		if m.Err != nil {
			errMsg = m.Err.Error()
		}
		return ErrorStyle.Render(fmt.Sprintf("❌ Error: %v", errMsg))
	default:
		return ""
	}
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
