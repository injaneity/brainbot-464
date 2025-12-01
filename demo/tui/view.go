package tui

import (
	"fmt"
	"strings"
)

// View implements tea.Model interface
func (m Model) View() string {
	var b strings.Builder

	// Title
	b.WriteString(TitleStyle.Render("🤖 BrainBot Integration Demo"))
	b.WriteString("\n\n")

	// Current state
	b.WriteString(m.getStateText())
	b.WriteString("\n\n")

	// Statistics
	if len(m.Articles) > 0 {
		stats := fmt.Sprintf("📊 Articles fetched: %d", len(m.Articles))
		b.WriteString(InfoStyle.Render(stats))
		b.WriteString("\n")
	}

	if len(m.DedupResults) > 0 {
		newCount, dupCount := m.countDedupResults()
		stats := fmt.Sprintf("   New: %d | Duplicates: %d", newCount, dupCount)
		b.WriteString(InfoStyle.Render(stats))
		b.WriteString("\n")
	}

	// Webhook info
	if m.WebhookPort != "" && m.State != StateError {
		webhookInfo := fmt.Sprintf("🌐 Webhook listening on: http://localhost:%s/webhook", m.WebhookPort)
		b.WriteString(InfoStyle.Render(webhookInfo))
		b.WriteString("\n\n")
	}

	// Logs
	if len(m.Logs) > 0 {
		b.WriteString(InfoStyle.Render("📝 Recent Activity:"))
		b.WriteString("\n")
		for _, logMsg := range m.Logs {
			b.WriteString(InfoStyle.Render("   " + logMsg))
			b.WriteString("\n")
		}
		b.WriteString("\n")
	}

	// Results
	if m.State == StateComplete && m.WebhookPayload != nil {
		resultBox := m.formatWebhookResult()
		b.WriteString(BoxStyle.Render(resultBox))
		b.WriteString("\n\n")
	}

	// Help text
	if m.State == StateIdle {
		b.WriteString(InfoStyle.Render("Press 'd' to start demo | Press 'q' or Ctrl+C to quit"))
	} else if m.State != StateComplete {
		b.WriteString(InfoStyle.Render("Press 'q' or Ctrl+C to quit"))
	} else {
		b.WriteString(HighlightStyle.Render("Press 'q' or Ctrl+C to exit"))
	}

	return b.String()
}
