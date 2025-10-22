package main

// Default configuration values
const (
	DefaultFeedPreset = "st"
	DefaultCount      = 10
)

// FeedPresets maps friendly names to RSS feed URLs
var FeedPresets = map[string]string{
	"cna": "https://www.channelnewsasia.com/api/v1/rss-outbound-feed?_format=xml",
	"st":  "https://www.straitstimes.com/news/singapore/rss.xml",
	"hn":  "https://hnrss.org/newest",
	"tr":  "https://www.technologyreview.com/feed/",
}

// ResolveFeedURL resolves a feed identifier to a URL
// If the input is a preset name, returns the corresponding URL
// Otherwise, returns the input as-is (assuming it's a direct URL)
func ResolveFeedURL(feedInput string) string {
	if url, exists := FeedPresets[feedInput]; exists {
		return url
	}
	return feedInput
}
