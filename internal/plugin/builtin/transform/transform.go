package transform

import (
	"context"
	"encoding/json"

	"github.com/mahin273/RateMesh/internal/plugin"
)

type TransformPlugin struct{}

type HeaderTransform struct {
	Add    map[string]string `json:"add"`
	Remove []string          `json:"remove"`
}

// TransformConfig holds the rules for header manipulation.
type TransformConfig struct {
	Request  HeaderTransform `json:"request"`
	Response HeaderTransform `json:"response"`
}

// NewTransformPlugin constructs a TransformPlugin.
func NewTransformPlugin() plugin.GatewayPlugin {
	return &TransformPlugin{}
}

func (p *TransformPlugin) Name() string {
	return "transform"
}

func (p *TransformPlugin) Priority() int {
	return 50 // Transform runs in the middle of request execution
}

func (p *TransformPlugin) OnRequest(ctx context.Context, rc *plugin.RequestContext) (*plugin.ShortCircuit, error) {
	configBytes := plugin.GetPluginConfig(ctx, p.Name())
	if configBytes == nil {
		return nil, nil
	}

	var cfg TransformConfig
	if err := json.Unmarshal(configBytes, &cfg); err != nil {
		return nil, nil
	}

	// 1. Add headers
	for k, v := range cfg.Request.Add {
		rc.Request.Header.Set(k, v)
	}

	// 2. Remove headers
	for _, k := range cfg.Request.Remove {
		rc.Request.Header.Del(k)
	}

	return nil, nil
}

func (p *TransformPlugin) OnResponse(ctx context.Context, rc *plugin.ResponseContext) error {
	configBytes := plugin.GetPluginConfig(ctx, p.Name())
	if configBytes == nil {
		return nil
	}

	var cfg TransformConfig
	if err := json.Unmarshal(configBytes, &cfg); err != nil {
		return nil
	}

	if rc.Response != nil {
		// 1. Add headers
		for k, v := range cfg.Response.Add {
			rc.Response.Header.Set(k, v)
		}

		// 2. Remove headers
		for _, k := range cfg.Response.Remove {
			rc.Response.Header.Del(k)
		}
	}

	return nil
}
