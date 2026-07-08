package ratelimit

import (
	"context"
	"time"
)

// Result holds the outcome of a rate limiting check.
type Result struct {
	Allowed   bool          // True if the request is permitted.
	Remaining int           // Approximate number of remaining tokens/calls in current window.
	Reset     time.Duration // Time remaining until the rate limit bucket/window resets.
}

// RateLimitStrategy defines the interface for executing a rate limiting check.
type RateLimitStrategy interface {
	// Check evaluates whether a request should be allowed under the specified limits.
	Check(ctx context.Context, key string, limit int, window int, burst int) (*Result, error)
}
