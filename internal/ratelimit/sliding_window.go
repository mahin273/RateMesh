package ratelimit

import (
	"context"
	_ "embed"
	"fmt"
	"strconv"
	"sync/atomic"
	"time"

	"github.com/mahin273/RateMesh/internal/redisclient"
	"github.com/redis/go-redis/v9"
)

//go:embed lua/sliding_window.lua
var slidingWindowLua string

var reqCounter uint64

// nextReqID generates a high-performance, collision-resistant request identifier.
func nextReqID() string {
	val := atomic.AddUint64(&reqCounter, 1)
	return strconv.FormatInt(time.Now().UnixNano(), 36) + "-" + strconv.FormatUint(val, 36)
}

type SlidingWindowStrategy struct {
	client *redisclient.Client
	sha    string
}

// NewSlidingWindowStrategy loads the sliding window Lua script into Redis.
func NewSlidingWindowStrategy(client *redisclient.Client) (*SlidingWindowStrategy, error) {
	ctx := context.Background()
	sha, err := client.ScriptLoad(ctx, slidingWindowLua).Result()
	if err != nil {
		return nil, fmt.Errorf("failed to preload sliding_window Lua script: %w", err)
	}
	return &SlidingWindowStrategy{
		client: client,
		sha:    sha,
	}, nil
}

// Check implements RateLimitStrategy.
func (s *SlidingWindowStrategy) Check(ctx context.Context, key string, limit int, window int, burst int) (*Result, error) {
	// ARGV[1] = now (ms)
	// ARGV[2] = window (ms)
	// ARGV[3] = limit
	// ARGV[4] = request_id (unique value)
	nowMs := time.Now().UnixMilli()
	windowMs := int64(window) * 1000
	reqID := nextReqID()

	res, err := s.client.EvalSha(ctx, s.sha, []string{key}, nowMs, windowMs, limit, reqID).Result()
	if err != nil {
		// Reload if script cache is cleared in Redis
		if redis.HasErrorPrefix(err, "NOSCRIPT") {
			var reloadErr error
			s.sha, reloadErr = s.client.ScriptLoad(ctx, slidingWindowLua).Result()
			if reloadErr != nil {
				return nil, fmt.Errorf("failed to reload sliding_window Lua script: %w", reloadErr)
			}
			res, err = s.client.EvalSha(ctx, s.sha, []string{key}, nowMs, windowMs, limit, reqID).Result()
			if err != nil {
				return nil, err
			}
		} else {
			return nil, err
		}
	}

	slice, ok := res.([]interface{})
	if !ok || len(slice) < 2 {
		return nil, fmt.Errorf("unexpected sliding_window Lua output type: %T", res)
	}

	allowedInt, ok1 := slice[0].(int64)
	remainingInt, ok2 := slice[1].(int64)
	if !ok1 || !ok2 {
		return nil, fmt.Errorf("unexpected sliding_window Lua output values: %T, %T", slice[0], slice[1])
	}

	// Simple reset duration is the window duration itself
	resetDur := time.Duration(window) * time.Second

	return &Result{
		Allowed:   allowedInt == 1,
		Remaining: int(remainingInt),
		Reset:     resetDur,
	}, nil
}
