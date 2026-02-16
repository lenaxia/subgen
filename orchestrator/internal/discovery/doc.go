// Package discovery provides worker discovery mechanisms for the orchestrator.
//
// The discovery package implements pluggable worker discovery strategies that
// allow the orchestrator to scale from single-pod to multi-worker deployments
// without code changes.
//
// # Worker Discovery Interface
//
// The core abstraction is the WorkerDiscovery interface, which defines how
// workers are found and monitored:
//
//	type WorkerDiscovery interface {
//	    GetWorkers(ctx context.Context) ([]Worker, error)
//	    Watch(ctx context.Context) (<-chan WorkerEvent, error)
//	}
//
// # Implementations
//
// Phase 1 - Localhost Discovery:
// - Returns single worker at static address (localhost:50051)
// - No dynamic discovery (Watch returns closed channel)
// - Direct gRPC health check
//
// Phase 2 - Kubernetes Discovery:
// - Discovers workers via K8s Endpoints API
// - Dynamic discovery (Watch monitors endpoint changes)
// - Service name: subgen-worker
// - Port: 50051
//
// # Worker Pool
//
// The Pool type manages a collection of workers with:
// - Load balancing strategies (round-robin, least-loaded)
// - Automatic health checking (30s interval)
// - Worker event handling (add/remove/update)
// - Thread-safe operations
//
// # Factory Pattern
//
// NewDiscovery creates the appropriate discovery implementation based on
// configuration:
//
//	discovery, err := NewDiscovery(config, log)
//
// The discovery mode is configured via WORKER_DISCOVERY environment variable:
// - "localhost" - Single worker (Phase 1)
// - "kubernetes" - K8s service discovery (Phase 2)
//
// # Metrics
//
// Prometheus metrics are provided for:
// - Worker count (by health status)
// - Discovery errors
// - Worker selection count (by strategy)
// - Health check duration
//
// # Example Usage
//
//	// Create discovery
//	discovery, err := NewDiscovery(config, log)
//	if err != nil {
//	    log.Fatal(err)
//	}
//
//	// Create worker pool
//	pool := NewPool(discovery, RoundRobin, log)
//
//	// Start pool (discovery + health checks)
//	if err := pool.Start(ctx); err != nil {
//	    log.Fatal(err)
//	}
//
//	// Select worker for job
//	worker, err := pool.SelectWorker()
//	if err != nil {
//	    log.Error("No workers available")
//	}
package discovery
