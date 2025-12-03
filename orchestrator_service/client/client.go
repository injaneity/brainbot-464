package client

import (
	"net/http"
	"os"
	"time"
)

// IngestionClient represents the client for the ingestion service API
type IngestionClient struct {
	baseURL    string
	httpClient *http.Client
}

// NewIngestionClient creates a new ingestion service client
func NewIngestionClient(baseURL string) *IngestionClient {
	if baseURL == "" {
		baseURL = getEnvOrDefault("API_URL", "http://ingestion-service:8080")
	}
	return &IngestionClient{
		baseURL:    baseURL,
		httpClient: &http.Client{Timeout: 30 * time.Second},
	}
}

// getEnvOrDefault returns the value of an environment variable or a default value
func getEnvOrDefault(key, defaultVal string) string {
	if val := os.Getenv(key); val != "" {
		return val
	}
	return defaultVal
}
