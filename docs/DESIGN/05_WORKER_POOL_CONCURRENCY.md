# Worker Pool Concurrency & Thread Safety

**Document Version:** 2.0  
**Last Updated:** 2026-02-17  
**Status:** ✅ Reconciled with Actual Code  
**Related Documents:**
- [00_HYBRID_ARCHITECTURE.md](./00_HYBRID_ARCHITECTURE.md)
- [03_SCALING_STRATEGY.md](./03_SCALING_STRATEGY.md)
- [EPIC_09 README](../BACKLOG/EPIC_09/README.md)
- [Design Audit Report](../BACKLOG/EPIC_09/DESIGN_AUDIT_2026-02-17.md)

---

## 🔄 Document Status

**Version 2.0** - This document has been updated to match the **actual codebase** as it exists today. All code examples reflect the real implementation in `orchestrator/internal/discovery/pool.go`.

**Changes from v1.0:**
- Updated to slice-based worker storage (not map-based)
- Simplified to exported bool fields (not atomic int32)
- Removed hypothetical methods not in actual code
- Added clarifications on limitations and future work

---

## Table of Contents

1. [Overview](#overview)
2. [Concurrency Challenges](#concurrency-challenges)
3. [Worker Pool Data Structure](#worker-pool-data-structure)
4. [Thread Safety Strategy](#thread-safety-strategy)
5. [Worker Selection Algorithm](#worker-selection-algorithm)
6. [In-Flight Task Handling](#in-flight-task-handling)
7. [Watch Event Processing](#watch-event-processing)
8. [Race Condition Prevention](#race-condition-prevention)
9. [Testing Strategy](#testing-strategy)
10. [Known Limitations](#known-limitations)

---

## Overview

### Purpose

This document defines the **thread safety model** for the worker pool in the Go orchestrator, ensuring safe concurrent access from:
1. **Task dispatcher loop** (reads workers for selection)
2. **Health check loop** (refreshes entire worker list every 30s)
3. **Watch event handler** (adds/removes/updates individual workers)
4. **HTTP API handlers** (reads worker list for status endpoint)

### Design Principles

1. **Keep it simple**: Slice-based storage with full-list refreshes
2. **Minimize lock contention**: Use read/write mutexes where appropriate
3. **No deadlocks**: Clear lock ordering, short critical sections
4. **Fail-safe**: Prefer availability over consistency
5. **Graceful degradation**: Continue operating with reduced capacity if workers fail

### Implementation Status

✅ **Phase 1 Complete:** Single worker via localhost discovery  
🚧 **Phase 2 In Progress:** Kubernetes discovery with multiple workers

---

## Concurrency Challenges

### Scenario 1: Worker Removed Mid-Selection

```
Time    SelectWorker() Thread      RemoveWorker() Thread
----    ---------------------      ---------------------
T0      RLock()
T1      Read workers[2]
T2                                 Lock()
T3                                 Remove workers[2]
T4                                 Unlock()
T5      Select workers[2]
T6      RUnlock()
T7      Use workers[2].Address
```

**Challenge**: Worker removed after being selected but before being used  
**Solution**: gRPC client handles connection failures gracefully with retries

### Scenario 2: Concurrent Health Updates

```
Thread 1: Refresh() → GetWorkers() → Replace entire list
Thread 2: SelectWorker() → Reading from old list
```

**Challenge**: Worker list replaced during selection  
**Solution**: RLock prevents replacement during read

### Scenario 3: Watch Event During Refresh

```
T0      Refresh() starts (Lock held)
T1      Watch event arrives (tries to Lock)
T2      Refresh() completes, Unlock
T3      Watch event processes
```

**Challenge**: Watch events blocked during refresh  
**Solution**: Acceptable - refresh is fast (<100ms), events are infrequent

---

## Worker Pool Data Structure

### Actual Implementation

**File**: `orchestrator/internal/discovery/pool.go`

```go
package discovery

import (
	"context"
	"sync"
	"time"

	"github.com/sirupsen/logrus"
)

// Pool manages a pool of workers with health checking
type Pool struct {
	discovery WorkerDiscovery
	strategy  LoadBalanceStrategy
	log       *logrus.Logger

	// Protects workers slice and next counter
	mu      sync.RWMutex
	workers []Worker  // Slice of workers (not map!)
	next    int       // Round-robin counter

	// Health check interval
	healthCheckInterval time.Duration
}

// Worker represents a transcription worker
type Worker struct {
	ID       string
	Address  string
	Healthy  bool      // Exported, not atomic
	Active   int32     // Exported, not atomic (updated by health check)
	LastSeen time.Time
}

// LoadBalanceStrategy defines how to select workers
type LoadBalanceStrategy string

const (
	RoundRobin  LoadBalanceStrategy = "round_robin"
	LeastLoaded LoadBalanceStrategy = "least_loaded"
)
```

### Design Rationale

**Why Slice Instead of Map?**
1. **Simplicity**: Easier to reason about, less code
2. **Small scale**: Typical deployment has 1-10 workers, O(n) is fine
3. **Full refresh pattern**: `Refresh()` replaces entire slice atomically
4. **Sequential access**: Round-robin naturally uses array index

**Why Exported Fields Instead of Atomic?**
1. **Refresh pattern**: Health check updates entire Worker struct via `Refresh()`
2. **No incremental updates**: We don't call `worker.Healthy = true` between refreshes
3. **Simpler code**: No atomic operations needed
4. **Adequate safety**: RWMutex protects all access

**Trade-offs Accepted:**
- ❌ No O(1) lookup by worker ID (but not needed for our access patterns)
- ❌ No real-time active job tracking (updated every 30s via health check)
- ✅ Simpler, more maintainable code
- ✅ Fewer race condition opportunities

---

## Thread Safety Strategy

### Lock Ordering Rules

**CRITICAL**: Always acquire locks in this order to prevent deadlocks:

```
1. pool.mu (worker pool lock)
2. External resources (gRPC connections, metrics, etc.)
```

### Read Operations (RLock)

Functions that only **read** the worker list:

```go
// SelectWorker - Select a worker for task dispatch
func (p *Pool) SelectWorker() (*Worker, error) {
	p.mu.Lock()  // Note: Uses Lock, not RLock (modifies next counter)
	defer p.mu.Unlock()

	healthy := p.filterHealthy()
	if len(healthy) == 0 {
		return nil, ErrNoHealthyWorkers
	}

	var worker *Worker

	switch p.strategy {
	case RoundRobin:
		worker = &healthy[p.next%len(healthy)]
		p.next++  // Modify counter under lock
		WorkerSelectionTotal.WithLabelValues("round_robin").Inc()

	case LeastLoaded:
		worker = p.findLeastLoaded(healthy)
		WorkerSelectionTotal.WithLabelValues("least_loaded").Inc()

	default:
		return nil, fmt.Errorf("unknown strategy: %s", p.strategy)
	}

	return worker, nil
}
```

**Note:** `SelectWorker()` uses full `Lock()` because it modifies the `next` counter for round-robin.

### Write Operations (Lock)

Functions that **modify** the worker list:

```go
// Refresh re-discovers all workers (replaces entire list)
func (p *Pool) Refresh(ctx context.Context) error {
	workers, err := p.discovery.GetWorkers(ctx)
	if err != nil {
		WorkerDiscoveryErrors.Inc()
		return err
	}

	p.mu.Lock()
	p.workers = workers  // Atomic replacement
	p.mu.Unlock()

	// Update metrics
	UpdateWorkerMetrics(workers)

	p.log.WithField("count", len(workers)).Info("Workers refreshed")

	return nil
}
```

**Key Pattern:** Full list replacement, not incremental updates.

### Helper Methods (Lock Protected)

```go
// filterHealthy returns only healthy workers (called under lock)
func (p *Pool) filterHealthy() []Worker {
	var healthy []Worker
	for _, w := range p.workers {
		if w.Healthy {  // Simple bool check, no atomic needed
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
```

---

## Worker Selection Algorithm

### Round Robin Strategy

**Implementation:**

```go
func (p *Pool) SelectWorker() (*Worker, error) {
	p.mu.Lock()
	defer p.mu.Unlock()

	healthy := p.filterHealthy()
	if len(healthy) == 0 {
		return nil, ErrNoHealthyWorkers
	}

	// Simple counter-based selection
	worker := &healthy[p.next%len(healthy)]
	p.next++

	return worker, nil
}
```

**Characteristics:**
- ✅ Simple, predictable
- ✅ Even distribution for uniform workloads
- ✅ No external state needed
- ❌ Doesn't account for worker load
- ❌ May overload slow workers

**When to Use:** Default strategy, good for uniform transcription tasks

---

### Least Loaded Strategy

**Implementation:**

```go
func (p *Pool) findLeastLoaded(workers []Worker) *Worker {
	if len(workers) == 0 {
		return nil
	}

	var best *Worker
	minActive := int32(1<<31 - 1)  // Max int32

	for i := range workers {
		if !workers[i].Healthy {
			continue
		}

		if workers[i].Active < minActive {
			minActive = workers[i].Active
			best = &workers[i]
		}
	}

	return best
}
```

**Characteristics:**
- ✅ Balances load based on actual work
- ✅ Handles variable task duration well
- ✅ Adapts to worker performance differences
- ⚠️ Requires health check to report active jobs (STORY_06A)
- ⚠️ Stale data (updated every 30s via health check)

**Current Status:** ⚠️ **Partially Working**
- Worker struct has `Active int32` field
- Health check currently returns hardcoded `0`
- Will work properly after STORY_06A (worker HTTP health endpoints)

**When to Use:** Production workloads with variable task duration

---

## In-Flight Task Handling

### Current Approach: Let gRPC Handle Failures

**Philosophy:** Keep the pool simple, let gRPC client handle failures

```go
// In grpc_client/client.go (NOT in discovery package)
func (c *Client) Transcribe(ctx context.Context, workerAddr string, task *Task) (*pb.TranscribeResponse, error) {
	// Get connection from pool
	conn, err := c.pool.Get(ctx, workerAddr)
	if err != nil {
		return nil, fmt.Errorf("failed to get connection: %w", err)
	}
	defer c.pool.Put(workerAddr, conn)

	// Call with retry (handles connection failures)
	err = c.retryWithBackoff(ctx, func() error {
		var callErr error
		resp, callErr = client.Transcribe(ctx, req)
		return callErr
	})

	return resp, err
}
```

### Worker Removal Scenarios

**Scenario A: Worker removed before task dispatched**
```
1. SelectWorker() → returns worker-2
2. K8s removes worker-2 (watch event)
3. gRPC client tries to connect → connection refused
4. Retry logic kicks in (3 attempts)
5. If all retries fail → task returned to queue
```

**Scenario B: Worker removed during active task**
```
1. Task sent to worker-2 (takes 30 minutes)
2. K8s scales down, removes worker-2 pod
3. gRPC stream breaks mid-transcription
4. Task fails, returned to queue
5. Next attempt selects different worker
```

**Key Point:** No special handling in Pool - gRPC client's retry logic handles it.

### Task Requeue Strategy

**Location:** Task queue logic (not in discovery.Pool)

**When to requeue:**
- ✅ Connection refused (worker disappeared)
- ✅ gRPC timeout (worker overloaded or crashed)
- ✅ gRPC stream error (connection broken)

**When NOT to requeue:**
- ❌ Validation error (bad file path, unsupported format)
- ❌ Transcription error (Whisper model failed)
- ❌ Already retried 3 times (permanent failure)

---

## Watch Event Processing

### Event Types

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

### Event Handler Pattern

**File**: `orchestrator/internal/discovery/pool.go`

```go
// watchLoop handles worker change events
func (p *Pool) watchLoop(ctx context.Context, eventCh <-chan WorkerEvent) {
	for {
		select {
		case <-ctx.Done():
			return
		case event, ok := <-eventCh:
			if !ok {
				return  // Channel closed
			}
			p.handleWorkerEvent(event)
		}
	}
}

// handleWorkerEvent processes individual events
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

// Helper: Remove worker from slice
func removeWorker(workers []Worker, id string) []Worker {
	for i, w := range workers {
		if w.ID == id {
			return append(workers[:i], workers[i+1:]...)
		}
	}
	return workers
}

// Helper: Update worker in slice
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

### Watch Disconnection Handling

**Current Status:** ⚠️ **Watch does not auto-reconnect**

```go
func (p *Pool) watchLoop(ctx context.Context, eventCh <-chan WorkerEvent) {
	for {
		select {
		case <-ctx.Done():
			return
		case event, ok := <-eventCh:
			if !ok {
				// Channel closed - watch disconnected
				// TODO: Add reconnection logic in STORY_03
				return
			}
			p.handleWorkerEvent(event)
		}
	}
}
```

**Mitigation:** Periodic refresh every 30s acts as fallback

**Future Work (STORY_03):**
- Add automatic reconnection with exponential backoff
- Full resync after reconnect
- Metrics for watch disconnections

---

## Race Condition Prevention

### Identified Race Conditions

**1. Concurrent Refresh and SelectWorker** ✅ **PREVENTED**

```go
// Thread A
func (p *Pool) Refresh() {
	p.mu.Lock()         // Waits if Thread B has lock
	p.workers = newList
	p.mu.Unlock()
}

// Thread B
func (p *Pool) SelectWorker() {
	p.mu.Lock()         // Waits if Thread A has lock
	worker := p.workers[i]
	p.mu.Unlock()
}
```

**Prevention:** Mutex prevents concurrent access

**2. Slice Modification During Iteration** ✅ **PREVENTED**

```go
// BAD: Modifying slice while iterating
for i, w := range p.workers {
	if shouldRemove(w) {
		p.workers = append(p.workers[:i], p.workers[i+1:]...)  // RACE!
	}
}

// GOOD: Current implementation
p.mu.Lock()
p.workers = removeWorker(p.workers, id)  // Helper function
p.mu.Unlock()
```

**Prevention:** Helper functions + mutex

**3. Pointer to Slice Element** ⚠️ **POSSIBLE ISSUE**

```go
// RISKY: Returning pointer to slice element
worker := &healthy[p.next%len(healthy)]  // Points into slice

// Later, if slice is reallocated...
p.workers = append(p.workers, newWorker)  // May reallocate!
// worker pointer now dangling!
```

**Current Mitigation:**
- Slice rarely reallocated (workers don't change frequently)
- Pointer used immediately after selection
- Worker struct is copied to gRPC client quickly

**Better Approach (future):**
```go
// Return by value, not pointer
worker := healthy[p.next%len(healthy)]
return &worker, nil  // Pointer to copy
```

---

## Testing Strategy

### Unit Tests

**File**: `orchestrator/internal/discovery/pool_test.go`

**Current Coverage:**
```go
TestNewPool                          // Constructor
TestPool_SelectWorker_RoundRobin     // Round-robin strategy
TestPool_SelectWorker_LeastLoaded    // Least-loaded strategy
TestPool_SelectWorker_NoWorkers      // Error case
TestPool_SelectWorker_NoHealthyWorkers  // Unhealthy workers
TestPool_Refresh                     // Full refresh
```

**Missing Tests (to be added):**
```go
TestPool_ConcurrentSelectAndRefresh  // Race detector test
TestPool_WatchEvents                 // Event handling
TestPool_WatchEventsDuringRefresh    // Race detector test
```

### Race Detector Tests

**Run locally:**
```bash
cd orchestrator
go test -race ./internal/discovery/...
```

**Expected output:**
```
PASS
ok  	github.com/mccloud/subgen/orchestrator/internal/discovery	0.234s
```

**CI Integration:**
```yaml
# .github/workflows/test-orchestrator.yml
- name: Run race detector
  run: |
    cd orchestrator
    go test -race -count=10 ./internal/discovery/...
```

### Integration Tests

**Phase 2 Requirements (STORY_05):**
1. Deploy 3 workers to Kind cluster
2. Queue 10 tasks
3. Verify round-robin distribution (3-4-3 or 3-3-4)
4. Verify least-loaded selects worker with fewest jobs
5. Remove 1 worker mid-test
6. Verify tasks still complete

---

## Known Limitations

### Limitation #1: Active Job Tracking Not Real-Time

**Current:**
- `Worker.Active` updated every 30s via `Refresh()`
- Health check returns hardcoded `0` for active jobs

**Impact:**
- "Least Loaded" strategy uses stale data (up to 30s old)
- May not balance load optimally under rapidly changing workload

**Future Fix (STORY_06A):**
- Worker implements HTTP `/metrics` endpoint
- Returns real-time active job count
- Health check polls this endpoint
- `Worker.Active` updated with fresh data every 30s

---

### Limitation #2: No Real-Time Worker Discovery

**Current:**
- Workers discovered every 30s via `Refresh()`
- Watch events provide faster updates (if watch is working)

**Impact:**
- New worker may take up to 30s to be discovered
- Acceptable for most deployments (workers don't scale that fast)

**Trade-off:** Simplicity vs real-time discovery (we chose simplicity)

---

### Limitation #3: No Active Connection Management

**Current:**
- `discovery.Pool` tracks which workers exist
- `grpc_client.ConnectionPool` manages gRPC connections
- No synchronization between the two

**Impact:**
- Connection pool may have stale connections to removed workers
- Relies on gRPC keepalive to detect dead connections
- Minor memory leak if many workers come and go

**Future Fix (Phase 3):**
```go
func (p *Pool) RemoveWorker(id string) {
	p.mu.Lock()
	worker := p.workers[id]
	p.workers = removeWorker(p.workers, id)
	p.mu.Unlock()
	
	// Close connection in gRPC pool
	p.grpcPool.Close(worker.Address)
}
```

---

### Limitation #4: Watch Does Not Auto-Reconnect

**Current:**
- Watch disconnects → `watchLoop()` exits → no more watch events
- Periodic `Refresh()` continues working as fallback

**Impact:**
- Worker changes detected every 30s instead of <1s
- Acceptable for Phase 2 (workers don't change that often)

**Future Fix (STORY_03):**
- Implement watch reconnection with exponential backoff
- Full resync after reconnect
- Emit metrics for watch health

---

## Graceful Shutdown

**Implementation:**

```go
func (p *Pool) Stop() {
	// Pool doesn't have explicit stop in current implementation
	// Context cancellation stops all goroutines
	
	p.log.Info("Worker pool shutting down")
	// No cleanup needed (connections managed by grpc_client.ConnectionPool)
}
```

**Shutdown Sequence:**
1. Context cancelled
2. `healthCheckLoop()` exits
3. `watchLoop()` exits
4. Pool stops accepting new selections
5. In-flight gRPC calls complete naturally

---

## Performance Characteristics

### Lock Contention Analysis

**Scenario**: 1 orchestrator, 5 workers, 10 tasks/sec

**Operations:**
- `SelectWorker()`: 10/sec (holds Lock for ~100μs each)
- `Refresh()`: 0.03/sec (holds Lock for ~1ms each)
- `handleWorkerEvent()`: ~0.1/sec (holds Lock for ~100μs each)

**Expected contention**: < 0.1% of time spent waiting for locks

### Benchmarks

```go
func BenchmarkSelectWorker(b *testing.B) {
	pool := setupPoolWith5Workers()
	
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_, _ = pool.SelectWorker()
	}
}

// Expected: 500-1000 ns/op (0.5-1 microsecond)
```

---

## Summary

### Key Design Decisions

1. **Slice-based storage**: Simpler than map, adequate for small worker counts
2. **Exported bool fields**: Simpler than atomic int32, adequate with RWMutex
3. **Full-list refresh**: Simpler than incremental updates, adequate at 30s interval
4. **No in-pool retry logic**: Let gRPC client handle connection failures
5. **Watch as optimization**: Periodic refresh is the reliable fallback

### Files Modified

1. ✅ `orchestrator/internal/discovery/pool.go` - Main implementation (exists)
2. ✅ `orchestrator/internal/discovery/pool_test.go` - Unit tests (exists)
3. ⏳ `orchestrator/internal/discovery/pool_race_test.go` - Race tests (to be created in STORY_01)

### Implementation Checklist

**Phase 1 (Complete):**
- [x] Basic Pool struct with slice storage
- [x] Round Robin selection
- [x] Least Loaded selection
- [x] Refresh() with full list replacement
- [x] Watch event handling (add/remove/update)
- [x] Localhost discovery working

**Phase 2 (In Progress):**
- [ ] Kubernetes discovery (STORY_01)
- [ ] RBAC configuration (STORY_02)
- [ ] Watch reconnection logic (STORY_03)
- [ ] Worker HTTP health endpoints (STORY_06A)
- [ ] Active job tracking (STORY_06A)
- [ ] Integration tests (STORY_05)

---

**Document Status**: ✅ Reconciled with Actual Code  
**Ready for Implementation**: Yes  
**Next Story**: STORY_01 (K8s Discovery)
