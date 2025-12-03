package state

import (
	"fmt"
	"orchestrator/client"
	"orchestrator/types"
	"sync"
	"time"
)

// Manager holds the complete orchestrator state with thread-safe access
type Manager struct {
	mu sync.RWMutex

	// Current state
	currentState types.State

	// Data
	articles       []*types.Article
	dedupResults   []types.ArticleResult
	generationUUID string

	// Logs (ring buffer)
	logs    []types.LogEntry
	maxLogs int
	lastErr error

	// Webhook
	webhookPayload *types.WebhookPayload
	webhookPort    string

	// Dependencies
	ingestionClient *client.IngestionClient
}

// NewManager creates a new state manager
func NewManager(webhookPort string, ingestionClient *client.IngestionClient) *Manager {
	return &Manager{
		currentState:    types.StateIdle,
		ingestionClient: ingestionClient,
		webhookPort:     webhookPort,
		logs:            make([]types.LogEntry, 0),
		maxLogs:         50, // Keep last 50 log entries
	}
}

// AddLog adds a log entry (thread-safe)
func (m *Manager) AddLog(message string) {
	m.mu.Lock()
	defer m.mu.Unlock()

	entry := types.LogEntry{
		Timestamp: time.Now(),
		Message:   message,
	}

	m.logs = append(m.logs, entry)
	if len(m.logs) > m.maxLogs {
		m.logs = m.logs[len(m.logs)-m.maxLogs:]
	}
}

// GetStatus returns a snapshot of the current state (thread-safe)
func (m *Manager) GetStatus() types.StatusResponse {
	m.mu.RLock()
	defer m.mu.RUnlock()

	newCount, dupCount := m.countDedupResults()

	resp := types.StatusResponse{
		State:          m.currentState,
		Logs:           append([]types.LogEntry{}, m.logs...), // Copy slice
		ArticleCount:   len(m.articles),
		NewCount:       newCount,
		DuplicateCount: dupCount,
		GenerationUUID: m.generationUUID,
		WebhookPayload: m.webhookPayload,
	}

	if m.lastErr != nil {
		resp.Error = m.lastErr.Error()
	}

	return resp
}

// SetState sets the current state (thread-safe)
func (m *Manager) SetState(state types.State) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.currentState = state
}

// GetState gets the current state (thread-safe)
func (m *Manager) GetState() types.State {
	m.mu.RLock()
	defer m.mu.RUnlock()
	return m.currentState
}

// SetError sets the error state
func (m *Manager) SetError(err error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.currentState = types.StateError
	m.lastErr = err

	// Add log entry (must be done inside lock since we're already holding it)
	entry := types.LogEntry{
		Timestamp: time.Now(),
		Message:   fmt.Sprintf("Error: %v", err),
	}
	m.logs = append(m.logs, entry)
	if len(m.logs) > m.maxLogs {
		m.logs = m.logs[len(m.logs)-m.maxLogs:]
	}
}

// SetArticles sets the articles list (thread-safe)
func (m *Manager) SetArticles(articles []*types.Article) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.articles = articles
}

// GetArticles gets the articles list (thread-safe)
func (m *Manager) GetArticles() []*types.Article {
	m.mu.RLock()
	defer m.mu.RUnlock()
	return m.articles
}

// SetDedupResults sets the deduplication results (thread-safe)
func (m *Manager) SetDedupResults(results []types.ArticleResult) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.dedupResults = results
}

// GetDedupResults gets the deduplication results (thread-safe)
func (m *Manager) GetDedupResults() []types.ArticleResult {
	m.mu.RLock()
	defer m.mu.RUnlock()
	return m.dedupResults
}

// SetGenerationUUID sets the generation UUID (thread-safe)
func (m *Manager) SetGenerationUUID(uuid string) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.generationUUID = uuid
}

// GetWebhookPort gets the webhook port (thread-safe)
func (m *Manager) GetWebhookPort() string {
	m.mu.RLock()
	defer m.mu.RUnlock()
	return m.webhookPort
}

// SetWebhookPayload sets the webhook payload and transitions to complete state
func (m *Manager) SetWebhookPayload(payload *types.WebhookPayload) {
	m.mu.Lock()
	defer m.mu.Unlock()

	m.webhookPayload = payload
	m.currentState = types.StateComplete

	entry := types.LogEntry{
		Timestamp: time.Now(),
		Message:   "Webhook received from generation service!",
	}
	m.logs = append(m.logs, entry)
	if len(m.logs) > m.maxLogs {
		m.logs = m.logs[len(m.logs)-m.maxLogs:]
	}
}

// GetIngestionClient returns the ingestion client
func (m *Manager) GetIngestionClient() *client.IngestionClient {
	return m.ingestionClient
}

// countDedupResults counts new and duplicate articles (must hold lock)
func (m *Manager) countDedupResults() (newCount, dupCount int) {
	for _, r := range m.dedupResults {
		switch r.Status {
		case "new":
			newCount++
		case "duplicate":
			dupCount++
		}
	}
	return
}
