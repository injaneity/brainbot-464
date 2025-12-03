package api

import (
	"brainbot/ingestion_service/rssfeeds"
	"brainbot/shared/rss"
	"net/http"

	"github.com/gin-gonic/gin"
)

type FetchRequest struct {
	FeedPreset string `json:"feed_preset"`
	Count      int    `json:"count"`
}

func RegisterRSSRoutes(r *gin.Engine) {
	r.POST("/fetch", FetchArticles)
	r.GET("/presets", GetPresets)
}

func GetPresets(c *gin.Context) {
	c.JSON(http.StatusOK, rss.FeedPresets)
}

func FetchArticles(c *gin.Context) {
	var req FetchRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	if req.FeedPreset == "" {
		req.FeedPreset = rssfeeds.DefaultFeedPreset
	}
	if req.Count == 0 {
		req.Count = rssfeeds.DefaultCount
	}

	feedURL := rssfeeds.ResolveFeedURL(req.FeedPreset)
	articles, err := rssfeeds.FetchFeed(feedURL, req.Count)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to fetch feed: " + err.Error()})
		return
	}

	// Extract full content for all articles
	rssfeeds.ExtractAllContent(articles)

	c.JSON(http.StatusOK, articles)
}
