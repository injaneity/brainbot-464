# BrainBot RSS Ingestor

A modern Go application that ingests RSS feeds, extracts full article content, and outputs structured JSON data.

## Features

- **Feed Presets** - 4 pre-configured popular news feeds (CNA, Straits Times, HN, Tech Review)
- Fetches and parses RSS/Atom feeds
- Extracts full article content using Mozilla's Readability algorithm
- Concurrent processing with worker pool (5 workers)
- Outputs clean JSON for downstream processing
- Graceful error handling (failed articles included with error field)

## Installation

```bash
cd brainbot
go mod download
go build
```

## Usage

### List available feed presets:
```bash
./brainbot -feeds
```

Output:
```
Available feed presets:
  cna          https://www.channelnewsasia.com/api/v1/rss-outbound-feed?_format=xml
  hn           https://hnrss.org/newest
  st           https://www.straitstimes.com/news/singapore/rss.xml
  tr           https://www.technologyreview.com/feed/

Default: st
```

### Basic usage with default preset (Straits Times, 10 articles):
```bash
./brainbot > articles.json
```

### Use a specific preset:
```bash
./brainbot -feed=cna > articles.json
./brainbot -feed=hn -count=20 > articles.json
./brainbot -feed=tr -count=5 > articles.json
```

### Use a custom feed URL:
```bash
./brainbot -feed="https://example.com/rss" -count=15 > articles.json
```

### Flags

- `-feed` - Feed preset name OR custom RSS feed URL (default: `st`)
- `-count` - Number of articles to fetch (default: `10`)
- `-feeds` - List all available feed presets and exit

## Output Format

JSON output is written to **stdout**, progress logs to **stderr**:

```json
{
  "feed_url": "https://www.straitstimes.com/news/singapore/rss.xml",
  "fetched_at": "2025-10-23T14:30:00Z",
  "article_count": 20,
  "articles": [
    {
      "id": "abc123...",
      "title": "Article Title",
      "url": "https://...",
      "published_at": "2025-10-23T10:00:00Z",
      "fetched_at": "2025-10-23T14:30:01Z",
      "summary": "Original RSS summary...",
      "author": "Author Name",
      "categories": ["Politics", "Singapore"],
      "full_content": "<p>Full HTML content...</p>",
      "full_content_text": "Plain text version...",
      "excerpt": "Brief excerpt...",
      "image_url": "https://...",
      "extraction_error": ""
    }
  ]
}
```

## Architecture

- **config.go** - Feed presets and default configuration values
- **article.go** - Data models (Article, FeedResult)
- **fetcher.go** - RSS feed fetching using `gofeed`
- **extractor.go** - Content extraction using `go-readability` with worker pool
- **main.go** - CLI entry point and orchestration

## Dependencies

- [gofeed](https://github.com/mmcdole/gofeed) - Universal feed parser
- [go-readability](https://github.com/go-shiori/go-readability) - Mozilla Readability port

## Error Handling

If article extraction fails (404, timeout, paywall, etc.), the article is still included in the output with:
- Partial metadata from RSS feed
- Empty `full_content` and `full_content_text` fields
- Error message in `extraction_error` field

## Future Enhancements

- Database storage integration
- HTTP API server mode
- Scheduled/daemon mode for periodic ingestion
- Custom extraction timeout configuration
- Retry logic for failed extractions
