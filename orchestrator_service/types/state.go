package types

import (
	"brainbot/shared/types"
)

// State represents the orchestrator state machine
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

// StatusResponse is the JSON response for GET /api/status
type StatusResponse = types.StatusResponse
