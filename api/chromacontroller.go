package api

import (
	"net/http"
	"os"
	"strconv"

	"brainbot/deduplication"

	"github.com/gin-gonic/gin"
)

// RegisterChromaRoutes registers Chroma-related routes.
func RegisterChromaRoutes(r *gin.Engine) {
	g := r.Group("/api/chroma")
	g.GET("/articles", handleGetAllChromaArticles)
}

// handleGetAllChromaArticles returns documents from the Chroma collection with optional pagination.
// Query params: limit (int, optional), offset (int, optional)
func handleGetAllChromaArticles(c *gin.Context) {
	host := os.Getenv("CHROMA_HOST")
	if host == "" {
		host = "localhost"
	}
	port := 8000
	if v := os.Getenv("CHROMA_PORT"); v != "" {
		if p, err := strconv.Atoi(v); err == nil {
			port = p
		}
	}
	collection := os.Getenv("CHROMA_COLLECTION")
	if collection == "" {
		collection = "brainbot_articles"
	}

	limit := 0
	if v := c.Query("limit"); v != "" {
		if n, err := strconv.Atoi(v); err == nil {
			limit = n
		}
	}
	offset := 0
	if v := c.Query("offset"); v != "" {
		if n, err := strconv.Atoi(v); err == nil {
			offset = n
		}
	}

	cfg := deduplication.ChromaConfig{Host: host, Port: port, CollectionName: collection}
	chroma, err := deduplication.NewChromaReadOnly(cfg)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	defer chroma.Close()

	res, err := chroma.ListDocuments(limit, offset)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	// Aggregate by index into a list of objects: { id, metadata, document }
	n := len(res.IDs)
	if ln := len(res.Metadatas); ln < n {
		n = ln
	}
	if ln := len(res.Documents); ln < n {
		n = ln
	}
	items := make([]gin.H, 0, n)
	for i := 0; i < n; i++ {
		items = append(items, gin.H{
			"id":       res.IDs[i],
			"metadata": res.Metadatas[i],
			"document": res.Documents[i],
		})
	}

	c.JSON(http.StatusOK, items)
}
