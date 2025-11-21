package api

import (
	"github.com/gin-gonic/gin"
)

// NewRouter constructs a Gin engine with registered routes.
func NewRouter() *gin.Engine {
	r := gin.New()
	// Minimal middleware: recovery; logger optional to reduce verbosity
	r.Use(gin.Recovery())

	// Register resource routers
	RegisterArticleRoutes(r)
	RegisterChromaRoutes(r)
	RegisterDeduplicationRoutes(r)
	RegisterHealthRoutes(r)
	RegisterRSSRoutes(r)
	return r
}
