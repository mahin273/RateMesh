package gateway

import (
	"log"
	"net/http"

	"github.com/go-chi/chi/v5"
	"github.com/go-chi/chi/v5/middleware"
	"github.com/mahin273/RateMesh/internal/policy"
)

// NewRouter constructs and configures the Chi router middleware chain and handlers.
func NewRouter(policyService policy.Service, proxy *Proxy) *chi.Mux {
	r := chi.NewRouter()

	// Standard middleware
	r.Use(middleware.RequestID)
	r.Use(middleware.RealIP)
	r.Use(middleware.Logger)
	r.Use(middleware.Recoverer)

	// Tenant Resolution Middleware
	r.Use(TenantResolver(policyService))

	// Main API gateway routing logic
	r.HandleFunc("/*", func(w http.ResponseWriter, req *http.Request) {
		tenant := GetTenantFromContext(req.Context())
		if tenant == nil {
			http.Error(w, "unauthorized tenant context", http.StatusUnauthorized)
			return
		}

		// Resolve policy for the requested route and HTTP method
		p, err := policyService.ResolveRoutePolicy(req.Context(), tenant.ID, req.Method, req.URL.Path)
		if err != nil {
			log.Printf("error resolving policy: %v", err)
			http.Error(w, "internal server error", http.StatusInternalServerError)
			return
		}

		if p == nil {
			http.Error(w, "no route policy matches requested pattern", http.StatusNotFound)
			return
		}

		// Forward to upstream target service
		proxy.ServeHTTP(w, req)
	})

	return r
}
