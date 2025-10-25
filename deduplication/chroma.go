package deduplication

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
)

// Chroma wraps the Chroma vector database REST API
type Chroma struct {
	baseURL        string
	tenant         string
	database       string
	collectionName string
	collectionID   string
	httpClient     *http.Client
	embeddingModel string
	embedder       EmbeddingsProvider
}

// ChromaConfig holds configuration for Chroma connection
type ChromaConfig struct {
	Host           string
	Port           int
	CollectionName string
	EmbeddingModel string
}

// Document represents a document to be stored in Chroma
type Document struct {
	ID       string
	Content  string
	Metadata map[string]interface{}
}

// QueryResults represents the response from a similarity query
type QueryResults struct {
	IDs        [][]string                 `json:"ids"`
	Distances  [][]float32                `json:"distances"`
	Metadatas  [][]map[string]interface{} `json:"metadatas"`
	Documents  [][]string                 `json:"documents"`
	Embeddings interface{}                `json:"embeddings"`
}

// GetResults represents the response from a get request
type GetResults struct {
	IDs        []string                 `json:"ids"`
	Metadatas  []map[string]interface{} `json:"metadatas"`
	Documents  []string                 `json:"documents"`
	Embeddings interface{}              `json:"embeddings"`
}

// NewChroma creates a new Chroma wrapper instance
func NewChroma(config ChromaConfig) (*Chroma, error) {
	baseURL := fmt.Sprintf("http://%s:%d/api/v2", config.Host, config.Port)

	// Use default embedding model if none specified
	embeddingModel := getDefaultEmbeddingModel(config.EmbeddingModel)

	wrapper := &Chroma{
		baseURL:        baseURL,
		tenant:         "default_tenant",
		database:       "default_database",
		collectionName: config.CollectionName,
		httpClient:     &http.Client{},
		embeddingModel: embeddingModel,
	}

	// Initialize an embeddings provider (required for Chroma v2 REST API)
	// Chroma v2 expects client-supplied embeddings (query_embeddings, embeddings).
	wrapper.embedder = NewDefaultEmbeddingsProvider(wrapper.embeddingModel)
	if wrapper.embedder == nil {
		return nil, fmt.Errorf("no embeddings provider configured. Set COHERE_API_KEY or OPENAI_API_KEY (and optionally embedding model) to enable client-side embeddings required by Chroma v2")
	}
	log.Printf("Using embeddings provider: %s", wrapper.embedder.ModelName())

	// Get or create collection
	collectionID, err := wrapper.getOrCreateCollection(config.CollectionName)
	if err != nil {
		return nil, fmt.Errorf("failed to get or create collection: %w", err)
	}

	wrapper.collectionID = collectionID
	return wrapper, nil
}

// GetEmbeddingModel returns the current embedding model
func (c *Chroma) GetEmbeddingModel() string {
	return c.embeddingModel
}

// SetEmbeddingModel updates the embedding model (for future use)
func (c *Chroma) SetEmbeddingModel(model string) {
	c.embeddingModel = model
}

// getDefaultEmbeddingModel returns a default embedding model if none is specified
func getDefaultEmbeddingModel(model string) string {
	if model == "" {
		return "sentence-transformers/all-MiniLM-L6-v2" // Default model
	}
	return model
}

// getOrCreateCollection gets an existing collection or creates a new one
func (c *Chroma) getOrCreateCollection(name string) (string, error) {
	// Try to get existing collection
	url := fmt.Sprintf("%s/tenants/%s/databases/%s/collections/%s", c.baseURL, c.tenant, c.database, name)
	resp, err := c.httpClient.Get(url)

	if err == nil && resp.StatusCode == http.StatusOK {
		defer resp.Body.Close()
		var result map[string]interface{}
		if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
			return "", err
		}
		log.Printf("Using existing collection: %s", name)
		return result["id"].(string), nil
	}

	// Create new collection with embedding function
	log.Printf("Creating new collection: %s", name)
	createURL := fmt.Sprintf("%s/tenants/%s/databases/%s/collections", c.baseURL, c.tenant, c.database)
	payload := map[string]interface{}{
		"name": name,
		"metadata": map[string]interface{}{
			"description": "BrainBot article deduplication collection",
		},
		"get_or_create": true,
	}

	jsonData, err := json.Marshal(payload)
	if err != nil {
		return "", err
	}

	resp, err = c.httpClient.Post(createURL, "application/json", bytes.NewBuffer(jsonData))
	if err != nil {
		return "", fmt.Errorf("failed to create collection: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK && resp.StatusCode != http.StatusCreated {
		body, _ := io.ReadAll(resp.Body)
		return "", fmt.Errorf("failed to create collection (status %d): %s", resp.StatusCode, string(body))
	}

	var result map[string]interface{}
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return "", err
	}

	if err := json.Unmarshal(body, &result); err != nil {
		return "", fmt.Errorf("failed to parse response: %w, body: %s", err, string(body))
	}

	return result["id"].(string), nil
}

// collectionURL returns the base URL for collection operations
func (c *Chroma) collectionURL() string {
	return fmt.Sprintf("%s/tenants/%s/databases/%s/collections/%s", c.baseURL, c.tenant, c.database, c.collectionID)
}

// AddDocument adds a single document to the collection
func (c *Chroma) AddDocument(doc Document) error {
	documents := []string{doc.Content}
	metadatas := []map[string]interface{}{doc.Metadata}
	ids := []string{doc.ID}

	url := fmt.Sprintf("%s/add", c.collectionURL())
	payload := map[string]interface{}{
		"documents": documents,
		"metadatas": metadatas,
		"ids":       ids,
	}

	// Generate embeddings client-side to comply with Chroma v2
	if c.embedder == nil {
		return fmt.Errorf("embeddings provider not configured")
	}
	embs, err := c.embedder.EmbedTexts(documents)
	if err != nil {
		return fmt.Errorf("failed to generate embeddings: %w", err)
	}
	payload["embeddings"] = embs

	jsonData, err := json.Marshal(payload)
	if err != nil {
		return err
	}

	resp, err := c.httpClient.Post(url, "application/json", bytes.NewBuffer(jsonData))
	if err != nil {
		return fmt.Errorf("failed to add document: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK && resp.StatusCode != http.StatusCreated {
		body, _ := io.ReadAll(resp.Body)
		return fmt.Errorf("failed to add document: %s", string(body))
	}

	log.Printf("Added document with ID: %s", doc.ID)
	return nil
}

// AddDocuments adds multiple documents to the collection
func (c *Chroma) AddDocuments(docs []Document) error {
	if len(docs) == 0 {
		return nil
	}

	documents := make([]string, len(docs))
	metadatas := make([]map[string]interface{}, len(docs))
	ids := make([]string, len(docs))

	for i, doc := range docs {
		documents[i] = doc.Content
		metadatas[i] = doc.Metadata
		ids[i] = doc.ID
	}

	url := fmt.Sprintf("%s/add", c.collectionURL())
	payload := map[string]interface{}{
		"documents": documents,
		"metadatas": metadatas,
		"ids":       ids,
	}

	if c.embedder == nil {
		return fmt.Errorf("embeddings provider not configured")
	}
	embs, err := c.embedder.EmbedTexts(documents)
	if err != nil {
		return fmt.Errorf("failed to generate embeddings: %w", err)
	}
	payload["embeddings"] = embs

	jsonData, err := json.Marshal(payload)
	if err != nil {
		return err
	}

	resp, err := c.httpClient.Post(url, "application/json", bytes.NewBuffer(jsonData))
	if err != nil {
		return fmt.Errorf("failed to add documents: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK && resp.StatusCode != http.StatusCreated {
		body, _ := io.ReadAll(resp.Body)
		return fmt.Errorf("failed to add documents: %s", string(body))
	}

	log.Printf("Added %d documents to collection", len(docs))
	return nil
}

// QuerySimilar searches for similar documents
func (c *Chroma) QuerySimilar(queryText string, nResults int) (*QueryResults, error) {
	url := fmt.Sprintf("%s/query", c.collectionURL())
	payload := map[string]interface{}{
		"n_results": nResults,
		// Explicitly request fields commonly needed
		"include": []string{"metadatas", "documents", "distances", "embeddings", "uris"},
	}

	if c.embedder == nil {
		return nil, fmt.Errorf("embeddings provider not configured")
	}
	embs, err := c.embedder.EmbedTexts([]string{queryText})
	if err != nil {
		return nil, fmt.Errorf("failed to generate query embeddings: %w", err)
	}
	payload["query_embeddings"] = embs

	jsonData, err := json.Marshal(payload)
	if err != nil {
		return nil, err
	}

	resp, err := c.httpClient.Post(url, "application/json", bytes.NewBuffer(jsonData))
	if err != nil {
		return nil, fmt.Errorf("failed to query collection: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return nil, fmt.Errorf("failed to query collection: %s", string(body))
	}

	var result QueryResults
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return nil, err
	}

	return &result, nil
}

// QuerySimilarWithMetadata searches for similar documents with metadata filtering
func (c *Chroma) QuerySimilarWithMetadata(queryText string, nResults int, where map[string]interface{}) (*QueryResults, error) {
	url := fmt.Sprintf("%s/query", c.collectionURL())
	payload := map[string]interface{}{
		"n_results": nResults,
		"where":     where,
		"include":   []string{"metadatas", "documents", "distances", "embeddings", "uris"},
	}

	if c.embedder == nil {
		return nil, fmt.Errorf("embeddings provider not configured")
	}
	embs, err := c.embedder.EmbedTexts([]string{queryText})
	if err != nil {
		return nil, fmt.Errorf("failed to generate query embeddings: %w", err)
	}
	payload["query_embeddings"] = embs

	jsonData, err := json.Marshal(payload)
	if err != nil {
		return nil, err
	}

	resp, err := c.httpClient.Post(url, "application/json", bytes.NewBuffer(jsonData))
	if err != nil {
		return nil, fmt.Errorf("failed to query collection with metadata: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return nil, fmt.Errorf("failed to query collection with metadata: %s", string(body))
	}

	var result QueryResults
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return nil, err
	}

	return &result, nil
}

// GetDocument retrieves a document by ID
func (c *Chroma) GetDocument(id string) (*GetResults, error) {
	url := fmt.Sprintf("%s/get", c.collectionURL())
	payload := map[string]interface{}{
		"ids": []string{id},
	}

	jsonData, err := json.Marshal(payload)
	if err != nil {
		return nil, err
	}

	resp, err := c.httpClient.Post(url, "application/json", bytes.NewBuffer(jsonData))
	if err != nil {
		return nil, fmt.Errorf("failed to get document: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return nil, fmt.Errorf("failed to get document: %s", string(body))
	}

	var result GetResults
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return nil, err
	}

	return &result, nil
}

// DeleteDocument removes a document by ID
func (c *Chroma) DeleteDocument(id string) error {
	url := fmt.Sprintf("%s/delete", c.collectionURL())
	payload := map[string]interface{}{
		"ids": []string{id},
	}

	jsonData, err := json.Marshal(payload)
	if err != nil {
		return err
	}

	resp, err := c.httpClient.Post(url, "application/json", bytes.NewBuffer(jsonData))
	if err != nil {
		return fmt.Errorf("failed to delete document: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return fmt.Errorf("failed to delete document: %s", string(body))
	}

	log.Printf("Deleted document with ID: %s", id)
	return nil
}

// UpdateDocument updates an existing document
func (c *Chroma) UpdateDocument(doc Document) error {
	url := fmt.Sprintf("%s/update", c.collectionURL())
	payload := map[string]interface{}{
		"ids":       []string{doc.ID},
		"metadatas": []map[string]interface{}{doc.Metadata},
	}

	jsonData, err := json.Marshal(payload)
	if err != nil {
		return err
	}

	resp, err := c.httpClient.Post(url, "application/json", bytes.NewBuffer(jsonData))
	if err != nil {
		return fmt.Errorf("failed to update document: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return fmt.Errorf("failed to update document: %s", string(body))
	}

	log.Printf("Updated document with ID: %s", doc.ID)
	return nil
}

// Count returns the number of documents in the collection
func (c *Chroma) Count() (int, error) {
	url := fmt.Sprintf("%s/count", c.collectionURL())

	resp, err := c.httpClient.Get(url)
	if err != nil {
		return 0, fmt.Errorf("failed to count documents: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return 0, fmt.Errorf("failed to count documents: %s", string(body))
	}

	var count int
	if err := json.NewDecoder(resp.Body).Decode(&count); err != nil {
		return 0, err
	}

	return count, nil
}

// CheckSimilarity finds documents similar to the given content with a similarity threshold
func (c *Chroma) CheckSimilarity(content string, threshold float32, maxResults int) (*QueryResults, error) {
	results, err := c.QuerySimilar(content, maxResults)
	if err != nil {
		return nil, err
	}

	// Note: Similarity filtering would depend on the actual distance values in results
	// For now, returning all results - you can add threshold logic based on distances
	return results, nil
}

// Close cleans up the wrapper (if needed)
func (c *Chroma) Close() error {
	// The chroma-go client doesn't seem to have an explicit close method
	// This is here for future compatibility or cleanup if needed
	log.Println("Chroma closed")
	return nil
}
