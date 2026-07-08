package gateway

import (
	"net/http"

	"github.com/go-chi/chi/v5"
	"github.com/go-chi/chi/v5/middleware"
	"github.com/mahin273/RateMesh/internal/policy"
	"github.com/prometheus/client_golang/prometheus/promhttp"
)

// NewRouter constructs and configures the Chi router middleware chain and handlers.
func NewRouter(policyService policy.Service, pluginExecutor func(http.Handler) http.Handler, proxy *Proxy) *chi.Mux {
	r := chi.NewRouter()

	// Standard middleware
	r.Use(middleware.RequestID)
	r.Use(middleware.RealIP)
	r.Use(middleware.Logger)
	r.Use(middleware.Recoverer)

	// Tenant Resolution Middleware
	r.Use(TenantResolver(policyService))

	// Plugin Execution Middleware (runs auth, rate-limit, transform, logging)
	r.Use(pluginExecutor)

	// Expose Prometheus metrics endpoint for scrapers
	r.Handle("/metrics", promhttp.Handler())

	// Main API gateway routing logic (proxies allowed requests)
	r.HandleFunc("/*", func(w http.ResponseWriter, req *http.Request) {
		proxy.ServeHTTP(w, req)
	})

	return r
}
