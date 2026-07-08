package gateway

import (
	"context"
	"net/http"
	"net/http/httptest"
	"sync/atomic"
	"testing"

	"github.com/mahin273/RateMesh/internal/plugin"
	"github.com/mahin273/RateMesh/internal/policy"
)

func TestProxyRetries(t *testing.T) {
	var requestCount int64

	// Upstream fails twice and then returns success
	upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		count := atomic.AddInt64(&requestCount, 1)
		if count < 3 {
			w.WriteHeader(http.StatusInternalServerError)
			w.Write([]byte("error"))
		} else {
			w.WriteHeader(http.StatusOK)
			w.Write([]byte("success"))
		}
	}))
	defer upstream.Close()

	registry := plugin.NewRegistry()
	proxy, err := NewProxy(upstream.URL, registry)
	if err != nil {
		t.Fatalf("failed to create proxy: %v", err)
	}

	w := httptest.NewRecorder()
	req := httptest.NewRequest("GET", "/test", nil)

	// Inject context helpers to bypass execution errors
	ctx := policy.AttachTenantToContext(req.Context(), &policy.Tenant{ID: "tenant-1"})
	ctx = plugin.AttachPluginConfigs(ctx, map[string][]byte{})
	ctx = context.WithValue(ctx, plugin.EnabledPluginsKey, map[string]bool{})
	req = req.WithContext(ctx)

	proxy.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("expected status 200 after retries, got %d", w.Code)
	}

	finalCount := atomic.LoadInt64(&requestCount)
	if finalCount != 3 {
		t.Errorf("expected exactly 3 requests (2 retries + 1 success), got %d", finalCount)
	}
}

func TestProxyCircuitBreaker(t *testing.T) {
	var requestCount int64

	// Upstream always fails
	upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		atomic.AddInt64(&requestCount, 1)
		w.WriteHeader(http.StatusInternalServerError)
	}))
	defer upstream.Close()

	registry := plugin.NewRegistry()
	proxy, err := NewProxy(upstream.URL, registry)
	if err != nil {
		t.Fatalf("failed to create proxy: %v", err)
	}

	// Send 5 failures to trip the circuit breaker
	for i := 0; i < 5; i++ {
		w := httptest.NewRecorder()
		req := httptest.NewRequest("GET", "/test", nil)
		ctx := policy.AttachTenantToContext(req.Context(), &policy.Tenant{ID: "tenant-1"})
		ctx = plugin.AttachPluginConfigs(ctx, map[string][]byte{})
		ctx = context.WithValue(ctx, plugin.EnabledPluginsKey, map[string]bool{})
		req = req.WithContext(ctx)

		proxy.ServeHTTP(w, req)
	}

	// 6th request should fail immediately with 503 (circuit breaker open)
	w := httptest.NewRecorder()
	req := httptest.NewRequest("GET", "/test", nil)
	ctx := policy.AttachTenantToContext(req.Context(), &policy.Tenant{ID: "tenant-1"})
	ctx = plugin.AttachPluginConfigs(ctx, map[string][]byte{})
	ctx = context.WithValue(ctx, plugin.EnabledPluginsKey, map[string]bool{})
	req = req.WithContext(ctx)

	proxy.ServeHTTP(w, req)

	if w.Code != http.StatusServiceUnavailable {
		t.Errorf("expected status 503 (circuit breaker open), got %d", w.Code)
	}
}
