package tui

import (
	"context"
	"fmt"

	tea "github.com/charmbracelet/bubbletea"
)

// Update implements tea.Model interface
func (m Model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.KeyMsg:
		return m.handleKeyPress(msg)
	case CacheClearedMsg:
		return m.handleCacheCleared(msg)
	case FetchCompleteMsg:
		return m.handleFetchComplete(msg)
	case DeduplicationCompleteMsg:
		return m.handleDeduplicationComplete(msg)
	case GenerationRequestSentMsg:
		return m.handleGenerationRequestSent(msg)
	case WebhookReceivedMsg:
		return m.handleWebhookReceived(msg)
	case ErrorMsg:
		return m.handleError(msg)
	}
	return m, nil
}

// handleKeyPress processes keyboard input
func (m Model) handleKeyPress(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch msg.String() {
	case "ctrl+c", "q":
		if m.WebhookServer != nil {
			_ = m.WebhookServer.Shutdown(context.Background())
		}
		return m, tea.Quit
	case "d", "D":
		if m.State == StateIdle {
			m.State = StateClearing
			m = m.AddLog("Clearing ChromaDB cache...")
			return m, ClearCacheAndFetch(m.AppClient)
		}
	}
	return m, nil
}

// handleCacheCleared processes cache clearing completion
func (m Model) handleCacheCleared(msg CacheClearedMsg) (tea.Model, tea.Cmd) {
	if msg.Err != nil {
		m.State = StateError
		m.Err = fmt.Errorf("failed to clear cache: %w", msg.Err)
		return m, nil
	}
	m.State = StateFetching
	m = m.AddLog("Cache cleared successfully")
	return m, FetchArticles(m.AppClient)
}

// handleFetchComplete processes fetch completion
func (m Model) handleFetchComplete(msg FetchCompleteMsg) (tea.Model, tea.Cmd) {
	if msg.Err != nil {
		m.State = StateError
		m.Err = msg.Err
		return m, nil
	}
	m.Articles = msg.Articles
	m.State = StateDeduplicating
	m = m.AddLog(fmt.Sprintf("Fetched %d articles", len(msg.Articles)))
	return m, DeduplicateArticles(m.AppClient, msg.Articles)
}

// handleDeduplicationComplete processes deduplication completion
func (m Model) handleDeduplicationComplete(msg DeduplicationCompleteMsg) (tea.Model, tea.Cmd) {
	if msg.Err != nil {
		m.State = StateError
		m.Err = msg.Err
		return m, nil
	}
	m.DedupResults = msg.Results
	m.State = StateSending

	newCount := 0
	for _, r := range msg.Results {
		if r.Status == "new" {
			newCount++
		}
	}
	m = m.AddLog(fmt.Sprintf("Found %d new articles", newCount))

	return m, SendGenerationRequest(m.DedupResults, m.WebhookPort)
}

// handleGenerationRequestSent processes generation request sent
func (m Model) handleGenerationRequestSent(msg GenerationRequestSentMsg) (tea.Model, tea.Cmd) {
	if msg.Err != nil {
		m.State = StateError
		m.Err = msg.Err
		return m, nil
	}
	m.GenerationUUID = msg.UUID
	m.State = StateWaiting
	m = m.AddLog(fmt.Sprintf("Generation request sent with UUID: %s", msg.UUID))
	return m, nil
}

// handleWebhookReceived processes webhook reception
func (m Model) handleWebhookReceived(msg WebhookReceivedMsg) (tea.Model, tea.Cmd) {
	m.WebhookPayload = &msg.Payload
	m.WebhookReceived = true
	m.State = StateComplete
	m = m.AddLog("Webhook received from generation service!")
	return m, nil
}

// handleError processes errors
func (m Model) handleError(msg ErrorMsg) (tea.Model, tea.Cmd) {
	m.State = StateError
	m.Err = msg.Err
	return m, nil
}
