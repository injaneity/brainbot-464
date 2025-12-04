package api

import (
	"brainbot/ingestion_service/deduplication"
	"brainbot/ingestion_service/storage"
	"brainbot/ingestion_service/types"
	"context"
	"log"
	"net/http"
	"os"
	"strconv"
	"time"

	"github.com/gin-gonic/gin"
)

// RegisterDeduplicationRoutes registers deduplication service endpoints.
func RegisterDeduplicationRoutes(r *gin.Engine) {
	g := r.Group("/api/deduplication")
	g.POST("/check", handleCheckDuplicate)
	g.POST("/add", handleAddArticle)
	g.POST("/process", handleProcessArticle)
	g.DELETE("/clear", handleClearCache)
	g.GET("/count", handleGetCount)
}

// CheckDuplicateRequest represents the request to check for duplicates
type CheckDuplicateRequest struct {
	Article *types.Article `json:"article" binding:"required"`
}

// CheckDuplicateResponse represents the response from duplicate check
type CheckDuplicateResponse struct {
	IsDuplicate     bool      `json:"is_duplicate"`
	MatchingID      string    `json:"matching_id,omitempty"`
	SimilarityScore float32   `json:"similarity_score,omitempty"`
	CheckedAt       time.Time `json:"checked_at"`
}

// AddArticleRequest represents the request to add an article
type AddArticleRequest struct {
	Article *types.Article `json:"article" binding:"required"`
}

// ProcessArticleRequest represents the request to process (check + add if new)
type ProcessArticleRequest struct {
	Article *types.Article `json:"article" binding:"required"`
}

// ProcessArticleResponse represents the response from processing an article
type ProcessArticleResponse struct {
	Status              string                             `json:"status"` // "new", "duplicate", "error"
	DeduplicationResult *deduplication.DeduplicationResult `json:"deduplication_result,omitempty"`
	PresignedURL        string                             `json:"presigned_url,omitempty"`
	Error               string                             `json:"error,omitempty"`
}

// handleCheckDuplicate checks if an article is a duplicate
func handleCheckDuplicate(c *gin.Context) {
	var req CheckDuplicateRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	deduplicator, err := initializeDeduplicator()
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to initialize deduplicator: " + err.Error()})
		return
	}
	defer deduplicator.Close()

	result, err := deduplicator.CheckForDuplicates(req.Article)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to check duplicates: " + err.Error()})
		return
	}

	response := CheckDuplicateResponse{
		IsDuplicate:     result.IsDuplicate,
		MatchingID:      result.MatchingID,
		SimilarityScore: result.SimilarityScore,
		CheckedAt:       result.CheckedAt,
	}

	c.JSON(http.StatusOK, response)
}

// handleAddArticle adds an article to the vector database
func handleAddArticle(c *gin.Context) {
	var req AddArticleRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	deduplicator, err := initializeDeduplicator()
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to initialize deduplicator: " + err.Error()})
		return
	}
	defer deduplicator.Close()

	err = deduplicator.AddArticle(req.Article)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to add article: " + err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"status":     "added",
		"article_id": req.Article.ID,
	})
}

// handleProcessArticle processes an article (checks for duplicates and adds if new)
func handleProcessArticle(c *gin.Context) {
	var req ProcessArticleRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	deduplicator, err := initializeDeduplicator()
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to initialize deduplicator: " + err.Error()})
		return
	}
	defer deduplicator.Close()

	s3Client, err := initializeS3(c.Request.Context())
	if err != nil {
		log.Printf("Warning: Failed to initialize S3: %v", err)
		// Proceeding without S3 might be critical failure depending on requirements.
		// User said "we create a new s3 object", so it seems required.
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to initialize S3: " + err.Error()})
		return
	}

	result, err := deduplicator.ProcessArticle(c.Request.Context(), req.Article)
	if err != nil {
		response := ProcessArticleResponse{
			Status: "error",
			Error:  err.Error(),
		}
		c.JSON(http.StatusInternalServerError, response)
		return
	}

	status := "new"
	var presignedURL string

	// Determine content to use
	content := req.Article.FullContentText
	if content == "" {
		content = req.Article.FullContent
	}
	if content == "" {
		content = req.Article.Summary
	}

	if result.IsExactDuplicate {
		status = "duplicate"
		// Exact duplicate: Do nothing with S3
	} else if result.IsDuplicate {
		status = "duplicate"
		// Similar duplicate: Append to existing S3 object
		if result.MatchingID != "" {
			err := s3Client.AppendToArticleObject(c.Request.Context(), result.MatchingID, content)
			if err != nil {
				log.Printf("Error appending to S3 for article %s (match %s): %v", req.Article.ID, result.MatchingID, err)
				// We don't fail the request, but log the error
			}
		}
	} else {
		// New article
		status = "new"
		// Create new S3 object
		err := s3Client.CreateArticleObject(c.Request.Context(), req.Article.ID, req.Article.Title, content)
		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to create S3 object: " + err.Error()})
			return
		}

		// Generate Pre-signed URL
		presignedURL, err = s3Client.GeneratePresignedURL(c.Request.Context(), req.Article.ID, 15*time.Minute)
		if err != nil {
			log.Printf("Error generating presigned URL for article %s: %v", req.Article.ID, err)
		}
	}

	response := ProcessArticleResponse{
		Status:              status,
		DeduplicationResult: result,
		PresignedURL:        presignedURL,
	}

	c.JSON(http.StatusOK, response)
}

// handleClearCache clears all documents from the ChromaDB collection
func handleClearCache(c *gin.Context) {
	chromaConfig := deduplication.ChromaConfig{
		Host:           getEnvOrDefault("CHROMA_HOST", "localhost"),
		Port:           getEnvPortOrDefault("CHROMA_PORT", 8000),
		CollectionName: getEnvOrDefault("CHROMA_COLLECTION", "brainbot_articles"),
		EmbeddingModel: "",
	}

	chroma, err := deduplication.NewChromaReadOnly(chromaConfig)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to connect to ChromaDB: " + err.Error()})
		return
	}
	defer chroma.Close()

	err = chroma.ClearCollection()
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to clear cache: " + err.Error()})
		return
	}

	// Also clear Redis Bloom filter keys
	redisConfig := deduplication.RedisConfig{
		Addr:     getEnvOrDefault("REDIS_ADDR", "localhost:6379"),
		Password: getEnvOrDefault("REDIS_PASSWORD", ""),
		DB:       getEnvPortOrDefault("REDIS_DB", 0),
	}

	deduplicator, err := deduplication.NewDeduplicator(deduplication.DeduplicatorConfig{
		ChromaConfig: chromaConfig,
		RedisConfig:  redisConfig,
	})
	if err == nil {
		defer deduplicator.Close()
		// We need to expose a method to clear Redis keys in Deduplicator, or do it manually here.
		// Since Deduplicator struct has private redis client, we should add a method there.
		// For now, let's assume we can add a method ClearBloomFilter to Deduplicator.
		if err := deduplicator.ClearBloomFilter(c.Request.Context()); err != nil {
			log.Printf("Warning: Failed to clear Redis Bloom filter: %v", err)
		}
	} else {
		log.Printf("Warning: Failed to connect to Redis for clearing: %v", err)
	}

	c.JSON(http.StatusOK, gin.H{
		"status": "cleared",
	})
}

// handleGetCount returns the number of documents in the collection
func handleGetCount(c *gin.Context) {
	chromaConfig := deduplication.ChromaConfig{
		Host:           getEnvOrDefault("CHROMA_HOST", "localhost"),
		Port:           getEnvPortOrDefault("CHROMA_PORT", 8000),
		CollectionName: getEnvOrDefault("CHROMA_COLLECTION", "brainbot_articles"),
		EmbeddingModel: "",
	}

	chroma, err := deduplication.NewChromaReadOnly(chromaConfig)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to connect to ChromaDB: " + err.Error()})
		return
	}
	defer chroma.Close()

	count, err := chroma.Count()
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to get count: " + err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"count": count,
	})
}

// Helper function to initialize deduplicator with configuration from environment
func initializeDeduplicator() (*deduplication.Deduplicator, error) {
	chromaConfig := deduplication.ChromaConfig{
		Host:           getEnvOrDefault("CHROMA_HOST", "localhost"),
		Port:           getEnvPortOrDefault("CHROMA_PORT", 8000),
		CollectionName: getEnvOrDefault("CHROMA_COLLECTION", "brainbot_articles"),
		EmbeddingModel: "",
	}

	redisConfig := deduplication.RedisConfig{
		Addr:     getEnvOrDefault("REDIS_ADDR", "localhost:6379"),
		Password: getEnvOrDefault("REDIS_PASSWORD", ""),
		DB:       getEnvPortOrDefault("REDIS_DB", 0),
	}

	deduplicatorConfig := deduplication.DeduplicatorConfig{
		ChromaConfig:        chromaConfig,
		RedisConfig:         redisConfig,
		SimilarityThreshold: 0, // Use default
		MaxSearchResults:    0, // Use default
	}

	return deduplication.NewDeduplicator(deduplicatorConfig)
}

func initializeS3(ctx context.Context) (*storage.S3Client, error) {
	bucket := getEnvOrDefault("S3_BUCKET", "")
	if bucket == "" {
		return nil, os.ErrNotExist // Or custom error "S3_BUCKET not set"
	}
	prefix := getEnvOrDefault("S3_PREFIX", "")
	region := getEnvOrDefault("S3_REGION", "us-east-1")

	return storage.NewS3Client(ctx, bucket, prefix, region)
}

func getEnvOrDefault(key, defaultVal string) string {
	if val := os.Getenv(key); val != "" {
		return val
	}
	return defaultVal
}

func getEnvPortOrDefault(key string, defaultVal int) int {
	if val := os.Getenv(key); val != "" {
		if port, err := strconv.Atoi(val); err == nil {
			return port
		}
	}
	return defaultVal
}
