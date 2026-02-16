package queue

import (
	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promauto"
)

// QueueMetrics holds Prometheus metrics for the queue
type QueueMetrics struct {
	// Gauge metrics
	QueueSize      prometheus.Gauge
	ProcessingSize prometheus.Gauge

	// Counter metrics
	TasksQueued    *prometheus.CounterVec
	TasksCompleted *prometheus.CounterVec
	TasksFailed    *prometheus.CounterVec

	// Histogram metrics
	TaskWaitTime       *prometheus.HistogramVec
	TaskProcessingTime *prometheus.HistogramVec
}

// NewQueueMetrics creates Prometheus metrics for the queue
func NewQueueMetrics() *QueueMetrics {
	return NewQueueMetricsWithRegistry(prometheus.DefaultRegisterer)
}

// NewQueueMetricsWithRegistry creates metrics with a custom registry
func NewQueueMetricsWithRegistry(registerer prometheus.Registerer) *QueueMetrics {
	factory := promauto.With(registerer)

	return &QueueMetrics{
		QueueSize: factory.NewGauge(prometheus.GaugeOpts{
			Name: "subgen_queue_size",
			Help: "Current number of tasks in the queue",
		}),

		ProcessingSize: factory.NewGauge(prometheus.GaugeOpts{
			Name: "subgen_queue_processing_size",
			Help: "Current number of tasks being processed",
		}),

		TasksQueued: factory.NewCounterVec(
			prometheus.CounterOpts{
				Name: "subgen_tasks_queued_total",
				Help: "Total number of tasks queued",
			},
			[]string{"type"}, // Task type label
		),

		TasksCompleted: factory.NewCounterVec(
			prometheus.CounterOpts{
				Name: "subgen_tasks_completed_total",
				Help: "Total number of tasks completed",
			},
			[]string{"type"},
		),

		TasksFailed: factory.NewCounterVec(
			prometheus.CounterOpts{
				Name: "subgen_tasks_failed_total",
				Help: "Total number of tasks that failed",
			},
			[]string{"type"},
		),

		TaskWaitTime: factory.NewHistogramVec(
			prometheus.HistogramOpts{
				Name:    "subgen_task_wait_time_seconds",
				Help:    "Time tasks spent waiting in queue",
				Buckets: []float64{1, 5, 10, 30, 60, 300, 600, 1800}, // 1s to 30min
			},
			[]string{"type"},
		),

		TaskProcessingTime: factory.NewHistogramVec(
			prometheus.HistogramOpts{
				Name:    "subgen_task_processing_time_seconds",
				Help:    "Time tasks spent processing",
				Buckets: []float64{10, 30, 60, 120, 300, 600, 1800, 3600}, // 10s to 1hr
			},
			[]string{"type"},
		),
	}
}
