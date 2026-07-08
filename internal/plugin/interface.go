package plugin

import (
	"context"
	"net/http"
)

// RequestContext represents the state and request object passed to OnRequest hooks.
type RequestContext struct {
	TenantID string
	Route    string
	Request  *http.Request
}

// ResponseContext represents the state and response object passed to OnResponse hooks.
type ResponseContext struct {
	TenantID string
	Response *http.Response
}

// ShortCircuit is returned by OnRequest hooks to signal that the request processing
// should terminate early with a specific status and body (e.g., auth failure).
type ShortCircuit struct {
	StatusCode int
	Body       []byte
	Headers    http.Header
}

// GatewayPlugin defines the interface that all API gateway plugins must satisfy.
type GatewayPlugin interface {
	Name() string
	Priority() int // Lower values run first
	OnRequest(ctx context.Context, rc *RequestContext) (*ShortCircuit, error)
	OnResponse(ctx context.Context, rc *ResponseContext) error
}

type pluginContextKey string

const ConfigsKey pluginContextKey = "plugin_configs"

// AttachPluginConfigs attaches plugin configs to the context.
func AttachPluginConfigs(ctx context.Context, configs map[string][]byte) context.Context {
	return context.WithValue(ctx, ConfigsKey, configs)
}

// GetPluginConfig retrieves the plugin-specific configuration byte slice.
func GetPluginConfig(ctx context.Context, pluginName string) []byte {
	if val := ctx.Value(ConfigsKey); val != nil {
		if m, ok := val.(map[string][]byte); ok {
			return m[pluginName]
		}
	}
	return nil
}
