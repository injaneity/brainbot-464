package api

import (
	"context"
	"net/http"

	"brainbot/orchestrator"

	"github.com/gin-gonic/gin"
)

// RegisterRSSRoutes registers RSS-related endpoints.
func RegisterRSSRoutes(r *gin.Engine) {
	g := r.Group("/api/rss")
	g.POST("/refresh", handleRSSRefresh)
}

// handleRSSRefresh triggers the orchestrator to fetch, deduplicate, and optionally upload.
// It runs asynchronously and returns 202 Accepted immediately.
func handleRSSRefresh(c *gin.Context) {
	go func() {
		_ = orchestrator.RunOnce(context.Background())
	}()
	c.JSON(http.StatusAccepted, gin.H{"status": "refresh started"})
}
