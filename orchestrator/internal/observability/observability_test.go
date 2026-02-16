package observability

import (
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/gofiber/fiber/v2"
	"github.com/prometheus/client_golang/prometheus"
	"github.com/sirupsen/logrus"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// Mock worker pool for testing
type mockWorkerPool struct {
	workers        int
	healthyWorkers int
	hasError       bool
}

func (m *mockWorkerPool) GetWorkers() ([]Worker, error) {
	if m.hasError {
		return nil, assert.AnError
	}
	workers := make([]Worker, m.workers)
	for i := 0; i < m.healthyWorkers && i < m.workers; i++ {
		workers[i].Healthy = true
	}
	return workers, nil
}

// Mock queue for testing
type mockQueue struct {
	size       int
	processing int
}

func (m *mockQueue) Size() int            { return m.size }
func (m *mockQueue) ProcessingCount() int { return m.processing }
func (m *mockQueue) IsIdle() bool         { return m.size == 0 && m.processing == 0 }

func TestHealthEndpoint_Success(t *testing.T) {
	// Setup
	app := fiber.New()
	log := logrus.New()
	log.SetLevel(logrus.FatalLevel)

	metrics := NewMetrics()
	pool := &mockWorkerPool{workers: 3, healthyWorkers: 3}
	queue := &mockQueue{size: 5, processing: 2}

	startTime := time.Now().Add(-1 * time.Hour)

	// Register health endpoint
	RegisterHealthEndpoints(app, metrics, pool, queue, startTime, log)

	// Test
	req := httptest.NewRequest(http.MethodGet, "/health", nil)
	resp, err := app.Test(req)

	// Verify
	require.NoError(t, err)
	assert.Equal(t, http.StatusOK, resp.StatusCode)
}

func TestReadyEndpoint_Ready(t *testing.T) {
	// Setup
	app := fiber.New()
	log := logrus.New()
	log.SetLevel(logrus.FatalLevel)

	metrics := NewMetrics()
	pool := &mockWorkerPool{workers: 3, healthyWorkers: 2}
	queue := &mockQueue{size: 5, processing: 2}
	startTime := time.Now()

	// Register endpoints
	RegisterHealthEndpoints(app, metrics, pool, queue, startTime, log)

	// Test
	req := httptest.NewRequest(http.MethodGet, "/ready", nil)
	resp, err := app.Test(req)

	// Verify
	require.NoError(t, err)
	assert.Equal(t, http.StatusOK, resp.StatusCode)
}

func TestReadyEndpoint_NotReady_NoWorkers(t *testing.T) {
	// Setup
	app := fiber.New()
	log := logrus.New()
	log.SetLevel(logrus.FatalLevel)

	metrics := NewMetrics()
	pool := &mockWorkerPool{workers: 0, healthyWorkers: 0}
	queue := &mockQueue{size: 5, processing: 2}
	startTime := time.Now()

	// Register endpoints
	RegisterHealthEndpoints(app, metrics, pool, queue, startTime, log)

	// Test
	req := httptest.NewRequest(http.MethodGet, "/ready", nil)
	resp, err := app.Test(req)

	// Verify
	require.NoError(t, err)
	assert.Equal(t, http.StatusServiceUnavailable, resp.StatusCode)
}

func TestReadyEndpoint_NotReady_NoHealthyWorkers(t *testing.T) {
	// Setup
	app := fiber.New()
	log := logrus.New()
	log.SetLevel(logrus.FatalLevel)

	metrics := NewMetrics()
	pool := &mockWorkerPool{workers: 3, healthyWorkers: 0}
	queue := &mockQueue{size: 5, processing: 2}
	startTime := time.Now()

	// Register endpoints
	RegisterHealthEndpoints(app, metrics, pool, queue, startTime, log)

	// Test
	req := httptest.NewRequest(http.MethodGet, "/ready", nil)
	resp, err := app.Test(req)

	// Verify
	require.NoError(t, err)
	assert.Equal(t, http.StatusServiceUnavailable, resp.StatusCode)
}

func TestRequestLoggerMiddleware(t *testing.T) {
	// Setup
	app := fiber.New()
	log := logrus.New()
	log.SetLevel(logrus.FatalLevel)

	metrics := NewMetricsWithRegistry(prometheus.NewRegistry())

	// Add middleware
	app.Use(RequestLoggerMiddleware(metrics, log))

	// Add test route
	app.Get("/test", func(c *fiber.Ctx) error {
		return c.SendString("OK")
	})

	// Test
	req := httptest.NewRequest(http.MethodGet, "/test", nil)
	resp, err := app.Test(req)

	// Verify
	require.NoError(t, err)
	assert.Equal(t, http.StatusOK, resp.StatusCode)

	// Metrics were updated (validated by no panic)
}

func TestPanicRecoveryMiddleware(t *testing.T) {
	// Setup
	app := fiber.New()
	log := logrus.New()
	log.SetLevel(logrus.FatalLevel)

	// Add panic recovery
	app.Use(PanicRecoveryMiddleware(log))

	// Add route that panics
	app.Get("/panic", func(c *fiber.Ctx) error {
		panic("test panic")
	})

	// Test
	req := httptest.NewRequest(http.MethodGet, "/panic", nil)
	resp, err := app.Test(req)

	// Verify - should recover and return 500
	require.NoError(t, err)
	assert.Equal(t, http.StatusInternalServerError, resp.StatusCode)
}

func TestMetrics_Initialization(t *testing.T) {
	// Test default metrics
	metrics := NewMetrics()
	assert.NotNil(t, metrics.HTTPRequests)
	assert.NotNil(t, metrics.HTTPDuration)
	assert.NotNil(t, metrics.HTTPRequestsInFlight)
	assert.NotNil(t, metrics.WorkerCount)
	assert.NotNil(t, metrics.WorkerHealthy)
	assert.NotNil(t, metrics.Up)

	// Set up metric
	metrics.SetUp()
	// Note: Would need to query Prometheus to verify, but at least it doesn't panic
}

func TestMetrics_WithCustomRegistry(t *testing.T) {
	// Test with custom registry
	registry := prometheus.NewRegistry()
	metrics := NewMetricsWithRegistry(registry)

	assert.NotNil(t, metrics.HTTPRequests)
	assert.NotNil(t, metrics.registry)

	// Should be able to create multiple without conflicts
	metrics2 := NewMetricsWithRegistry(prometheus.NewRegistry())
	assert.NotNil(t, metrics2.HTTPRequests)
}
