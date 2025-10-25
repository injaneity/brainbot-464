package deduplication

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"os"
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
