package api

import (
	"brainbot/deduplication"
	"brainbot/types"
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

	result, err := deduplicator.ProcessArticle(req.Article)
	if err != nil {
		response := ProcessArticleResponse{
			Status: "error",
			Error:  err.Error(),
		}
		c.JSON(http.StatusInternalServerError, response)
		return
	}

	status := "new"
	if result.IsDuplicate {
		status = "duplicate"
	}

	response := ProcessArticleResponse{
		Status:              status,
		DeduplicationResult: result,
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

	deduplicatorConfig := deduplication.DeduplicatorConfig{
		ChromaConfig:        chromaConfig,
		SimilarityThreshold: 0, // Use default
		MaxSearchResults:    0, // Use default
	}

	return deduplication.NewDeduplicator(deduplicatorConfig)
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
