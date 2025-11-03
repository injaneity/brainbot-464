package api

import (
	"net/http"

	rssfeeds "brainbot/rssfeeds"
	"brainbot/types"

	"github.com/gin-gonic/gin"
)

// RegisterArticleRoutes registers article-related routes.
func RegisterArticleRoutes(r *gin.Engine) {
	r.POST("/articles", handlePostArticle)
}

// handlePostArticle accepts a JSON payload of types.Article and returns a simple acknowledgement.
func handlePostArticle(c *gin.Context) {
	var a types.Article
	if err := c.ShouldBindJSON(&a); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	// Always ensure the ID is a generated hash, not the raw URL
	if a.URL != "" {
		a.ID = rssfeeds.GenerateID(a.URL)
	} else if a.ID != "" {
		// If no URL, but an ID was provided, normalize it by hashing
		a.ID = rssfeeds.GenerateID(a.ID)
	}

	c.JSON(http.StatusOK, gin.H{
		"status": "received",
		"article": gin.H{
			"id":        a.ID,
			"title":     a.Title,
			"url":       a.URL,
			"published": a.PublishedAt,
		},
	})
}
