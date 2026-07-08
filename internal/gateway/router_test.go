package gateway

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/mahin273/RateMesh/internal/policy"
	"github.com/mahin273/RateMesh/internal/ratelimit"
)

type mockPolicyService struct {
	tenant *policy.Tenant
	policy *policy.RoutePolicy
	err    error
}

func (m *mockPolicyService) GetTenant(ctx context.Context, id string) (*policy.Tenant, error) {
	if m.tenant != nil && m.tenant.ID == id {
		return m.tenant, nil
	}
	return nil, m.err
}

func (m *mockPolicyService) ResolveRoutePolicy(ctx context.Context, tenantID, method, path string) (*policy.RoutePolicy, error) {
	return m.policy, m.err
}

type mockRateLimitStrategy struct {
	result *ratelimit.Result
	err    error
}

func (m *mockRateLimitStrategy) Check(ctx context.Context, key string, limit int, window int, burst int) (*ratelimit.Result, error) {
	if m.err != nil {
		return nil, m.err
	}
	return m.result, nil
}

func TestRouterWorkflow(t *testing.T) {
	// Set up a mock upstream server
	upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		w.Write([]byte("upstream-response"))
	}))
	defer upstream.Close()

	proxy, err := NewProxy(upstream.URL)
	if err != nil {
		t.Fatalf("failed to create proxy: %v", err)
	}

	tests := []struct {
		name            string
		headerTenantID  string
		host            string
		tenant          *policy.Tenant
		policy          *policy.RoutePolicy
		rateLimitResult *ratelimit.Result
		expectedStatus  int
		expectedBody    string
	}{
		{
			name:           "missing tenant identifier",
			headerTenantID: "",
			host:           "localhost",
			expectedStatus: http.StatusUnauthorized,
			expectedBody:   "missing tenant identifier",
		},
		{
			name:           "tenant not found",
			headerTenantID: "non-existent",
			host:           "localhost",
			tenant:         nil,
			expectedStatus: http.StatusForbidden,
			expectedBody:   "tenant not found or inactive",
		},
		{
			name:           "no matching route policy",
			headerTenantID: "tenant-1",
			host:           "localhost",
			tenant:         &policy.Tenant{ID: "tenant-1"},
			policy:         nil,
			expectedStatus: http.StatusNotFound,
			expectedBody:   "no route policy matches requested pattern",
		},
		{
			name:           "successful proxy forwarding",
			headerTenantID: "tenant-1",
			host:           "localhost",
			tenant:         &policy.Tenant{ID: "tenant-1"},
			policy:         &policy.RoutePolicy{ID: "policy-1"},
			expectedStatus: http.StatusOK,
			expectedBody:   "upstream-response",
		},
		{
			name:           "successful subdomain resolution",
			headerTenantID: "",
			host:           "tenant-sub.localhost",
			tenant:         &policy.Tenant{ID: "tenant-sub"},
			policy:         &policy.RoutePolicy{ID: "policy-sub"},
			expectedStatus: http.StatusOK,
			expectedBody:   "upstream-response",
		},
		{
			name:            "rate limit exceeded (429)",
			headerTenantID:  "tenant-1",
			host:            "localhost",
			tenant:          &policy.Tenant{ID: "tenant-1"},
			policy:          &policy.RoutePolicy{ID: "policy-1", LimitPerWindow: 100},
			rateLimitResult: &ratelimit.Result{Allowed: false, Remaining: 0, Reset: 10 * time.Second},
			expectedStatus:  http.StatusTooManyRequests,
			expectedBody:    "too many requests",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			svc := &mockPolicyService{
				tenant: tt.tenant,
				policy: tt.policy,
			}

			// Configure mock rate limiter strategy
			var rlResult *ratelimit.Result
			if tt.rateLimitResult != nil {
				rlResult = tt.rateLimitResult
			} else {
				rlResult = &ratelimit.Result{Allowed: true, Remaining: 99, Reset: 5 * time.Second}
			}

			mockStrat := &mockRateLimitStrategy{result: rlResult}
			localStore := ratelimit.NewLocalBucketStore()
			rateLimiter := ratelimit.RateLimiter(svc, mockStrat, mockStrat, localStore)

			router := NewRouter(svc, rateLimiter, proxy)

			req := httptest.NewRequest("GET", "/test-path", nil)
			if tt.host != "" {
				req.Host = tt.host
			} else {
				req.Host = "localhost"
			}

			if tt.headerTenantID != "" {
				req.Header.Set("X-Tenant-ID", tt.headerTenantID)
			}

			w := httptest.NewRecorder()
			router.ServeHTTP(w, req)

			if w.Code != tt.expectedStatus {
				t.Errorf("expected status %d, got %d", tt.expectedStatus, w.Code)
			}

			body := w.Body.String()
			if !strings.Contains(body, tt.expectedBody) {
				t.Errorf("expected body to contain %q, got %q", tt.expectedBody, body)
			}
		})
	}
}
