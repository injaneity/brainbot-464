package rssfeeds

// import (
// 	"encoding/json"
// 	"flag"
// 	"fmt"
// 	"log"
// 	"os"
// 	"sort"
// 	"time"

// 	"brainbot/types"
// )

// func main() {
// 	// Parse command-line flags
// 	feed := flag.String("feed", DefaultFeedPreset, "RSS feed preset name or URL (use -feeds to list presets)")
// 	count := flag.Int("count", DefaultCount, "Number of articles to fetch")
// 	listFeeds := flag.Bool("feeds", false, "List available feed presets and exit")
// 	flag.Parse()

// 	// List available feeds if requested
// 	if *listFeeds {
// 		fmt.Println("Available feed presets:")

// 		// Sort preset names alphabetically for consistent output
// 		names := make([]string, 0, len(FeedPresets))
// 		for name := range FeedPresets {
// 			names = append(names, name)
// 		}
// 		sort.Strings(names)

// 		for _, name := range names {
// 			fmt.Printf("  %-12s %s\n", name, FeedPresets[name])
// 		}
// 		fmt.Printf("\nDefault: %s\n", DefaultFeedPreset)
// 		fmt.Println("\nUsage:")
// 		fmt.Println("  ./brainbot -feed=st")
// 		fmt.Println("  ./brainbot -feed=https://example.com/rss")
// 		os.Exit(0)
// 	}

// 	// Resolve feed preset or use as URL
// 	feedURL := ResolveFeedURL(*feed)

// 	// Log to stderr so JSON output to stdout is clean
// 	log.SetOutput(os.Stderr)
// 	log.Printf("Feed input: %s", *feed)
// 	log.Printf("Resolved URL: %s", feedURL)
// 	log.Printf("Target article count: %d", *count)

// 	// Fetch RSS feed
// 	articles, err := FetchFeed(feedURL, *count)
// 	if err != nil {
// 		log.Fatalf("Failed to fetch feed: %v", err)
// 	}
// 	log.Printf("Fetched %d articles from feed", len(articles))

// 	// Extract full content for all articles
// 	log.Printf("Extracting full content using %d workers...", WorkerCount)
// 	ExtractAllContent(articles)

// 	// Count successful extractions
// 	successCount := 0
// 	for _, article := range articles {
// 		if article.ExtractionError == "" {
// 			successCount++
// 		}
// 	}
// 	log.Printf("Successfully extracted %d/%d articles", successCount, len(articles))

// 	// Create result wrapper
// 	result := types.FeedResult{
// 		FeedURL:      feedURL,
// 		FetchedAt:    time.Now(),
// 		ArticleCount: len(articles),
// 		Articles:     articles,
// 	}

// 	// Output JSON to stdout
// 	encoder := json.NewEncoder(os.Stdout)
// 	encoder.SetIndent("", "  ")
// 	if err := encoder.Encode(result); err != nil {
// 		log.Fatalf("Failed to encode JSON: %v", err)
// 	}

// 	// Exit with error code if some extractions failed
// 	if successCount < len(articles) {
// 		log.Printf("Warning: %d articles failed to extract", len(articles)-successCount)
// 		fmt.Fprintln(os.Stderr, "Note: Failed articles are included in JSON with extraction_error field")
// 	}
// }
