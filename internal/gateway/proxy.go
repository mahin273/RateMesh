package gateway

import (
	"fmt"
	"log"
	"net/http"
	"net/http/httputil"
	"net/url"
	"time"

	"github.com/mahin273/RateMesh/internal/observability"
	"github.com/mahin273/RateMesh/internal/plugin"
	"github.com/mahin273/RateMesh/internal/policy"
	"github.com/sony/gobreaker/v2"
)

// Proxy wraps httputil.ReverseProxy to manage forwarding to downstream target services.
type Proxy struct {
	target *url.URL
	proxy  *httputil.ReverseProxy
}

type roundTripperWithCB struct {
	transport http.RoundTripper
	cb        *gobreaker.CircuitBreaker[any]
}

func (rt *roundTripperWithCB) RoundTrip(req *http.Request) (*http.Response, error) {
	res, err := rt.cb.Execute(func() (any, error) {
		var innerErr error
		var lastResp *http.Response
		for i := 0; i < 3; i++ {
			// Support retrying request bodies if GetBody is defined
			if req.Body != nil && req.GetBody != nil {
				if body, err := req.GetBody(); err == nil {
					req.Body = body
				}
			}

			lastResp, innerErr = rt.transport.RoundTrip(req)
			if innerErr == nil && lastResp.StatusCode < 500 {
				return lastResp, nil
			}
			if i < 2 {
				time.Sleep(50 * time.Millisecond)
			}
		}
		if innerErr != nil {
			return nil, innerErr
		}
		return nil, fmt.Errorf("upstream server failure status %d", lastResp.StatusCode)
	})

	if err != nil {
		return nil, err
	}
	return res.(*http.Response), nil
}

// NewProxy creates a new ReverseProxy configured for the target URL.
// It hooks into ModifyResponse to run plugin OnResponse interceptors.
func NewProxy(targetURL string, registry *plugin.Registry) (*Proxy, error) {
	target, err := url.Parse(targetURL)
	if err != nil {
		return nil, err
	}

	proxy := httputil.NewSingleHostReverseProxy(target)

	// Set up circuit breaker settings
	cbSettings := gobreaker.Settings{
		Name:        "upstream-circuit-breaker",
		MaxRequests: 3,
		Interval:    10 * time.Second,
		Timeout:     5 * time.Second,
		ReadyToTrip: func(counts gobreaker.Counts) bool {
			failureRate := float64(counts.TotalFailures) / float64(counts.Requests)
			return counts.Requests >= 5 && failureRate >= 0.5
		},
	}
	cb := gobreaker.NewCircuitBreaker[any](cbSettings)

	// Wrap basic transport with circuit breaker and retry handler
	baseTransport := &http.Transport{
		MaxIdleConns:          100,
		IdleConnTimeout:       90 * time.Second,
		TLSHandshakeTimeout:   10 * time.Second,
		ResponseHeaderTimeout: 5 * time.Second,
	}

	proxy.Transport = &roundTripperWithCB{
		transport: baseTransport,
		cb:        cb,
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
		if err == gobreaker.ErrOpenState {
			log.Printf("circuit breaker open for %s: rejecting request", targetURL)
			http.Error(w, "upstream service circuit breaker is open", http.StatusServiceUnavailable)
			return
		}
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
	// Start an OpenTelemetry span for proxy execution
	ctx, span := observability.Tracer.Start(r.Context(), "Proxy.ServeHTTP")
	defer span.End()

	r = r.WithContext(ctx)

	// Adjust Host to match target
	r.Host = p.target.Host
	p.proxy.ServeHTTP(w, r)
}
