package redis

import (
	"context"
	"fmt"
	"strconv"
	"time"

	"github.com/redis/go-redis/v9"
)

// RateLimiter implements domain.RateLimiter as a sliding-window log per
// key, using a Redis sorted set: each allowed call adds one member scored
// by its own timestamp, and every call first evicts members older than
// the window before counting. A true sliding window (not the
// coarser fixed-window "counter reset every N seconds" scheme) is what
// M2 §4/§9 calls for — a fixed window lets a client burst up to 2x the
// limit across a window boundary.
type RateLimiter struct {
	client *redis.Client
}

// NewRateLimiter constructs a RateLimiter over an already-connected
// client (see NewClient).
func NewRateLimiter(client *redis.Client) *RateLimiter {
	return &RateLimiter{client: client}
}

// Allow implements domain.RateLimiter. It always records the current
// attempt (even one that ends up over threshold) before reporting the
// result, matching M2 §4's intent that a caller already over the limit
// stays over the limit rather than the log resetting because nothing
// gets added on a rejected call — that would let an attacker retry
// indefinitely at exactly the threshold rate.
func (r *RateLimiter) Allow(ctx context.Context, key string, limit int, window time.Duration) (bool, error) {
	if limit <= 0 {
		return false, nil
	}

	now := time.Now()
	member := strconv.FormatInt(now.UnixNano(), 10)
	windowStart := now.Add(-window).UnixNano()

	pipe := r.client.TxPipeline()
	pipe.ZRemRangeByScore(ctx, key, "-inf", strconv.FormatInt(windowStart, 10))
	pipe.ZAdd(ctx, key, redis.Z{Score: float64(now.UnixNano()), Member: member})
	countCmd := pipe.ZCard(ctx, key)
	// The key must outlive the window by a small margin so a low-traffic
	// key doesn't linger in Redis forever, but must not expire mid-window
	// and silently reset an active counter — window+1s bounds that risk
	// without meaningfully affecting the rate-limit decision itself.
	pipe.Expire(ctx, key, window+time.Second)

	if _, err := pipe.Exec(ctx); err != nil {
		return false, fmt.Errorf("checking rate limit for %q: %w", key, err)
	}

	return countCmd.Val() <= int64(limit), nil
}
