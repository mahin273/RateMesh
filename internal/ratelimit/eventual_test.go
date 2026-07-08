package ratelimit

import (
	"context"
	"sync/atomic"
	"testing"
	"time"

	"github.com/mahin273/RateMesh/internal/redisclient"
)

func TestLocalBucketRefillAndThrottling(t *testing.T) {
	// 1. Create a local bucket with limit = 10, window = 10s, burst = 0
	// RefillRate = 1 token/sec
	bucket := NewLocalBucket(10, 10, 0)

	// Initial state: full
	if bucket.LocalTokens != 10.0 {
		t.Errorf("expected 10 tokens, got %f", bucket.LocalTokens)
	}

	// Consume 10 tokens
	now := time.Now()
	for i := 0; i < 10; i++ {
		allowed, _ := bucket.Allow(now)
		if !allowed {
			t.Errorf("expected token consume %d to be allowed", i)
		}
	}

	// Token 11 should be denied
	allowed, _ := bucket.Allow(now)
	if allowed {
		t.Error("expected token 11 to be denied")
	}

	// Move time forward by 2 seconds
	now = now.Add(2 * time.Second)

	// Check refill: should have refilled 2 tokens (elapsed = 2s, rate = 1 token/s)
	allowed, _ = bucket.Allow(now)
	if !allowed {
		t.Error("expected refilled token 1 to be allowed")
	}
	allowed, _ = bucket.Allow(now)
	if !allowed {
		t.Error("expected refilled token 2 to be allowed")
	}
	allowed, _ = bucket.Allow(now)
	if allowed {
		t.Error("expected token 3 to be denied (only 2 refilled)")
	}

	// 2. Test Soft Correction (Global Limit Exceeded)
	bucket.SetGlobalExceeded(true) // Triggers throttling (refill rate reduced by 75%)

	// Move time forward by 4 seconds
	now = now.Add(4 * time.Second)

	// Under normal refill rate, 4 seconds = 4 tokens.
	// Under throttled rate (25%), 4 seconds = 1 token.
	allowed, _ = bucket.Allow(now)
	if !allowed {
		t.Error("expected refilled token under throttle to be allowed")
	}
	allowed, _ = bucket.Allow(now)
	if allowed {
		t.Error("expected second token under throttle to be denied (only 1 token refilled)")
	}
}

func TestReconcilerSync(t *testing.T) {
	ctx := context.Background()

	// Connect to local Redis
	client, err := redisclient.NewClient("localhost:6379")
	if err != nil {
		t.Skip("Redis is not running on localhost:6379; skipping reconciler test")
	}
	defer client.Close()

	testKey := "test:ratelimit:eventual-sync"
	client.Del(ctx, "ratelimit:eventual:"+testKey)

	store := NewLocalBucketStore()
	bucket := store.GetOrCreate(testKey, 5, 10, 0)

	// Consume 3 tokens locally
	now := time.Now()
	for i := 0; i < 3; i++ {
		bucket.Allow(now)
	}

	// Delta should be 3
	delta := atomic.LoadInt64(&bucket.ConsumedSinceLastSync)
	if delta != 3 {
		t.Errorf("expected delta to be 3, got %d", delta)
	}

	// Create and trigger reconciler
	reconciler := NewReconciler(client, store, 100*time.Millisecond)
	reconciler.reconcile(ctx)

	// After reconcile, delta should be drained to 0
	deltaAfter := atomic.LoadInt64(&bucket.ConsumedSinceLastSync)
	if deltaAfter != 0 {
		t.Errorf("expected delta to be drained to 0, got %d", deltaAfter)
	}

	// Global count in Redis should be 3
	redisCount, err := client.Get(ctx, "ratelimit:eventual:"+testKey).Int64()
	if err != nil {
		t.Fatalf("failed to query redis count: %v", err)
	}
	if redisCount != 3 {
		t.Errorf("expected Redis count to be 3, got %d", redisCount)
	}

	// Global limit is 5. We consumed 3. Limit should not be exceeded.
	if bucket.GlobalLimitExceeded {
		t.Error("expected global limit not to be exceeded")
	}

	// Consume 3 more tokens (total 6, exceeding limit of 5)
	for i := 0; i < 3; i++ {
		bucket.Allow(now)
	}
	reconciler.reconcile(ctx)

	// Global limit should now be exceeded
	if !bucket.GlobalLimitExceeded {
		t.Error("expected global limit to be exceeded after consuming 6 tokens")
	}
}
