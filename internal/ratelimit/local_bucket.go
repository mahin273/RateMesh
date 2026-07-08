package ratelimit

import (
	"sync"
	"sync/atomic"
	"time"
)

// LocalBucket represents an in-memory, thread-safe token bucket for eventual mode.
type LocalBucket struct {
	mu                    sync.Mutex
	Limit                 int
	Window                int
	Burst                 int
	RefillRate            float64   // tokens per second
	LocalTokens           float64   // remaining local tokens
	LastRefill            time.Time // timestamp of the last local refill
	ConsumedSinceLastSync int64     // delta tokens consumed, drained by the reconciler
	GlobalLimitExceeded   bool      // flag set by the reconciler if global limits are breached
}

// NewLocalBucket initializes a new LocalBucket.
func NewLocalBucket(limit, window, burst int) *LocalBucket {
	return &LocalBucket{
		Limit:       limit,
		Window:      window,
		Burst:       burst,
		RefillRate:  float64(limit) / float64(window),
		LocalTokens: float64(limit + burst),
		LastRefill:  time.Now(),
	}
}

// Allow checks if the local bucket has enough tokens and consumes one if available.
func (b *LocalBucket) Allow(now time.Time) (bool, int) {
	b.mu.Lock()
	defer b.mu.Unlock()

	// 1. Calculate local refills based on elapsed time
	elapsed := now.Sub(b.LastRefill).Seconds()
	if elapsed > 0 {
		rate := b.RefillRate
		if b.GlobalLimitExceeded {
			// Soft correction: throttle refill speed by 75% to let the global quota recover
			rate = b.RefillRate * 0.25
		}
		refilled := elapsed * rate
		maxTokens := float64(b.Limit + b.Burst)
		b.LocalTokens = b.LocalTokens + refilled
		if b.LocalTokens > maxTokens {
			b.LocalTokens = maxTokens
		}
		b.LastRefill = now
	}

	// 2. Consume token if available
	if b.LocalTokens >= 1.0 {
		b.LocalTokens -= 1.0
		// Increment sync delta atomically so it can be drained without lock contention
		atomic.AddInt64(&b.ConsumedSinceLastSync, 1)
		return true, int(b.LocalTokens)
	}

	return false, int(b.LocalTokens)
}

// SetGlobalExceeded updates the global quota flag.
func (b *LocalBucket) SetGlobalExceeded(exceeded bool) {
	b.mu.Lock()
	defer b.mu.Unlock()
	b.GlobalLimitExceeded = exceeded
}

// LocalBucketStore manages a collection of LocalBuckets in a thread-safe map.
type LocalBucketStore struct {
	buckets sync.Map // map[string]*LocalBucket
}

// NewLocalBucketStore creates a new LocalBucketStore instance.
func NewLocalBucketStore() *LocalBucketStore {
	return &LocalBucketStore{}
}

// GetOrCreate retrieves an existing local bucket or initializes one if missing.
func (s *LocalBucketStore) GetOrCreate(key string, limit, window, burst int) *LocalBucket {
	if val, ok := s.buckets.Load(key); ok {
		return val.(*LocalBucket)
	}
	newBucket := NewLocalBucket(limit, window, burst)
	actual, _ := s.buckets.LoadOrStore(key, newBucket)
	return actual.(*LocalBucket)
}

// Range visits all buckets in the store. Useful for the reconciler.
func (s *LocalBucketStore) Range(f func(key string, bucket *LocalBucket) bool) {
	s.buckets.Range(func(k, v interface{}) bool {
		return f(k.(string), v.(*LocalBucket))
	})
}
