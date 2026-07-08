package ratelimit

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"net/http"
	"strconv"

	"github.com/mahin273/RateMesh/internal/policy"
)

type contextKey string

const PolicyKey contextKey = "policy"

// RateLimiter returns a middleware that resolves route policy and enforces strict rate limits.
func RateLimiter(
	policyService policy.Service,
	tokenBucket RateLimitStrategy,
	slidingWindow RateLimitStrategy,
) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			tenant := policy.GetTenantFromContext(r.Context())
			if tenant == nil {
				http.Error(w, "unauthorized tenant context", http.StatusUnauthorized)
				return
			}

			// Resolve route policy
			p, err := policyService.ResolveRoutePolicy(r.Context(), tenant.ID, r.Method, r.URL.Path)
			if err != nil {
				http.Error(w, "internal server error", http.StatusInternalServerError)
				return
			}

			if p == nil {
				http.Error(w, "no route policy matches requested pattern", http.StatusNotFound)
				return
			}

			// Attach policy to context
			ctx := context.WithValue(r.Context(), PolicyKey, p)
			r = r.WithContext(ctx)

			// Determine key and strategy
			routeHash := hashRoute(p.RoutePattern)
			var limitKey string
			var strategy RateLimitStrategy

			switch p.Strategy {
			case policy.StrategySlidingWindow:
				limitKey = fmt.Sprintf("ratelimit:sw:%s:%s", tenant.ID, routeHash)
				strategy = slidingWindow
			case policy.StrategyTokenBucket:
				fallthrough
			default:
				limitKey = fmt.Sprintf("ratelimit:%s:%s", tenant.ID, routeHash)
				strategy = tokenBucket
			}

			// Strict mode synchronous check (reconciler and local counting are wired in Phase 3)
			res, err := strategy.Check(r.Context(), limitKey, p.LimitPerWindow, p.WindowSeconds, p.BurstAllowance)
			if err != nil {
				// Standard rate-limiting error fallback
				http.Error(w, "rate limit evaluation failed", http.StatusInternalServerError)
				return
			}

			// Inject Rate-Limiting Headers
			w.Header().Set("X-RateLimit-Limit", strconv.Itoa(p.LimitPerWindow))
			w.Header().Set("X-RateLimit-Remaining", strconv.Itoa(res.Remaining))
			w.Header().Set("X-RateLimit-Reset", strconv.Itoa(int(res.Reset.Seconds())))

			if !res.Allowed {
				http.Error(w, "too many requests", http.StatusTooManyRequests)
				return
			}

			next.ServeHTTP(w, r)
		})
	}
}

// hashRoute generates a short 8-character hex hash from the route pattern.
func hashRoute(routePattern string) string {
	hash := sha256.Sum256([]byte(routePattern))
	return hex.EncodeToString(hash[:4])
}

// GetPolicyFromContext retrieves the matching route policy from the request context.
func GetPolicyFromContext(ctx context.Context) *policy.RoutePolicy {
	if val := ctx.Value(PolicyKey); val != nil {
		if p, ok := val.(*policy.RoutePolicy); ok {
			return p
		}
	}
	return nil
}
