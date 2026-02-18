# STORY_01: Kubernetes Worker Discovery

**Epic:** EPIC_09  
**Status:** Not Started  
**Assignee:** TBD  
**Effort:** 8-10 hours  
**Dependencies:** None (can start immediately)

---

## User Story

As an **orchestrator**,  
I want to **discover worker pods via Kubernetes Endpoints API**,  
So that **I can distribute transcription tasks across multiple workers**.

---

## Acceptance Criteria

### Core Functionality
- [ ] **CRITICAL:** Add `client *kubernetes.Clientset` field to `KubernetesDiscovery` struct
- [ ] K8s in-cluster client initializes successfully in `NewKubernetesDiscovery()`
- [ ] Endpoints API query returns worker IPs from K8s
- [ ] Worker addresses parsed correctly (IP:port format)
- [ ] Health checks performed for each discovered worker (gRPC HealthCheck RPC)
- [ ] Unhealthy workers marked as `Healthy: false` in returned list
- [ ] Discovery errors handled gracefully (returns `[]Worker{}`, not crash)

### Docker Compatibility (CRITICAL)
- [ ] **CRITICAL:** Error message explains Docker vs K8s when run outside cluster
- [ ] Docker Compose still works with `WORKER_DISCOVERY=localhost` (the default)
- [ ] Clear error if user accidentally sets `WORKER_DISCOVERY=kubernetes` in Docker
- [ ] Log message shows which discovery mode is active on startup

### Testing
- [ ] Unit tests cover discovery logic with mocked K8s client
- [ ] Unit test verifies helpful error outside K8s cluster
- [ ] Integration tests with real K8s Endpoints (Kind cluster)
- [ ] Docker Compose smoke test (verify localhost mode still works)

---

## Technical Design

### Implementation File

`orchestrator/internal/discovery/kubernetes.go`

### Key Changes

**1. Update KubernetesDiscovery struct** (ADD client field)

```go
type KubernetesDiscovery struct {
	client    *kubernetes.Clientset  // ADD THIS LINE
	namespace string
	service   string
	port      int32
	log       *logrus.Logger
}
```

**2. Initialize K8s Client** (with Docker compatibility)

```go
import (
    "strings"
    "k8s.io/client-go/kubernetes"
    "k8s.io/client-go/rest"
    metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

func NewKubernetesDiscovery(namespace, service string, port int32, log *logrus.Logger) (*KubernetesDiscovery, error) {
    // Get in-cluster K8s config
    config, err := rest.InClusterConfig()
    if err != nil {
        // CRITICAL: Provide helpful error for Docker deployments
        if strings.Contains(err.Error(), "unable to load in-cluster configuration") {
            return nil, fmt.Errorf(
                "kubernetes discovery requires running inside a Kubernetes cluster. "+
                "For Docker Compose deployments, use WORKER_DISCOVERY=localhost (or omit, it's the default). "+
                "Original error: %w", err)
        }
        return nil, fmt.Errorf("failed to get in-cluster config: %w", err)
    }
    
    // Create K8s clientset
    clientset, err := kubernetes.NewForConfig(config)
    if err != nil {
        return nil, fmt.Errorf("failed to create K8s client: %w", err)
    }
    
    log.Info("Kubernetes discovery initialized successfully (running in K8s cluster)")
    
    return &KubernetesDiscovery{
        client:    clientset,  // CRITICAL: Set this field
        namespace: namespace,
        service:   service,
        port:      port,
        log:       log,
    }, nil
}
```

**Why This Matters:**
- `rest.InClusterConfig()` ONLY works inside Kubernetes pods
- Will fail in Docker Compose with confusing error
- Better error message prevents user confusion
- Docker deployments should use `WORKER_DISCOVERY=localhost` (the default)

**3. Implement GetWorkers()** (Replace TODOs with actual code)

```go
func (d *KubernetesDiscovery) GetWorkers(ctx context.Context) ([]Worker, error) {
    // Get Endpoints object for worker service
    endpoints, err := d.client.CoreV1().Endpoints(d.namespace).Get(
        ctx, d.service, metav1.GetOptions{},
    )
    if err != nil {
        // Handle errors gracefully
        if k8sErrors.IsNotFound(err) {
            d.log.Warn("Worker service endpoints not found - workers may not be deployed yet")
            return []Worker{}, nil  // Return empty slice, not error
        }
        if k8sErrors.IsForbidden(err) {
            d.log.Error("RBAC permission denied - check ServiceAccount/Role/RoleBinding")
            return []Worker{}, fmt.Errorf("RBAC permission denied: %w", err)
        }
        return []Worker{}, fmt.Errorf("failed to get endpoints: %w", err)
    }
    
    // Check if any pods are ready
    if len(endpoints.Subsets) == 0 {
        d.log.Debug("Endpoints exist but no ready pods yet")
        return []Worker{}, nil
    }
    
    // Parse worker IPs from endpoint subsets
    var workers []Worker
    for _, subset := range endpoints.Subsets {
        for _, addr := range subset.Addresses {
            workerAddr := fmt.Sprintf("%s:%d", addr.IP, d.port)
            
            // Perform health check via gRPC
            healthy, activeJobs := d.checkWorkerHealth(ctx, workerAddr)
            
            worker := Worker{
                ID:       addr.TargetRef.Name, // Pod name
                Address:  workerAddr,
                Healthy:  healthy,
                Active:   activeJobs,
                LastSeen: time.Now(),
            }
            
            workers = append(workers, worker)
        }
    }
    
    d.log.WithField("count", len(workers)).Info("Discovered workers from K8s")
    
    return workers, nil
}
```

**4. Implement Health Check** (Update existing stub)

```go
func (d *KubernetesDiscovery) checkWorkerHealth(ctx context.Context, address string) (bool, int32) {
    // Create timeout context for health check (5 seconds)
    ctx, cancel := context.WithTimeout(ctx, 5*time.Second)
    defer cancel()

    // Try to connect to worker
    conn, err := grpc.DialContext(ctx, address,
        grpc.WithTransportCredentials(insecure.NewCredentials()),
        grpc.WithBlock(),
    )
    if err != nil {
        // Connection failed - worker unhealthy
        return false, 0
    }
    defer conn.Close()

    // Call HealthCheck RPC
    client := pb.NewTranscriptionServiceClient(conn)
    resp, err := client.HealthCheck(ctx, &pb.HealthCheckRequest{})
    if err != nil {
        // Health check failed
        return false, 0
    }

    // Check response status
    healthy := resp.Status == pb.HealthCheckResponse_HEALTHY
    activeJobs := resp.JobsActive  // Note: Proto field name is JobsActive (generated Go code)

    return healthy, activeJobs
}
```

**Note:** This implementation actually calls the gRPC health check, unlike the current stub which returns hardcoded values.

---

## Testing Strategy

### Unit Tests

```go
// TestNewKubernetesDiscovery_Success
// TestNewKubernetesDiscovery_InClusterConfigError
// TestGetWorkers_Success
// TestGetWorkers_NoEndpoints
// TestGetWorkers_EmptySubsets
// TestGetWorkers_HealthCheckFails
```

### Integration Tests

Requires K8s test environment (kind or minikube):
1. Create test namespace
2. Create test Endpoints object
3. Call GetWorkers()
4. Verify workers discovered

---

## Dependencies

**Go Modules to Add:**
```bash
go get k8s.io/client-go@latest
go get k8s.io/apimachinery@latest
go get k8s.io/api@latest
```

**Existing Code:**
- ✅ `discovery.Worker` struct exists (`internal/discovery/discovery.go`)
- ✅ `pb.TranscriptionServiceClient` exists (`pkg/pb/transcription_grpc.pb.go`)
- ✅ `pb.HealthCheckRequest/Response` defined (`api/transcription.proto`)
- ⚠️ `KubernetesDiscovery` struct exists but incomplete (missing `client` field)

**Blockers:** None - can start immediately

---

## Files to Modify

- `orchestrator/internal/discovery/kubernetes.go` - Main implementation
- `orchestrator/internal/discovery/kubernetes_test.go` - Unit tests
- `orchestrator/go.mod` - Add K8s dependencies

---

## Definition of Done

### Implementation
- [ ] Implementation complete
- [ ] `client` field added to `KubernetesDiscovery` struct
- [ ] Error handling comprehensive (including Docker compatibility)
- [ ] Logging added (debug + info levels)
- [ ] Helpful error message for non-K8s environments

### Testing
- [ ] Unit tests written and passing
- [ ] Unit test for non-K8s environment error
- [ ] Integration tests written and passing (Kind cluster)
- [ ] Docker Compose smoke test passes (localhost mode)
- [ ] Code reviewed

### Documentation
- [ ] Work log created
- [ ] docker-compose.yml has comment about WORKER_DISCOVERY
- [ ] README updated with deployment modes explanation (Docker vs K8s)

---

**Story Owner:** TBD  
**Created:** 2026-02-17
