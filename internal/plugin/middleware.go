package plugin

import (
	"context"
	"net/http"

	"github.com/mahin273/RateMesh/internal/policy"
)

type pluginCtxKey string

const EnabledPluginsKey pluginCtxKey = "enabled_plugins"

// PluginExecutor returns a middleware that resolves tenant plugins and runs OnRequest hooks.
func PluginExecutor(policyService policy.Service, registry *Registry) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			tenant := policy.GetTenantFromContext(r.Context())
			if tenant == nil {
				http.Error(w, "unauthorized tenant context", http.StatusUnauthorized)
				return
			}

			// Retrieve resolved route policy from context (assumes PolicyResolver middleware ran first)
			// Wait, we can resolve policy here directly, or define a small middleware in policy package.
			// Let's resolve it here directly or in policy package. Resolving here directly is simple:
			p, err := policyService.ResolveRoutePolicy(r.Context(), tenant.ID, r.Method, r.URL.Path)
			if err != nil {
				http.Error(w, "internal server error", http.StatusInternalServerError)
				return
			}

			if p == nil {
				http.Error(w, "no route policy matches requested pattern", http.StatusNotFound)
				return
			}

			// Attach policy to context so plugins can read it
			ctx := context.WithValue(r.Context(), "policy", p)

			// Fetch enabled plugins for this tenant
			tenantPlugins, err := policyService.GetPluginsByTenant(ctx, tenant.ID)
			if err != nil {
				http.Error(w, "failed to load tenant plugins", http.StatusInternalServerError)
				return
			}

			// Build config map and enabled map
			configMap := make(map[string][]byte)
			enabledMap := make(map[string]bool)
			for _, tp := range tenantPlugins {
				configMap[tp.PluginName] = []byte(tp.ConfigJSON)
				enabledMap[tp.PluginName] = true
			}

			// Attach configurations and enabled set to request context
			ctx = AttachPluginConfigs(ctx, configMap)
			ctx = context.WithValue(ctx, EnabledPluginsKey, enabledMap)
			r = r.WithContext(ctx)

			// Execute OnRequest hooks
			rc := &RequestContext{
				TenantID: tenant.ID,
				Route:    p.RoutePattern,
				Request:  r,
			}

			sc, err := registry.RunOnRequest(ctx, rc, enabledMap)
			if err != nil {
				http.Error(w, "plugin execution error", http.StatusInternalServerError)
				return
			}

			if sc != nil {
				// Copy short-circuit headers
				for k, values := range sc.Headers {
					for _, v := range values {
						w.Header().Add(k, v)
					}
				}
				w.WriteHeader(sc.StatusCode)
				w.Write(sc.Body)
				return
			}

			next.ServeHTTP(w, r)
		})
	}
}

// GetEnabledPlugins retrieves the set of enabled plugins from context.
func GetEnabledPlugins(ctx context.Context) map[string]bool {
	if val := ctx.Value(EnabledPluginsKey); val != nil {
		if m, ok := val.(map[string]bool); ok {
			return m
		}
	}
	return nil
}
