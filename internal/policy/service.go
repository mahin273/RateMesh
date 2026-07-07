package policy

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"strings"
	"time"
)

// Cache interface to allow Redis cache integration in later phases.
type Cache interface {
	Get(ctx context.Context, key string) (string, error)
	Set(ctx context.Context, key string, value string, ttl time.Duration) error
}

type Service interface {
	GetTenant(ctx context.Context, id string) (*Tenant, error)
	ResolveRoutePolicy(ctx context.Context, tenantID, method, path string) (*RoutePolicy, error)
}

type service struct {
	repo  Repository
	cache Cache
}

func NewService(repo Repository, cache Cache) Service {
	return &service{
		repo:  repo,
		cache: cache,
	}
}

func (s *service) GetTenant(ctx context.Context, id string) (*Tenant, error) {
	return s.repo.GetTenant(ctx, id)
}

func (s *service) ResolveRoutePolicy(ctx context.Context, tenantID, method, path string) (*RoutePolicy, error) {
	cacheKey := fmt.Sprintf("policy_cache:%s:%s:%s", tenantID, method, path)

	// Attempt cache lookup if cache is available
	if s.cache != nil {
		if cachedVal, err := s.cache.Get(ctx, cacheKey); err == nil && cachedVal != "" {
			var p RoutePolicy
			if err := json.Unmarshal([]byte(cachedVal), &p); err == nil {
				return &p, nil
			}
		}
	}

	// Fallback to database
	policies, err := s.repo.GetRoutePoliciesByTenant(ctx, tenantID)
	if err != nil {
		return nil, fmt.Errorf("failed to fetch route policies: %w", err)
	}

	var matchedPolicy *RoutePolicy
	for _, p := range policies {
		if (p.Method == "*" || strings.EqualFold(p.Method, method)) && matchRoute(p.RoutePattern, path) {
			matchedPolicy = p
			break
		}
	}

	// If a match is found and cache is available, populate the cache
	if matchedPolicy != nil && s.cache != nil {
		if data, err := json.Marshal(matchedPolicy); err == nil {
			if err := s.cache.Set(ctx, cacheKey, string(data), 60*time.Second); err != nil {
				log.Printf("failed to populate policy cache: %v", err)
			}
		}
	}

	return matchedPolicy, nil
}

func matchRoute(pattern, path string) bool {
	if pattern == path {
		return true
	}
	if pattern == "*" || pattern == "/*" {
		return true
	}
	// Prefix wildcard pattern support, e.g., "/api/v1/*"
	if strings.HasSuffix(pattern, "*") {
		prefix := pattern[:len(pattern)-1]
		return strings.HasPrefix(path, prefix)
	}
	return false
}
