package plugin

import (
	"context"
	"sort"
)

// Registry manages the set of active plugins and executes them in order.
type Registry struct {
	plugins []GatewayPlugin
}

// NewRegistry initializes a new Registry instance.
func NewRegistry() *Registry {
	return &Registry{
		plugins: make([]GatewayPlugin, 0),
	}
}

// Register adds a plugin and resorts the execution order by Priority.
func (r *Registry) Register(p GatewayPlugin) {
	r.plugins = append(r.plugins, p)
	// Sort by Priority ascending (lower values execute first on request)
	sort.Slice(r.plugins, func(i, j int) bool {
		return r.plugins[i].Priority() < r.plugins[j].Priority()
	})
}

// RunOnRequest executes OnRequest for all registered and enabled plugins.
func (r *Registry) RunOnRequest(ctx context.Context, rc *RequestContext, enabled map[string]bool) (*ShortCircuit, error) {
	for _, p := range r.plugins {
		if !enabled[p.Name()] {
			continue
		}
		sc, err := p.OnRequest(ctx, rc)
		if err != nil || sc != nil {
			return sc, err
		}
	}
	return nil, nil
}

// RunOnResponse executes OnResponse in reverse priority order for enabled plugins.
func (r *Registry) RunOnResponse(ctx context.Context, rc *ResponseContext, enabled map[string]bool) error {
	for i := len(r.plugins) - 1; i >= 0; i-- {
		p := r.plugins[i]
		if !enabled[p.Name()] {
			continue
		}
		if err := p.OnResponse(ctx, rc); err != nil {
			return err
		}
	}
	return nil
}
