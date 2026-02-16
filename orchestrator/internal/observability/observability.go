package observability

import (
	"fmt"
	"time"

	"github.com/gofiber/fiber/v2"
	"github.com/prometheus/client_golang/prometheus"
	"github.com/sirupsen/logrus"
)

// Metrics holds all Prometheus metrics for observability
type Metrics struct {
	// HTTP metrics
	HTTPRequests         *prometheus.CounterVec
	HTTPDuration         *prometheus.HistogramVec
	HTTPRequestsInFlight prometheus.Gauge

	// Worker metrics
	WorkerCount   prometheus.Gauge
	WorkerHealthy prometheus.Gauge

	// Application up indicator
	Up prometheus.Gauge

	registry *prometheus.Registry
}

// NewMetrics creates global metrics using default registry
func NewMetrics() *Metrics {
	return NewMetricsWithRegistry(nil)
}

// NewMetricsWithRegistry creates metrics with a custom registry
func NewMetricsWithRegistry(registry *prometheus.Registry) *Metrics {
	if registry == nil {
		registry = prometheus.NewRegistry()
	}

	httpRequests := prometheus.NewCounterVec(
		prometheus.CounterOpts{
			Name: "subgen_http_requests_total",
			Help: "Total number of HTTP requests",
		},
		[]string{"method", "endpoint", "status"},
	)

	httpDuration := prometheus.NewHistogramVec(
		prometheus.HistogramOpts{
			Name:    "subgen_http_request_duration_seconds",
			Help:    "HTTP request duration in seconds",
			Buckets: []float64{0.001, 0.005, 0.01, 0.05, 0.1, 0.5, 1.0, 5.0, 10.0},
		},
		[]string{"method", "endpoint"},
	)

	httpRequestsInFlight := prometheus.NewGauge(prometheus.GaugeOpts{
		Name: "subgen_http_requests_in_flight",
		Help: "Current number of HTTP requests being processed",
	})

	workerCount := prometheus.NewGauge(prometheus.GaugeOpts{
		Name: "subgen_worker_count",
		Help: "Number of discovered workers",
	})

	workerHealthy := prometheus.NewGauge(prometheus.GaugeOpts{
		Name: "subgen_worker_healthy",
		Help: "Number of healthy workers",
	})

	up := prometheus.NewGauge(prometheus.GaugeOpts{
		Name: "subgen_up",
		Help: "Always 1 - indicates service is up",
	})

	// Register metrics
	registry.Register(httpRequests)
	registry.Register(httpDuration)
	registry.Register(httpRequestsInFlight)
	registry.Register(workerCount)
	registry.Register(workerHealthy)
	registry.Register(up)

	return &Metrics{
		HTTPRequests:         httpRequests,
		HTTPDuration:         httpDuration,
		HTTPRequestsInFlight: httpRequestsInFlight,
		WorkerCount:          workerCount,
		WorkerHealthy:        workerHealthy,
		Up:                   up,
		registry:             registry,
	}
}

// SetUp sets the up metric to 1
func (m *Metrics) SetUp() {
	m.Up.Set(1)
}

// Worker represents a worker node (interface for testing)
type Worker struct {
	ID      string
	Address string
	Healthy bool
	Active  int
}

// WorkerPool interface for health checks
type WorkerPool interface {
	GetWorkers() ([]Worker, error)
}

// Queue interface for health checks
type Queue interface {
	Size() int
	ProcessingCount() int
	IsIdle() bool
}

// RequestLoggerMiddleware logs HTTP requests with structured logging
func RequestLoggerMiddleware(metrics *Metrics, log *logrus.Logger) fiber.Handler {
	return func(c *fiber.Ctx) error {
		start := time.Now()

		// Track in-flight requests
		metrics.HTTPRequestsInFlight.Inc()
		defer metrics.HTTPRequestsInFlight.Dec()

		// Process request
		err := c.Next()

		duration := time.Since(start)
		status := c.Response().StatusCode()

		// Log request
		log.WithFields(logrus.Fields{
			"method":      c.Method(),
			"path":        c.Path(),
			"status":      status,
			"duration_ms": duration.Milliseconds(),
			"ip":          c.IP(),
			"user_agent":  c.Get("User-Agent"),
		}).Info("HTTP request")

		// Update metrics
		metrics.HTTPRequests.WithLabelValues(
			c.Method(),
			c.Path(),
			fmt.Sprintf("%d", status),
		).Inc()

		metrics.HTTPDuration.WithLabelValues(
			c.Method(),
			c.Path(),
		).Observe(duration.Seconds())

		return err
	}
}

// PanicRecoveryMiddleware recovers from panics and returns 500
func PanicRecoveryMiddleware(log *logrus.Logger) fiber.Handler {
	return func(c *fiber.Ctx) error {
		defer func() {
			if r := recover(); r != nil {
				log.WithFields(logrus.Fields{
					"panic": r,
					"path":  c.Path(),
				}).Error("Panic recovered")

				c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
					"error": "Internal server error",
				})
			}
		}()

		return c.Next()
	}
}

// RegisterHealthEndpoints adds /health and /ready endpoints to the app
func RegisterHealthEndpoints(
	app *fiber.App,
	metrics *Metrics,
	pool WorkerPool,
	queue Queue,
	startTime time.Time,
	log *logrus.Logger,
) {
	// Health endpoint - basic liveness check
	app.Get("/health", func(c *fiber.Ctx) error {
		uptime := time.Since(startTime)

		return c.JSON(fiber.Map{
			"status":  "healthy",
			"version": "v0.1.0", // TODO: Get from build ldflags
			"uptime":  uptime.String(),
		})
	})

	// Ready endpoint - readiness check (checks dependencies)
	app.Get("/ready", func(c *fiber.Ctx) error {
		// Check if workers are available
		workers, err := pool.GetWorkers()
		if err != nil || len(workers) == 0 {
			return c.Status(fiber.StatusServiceUnavailable).JSON(fiber.Map{
				"status": "not_ready",
				"reason": "no workers available",
			})
		}

		// Check if at least one worker is healthy
		healthyCount := 0
		for _, w := range workers {
			if w.Healthy {
				healthyCount++
			}
		}

		if healthyCount == 0 {
			return c.Status(fiber.StatusServiceUnavailable).JSON(fiber.Map{
				"status": "not_ready",
				"reason": "no healthy workers",
			})
		}

		return c.JSON(fiber.Map{
			"status":          "ready",
			"workers_total":   len(workers),
			"workers_healthy": healthyCount,
			"queue_size":      queue.Size(),
			"processing":      queue.ProcessingCount(),
		})
	})

	// Queue status endpoint
	app.Get("/queue", func(c *fiber.Ctx) error {
		return c.JSON(fiber.Map{
			"queue_size": queue.Size(),
			"processing": queue.ProcessingCount(),
			"is_idle":    queue.IsIdle(),
		})
	})
}
