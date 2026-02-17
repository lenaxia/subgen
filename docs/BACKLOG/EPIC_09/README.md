# EPIC_09: Horizontal Scaling & Multi-Worker Support (Phase 2)

**Status:** Not Started  
**Estimated Effort:** 24-32 hours  
**Duration:** 4-5 days  
**Can Parallelize:** ❌ No (depends on EPIC_04)

---

## Overview

Implement **Phase 2** of the scaling strategy: separate orchestrator and worker deployments with dynamic worker discovery. This enables true horizontal scaling where workers can be added/removed dynamically, and the orchestrator automatically discovers and load-balances across them.

**Current State:** Phase 1 works (single worker in same pod as orchestrator)  
**Goal:** Enable autoscaling of workers from 1 to N with zero orchestrator code changes

---

## Problem Statement

As documented in the investigation (2026-02-17):

**What Works:**
- ✅ Single worker deployment (`WORKER_DISCOVERY=localhost`)
- ✅ Worker pool infrastructure exists
- ✅ Load balancing strategies implemented (Round Robin, Least Loaded)
- ✅ Health checking loop

**What Doesn't Work:**
- ❌ Kubernetes worker discovery not implemented
- ❌ Multiple workers not supported
- ❌ Autoscaling would fail (new workers not discovered)
- ❌ All tasks go to single worker even if replicas=3

**Current Code State:**
```go
// orchestrator/internal/discovery/kubernetes.go:45
func (d *KubernetesDiscovery) GetWorkers(ctx context.Context) ([]Worker, error) {
    // TODO: Implement K8s endpoint discovery
    return nil, fmt.Errorf("kubernetes discovery not yet implemented")
}
```

---

## Goals

1. Implement Kubernetes worker discovery via Endpoints API
2. Configure RBAC for K8s API access
3. Enable dynamic worker watch (add/remove events)
4. Test with 2-5 workers
5. Validate load balancing strategies
6. Document Phase 2 deployment
7. Create autoscaling configurations

---

## Design References

- [00_HYBRID_ARCHITECTURE.md](../../DESIGN/00_HYBRID_ARCHITECTURE.md) - System architecture
- [03_SCALING_STRATEGY.md](../../DESIGN/03_SCALING_STRATEGY.md) - Phase 1 → Phase 2 scaling
- [04_K8S_DEPLOYMENT.md](../../DESIGN/04_K8S_DEPLOYMENT.md) - K8s deployment (needs Phase 2 update)

---

## User Stories

### [STORY_01: Kubernetes Worker Discovery](./stories/STORY_01_k8s_discovery.md)
**Status:** Not Started  
**Effort:** 8-10 hours  
**Summary:** Implement K8s Endpoints API discovery, watch for worker changes

**Key Tasks:**
- Initialize K8s in-cluster client
- Query Endpoints API for worker IPs
- Parse worker addresses from endpoint subsets
- Implement health checks for discovered workers
- Handle errors gracefully (worker pod not ready yet)

### [STORY_02: RBAC Configuration](./stories/STORY_02_rbac.md)
**Status:** Not Started  
**Effort:** 3-4 hours  
**Summary:** ServiceAccount, Role, RoleBinding for Endpoints API access

**Key Tasks:**
- Create ServiceAccount for orchestrator
- Define Role with Endpoints read permissions
- Create RoleBinding
- Update bjw-s values.yaml to use ServiceAccount
- Test RBAC with `kubectl auth can-i`

### [STORY_03: Dynamic Worker Watch](./stories/STORY_03_worker_watch.md)
**Status:** Not Started  
**Effort:** 6-8 hours  
**Summary:** Watch K8s Endpoints for worker add/remove/update events

**Key Tasks:**
- Implement K8s watch on Endpoints
- Handle worker added events
- Handle worker removed events
- Handle worker updated events (health status)
- Update worker pool in real-time
- Log worker lifecycle events

### [STORY_04: Phase 2 Deployment Configuration](./stories/STORY_04_phase2_deployment.md)
**Status:** Not Started  
**Effort:** 4-6 hours  
**Summary:** Separate orchestrator and worker Helm values, StatefulSet for workers

**Key Tasks:**
- Create `values-phase2-orchestrator.yaml`
- Create `values-phase2-workers.yaml`
- Configure worker StatefulSet (replicas: 3)
- Configure ClusterIP service for workers
- Set `WORKER_DISCOVERY=kubernetes` in orchestrator
- Document deployment procedure

### [STORY_05: Load Balancing Testing](./stories/STORY_05_load_balancing.md)
**Status:** Not Started  
**Effort:** 3-4 hours  
**Summary:** Test Round Robin and Least Loaded strategies with multiple workers

**Key Tasks:**
- Deploy 3 workers
- Queue 10 tasks
- Verify Round Robin distributes evenly
- Test Least Loaded with mixed workloads
- Monitor worker metrics (active jobs)
- Document load distribution patterns

---

## Acceptance Criteria

- [ ] All 5 stories completed
- [ ] Kubernetes discovery implementation passes tests
- [ ] RBAC configured correctly (orchestrator can read Endpoints)
- [ ] Worker watch detects add/remove events in <30 seconds
- [ ] Phase 2 deployment works with 1 worker
- [ ] Phase 2 deployment works with 3 workers
- [ ] Phase 2 deployment works with 5 workers
- [ ] Scaling from 3→5 workers works without orchestrator restart
- [ ] Scaling from 5→3 workers works without losing tasks
- [ ] Both load balancing strategies tested and validated
- [ ] No tasks go to unhealthy workers
- [ ] Documentation complete (Phase 2 deployment guide)
- [ ] Work logs created for all stories

---

## Technical Implementation

### 1. Kubernetes Client Initialization

```go
// orchestrator/internal/discovery/kubernetes.go
import (
    "k8s.io/client-go/kubernetes"
    "k8s.io/client-go/rest"
    metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

func NewKubernetesDiscovery(namespace, service string, port int32, log *logrus.Logger) (*KubernetesDiscovery, error) {
    // Initialize in-cluster K8s client
    config, err := rest.InClusterConfig()
    if err != nil {
        return nil, fmt.Errorf("failed to get in-cluster config: %w", err)
    }
    
    clientset, err := kubernetes.NewForConfig(config)
    if err != nil {
        return nil, fmt.Errorf("failed to create K8s client: %w", err)
    }
    
    return &KubernetesDiscovery{
        client:    clientset,
        namespace: namespace,
        service:   service,
        port:      port,
        log:       log,
    }, nil
}
```

### 2. Endpoints Discovery

```go
func (d *KubernetesDiscovery) GetWorkers(ctx context.Context) ([]Worker, error) {
    // Get Endpoints object for worker service
    endpoints, err := d.client.CoreV1().Endpoints(d.namespace).Get(
        ctx, d.service, metav1.GetOptions{},
    )
    if err != nil {
        return nil, fmt.Errorf("failed to get endpoints: %w", err)
    }
    
    var workers []Worker
    
    // Parse IPs from endpoint subsets
    for _, subset := range endpoints.Subsets {
        for _, addr := range subset.Addresses {
            workerAddr := fmt.Sprintf("%s:%d", addr.IP, d.port)
            
            // Check worker health
            healthy, active := d.checkWorkerHealth(ctx, workerAddr)
            
            worker := Worker{
                ID:       addr.TargetRef.Name, // Pod name
                Address:  workerAddr,
                Healthy:  healthy,
                Active:   active,
                LastSeen: time.Now(),
            }
            
            workers = append(workers, worker)
        }
    }
    
    d.log.WithField("count", len(workers)).Info("Discovered workers from K8s")
    
    return workers, nil
}
```

### 3. Watch Implementation

```go
func (d *KubernetesDiscovery) Watch(ctx context.Context) (<-chan WorkerEvent, error) {
    // Create watch on Endpoints
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
        
        for {
            select {
            case <-ctx.Done():
                return
            case event, ok := <-watcher.ResultChan():
                if !ok {
                    return
                }
                
                // Parse event and send to channel
                d.handleEndpointEvent(ctx, event, ch)
            }
        }
    }()
    
    return ch, nil
}
```

### 4. RBAC Configuration

```yaml
# ServiceAccount
apiVersion: v1
kind: ServiceAccount
metadata:
  name: subgen-orchestrator
  namespace: media

---
# Role
apiVersion: rbac.authorization.k8s.io/v1
kind: Role
metadata:
  name: subgen-orchestrator
  namespace: media
rules:
- apiGroups: [""]
  resources: ["endpoints"]
  verbs: ["get", "list", "watch"]

---
# RoleBinding
apiVersion: rbac.authorization.k8s.io/v1
kind: RoleBinding
metadata:
  name: subgen-orchestrator
  namespace: media
subjects:
- kind: ServiceAccount
  name: subgen-orchestrator
  namespace: media
roleRef:
  kind: Role
  name: subgen-orchestrator
  apiGroup: rbac.authorization.k8s.io
```

### 5. bjw-s Orchestrator Values (Phase 2)

```yaml
# values-phase2-orchestrator.yaml
defaultPodOptions:
  serviceAccountName: subgen-orchestrator  # Use custom SA
  automountServiceAccountToken: true       # Need K8s API access

controllers:
  main:
    type: deployment
    replicas: 1
    
    containers:
      orchestrator:
        image:
          repository: ghcr.io/lenaxia/subgen-orchestrator
          tag: "latest"
        
        env:
          # Phase 2: Kubernetes discovery
          WORKER_DISCOVERY: "kubernetes"
          WORKER_SERVICE_NAME: "subgen-worker"
          WORKER_NAMESPACE: "media"
          WORKER_PORT: "50051"
          LOAD_BALANCE_STRATEGY: "least_loaded"
          
          # ... (rest of config)
```

### 6. bjw-s Worker Values (Phase 2)

```yaml
# values-phase2-workers.yaml
controllers:
  main:
    type: statefulset  # StatefulSet for stable identities
    replicas: 3        # Scale here!
    
    containers:
      worker:
        image:
          repository: ghcr.io/lenaxia/subgen-worker
          tag: "latest"
        
        # ... (same as Phase 1)

service:
  main:
    controller: main
    type: ClusterIP  # Internal only
    ports:
      grpc:
        port: 50051
```

---

## Dependencies

**Requires:**
- EPIC_04 (K8s Deployment Phase 1) - **MUST be complete**
- Working single-worker deployment
- K8s cluster with RBAC enabled

**Blocks:**
- Autoscaling (HPA/KEDA)
- Production high-volume deployments

**Parallelizable With:**
- None (sequential epic)

---

## Deployment Procedure

### Phase 2 Installation

```bash
# 1. Create RBAC resources
kubectl apply -f deploy/rbac.yaml

# 2. Install workers first
helm install subgen-worker bjw-s/app-template \
  --namespace media \
  --values deploy/values-phase2-workers.yaml

# 3. Wait for workers to be ready
kubectl wait --for=condition=ready pod \
  -l app.kubernetes.io/name=subgen-worker \
  -n media \
  --timeout=300s

# 4. Install orchestrator
helm install subgen-orchestrator bjw-s/app-template \
  --namespace media \
  --values deploy/values-phase2-orchestrator.yaml

# 5. Verify worker discovery
kubectl logs -n media -l app.kubernetes.io/name=subgen-orchestrator --tail=50
# Should see: "Discovered 3 workers from K8s"
```

### Scaling Workers

```bash
# Scale to 5 workers
kubectl scale statefulset subgen-worker --replicas=5 -n media

# Verify discovery (should happen within 30 seconds)
kubectl logs -n media -l app.kubernetes.io/name=subgen-orchestrator --tail=20
# Should see: "Worker added: subgen-worker-3"
#             "Worker added: subgen-worker-4"
#             "Discovered 5 workers from K8s"
```

---

## Validation Tests

### 1. Worker Discovery

```bash
# Deploy 3 workers
kubectl get pods -n media -l app.kubernetes.io/name=subgen-worker
# NAME                READY   STATUS    RESTARTS   AGE
# subgen-worker-0     1/1     Running   0          2m
# subgen-worker-1     1/1     Running   0          2m
# subgen-worker-2     1/1     Running   0          2m

# Check orchestrator discovered them
kubectl logs -n media -l app.kubernetes.io/name=subgen-orchestrator | grep "Discovered"
# INFO: Discovered 3 workers from K8s
```

### 2. Load Distribution

```bash
# Queue 10 tasks via API
for i in {1..10}; do
  curl -X POST http://orchestrator-ip:9000/batch \
    -d '{"path":"/media/test/file'$i'.mp4"}'
done

# Check worker metrics
curl http://orchestrator-ip:9090/metrics | grep worker_active_jobs
# subgen_worker_active_jobs{worker="subgen-worker-0"} 3
# subgen_worker_active_jobs{worker="subgen-worker-1"} 4
# subgen_worker_active_jobs{worker="subgen-worker-2"} 3
```

### 3. Dynamic Scaling

```bash
# Scale up from 3 to 5
kubectl scale statefulset subgen-worker --replicas=5 -n media

# Wait 30 seconds for discovery
sleep 30

# Verify new workers discovered
kubectl logs -n media -l app.kubernetes.io/name=subgen-orchestrator --tail=10
# INFO: Worker added: subgen-worker-3
# INFO: Worker added: subgen-worker-4
# INFO: Discovered 5 workers from K8s
```

### 4. Worker Removal

```bash
# Scale down from 5 to 3
kubectl scale statefulset subgen-worker --replicas=3 -n media

# Wait 60 seconds
sleep 60

# Verify workers removed
kubectl logs -n media -l app.kubernetes.io/name=subgen-orchestrator --tail=10
# INFO: Worker removed: subgen-worker-4
# INFO: Worker removed: subgen-worker-3
# INFO: Discovered 3 workers from K8s
```

---

## Timeline

**Day 1:**
- STORY_01 (K8s Discovery) - 8-10 hours

**Day 2:**
- STORY_02 (RBAC) - 3-4 hours
- STORY_03 (Worker Watch) - Start (4 hours)

**Day 3:**
- STORY_03 (Worker Watch) - Finish (2-4 hours)
- STORY_04 (Phase 2 Deployment) - 4-6 hours

**Day 4:**
- STORY_05 (Load Balancing Testing) - 3-4 hours
- Documentation & polish - 2-3 hours

**Day 5 (Buffer):**
- Testing & bug fixes
- Work logs
- Final validation

---

## Risks & Mitigation

| Risk | Impact | Mitigation |
|------|--------|------------|
| K8s client initialization fails | High | Graceful fallback to localhost mode |
| Watch disconnects | Medium | Automatic reconnect with exponential backoff |
| Worker discovery lag | Medium | Periodic refresh every 30s as backup |
| RBAC misconfiguration | High | Test with `kubectl auth can-i` before deployment |
| Race condition in pool updates | Medium | Use mutex locks, write comprehensive tests |
| Worker removed while processing | High | Graceful shutdown, requeue tasks |

---

## Monitoring & Metrics

### New Metrics

```go
// Worker discovery
subgen_workers_discovered_total{source="kubernetes"} 5

// Worker events
subgen_worker_events_total{type="added"} 2
subgen_worker_events_total{type="removed"} 1
subgen_worker_events_total{type="updated"} 10

// Load balancing
subgen_worker_selection_total{strategy="least_loaded"} 100
subgen_worker_active_jobs{worker="subgen-worker-0"} 3
```

### Alerts

```yaml
# No healthy workers
- alert: NoHealthyWorkers
  expr: sum(subgen_worker_healthy) == 0
  for: 1m
  
# Worker discovery failing
- alert: WorkerDiscoveryFailing
  expr: rate(subgen_worker_discovery_errors_total[5m]) > 0.1
  for: 5m
```

---

## Definition of Done

- [ ] All 5 stories completed with ✅ status
- [ ] Kubernetes discovery implementation complete
- [ ] RBAC resources created and tested
- [ ] Worker watch implemented and tested
- [ ] Phase 2 deployment works with 1, 3, 5 workers
- [ ] Scaling up works (3→5) without restart
- [ ] Scaling down works (5→3) without task loss
- [ ] Round Robin distributes tasks evenly
- [ ] Least Loaded selects least busy worker
- [ ] No tasks sent to unhealthy workers
- [ ] Worker removal handled gracefully
- [ ] Documentation complete (Phase 2 guide)
- [ ] Work logs created for all stories
- [ ] Integration tests pass
- [ ] Load testing validates distribution

---

## Next Steps After EPIC_09

**Autoscaling:**
- Horizontal Pod Autoscaler (HPA) based on queue size
- KEDA for advanced scaling (queue metrics)

**Production Hardening:**
- Worker affinity/anti-affinity rules
- Resource quotas
- Network policies
- Pod disruption budgets

---

## References

- README-LLM.md - Development workflow
- [03_SCALING_STRATEGY.md](../../DESIGN/03_SCALING_STRATEGY.md) - Phase 2 architecture
- [04_K8S_DEPLOYMENT.md](../../DESIGN/04_K8S_DEPLOYMENT.md) - Needs update for Phase 2
- K8s client-go docs: https://github.com/kubernetes/client-go
- Endpoints API: https://kubernetes.io/docs/reference/kubernetes-api/service-resources/endpoints-v1/

---

## Investigation Reference

This epic was created based on investigation on 2026-02-17:
- Question: "Can orchestrator handle multiple workers as is?"
- Answer: **No** - Kubernetes discovery not implemented
- See: Conversation about autoscaling and worker pools

---

**Epic Owner:** TBD  
**Created:** 2026-02-17  
**Last Updated:** 2026-02-17  
**Priority:** Medium (after Phase 1 deployment working)
