package ratelimit

import (
	"context"
	"fmt"
	"log"
	"strconv"
	"sync"
	"sync/atomic"
	"time"

	"github.com/mahin273/RateMesh/internal/observability"
	"github.com/mahin273/RateMesh/internal/redisclient"
	"github.com/redis/go-redis/v9"
)

// Reconciler executes periodic background sync of in-memory eventual rate limit deltas to Redis.
type Reconciler struct {
	client   *redisclient.Client
	store    *LocalBucketStore
	interval time.Duration
	stopChan chan struct{}
	wg       sync.WaitGroup
}

// NewReconciler constructs a Reconciler.
func NewReconciler(client *redisclient.Client, store *LocalBucketStore, interval time.Duration) *Reconciler {
	return &Reconciler{
		client:   client,
		store:    store,
		interval: interval,
		stopChan: make(chan struct{}),
	}
}

// Start spawns the reconciler's background loop.
func (r *Reconciler) Start() {
	r.wg.Add(1)
	go r.run()
}

// Stop stops the background loop and flushes all remaining local counters.
func (r *Reconciler) Stop() {
	close(r.stopChan)
	r.wg.Wait()
}

func (r *Reconciler) run() {
	defer r.wg.Done()
	ticker := time.NewTicker(r.interval)
	defer ticker.Stop()

	for {
		select {
		case <-r.stopChan:
			// Flush final deltas before exiting to prevent rate limit data loss on restarts
			r.reconcile(context.Background())
			return
		case <-ticker.C:
			r.reconcile(context.Background())
		}
	}
}

type pendingUpdate struct {
	bucket  *LocalBucket
	incrCmd *redis.IntCmd
	getCmd  *redis.StringCmd
}

func (r *Reconciler) reconcile(ctx context.Context) {
	startTime := time.Now()

	pipe := r.client.Pipeline()
	var updates []pendingUpdate

	r.store.Range(func(key string, b *LocalBucket) bool {
		delta := atomic.SwapInt64(&b.ConsumedSinceLastSync, 0)
		redisKey := fmt.Sprintf("ratelimit:eventual:%s", key)

		u := pendingUpdate{bucket: b}
		if delta > 0 {
			u.incrCmd = pipe.IncrBy(ctx, redisKey, delta)
			pipe.Expire(ctx, redisKey, time.Duration(b.Window)*time.Second)
		} else {
			u.getCmd = pipe.Get(ctx, redisKey)
		}
		updates = append(updates, u)
		return true
	})

	if len(updates) == 0 {
		return
	}

	_, err := pipe.Exec(ctx)

	// Record execution metrics
	duration := time.Since(startTime).Seconds()
	observability.ReconcilerSyncDuration.Observe(duration)

	status := "success"
	if err != nil && err != redis.Nil {
		status = "error"
		log.Printf("reconciler pipeline execution error: %v", err)
	}
	observability.ReconcilerSyncsTotal.WithLabelValues(status).Inc()

	for _, u := range updates {
		var globalCount int64
		if u.incrCmd != nil {
			if val, err := u.incrCmd.Result(); err == nil {
				globalCount = val
			}
		} else if u.getCmd != nil {
			if valStr, err := u.getCmd.Result(); err == nil {
				if val, err := strconv.ParseInt(valStr, 10, 64); err == nil {
					globalCount = val
				}
			}
		}

		u.bucket.SetGlobalExceeded(globalCount >= int64(u.bucket.Limit))
	}
}
