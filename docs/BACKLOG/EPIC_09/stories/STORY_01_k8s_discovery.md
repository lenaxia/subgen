# STORY_01: Kubernetes Worker Discovery

**Epic:** EPIC_09  
**Status:** Not Started  
**Assignee:** TBD  
**Effort:** 8-10 hours

---

## User Story

As an **orchestrator**,  
I want to **discover worker pods via Kubernetes Endpoints API**,  
So that **I can distribute transcription tasks across multiple workers**.

---

## Acceptance Criteria

- [ ] K8s in-cluster client initializes successfully
- [ ] Endpoints API query returns worker IPs
- [ ] Worker addresses parsed correctly (IP:port format)
- [ ] Health checks performed for each discovered worker
- [ ] Unhealthy workers marked in worker list
- [ ] Discovery errors handled gracefully (returns empty list, not crash)
- [ ] Unit tests cover discovery logic
- [ ] Integration tests with real K8s Endpoints

---

## Technical Design

### Implementation File

`orchestrator/internal/discovery/kubernetes.go`

### Key Changes

1. **Initialize K8s Client**
```go
import (
    "k8s.io/client-go/kubernetes"
    "k8s.io/client-go/rest"
)

func NewKubernetesDiscovery(...) (*KubernetesDiscovery, error) {
    config, err := rest.InClusterConfig()
    if err != nil {
        return nil, fmt.Errorf("failed to get in-cluster config: %w", err)
    }
    
    clientset, err := kubernetes.NewForConfig(config)
    if err != nil {
        return nil, fmt.Errorf("failed to create K8s client: %w", err)
    }
    
    return &KubernetesDiscovery{
        client: clientset,
        // ...
    }, nil
}
```

2. **Implement GetWorkers()**
```go
func (d *KubernetesDiscovery) GetWorkers(ctx context.Context) ([]Worker, error) {
    endpoints, err := d.client.CoreV1().Endpoints(d.namespace).Get(
        ctx, d.service, metav1.GetOptions{},
    )
    if err != nil {
        return nil, fmt.Errorf("failed to get endpoints: %w", err)
    }
    
    var workers []Worker
    for _, subset := range endpoints.Subsets {
        for _, addr := range subset.Addresses {
            worker := d.createWorker(ctx, addr)
            workers = append(workers, worker)
        }
    }
    
    return workers, nil
}
```

3. **Health Check Helper**
```go
func (d *KubernetesDiscovery) checkWorkerHealth(ctx context.Context, address string) (bool, int32) {
    // Use existing gRPC health check
    // Return (healthy bool, active jobs int32)
}
```

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

- `k8s.io/client-go` Go module
- `k8s.io/apimachinery` Go module
- Existing gRPC client for health checks

---

## Files to Modify

- `orchestrator/internal/discovery/kubernetes.go` - Main implementation
- `orchestrator/internal/discovery/kubernetes_test.go` - Unit tests
- `orchestrator/go.mod` - Add K8s dependencies

---

## Definition of Done

- [ ] Implementation complete
- [ ] Unit tests written and passing
- [ ] Integration tests written and passing
- [ ] Code reviewed
- [ ] Error handling comprehensive
- [ ] Logging added (debug + info levels)
- [ ] Work log created

---

**Story Owner:** TBD  
**Created:** 2026-02-17
