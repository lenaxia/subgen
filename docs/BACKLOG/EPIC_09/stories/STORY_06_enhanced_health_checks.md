# STORY_06: Enhanced Health Checks for Docker & Kubernetes

**Epic:** EPIC_09  
**Status:** Not Started  
**Assignee:** TBD  
**Effort:** 4-6 hours  
**Priority:** High (Required for both Docker and K8s deployments)

---

## User Story

As a **platform engineer**,  
I want **comprehensive health checks on both orchestrator and workers**,  
So that **Docker healthchecks and K8s probes can accurately determine service health**.

---

## Background

Current state:
- ✅ Orchestrator has `/health` and `/ready` endpoints
- ✅ Worker has gRPC HealthCheck method
- ❌ Worker lacks HTTP health endpoints (K8s prefers HTTP probes)
- ❌ Health checks need more depth (error rates, disk space, CPU)
- ❌ No K8s-friendly endpoint aliases (`/healthz`, `/livez`, `/readyz`)

User requirements:
1. Maintain Docker Compose support (non-K8s deployments)
2. Workers need deep health checks for K8s and orchestrator monitoring

---

## Acceptance Criteria

### Worker
- [ ] HTTP health server running on port 8080
- [ ] `/health` endpoint for liveness (returns 200 if alive)
- [ ] `/ready` endpoint for readiness (returns 503 if not ready)
- [ ] `/metrics` endpoint for detailed stats (JSON)
- [ ] Enhanced gRPC health check with new fields
- [ ] Worker tracks: errors, CPU, disk space, last job timestamp

### Orchestrator
- [ ] `/healthz` endpoint (alias to `/health`)
- [ ] `/livez` endpoint (alias to `/health`)
- [ ] `/readyz` endpoint (alias to `/ready`)
- [ ] Enhanced worker health monitoring

### Both Modes
- [ ] Docker Compose deployment tested
- [ ] K8s Phase 1 deployment tested
- [ ] K8s Phase 2 deployment tested
- [ ] Documentation complete

---

## Technical Design

### 1. Worker HTTP Health Server

**File**: `worker/src/http_server.py` (NEW)

```python
"""
HTTP health check server for Kubernetes probes.

Runs alongside gRPC server on separate port (8080).
Provides simple HTTP endpoints for liveness and readiness checks.
"""

import logging
import time
import psutil
import os
from flask import Flask, jsonify
from typing import TYPE_CHECKING

if TYPE_CHECKING:
    from grpc_server.service import TranscriptionService

logger = logging.getLogger(__name__)

app = Flask(__name__)
_service: 'TranscriptionService' = None


def init_health_server(service: 'TranscriptionService'):
    """Initialize health server with reference to gRPC service"""
    global _service
    _service = service
    logger.info("Health server initialized")


@app.route("/health", methods=["GET"])
def health():
    """
    Liveness probe - is the worker alive?
    
    Returns 200 if process is running.
    Never returns 5xx (otherwise K8s will restart pod).
    """
    return jsonify({
        "status": "alive",
        "timestamp": int(time.time())
    }), 200


@app.route("/ready", methods=["GET"])
def ready():
    """
    Readiness probe - can the worker accept new tasks?
    
    Returns:
        200 - Worker is ready to accept tasks
        503 - Worker is alive but not ready (don't send traffic)
    
    Checks:
        - Memory usage below threshold
        - No excessive consecutive errors
        - Model can be loaded (if needed)
        - Disk space available
    """
    if _service is None:
        return jsonify({
            "status": "not_ready",
            "reason": "service_not_initialized"
        }), 503
    
    # Check memory threshold
    if _service.stats["memory_mb"] > _service.config.system.memory_threshold_mb:
        return jsonify({
            "status": "not_ready",
            "reason": "memory_threshold_exceeded",
            "memory_mb": _service.stats["memory_mb"],
            "threshold_mb": _service.config.system.memory_threshold_mb
        }), 503
    
    # Check consecutive errors
    if _service.stats.get("consecutive_errors", 0) > 3:
        return jsonify({
            "status": "not_ready",
            "reason": "too_many_consecutive_errors",
            "consecutive_errors": _service.stats["consecutive_errors"]
        }), 503
    
    # Check disk space (models directory)
    model_path = _service.config.whisper.model_path
    if os.path.exists(model_path):
        stat = os.statvfs(model_path)
        free_mb = (stat.f_bavail * stat.f_frsize) / (1024 * 1024)
        if free_mb < 500:  # Less than 500MB free
            return jsonify({
                "status": "not_ready",
                "reason": "insufficient_disk_space",
                "free_mb": int(free_mb)
            }), 503
    
    # Worker is ready
    return jsonify({
        "status": "ready",
        "memory_mb": _service.stats["memory_mb"],
        "jobs_active": _service.stats["jobs_active"],
        "model_loaded": _service.model_manager.is_loaded(),
        "uptime_seconds": int(time.time() - _service.start_time)
    }), 200


@app.route("/metrics", methods=["GET"])
def metrics():
    """
    Detailed metrics endpoint for monitoring systems.
    
    Returns comprehensive worker statistics in JSON format.
    """
    if _service is None:
        return jsonify({"error": "service_not_initialized"}), 503
    
    # Get CPU and memory
    process = psutil.Process()
    cpu_percent = process.cpu_percent(interval=0.1)
    memory_mb = int(process.memory_info().rss / (1024 * 1024))
    
    # Get disk space
    model_path = _service.config.whisper.model_path
    disk_available_mb = 0
    if os.path.exists(model_path):
        stat = os.statvfs(model_path)
        disk_available_mb = int((stat.f_bavail * stat.f_frsize) / (1024 * 1024))
    
    return jsonify({
        # Resource usage
        "memory_mb": memory_mb,
        "memory_threshold_mb": _service.config.system.memory_threshold_mb,
        "cpu_percent": cpu_percent,
        "disk_available_mb": disk_available_mb,
        
        # Model state
        "model_loaded": _service.model_manager.is_loaded(),
        "model_name": _service.config.whisper.model_name,
        
        # Job statistics
        "jobs_processed": _service.stats["jobs_processed"],
        "jobs_active": _service.stats["jobs_active"],
        "jobs_failed": _service.stats.get("jobs_failed", 0),
        "consecutive_errors": _service.stats.get("consecutive_errors", 0),
        "last_job_timestamp": _service.stats.get("last_job_timestamp"),
        
        # System info
        "uptime_seconds": int(time.time() - _service.start_time),
        "version": _service.config.version,
        "pid": os.getpid(),
    }), 200


def run_health_server(host: str = "0.0.0.0", port: int = 8080):
    """
    Run health check HTTP server.
    
    Should be run in a separate thread from gRPC server.
    """
    logger.info(f"Starting health server on {host}:{port}")
    app.run(host=host, port=port, threaded=True)
```

**Integration in main.py**:

```python
# worker/src/main.py

import threading
from http_server import init_health_server, run_health_server

# After creating service
service = TranscriptionService(config)

# Initialize health server
init_health_server(service)

# Start health server in background thread
health_thread = threading.Thread(
    target=run_health_server,
    args=("0.0.0.0", 8080),
    daemon=True
)
health_thread.start()
logger.info("Health server started on port 8080")

# Start gRPC server (main thread)
server.serve()
```

---

### 2. Enhanced Worker gRPC Health Check

**File**: `worker/proto/worker.proto`

```protobuf
message HealthCheckResponse {
    enum Status {
        HEALTHY = 0;
        UNHEALTHY = 1;
        DEGRADED = 2;  // Can work but not optimal
    }
    
    Status status = 1;
    int32 memory_mb = 2;
    bool model_loaded = 3;
    int32 jobs_processed = 4;
    int32 jobs_active = 5;
    string version = 6;
    int64 uptime_seconds = 7;
    
    // NEW FIELDS
    int32 jobs_failed = 8;               // Total failed jobs
    int32 consecutive_errors = 9;        // Recent error streak (resets on success)
    int64 last_job_timestamp = 10;       // Unix timestamp of last completed job
    float cpu_percent = 11;              // Current CPU usage percentage
    int64 disk_available_mb = 12;        // Free space in models directory
    string degraded_reason = 13;         // Why worker is degraded (if status=DEGRADED)
}
```

**Update service.py**:

```python
def HealthCheck(self, request, context):
    """Enhanced health check with additional metrics"""
    
    # Get process stats
    process = psutil.Process()
    memory_mb = int(process.memory_info().rss / (1024 * 1024))
    cpu_percent = process.cpu_percent(interval=0.1)
    
    # Get disk space
    disk_available_mb = 0
    if os.path.exists(self.config.whisper.model_path):
        stat = os.statvfs(self.config.whisper.model_path)
        disk_available_mb = int((stat.f_bavail * stat.f_frsize) / (1024 * 1024))
    
    # Determine status
    degraded_reason = ""
    if memory_mb > self.config.system.memory_threshold_mb:
        status = transcription_pb2.HealthCheckResponse.UNHEALTHY
    elif self.stats.get("consecutive_errors", 0) > 3:
        status = transcription_pb2.HealthCheckResponse.DEGRADED
        degraded_reason = f"consecutive_errors={self.stats['consecutive_errors']}"
    elif disk_available_mb < 500:
        status = transcription_pb2.HealthCheckResponse.DEGRADED
        degraded_reason = f"low_disk_space={disk_available_mb}MB"
    else:
        status = transcription_pb2.HealthCheckResponse.HEALTHY
    
    # Calculate uptime
    uptime = int(time.time() - self.start_time) if self.start_time else 0
    
    return transcription_pb2.HealthCheckResponse(
        status=status,
        memory_mb=memory_mb,
        model_loaded=self.model_manager.is_loaded(),
        jobs_processed=self.stats["jobs_processed"],
        jobs_active=self.stats["jobs_active"],
        version=self.config.version,
        uptime_seconds=uptime,
        # New fields
        jobs_failed=self.stats.get("jobs_failed", 0),
        consecutive_errors=self.stats.get("consecutive_errors", 0),
        last_job_timestamp=self.stats.get("last_job_timestamp", 0),
        cpu_percent=cpu_percent,
        disk_available_mb=disk_available_mb,
        degraded_reason=degraded_reason,
    )
```

**Track stats in Transcribe method**:

```python
def Transcribe(self, request, context):
    try:
        # ... transcription logic ...
        
        # On success
        self.stats["jobs_processed"] += 1
        self.stats["consecutive_errors"] = 0  # Reset on success
        self.stats["last_job_timestamp"] = int(time.time())
        
    except Exception as e:
        # On failure
        self.stats["jobs_failed"] = self.stats.get("jobs_failed", 0) + 1
        self.stats["consecutive_errors"] = self.stats.get("consecutive_errors", 0) + 1
        raise
```

---

### 3. Orchestrator K8s-Friendly Endpoints

**File**: `orchestrator/internal/observability/observability.go`

```go
// RegisterHealthEndpoints adds health endpoints with K8s aliases
func RegisterHealthEndpoints(
	app *fiber.App,
	metrics *Metrics,
	pool WorkerPool,
	queue Queue,
	startTime time.Time,
	log *logrus.Logger,
) {
	// Primary endpoints
	app.Get("/health", healthHandler(startTime))
	app.Get("/ready", readyHandler(pool, queue))
	app.Get("/queue", queueHandler(queue))
	
	// K8s-friendly aliases
	app.Get("/healthz", healthHandler(startTime))   // K8s convention
	app.Get("/livez", healthHandler(startTime))     // K8s convention
	app.Get("/readyz", readyHandler(pool, queue))   // K8s convention
}

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

func readyHandler(pool WorkerPool, queue Queue) fiber.Handler {
	return func(c *fiber.Ctx) error {
		workers, err := pool.GetWorkers()
		if err != nil || len(workers) == 0 {
			return c.Status(fiber.StatusServiceUnavailable).JSON(fiber.Map{
				"status": "not_ready",
				"reason": "no_workers_available",
			})
		}
		
		// Check if at least one worker is healthy
		healthyCount := 0
		for _, w := range workers {
			if w.Healthy {
				healthyCount++
			}
		}
		
		if healthyCount == 0 {
			return c.Status(fiber.StatusServiceUnavailable).JSON(fiber.Map{
				"status": "not_ready",
				"reason": "no_healthy_workers",
			})
		}
		
		return c.JSON(fiber.Map{
			"status":          "ready",
			"workers_total":   len(workers),
			"workers_healthy": healthyCount,
			"queue_size":      queue.Size(),
			"processing":      queue.ProcessingCount(),
		})
	}
}
```

---

## Docker Compose Configuration

**Update**: `docker-compose.yml`

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

  worker:
    # ... existing config ...
    ports:
      - "8080:8080"  # Expose health endpoint
    healthcheck:
      test: ["CMD", "curl", "-f", "http://localhost:8080/health"]
      interval: 30s
      timeout: 5s
      retries: 3
      start_period: 60s  # Allow time for model download
```

---

## Kubernetes Probe Configuration

**Update**: `deploy/values-phase2-workers.yaml`

```yaml
containers:
  worker:
    ports:
      - name: grpc
        containerPort: 50051
      - name: http-health  # NEW
        containerPort: 8080
    
    probes:
      liveness:
        enabled: true
        type: HTTP        # Changed from GRPC
        port: 8080        # HTTP port
        path: /health
        spec:
          initialDelaySeconds: 30
          periodSeconds: 30
          timeoutSeconds: 5
          failureThreshold: 3
      
      readiness:
        enabled: true
        type: HTTP        # Changed from GRPC
        port: 8080        # HTTP port
        path: /ready
        spec:
          initialDelaySeconds: 10
          periodSeconds: 10
          timeoutSeconds: 3
          failureThreshold: 3
      
      startup:
        enabled: true
        type: HTTP        # Changed from GRPC
        port: 8080        # HTTP port
        path: /health
        spec:
          initialDelaySeconds: 10
          periodSeconds: 10
          timeoutSeconds: 5
          failureThreshold: 30  # 5 min for model download
```

---

## Testing Strategy

### Unit Tests

**Worker** (`worker/tests/test_http_server.py`):
```python
def test_health_endpoint_returns_200():
    """Test liveness probe"""
    response = client.get("/health")
    assert response.status_code == 200
    assert response.json["status"] == "alive"

def test_ready_endpoint_when_ready():
    """Test readiness when worker is healthy"""
    # Setup: memory below threshold, no errors
    response = client.get("/ready")
    assert response.status_code == 200
    assert response.json["status"] == "ready"

def test_ready_endpoint_when_not_ready():
    """Test readiness when worker has issues"""
    # Setup: memory above threshold
    response = client.get("/ready")
    assert response.status_code == 503
    assert response.json["reason"] == "memory_threshold_exceeded"
```

**Orchestrator** (`orchestrator/internal/observability/observability_test.go`):
```go
func TestHealthzEndpoint(t *testing.T) {
	app := fiber.New()
	RegisterHealthEndpoints(app, ...)
	
	req := httptest.NewRequest("GET", "/healthz", nil)
	resp, _ := app.Test(req)
	
	assert.Equal(t, 200, resp.StatusCode)
}

func TestReadyzWithNoWorkers(t *testing.T) {
	// Test returns 503 when no workers available
}
```

### Integration Tests

```bash
#!/bin/bash
# test/health-checks/test-docker-compose.sh

echo "Testing Docker Compose health checks..."

# Start services
docker-compose up -d

# Wait for startup
sleep 30

# Test orchestrator health
curl -f http://localhost:9000/health || exit 1
curl -f http://localhost:9000/healthz || exit 1
curl -f http://localhost:9000/ready || exit 1

# Test worker health
curl -f http://localhost:8080/health || exit 1
curl -f http://localhost:8080/ready || exit 1
curl -f http://localhost:8080/metrics || exit 1

echo "✅ All health checks passed"
```

---

## Documentation

Create `docs/DEPLOYMENT/health-checks.md`:

```markdown
# Health Check Endpoints

## Orchestrator Endpoints

### GET /health (or /healthz, /livez)
**Purpose**: Liveness check  
**Returns**: 200 if orchestrator is alive

**Example**:
```bash
curl http://orchestrator:9000/health
# {"status":"healthy","version":"v0.1.9","uptime":"2h30m15s"}
```

### GET /ready (or /readyz)
**Purpose**: Readiness check  
**Returns**: 200 if ready, 503 if not ready

**Ready when**:
- At least one worker available
- At least one worker healthy

**Example**:
```bash
curl http://orchestrator:9000/ready
# {"status":"ready","workers_total":3,"workers_healthy":3,"queue_size":0,"processing":0}
```

## Worker Endpoints

### GET /health (HTTP port 8080)
**Purpose**: Liveness check  
**Returns**: Always 200 (process is alive)

### GET /ready (HTTP port 8080)
**Purpose**: Readiness check  
**Returns**: 200 if ready, 503 if not ready

**Not ready when**:
- Memory exceeds threshold
- 3+ consecutive errors
- Disk space < 500MB

### GET /metrics (HTTP port 8080)
**Purpose**: Detailed monitoring stats  
**Returns**: JSON with comprehensive metrics

**Fields**: memory_mb, cpu_percent, model_loaded, jobs_processed, jobs_active, jobs_failed, etc.
```

---

## Definition of Done

- [ ] Worker HTTP health server implemented
- [ ] Worker `/health`, `/ready`, `/metrics` endpoints working
- [ ] Enhanced gRPC health check with new fields
- [ ] Orchestrator `/healthz`, `/livez`, `/readyz` aliases added
- [ ] Docker Compose healthchecks configured
- [ ] K8s HTTP probes configured
- [ ] Unit tests passing (orchestrator + worker)
- [ ] Integration tests passing (Docker + K8s)
- [ ] Documentation complete
- [ ] Both Docker and K8s modes tested
- [ ] Work log created

---

**Story Owner:** TBD  
**Created:** 2026-02-17  
**Related Work Log:** 0078_2026-02-17_docker_support_and_deep_health_checks.md
