// Package observability provides metrics, logging, and health checks for the orchestrator.
//
// Features:
// - Prometheus metrics for HTTP requests, workers, and system health
// - Structured logging middleware with logrus
// - Panic recovery middleware for graceful error handling
// - Health check endpoints (/health for liveness, /ready for readiness)
// - Queue status endpoint (/queue)
//
// Metrics Exposed:
// - subgen_http_requests_total - Total HTTP requests by method, endpoint, status
// - subgen_http_request_duration_seconds - Request latency histogram
// - subgen_http_requests_in_flight - Current active requests
// - subgen_worker_count - Number of discovered workers
// - subgen_worker_healthy - Number of healthy workers
// - subgen_up - Always 1 (service availability indicator)
//
// Usage:
//
//	metrics := observability.NewMetrics()
//	metrics.SetUp()
//
//	app := fiber.New()
//	app.Use(observability.RequestLoggerMiddleware(metrics, log))
//	app.Use(observability.PanicRecoveryMiddleware(log))
//
//	observability.RegisterHealthEndpoints(app, metrics, pool, queue, startTime, log)
package observability
