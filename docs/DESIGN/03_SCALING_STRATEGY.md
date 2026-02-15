# Scaling Strategy: Phase 1 → Phase 2

**Document Version:** 1.0  
**Last Updated:** 2026-02-15  
**Status:** Draft  
**Related Documents:**
- [00_HYBRID_ARCHITECTURE.md](./00_HYBRID_ARCHITECTURE.md)
- [01_GRPC_PROTOCOL.md](./01_GRPC_PROTOCOL.md)
- [04_K8S_DEPLOYMENT.md](./04_K8S_DEPLOYMENT.md)

---

## Table of Contents

1. [Overview](#overview)
2. [Phase 1: Single Pod Deployment](#phase-1-single-pod-deployment)
3. [Phase 2: Scaled Worker Pool](#phase-2-scaled-worker-pool)
4. [Worker Discovery Abstraction](#worker-discovery-abstraction)
5. [Load Balancing Strategies](#load-balancing-strategies)
6. [Migration Path](#migration-path)
7. [Performance Characteristics](#performance-characteristics)

---

## Overview

### Design Philosophy

**Build for scale from Day 1, deploy simple initially.**

The architecture supports both single-pod and multi-worker deployments **without code changes**. Scaling is controlled purely through configuration.

### Why This Approach?

| Requirement | Phase 1 Solution | Phase 2 Solution |
|-------------|------------------|------------------|
| Low load (2 episodes/day) | Single pod sufficient | N/A |
| Resource efficiency | Minimal overhead | Scale workers independently |
| Future growth | Built-in scaling | Add workers via config |
| Complexity | Simple deployment | Same code, different config |
| Development speed | Ship faster | No rewrite needed |

**Key Insight:** Most users start small, some grow large. Build the abstraction upfront to avoid costly rewrites.

---

## Phase 1: Single Pod Deployment

### Architecture

```
┌─────────────────────────────────────────────────────────────┐
│  Pod: subgen-0                                              │
│  Namespace: media                                           │
│  Deployment: subgen (1 replica)                             │
├─────────────────────────────────────────────────────────────┤
│                                                              │
│  ┌───────────────────────────────────────────────────┐     │
│  │  Container: orchestrator                           │     │
│  │  Image: ghcr.io/user/subgen-orchestrator:latest   │     │
│  │                                                     │     │
│  │  • HTTP server (webhooks) - Port 9000             │     │
│  │  • Prometheus metrics - Port 9090                  │     │
│  │  • gRPC client to localhost:50051                 │     │
│  │  • In-memory queue (max 1000 tasks)              │     │
│  │                                                     │     │
│  │  Resources:                                        │     │
│  │    Request: 64Mi RAM, 0.1 CPU                     │     │
│  │    Limit:   256Mi RAM, 0.5 CPU                    │     │
│  └───────────────────────────────────────────────────┘     │
│                    ↓ gRPC localhost:50051                   │
│  ┌───────────────────────────────────────────────────┐     │
│  │  Container: worker                                 │     │
│  │  Image: ghcr.io/user/subgen-worker:latest         │     │
│  │                                                     │     │
│  │  • gRPC server - Port 50051                       │     │
│  │  • Whisper transcription engine                   │     │
│  │  • Model cache: /models (PVC)                     │     │
│  │                                                     │     │
│  │  Resources:                                        │     │
│  │    Request: 2Gi RAM, 0.5 CPU                      │     │
│  │    Limit:   4Gi RAM, 2.0 CPU                      │     │
│  └───────────────────────────────────────────────────┘     │
│                                                              │
└─────────────────────────────────────────────────────────────┘
         ↓ NFS Mount: /media (shared filesystem)
┌─────────────────────────────────────────────────────────────┐
│  NFS Server: 192.168.1.10                                   │
│  Path: /mnt/pool/media                                      │
│  Access: RW (read-write)                                    │
└─────────────────────────────────────────────────────────────┘
```

### Configuration

**Environment Variables (Orchestrator):**

```yaml
env:
  WORKER_DISCOVERY: "localhost"               # ← Phase 1 mode
  PYTHON_WORKER_ADDRESS: "localhost:50051"
  QUEUE_MAX_SIZE: "1000"
  WEBHOOK_PORT: "9000"
  METRICS_PORT: "9090"
```

### Characteristics

| Metric | Value | Notes |
|--------|-------|-------|
| **Throughput** | 1-3 transcriptions/hour | Depends on file length |
| **Concurrency** | 1 (sequential processing) | Single worker |
| **Memory** | 2-4GB total | Mostly worker |
| **CPU** | 0.5-2.5 CPU total | Mostly worker during transcription |
| **Startup Time** | 30-60s | Model download on first run |
| **Max Queue Size** | 1000 tasks | Bounded to prevent OOM |

### When to Use Phase 1

- ✅ Personal Plex/Jellyfin server (< 10 episodes/day)
- ✅ Testing/development environment
- ✅ Resource-constrained environment (single node)
- ✅ Simple deployment requirements

### Limitations

- ❌ No parallel processing (1 transcription at a time)
- ❌ Single point of failure (pod restart = downtime)
- ❌ Cannot handle burst traffic (queue fills up)

---

## Phase 2: Scaled Worker Pool

### Architecture

```
┌─────────────────────────────────────────────────────────────┐
│  Pod: subgen-orchestrator-7d8f9c-xyz                        │
│  Deployment: subgen-orchestrator (1 replica)                │
├─────────────────────────────────────────────────────────────┤
│  Container: orchestrator                                     │
│  • Discovers workers via Kubernetes Service API            │
│  • Load balances requests (round-robin or least-loaded)    │
│  • Health checks all workers every 30s                     │
│  • Requeues tasks if worker fails                          │
└─────────────────────────────────────────────────────────────┘
                     ↓ gRPC via Service
┌─────────────────────────────────────────────────────────────┐
│  Service: subgen-worker (ClusterIP)                         │
│  Port: 50051                                                 │
│  Selector: app=subgen-worker                                │
└─────────────────────────────────────────────────────────────┘
       ↓                    ↓                    ↓
┌───────────────┐  ┌───────────────┐  ┌───────────────┐
│ Pod: worker-0 │  │ Pod: worker-1 │  │ Pod: worker-2 │
│ StatefulSet   │  │ StatefulSet   │  │ StatefulSet   │
├───────────────┤  ├───────────────┤  ├───────────────┤
│ gRPC :50051   │  │ gRPC :50051   │  │ gRPC :50051   │
│ 2-4GB RAM     │  │ 2-4GB RAM     │  │ 2-4GB RAM     │
│ PVC: models   │  │ PVC: models   │  │ PVC: models   │
└───────────────┘  └───────────────┘  └───────────────┘
```

### Configuration

**Environment Variables (Orchestrator):**

```yaml
env:
  WORKER_DISCOVERY: "kubernetes"              # ← Phase 2 mode
  WORKER_SERVICE_NAME: "subgen-worker"
  WORKER_NAMESPACE: "media"
  WORKER_PORT: "50051"
  LOAD_BALANCE_STRATEGY: "least_loaded"       # or "round_robin"
  QUEUE_MAX_SIZE: "5000"                      # Increased for burst traffic
```

**Worker Deployment:**

```yaml
apiVersion: apps/v1
kind: StatefulSet
metadata:
  name: subgen-worker
  namespace: media
spec:
  replicas: 3                                   # ← Scale here
  serviceName: subgen-worker
  selector:
    matchLabels:
      app: subgen-worker
  template:
    spec:
      containers:
      - name: worker
        image: ghcr.io/user/subgen-worker:latest
        ports:
        - containerPort: 50051
          name: grpc
```

### Characteristics

| Metric | Value | Notes |
|--------|-------|-------|
| **Throughput** | 3-9 transcriptions/hour | 3 workers × 1-3/hour |
| **Concurrency** | 3 (parallel processing) | 3 workers |
| **Memory** | 6-12GB total | 2-4GB per worker |
| **CPU** | 1.5-6 CPU total | 0.5-2 CPU per worker |
| **Startup Time** | 30-60s per worker | Staggered startup |
| **Max Queue Size** | 5000 tasks | Higher capacity |

### When to Use Phase 2

- ✅ Shared media server (10+ episodes/day)
- ✅ Multiple users/libraries
- ✅ Burst traffic (batch imports)
- ✅ High availability requirements
- ✅ Multiple nodes available

### Benefits

- ✅ Parallel processing (3× throughput)
- ✅ High availability (workers can fail independently)
- ✅ Better resource utilization (spread across nodes)
- ✅ Horizontal scaling (add more workers easily)

---

## Worker Discovery Abstraction

### Interface Design

The orchestrator uses a pluggable **Worker Discovery** interface that abstracts how workers are found.

```go
// internal/worker/discovery.go

// WorkerDiscovery finds available workers
type WorkerDiscovery interface {
    // GetWorkers returns all healthy workers
    GetWorkers(ctx context.Context) ([]Worker, error)
    
    // Watch for worker changes (add/remove)
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
    Type   EventType  // Added, Removed, Updated
    Worker Worker
}
```

---

### Phase 1 Implementation: Localhost Discovery

**Single worker on localhost.**

```go
// internal/worker/discovery_localhost.go

type LocalhostDiscovery struct {
    address string
    client  pb.TranscriptionServiceClient
}

func NewLocalhostDiscovery(address string) *LocalhostDiscovery {
    return &LocalhostDiscovery{
        address: address,
    }
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

func (d *LocalhostDiscovery) Watch(ctx context.Context) (<-chan WorkerEvent, error) {
    // No dynamic discovery needed for localhost
    ch := make(chan WorkerEvent)
    close(ch)
    return ch, nil
}
```

**Usage:**

```go
discovery := NewLocalhostDiscovery("localhost:50051")
workers, err := discovery.GetWorkers(ctx)
// Returns: [{ID: "worker-local", Address: "localhost:50051", Healthy: true}]
```

---

### Phase 2 Implementation: Kubernetes Discovery

**Discovers workers via Kubernetes Endpoints API.**

```go
// internal/worker/discovery_kubernetes.go

import (
    metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
    "k8s.io/client-go/kubernetes"
    "k8s.io/client-go/rest"
)

type KubernetesDiscovery struct {
    client    *kubernetes.Clientset
    namespace string
    service   string
    port      int32
}

func NewKubernetesDiscovery(namespace, service string, port int32) (*KubernetesDiscovery, error) {
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
    }, nil
}

func (d *KubernetesDiscovery) GetWorkers(ctx context.Context) ([]Worker, error) {
    // Get service endpoints
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
    
    return workers, nil
}

func (d *KubernetesDiscovery) checkWorkerHealth(ctx context.Context, address string) (bool, int32) {
    ctx, cancel := context.WithTimeout(ctx, 5*time.Second)
    defer cancel()
    
    conn, err := grpc.DialContext(ctx, address, grpc.WithInsecure())
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

func (d *KubernetesDiscovery) Watch(ctx context.Context) (<-chan WorkerEvent, error) {
    // Watch endpoint changes
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
            
            // Convert to worker events
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

**Usage:**

```go
discovery, _ := NewKubernetesDiscovery("media", "subgen-worker", 50051)
workers, err := discovery.GetWorkers(ctx)
// Returns: [
//   {ID: "worker-subgen-worker-0", Address: "10.244.1.5:50051", Healthy: true},
//   {ID: "worker-subgen-worker-1", Address: "10.244.1.6:50051", Healthy: true},
//   {ID: "worker-subgen-worker-2", Address: "10.244.1.7:50051", Healthy: true},
// ]
```

---

### Factory Pattern

**Configuration-driven discovery selection.**

```go
// internal/worker/discovery_factory.go

func NewDiscovery(config *Config) (WorkerDiscovery, error) {
    switch config.WorkerDiscovery {
    case "localhost":
        return NewLocalhostDiscovery(config.PythonWorkerAddress), nil
    
    case "kubernetes":
        return NewKubernetesDiscovery(
            config.WorkerNamespace,
            config.WorkerServiceName,
            config.WorkerPort,
        )
    
    default:
        return nil, fmt.Errorf("unknown worker discovery: %s", config.WorkerDiscovery)
    }
}
```

**Usage:**

```go
// main.go
config := LoadConfig()
discovery, err := worker.NewDiscovery(config)
if err != nil {
    log.Fatal(err)
}

workerPool := worker.NewPool(discovery)
```

---

## Load Balancing Strategies

### Round-Robin

**Simple, fair distribution.**

```go
// internal/worker/loadbalancer_roundrobin.go

type RoundRobinBalancer struct {
    mu      sync.Mutex
    workers []Worker
    next    int
}

func (b *RoundRobinBalancer) SelectWorker(ctx context.Context) (*Worker, error) {
    b.mu.Lock()
    defer b.mu.Unlock()
    
    if len(b.workers) == 0 {
        return nil, ErrNoWorkersAvailable
    }
    
    // Filter healthy workers
    healthy := b.filterHealthy()
    if len(healthy) == 0 {
        return nil, ErrNoHealthyWorkers
    }
    
    // Round-robin
    worker := &healthy[b.next%len(healthy)]
    b.next++
    
    return worker, nil
}

func (b *RoundRobinBalancer) filterHealthy() []Worker {
    var healthy []Worker
    for _, w := range b.workers {
        if w.Healthy {
            healthy = append(healthy, w)
        }
    }
    return healthy
}
```

**Pros:**
- ✅ Simple implementation
- ✅ Fair distribution
- ✅ Predictable

**Cons:**
- ❌ Ignores worker load
- ❌ May overload busy workers

---

### Least-Loaded

**Routes to worker with fewest active jobs.**

```go
// internal/worker/loadbalancer_leastloaded.go

type LeastLoadedBalancer struct {
    mu      sync.Mutex
    workers []Worker
}

func (b *LeastLoadedBalancer) SelectWorker(ctx context.Context) (*Worker, error) {
    b.mu.Lock()
    defer b.mu.Unlock()
    
    // Filter healthy workers
    healthy := b.filterHealthy()
    if len(healthy) == 0 {
        return nil, ErrNoHealthyWorkers
    }
    
    // Find worker with least active jobs
    leastLoaded := &healthy[0]
    for i := range healthy {
        if healthy[i].Active < leastLoaded.Active {
            leastLoaded = &healthy[i]
        }
    }
    
    return leastLoaded, nil
}
```

**Pros:**
- ✅ Better load distribution
- ✅ Handles varying job durations
- ✅ Prevents overload

**Cons:**
- ❌ Slightly more complex
- ❌ Requires health check updates

**Recommended:** Use least-loaded for Phase 2.

---

## Migration Path

### Step 1: Deploy Phase 1

```bash
# Install with single pod
helm install subgen bjw-s/app-template \
  --namespace media \
  --values deploy/values-phase1.yaml
```

**values-phase1.yaml:**

```yaml
controllers:
  main:
    replicas: 1
    containers:
      orchestrator:
        env:
          WORKER_DISCOVERY: "localhost"
      worker:
        # Worker in same pod
```

---

### Step 2: Monitor Performance

**Metrics to watch:**

```promql
# Queue size over time
subgen_queue_size

# Queue wait time
rate(subgen_task_wait_time_seconds[5m])

# Worker utilization
subgen_worker_jobs_active / subgen_worker_jobs_total
```

**Indicators to scale:**

- Queue size consistently > 50
- Queue wait time > 10 minutes
- Worker utilization > 90%

---

### Step 3: Upgrade to Phase 2

```bash
# Upgrade to separate deployments
helm upgrade subgen bjw-s/app-template \
  --namespace media \
  --values deploy/values-phase2.yaml
```

**values-phase2.yaml:**

```yaml
# Orchestrator deployment
controllers:
  orchestrator:
    replicas: 1
    containers:
      orchestrator:
        env:
          WORKER_DISCOVERY: "kubernetes"
          WORKER_SERVICE_NAME: "subgen-worker"
          WORKER_NAMESPACE: "media"

---

# Worker deployment (separate chart instance or multi-chart)
controllers:
  worker:
    type: statefulset
    replicas: 3                           # ← Start with 3 workers
    containers:
      worker:
        # Worker config
```

**Zero downtime migration:**

1. Deploy new workers (StatefulSet)
2. Wait for workers to be healthy
3. Update orchestrator config (WORKER_DISCOVERY=kubernetes)
4. Orchestrator automatically discovers new workers
5. Remove old single-pod deployment

---

### Step 4: Scale Workers

```bash
# Scale to 5 workers
kubectl scale statefulset subgen-worker --replicas=5 -n media

# Or via Helm
helm upgrade subgen-worker bjw-s/app-template \
  --namespace media \
  --set controllers.worker.replicas=5
```

**Orchestrator automatically detects new workers** via Kubernetes watch.

---

## Performance Characteristics

### Latency

| Phase | Metric | P50 | P95 | P99 |
|-------|--------|-----|-----|-----|
| **Phase 1** | Queue wait time | 0s | 5min | 30min |
| **Phase 2** | Queue wait time | 0s | 1min | 5min |

**Assumption:** 1-hour video takes ~10 minutes to transcribe

---

### Throughput

| Workers | Transcriptions/Hour | Transcriptions/Day |
|---------|---------------------|--------------------|
| 1 (Phase 1) | 1-3 | 24-72 |
| 3 (Phase 2) | 3-9 | 72-216 |
| 5 (Phase 2) | 5-15 | 120-360 |

**Assumption:** Transcription time = 10-30 minutes per file

---

### Cost Analysis

**Phase 1 (Single Pod):**

| Resource | Cost/Month | Notes |
|----------|------------|-------|
| CPU (0.6 avg) | $15 | 0.1 + 0.5 |
| RAM (2.3GB avg) | $10 | 64Mi + 2Gi |
| **Total** | **$25/month** | |

**Phase 2 (3 Workers):**

| Resource | Cost/Month | Notes |
|----------|------------|-------|
| Orchestrator CPU (0.1) | $2 | |
| Orchestrator RAM (64Mi) | $1 | |
| Worker CPU (1.5 avg) | $40 | 0.5 × 3 |
| Worker RAM (6GB avg) | $30 | 2GB × 3 |
| **Total** | **$73/month** | |

**Cost per transcription:**

- Phase 1: $0.01 (25 transcriptions/day)
- Phase 2: $0.04 (75 transcriptions/day)

---

## Summary

### Key Points

- ✅ **Single codebase** supports both Phase 1 and Phase 2
- ✅ **Configuration-driven** scaling (no code changes)
- ✅ **Worker Discovery abstraction** enables different deployment models
- ✅ **Zero-downtime migration** from Phase 1 → Phase 2
- ✅ **Horizontal scaling** via `kubectl scale` or Helm values

### Migration Decision Matrix

| Metric | Phase 1 Threshold | Action |
|--------|-------------------|--------|
| Queue size | > 100 for 1 hour | Consider Phase 2 |
| Queue wait time | > 30 minutes | Scale to Phase 2 |
| Episodes/day | > 50 | Scale to Phase 2 |
| Burst imports | > 20 episodes | Scale to Phase 2 temporarily |

### Next Steps

1. Implement both discovery strategies (EPIC_01 STORY_06)
2. Test Phase 1 deployment (EPIC_04)
3. Document migration procedure (EPIC_05)
4. Load test scaling behavior (EPIC_03)

---

**Status:** Ready for implementation  
**Related Epics:** EPIC_01, EPIC_04, EPIC_05  
**Owner:** TBD
