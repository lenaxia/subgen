package discovery

import (
	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promauto"
)

var (
	// WorkerCount tracks the number of workers
	WorkerCount = promauto.NewGaugeVec(
		prometheus.GaugeOpts{
			Name: "subgen_worker_count",
			Help: "Number of workers by health status",
		},
		[]string{"status"}, // healthy, unhealthy
	)

	// WorkerDiscoveryErrors tracks discovery errors
	WorkerDiscoveryErrors = promauto.NewCounter(
		prometheus.CounterOpts{
			Name: "subgen_worker_discovery_errors_total",
			Help: "Total number of worker discovery errors",
		},
	)

	// WorkerSelectionTotal tracks worker selections
	WorkerSelectionTotal = promauto.NewCounterVec(
		prometheus.CounterOpts{
			Name: "subgen_worker_selection_total",
			Help: "Total number of worker selections",
		},
		[]string{"strategy"}, // round_robin, least_loaded
	)

	// WorkerHealthCheckDuration tracks health check duration
	WorkerHealthCheckDuration = promauto.NewHistogram(
		prometheus.HistogramOpts{
			Name:    "subgen_worker_health_check_duration_seconds",
			Help:    "Duration of worker health checks",
			Buckets: prometheus.DefBuckets,
		},
	)

	// WorkerWatchEventsTotal tracks watch events by type (added, removed, updated, error)
	WorkerWatchEventsTotal = promauto.NewCounterVec(
		prometheus.CounterOpts{
			Name: "subgen_worker_watch_events_total",
			Help: "Total worker watch events by type",
		},
		[]string{"type"}, // added, removed, updated, error
	)

	// WorkerWatchReconnectsTotal tracks watch reconnection attempts
	WorkerWatchReconnectsTotal = promauto.NewCounter(
		prometheus.CounterOpts{
			Name: "subgen_worker_watch_reconnects_total",
			Help: "Total watch reconnection attempts",
		},
	)

	// WorkerWatchErrorsTotal tracks watch errors
	WorkerWatchErrorsTotal = promauto.NewCounter(
		prometheus.CounterOpts{
			Name: "subgen_worker_watch_errors_total",
			Help: "Total watch errors from Kubernetes API",
		},
	)
)

// UpdateWorkerMetrics updates worker count metrics
func UpdateWorkerMetrics(workers []Worker) {
	healthy := 0
	unhealthy := 0

	for _, w := range workers {
		if w.Healthy {
			healthy++
		} else {
			unhealthy++
		}
	}

	WorkerCount.WithLabelValues("healthy").Set(float64(healthy))
	WorkerCount.WithLabelValues("unhealthy").Set(float64(unhealthy))
}
