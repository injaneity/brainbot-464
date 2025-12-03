package tui

import (
	"fmt"
	"strings"
)

// View implements tea.Model interface
func (m Model) View() string {
	var b strings.Builder

	// Title
	b.WriteString(TitleStyle.Render("🤖 BrainBot Integration Demo (Client)"))
	b.WriteString("\n\n")

	// Connection status
	if m.Connected {
		b.WriteString(InfoStyle.Render("🟢 Connected to orchestrator"))
		b.WriteString("\n")
		b.WriteString(InfoStyle.Render(TextCronNote))
	} else {
		b.WriteString(ErrorStyle.Render("🔴 Disconnected from orchestrator"))
		if m.Err != nil {
			b.WriteString("\n")
			b.WriteString(ErrorStyle.Render(fmt.Sprintf("   %v", m.Err)))
		}
	}
	b.WriteString("\n\n")

	// Feed Selection
	b.WriteString(InfoStyle.Render("📡 Configured Feeds: "))
	for i, feed := range m.AvailableFeeds {
		if i > 0 {
			b.WriteString(", ")
		}
		b.WriteString(HighlightStyle.Render(feed.Name))
	}
	b.WriteString("\n\n")

	// Current state
	b.WriteString(m.getStateText())
	b.WriteString("\n\n")

	// Statistics (from orchestrator)
	if m.Connected && m.ArticleCount > 0 {
		stats := fmt.Sprintf("📊 Articles fetched: %d", m.ArticleCount)
		b.WriteString(InfoStyle.Render(stats))
		b.WriteString("\n")
	}

	if m.Connected && (m.NewCount > 0 || m.DuplicateCount > 0) {
		stats := fmt.Sprintf("   New: %d | Duplicates: %d", m.NewCount, m.DuplicateCount)
		b.WriteString(InfoStyle.Render(stats))
		b.WriteString("\n\n")
	}

	// Logs (from orchestrator)
	if m.Connected && len(m.Logs) > 0 {
		b.WriteString(InfoStyle.Render("📝 Recent Activity:"))
		b.WriteString("\n")

		// Show last 10 logs
		startIdx := 0
		if len(m.Logs) > 10 {
			startIdx = len(m.Logs) - 10
		}

		for _, logEntry := range m.Logs[startIdx:] {
			timestamp := logEntry.Timestamp.Format("15:04:05")
			b.WriteString(InfoStyle.Render(fmt.Sprintf("   [%s] %s", timestamp, logEntry.Message)))
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
	if m.State == StateIdle || m.State == StateComplete || m.State == StateError {
		b.WriteString(InfoStyle.Render(TextFooterIdle))
	} else {
		b.WriteString(InfoStyle.Render(TextFooterRunning))
	}

	return b.String()
}
