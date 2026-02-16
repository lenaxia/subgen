# STORY_06: Worker Discovery & Pool Management

**Status:** Not Started  
**Effort:** 8-10 hours  
**Epic:** EPIC_01 (Go Orchestrator Core)  
**Created:** 2026-02-15

---

## User Story

**As a** developer  
**I want** pluggable worker discovery (localhost vs Kubernetes)  
**So that** the orchestrator can scale from single-pod to multi-worker deployments without code changes

---

## Acceptance Criteria

- [ ] WorkerDiscovery interface with 2 implementations
- [ ] LocalhostDiscovery for Phase 1 (single worker)
- [ ] KubernetesDiscovery for Phase 2 (worker pool)
- [ ] Factory pattern for configuration-driven selection
- [ ] Worker health checking every 30s
- [ ] WorkerPool for load balancing (round-robin and least-loaded)
- [ ] Automatic worker removal on failed health check
- [ ] Watch for worker changes (K8s only)
- [ ] 12+ test cases
- [ ] Integration with queue from STORY_04
- [ ] Prometheus metrics for worker count and health
- [ ] Work log created

---

## Integration Points

### Scaling Strategy Document (03_SCALING_STRATEGY.md)

**Location:** `/home/mikekao/personal/subgen/docs/DESIGN/03_SCALING_STRATEGY.md:237-316`

**Phase 1 - Localhost Discovery:**
```go
// internal/worker/discovery_localhost.go

type LocalhostDiscovery struct {
    address string
    client  pb.TranscriptionServiceClient
}

func (d *LocalhostDiscovery) GetWorkers(ctx context.Context) ([]Worker, error) {
    // Health check localhost worker
    conn, err := grpc.DialContext(ctx, d.address, grpc.WithInsecure())
    if err != nil {
        return nil, fmt.Errorf("failed to connect to localhost worker: %w", err)
    }
    defer conn.Close()
    
    client := pb.NewTranscriptionServiceClient(conn)
    resp, err := client.HealthCheck(ctx, &pb.HealthCheckRequest{})
    if err != nil {
        return nil, fmt.Errorf("health check failed: %w", err)
    }
    
    worker := Worker{
        ID:       "worker-local",
        Address:  d.address,
        Healthy:  resp.Status == pb.HealthCheckResponse_HEALTHY,
        Active:   resp.JobsActive,
        LastSeen: time.Now(),
    }
    
    return []Worker{worker}, nil
}
```

**Key Behaviors:**
- Single worker at localhost:50051
- Direct gRPC health check
- No dynamic discovery (static address)
- Returns single-element worker list

**Phase 2 - Kubernetes Discovery:**

Uses Kubernetes Endpoints API to discover worker pods:

```go
// Discovers workers via K8s Endpoints API
endpoints, err := d.client.CoreV1().Endpoints(d.namespace).Get(ctx, d.service, metav1.GetOptions{})

for _, subset := range endpoints.Subsets {
    for _, addr := range subset.Addresses {
        workerAddr := fmt.Sprintf("%s:%d", addr.IP, d.port)
        // Health check each worker
        workers = append(workers, Worker{...})
    }
}
```

**Key Details:**
- Service name: `subgen-worker`
- Port: `50051`
- Uses in-cluster config (`rest.InClusterConfig()`)
- Watches endpoint changes for dynamic discovery

---

## Technical Design

### File Structure

```
internal/worker/
├── discovery.go            # Interface and common types
├── discovery_localhost.go  # Phase 1 implementation
├── discovery_kubernetes.go # Phase 2 implementation
├── discovery_factory.go    # Configuration-driven factory
├── pool.go                 # Worker pool with load balancing
├── pool_test.go            # Pool tests
├── discovery_test.go       # Discovery tests
└── metrics.go              # Prometheus metrics
```

---

### Core Interface (discovery.go)

**File:** `internal/worker/discovery.go`

```go
package worker

import (
	"context"
	"time"
)

// WorkerDiscovery finds available workers
type WorkerDiscovery interface {
	// GetWorkers returns all healthy workers
	GetWorkers(ctx context.Context) ([]Worker, error)
	
	// Watch for worker changes (add/remove)
	// Returns channel that emits WorkerEvent on changes
	Watch(ctx context.Context) (<-chan WorkerEvent, error)
}

// Worker represents a transcription worker
type Worker struct {
	ID       string    // Unique identifier
	Address  string    // gRPC address (host:port)
	Healthy  bool      // Health check status
	Active   int32     // Active jobs
	LastSeen time.Time // Last health check
}

// WorkerEvent represents a change in worker availability
type WorkerEvent struct {
	Type   EventType
	Worker Worker
}

type EventType string

const (
	EventTypeAdded   EventType = "added"
	EventTypeRemoved EventType = "removed"
	EventTypeUpdated EventType = "updated"
)
```

---

### Localhost Discovery (discovery_localhost.go)

**File:** `internal/worker/discovery_localhost.go`

```go
package worker

import (
	"context"
	"fmt"
	"time"
	
	"github.com/sirupsen/logrus"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"
	
	pb "github.com/your-org/subgen/orchestrator/pkg/api/v1"
)

// LocalhostDiscovery implements WorkerDiscovery for single local worker
type LocalhostDiscovery struct {
	address string
	log     *logrus.Logger
}

// NewLocalhostDiscovery creates a localhost worker discovery
func NewLocalhostDiscovery(address string, log *logrus.Logger) *LocalhostDiscovery {
	return &LocalhostDiscovery{
		address: address,
		log:     log,
	}
}

// GetWorkers returns the single localhost worker (if healthy)
func (d *LocalhostDiscovery) GetWorkers(ctx context.Context) ([]Worker, error) {
	conn, err := grpc.DialContext(ctx, d.address, 
		grpc.WithTransportCredentials(insecure.NewCredentials()),
		grpc.WithBlock(),
	)
	if err != nil {
		return nil, fmt.Errorf("failed to connect to localhost worker: %w", err)
	}
	defer conn.Close()
	
	client := pb.NewTranscriptionServiceClient(conn)
	resp, err := client.HealthCheck(ctx, &pb.HealthCheckRequest{})
	if err != nil {
		return nil, fmt.Errorf("health check failed: %w", err)
	}
	
	worker := Worker{
		ID:       "worker-local",
		Address:  d.address,
		Healthy:  resp.Status == pb.HealthCheckResponse_HEALTHY,
		Active:   resp.JobsActive,
		LastSeen: time.Now(),
	}
	
	d.log.WithFields(logrus.Fields{
		"address": worker.Address,
		"healthy": worker.Healthy,
		"active":  worker.Active,
	}).Debug("Localhost worker discovered")
	
	return []Worker{worker}, nil
}

// Watch returns empty channel (no dynamic discovery for localhost)
func (d *LocalhostDiscovery) Watch(ctx context.Context) (<-chan WorkerEvent, error) {
	ch := make(chan WorkerEvent)
	close(ch) // No events for static localhost
	return ch, nil
}
```

---

### Kubernetes Discovery (discovery_kubernetes.go)

**File:** `internal/worker/discovery_kubernetes.go`

```go
package worker

import (
	"context"
	"fmt"
	"time"
	
	"github.com/sirupsen/logrus"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"
	
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/rest"
	
	pb "github.com/your-org/subgen/orchestrator/pkg/api/v1"
)

// KubernetesDiscovery implements WorkerDiscovery for K8s worker pods
type KubernetesDiscovery struct {
	client    *kubernetes.Clientset
	namespace string
	service   string
	port      int32
	log       *logrus.Logger
}

// NewKubernetesDiscovery creates K8s worker discovery
func NewKubernetesDiscovery(namespace, service string, port int32, log *logrus.Logger) (*KubernetesDiscovery, error) {
	// In-cluster config
	config, err := rest.InClusterConfig()
	if err != nil {
		return nil, fmt.Errorf("failed to get in-cluster config: %w", err)
	}
	
	client, err := kubernetes.NewForConfig(config)
	if err != nil {
		return nil, fmt.Errorf("failed to create k8s client: %w", err)
	}
	
	return &KubernetesDiscovery{
		client:    client,
		namespace: namespace,
		service:   service,
		port:      port,
		log:       log,
	}, nil
}

// GetWorkers discovers all worker pods via K8s Endpoints API
func (d *KubernetesDiscovery) GetWorkers(ctx context.Context) ([]Worker, error) {
	endpoints, err := d.client.CoreV1().Endpoints(d.namespace).Get(ctx, d.service, metav1.GetOptions{})
	if err != nil {
		return nil, fmt.Errorf("failed to get endpoints: %w", err)
	}
	
	var workers []Worker
	
	for _, subset := range endpoints.Subsets {
		for _, addr := range subset.Addresses {
			workerAddr := fmt.Sprintf("%s:%d", addr.IP, d.port)
			workerID := fmt.Sprintf("worker-%s", addr.TargetRef.Name)
			
			// Health check each worker
			healthy, active := d.checkWorkerHealth(ctx, workerAddr)
			
			workers = append(workers, Worker{
				ID:       workerID,
				Address:  workerAddr,
				Healthy:  healthy,
				Active:   active,
				LastSeen: time.Now(),
			})
		}
	}
	
	if len(workers) == 0 {
		return nil, fmt.Errorf("no healthy workers found")
	}
	
	d.log.WithField("count", len(workers)).Info("Discovered K8s workers")
	
	return workers, nil
}

// checkWorkerHealth performs gRPC health check
func (d *KubernetesDiscovery) checkWorkerHealth(ctx context.Context, address string) (bool, int32) {
	ctx, cancel := context.WithTimeout(ctx, 5*time.Second)
	defer cancel()
	
	conn, err := grpc.DialContext(ctx, address, 
		grpc.WithTransportCredentials(insecure.NewCredentials()),
		grpc.WithBlock(),
	)
	if err != nil {
		return false, 0
	}
	defer conn.Close()
	
	client := pb.NewTranscriptionServiceClient(conn)
	resp, err := client.HealthCheck(ctx, &pb.HealthCheckRequest{})
	if err != nil {
		return false, 0
	}
	
	return resp.Status == pb.HealthCheckResponse_HEALTHY, resp.JobsActive
}

// Watch monitors K8s endpoints for worker changes
func (d *KubernetesDiscovery) Watch(ctx context.Context) (<-chan WorkerEvent, error) {
	watcher, err := d.client.CoreV1().Endpoints(d.namespace).Watch(ctx, metav1.ListOptions{
		FieldSelector: fmt.Sprintf("metadata.name=%s", d.service),
	})
	if err != nil {
		return nil, fmt.Errorf("failed to watch endpoints: %w", err)
	}
	
	ch := make(chan WorkerEvent)
	
	go func() {
		defer close(ch)
		defer watcher.Stop()
		
		for event := range watcher.ResultChan() {
			endpoints := event.Object.(*corev1.Endpoints)
			
			for _, subset := range endpoints.Subsets {
				for _, addr := range subset.Addresses {
					ch <- WorkerEvent{
						Type: EventTypeUpdated,
						Worker: Worker{
							ID:      fmt.Sprintf("worker-%s", addr.TargetRef.Name),
							Address: fmt.Sprintf("%s:%d", addr.IP, d.port),
							Healthy: true,
						},
					}
				}
			}
		}
	}()
	
	return ch, nil
}
```

---

### Factory (discovery_factory.go)

**File:** `internal/worker/discovery_factory.go`

```go
package worker

import (
	"fmt"
	
	"github.com/sirupsen/logrus"
	"github.com/your-org/subgen/orchestrator/internal/config"
)

// NewDiscovery creates WorkerDiscovery based on configuration
func NewDiscovery(cfg *config.Config, log *logrus.Logger) (WorkerDiscovery, error) {
	switch cfg.Worker.Discovery {
	case "localhost":
		return NewLocalhostDiscovery(cfg.Worker.Address, log), nil
	
	case "kubernetes":
		return NewKubernetesDiscovery(
			cfg.Worker.Namespace,
			cfg.Worker.ServiceName,
			int32(cfg.Worker.Port),
			log,
		)
	
	default:
		return nil, fmt.Errorf("unknown worker discovery: %s", cfg.Worker.Discovery)
	}
}
```

---

### Worker Pool (pool.go)

**File:** `internal/worker/pool.go`

```go
package worker

import (
	"context"
	"errors"
	"sync"
	"time"
	
	"github.com/sirupsen/logrus"
)

var (
	ErrNoWorkersAvailable = errors.New("no workers available")
	ErrNoHealthyWorkers   = errors.New("no healthy workers")
)

// LoadBalanceStrategy determines how to select workers
type LoadBalanceStrategy string

const (
	RoundRobin  LoadBalanceStrategy = "round_robin"
	LeastLoaded LoadBalanceStrategy = "least_loaded"
)

// Pool manages a pool of workers with health checking
type Pool struct {
	discovery WorkerDiscovery
	strategy  LoadBalanceStrategy
	log       *logrus.Logger
	
	mu      sync.RWMutex
	workers []Worker
	next    int // For round-robin
	
	// Health check interval
	healthCheckInterval time.Duration
}

// NewPool creates a worker pool
func NewPool(discovery WorkerDiscovery, strategy LoadBalanceStrategy, log *logrus.Logger) *Pool {
	return &Pool{
		discovery:           discovery,
		strategy:            strategy,
		log:                 log,
		workers:             []Worker{},
		healthCheckInterval: 30 * time.Second,
	}
}

// Start begins worker discovery and health checking
func (p *Pool) Start(ctx context.Context) error {
	// Initial discovery
	if err := p.Refresh(ctx); err != nil {
		return err
	}
	
	// Start health check loop
	go p.healthCheckLoop(ctx)
	
	// Watch for changes (if supported)
	eventCh, err := p.discovery.Watch(ctx)
	if err != nil {
		p.log.WithError(err).Warn("Worker watch not supported")
	} else {
		go p.watchLoop(ctx, eventCh)
	}
	
	return nil
}

// SelectWorker chooses a worker based on load balancing strategy
func (p *Pool) SelectWorker() (*Worker, error) {
	p.mu.Lock()
	defer p.mu.Unlock()
	
	healthy := p.filterHealthy()
	if len(healthy) == 0 {
		return nil, ErrNoHealthyWorkers
	}
	
	var worker *Worker
	
	switch p.strategy {
	case RoundRobin:
		worker = &healthy[p.next%len(healthy)]
		p.next++
	
	case LeastLoaded:
		worker = p.findLeastLoaded(healthy)
	
	default:
		return nil, fmt.Errorf("unknown strategy: %s", p.strategy)
	}
	
	return worker, nil
}

// Refresh re-discovers all workers
func (p *Pool) Refresh(ctx context.Context) error {
	workers, err := p.discovery.GetWorkers(ctx)
	if err != nil {
		return err
	}
	
	p.mu.Lock()
	p.workers = workers
	p.mu.Unlock()
	
	p.log.WithField("count", len(workers)).Info("Workers refreshed")
	
	return nil
}

// healthCheckLoop periodically refreshes worker list
func (p *Pool) healthCheckLoop(ctx context.Context) {
	ticker := time.NewTicker(p.healthCheckInterval)
	defer ticker.Stop()
	
	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			if err := p.Refresh(ctx); err != nil {
				p.log.WithError(err).Error("Failed to refresh workers")
			}
		}
	}
}

// watchLoop handles worker change events
func (p *Pool) watchLoop(ctx context.Context, eventCh <-chan WorkerEvent) {
	for {
		select {
		case <-ctx.Done():
			return
		case event, ok := <-eventCh:
			if !ok {
				return
			}
			p.handleWorkerEvent(event)
		}
	}
}

// handleWorkerEvent processes worker add/remove/update events
func (p *Pool) handleWorkerEvent(event WorkerEvent) {
	p.mu.Lock()
	defer p.mu.Unlock()
	
	switch event.Type {
	case EventTypeAdded:
		p.workers = append(p.workers, event.Worker)
		p.log.WithField("worker_id", event.Worker.ID).Info("Worker added")
	
	case EventTypeRemoved:
		p.workers = removeWorker(p.workers, event.Worker.ID)
		p.log.WithField("worker_id", event.Worker.ID).Info("Worker removed")
	
	case EventTypeUpdated:
		p.workers = updateWorker(p.workers, event.Worker)
		p.log.WithField("worker_id", event.Worker.ID).Debug("Worker updated")
	}
}

// filterHealthy returns only healthy workers
func (p *Pool) filterHealthy() []Worker {
	var healthy []Worker
	for _, w := range p.workers {
		if w.Healthy {
			healthy = append(healthy, w)
		}
	}
	return healthy
}

// findLeastLoaded returns worker with fewest active jobs
func (p *Pool) findLeastLoaded(workers []Worker) *Worker {
	if len(workers) == 0 {
		return nil
	}
	
	leastLoaded := &workers[0]
	for i := range workers {
		if workers[i].Active < leastLoaded.Active {
			leastLoaded = &workers[i]
		}
	}
	return leastLoaded
}

// Helper functions for worker list manipulation
func removeWorker(workers []Worker, id string) []Worker {
	for i, w := range workers {
		if w.ID == id {
			return append(workers[:i], workers[i+1:]...)
		}
	}
	return workers
}

func updateWorker(workers []Worker, updated Worker) []Worker {
	for i, w := range workers {
		if w.ID == updated.ID {
			workers[i] = updated
			return workers
		}
	}
	return append(workers, updated)
}
```

---

## Test Cases (12+)

1. LocalhostDiscovery returns single worker
2. LocalhostDiscovery health check failure
3. KubernetesDiscovery returns multiple workers
4. KubernetesDiscovery handles no endpoints
5. Factory selects correct discovery type
6. Pool round-robin selection
7. Pool least-loaded selection
8. Pool health check removes unhealthy workers
9. Pool handles worker events (add/remove/update)
10. Pool concurrent SelectWorker calls (thread safety)
11. Worker watch detects new workers
12. Metrics updated correctly

---

## Implementation Steps

### Step 1: Implement Core Types (1 hour)
Create discovery.go with interface and Worker struct

### Step 2: Implement Localhost Discovery (1 hour)
Simple single-worker discovery with gRPC health check

### Step 3: Implement Kubernetes Discovery (3 hours)
- Set up K8s client
- Query Endpoints API
- Health check each worker
- Implement Watch

### Step 4: Implement Worker Pool (2 hours)
- Round-robin and least-loaded strategies
- Health check loop
- Event handling

### Step 5: Write Tests (2 hours)
Mock K8s API and gRPC clients, test all scenarios

### Step 6: Integration (1 hour)
Connect pool to queue for task distribution

---

## Dependencies

**Requires:**
- STORY_01 (Project Setup) ✅
- STORY_02 (Configuration) ✅
- STORY_04 (Queue) ✅

**Blocks:**
- STORY_07 (gRPC Client) - needs worker pool

---

## Definition of Done

- [ ] All 12+ tests passing
- [ ] Localhost discovery works
- [ ] Kubernetes discovery works
- [ ] Factory pattern implemented
- [ ] Worker pool with load balancing
- [ ] Health checking every 30s
- [ ] Watch for worker changes
- [ ] Thread-safe operations
- [ ] Prometheus metrics
- [ ] Manual testing with real K8s cluster
- [ ] Code passes golangci-lint
- [ ] Work log created
- [ ] Coverage > 80%

---

**Owner:** TBD  
**Created:** 2026-02-15  
**Last Updated:** 2026-02-15
