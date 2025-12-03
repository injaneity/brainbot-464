package tui

import "time"

// Messages for the tea program (polling-based)

// StatusUpdateMsg is sent when we receive status from orchestrator
type StatusUpdateMsg struct {
	Status *StatusResponse
	Err    error
}

// TickMsg is sent periodically to trigger polling
type TickMsg struct {
	Time time.Time
}

// StartWorkflowMsg is sent when user triggers workflow
type StartWorkflowMsg struct {
	Err error
}
