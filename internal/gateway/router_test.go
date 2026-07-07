package gateway

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/mahin273/RateMesh/internal/policy"
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
		name           string
		headerTenantID string
		host           string
		tenant         *policy.Tenant
		policy         *policy.RoutePolicy
		expectedStatus int
		expectedBody   string
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
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			svc := &mockPolicyService{
				tenant: tt.tenant,
				policy: tt.policy,
			}
			router := NewRouter(svc, proxy)

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
