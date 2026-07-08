package ratelimit

import (
	"context"
	_ "embed"
	"fmt"
	"time"

	"github.com/mahin273/RateMesh/internal/redisclient"
	"github.com/redis/go-redis/v9"
)

//go:embed lua/token_bucket.lua
var tokenBucketLua string

type TokenBucketStrategy struct {
	client *redisclient.Client
	sha    string
}

// NewTokenBucketStrategy loads the Lua script into Redis and returns the strategy instance.
func NewTokenBucketStrategy(client *redisclient.Client) (*TokenBucketStrategy, error) {
	ctx := context.Background()
	sha, err := client.ScriptLoad(ctx, tokenBucketLua).Result()
	if err != nil {
		return nil, fmt.Errorf("failed to preload token_bucket Lua script: %w", err)
	}
	return &TokenBucketStrategy{
		client: client,
		sha:    sha,
	}, nil
}

// Check implements RateLimitStrategy.
func (s *TokenBucketStrategy) Check(ctx context.Context, key string, limit int, window int, burst int) (*Result, error) {
	// ARGV[1] = max_tokens (limit + burst)
	// ARGV[2] = refill_rate_per_sec (limit / window)
	// ARGV[3] = now (ms)
	// ARGV[4] = requested_tokens (1)
	maxTokens := limit + burst
	refillRate := float64(limit) / float64(window)
	nowMs := time.Now().UnixMilli()

	res, err := s.client.EvalSha(ctx, s.sha, []string{key}, maxTokens, refillRate, nowMs, 1).Result()
	if err != nil {
		// Reload if script cache is cleared in Redis
		if redis.HasErrorPrefix(err, "NOSCRIPT") {
			var reloadErr error
			s.sha, reloadErr = s.client.ScriptLoad(ctx, tokenBucketLua).Result()
			if reloadErr != nil {
				return nil, fmt.Errorf("failed to reload token_bucket Lua script: %w", reloadErr)
			}
			res, err = s.client.EvalSha(ctx, s.sha, []string{key}, maxTokens, refillRate, nowMs, 1).Result()
			if err != nil {
				return nil, err
			}
		} else {
			return nil, err
		}
	}

	slice, ok := res.([]interface{})
	if !ok || len(slice) < 2 {
		return nil, fmt.Errorf("unexpected token_bucket Lua output type: %T", res)
	}

	allowedInt, ok1 := slice[0].(int64)
	remainingInt, ok2 := slice[1].(int64)
	if !ok1 || !ok2 {
		return nil, fmt.Errorf("unexpected token_bucket Lua output values: %T, %T", slice[0], slice[1])
	}

	// Calculate reset duration (time to completely refill the token bucket)
	var resetDur time.Duration
	if remainingInt < int64(maxTokens) && refillRate > 0 {
		missingTokens := float64(int64(maxTokens) - remainingInt)
		secondsToRefill := missingTokens / refillRate
		resetDur = time.Duration(secondsToRefill) * time.Second
	}

	return &Result{
		Allowed:   allowedInt == 1,
		Remaining: int(remainingInt),
		Reset:     resetDur,
	}, nil
}
