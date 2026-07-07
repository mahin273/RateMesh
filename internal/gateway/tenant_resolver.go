package gateway

import (
	"context"
	"net/http"
	"strings"

	"github.com/mahin273/RateMesh/internal/policy"
)

type contextKey string

const TenantKey contextKey = "tenant"

// TenantResolver middleware extracts and validates tenant from header, query, or subdomain.
func TenantResolver(policyService policy.Service) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			var tenantID string

			// 1. Extract from header X-Tenant-ID
			if id := r.Header.Get("X-Tenant-ID"); id != "" {
				tenantID = id
			}

			// 2. Extract from query param tenant_id
			if tenantID == "" {
				tenantID = r.URL.Query().Get("tenant_id")
			}

			// 3. Extract from subdomain if applicable (e.g., tenant-a.localhost)
			if tenantID == "" {
				host := r.Host
				if parts := strings.Split(host, "."); len(parts) > 1 {
					// Check if first part is not "localhost", "www", or IP-like
					first := parts[0]
					if first != "localhost" && first != "www" && first != "127" && first != "0" {
						tenantID = first
					}
				}
			}

			if tenantID == "" {
				http.Error(w, "missing tenant identifier", http.StatusUnauthorized)
				return
			}

			// Validate tenant exists
			tenant, err := policyService.GetTenant(r.Context(), tenantID)
			if err != nil {
				http.Error(w, "internal server error during tenant validation", http.StatusInternalServerError)
				return
			}

			if tenant == nil {
				http.Error(w, "tenant not found or inactive", http.StatusForbidden)
				return
			}

			// Attach tenant to context
			ctx := context.WithValue(r.Context(), TenantKey, tenant)
			next.ServeHTTP(w, r.WithContext(ctx))
		})
	}
}

// GetTenantFromContext helper to retrieve Tenant from request context
func GetTenantFromContext(ctx context.Context) *policy.Tenant {
	if val := ctx.Value(TenantKey); val != nil {
		if t, ok := val.(*policy.Tenant); ok {
			return t
		}
	}
	return nil
}
