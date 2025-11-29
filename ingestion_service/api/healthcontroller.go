package api

import (
	"net/http"

	"github.com/gin-gonic/gin"
)

// RegisterHealthRoutes registers health check endpoints.
func RegisterHealthRoutes(r *gin.Engine) {
	r.GET("/api/health", handleHealth)
}

func handleHealth(c *gin.Context) {
	c.JSON(http.StatusOK, gin.H{"status": "ok"})
}
