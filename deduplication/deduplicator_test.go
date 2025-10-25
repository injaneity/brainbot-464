package deduplication

import (
	"encoding/json"
	"math/rand"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"testing"
	"time"

	"brainbot/types"
)

type storedDocument struct {
	content  string
	metadata map[string]interface{}
}

type fakeVectorStore struct {
	docs map[string]storedDocument
}

func newFakeVectorStore() *fakeVectorStore {
	return &fakeVectorStore{docs: make(map[string]storedDocument)}
}

func (f *fakeVectorStore) QuerySimilar(queryText string, nResults int) (*QueryResults, error) {
	if nResults <= 0 {
		return &QueryResults{}, nil
	}

	type candidate struct {
		id       string
		distance float32
		metadata map[string]interface{}
	}

	candidates := make([]candidate, 0, len(f.docs))

	for id, doc := range f.docs {
		sim := similarityScore(queryText, doc.content)
		if sim <= 0 {
			continue
		}

		candidates = append(candidates, candidate{
			id:       id,
			distance: 1 - sim,
			metadata: cloneMetadata(doc.metadata),
		})
	}

	if len(candidates) == 0 {
		return &QueryResults{}, nil
	}

	sort.Slice(candidates, func(i, j int) bool {
		return candidates[i].distance < candidates[j].distance
	})

	if len(candidates) > nResults {
		candidates = candidates[:nResults]
	}

	ids := make([]string, len(candidates))
	distances := make([]float32, len(candidates))
	metadatas := make([]map[string]interface{}, len(candidates))

	for i, candidate := range candidates {
		ids[i] = candidate.id
		distances[i] = candidate.distance
		metadatas[i] = candidate.metadata
	}

	return &QueryResults{
		Ids:       [][]string{ids},
		Distances: [][]float32{distances},
		Metadatas: [][]map[string]interface{}{metadatas},
	}, nil
}

func (f *fakeVectorStore) AddDocument(doc Document) error {
	f.docs[doc.ID] = storedDocument{
		content:  doc.Content,
		metadata: cloneMetadata(doc.Metadata),
	}
	return nil
}

func (f *fakeVectorStore) GetDocument(id string) (*GetResults, error) {
	stored, ok := f.docs[id]
	if !ok {
		return &GetResults{}, nil
	}

	return &GetResults{
		Ids:       []string{id},
		Documents: []string{stored.content},
		Metadatas: []map[string]interface{}{cloneMetadata(stored.metadata)},
	}, nil
}

func (f *fakeVectorStore) UpdateDocument(doc Document) error {
	stored, ok := f.docs[doc.ID]
	if !ok {
		return nil
	}

	if doc.Metadata != nil {
		stored.metadata = cloneMetadata(doc.Metadata)
	}

	f.docs[doc.ID] = stored
	return nil
}

func (f *fakeVectorStore) DeleteDocument(id string) error {
	delete(f.docs, id)
	return nil
}

func (f *fakeVectorStore) Count() (int, error) {
	return len(f.docs), nil
}

func (f *fakeVectorStore) GetEmbeddingModel() string {
	return "fake-test-model"
}

func (f *fakeVectorStore) Close() error { return nil }

func similarityScore(a, b string) float32 {
	if a == "" || b == "" {
		return 0
	}
	if a == b {
		return 1
	}

	tokensA := tokenize(a)
	tokensB := tokenize(b)
	if len(tokensA) == 0 || len(tokensB) == 0 {
		return 0
	}

	intersection := 0
	seen := make(map[string]struct{}, len(tokensA))
	for _, token := range tokensA {
		seen[token] = struct{}{}
	}

	counted := make(map[string]struct{}, len(tokensB))
	for _, token := range tokensB {
		if _, ok := counted[token]; ok {
			continue
		}
		counted[token] = struct{}{}
		if _, ok := seen[token]; ok {
			intersection++
		}
	}

	union := len(seen) + len(counted) - intersection
	if union == 0 {
		return 0
	}

	return float32(intersection) / float32(union)
}

func tokenize(input string) []string {
	fields := strings.Fields(strings.ToLower(input))
	tokens := make([]string, 0, len(fields))
	for _, field := range fields {
		cleaned := strings.Trim(field, ".,;:!?\"'()[]{}")
		if cleaned != "" {
			tokens = append(tokens, cleaned)
		}
	}
	return tokens
}

func cloneMetadata(input map[string]interface{}) map[string]interface{} {
	if input == nil {
		return nil
	}

	cloned := make(map[string]interface{}, len(input))
	for key, value := range input {
		cloned[key] = value
	}
	return cloned
}

func loadArticlesFromFixture(t *testing.T) *types.FeedResult {
	t.Helper()

	path := filepath.Join("..", "articles.json")
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("failed to read fixture: %v", err)
	}

	var feed types.FeedResult
	if err := json.Unmarshal(data, &feed); err != nil {
		t.Fatalf("failed to decode fixture: %v", err)
	}

	return &feed
}

func TestDeduplicatorDetectsDuplicateFromVectorStore(t *testing.T) {
	feed := loadArticlesFromFixture(t)
	if feed.ArticleCount == 0 || len(feed.Articles) == 0 {
		t.Fatal("fixture must contain at least one article")
	}

	fakeStore := newFakeVectorStore()
	dedup, err := NewDeduplicatorWithClient(fakeStore, DeduplicatorConfig{})
	if err != nil {
		t.Fatalf("failed to create deduplicator: %v", err)
	}

	for _, article := range feed.Articles {
		if err := dedup.AddArticle(article); err != nil {
			t.Fatalf("failed to add article %s: %v", article.ID, err)
		}
	}

	rnd := rand.New(rand.NewSource(42))
	idx := rnd.Intn(len(feed.Articles))
	original := feed.Articles[idx]
	candidate := *original

	result, err := dedup.CheckForDuplicates(&candidate)
	if err != nil {
		t.Fatalf("check for duplicates failed: %v", err)
	}

	if !result.IsDuplicate {
		t.Fatalf("expected duplicate match for %s", candidate.ID)
	}

	if result.MatchingID != original.ID {
		t.Fatalf("expected matching ID %s, got %s", original.ID, result.MatchingID)
	}

	if result.SimilarityScore < SimilarityThreshold {
		t.Fatalf("expected similarity >= %.2f, got %.2f", SimilarityThreshold, result.SimilarityScore)
	}

	stored := fakeStore.docs[original.ID]
	lastUpdate, ok := stored.metadata["last_update"].(string)
	if !ok || lastUpdate == "" {
		t.Fatalf("expected last_update metadata for %s", original.ID)
	}

	if _, err := time.Parse(time.RFC3339, lastUpdate); err != nil {
		t.Fatalf("expected last_update to be RFC3339, got %q: %v", lastUpdate, err)
	}
}
