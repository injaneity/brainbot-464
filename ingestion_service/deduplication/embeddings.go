package deduplication

import (
	"bytes"
	"context"
	"crypto/tls"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"os"
	"strings"
	"time"

	cohere "github.com/cohere-ai/cohere-go/v2"
	cohereclient "github.com/cohere-ai/cohere-go/v2/client"
)

// EmbeddingsProvider abstracts a text->embedding generator
// Implementations should return one embedding vector per input text.
type EmbeddingsProvider interface {
	EmbedTexts(texts []string) ([][]float32, error)
	ModelName() string
}

// NewDefaultEmbeddingsProvider returns an embeddings provider if configured via env
// Currently supports OpenAI when OPENAI_API_KEY is set.
func NewDefaultEmbeddingsProvider(preferredModel string) EmbeddingsProvider {
	// Prefer Cohere if configured
	if cohereKey := os.Getenv("COHERE_API_KEY"); cohereKey != "" {
		model := preferredModel
		if model == "" || !strings.HasPrefix(model, "embed-") {
			// Reasonable default for Cohere v3 embeddings; choose english by default
			model = "embed-english-v3.0"
		}
		// Create a custom HTTP client that forces HTTP/1.1 to avoid HTTP/2 protocol errors
		httpClient := &http.Client{
			Timeout: 60 * time.Second,
			Transport: &http.Transport{
				TLSNextProto: make(map[string]func(authority string, c *tls.Conn) http.RoundTripper),
				ForceAttemptHTTP2: false,
			},
		}
		client := cohereclient.NewClient(
			cohereclient.WithToken(cohereKey),
			cohereclient.WithHTTPClient(httpClient),
		)
		return &CohereEmbeddings{client: client, model: model}
	}

	apiKey := os.Getenv("OPENAI_API_KEY")
	if apiKey != "" {
		model := preferredModel
		if model == "" {
			// Reasonable default for OpenAI embeddings
			model = "text-embedding-3-small"
		}
		return &OpenAIEmbeddings{apiKey: apiKey, model: model}
	}
	return nil
}

// CohereEmbeddings implements EmbeddingsProvider using the Cohere Embed API (v2)
// Docs: https://docs.cohere.com/reference/embed
// SDK: github.com/cohere-ai/cohere-go/v2
type CohereEmbeddings struct {
	client *cohereclient.Client
	model  string
}

func (c *CohereEmbeddings) ModelName() string { return c.model }

func (c *CohereEmbeddings) EmbedTexts(texts []string) ([][]float32, error) {
	if len(texts) == 0 {
		return [][]float32{}, nil
	}

	// Use a short per-request timeout to avoid hanging
	ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
	defer cancel()

	// Use the V2.Embed API which has better HTTP/2 handling
	resp, err := c.client.V2.Embed(
		ctx,
		&cohere.V2EmbedRequest{
			Texts:          texts,
			Model:          c.model,
			InputType:      cohere.EmbedInputTypeSearchDocument,
			EmbeddingTypes: []cohere.EmbeddingType{cohere.EmbeddingTypeFloat},
		},
	)
	if err != nil {
		return nil, fmt.Errorf("cohere embed error: %w", err)
	}
	if resp == nil {
		return nil, errors.New("cohere embed returned empty response")
	}

	// Extract float embeddings from the response
	if resp.Embeddings == nil || resp.Embeddings.Float == nil {
		return nil, errors.New("cohere embed returned no float embeddings")
	}

	floats := resp.Embeddings.Float
	if len(floats) != len(texts) {
		return nil, errors.New("embedding count mismatch")
	}

	out := make([][]float32, len(floats))
	for i, vec := range floats {
		fv := make([]float32, len(vec))
		for j, v := range vec {
			fv[j] = float32(v)
		}
		out[i] = fv
	}
	return out, nil
}

// OpenAIEmbeddings implements EmbeddingsProvider using the OpenAI Embeddings API
// Docs: https://platform.openai.com/docs/guides/embeddings
// Endpoint: POST https://api.openai.com/v1/embeddings
// Request: {"input": ["text1", ...], "model": "text-embedding-3-small"}
// Response: {"data": [{"embedding": [...], "index": 0}, ...]}
type OpenAIEmbeddings struct {
	apiKey   string
	model    string
	endpoint string
}

func (o *OpenAIEmbeddings) ModelName() string { return o.model }

func (o *OpenAIEmbeddings) EmbedTexts(texts []string) ([][]float32, error) {
	if len(texts) == 0 {
		return [][]float32{}, nil
	}

	endpoint := o.endpoint
	if endpoint == "" {
		endpoint = "https://api.openai.com/v1/embeddings"
	}

	payload := map[string]interface{}{
		"input": texts,
		"model": o.model,
	}
	b, err := json.Marshal(payload)
	if err != nil {
		return nil, err
	}

	req, err := http.NewRequest("POST", endpoint, bytes.NewBuffer(b))
	if err != nil {
		return nil, err
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", fmt.Sprintf("Bearer %s", o.apiKey))
	if org := os.Getenv("OPENAI_ORG_ID"); org != "" {
		req.Header.Set("OpenAI-Organization", org)
	}

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		var body map[string]interface{}
		_ = json.NewDecoder(resp.Body).Decode(&body)
		return nil, fmt.Errorf("openai embeddings error: status %d: %v", resp.StatusCode, body)
	}

	var parsed struct {
		Data []struct {
			Embedding []float64 `json:"embedding"`
			Index     int       `json:"index"`
		} `json:"data"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&parsed); err != nil {
		return nil, err
	}
	if len(parsed.Data) != len(texts) {
		return nil, errors.New("embedding count mismatch")
	}

	out := make([][]float32, len(parsed.Data))
	for i, d := range parsed.Data {
		vec := make([]float32, len(d.Embedding))
		for j, v := range d.Embedding {
			vec[j] = float32(v)
		}
		out[i] = vec
	}
	return out, nil
}
