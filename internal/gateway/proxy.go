package gateway

import (
	"log"
	"net/http"
	"net/http/httputil"
	"net/url"
	"time"

	"github.com/mahin273/RateMesh/internal/plugin"
	"github.com/mahin273/RateMesh/internal/policy"
)

// Proxy wraps httputil.ReverseProxy to manage forwarding to downstream target services.
type Proxy struct {
	target *url.URL
	proxy  *httputil.ReverseProxy
}

// NewProxy creates a new ReverseProxy configured for the target URL.
// It hooks into ModifyResponse to run plugin OnResponse interceptors.
func NewProxy(targetURL string, registry *plugin.Registry) (*Proxy, error) {
	target, err := url.Parse(targetURL)
	if err != nil {
		return nil, err
	}

	proxy := httputil.NewSingleHostReverseProxy(target)

	// Use custom transport with explicit timeouts
	proxy.Transport = &http.Transport{
		MaxIdleConns:          100,
		IdleConnTimeout:       90 * time.Second,
		TLSHandshakeTimeout:   10 * time.Second,
		ResponseHeaderTimeout: 5 * time.Second, // Timeout waiting for response header
	}

	// Intercept response from upstream to run plugin OnResponse hooks
	proxy.ModifyResponse = func(resp *http.Response) error {
		if resp.Request == nil {
			return nil
		}

		tenant := policy.GetTenantFromContext(resp.Request.Context())
		if tenant == nil {
			return nil
		}

		enabled := plugin.GetEnabledPlugins(resp.Request.Context())
		if enabled == nil {
			return nil
		}

		rc := &plugin.ResponseContext{
			TenantID: tenant.ID,
			Response: resp,
		}

		return registry.RunOnResponse(resp.Request.Context(), rc, enabled)
	}

	proxy.ErrorHandler = func(w http.ResponseWriter, r *http.Request, err error) {
		log.Printf("proxy error forwarding request to %s: %v", targetURL, err)
		http.Error(w, "upstream service unavailable", http.StatusBadGateway)
	}

	return &Proxy{
		target: target,
		proxy:  proxy,
	}, nil
}

// ServeHTTP implements http.Handler, forwarding the request to the upstream target.
func (p *Proxy) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	// Adjust Host to match target
	r.Host = p.target.Host
	p.proxy.ServeHTTP(w, r)
}
