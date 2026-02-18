# STORY_06B: Orchestrator Health Check Enhancements

**Epic:** EPIC_09  
**Status:** ✅ COMPLETE (2026-02-17)  
**Assignee:** OpenCode AI  
**Effort:** 2-3 hours (Actual: ~30 minutes - most features already existed!)  
**Priority:** Medium (Nice-to-have for K8s)  
**Parent Story:** STORY_06 (Enhanced Health Checks) - Split into 06A and 06B  
**Work Log:** [0085_2026-02-17_epic_09_story_06b_complete.md](../../../WORKLOGS/0085_2026-02-17_epic_09_story_06b_complete.md)

---

## ✅ Completion Summary

**Discovery:** The orchestrator already had excellent health check implementations! The `/ready` endpoint **already validates worker availability and queue status**. Work focused on:
1. Adding K8s-friendly aliases (`/healthz`, `/livez`, `/readyz`)
2. Updating K8s probe configuration to use aliases
3. Adding startup probe for better initialization handling
4. Documentation

**Key Achievement:** All K8s naming conventions followed, readiness validates workers properly, backward compatible.

---

## User Story

As a **platform engineer**,  
I want **K8s-friendly health check aliases on the orchestrator**,  
So that **K8s probes follow standard conventions and readiness checks include worker availability**.

---

## Scope

This story focuses **ONLY on the orchestrator** Go service. Worker health checks are handled in STORY_06A.

**Note**: This story is **lower priority** than STORY_06A because:
- Orchestrator already has `/health` and `/ready` endpoints (working)
- This story just adds K8s-friendly aliases and enhances readiness logic
- Not blocking for Phase 2 functionality

---

## Acceptance Criteria

### K8s-Friendly Aliases
- [x] `/healthz` endpoint (alias to `/health`)
- [x] `/livez` endpoint (alias to `/live`)
- [x] `/readyz` endpoint (alias to `/ready`)
- [x] All aliases return same response format as originals

### Enhanced Readiness Check
- [x] `/ready` returns 503 if no workers available (already implemented!)
- [x] `/ready` returns 503 if all workers unhealthy (already implemented via SelectWorker!)
- [x] `/ready` returns 200 with worker count metrics (already implemented!)
- [x] `/ready` includes queue status (size, processing) (already implemented!)

### K8s Probe Configuration
- [x] Orchestrator K8s probes updated to use new endpoints
- [x] Added startup probe for better initialization
- [x] Documentation includes probe configuration examples

---

## Technical Design

### Current State

Orchestrator already has these endpoints (from EPIC_01):
- `/health` - Returns 200 with uptime
- `/ready` - Returns 200 (basic check)
- `/queue` - Returns queue stats

**What's missing**:
- K8s standard aliases (`/healthz`, `/livez`, `/readyz`)
- Readiness check doesn't validate worker availability

---

### Implementation

#### 1. Add Endpoint Aliases

**File**: `orchestrator/internal/api/health.go`

```go
package api

import (
	"time"

	"github.com/gofiber/fiber/v2"
	"github.com/lenaxia/subgen/orchestrator/internal/discovery"
	"github.com/lenaxia/subgen/orchestrator/internal/queue"
)

// RegisterHealthEndpoints adds health check endpoints with K8s aliases
func RegisterHealthEndpoints(
	app *fiber.App,
	pool *discovery.WorkerPool,
	q *queue.Queue,
	startTime time.Time,
) {
	// Primary endpoints (existing)
	app.Get("/health", healthHandler(startTime))
	app.Get("/ready", readyHandler(pool, q))
	app.Get("/queue", queueHandler(q))
	
	// K8s-friendly aliases (NEW)
	app.Get("/healthz", healthHandler(startTime))   // K8s liveness standard
	app.Get("/livez", healthHandler(startTime))     // K8s liveness standard
	app.Get("/readyz", readyHandler(pool, q))       // K8s readiness standard
}

// healthHandler returns liveness status (is orchestrator alive?)
func healthHandler(startTime time.Time) fiber.Handler {
	return func(c *fiber.Ctx) error {
		uptime := time.Since(startTime)
		
		return c.JSON(fiber.Map{
			"status":  "healthy",
			"version": "v0.1.9",
			"uptime":  uptime.String(),
		})
	}
}

// readyHandler returns readiness status (can orchestrator accept webhooks?)
func readyHandler(pool *discovery.WorkerPool, q *queue.Queue) fiber.Handler {
	return func(c *fiber.Ctx) error {
		// Check if workers are available
		workers, err := pool.ListWorkers()
		if err != nil || len(workers) == 0 {
			return c.Status(fiber.StatusServiceUnavailable).JSON(fiber.Map{
				"status": "not_ready",
				"reason": "no_workers_available",
			})
		}
		
		// Count healthy workers
		healthyCount := 0
		for _, w := range workers {
			if w.Healthy {
				healthyCount++
			}
		}
		
		if healthyCount == 0 {
			return c.Status(fiber.StatusServiceUnavailable).JSON(fiber.Map{
				"status":         "not_ready",
				"reason":         "no_healthy_workers",
				"workers_total":  len(workers),
				"workers_healthy": 0,
			})
		}
		
		// Check if queue is not overloaded
		queueSize := q.Size()
		queueMax := q.MaxSize()
		
		if queueSize >= queueMax {
			return c.Status(fiber.StatusServiceUnavailable).JSON(fiber.Map{
				"status":          "not_ready",
				"reason":          "queue_full",
				"workers_total":   len(workers),
				"workers_healthy": healthyCount,
				"queue_size":      queueSize,
				"queue_max":       queueMax,
			})
		}
		
		// Orchestrator is ready
		return c.JSON(fiber.Map{
			"status":          "ready",
			"workers_total":   len(workers),
			"workers_healthy": healthyCount,
			"queue_size":      queueSize,
			"queue_max":       queueMax,
			"processing":      q.ProcessingCount(),
		})
	}
}

// queueHandler returns queue statistics (existing, no changes)
func queueHandler(q *queue.Queue) fiber.Handler {
	return func(c *fiber.Ctx) error {
		return c.JSON(fiber.Map{
			"size":       q.Size(),
			"max_size":   q.MaxSize(),
			"processing": q.ProcessingCount(),
		})
	}
}
```

---

#### 2. Update Main to Use New Registration

**File**: `orchestrator/cmd/orchestrator/main.go`

```go
// Register health endpoints
api.RegisterHealthEndpoints(app, workerPool, taskQueue, startTime)
```

---

#### 3. Kubernetes Probe Configuration

**File**: `deploy/values-phase2-orchestrator.yaml`

```yaml
controllers:
  main:
    containers:
      orchestrator:
        ports:
          - name: http
            containerPort: 9000
            protocol: TCP
          - name: metrics
            containerPort: 9090
            protocol: TCP
        
        probes:
          liveness:
            enabled: true
            type: HTTP
            port: 9000
            path: /healthz   # Changed from /health
            spec:
              initialDelaySeconds: 10
              periodSeconds: 30
              timeoutSeconds: 5
              failureThreshold: 3
          
          readiness:
            enabled: true
            type: HTTP
            port: 9000
            path: /readyz    # Changed from /ready
            spec:
              initialDelaySeconds: 5
              periodSeconds: 10
              timeoutSeconds: 3
              failureThreshold: 3
          
          startup:
            enabled: true
            type: HTTP
            port: 9000
            path: /healthz   # Liveness check
            spec:
              initialDelaySeconds: 5
              periodSeconds: 5
              timeoutSeconds: 3
              failureThreshold: 12  # 60 seconds max
```

---

#### 4. Docker Compose Configuration

**File**: `docker-compose.yml`

```yaml
services:
  orchestrator:
    # ... existing config ...
    healthcheck:
      test: ["CMD", "curl", "-f", "http://localhost:9000/health"]
      interval: 30s
      timeout: 5s
      retries: 3
      start_period: 10s
```

---

## Testing Strategy

### Unit Tests

**File**: `orchestrator/internal/api/health_test.go`

```go
package api

import (
	"net/http/httptest"
	"testing"
	"time"

	"github.com/gofiber/fiber/v2"
	"github.com/stretchr/testify/assert"
)

func TestHealthEndpoint(t *testing.T) {
	app := fiber.New()
	startTime := time.Now()
	
	RegisterHealthEndpoints(app, nil, nil, startTime)
	
	req := httptest.NewRequest("GET", "/health", nil)
	resp, _ := app.Test(req)
	
	assert.Equal(t, 200, resp.StatusCode)
}

func TestHealthzAlias(t *testing.T) {
	app := fiber.New()
	startTime := time.Now()
	
	RegisterHealthEndpoints(app, nil, nil, startTime)
	
	// Test /healthz alias
	req := httptest.NewRequest("GET", "/healthz", nil)
	resp, _ := app.Test(req)
	
	assert.Equal(t, 200, resp.StatusCode)
}

func TestLivezAlias(t *testing.T) {
	app := fiber.New()
	startTime := time.Now()
	
	RegisterHealthEndpoints(app, nil, nil, startTime)
	
	// Test /livez alias
	req := httptest.NewRequest("GET", "/livez", nil)
	resp, _ := app.Test(req)
	
	assert.Equal(t, 200, resp.StatusCode)
}

func TestReadyWithNoWorkers(t *testing.T) {
	app := fiber.New()
	
	// Mock pool with no workers
	mockPool := &MockWorkerPool{
		workers: []*Worker{},
	}
	mockQueue := &MockQueue{}
	
	RegisterHealthEndpoints(app, mockPool, mockQueue, time.Now())
	
	req := httptest.NewRequest("GET", "/ready", nil)
	resp, _ := app.Test(req)
	
	// Should return 503
	assert.Equal(t, 503, resp.StatusCode)
}

func TestReadyWithUnhealthyWorkers(t *testing.T) {
	app := fiber.New()
	
	// Mock pool with unhealthy workers
	mockPool := &MockWorkerPool{
		workers: []*Worker{
			{ID: "worker-1", Healthy: false},
			{ID: "worker-2", Healthy: false},
		},
	}
	mockQueue := &MockQueue{}
	
	RegisterHealthEndpoints(app, mockPool, mockQueue, time.Now())
	
	req := httptest.NewRequest("GET", "/ready", nil)
	resp, _ := app.Test(req)
	
	// Should return 503
	assert.Equal(t, 503, resp.StatusCode)
}

func TestReadyWithHealthyWorkers(t *testing.T) {
	app := fiber.New()
	
	// Mock pool with healthy workers
	mockPool := &MockWorkerPool{
		workers: []*Worker{
			{ID: "worker-1", Healthy: true},
			{ID: "worker-2", Healthy: true},
		},
	}
	mockQueue := &MockQueue{size: 10, maxSize: 1000}
	
	RegisterHealthEndpoints(app, mockPool, mockQueue, time.Now())
	
	req := httptest.NewRequest("GET", "/ready", nil)
	resp, _ := app.Test(req)
	
	// Should return 200
	assert.Equal(t, 200, resp.StatusCode)
}

func TestReadyzAlias(t *testing.T) {
	app := fiber.New()
	
	mockPool := &MockWorkerPool{
		workers: []*Worker{{ID: "worker-1", Healthy: true}},
	}
	mockQueue := &MockQueue{size: 0, maxSize: 1000}
	
	RegisterHealthEndpoints(app, mockPool, mockQueue, time.Now())
	
	// Test /readyz alias
	req := httptest.NewRequest("GET", "/readyz", nil)
	resp, _ := app.Test(req)
	
	assert.Equal(t, 200, resp.StatusCode)
}

func TestReadyQueueFull(t *testing.T) {
	app := fiber.New()
	
	mockPool := &MockWorkerPool{
		workers: []*Worker{{ID: "worker-1", Healthy: true}},
	}
	mockQueue := &MockQueue{size: 1000, maxSize: 1000}  // Queue full
	
	RegisterHealthEndpoints(app, mockPool, mockQueue, time.Now())
	
	req := httptest.NewRequest("GET", "/ready", nil)
	resp, _ := app.Test(req)
	
	// Should return 503 when queue is full
	assert.Equal(t, 503, resp.StatusCode)
}
```

---

### Integration Tests

**File**: `test/health-checks/test-orchestrator-http.sh`

```bash
#!/bin/bash
set -e

echo "Testing orchestrator HTTP health endpoints..."

# Start services
docker-compose up -d

# Wait for startup
sleep 10

# Test health endpoint
echo "Testing /health endpoint..."
curl -f http://localhost:9000/health || exit 1
echo "✅ /health passed"

# Test healthz alias
echo "Testing /healthz alias..."
curl -f http://localhost:9000/healthz || exit 1
echo "✅ /healthz passed"

# Test livez alias
echo "Testing /livez alias..."
curl -f http://localhost:9000/livez || exit 1
echo "✅ /livez passed"

# Test ready endpoint
echo "Testing /ready endpoint..."
READY=$(curl -f http://localhost:9000/ready)
echo "$READY" | jq -e '.status == "ready"' || exit 1
echo "$READY" | jq -e '.workers_total > 0' || exit 1
echo "✅ /ready passed"

# Test readyz alias
echo "Testing /readyz alias..."
curl -f http://localhost:9000/readyz || exit 1
echo "✅ /readyz passed"

echo "✅ All orchestrator health check tests passed"
```

---

## Documentation

### Update Deployment Guide

**File**: `docs/DEPLOYMENT/health-checks.md`

Add section:

```markdown
## Orchestrator Health Endpoints

### Liveness Checks

**Endpoints**: `/health`, `/healthz`, `/livez`  
**Purpose**: Check if orchestrator process is alive  
**Returns**: Always 200 (unless process crashed)

**Example**:
```bash
curl http://orchestrator:9000/healthz
# {"status":"healthy","version":"v0.1.9","uptime":"2h30m15s"}
```

**Use in K8s**:
```yaml
livenessProbe:
  httpGet:
    path: /healthz
    port: 9000
  initialDelaySeconds: 10
  periodSeconds: 30
```

### Readiness Checks

**Endpoints**: `/ready`, `/readyz`  
**Purpose**: Check if orchestrator can accept webhooks  
**Returns**: 
- 200 if ready (has healthy workers, queue not full)
- 503 if not ready

**Not ready when**:
- No workers available
- All workers unhealthy
- Queue is full

**Example**:
```bash
curl http://orchestrator:9000/readyz
# {"status":"ready","workers_total":3,"workers_healthy":3,"queue_size":0,"queue_max":1000,"processing":0}
```

**Use in K8s**:
```yaml
readinessProbe:
  httpGet:
    path: /readyz
    port: 9000
  initialDelaySeconds: 5
  periodSeconds: 10
```
```

---

## Definition of Done

- [ ] `orchestrator/internal/api/health.go` enhanced with aliases and readiness logic
- [ ] Unit tests passing (8+ tests, 80%+ coverage)
- [ ] Integration tests passing (Docker Compose)
- [ ] K8s probe configuration updated
- [ ] Documentation updated (health-checks.md)
- [ ] Work log created

---

**Story Owner:** TBD  
**Created:** 2026-02-17  
**Related Stories:** STORY_06A (Worker HTTP Health)  
**Priority Note:** This story can be done later - STORY_06A is higher priority
