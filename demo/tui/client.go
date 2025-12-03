package tui

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"time"
)

// OrchestratorClient is a thin HTTP client for the orchestrator API
type OrchestratorClient struct {
	baseURL string
	client  *http.Client
}

// NewOrchestratorClient creates a new orchestrator client
func NewOrchestratorClient(baseURL string) *OrchestratorClient {
	return &OrchestratorClient{
		baseURL: baseURL,
		client: &http.Client{
			Timeout: 5 * time.Second,
		},
	}
}

// GetStatus fetches the current status from the orchestrator
func (c *OrchestratorClient) GetStatus() (*StatusResponse, error) {
	resp, err := c.client.Get(c.baseURL + "/api/status")
	if err != nil {
		return nil, fmt.Errorf("failed to get status: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return nil, fmt.Errorf("server returned %d: %s", resp.StatusCode, string(body))
	}

	var status StatusResponse
	if err := json.NewDecoder(resp.Body).Decode(&status); err != nil {
		return nil, fmt.Errorf("failed to decode response: %w", err)
	}

	return &status, nil
}

// ResetAndFetch triggers the workflow on the orchestrator (clears cache)
func (c *OrchestratorClient) ResetAndFetch() error {
	resp, err := c.client.Post(c.baseURL+"/api/start", "application/json", bytes.NewReader([]byte("{}")))
	if err != nil {
		return fmt.Errorf("failed to start workflow: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusAccepted {
		body, _ := io.ReadAll(resp.Body)
		return fmt.Errorf("server returned %d: %s", resp.StatusCode, string(body))
	}

	return nil
}

// FetchNew triggers the refresh workflow on the orchestrator (keeps cache)
func (c *OrchestratorClient) FetchNew() error {
	resp, err := c.client.Post(c.baseURL+"/api/refresh", "application/json", bytes.NewReader([]byte("{}")))
	if err != nil {
		return fmt.Errorf("failed to start refresh workflow: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusAccepted {
		body, _ := io.ReadAll(resp.Body)
		return fmt.Errorf("server returned %d: %s", resp.StatusCode, string(body))
	}

	return nil
}
