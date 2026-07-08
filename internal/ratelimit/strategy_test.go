package ratelimit

import (
	"context"
	"testing"

	"github.com/mahin273/RateMesh/internal/redisclient"
)

func TestStrictStrategies(t *testing.T) {
	ctx := context.Background()

	// Connect to local Redis, skipping test if Redis is not running
	client, err := redisclient.NewClient("localhost:6379")
	if err != nil {
		t.Skip("Redis is not running on localhost:6379; skipping integration test. Error: ", err)
	}
	defer client.Close()

	testKeyTB := "test:ratelimit:tb"
	testKeySW := "test:ratelimit:sw"
	client.Del(ctx, testKeyTB, testKeySW)

	t.Run("TokenBucketStrategy", func(t *testing.T) {
		tb, err := NewTokenBucketStrategy(client)
		if err != nil {
			t.Fatalf("failed to create token bucket strategy: %v", err)
		}

		// Check 1: Allowed, limit 5, window 10s, burst 0
		res, err := tb.Check(ctx, testKeyTB, 5, 10, 0)
		if err != nil {
			t.Fatalf("check failed: %v", err)
		}
		if !res.Allowed {
			t.Error("expected check 1 to be allowed")
		}
		if res.Remaining != 4 {
			t.Errorf("expected 4 remaining tokens, got %d", res.Remaining)
		}

		// Consume remaining 4 tokens
		for i := 0; i < 4; i++ {
			res, err = tb.Check(ctx, testKeyTB, 5, 10, 0)
			if err != nil {
				t.Fatalf("check failed at iteration %d: %v", i, err)
			}
			if !res.Allowed {
				t.Errorf("expected check at iteration %d to be allowed", i)
			}
		}

		// Check 6: Exceeded limit
		res, err = tb.Check(ctx, testKeyTB, 5, 10, 0)
		if err != nil {
			t.Fatalf("check failed: %v", err)
		}
		if res.Allowed {
			t.Error("expected check 6 to be denied")
		}
		if res.Remaining != 0 {
			t.Errorf("expected 0 remaining tokens, got %d", res.Remaining)
		}
	})

	t.Run("SlidingWindowStrategy", func(t *testing.T) {
		sw, err := NewSlidingWindowStrategy(client)
		if err != nil {
			t.Fatalf("failed to create sliding window strategy: %v", err)
		}

		// Check 1: Allowed, limit 3, window 2s
		res, err := sw.Check(ctx, testKeySW, 3, 2, 0)
		if err != nil {
			t.Fatalf("check failed: %v", err)
		}
		if !res.Allowed {
			t.Error("expected check 1 to be allowed")
		}

		// Consume remaining 2
		_, _ = sw.Check(ctx, testKeySW, 3, 2, 0)
		_, _ = sw.Check(ctx, testKeySW, 3, 2, 0)

		// Check 4: Exceeded
		res, err = sw.Check(ctx, testKeySW, 3, 2, 0)
		if err != nil {
			t.Fatalf("check failed: %v", err)
		}
		if res.Allowed {
			t.Error("expected check 4 to be denied")
		}
	})
}
