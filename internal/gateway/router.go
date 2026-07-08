package gateway

import (
	"net/http"

	"github.com/go-chi/chi/v5"
	"github.com/go-chi/chi/v5/middleware"
	"github.com/mahin273/RateMesh/internal/policy"
)

// NewRouter constructs and configures the Chi router middleware chain and handlers.
func NewRouter(policyService policy.Service, rateLimiter func(http.Handler) http.Handler, proxy *Proxy) *chi.Mux {
	r := chi.NewRouter()

	// Standard middleware
	r.Use(middleware.RequestID)
	r.Use(middleware.RealIP)
	r.Use(middleware.Logger)
	r.Use(middleware.Recoverer)

	// Tenant Resolution Middleware
	r.Use(TenantResolver(policyService))

	// Rate Limiting Middleware (resolves policy and enforces limits)
	r.Use(rateLimiter)

	// Main API gateway routing logic (proxies allowed requests)
	r.HandleFunc("/*", func(w http.ResponseWriter, req *http.Request) {
		proxy.ServeHTTP(w, req)
	})

	return r
}
