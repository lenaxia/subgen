package grpc_client

import (
	"github.com/prometheus/client_golang/prometheus"
)

// ClientMetrics holds Prometheus metrics for gRPC client
type ClientMetrics struct {
	RPCCalls    *prometheus.CounterVec
	RPCErrors   *prometheus.CounterVec
	RPCDuration *prometheus.HistogramVec
	registry    *prometheus.Registry
}

// NewClientMetrics creates Prometheus metrics for gRPC client
// If registry is nil, uses the default registry
func NewClientMetrics() *ClientMetrics {
	return NewClientMetricsWithRegistry(nil)
}

// NewClientMetricsWithRegistry creates metrics with a custom registry
func NewClientMetricsWithRegistry(registry *prometheus.Registry) *ClientMetrics {
	if registry == nil {
		registry = prometheus.DefaultRegisterer.(*prometheus.Registry)
	}

	rpcCalls := prometheus.NewCounterVec(
		prometheus.CounterOpts{
			Name: "subgen_grpc_calls_total",
			Help: "Total number of gRPC calls by method",
		},
		[]string{"method"},
	)

	rpcErrors := prometheus.NewCounterVec(
		prometheus.CounterOpts{
			Name: "subgen_grpc_errors_total",
			Help: "Total number of gRPC errors by method",
		},
		[]string{"method"},
	)

	rpcDuration := prometheus.NewHistogramVec(
		prometheus.HistogramOpts{
			Name:    "subgen_grpc_duration_seconds",
			Help:    "gRPC call duration in seconds",
			Buckets: []float64{0.1, 1, 10, 30, 60, 300, 600, 1800, 3600}, // 0.1s to 1hr
		},
		[]string{"method", "status"},
	)

	// Register metrics (ignore errors if already registered)
	registry.Register(rpcCalls)
	registry.Register(rpcErrors)
	registry.Register(rpcDuration)

	return &ClientMetrics{
		RPCCalls:    rpcCalls,
		RPCErrors:   rpcErrors,
		RPCDuration: rpcDuration,
		registry:    registry,
	}
}
