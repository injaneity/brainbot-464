package rss

// FeedConfig represents the configuration for a single RSS feed
type FeedConfig struct {
	Name string `json:"name"`
	URL  string `json:"url"`
}

// FeedPresets maps friendly keys to RSS feed configurations
var FeedPresets = map[string]FeedConfig{
	"cna": {
		Name: "Channel News Asia",
		URL:  "https://www.channelnewsasia.com/api/v1/rss-outbound-feed?_format=xml",
	},
	"st": {
		Name: "Straits Times",
		URL:  "https://www.straitstimes.com/news/singapore/rss.xml",
	},
	"hn": {
		Name: "Hacker News",
		URL:  "https://hnrss.org/newest",
	},
	"tr": {
		Name: "Technology Review",
		URL:  "https://www.technologyreview.com/feed/",
	},
}
