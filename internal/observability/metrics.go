package observability

import (
	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promauto"
)

var (
	// HttpRequestsTotal tracks total requests proxied by RateMesh.
	HttpRequestsTotal = promauto.NewCounterVec(
		prometheus.CounterOpts{
			Name: "ratemesh_http_requests_total",
			Help: "Total HTTP requests handled by the RateMesh gateway",
		},
		[]string{"tenant", "path", "method", "status"},
	)

	// HttpRequestDuration tracks latency of proxied requests.
	HttpRequestDuration = promauto.NewHistogramVec(
		prometheus.HistogramOpts{
			Name:    "ratemesh_http_request_duration_seconds",
			Help:    "HTTP request latency histograms",
			Buckets: prometheus.DefBuckets,
		},
		[]string{"tenant", "path", "method"},
	)

	// RateLimitHitsTotal tracks rate limiter decisions (allowed or blocked).
	RateLimitHitsTotal = promauto.NewCounterVec(
		prometheus.CounterOpts{
			Name: "ratemesh_rate_limit_hits_total",
			Help: "Total rate limiter requests evaluated",
		},
		[]string{"tenant", "route", "strategy", "mode", "action"},
	)

	// ReconcilerSyncsTotal tracks the success/failure of background reconciler loops.
	ReconcilerSyncsTotal = promauto.NewCounterVec(
		prometheus.CounterOpts{
			Name: "ratemesh_reconciler_syncs_total",
			Help: "Total reconciler sync cycles executed",
		},
		[]string{"status"},
	)

	// ReconcilerSyncDuration tracks execution duration of reconciler loops.
	ReconcilerSyncDuration = promauto.NewHistogram(
		prometheus.HistogramOpts{
			Name:    "ratemesh_reconciler_sync_duration_seconds",
			Help:    "Duration of reconciler sync cycles",
			Buckets: prometheus.DefBuckets,
		},
	)
)
