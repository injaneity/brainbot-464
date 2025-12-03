package rssfeeds

import "brainbot/shared/rss"

// Default configuration values
const (
	DefaultFeedPreset = "st"
	DefaultCount      = 10
)

// ResolveFeedURL resolves a feed identifier to a URL
// If the input is a preset name, returns the corresponding URL
// Otherwise, returns the input as-is (assuming it's a direct URL)
func ResolveFeedURL(feedInput string) string {
	if config, exists := rss.FeedPresets[feedInput]; exists {
		return config.URL
	}
	return feedInput
}
