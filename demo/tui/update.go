package tui

import (
	"fmt"

	tea "github.com/charmbracelet/bubbletea"
)

// Update implements tea.Model interface (polling-based version)
func (m Model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.KeyMsg:
		return m.handleKeyPress(msg)

	case StatusUpdateMsg:
		return m.handleStatusUpdate(msg)

	case StartWorkflowMsg:
		return m.handleStartWorkflow(msg)

	case TickMsg:
		// Poll again
		return m, tea.Batch(
			pollStatus(m.OrchestratorClient),
			tickCmd(),
		)
	}

	return m, nil
}

// handleKeyPress processes keyboard input
func (m Model) handleKeyPress(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch msg.String() {
	case "ctrl+c", "q":
		// Just quit the TUI - orchestrator keeps running
		return m, tea.Quit

	case "x", "X":
		// Shutdown orchestrator and quit
		m.ExitCode = 10
		return m, tea.Quit

	case "d", "D":
		// Start workflow (only if idle or complete)
		if m.Connected && (m.State == StateIdle || m.State == StateComplete || m.State == StateError) {
			return m, startWorkflow(m.OrchestratorClient)
		}
	}

	return m, nil
}

// handleStatusUpdate processes status update from orchestrator
func (m Model) handleStatusUpdate(msg StatusUpdateMsg) (tea.Model, tea.Cmd) {
	if msg.Err != nil {
		m.Connected = false
		m.Err = fmt.Errorf("failed to connect to orchestrator: %w", msg.Err)
		return m, nil
	}

	// Successfully connected
	m.Connected = true

	// Update local state from orchestrator
	status := msg.Status
	m.State = status.State
	m.Logs = status.Logs
	m.ArticleCount = status.ArticleCount
	m.NewCount = status.NewCount
	m.DuplicateCount = status.DuplicateCount
	m.GenerationUUID = status.GenerationUUID
	m.WebhookPayload = status.WebhookPayload

	if status.Error != "" {
		m.Err = fmt.Errorf("%s", status.Error)
	} else {
		m.Err = nil
	}

	return m, nil
}

// handleStartWorkflow processes workflow start response
func (m Model) handleStartWorkflow(msg StartWorkflowMsg) (tea.Model, tea.Cmd) {
	if msg.Err != nil {
		m.Err = fmt.Errorf("failed to start workflow: %w", msg.Err)
	}
	// Status will be updated via next poll
	return m, nil
}
