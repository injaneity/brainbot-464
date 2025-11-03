package deduplication

import (
	"brainbot/types"
	"fmt"
	"log"
	"strings"
	"time"
)

const (
	SimilarityThreshold float32       = 0.95
	MaxSearchResults    int           = 5
	TTL                 time.Duration = 24 * time.Hour // 24 hours
)

// VectorClient describes the minimal Chroma functionality required by the deduplicator.
type VectorClient interface {
	QuerySimilar(queryText string, nResults int) (*QueryResults, error)
	AddDocument(doc Document) error
	GetDocument(id string) (*GetResults, error)
	UpdateDocument(doc Document) error
	DeleteDocument(id string) error
	Count() (int, error)
	GetEmbeddingModel() string
	Close() error
}

// DeduplicationResult contains the result of deduplication check
type DeduplicationResult struct {
	IsDuplicate     bool      `json:"is_duplicate"`
	MatchingID      string    `json:"matching_id,omitempty"`
	SimilarityScore float32   `json:"similarity_score,omitempty"`
	CheckedAt       time.Time `json:"checked_at"`
}

// Deduplicator handles article deduplication using vector embeddings
type Deduplicator struct {
	vector              VectorClient
	similarityThreshold float32
	maxSearchResults    int
	bloom               *RedisBloom
}

// DeduplicatorConfig holds configuration for the deduplicator
type DeduplicatorConfig struct {
	ChromaConfig        ChromaConfig
	SimilarityThreshold float32 // Default: 0.95 (95%)
	MaxSearchResults    int     // Default: 5
	// Optional Bloom filter configuration. If nil, Bloom checks are disabled.
	BloomConfig *BloomConfig
}

// NewDeduplicator creates a new instance of the deduplicator
func NewDeduplicator(config DeduplicatorConfig) (*Deduplicator, error) {
	cfg := applyConfigDefaults(config)

	// Initialize Chroma connection
	chroma, err := NewChroma(cfg.ChromaConfig)
	if err != nil {
		return nil, fmt.Errorf("failed to initialize Chroma: %w", err)
	}

	// Initialize RedisBloom if configured
	var bloomClient *RedisBloom
	if cfg.BloomConfig != nil {
		b, err := NewRedisBloom(*cfg.BloomConfig)
		if err != nil {
			return nil, fmt.Errorf("failed to initialize RedisBloom: %w", err)
		}
		bloomClient = b
	}

	return &Deduplicator{
		vector:              chroma,
		similarityThreshold: cfg.SimilarityThreshold,
		maxSearchResults:    cfg.MaxSearchResults,
		bloom:               bloomClient,
	}, nil
}

// NewDeduplicatorWithClient constructs a deduplicator from a preconfigured vector client.
func NewDeduplicatorWithClient(client VectorClient, config DeduplicatorConfig) (*Deduplicator, error) {
	if client == nil {
		return nil, fmt.Errorf("vector client cannot be nil")
	}

	cfg := applyConfigDefaults(config)

	var bloomClient *RedisBloom
	if cfg.BloomConfig != nil {
		b, err := NewRedisBloom(*cfg.BloomConfig)
		if err != nil {
			return nil, fmt.Errorf("failed to initialize RedisBloom: %w", err)
		}
		bloomClient = b
	}

	return &Deduplicator{
		vector:              client,
		similarityThreshold: cfg.SimilarityThreshold,
		maxSearchResults:    cfg.MaxSearchResults,
		bloom:               bloomClient,
	}, nil
}

// CheckForDuplicates checks if the given article is a duplicate of existing articles
func (d *Deduplicator) CheckForDuplicates(article *types.Article) (*DeduplicationResult, error) {
	checkTime := time.Now()

	// Fast-path: check probabilistic exact-duplicate filter (URL+title hash)
	if d.bloom != nil {
		if hash, err := NormalizeAndHash(article); err == nil {
			exists, err := d.bloom.Exists(hash)
			if err != nil {
				// Log and continue with vector check on failure to query bloom
				log.Printf("Warning: bloom check failed: %v", err)
			} else if exists {
				// Probable exact duplicate detected within TTL window
				return &DeduplicationResult{
					IsDuplicate: true,
					CheckedAt:   checkTime,
				}, nil
			}
		} else {
			log.Printf("Warning: failed to compute bloom hash: %v", err)
		}
	}

	// Extract full text content for embedding
	content := d.extractFullText(article)
	if content == "" {
		log.Printf("Warning: No content to check for article %s", article.ID)
		return &DeduplicationResult{
			IsDuplicate: false,
			CheckedAt:   checkTime,
		}, nil
	}

	// Search for similar articles
	results, err := d.vector.QuerySimilar(content, d.maxSearchResults)
	if err != nil {
		return nil, fmt.Errorf("failed to query similar articles: %w", err)
	}

	// Find the best result that meets the similarity threshold
	var bestMatch *DeduplicationResult
	var bestSimilarity float32 = 0
	cutoffTime := checkTime.Add(-TTL)

	if len(results.Distances) > 0 && len(results.Distances[0]) > 0 {
		for i, distance := range results.Distances[0] {
			// Convert distance to similarity (assuming cosine distance)
			// Cosine distance = 1 - cosine similarity
			similarity := 1.0 - distance

			if similarity < d.similarityThreshold || len(results.IDs) == 0 || len(results.IDs[0]) <= i {
				continue
			}

			matchingID := results.IDs[0][i]

			var metadata map[string]interface{}
			if len(results.Metadatas) > 0 && len(results.Metadatas[0]) > i {
				metadata = results.Metadatas[0][i]
			}

			lastUpdate, err := resolveLastUpdateTimestamp(metadata)
			if err != nil {
				log.Printf("Warning: skipping candidate %s due to metadata issue: %v", matchingID, err)
				d.deleteDocumentWithLog(matchingID, "invalid or missing TTL metadata")
				continue
			}

			if lastUpdate.Before(cutoffTime) {
				log.Printf("Removing stale article %s last updated at %s (cutoff %s)",
					matchingID, lastUpdate.Format(time.RFC3339), cutoffTime.Format(time.RFC3339))
				d.deleteDocumentWithLog(matchingID, "exceeded TTL")
				continue
			}

			// Check if this is the best match so far
			if similarity > bestSimilarity {
				bestSimilarity = similarity

				bestMatch = &DeduplicationResult{
					IsDuplicate:     true,
					MatchingID:      matchingID,
					SimilarityScore: similarity,
					CheckedAt:       checkTime,
				}
			}
		}
	}

	// If we found a match, update last retrieval time and return it
	if bestMatch != nil {
		err := d.updateLastRetrievalTime(bestMatch.MatchingID)
		if err != nil {
			log.Printf("Warning: failed to update last retrieval time for %s: %v", bestMatch.MatchingID, err)
		}

		log.Printf("Found duplicate article: %s matches %s with %.2f%% similarity",
			article.ID, bestMatch.MatchingID, bestMatch.SimilarityScore*100)

		return bestMatch, nil
	}

	// No duplicates found
	return &DeduplicationResult{
		IsDuplicate: false,
		CheckedAt:   checkTime,
	}, nil
}

// AddArticle adds a new article to the vector database
func (d *Deduplicator) AddArticle(article *types.Article) error {
	content := d.extractFullText(article)
	if content == "" {
		return fmt.Errorf("no content to embed for article %s", article.ID)
	}

	currentTime := time.Now()

	// Create metadata with last retrieval time
	metadata := map[string]interface{}{
		"article_id":        article.ID,
		"title":             article.Title,
		"url":               article.URL,
		"published_at":      article.PublishedAt.Format(time.RFC3339),
		"fetched_at":        article.FetchedAt.Format(time.RFC3339),
		"author":            article.Author,
		"last_retrieved_at": currentTime.Format(time.RFC3339),
		"last_update":       currentTime.Format(time.RFC3339),
		"added_at":          currentTime.Format(time.RFC3339),
	}

	// Chroma v2 REST API may not support arbitrary array types in metadata.
	// Store categories as a comma-separated string when present to avoid
	// deserialization errors.
	if len(article.Categories) > 0 {
		metadata["categories"] = strings.Join(article.Categories, ", ")
	}

	// Create document for Chroma
	doc := Document{
		ID:       article.ID,
		Content:  content,
		Metadata: metadata,
	}

	// Add to vector database
	err := d.vector.AddDocument(doc)
	if err != nil {
		return fmt.Errorf("failed to add article to vector database: %w", err)
	}

	// Add to probabilistic bloom filter (optional)
	if d.bloom != nil {
		if hash, err := NormalizeAndHash(article); err == nil {
			if err := d.bloom.Add(hash); err != nil {
				log.Printf("Warning: failed to add article to bloom filter: %v", err)
			}
		} else {
			log.Printf("Warning: failed to compute bloom hash for add: %v", err)
		}
	}

	log.Printf("Added article %s to vector database", article.ID)
	return nil
}

// ProcessArticle performs both duplicate check and addition if not duplicate
func (d *Deduplicator) ProcessArticle(article *types.Article) (*DeduplicationResult, error) {
	// First check for duplicates
	result, err := d.CheckForDuplicates(article)
	if err != nil {
		return nil, err
	}

	// If not a duplicate, add to database
	if !result.IsDuplicate {
		err := d.AddArticle(article)
		if err != nil {
			return nil, fmt.Errorf("failed to add new article: %w", err)
		}
	}

	return result, nil
}

// extractFullText extracts the most comprehensive text content from the article
func (d *Deduplicator) extractFullText(article *types.Article) string {
	// Priority order: FullContentText > FullContent > Summary > Title
	if article.FullContentText != "" {
		return article.FullContentText
	}
	if article.FullContent != "" {
		return article.FullContent
	}
	if article.Summary != "" {
		return article.Summary
	}
	return article.Title
}

func resolveLastUpdateTimestamp(metadata map[string]interface{}) (time.Time, error) {
	if metadata == nil {
		return time.Time{}, fmt.Errorf("metadata missing")
	}

	timestampKeys := []string{"last_update", "last_retrieved_at", "added_at"}
	var lastErr error

	for _, key := range timestampKeys {
		value, ok := metadata[key]
		if !ok {
			continue
		}

		timestamp, err := parseMetadataTime(value)
		if err != nil {
			lastErr = fmt.Errorf("invalid %s value: %w", key, err)
			continue
		}

		return timestamp, nil
	}

	if lastErr != nil {
		return time.Time{}, lastErr
	}

	return time.Time{}, fmt.Errorf("timestamp metadata not found")
}

func parseMetadataTime(value interface{}) (time.Time, error) {
	switch v := value.(type) {
	case string:
		if strings.TrimSpace(v) == "" {
			return time.Time{}, fmt.Errorf("empty string")
		}
		timestamp, err := time.Parse(time.RFC3339, v)
		if err != nil {
			return time.Time{}, err
		}
		return timestamp, nil
	case time.Time:
		return v, nil
	default:
		return time.Time{}, fmt.Errorf("unsupported time value type %T", value)
	}
}

func (d *Deduplicator) deleteDocumentWithLog(articleID, reason string) {
	if err := d.vector.DeleteDocument(articleID); err != nil {
		log.Printf("Warning: failed to delete document %s (%s): %v", articleID, reason, err)
		return
	}

	log.Printf("Deleted document %s (%s)", articleID, reason)
}

// updateLastRetrievalTime updates the last retrieval timestamp for a document
func (d *Deduplicator) updateLastRetrievalTime(articleID string) error {
	// Get current document to preserve existing metadata
	result, err := d.vector.GetDocument(articleID)
	if err != nil {
		return fmt.Errorf("failed to get document for update: %w", err)
	}

	if len(result.Metadatas) == 0 {
		return fmt.Errorf("document %s not found", articleID)
	}

	// Update the last retrieved timestamp
	metadata := result.Metadatas[0]
	currentTime := time.Now()
	metadata["last_retrieved_at"] = currentTime.Format(time.RFC3339)
	metadata["last_update"] = currentTime.Format(time.RFC3339)

	// Update document
	doc := Document{
		ID:       articleID,
		Metadata: metadata,
	}

	return d.vector.UpdateDocument(doc)
}

// CleanupOldArticles removes articles that haven't been retrieved in the last 24 hours
func (d *Deduplicator) CleanupOldArticles() error {
	cutoffTime := time.Now().Add(-TTL)

	// Query all documents to check their last retrieval time
	// Note: This is a simplified approach. In a production system, you might want
	// to use metadata filtering if Chroma supports it for this use case
	count, err := d.vector.Count()
	if err != nil {
		return fmt.Errorf("failed to get document count: %w", err)
	}

	if count == 0 {
		log.Println("No documents to cleanup")
		return nil
	}

	// For now, we'll need to implement a more sophisticated cleanup mechanism
	// This could involve querying documents and checking metadata
	log.Printf("Cleanup check completed. Found %d documents total", count)
	log.Printf("Cutoff time for cleanup: %s", cutoffTime.Format(time.RFC3339))

	// TODO: Implement actual cleanup logic when Chroma provides better metadata querying
	return nil
}

// Close closes the deduplicator and cleans up resources
func (d *Deduplicator) Close() error {
	var err1 error
	if d.vector != nil {
		err1 = d.vector.Close()
	}
	if d.bloom != nil {
		if err := d.bloom.Close(); err != nil {
			if err1 != nil {
				return fmt.Errorf("vector close error: %v; bloom close error: %w", err1, err)
			}
			return err
		}
	}
	return err1
}

func applyConfigDefaults(config DeduplicatorConfig) DeduplicatorConfig {
	if config.SimilarityThreshold == 0 {
		config.SimilarityThreshold = SimilarityThreshold
	}
	if config.MaxSearchResults == 0 {
		config.MaxSearchResults = MaxSearchResults
	}
	return config
}
