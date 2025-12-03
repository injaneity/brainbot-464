package types

import "time"

// ArticleResult represents the processing result for a single article
type ArticleResult struct {
	Article             *Article             `json:"article"`
	Status              string               `json:"status"` // "new", "duplicate", "failed", "error"
	DeduplicationResult *DeduplicationResult `json:"deduplication_result,omitempty"`
	Error               string               `json:"error,omitempty"`
}

// DeduplicationResult contains the result of deduplication check
type DeduplicationResult struct {
	IsDuplicate      bool      `json:"is_duplicate"`
	IsExactDuplicate bool      `json:"is_exact_duplicate,omitempty"`
	MatchingID       string    `json:"matching_id,omitempty"`
	SimilarityScore  float32   `json:"similarity_score,omitempty"`
	CheckedAt        time.Time `json:"checked_at"`
}
