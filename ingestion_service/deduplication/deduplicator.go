package deduplication

import (
	"brainbot/ingestion_service/types"
	"context"
	"fmt"
	"log"
	"strings"
	"time"

	"github.com/redis/go-redis/v9"
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
type DeduplicationResult = types.DeduplicationResult

// Deduplicator handles article deduplication using vector embeddings and Redis Bloom filter
type Deduplicator struct {
	vector              VectorClient
	redis               *redis.Client
	similarityThreshold float32
	maxSearchResults    int
}

// DeduplicatorConfig holds configuration for the deduplicator
type DeduplicatorConfig struct {
	ChromaConfig        ChromaConfig
	RedisConfig         RedisConfig
	SimilarityThreshold float32 // Default: 0.95 (95%)
	MaxSearchResults    int     // Default: 5
}

type RedisConfig struct {
	Addr     string
	Password string
	DB       int
}

// NewDeduplicator creates a new instance of the deduplicator
func NewDeduplicator(config DeduplicatorConfig) (*Deduplicator, error) {
	cfg := applyConfigDefaults(config)

	// Initialize Chroma connection
	chroma, err := NewChroma(cfg.ChromaConfig)
	if err != nil {
		return nil, fmt.Errorf("failed to initialize Chroma: %w", err)
	}

	// Initialize Redis connection
	rdb := redis.NewClient(&redis.Options{
		Addr:     cfg.RedisConfig.Addr,
		Password: cfg.RedisConfig.Password,
		DB:       cfg.RedisConfig.DB,
	})

	// Test Redis connection
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	if err := rdb.Ping(ctx).Err(); err != nil {
		log.Printf("Warning: Failed to connect to Redis: %v. Bloom filter will be disabled.", err)
	}

	return &Deduplicator{
		vector:              chroma,
		redis:               rdb,
		similarityThreshold: cfg.SimilarityThreshold,
		maxSearchResults:    cfg.MaxSearchResults,
	}, nil
}

// NewDeduplicatorWithClient constructs a deduplicator from a preconfigured vector client.
func NewDeduplicatorWithClient(client VectorClient, config DeduplicatorConfig) (*Deduplicator, error) {
	if client == nil {
		return nil, fmt.Errorf("vector client cannot be nil")
	}

	cfg := applyConfigDefaults(config)

	return &Deduplicator{
		vector:              client,
		similarityThreshold: cfg.SimilarityThreshold,
		maxSearchResults:    cfg.MaxSearchResults,
	}, nil
}

// CheckExactDuplicate checks if the article is an exact duplicate using Redis Bloom filter
func (d *Deduplicator) CheckExactDuplicate(ctx context.Context, article *types.Article) (bool, error) {
	if d.redis == nil {
		return false, nil
	}

	// Check URL
	existsURL, err := d.redis.Do(ctx, "BF.EXISTS", "articles:bloom:url", article.URL).Bool()
	if err != nil {
		return false, fmt.Errorf("failed to check URL in bloom filter: %w", err)
	}
	if existsURL {
		return true, nil
	}

	// Check Title
	existsTitle, err := d.redis.Do(ctx, "BF.EXISTS", "articles:bloom:title", article.Title).Bool()
	if err != nil {
		return false, fmt.Errorf("failed to check Title in bloom filter: %w", err)
	}
	if existsTitle {
		return true, nil
	}

	return false, nil
}

// AddExactDuplicate adds the article to the Redis Bloom filter
func (d *Deduplicator) AddExactDuplicate(ctx context.Context, article *types.Article) error {
	if d.redis == nil {
		return nil
	}

	// Add URL
	_, err := d.redis.Do(ctx, "BF.ADD", "articles:bloom:url", article.URL).Result()
	if err != nil {
		return fmt.Errorf("failed to add URL to bloom filter: %w", err)
	}

	// Add Title
	_, err = d.redis.Do(ctx, "BF.ADD", "articles:bloom:title", article.Title).Result()
	if err != nil {
		return fmt.Errorf("failed to add Title to bloom filter: %w", err)
	}

	return nil
}

// CheckForDuplicates checks if the given article is a duplicate of existing articles
func (d *Deduplicator) CheckForDuplicates(article *types.Article) (*DeduplicationResult, error) {
	checkTime := time.Now()

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

	log.Printf("Added article %s to vector database", article.ID)
	return nil
}

// ProcessArticle performs both duplicate check and addition if not duplicate
func (d *Deduplicator) ProcessArticle(ctx context.Context, article *types.Article) (*DeduplicationResult, error) {
	// 1. Check Exact Duplicate (Redis Bloom)
	isExact, err := d.CheckExactDuplicate(ctx, article)
	if err != nil {
		log.Printf("Warning: Redis Bloom check failed: %v", err)
	}
	if isExact {
		log.Printf("Exact duplicate found for article %s (URL/Title)", article.ID)
		return &DeduplicationResult{
			IsDuplicate:      true,
			IsExactDuplicate: true,
			MatchingID:       article.ID,
			CheckedAt:        time.Now(),
		}, nil
	}

	// 2. Check Vector Duplicates
	result, err := d.CheckForDuplicates(article)
	if err != nil {
		return nil, err
	}

	// 3. Add to Bloom Filter (regardless of whether it's similar or new, as long as it's not exact duplicate)
	if err := d.AddExactDuplicate(ctx, article); err != nil {
		log.Printf("Warning: Failed to add to Bloom Filter: %v", err)
	}

	// 4. If not duplicate (similar), add to Vector DB
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
	if d.redis != nil {
		d.redis.Close()
	}
	return d.vector.Close()
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
