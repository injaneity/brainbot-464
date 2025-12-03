package tui

import (
	"time"

	tea "github.com/charmbracelet/bubbletea"
)

// pollStatus creates a command to poll orchestrator status
func pollStatus(client *OrchestratorClient) tea.Cmd {
	return func() tea.Msg {
		status, err := client.GetStatus()
		return StatusUpdateMsg{
			Status: status,
			Err:    err,
		}
	}
}

// startWorkflow creates a command to start the workflow
func startWorkflow(client *OrchestratorClient) tea.Cmd {
	return func() tea.Msg {
		err := client.Start()
		return StartWorkflowMsg{Err: err}
	}
}

// tickCmd creates a command that ticks every 500ms for polling
func tickCmd() tea.Cmd {
	return tea.Tick(500*time.Millisecond, func(t time.Time) tea.Msg {
		return TickMsg{Time: t}
	})
}
