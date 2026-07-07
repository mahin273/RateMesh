package gateway

import (
	"log"
	"net/http"
	"net/http/httputil"
	"net/url"
	"time"
)

// Proxy wraps httputil.ReverseProxy to manage forwarding to downstream target services.
type Proxy struct {
	target *url.URL
	proxy  *httputil.ReverseProxy
}

// NewProxy creates a new ReverseProxy configured for the target URL.
func NewProxy(targetURL string) (*Proxy, error) {
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
