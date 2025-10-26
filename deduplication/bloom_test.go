package deduplication

import (
	"brainbot/types"
	"testing"
)

func TestNormalizeTitleAndURLAndHash(t *testing.T) {
	cases := []struct {
		name          string
		url           string
		title         string
		wantNormURL   string
		wantNormTitle string
	}{
		{"simple", "https://example.com/path", "Hello World", "https://example.com/path", "hello world"},
		{"utm and fragment", "https://example.com/path?utm_source=feed#section", "  Hello   World  ", "https://example.com/path", "hello world"},
		{"uppercase host", "HTTP://Example.COM/", "TiTle", "http://example.com", "title"},
		{"tracking params", "https://example.com/?fbclid=XYZ&gclid=ABC&utm_medium=1", "T", "https://example.com", "t"},
	}

	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			nu := normalizeURL(c.url)
			if nu != c.wantNormURL {
				t.Fatalf("normalizeURL(%q) = %q; want %q", c.url, nu, c.wantNormURL)
			}
			nt := normalizeTitle(c.title)
			if nt != c.wantNormTitle {
				t.Fatalf("normalizeTitle(%q) = %q; want %q", c.title, nt, c.wantNormTitle)
			}
			// Hash should be stable and not empty
			// Use a simple Article stub
			a := &types.Article{URL: c.url, Title: c.title}
			h, err := NormalizeAndHash(a)
			if err != nil {
				t.Fatalf("NormalizeAndHash error: %v", err)
			}
			if h == "" {
				t.Fatalf("NormalizeAndHash returned empty hash")
			}
		})
	}
}
