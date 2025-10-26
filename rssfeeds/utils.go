package rssfeeds

import (
	"crypto/sha256"
	"encoding/hex"
)

// GenerateID creates a short, stable ID by hashing the provided string input
func GenerateID(input string) string {
	hash := sha256.Sum256([]byte(input))
	return hex.EncodeToString(hash[:])[:16]
}
