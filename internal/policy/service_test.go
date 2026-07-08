package policy

import (
	"context"
	"testing"
)

type mockRepository struct {
	tenants  map[string]*Tenant
	policies map[string][]*RoutePolicy
}

func (m *mockRepository) GetTenant(ctx context.Context, id string) (*Tenant, error) {
	return m.tenants[id], nil
}

func (m *mockRepository) GetRoutePoliciesByTenant(ctx context.Context, tenantID string) ([]*RoutePolicy, error) {
	return m.policies[tenantID], nil
}

func (m *mockRepository) CreateTenant(ctx context.Context, tenant *Tenant) error {
	m.tenants[tenant.ID] = tenant
	return nil
}

func (m *mockRepository) CreateRoutePolicy(ctx context.Context, policy *RoutePolicy) error {
	m.policies[policy.TenantID] = append(m.policies[policy.TenantID], policy)
	return nil
}

func (m *mockRepository) GetPluginsByTenant(ctx context.Context, tenantID string) ([]*Plugin, error) {
	return nil, nil
}

func (m *mockRepository) CreatePlugin(ctx context.Context, p *Plugin) error {
	return nil
}

func TestResolveRoutePolicy(t *testing.T) {
	repo := &mockRepository{
		tenants: map[string]*Tenant{
			"tenant-1": {ID: "tenant-1", Name: "Test Tenant", PlanTier: "free"},
		},
		policies: map[string][]*RoutePolicy{
			"tenant-1": {
				{
					ID:             "policy-1",
					TenantID:       "tenant-1",
					RoutePattern:   "/api/v1/users",
					Method:         "GET",
					Strategy:       StrategyTokenBucket,
					Mode:           ModeStrict,
					LimitPerWindow: 100,
					WindowSeconds:  60,
				},
				{
					ID:             "policy-2",
					TenantID:       "tenant-1",
					RoutePattern:   "/api/v1/posts/*",
					Method:         "*",
					Strategy:       StrategyTokenBucket,
					Mode:           ModeEventual,
					LimitPerWindow: 200,
					WindowSeconds:  60,
				},
			},
		},
	}

	service := NewService(repo, nil)

	tests := []struct {
		name       string
		tenantID   string
		method     string
		path       string
		expectedID string
	}{
		{
			name:       "exact match GET /api/v1/users",
			tenantID:   "tenant-1",
			method:     "GET",
			path:       "/api/v1/users",
			expectedID: "policy-1",
		},
		{
			name:       "exact match case insensitive method",
			tenantID:   "tenant-1",
			method:     "get",
			path:       "/api/v1/users",
			expectedID: "policy-1",
		},
		{
			name:       "wildcard route GET /api/v1/posts/123",
			tenantID:   "tenant-1",
			method:     "GET",
			path:       "/api/v1/posts/123",
			expectedID: "policy-2",
		},
		{
			name:       "wildcard route POST /api/v1/posts/new",
			tenantID:   "tenant-1",
			method:     "POST",
			path:       "/api/v1/posts/new",
			expectedID: "policy-2",
		},
		{
			name:       "no match path",
			tenantID:   "tenant-1",
			method:     "GET",
			path:       "/api/v1/comments",
			expectedID: "",
		},
		{
			name:       "no match method",
			tenantID:   "tenant-1",
			method:     "POST",
			path:       "/api/v1/users",
			expectedID: "",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			p, err := service.ResolveRoutePolicy(context.Background(), tt.tenantID, tt.method, tt.path)
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if tt.expectedID == "" {
				if p != nil {
					t.Errorf("expected nil policy, got %v", p.ID)
				}
			} else {
				if p == nil {
					t.Fatalf("expected policy %s, got nil", tt.expectedID)
				}
				if p.ID != tt.expectedID {
					t.Errorf("expected policy ID %s, got %s", tt.expectedID, p.ID)
				}
			}
		})
	}
}
