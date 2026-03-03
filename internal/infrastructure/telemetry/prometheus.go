package telemetry

import (
	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promauto"
)

var (
	// TokenConsumption tracks AI token usage per tenant, model, and operation type
	TokenConsumption = promauto.NewCounterVec(prometheus.CounterOpts{
		Name: "elysian_ai_token_consumption_total",
		Help: "Total AI tokens consumed by model and tenant",
	}, []string{"tenant_id", "model", "type"}) // type: "prompt" or "completion"

	// RagLatency measures distribution of vector retrieval times
	RagLatency = promauto.NewHistogramVec(prometheus.HistogramOpts{
		Name:    "elysian_rag_retrieval_latency_seconds",
		Help:    "Latency of RAG hybrid search operations",
		Buckets: prometheus.DefBuckets,
	}, []string{"tenant_id"})

	// NodeExecutionFailure tracks which workflow nodes fail most frequently
	NodeExecutionFailure = promauto.NewCounterVec(prometheus.CounterOpts{
		Name: "elysian_workflow_node_failures_total",
		Help: "Total failed workflow node executions",
	}, []string{"tenant_id", "node_type"})

	// NodeExecutionLatency tracks execution duration per node type
	NodeExecutionLatency = promauto.NewHistogramVec(prometheus.HistogramOpts{
		Name:    "elysian_workflow_node_latency_seconds",
		Help:    "Latency of workflow DAG node executions",
		Buckets: prometheus.DefBuckets,
	}, []string{"tenant_id", "node_type"})
)
