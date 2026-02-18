# Epic 9 Design Audit Report

**Date:** 2026-02-17  
**Auditor:** AI Assistant  
**Status:** Complete  
**Outcome:** ⚠️ **12 Critical Issues Found** - Design requires significant updates before implementation

---

## Executive Summary

I conducted a comprehensive audit of Epic 9's design documentation against the actual codebase. The design documents are well-written and thorough, but they have **significant inconsistencies** with the actual codebase that must be resolved before implementation begins.

### Key Findings

- ✅ **Good:** Design documents are comprehensive (1,800+ lines)
- ✅ **Good:** Concurrency patterns are well-thought-out
- ✅ **Good:** Error handling strategies are detailed
- ❌ **Critical:** Worker struct in design does NOT match actual codebase
- ❌ **Critical:** Design assumes atomic fields that don't exist in actual code
- ❌ **Critical:** WorkerPool API is completely different from design
- ❌ **Critical:** Health check integration is misunderstood
- ❌ **Medium:** Load balancing strategy enum doesn't match
- ❌ **Medium:** Several method signatures don't match

---

## Critical Issues (MUST FIX)

### Issue #1: Worker Struct Mismatch ⚠️ CRITICAL

**Design Assumption** (from `05_WORKER_POOL_CONCURRENCY.md:90-142`):
```go
type Worker struct {
    ID       string
    Address  string
    healthy  int32  // atomic field
    active   int32  // atomic field
    LastSeen time.Time
}
```

**Actual Code** (`orchestrator/internal/discovery/discovery.go:18-25`):
```go
type Worker struct {
    ID       string
    Address  string
    Healthy  bool      // NOT atomic, exported field
    Active   int32     // Exported, NOT atomic
    LastSeen time.Time
}
```

**Impact:** 
- All atomic operation code in design (lines 278-323 of concurrency doc) **won't compile**
- `atomic.LoadInt32(&w.healthy)` → **compile error** (field doesn't exist)
- `atomic.StoreInt32(&w.healthy, 1)` → **compile error**
- Pool implementation uses exported `Healthy bool`, not atomic `healthy int32`

**Fix Required:**
- [ ] Decision needed: Use atomic fields OR use mutex-protected bool?
- [ ] If atomic: Change actual code to match design
- [ ] If mutex: Rewrite entire concurrency document (20+ code blocks)

---

### Issue #2: WorkerPool Data Structure Completely Different ⚠️ CRITICAL

**Design Assumption** (from `05_WORKER_POOL_CONCURRENCY.md:90-130`):
```go
type WorkerPool struct {
    mu sync.RWMutex
    workers map[string]*Worker  // Map by ID
    activeJobs map[string]int32
    discovery WorkerDiscovery
    strategy LoadBalanceStrategy
    log *logrus.Logger
}
```

**Actual Code** (`orchestrator/internal/discovery/pool.go:27-38`):
```go
type Pool struct {
    discovery WorkerDiscovery
    strategy  LoadBalanceStrategy
    log       *logrus.Logger
    
    mu      sync.RWMutex
    workers []Worker  // SLICE, not map
    next    int       // Round-robin counter
    
    healthCheckInterval time.Duration
}
```

**Impact:**
- Design assumes map-based storage: `p.workers[workerID]`
- Actual code uses slice-based storage: `p.workers[i]`
- All O(1) lookups in design become O(n) iterations in actual code
- No `activeJobs` map exists - tracking not implemented
- Method signatures completely different

**Affected Design Code:**
- `AddWorker()` - Lines 223-245 (design) - expects map
- `RemoveWorker()` - Lines 247-271 (design) - expects map  
- `GetWorker()` - Lines 186-196 (design) - expects map
- `UpdateHealth()` - Lines 279-301 (design) - expects map lookup

**Fix Required:**
- [ ] **Option A:** Rewrite design to match actual slice-based approach
- [ ] **Option B:** Rewrite actual code to use map (breaking change)
- [ ] **Recommended:** Option A (actual code is simpler, works fine)

---

### Issue #3: Method Signature Mismatches ⚠️ CRITICAL

**3a. SelectWorker() Signature**

Design (line 88):
```go
func (p *WorkerPool) SelectWorker() (*Worker, error)
```

Actual:
```go
func (p *Pool) SelectWorker() (*Worker, error)  // Type name: Pool, not WorkerPool
```

**3b. UpdateHealth() - Design method doesn't exist**

Design (lines 279-301):
```go
func (p *WorkerPool) UpdateHealth(workerID string, healthy bool, activeJobs int32)
```

Actual:
```go
// Method does NOT exist in actual code
// Health updates happen via Refresh() → GetWorkers() → full replacement
```

**3c. IncrementActiveJobs() - Design method doesn't exist**

Design (lines 303-312):
```go
func (p *WorkerPool) IncrementActiveJobs(workerID string)
func (p *WorkerPool) DecrementActiveJobs(workerID string)
```

Actual:
```go
// Methods do NOT exist
// Active job tracking not implemented in Phase 1
```

**Impact:**
- All task dispatch code in design (lines 423-469) **won't compile**
- Double-check pattern depends on non-existent methods
- In-flight task handling (section 412-535) is completely fictional

**Fix Required:**
- [ ] Either implement these methods OR remove them from design
- [ ] Rewrite task dispatch section to match actual implementation

---

### Issue #4: Health Check Integration Misunderstanding ⚠️ CRITICAL

**Design Assumption** (from `kubernetes.go` design pseudocode):
```go
func (d *KubernetesDiscovery) checkWorkerHealth(ctx context.Context, address string) (bool, int32) {
    // TODO: Implement actual health check using protobuf-generated client
    // client := pb.NewTranscriptionServiceClient(conn)
    // resp, err := client.HealthCheck(ctx, &pb.HealthCheckRequest{})
    // return resp.Status == pb.HealthCheckResponse_HEALTHY, resp.JobsActive
}
```

**Actual Implementation** (`kubernetes.go:48-71`):
```go
func (d *KubernetesDiscovery) checkWorkerHealth(ctx context.Context, address string) (bool, int32) {
    // ... connection code ...
    // TODO: Implement health check using protobuf-generated client
    return true, 0  // Returns hardcoded values!
}
```

**Actual Proto** (`api/transcription.proto:162-195`):
```protobuf
message HealthCheckResponse {
  enum Status {
    UNKNOWN = 0;
    HEALTHY = 1;
    UNHEALTHY = 2;
    STARTING = 3;
  }
  Status status = 1;
  int32 jobs_active = 5;  // Field name is jobs_active, not JobsActive
}
```

**Impact:**
- Design references `resp.JobsActive` (Go naming) but actual proto is `jobs_active`
- Generated code will have `resp.JobsActive` correctly, but TODO comment is misleading
- Health check currently returns hardcoded `true, 0` - **NOT actually checking health**

**Fix Required:**
- [ ] Update design to show actual generated field names
- [ ] Clarify that health check is currently a stub
- [ ] Add Story dependency: Health check must be implemented before Phase 2 works

---

### Issue #5: Load Balance Strategy Type Mismatch ⚠️ MEDIUM

**Design** (`05_WORKER_POOL_CONCURRENCY.md:146-150`):
```go
type LoadBalanceStrategy string

const (
    RoundRobin  LoadBalanceStrategy = "round_robin"
    LeastLoaded LoadBalanceStrategy = "least_loaded"
)
```

**Actual** (`orchestrator/internal/discovery/pool.go:18-24`):
```go
type LoadBalanceStrategy string

const (
    RoundRobin  LoadBalanceStrategy = "round_robin"
    LeastLoaded LoadBalanceStrategy = "least_loaded"
)
```

**Status:** ✅ **MATCH** - No issue here, well done!

---

### Issue #6: Watch Event Processing - Missing Type Definition ⚠️ MEDIUM

**Design** (`05_WORKER_POOL_CONCURRENCY.md:541-623`):
```go
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

**Actual** (`orchestrator/internal/discovery/discovery.go:27-40`):
```go
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

**Status:** ✅ **MATCH** - Good!

---

### Issue #7: Missing K8s Client Field ⚠️ CRITICAL

**Design Expectation** (`EPIC_09/README.md:194-221`):
```go
type KubernetesDiscovery struct {
    client    kubernetes.Clientset  // K8s client
    namespace string
    service   string
    port      int32
    log       *logrus.Logger
}
```

**Actual** (`orchestrator/internal/discovery/kubernetes.go:13-19`):
```go
type KubernetesDiscovery struct {
    namespace string
    service   string
    port      int32
    log       *logrus.Logger
    // client field is MISSING!
}
```

**Impact:**
- All K8s API calls in STORY_01 won't work without client field
- Constructor doesn't initialize K8s client (lines 22-35)
- TODOs indicate this is intentional scaffolding

**Fix Required:**
- [ ] Add `client *kubernetes.Clientset` field to struct
- [ ] Initialize in `NewKubernetesDiscovery()`
- [ ] Update design to match actual struct definition

---

### Issue #8: Error Handling Return Values ⚠️ MEDIUM

**Design Pattern** (`06_K8S_API_ERROR_HANDLING.md:59-86`):
```go
func (d *KubernetesDiscovery) GetWorkers(ctx context.Context) ([]Worker, error) {
    if err != nil {
        if k8sErrors.IsForbidden(err) {
            // Return empty list to allow graceful degradation
            return []Worker{}, fmt.Errorf("RBAC permission denied")
        }
    }
    return workers, nil
}
```

**Design Inconsistency:**
- Sometimes returns `([]Worker{}, error)` (line 78)
- Sometimes returns `(nil, error)` (line 43)
- Sometimes returns `([]Worker{}, nil)` (line 172)

**Actual Code Expectation:**
```go
// Actual pool.go expects non-nil slice
func (p *Pool) Refresh(ctx context.Context) error {
    workers, err := p.discovery.GetWorkers(ctx)
    if err != nil {
        return err  // Error path
    }
    p.workers = workers  // Expects valid slice
}
```

**Impact:**
- Inconsistent error handling patterns confusing
- Need clear decision: Return empty slice + error OR nil + error?

**Fix Required:**
- [ ] Standardize on ONE pattern throughout design
- [ ] **Recommendation:** Always return `([]Worker{}, error)` for consistency

---

## Medium Priority Issues

### Issue #9: Missing Configuration Fields

**Design References** (`EPIC_09/README.md:342-367`):
```yaml
env:
  WORKER_SERVICE_NAME: "subgen-worker"
  WORKER_NAMESPACE: "media"
  WORKER_PORT: "50051"
  LOAD_BALANCE_STRATEGY: "least_loaded"
```

**Actual Config** (`orchestrator/internal/config/config.go:70-79`):
```go
type WorkerConfig struct {
    Discovery string
    Address   string
    Timeout   int // seconds
    
    // Kubernetes-specific fields
    Namespace   string  // ✅ Exists
    ServiceName string  // ✅ Exists
    Port        int32   // ✅ Exists
    // LOAD_BALANCE_STRATEGY field is MISSING!
}
```

**Impact:**
- No way to configure load balance strategy via env var
- Defaults to... what? Not specified!

**Fix Required:**
- [ ] Add `Strategy string` field to `WorkerConfig`
- [ ] Add `WORKER_LOAD_BALANCE_STRATEGY` env var
- [ ] Set default in `setDefaults()` (recommend "least_loaded")

---

### Issue #10: Metrics Variable Names Mismatch

**Design** (`05_WORKER_POOL_CONCURRENCY.md:566-578`):
```go
WorkerSelectionTotal.WithLabelValues("round_robin").Inc()
WorkerSelectionTotal.WithLabelValues("least_loaded").Inc()
```

**Actual** (`orchestrator/internal/discovery/metrics.go` - need to verify):
```go
// Assuming metrics exist, need to verify exact names
```

**Action:** Need to verify actual metric names match design

---

## Design Gaps (Missing Information)

### Gap #1: Worker Selection Call Chain Not Documented

**Current Reality:**
```
HTTP Handler (webhooks/server.go)
  → workerPool.SelectWorker() (discovery/pool.go)
    → grpcClient.Transcribe(workerAddr, task) (grpc_client/client.go)
      → ConnectionPool.Get(addr) (grpc_client/pool.go)
```

**Design Coverage:**
- ✅ `SelectWorker()` logic documented
- ❌ Integration with HTTP handlers not documented
- ❌ Connection pooling not mentioned in Epic 9 docs
- ❌ Relationship between discovery.Pool and grpc_client.ConnectionPool unclear

**Fix Required:**
- [ ] Add section showing full call chain
- [ ] Document how `discovery.Pool` and `grpc_client.ConnectionPool` interact
- [ ] Clarify that they're independent (one selects worker, other manages gRPC conns)

---

### Gap #2: No Documentation on Worker Removal During Active Task

**Scenario Not Addressed:**
1. Task assigned to worker-2 (takes 30 minutes to transcribe)
2. K8s scales down, removes worker-2
3. What happens to the in-flight task?

**Design Claims** (line 423-469):
- "Double-check pattern prevents dispatch to removed workers"
- **BUT:** This only prevents NEW tasks, not in-flight tasks

**Actual Behavior:**
- gRPC connection will fail when pod terminates
- Client retries (3 times with backoff)
- Task fails, needs requeue

**Fix Required:**
- [ ] Document in-flight task failure scenario
- [ ] Specify requeue behavior for failed tasks
- [ ] Add metrics for task failures due to worker removal

---

### Gap #3: Startup Sequence Not Defined

**Question:** What happens when orchestrator starts before workers?

**Design Silence:** No clear startup sequence documented

**Actual Code Behavior** (`pool.go:52-85`):
```go
func (p *Pool) Start(ctx context.Context) error {
    if err := p.Refresh(ctx); err != nil {
        p.log.Warn("Initial worker discovery failed, will retry in background")
        // CONTINUES! Doesn't fail startup
    }
    // ...
}
```

**Fix Required:**
- [ ] Document startup behavior: orchestrator CAN start with 0 workers
- [ ] Clarify that orchestrator will keep retrying discovery
- [ ] Specify what happens to queued tasks when no workers available

---

### Gap #4: RBAC Pre-flight Check Not Implemented

**Design Recommendation** (`06_K8S_API_ERROR_HANDLING.md:690-735`):
```go
func NewKubernetesDiscovery(...) (*KubernetesDiscovery, error) {
    // Verify RBAC permissions
    if err := discovery.verifyRBAC(ctx); err != nil {
        return nil, err  // Fatal error
    }
}
```

**Actual Code** (`kubernetes.go:22-35`):
```go
func NewKubernetesDiscovery(...) (*KubernetesDiscovery, error) {
    // TODO: Initialize K8s client
    // No RBAC verification
    return &KubernetesDiscovery{...}, nil
}
```

**Impact:**
- Orchestrator will start successfully even with RBAC misconfiguration
- Will fail at runtime when trying to discover workers
- Harder to debug ("why aren't workers being discovered?")

**Fix Required:**
- [ ] Implement `verifyRBAC()` in STORY_02
- [ ] Make it optional with flag? (for dev environments without K8s)
- [ ] Or: Document that RBAC errors will appear in logs, not at startup

---

### Gap #5: Watch Reconnection Not Implemented

**Design Expectation** (`05_WORKER_POOL_CONCURRENCY.md:550-670`):
- Automatic reconnection with exponential backoff
- Full resync after reconnect
- Metrics for watch disconnections

**Actual Implementation** (`pool.go:154-186`):
```go
func (p *Pool) watchLoop(ctx context.Context, eventCh <-chan WorkerEvent) {
    for {
        select {
        case <-ctx.Done():
            return
        case event, ok := <-eventCh:
            if !ok {
                return  // Channel closed, watch ended
            }
            p.handleWorkerEvent(event)
        }
    }
}
```

**Impact:**
- Watch closes → watchLoop exits → NO RECONNECTION
- Orchestrator falls back to 30s polling, but watch never restarts

**Fix Required:**
- [ ] STORY_03 must implement reconnection logic
- [ ] Or: Document that watch is "best effort", polling is safety net
- [ ] Add metric: `watch_active` gauge (0 or 1)

---

## Architectural Concerns

### Concern #1: Active Job Tracking Not Implemented

**Design Assumption:**
- "Least Loaded" strategy requires knowing active jobs per worker
- `atomic.AddInt32(&worker.active, 1)` when task starts
- `atomic.AddInt32(&worker.active, -1)` when task completes

**Actual Reality:**
- No tracking mechanism exists
- "Least Loaded" strategy uses `worker.Active` field, but nothing updates it
- Health check returns hardcoded `0` for active jobs

**Impact:**
- "Least Loaded" strategy will behave identically to "Round Robin"
- All workers will always have `Active=0`
- Load balancing won't actually balance load

**Fix Options:**
1. **Option A:** Implement active job tracking in STORY_06A (worker health endpoints)
2. **Option B:** Remove "Least Loaded" strategy from Phase 2, defer to Phase 3
3. **Option C:** Document limitation: "Least Loaded" uses last-known active count, updated every 30s

**Recommendation:** Option A - Implement properly in STORY_06A

---

### Concern #2: No Integration Between discovery.Pool and grpc_client.ConnectionPool

**Two Separate Pools:**
1. `discovery.Pool` - Tracks available workers
2. `grpc_client.ConnectionPool` - Manages gRPC connections

**Question:** How do they stay synchronized?
- Worker removed from discovery.Pool
- Connection still exists in grpc_client.ConnectionPool
- What closes the connection?

**Current Behavior:**
- Connections are never explicitly closed
- Rely on gRPC keepalive to detect dead connections
- Memory leak risk if many workers come/go

**Fix Required:**
- [ ] Document that ConnectionPool uses lazy cleanup
- [ ] Or: Add `Close(workerAddr)` method to ConnectionPool
- [ ] Call from `RemoveWorker()` in discovery.Pool

---

## Testing Gaps

### Gap #T1: No Race Detector Tests Written

**Design Promise** (`05_WORKER_POOL_CONCURRENCY.md:777-825`):
```bash
go test -race ./internal/discovery/...
```

**Actual:**
```bash
$ ls orchestrator/internal/discovery/*test.go
discovery_test.go
factory_test.go
localhost_test.go
pool_test.go

# No pool_race_test.go!
```

**Fix Required:**
- [ ] Create `pool_race_test.go` with concurrent access tests
- [ ] Add to CI pipeline

---

### Gap #T2: No K8s Integration Tests

**Design Requirement** (`EPIC_09/TESTING_PLAN.md`):
- Kind cluster setup
- Real Endpoints API tests
- Watch disconnection tests

**Actual:**
- No K8s integration tests exist yet
- Understandable (feature not implemented)

**Fix Required:**
- [ ] STORY_05 must include integration test setup
- [ ] Document test cluster requirements

---

## Summary of Required Actions

### Before Implementation Starts

1. **CRITICAL:** Decide on Worker struct design
   - [ ] Atomic fields OR mutex-protected fields?
   - [ ] Update design OR update actual code to match

2. **CRITICAL:** Reconcile WorkerPool data structure
   - [ ] Rewrite design to match slice-based pool
   - [ ] Or: Rewrite pool to use map (not recommended)

3. **CRITICAL:** Add missing methods to actual code OR remove from design
   - [ ] `UpdateHealth()`
   - [ ] `IncrementActiveJobs()`
   - [ ] `DecrementActiveJobs()`

4. **CRITICAL:** Implement health check integration
   - [ ] Actually call `HealthCheck()` RPC
   - [ ] Update `Worker.Active` from response

5. **HIGH:** Add K8s client field to KubernetesDiscovery
   - [ ] Add `client *kubernetes.Clientset`
   - [ ] Initialize in constructor

6. **MEDIUM:** Add load balance strategy configuration
   - [ ] Add to WorkerConfig
   - [ ] Add env var

7. **MEDIUM:** Standardize error return patterns
   - [ ] Always `([]Worker{}, error)` OR `(nil, error)`?
   - [ ] Choose one, update design

### Documentation Updates Needed

8. [ ] Document full call chain (handler → pool → grpc client)
9. [ ] Document startup behavior (0 workers OK)
10. [ ] Document in-flight task failure handling
11. [ ] Document watch reconnection strategy
12. [ ] Document ConnectionPool cleanup strategy

### Testing Requirements

13. [ ] Create race detector tests
14. [ ] Create K8s integration test plan
15. [ ] Verify all metrics exist and match design

---

## Positive Findings

### What's Good About The Design

1. ✅ **Comprehensive error catalog** - Every K8s error type documented
2. ✅ **Clear concurrency patterns** - RWMutex strategy is sound
3. ✅ **Good metrics coverage** - All important events tracked
4. ✅ **Thoughtful edge cases** - Watch disconnection, RBAC failures, etc.
5. ✅ **Practical deployment guide** - Helm values, RBAC manifests
6. ✅ **Testing strategy** - Kind setup, success criteria defined

---

## Recommendations

### Immediate Next Steps

1. **DO NOT start implementation** until critical issues resolved
2. **Schedule design review meeting** to decide on key questions:
   - Atomic fields vs mutex-protected fields?
   - Map-based vs slice-based pool?
   - When to implement active job tracking?
3. **Create new work log** documenting decisions made
4. **Update design documents** to match actual code structure
5. **Then and only then:** Begin STORY_01 implementation

### Implementation Order (After Design Fixed)

**Recommended:**
1. STORY_02 (RBAC) - Can be done independently
2. STORY_06A (Worker Health) - Critical for active job tracking
3. STORY_01 (K8s Discovery) - Now has health check integration
4. STORY_03 (Worker Watch) - Builds on discovery
5. STORY_04 (Phase 2 Deployment) - Integration testing
6. STORY_05 (Load Balancing) - Validation
7. STORY_06B (Orchestrator Health) - Nice-to-have

---

## Conclusion

Epic 9's design documents are **well-intentioned but inconsistent with reality**. The good news: both the design and the actual code are salvageable. The bad news: significant reconciliation work is required before implementation can proceed safely.

**Estimated Time to Fix Design Issues:** 4-6 hours  
**Risk if Not Fixed:** High risk of rework, failed PRs, and wasted implementation time

**Next Action:** Schedule design review meeting to resolve critical architectural decisions.

---

**Audit Status:** ✅ Complete  
**Confidence Level:** High (reviewed 2000+ lines of design + 1000+ lines of actual code)  
**Approval for Implementation:** ❌ **BLOCKED** until critical issues resolved
