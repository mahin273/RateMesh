package logging

import (
	"context"
	"log"
	"time"

	"github.com/mahin273/RateMesh/internal/plugin"
)

type contextKey string

const StartTimeKey contextKey = "log_start_time"

type LoggingPlugin struct{}

// NewLoggingPlugin constructs a LoggingPlugin.
func NewLoggingPlugin() plugin.GatewayPlugin {
	return &LoggingPlugin{}
}

func (p *LoggingPlugin) Name() string {
	return "logging"
}

func (p *LoggingPlugin) Priority() int {
	return 100 // Logger runs late on requests, early on responses
}

func (p *LoggingPlugin) OnRequest(ctx context.Context, rc *plugin.RequestContext) (*plugin.ShortCircuit, error) {
	// Store start time in request's internal context so it propagates to response
	startTime := time.Now()
	reqCtx := context.WithValue(rc.Request.Context(), StartTimeKey, startTime)
	rc.Request = rc.Request.WithContext(reqCtx)

	log.Printf("[Plugin:Logging] Request Start: Tenant=%s Method=%s Path=%s", rc.TenantID, rc.Request.Method, rc.Request.URL.Path)
	return nil, nil
}

func (p *LoggingPlugin) OnResponse(ctx context.Context, rc *plugin.ResponseContext) error {
	var elapsed time.Duration
	if rc.Response != nil && rc.Response.Request != nil {
		if val := rc.Response.Request.Context().Value(StartTimeKey); val != nil {
			if startTime, ok := val.(time.Time); ok {
				elapsed = time.Since(startTime)
			}
		}
	}

	statusCode := 0
	if rc.Response != nil {
		statusCode = rc.Response.StatusCode
	}

	log.Printf("[Plugin:Logging] Response End: Tenant=%s Status=%d Latency=%v", rc.TenantID, statusCode, elapsed)
	return nil
}
