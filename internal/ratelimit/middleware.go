package ratelimit

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"net/http"
	"strconv"
	"time"

	"github.com/mahin273/RateMesh/internal/observability"
	"github.com/mahin273/RateMesh/internal/plugin"
	"github.com/mahin273/RateMesh/internal/policy"
)

// RateLimitPlugin implements GatewayPlugin to handle distributed rate limiting in the plugin execution pipeline.
type RateLimitPlugin struct {
	policyService policy.Service
	tokenBucket   RateLimitStrategy
	slidingWindow RateLimitStrategy
	localStore    *LocalBucketStore
}

// NewRateLimitPlugin constructs a new RateLimitPlugin instance.
func NewRateLimitPlugin(
	policyService policy.Service,
	tokenBucket RateLimitStrategy,
	slidingWindow RateLimitStrategy,
	localStore *LocalBucketStore,
) plugin.GatewayPlugin {
	return &RateLimitPlugin{
		policyService: policyService,
		tokenBucket:   tokenBucket,
		slidingWindow: slidingWindow,
		localStore:    localStore,
	}
}

func (p *RateLimitPlugin) Name() string {
	return "rate-limit"
}

func (p *RateLimitPlugin) Priority() int {
	return 20 // Runs after auth (10) but before transform (50) and logging (100)
}

type rateLimitState struct {
	limit     int
	remaining int
	resetSecs int
}

func (p *RateLimitPlugin) OnRequest(ctx context.Context, rc *plugin.RequestContext) (*plugin.ShortCircuit, error) {
	ctx, span := observability.Tracer.Start(ctx, "RateLimitPlugin.OnRequest")
	defer span.End()

	tenant := policy.GetTenantFromContext(ctx)
	if tenant == nil {
		return &plugin.ShortCircuit{
			StatusCode: http.StatusUnauthorized,
			Body:       []byte("unauthorized tenant context"),
		}, nil
	}

	// Resolve route policy
	routePolicy, err := p.policyService.ResolveRoutePolicy(ctx, tenant.ID, rc.Request.Method, rc.Request.URL.Path)
	if err != nil {
		return &plugin.ShortCircuit{
			StatusCode: http.StatusInternalServerError,
			Body:       []byte("internal server error"),
		}, nil
	}

	if routePolicy == nil {
		return &plugin.ShortCircuit{
			StatusCode: http.StatusNotFound,
			Body:       []byte("no route policy matches requested pattern"),
		}, nil
	}

	// Attach policy to request context so downstream plugins can access it
	reqCtx := context.WithValue(rc.Request.Context(), "policy", routePolicy)
	rc.Request = rc.Request.WithContext(reqCtx)

	routeHash := hashRoute(routePolicy.RoutePattern)
	var limitKey string

	switch routePolicy.Strategy {
	case policy.StrategySlidingWindow:
		limitKey = fmt.Sprintf("ratelimit:sw:%s:%s", tenant.ID, routeHash)
	case policy.StrategyTokenBucket:
		fallthrough
	default:
		limitKey = fmt.Sprintf("ratelimit:%s:%s", tenant.ID, routeHash)
	}

	var allowed bool
	var remaining int
	var resetSecs int

	if routePolicy.Mode == policy.ModeEventual {
		bucket := p.localStore.GetOrCreate(limitKey, routePolicy.LimitPerWindow, routePolicy.WindowSeconds, routePolicy.BurstAllowance)
		allowed, remaining = bucket.Allow(time.Now())
		refillRate := float64(routePolicy.LimitPerWindow) / float64(routePolicy.WindowSeconds)
		maxTokens := routePolicy.LimitPerWindow + routePolicy.BurstAllowance
		if remaining < maxTokens && refillRate > 0 {
			missing := float64(maxTokens - remaining)
			resetSecs = int(missing / refillRate)
		}
	} else {
		var strategy RateLimitStrategy
		if routePolicy.Strategy == policy.StrategySlidingWindow {
			strategy = p.slidingWindow
		} else {
			strategy = p.tokenBucket
		}

		res, err := strategy.Check(ctx, limitKey, routePolicy.LimitPerWindow, routePolicy.WindowSeconds, routePolicy.BurstAllowance)
		if err != nil {
			return &plugin.ShortCircuit{
				StatusCode: http.StatusInternalServerError,
				Body:       []byte("rate limit evaluation failed"),
			}, nil
		}
		allowed = res.Allowed
		remaining = res.Remaining
		resetSecs = int(res.Reset.Seconds())
	}

	// Store rate limit parameters in context to allow headers injection in OnResponse
	state := &rateLimitState{
		limit:     routePolicy.LimitPerWindow,
		remaining: remaining,
		resetSecs: resetSecs,
	}
	reqCtx = context.WithValue(rc.Request.Context(), "rate_limit_state", state)
	rc.Request = rc.Request.WithContext(reqCtx)

	action := "allowed"
	if !allowed {
		action = "blocked"
	}
	observability.RateLimitHitsTotal.WithLabelValues(
		tenant.ID,
		routePolicy.RoutePattern,
		string(routePolicy.Strategy),
		string(routePolicy.Mode),
		action,
	).Inc()

	if !allowed {
		headers := make(http.Header)
		headers.Set("X-RateLimit-Limit", strconv.Itoa(routePolicy.LimitPerWindow))
		headers.Set("X-RateLimit-Remaining", strconv.Itoa(remaining))
		headers.Set("X-RateLimit-Reset", strconv.Itoa(resetSecs))

		return &plugin.ShortCircuit{
			StatusCode: http.StatusTooManyRequests,
			Body:       []byte("too many requests"),
			Headers:    headers,
		}, nil
	}

	return nil, nil
}

func (p *RateLimitPlugin) OnResponse(ctx context.Context, rc *plugin.ResponseContext) error {
	if rc.Response != nil && rc.Response.Request != nil {
		if val := rc.Response.Request.Context().Value("rate_limit_state"); val != nil {
			if state, ok := val.(*rateLimitState); ok {
				rc.Response.Header.Set("X-RateLimit-Limit", strconv.Itoa(state.limit))
				rc.Response.Header.Set("X-RateLimit-Remaining", strconv.Itoa(state.remaining))
				rc.Response.Header.Set("X-RateLimit-Reset", strconv.Itoa(state.resetSecs))
			}
		}
	}
	return nil
}

func hashRoute(routePattern string) string {
	hash := sha256.Sum256([]byte(routePattern))
	return hex.EncodeToString(hash[:4])
}
