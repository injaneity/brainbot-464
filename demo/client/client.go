package client

import (
	"net/http"
	"os"
	"time"
)

// Client represents the BrainBot application client
type Client struct {
	baseURL    string
	httpClient *http.Client
}

// NewClient creates a new application client
func NewClient(baseURL string) *Client {
	if baseURL == "" {
		baseURL = "http://localhost:8080"
	}
	return &Client{
		baseURL:    baseURL,
		httpClient: &http.Client{Timeout: 30 * time.Second},
	}
}

// GetEnvOrDefault returns the value of an environment variable or a default value
func GetEnvOrDefault(key, defaultVal string) string {
	if val := os.Getenv(key); val != "" {
		return val
	}
	return defaultVal
}
