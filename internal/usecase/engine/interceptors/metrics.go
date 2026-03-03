package interceptors

import (
	"time"

	"github.com/Elysian-Rebirth/backend-go/internal/infrastructure/telemetry"
	"github.com/Elysian-Rebirth/backend-go/internal/usecase/engine"
)

// NewTelemetryInterceptor tracks raw node execution latency and failure volume per tenant and node type.
// It uses Prometheus metric vectors exposed on /metrics.
func NewTelemetryInterceptor() engine.Interceptor {
	return func(node engine.Node, ctx *engine.ExecutionContext, next func() error) error {
		start := time.Now()
		err := next()
		duration := time.Since(start).Seconds()

		tenantIDStr, _ := ctx.Get("tenant_id")
		tenantID := "unknown"
		if s, ok := tenantIDStr.(string); ok && s != "" {
			tenantID = s
		}

		// Record Latency
		telemetry.NodeExecutionLatency.WithLabelValues(tenantID, node.Type).Observe(duration)

		// Record Failure Rate
		if err != nil {
			telemetry.NodeExecutionFailure.WithLabelValues(tenantID, node.Type).Inc()
		}

		return err
	}
}
