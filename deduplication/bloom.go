package deduplication

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"net/url"
	"os"
	"strconv"
	"strings"
	"time"

	"brainbot/types"

	"github.com/redis/go-redis/v9"
)

// BloomConfig configures RedisBloom connection and key
type BloomConfig struct {
	Addr     string // e.g. localhost:6379
	Password string
	DB       int
	Key      string // redis key for bloom filter
	TTL      time.Duration
	// Capacity sets the initial BF.RESERVE capacity (number of items)
	Capacity int
	// ErrorRate sets the desired false positive probability (e.g. 0.001)
	ErrorRate float64
	// If true, BF.RESERVE NONSCALING flag will be used
	NonScaling bool
}

// RedisBloom is a minimal Redis-backed Bloom wrapper using RedisBloom commands
type RedisBloom struct {
	client *redis.Client
	key    string
	ttl    time.Duration
}

// NewRedisBloomFromEnv creates a RedisBloom client using environment variables
// REDIS_ADDR, REDIS_PASS, REDIS_DB (optional), BLOOM_KEY (optional), BLOOM_TTL (optional seconds)
func NewRedisBloomFromEnv() (*RedisBloom, error) {
	addr := os.Getenv("REDIS_ADDR")
	if addr == "" {
		addr = "localhost:6379"
	}
	pass := os.Getenv("REDIS_PASS")
	// DB default 0
	db := 0
	// key default
	key := os.Getenv("BLOOM_KEY")
	if key == "" {
		key = "articles:bloom"
	}
	ttl := 24 * time.Hour
	if t := os.Getenv("BLOOM_TTL_SECONDS"); t != "" {
		if secs, err := strconv.Atoi(t); err == nil && secs > 0 {
			ttl = time.Duration(secs) * time.Second
		}
	}

	// Optional capacity and error rate for BF.RESERVE
	capacity := 100000
	if c := os.Getenv("BLOOM_CAPACITY"); c != "" {
		if v, err := strconv.Atoi(c); err == nil && v > 0 {
			capacity = v
		}
	}
	errorRate := 0.001
	if e := os.Getenv("BLOOM_ERROR_RATE"); e != "" {
		if v, err := strconv.ParseFloat(e, 64); err == nil && v > 0 {
			errorRate = v
		}
	}
	nonScaling := false
	if ns := os.Getenv("BLOOM_NONSCALING"); ns != "" {
		if b, err := strconv.ParseBool(ns); err == nil {
			nonScaling = b
		}
	}

	cfg := BloomConfig{Addr: addr, Password: pass, DB: db, Key: key, TTL: ttl, Capacity: capacity, ErrorRate: errorRate, NonScaling: nonScaling}
	return NewRedisBloom(cfg)
}

// NewRedisBloom creates a RedisBloom wrapper and verifies connectivity
func NewRedisBloom(cfg BloomConfig) (*RedisBloom, error) {
	client := redis.NewClient(&redis.Options{
		Addr:     cfg.Addr,
		Password: cfg.Password,
		DB:       cfg.DB,
	})

	// Ping to verify
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	if err := client.Ping(ctx).Err(); err != nil {
		return nil, fmt.Errorf("failed to connect to redis at %s: %w", cfg.Addr, err)
	}

	rb := &RedisBloom{client: client, key: cfg.Key, ttl: cfg.TTL}

	// If the key does not exist, attempt to create a Bloom filter with BF.RESERVE
	// using configured capacity and error rate. If Redis doesn't have the RedisBloom
	// module or BF.RESERVE fails for any other reason, we log the error and continue;
	// BF.ADD may auto-create the filter depending on RedisBloom settings.
	exists, err := client.Exists(ctx, cfg.Key).Result()
	if err == nil && exists == 0 {
		// BF.RESERVE <key> <error_rate> <capacity> [NONSCALING]
		args := []interface{}{cfg.Key, fmt.Sprintf("%f", cfg.ErrorRate), cfg.Capacity}
		if cfg.NonScaling {
			args = append(args, "NONSCALING")
		}
		if err := client.Do(ctx, append([]interface{}{"BF.RESERVE"}, args...)...).Err(); err != nil {
			// Non-fatal; continue but log
			// Note: avoid importing log here to keep bloom package minimal; fmt.Errorf returned
			// will be ignored by callers, so we don't return error on BF.RESERVE failure.
		}
	}

	return rb, nil
}

// Close closes the underlying Redis client
func (r *RedisBloom) Close() error {
	return r.client.Close()
}

// Exists checks if the hashed value is present in the bloom filter.
// Uses the RedisBloom BF.EXISTS command.
func (r *RedisBloom) Exists(hash string) (bool, error) {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	// BF.EXISTS <key> <item>
	res, err := r.client.Do(ctx, "BF.EXISTS", r.key, hash).Result()
	if err != nil {
		return false, err
	}

	switch v := res.(type) {
	case int64:
		return v == 1, nil
	case string:
		return v == "1", nil
	default:
		return false, fmt.Errorf("unexpected BF.EXISTS response type %T: %v", res, res)
	}
}

// Add inserts the hashed value into the bloom filter and ensures TTL on the key.
func (r *RedisBloom) Add(hash string) error {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	// BF.ADD <key> <item>
	if err := r.client.Do(ctx, "BF.ADD", r.key, hash).Err(); err != nil {
		return err
	}

	// Sliding window TTL behaviour: reset the expire on each add so that the
	// filter remains active for `ttl` after the most recent insertion.
	if err := r.client.Expire(ctx, r.key, r.ttl).Err(); err != nil {
		return err
	}
	return nil
}

// NormalizeAndHash normalizes the article's URL and title and returns a SHA-256 hex hash
// Normalization steps (reasonable assumptions):
// - URL: remove fragment, remove common tracking query params (utm_*, fbclid), lowercase host
// - Title: collapse whitespace and lowercase
// The result is sha256(normalizedURL + "|" + normalizedTitle)
func NormalizeAndHash(article *types.Article) (string, error) {
	if article == nil {
		return "", fmt.Errorf("nil article")
	}

	normURL := normalizeURL(article.URL)
	normTitle := normalizeTitle(article.Title)

	combined := normURL + "|" + normTitle

	h := sha256.Sum256([]byte(combined))
	return hex.EncodeToString(h[:]), nil
}

func normalizeTitle(t string) string {
	t = strings.TrimSpace(t)
	t = strings.ToLower(t)
	// collapse multiple whitespace
	fields := strings.Fields(t)
	return strings.Join(fields, " ")
}

func normalizeURL(raw string) string {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return ""
	}
	u, err := url.Parse(raw)
	if err != nil {
		// fallback: lowercase and trim
		return strings.ToLower(raw)
	}

	// Lowercase scheme and host
	u.Scheme = strings.ToLower(u.Scheme)
	u.Host = strings.ToLower(u.Host)

	// Remove fragment
	u.Fragment = ""

	// Remove common tracking query parameters
	q := u.Query()
	for k := range q {
		lk := strings.ToLower(k)
		if strings.HasPrefix(lk, "utm_") || lk == "fbclid" || lk == "gclid" {
			q.Del(k)
		}
	}
	u.RawQuery = q.Encode()

	// Trim trailing slash for normalization
	out := u.String()
	if strings.HasSuffix(out, "/") {
		out = strings.TrimRight(out, "/")
	}
	return out
}
