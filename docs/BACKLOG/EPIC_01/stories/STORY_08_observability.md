# STORY_08: Observability (Metrics, Logging, Health)

**Status:** Not Started  
**Effort:** 4-6 hours  
**Epic:** EPIC_01 (Go Orchestrator Core)  
**Created:** 2026-02-15

---

## User Story

**As a** system administrator  
**I want** Prometheus metrics, structured logging, and health checks  
**So that** I can monitor the orchestrator's health and performance in production

---

## Acceptance Criteria

- [ ] Prometheus metrics endpoint at `/metrics` on port 9090
- [ ] Structured logging with logrus (JSON format in production)
- [ ] Health check endpoint at `/health` returning JSON
- [ ] Readiness endpoint at `/ready` for Kubernetes probes
- [ ] All metrics from previous stories integrated
- [ ] Log levels configurable via LOG_LEVEL env var
- [ ] Request logging middleware for all HTTP endpoints
- [ ] Panic recovery middleware
- [ ] 8+ test cases
- [ ] Grafana dashboard JSON (optional, for reference)
- [ ] Work log created

---

## Integration Points

### Existing Metrics from Previous Stories

**Queue Metrics (STORY_04):**
- `subgen_queue_size` - Current queue size
- `subgen_queue_processing_size` - Tasks being processed
- `subgen_tasks_queued_total` - Total tasks queued
- `subgen_tasks_completed_total` - Total tasks completed
- `subgen_tasks_failed_total` - Total tasks failed
- `subgen_task_wait_time_seconds` - Time in queue
- `subgen_task_processing_time_seconds` - Processing duration

**gRPC Client Metrics (STORY_07):**
- `subgen_grpc_calls_total` - Total RPC calls
- `subgen_grpc_errors_total` - Total RPC errors
- `subgen_grpc_duration_seconds` - RPC duration

**Additional Metrics Needed:**
- `subgen_http_requests_total` - HTTP requests by endpoint
- `subgen_http_request_duration_seconds` - HTTP latency
- `subgen_worker_count` - Number of discovered workers
- `subgen_worker_healthy` - Number of healthy workers
- `subgen_up` - Always 1 (Prometheus target up indicator)

---

## Technical Design

### File Structure

```
internal/server/
├── server.go           # Main HTTP server
├── middleware.go       # Request logging, panic recovery
├── health.go           # Health check endpoints
└── server_test.go      # Server tests

internal/metrics/
├── metrics.go          # Global metrics registry
└── metrics_test.go     # Metrics tests
```

---

### Global Metrics (metrics.go)

**File:** `internal/metrics/metrics.go`

```go
package metrics

import (
	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promauto"
)

// GlobalMetrics holds all application metrics
type GlobalMetrics struct {
	// HTTP metrics
	HTTPRequests        *prometheus.CounterVec
	HTTPDuration        *prometheus.HistogramVec
	HTTPRequestsInFlight prometheus.Gauge
	
	// Worker metrics
	WorkerCount   prometheus.Gauge
	WorkerHealthy prometheus.Gauge
	
	// Application up indicator
	Up prometheus.Gauge
}

// New creates global metrics
func New() *GlobalMetrics {
	return &GlobalMetrics{
		HTTPRequests: promauto.NewCounterVec(
			prometheus.CounterOpts{
				Name: "subgen_http_requests_total",
				Help: "Total number of HTTP requests",
			},
			[]string{"method", "endpoint", "status"},
		),
		
		HTTPDuration: promauto.NewHistogramVec(
			prometheus.HistogramOpts{
				Name:    "subgen_http_request_duration_seconds",
				Help:    "HTTP request duration",
				Buckets: []float64{0.001, 0.005, 0.01, 0.05, 0.1, 0.5, 1.0, 5.0},
			},
			[]string{"method", "endpoint"},
		),
		
		HTTPRequestsInFlight: promauto.NewGauge(prometheus.GaugeOpts{
			Name: "subgen_http_requests_in_flight",
			Help: "Current number of HTTP requests being processed",
		}),
		
		WorkerCount: promauto.NewGauge(prometheus.GaugeOpts{
			Name: "subgen_worker_count",
			Help: "Number of discovered workers",
		}),
		
		WorkerHealthy: promauto.NewGauge(prometheus.GaugeOpts{
			Name: "subgen_worker_healthy",
			Help: "Number of healthy workers",
		}),
		
		Up: promauto.NewGauge(prometheus.GaugeOpts{
			Name: "subgen_up",
			Help: "Always 1 - indicates service is up",
		}),
	}
}

// SetUp sets the up metric to 1
func (m *GlobalMetrics) SetUp() {
	m.Up.Set(1)
}
```

---

### HTTP Server with Middleware (server.go)

**File:** `internal/server/server.go`

```go
package server

import (
	"context"
	"fmt"
	"time"
	
	"github.com/gofiber/fiber/v2"
	"github.com/gofiber/fiber/v2/middleware/recover"
	"github.com/prometheus/client_golang/prometheus/promhttp"
	"github.com/sirupsen/logrus"
	
	"github.com/your-org/subgen/orchestrator/internal/config"
	"github.com/your-org/subgen/orchestrator/internal/metrics"
	"github.com/your-org/subgen/orchestrator/internal/queue"
	"github.com/your-org/subgen/orchestrator/internal/worker"
)

// Server wraps Fiber HTTP server with dependencies
type Server struct {
	webhookApp  *fiber.App
	metricsApp  *fiber.App
	
	config      *config.Config
	queue       *queue.Queue
	workerPool  *worker.Pool
	metrics     *metrics.GlobalMetrics
	log         *logrus.Logger
}

// NewServer creates HTTP server with all routes
func NewServer(
	cfg *config.Config,
	q *queue.Queue,
	pool *worker.Pool,
	m *metrics.GlobalMetrics,
	log *logrus.Logger,
) *Server {
	// Webhook server (port 9000)
	webhookApp := fiber.New(fiber.Config{
		DisableStartupMessage: true,
		ErrorHandler:          errorHandler(log),
	})
	
	// Metrics server (port 9090)
	metricsApp := fiber.New(fiber.Config{
		DisableStartupMessage: true,
	})
	
	s := &Server{
		webhookApp:  webhookApp,
		metricsApp:  metricsApp,
		config:      cfg,
		queue:       q,
		workerPool:  pool,
		metrics:     m,
		log:         log,
	}
	
	s.setupWebhookRoutes()
	s.setupMetricsRoutes()
	
	return s
}

// setupWebhookRoutes configures webhook HTTP endpoints
func (s *Server) setupWebhookRoutes() {
	// Middleware
	s.webhookApp.Use(s.requestLoggerMiddleware())
	s.webhookApp.Use(recover.New())
	
	// Health endpoints
	s.webhookApp.Get("/health", s.handleHealth)
	s.webhookApp.Get("/ready", s.handleReady)
	s.webhookApp.Get("/", s.handleRoot)
	
	// Webhook endpoints
	s.webhookApp.Post("/plex", s.handlePlex)
	s.webhookApp.Post("/jellyfin", s.handleJellyfin)
	s.webhookApp.Post("/emby", s.handleEmby)
	s.webhookApp.Post("/tautulli", s.handleTautulli)
	s.webhookApp.Post("/asr", s.handleASR)
	
	// Queue status endpoint
	s.webhookApp.Get("/queue", s.handleQueueStatus)
}

// setupMetricsRoutes configures Prometheus metrics endpoint
func (s *Server) setupMetricsRoutes() {
	// Prometheus metrics at /metrics
	s.metricsApp.Get("/metrics", func(c *fiber.Ctx) error {
		handler := promhttp.Handler()
		handler.ServeHTTP(c.Response(), c.Request())
		return nil
	})
}

// Start starts both HTTP servers
func (s *Server) Start() error {
	s.log.Info("Starting HTTP servers...")
	
	// Start webhook server (port 9000)
	go func() {
		addr := fmt.Sprintf(":%d", s.config.WebhookPort)
		s.log.Infof("Webhook server listening on %s", addr)
		if err := s.webhookApp.Listen(addr); err != nil {
			s.log.Fatalf("Failed to start webhook server: %v", err)
		}
	}()
	
	// Start metrics server (port 9090)
	go func() {
		addr := fmt.Sprintf(":%d", s.config.MetricsPort)
		s.log.Infof("Metrics server listening on %s", addr)
		if err := s.metricsApp.Listen(addr); err != nil {
			s.log.Fatalf("Failed to start metrics server: %v", err)
		}
	}()
	
	return nil
}

// Shutdown gracefully stops both servers
func (s *Server) Shutdown(ctx context.Context) error {
	s.log.Info("Shutting down HTTP servers...")
	
	if err := s.webhookApp.ShutdownWithContext(ctx); err != nil {
		return fmt.Errorf("webhook server shutdown error: %w", err)
	}
	
	if err := s.metricsApp.ShutdownWithContext(ctx); err != nil {
		return fmt.Errorf("metrics server shutdown error: %w", err)
	}
	
	return nil
}

// errorHandler custom error handler
func errorHandler(log *logrus.Logger) fiber.ErrorHandler {
	return func(c *fiber.Ctx, err error) error {
		code := fiber.StatusInternalServerError
		
		if e, ok := err.(*fiber.Error); ok {
			code = e.Code
		}
		
		log.WithFields(logrus.Fields{
			"method": c.Method(),
			"path":   c.Path(),
			"status": code,
			"error":  err.Error(),
		}).Error("HTTP error")
		
		return c.Status(code).JSON(fiber.Map{
			"error": err.Error(),
		})
	}
}
```

---

### Middleware (middleware.go)

**File:** `internal/server/middleware.go`

```go
package server

import (
	"time"
	
	"github.com/gofiber/fiber/v2"
	"github.com/sirupsen/logrus"
)

// requestLoggerMiddleware logs HTTP requests with structured logging
func (s *Server) requestLoggerMiddleware() fiber.Handler {
	return func(c *fiber.Ctx) error {
		start := time.Now()
		
		// Track in-flight requests
		s.metrics.HTTPRequestsInFlight.Inc()
		defer s.metrics.HTTPRequestsInFlight.Dec()
		
		// Process request
		err := c.Next()
		
		duration := time.Since(start)
		status := c.Response().StatusCode()
		
		// Log request
		s.log.WithFields(logrus.Fields{
			"method":       c.Method(),
			"path":         c.Path(),
			"status":       status,
			"duration_ms":  duration.Milliseconds(),
			"ip":           c.IP(),
			"user_agent":   c.Get("User-Agent"),
		}).Info("HTTP request")
		
		// Update metrics
		s.metrics.HTTPRequests.WithLabelValues(
			c.Method(),
			c.Path(),
			fmt.Sprintf("%d", status),
		).Inc()
		
		s.metrics.HTTPDuration.WithLabelValues(
			c.Method(),
			c.Path(),
		).Observe(duration.Seconds())
		
		return err
	}
}
```

---

### Health Checks (health.go)

**File:** `internal/server/health.go`

```go
package server

import (
	"github.com/gofiber/fiber/v2"
)

// handleHealth returns basic health status
func (s *Server) handleHealth(c *fiber.Ctx) error {
	return c.JSON(fiber.Map{
		"status": "healthy",
		"version": "v0.1.0", // TODO: Get from build ldflags
		"uptime": s.getUptime(),
	})
}

// handleReady returns readiness status (for K8s probes)
func (s *Server) handleReady(c *fiber.Ctx) error {
	// Check if workers are available
	workers, err := s.workerPool.GetWorkers()
	if err != nil || len(workers) == 0 {
		return c.Status(503).JSON(fiber.Map{
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
		return c.Status(503).JSON(fiber.Map{
			"status": "not_ready",
			"reason": "no healthy workers",
		})
	}
	
	return c.JSON(fiber.Map{
		"status":         "ready",
		"workers_total":  len(workers),
		"workers_healthy": healthyCount,
		"queue_size":     s.queue.Size(),
		"processing":     s.queue.ProcessingCount(),
	})
}

// handleRoot returns welcome message
func (s *Server) handleRoot(c *fiber.Ctx) error {
	return c.JSON(fiber.Map{
		"message": "Subgen Go Orchestrator",
		"version": "v0.1.0",
		"endpoints": fiber.Map{
			"health":  "/health",
			"ready":   "/ready",
			"metrics": "http://localhost:9090/metrics",
			"queue":   "/queue",
		},
	})
}

// handleQueueStatus returns queue statistics
func (s *Server) handleQueueStatus(c *fiber.Ctx) error {
	return c.JSON(fiber.Map{
		"queue_size":       s.queue.Size(),
		"processing":       s.queue.ProcessingCount(),
		"queued_tasks":     s.queue.GetQueuedTasks(),
		"processing_tasks": s.queue.GetProcessingTasks(),
		"is_idle":          s.queue.IsIdle(),
	})
}

// getUptime calculates uptime (implement with start time tracking)
func (s *Server) getUptime() string {
	// TODO: Track start time and calculate uptime
	return "unknown"
}
```

---

### Logging Setup (main.go integration)

**File:** `cmd/orchestrator/main.go`

```go
package main

import (
	"context"
	"os"
	"os/signal"
	"syscall"
	
	"github.com/sirupsen/logrus"
	"github.com/your-org/subgen/orchestrator/internal/config"
	"github.com/your-org/subgen/orchestrator/internal/metrics"
	"github.com/your-org/subgen/orchestrator/internal/server"
)

func main() {
	// Setup logging
	log := setupLogging()
	
	log.Info("Subgen Orchestrator starting...")
	
	// Load configuration
	cfg, err := config.Load()
	if err != nil {
		log.Fatalf("Failed to load configuration: %v", err)
	}
	
	// Initialize metrics
	m := metrics.New()
	m.SetUp() // Set subgen_up = 1
	
	// Initialize components
	queue := initQueue(cfg, m, log)
	workerPool := initWorkerPool(cfg, log)
	grpcClient := initGRPCClient(cfg, m, log)
	
	// Start worker pool
	if err := workerPool.Start(context.Background()); err != nil {
		log.Fatalf("Failed to start worker pool: %v", err)
	}
	
	// Create HTTP server
	srv := server.NewServer(cfg, queue, workerPool, m, log)
	if err := srv.Start(); err != nil {
		log.Fatalf("Failed to start server: %v", err)
	}
	
	log.Info("Orchestrator started successfully")
	
	// Wait for interrupt signal
	sigCh := make(chan os.Signal, 1)
	signal.Notify(sigCh, os.Interrupt, syscall.SIGTERM)
	<-sigCh
	
	log.Info("Shutting down gracefully...")
	
	// Shutdown with timeout
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	
	if err := srv.Shutdown(ctx); err != nil {
		log.Errorf("Server shutdown error: %v", err)
	}
	
	log.Info("Orchestrator stopped")
}

// setupLogging configures structured logging
func setupLogging() *logrus.Logger {
	log := logrus.New()
	
	// Set log level from env var
	level := os.Getenv("LOG_LEVEL")
	switch level {
	case "debug":
		log.SetLevel(logrus.DebugLevel)
	case "warn":
		log.SetLevel(logrus.WarnLevel)
	case "error":
		log.SetLevel(logrus.ErrorLevel)
	default:
		log.SetLevel(logrus.InfoLevel)
	}
	
	// Use JSON formatter in production
	env := os.Getenv("ENVIRONMENT")
	if env == "production" {
		log.SetFormatter(&logrus.JSONFormatter{
			TimestampFormat: "2006-01-02T15:04:05.000Z07:00",
			FieldMap: logrus.FieldMap{
				logrus.FieldKeyTime:  "timestamp",
				logrus.FieldKeyLevel: "level",
				logrus.FieldKeyMsg:   "message",
			},
		})
	} else {
		// Use text formatter in development
		log.SetFormatter(&logrus.TextFormatter{
			FullTimestamp:   true,
			TimestampFormat: "15:04:05",
		})
	}
	
	return log
}
```

---

## Test Cases (8+)

1. Health endpoint returns 200
2. Ready endpoint returns 200 when workers available
3. Ready endpoint returns 503 when no workers
4. Metrics endpoint serves Prometheus format
5. Request logger middleware logs requests
6. Request logger updates metrics
7. Panic recovery middleware catches panics
8. Queue status endpoint returns correct stats

---

## Implementation Steps

### Step 1: Create Metrics Package (1 hour)
Define global metrics registry

### Step 2: Create Server Package (2 hours)
Implement HTTP server with Fiber

### Step 3: Add Middleware (1 hour)
Request logging and panic recovery

### Step 4: Add Health Endpoints (1 hour)
Health and readiness checks

### Step 5: Integrate with Main (1 hour)
Setup logging, metrics, graceful shutdown

### Step 6: Write Tests (1 hour)
Test all endpoints and middleware

### Step 7: Manual Testing (30 min)
- Start orchestrator
- Check `/health` endpoint
- Check `/ready` endpoint
- Check `/metrics` endpoint
- Verify logs are structured

---

## Dependencies

**Requires:**
- STORY_01 (Project Setup) ✅
- STORY_04 (Queue with metrics) ✅
- STORY_06 (Worker Pool) ✅
- STORY_07 (gRPC Client with metrics) ✅

**Blocks:**
- None (final story)

---

## Definition of Done

- [ ] All 8+ tests passing
- [ ] `/metrics` endpoint serves Prometheus format
- [ ] `/health` endpoint returns JSON
- [ ] `/ready` endpoint checks workers
- [ ] Structured logging (JSON in production)
- [ ] Request logging middleware
- [ ] Panic recovery middleware
- [ ] Graceful shutdown
- [ ] Log levels configurable
- [ ] All metrics integrated
- [ ] Manual testing complete
- [ ] Code passes golangci-lint
- [ ] Work log created
- [ ] Coverage > 70%

---

## Notes

### Prometheus Metrics Best Practices

1. **Use appropriate metric types:**
   - Counter: monotonically increasing (requests, errors)
   - Gauge: can go up/down (queue size, worker count)
   - Histogram: distribution of values (latency, duration)

2. **Label cardinality:**
   - Keep labels low cardinality (< 1000 unique values)
   - Don't use user IDs or file paths as labels

3. **Naming conventions:**
   - `{namespace}_{subsystem}_{name}_{unit}`
   - Example: `subgen_queue_size` (no unit for count)
   - Example: `subgen_http_request_duration_seconds` (with unit)

### Kubernetes Probes

**Liveness Probe:** Use `/health`
- Checks if application is alive
- Restart pod if fails

**Readiness Probe:** Use `/ready`
- Checks if application can serve traffic
- Remove from load balancer if fails

**Example K8s deployment:**
```yaml
livenessProbe:
  httpGet:
    path: /health
    port: 9000
  initialDelaySeconds: 30
  periodSeconds: 10

readinessProbe:
  httpGet:
    path: /ready
    port: 9000
  initialDelaySeconds: 5
  periodSeconds: 5
```

### Grafana Dashboard (Optional)

Example PromQL queries:

```promql
# Queue size over time
subgen_queue_size

# HTTP request rate
rate(subgen_http_requests_total[5m])

# HTTP latency P95
histogram_quantile(0.95, rate(subgen_http_request_duration_seconds_bucket[5m]))

# gRPC error rate
rate(subgen_grpc_errors_total[5m])

# Worker health
subgen_worker_healthy / subgen_worker_count
```

---

**Owner:** TBD  
**Created:** 2026-02-15  
**Last Updated:** 2026-02-15
