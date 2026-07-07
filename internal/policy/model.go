package policy

import (
	"time"
)

// Tenant represents a multi-tenant client.
type Tenant struct {
	ID        string    `json:"id"`
	Name      string    `json:"name"`
	PlanTier  string    `json:"plan_tier"`
	CreatedAt time.Time `json:"created_at"`
}

// RateLimitStrategy defines the algorithm used for rate limiting.
type RateLimitStrategy string

const (
	StrategyTokenBucket  RateLimitStrategy = "token_bucket"
	StrategySlidingWindow RateLimitStrategy = "sliding_window"
)

// RateLimitMode defines whether limiting is synchronous or eventual.
type RateLimitMode string

const (
	ModeStrict   RateLimitMode = "strict"
	ModeEventual RateLimitMode = "eventual"
)

// RoutePolicy holds rate limiting and routing definitions per tenant/route.
type RoutePolicy struct {
	ID             string            `json:"id"`
	TenantID       string            `json:"tenant_id"`
	RoutePattern   string            `json:"route_pattern"`
	Method         string            `json:"method"`
	Strategy       RateLimitStrategy `json:"strategy"`
	Mode           RateLimitMode     `json:"mode"`
	LimitPerWindow int               `json:"limit_per_window"`
	WindowSeconds  int               `json:"window_seconds"`
	BurstAllowance int               `json:"burst_allowance"`
	CreatedAt      time.Time         `json:"created_at"`
	UpdatedAt      time.Time         `json:"updated_at"`
}

// Plugin represents a tenant-level plugin config.
type Plugin struct {
	ID         string `json:"id"`
	TenantID   string `json:"tenant_id"`
	PluginName string `json:"plugin_name"`
	ConfigJSON string `json:"config_json"`
	Enabled    bool   `json:"enabled"`
	Priority   int    `json:"priority"`
}
