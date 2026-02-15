# Memory Management & Leak Prevention

**Document Version:** 1.0  
**Last Updated:** 2026-02-15  
**Status:** Draft  
**Related Documents:**
- [00_HYBRID_ARCHITECTURE.md](./00_HYBRID_ARCHITECTURE.md)
- [01_GRPC_PROTOCOL.md](./01_GRPC_PROTOCOL.md)

---

## Table of Contents

1. [Problem Statement](#problem-statement)
2. [Root Cause Analysis](#root-cause-analysis)
3. [Go Orchestrator Memory Management](#go-orchestrator-memory-management)
4. [Python Worker Memory Management](#python-worker-memory-management)
5. [Memory Monitoring](#memory-monitoring)
6. [Testing Strategy](#testing-strategy)
7. [Kubernetes Memory Limits](#kubernetes-memory-limits)

---

## Problem Statement

### Current Subgen Memory Leaks

**Confirmed Memory Leaks in `subgen.py`:**

1. **Unbounded `task_results` dictionary** (`subgen.py:234-236, 748-751`)
   - Global dictionary that grows indefinitely
   - Never cleaned up
   - Each transcription adds entry (task_id → result)
   - In production: 1,000 transcriptions = 1,000+ dict entries = GBs of memory

2. **Whisper model cleanup race condition** (`subgen.py:1165-1197`)
   - Timer-based cleanup can be delayed indefinitely
   - If new request arrives, timer resets
   - Model stays in memory forever

3. **BytesIO objects not closed** (`subgen.py:1100-1142`)
   - Audio extraction creates BytesIO buffers
   - Not explicitly closed
   - Python GC eventually cleans up, but not guaranteed

**Impact:**

```
Memory Usage Over Time (Current subgen.py):

8GB  │                                     ╱──────  OOM Kill
     │                                   ╱
6GB  │                              ╱─────
     │                          ╱────
4GB  │                    ╱──────
     │              ╱──────
2GB  │        ╱──────
     │  ╱──────
0GB  └──────────────────────────────────────────
     0    100   200   300   400   500   600   700
                  Number of Transcriptions
```

**Goal:** Prevent all memory leaks in new architecture.

---

## Root Cause Analysis

### Leak #1: Unbounded `task_results` Dictionary

**Location:** `subgen.py:234-236, 748-751`

```python
# PROBLEMATIC CODE
task_results = {}  # Global dict

def add_to_queue(task):
    task_id = str(uuid.uuid4())
    task_results[task_id] = {  # ← Never removed!
        'status': 'queued',
        'progress': 0,
        'result': None
    }
    queue.put((priority, task_id, task))
```

**Why it leaks:**

1. Every task creates entry in `task_results`
2. No cleanup logic after task completes
3. Dict grows unboundedly: 1 task/day × 365 days = 365 entries/year
4. Each entry holds references to task data (file paths, options, results)

**Why it exists:**

- Web UI progress tracking (`/status/{task_id}`)
- Not critical for core functionality

**Solution:**

- **Go orchestrator:** No `task_results` dict needed (stateless workers)
- **Queue state:** Tracked in orchestrator's in-memory queue (bounded size)
- **Completed tasks:** Logged to structured logs, not stored in memory

---

### Leak #2: Whisper Model Cleanup Race

**Location:** `subgen.py:1165-1197`

```python
# PROBLEMATIC CODE
model_cleanup_timer = None

def transcribe_audio(file_path, options):
    global model_cleanup_timer
    
    # Cancel existing timer if new request arrives
    if model_cleanup_timer:
        model_cleanup_timer.cancel()  # ← Delays cleanup indefinitely
    
    # Load model (if not loaded)
    load_whisper_model()
    
    # Transcribe
    result = model.transcribe(file_path)
    
    # Schedule cleanup in 30 seconds
    model_cleanup_timer = threading.Timer(30.0, unload_model)
    model_cleanup_timer.start()
```

**Why it leaks:**

1. Timer resets on every new request
2. In busy periods, timer never fires
3. Model stays in memory forever (2-4GB depending on model size)

**Why it exists:**

- Avoid reloading model for every request (slow: 10-30s to load)
- Keep model "warm" for next request

**Solution:**

- **Keep delay mechanism** (good for performance)
- **Add hard limit:** If memory > threshold, unload immediately
- **Context managers:** Ensure cleanup even on errors
- **Graceful shutdown:** Always unload model on worker shutdown

---

### Leak #3: BytesIO Not Closed

**Location:** `subgen.py:1100-1142`

```python
# PROBLEMATIC CODE
def extract_audio(video_path):
    container = av.open(video_path)
    audio_stream = container.streams.audio[0]
    
    # Extract audio to BytesIO
    audio_buffer = io.BytesIO()  # ← Not explicitly closed
    
    for packet in container.demux(audio_stream):
        for frame in packet.decode():
            audio_buffer.write(frame.to_ndarray().tobytes())
    
    return audio_buffer  # ← Caller may not close it
```

**Why it leaks:**

1. BytesIO not closed → holds reference to buffer
2. Python GC eventually cleans up, but not deterministic
3. In heavy load, GC can't keep up → memory accumulates

**Solution:**

- **Context managers:** Use `with` statements everywhere
- **Explicit cleanup:** Close all file handles, buffers, streams
- **RAII pattern:** Resource Acquisition Is Initialization

---

## Go Orchestrator Memory Management

### Design Principles

1. **No unbounded data structures**
2. **Context-based lifecycle management**
3. **Explicit cleanup with `defer`**
4. **Memory-efficient queue (bounded size)**
5. **Worker state tracked externally (not in orchestrator)**

---

### Queue Design: Bounded Priority Queue

**Problem Avoided:** Unbounded queue = memory leak

**Solution:**

```go
// internal/queue/queue.go

type PriorityQueue struct {
    mu        sync.RWMutex
    heap      *taskHeap
    maxSize   int              // ← Bounded!
    tasks     map[string]*Task // Deduplication map
}

func (q *PriorityQueue) Push(task *Task) error {
    q.mu.Lock()
    defer q.mu.Unlock()
    
    // Check size limit
    if len(q.heap.items) >= q.maxSize {
        return ErrQueueFull
    }
    
    // Check for duplicate
    if existing, ok := q.tasks[task.ID]; ok {
        // Update priority if higher
        if task.Priority > existing.Priority {
            q.heap.update(existing, task.Priority)
        }
        return nil
    }
    
    // Add to queue
    heap.Push(q.heap, task)
    q.tasks[task.ID] = task
    return nil
}

func (q *PriorityQueue) Pop() *Task {
    q.mu.Lock()
    defer q.mu.Unlock()
    
    if q.heap.Len() == 0 {
        return nil
    }
    
    task := heap.Pop(q.heap).(*Task)
    
    // Remove from dedup map
    delete(q.tasks, task.ID)  // ← Explicit cleanup!
    
    return task
}
```

**Memory Bounds:**

- Max queue size: 1,000 tasks (configurable)
- Each task: ~1KB
- Total memory: ~1MB (negligible)

---

### No Task Results Storage

**Current subgen.py:** Stores results in `task_results` dict  
**New orchestrator:** No storage, just logging

```go
// internal/webhooks/handler.go

func (h *WebhookHandler) HandlePlex(c *fiber.Ctx) error {
    // Parse webhook
    payload := parsePlexWebhook(c.Body())
    
    // Create task
    task := &queue.Task{
        ID:       uuid.New().String(),
        FilePath: payload.FilePath,
        Priority: queue.PriorityNormal,
        Options:  payload.Options,
    }
    
    // Add to queue
    if err := h.queue.Push(task); err != nil {
        return c.Status(503).JSON(fiber.Map{"error": "queue full"})
    }
    
    // Log and return immediately
    log.WithFields(logrus.Fields{
        "task_id":   task.ID,
        "file_path": redactPath(task.FilePath),
    }).Info("task queued")
    
    // No storage! Just return task ID for tracking logs
    return c.Status(202).JSON(fiber.Map{
        "task_id": task.ID,
        "status":  "queued",
    })
}
```

**Result:** No memory accumulation from completed tasks.

---

### Context-Based Lifecycle

**Pattern:** All long-running operations use context with timeout

```go
// internal/worker/pool.go

func (p *WorkerPool) ProcessTask(task *queue.Task) error {
    // Create context with timeout (5 hours)
    ctx, cancel := context.WithTimeout(context.Background(), 5*time.Hour)
    defer cancel()  // ← Always cleanup context
    
    // Get available worker
    worker, err := p.GetHealthyWorker(ctx)
    if err != nil {
        return err
    }
    
    // gRPC call with context
    resp, err := worker.Client.Transcribe(ctx, &pb.TranscribeRequest{
        FilePath:      task.FilePath,
        TaskType:      task.TaskType,
        ForceLanguage: task.ForceLanguage,
        Options:       task.Options,
    })
    
    if err != nil {
        return fmt.Errorf("transcribe failed: %w", err)
    }
    
    // Log result (no storage)
    log.WithFields(logrus.Fields{
        "task_id":       task.ID,
        "subtitle_path": resp.SubtitlePath,
        "duration_ms":   resp.Stats.TranscriptionTimeMs,
    }).Info("transcription completed")
    
    return nil
}
```

**Benefits:**

- Context automatically cancels on timeout
- No goroutine leaks
- No connection leaks
- Memory freed when function returns

---

### Defer Pattern for Cleanup

**Pattern:** Use `defer` for all cleanup operations

```go
// internal/grpcclient/client.go

func (c *Client) Transcribe(ctx context.Context, req *pb.TranscribeRequest) (*pb.TranscribeResponse, error) {
    // Record metrics
    start := time.Now()
    defer func() {
        duration := time.Since(start)
        transcriptionDuration.Observe(duration.Seconds())
    }()
    
    // gRPC call
    resp, err := c.grpcClient.Transcribe(ctx, req)
    if err != nil {
        transcriptionErrors.Inc()
        return nil, err
    }
    
    return resp, nil
}
```

**Benefits:**

- Cleanup always runs (even on errors)
- No resource leaks
- Metrics always recorded

---

## Python Worker Memory Management

### Design Principles

1. **No global state** (no `task_results`, no global model reference)
2. **Context managers for all resources**
3. **Explicit cleanup in finally blocks**
4. **Memory monitoring with automatic restart**
5. **Model lifecycle with hard limits**

---

### No Global State

**Current subgen.py:** Global `task_results`, global `model`  
**New worker:** Everything scoped to request

```python
# worker/server/grpc_server.py

class TranscriptionServicer(transcription_pb2_grpc.TranscriptionServiceServicer):
    def __init__(self, config: Config):
        self.config = config
        self.model_manager = ModelManager(config)  # ← Encapsulated, not global
        self.stats = WorkerStats()
    
    def Transcribe(
        self,
        request: transcription_pb2.TranscribeRequest,
        context: grpc.ServicerContext
    ) -> transcription_pb2.TranscribeResponse:
        # No global state accessed!
        # All state passed as parameters
        
        try:
            result = self._transcribe_internal(request)
            self.stats.increment_completed()  # Thread-safe
            return result
        except Exception as e:
            logger.exception("Transcription failed")
            context.abort(grpc.StatusCode.INTERNAL, str(e))
```

**Benefits:**

- No shared mutable state
- Thread-safe by design
- Easy to test (no global mocking)
- No memory leaks from global dicts

---

### Context Managers Everywhere

**Pattern:** Use `with` statements for all resources

```python
# worker/transcription/audio.py

def extract_audio(video_path: str, output_path: str) -> None:
    """Extract audio from video using ffmpeg."""
    
    # Context manager ensures cleanup
    with av.open(video_path) as container:
        audio_stream = container.streams.audio[0]
        
        # BytesIO with context manager
        with io.BytesIO() as audio_buffer:
            for packet in container.demux(audio_stream):
                for frame in packet.decode():
                    audio_buffer.write(frame.to_ndarray().tobytes())
            
            # Write to file
            audio_buffer.seek(0)
            with open(output_path, 'wb') as f:
                f.write(audio_buffer.read())
        
        # ← All resources automatically closed here
```

**Benefits:**

- Automatic cleanup even on exceptions
- No leaked file handles
- No leaked memory buffers

---

### Model Lifecycle Manager

**Goal:** Keep delayed cleanup (performance) + prevent leaks (correctness)

```python
# worker/transcription/model.py

import threading
import time
from typing import Optional
from faster_whisper import WhisperModel
import psutil

class ModelManager:
    """Thread-safe Whisper model lifecycle manager."""
    
    def __init__(self, config: Config):
        self.config = config
        self._model: Optional[WhisperModel] = None
        self._lock = threading.Lock()
        self._cleanup_timer: Optional[threading.Timer] = None
        self._last_used = 0.0
    
    def get_model(self) -> WhisperModel:
        """Get model (load if not loaded)."""
        with self._lock:
            # Cancel cleanup timer if active
            if self._cleanup_timer:
                self._cleanup_timer.cancel()
                self._cleanup_timer = None
            
            # Load model if not loaded
            if self._model is None:
                logger.info(f"Loading Whisper model: {self.config.whisper_model}")
                start = time.time()
                
                self._model = WhisperModel(
                    self.config.whisper_model,
                    device=self.config.device,
                    compute_type=self.config.compute_type,
                    download_root=self.config.model_path,
                    num_workers=self.config.whisper_threads,
                )
                
                duration = time.time() - start
                logger.info(f"Model loaded in {duration:.2f}s")
            
            self._last_used = time.time()
            return self._model
    
    def schedule_cleanup(self) -> None:
        """Schedule model unload after delay."""
        with self._lock:
            # Check memory threshold
            memory_mb = psutil.Process().memory_info().rss / (1024 * 1024)
            if memory_mb > self.config.memory_threshold_mb:
                logger.warning(f"Memory {memory_mb}MB > threshold {self.config.memory_threshold_mb}MB, unloading immediately")
                self._unload_model()
                return
            
            # Schedule delayed cleanup
            if self._cleanup_timer:
                self._cleanup_timer.cancel()
            
            self._cleanup_timer = threading.Timer(
                self.config.model_cleanup_delay,
                self._cleanup_if_idle
            )
            self._cleanup_timer.daemon = True
            self._cleanup_timer.start()
            
            logger.debug(f"Scheduled model cleanup in {self.config.model_cleanup_delay}s")
    
    def _cleanup_if_idle(self) -> None:
        """Cleanup model if still idle."""
        with self._lock:
            # Check if used since timer scheduled
            idle_time = time.time() - self._last_used
            if idle_time >= self.config.model_cleanup_delay:
                self._unload_model()
            else:
                # Still active, reschedule
                logger.debug(f"Model used recently, rescheduling cleanup")
                self.schedule_cleanup()
    
    def _unload_model(self) -> None:
        """Unload model (must hold lock)."""
        if self._model is not None:
            logger.info("Unloading Whisper model")
            
            # Clear VRAM if using GPU
            if self.config.clear_vram_on_complete:
                try:
                    import torch
                    if torch.cuda.is_available():
                        torch.cuda.empty_cache()
                except ImportError:
                    pass
            
            # Delete model
            del self._model
            self._model = None
            
            # Force GC
            import gc
            gc.collect()
            
            logger.info("Model unloaded")
    
    def shutdown(self) -> None:
        """Graceful shutdown."""
        with self._lock:
            if self._cleanup_timer:
                self._cleanup_timer.cancel()
            self._unload_model()
```

**Features:**

- ✅ Delayed cleanup (30s default)
- ✅ Hard memory limit (immediate unload)
- ✅ Thread-safe (lock-protected)
- ✅ Graceful shutdown
- ✅ No race conditions

---

### Memory Monitoring

**Goal:** Detect memory leaks early and restart worker

```python
# worker/utils/memory.py

import psutil
import logging
from typing import Dict

logger = logging.getLogger(__name__)

class MemoryMonitor:
    """Monitor memory usage and detect leaks."""
    
    def __init__(self, threshold_mb: int = 3000):
        self.threshold_mb = threshold_mb
        self.process = psutil.Process()
        self.baseline_mb = self._get_memory_mb()
    
    def _get_memory_mb(self) -> int:
        """Get current memory usage in MB."""
        return self.process.memory_info().rss // (1024 * 1024)
    
    def check_memory(self) -> Dict[str, any]:
        """Check memory usage."""
        current_mb = self._get_memory_mb()
        growth_mb = current_mb - self.baseline_mb
        
        status = {
            'current_mb': current_mb,
            'baseline_mb': self.baseline_mb,
            'growth_mb': growth_mb,
            'threshold_mb': self.threshold_mb,
            'healthy': current_mb < self.threshold_mb
        }
        
        if not status['healthy']:
            logger.error(
                f"Memory threshold exceeded: {current_mb}MB > {self.threshold_mb}MB"
            )
        
        return status
    
    def reset_baseline(self) -> None:
        """Reset baseline after cleanup."""
        self.baseline_mb = self._get_memory_mb()
        logger.info(f"Memory baseline reset to {self.baseline_mb}MB")
```

**Usage in gRPC Server:**

```python
# worker/server/grpc_server.py

class TranscriptionServicer:
    def __init__(self, config: Config):
        self.memory_monitor = MemoryMonitor(config.memory_threshold_mb)
    
    def HealthCheck(
        self,
        request: transcription_pb2.HealthCheckRequest,
        context: grpc.ServicerContext
    ) -> transcription_pb2.HealthCheckResponse:
        # Check memory
        mem_status = self.memory_monitor.check_memory()
        
        return transcription_pb2.HealthCheckResponse(
            status=(
                transcription_pb2.HealthCheckResponse.HEALTHY
                if mem_status['healthy']
                else transcription_pb2.HealthCheckResponse.UNHEALTHY
            ),
            memory_mb=mem_status['current_mb'],
            model_loaded=self.model_manager.is_loaded(),
            jobs_processed=self.stats.total_jobs,
            jobs_active=self.stats.active_jobs,
            version="1.0.0",
            uptime_seconds=int(time.time() - self.start_time)
        )
```

**Kubernetes Integration:**

```yaml
# Orchestrator checks health endpoint
# If UNHEALTHY: restart pod
livenessProbe:
  grpc:
    port: 50051
  initialDelaySeconds: 60
  periodSeconds: 60
  failureThreshold: 3  # 3 consecutive failures → restart
```

---

### Explicit Cleanup Pattern

**Pattern:** Always use try-finally for cleanup

```python
# worker/transcription/engine.py

def transcribe_audio(
    file_path: str,
    options: TranscribeOptions,
    model_manager: ModelManager
) -> TranscriptionResult:
    """Transcribe audio file to subtitles."""
    
    temp_audio_path = None
    
    try:
        # Get model
        model = model_manager.get_model()
        
        # Extract audio to temp file
        temp_audio_path = f"/tmp/{uuid.uuid4()}.wav"
        extract_audio(file_path, temp_audio_path)
        
        # Transcribe
        result = model.transcribe(
            temp_audio_path,
            task=options.task_type,
            language=options.force_language or None,
            beam_size=5,
            vad_filter=True,
        )
        
        # Generate subtitles
        subtitle_path = generate_subtitles(
            result,
            file_path,
            options
        )
        
        return TranscriptionResult(
            success=True,
            subtitle_path=subtitle_path,
            detected_language=result.language,
        )
    
    finally:
        # ← Always cleanup temp files
        if temp_audio_path and os.path.exists(temp_audio_path):
            try:
                os.remove(temp_audio_path)
            except OSError as e:
                logger.warning(f"Failed to remove temp file: {e}")
        
        # Schedule model cleanup
        model_manager.schedule_cleanup()
```

**Benefits:**

- Temp files always deleted (even on errors)
- Model cleanup always scheduled
- No leaked resources

---

## Memory Monitoring

### Orchestrator Metrics

**Prometheus Metrics:**

```go
// internal/metrics/metrics.go

var (
    // Memory usage
    memoryUsageBytes = promauto.NewGauge(prometheus.GaugeOpts{
        Name: "subgen_orchestrator_memory_bytes",
        Help: "Current memory usage in bytes",
    })
    
    // Queue size
    queueSize = promauto.NewGauge(prometheus.GaugeOpts{
        Name: "subgen_queue_size",
        Help: "Current number of tasks in queue",
    })
    
    // Goroutine count
    goroutineCount = promauto.NewGauge(prometheus.GaugeOpts{
        Name: "subgen_orchestrator_goroutines",
        Help: "Current number of goroutines",
    })
)

// Update metrics every 30 seconds
func StartMetricsCollector() {
    ticker := time.NewTicker(30 * time.Second)
    go func() {
        for range ticker.C {
            var m runtime.MemStats
            runtime.ReadMemStats(&m)
            memoryUsageBytes.Set(float64(m.Alloc))
            goroutineCount.Set(float64(runtime.NumGoroutine()))
        }
    }()
}
```

---

### Worker Metrics

**Prometheus Metrics:**

```python
# worker/utils/metrics.py

from prometheus_client import Gauge, Counter

# Memory
worker_memory_mb = Gauge(
    'subgen_worker_memory_mb',
    'Current memory usage in MB'
)

# Model status
worker_model_loaded = Gauge(
    'subgen_worker_model_loaded',
    'Is Whisper model loaded (0=no, 1=yes)'
)

# Jobs
worker_jobs_total = Counter(
    'subgen_worker_jobs_total',
    'Total jobs processed'
)

worker_jobs_active = Gauge(
    'subgen_worker_jobs_active',
    'Currently processing jobs'
)

def update_metrics(memory_monitor: MemoryMonitor, model_manager: ModelManager):
    """Update Prometheus metrics."""
    mem_status = memory_monitor.check_memory()
    worker_memory_mb.set(mem_status['current_mb'])
    worker_model_loaded.set(1 if model_manager.is_loaded() else 0)
```

---

### Alerting

**Prometheus Alert Rules:**

```yaml
# prometheus-alerts.yaml

groups:
  - name: subgen
    interval: 30s
    rules:
      # Memory leak detection
      - alert: SubgenWorkerMemoryLeak
        expr: subgen_worker_memory_mb > 3500
        for: 5m
        labels:
          severity: critical
        annotations:
          summary: "Worker memory leak detected"
          description: "Worker memory {{ $value }}MB > 3500MB for 5 minutes"
      
      # Model stuck loaded
      - alert: SubgenModelStuckLoaded
        expr: subgen_worker_model_loaded == 1 AND subgen_worker_jobs_active == 0
        for: 10m
        labels:
          severity: warning
        annotations:
          summary: "Whisper model stuck loaded"
          description: "Model loaded but no active jobs for 10 minutes"
      
      # Queue growing
      - alert: SubgenQueueGrowing
        expr: rate(subgen_queue_size[5m]) > 0
        for: 30m
        labels:
          severity: warning
        annotations:
          summary: "Queue growing faster than processing"
          description: "Queue size increasing for 30 minutes"
```

---

## Testing Strategy

### Memory Leak Tests (Go)

```go
// orchestrator/test/integration/memory_test.go

func TestNoMemoryLeak_1000Transcriptions(t *testing.T) {
    // Start orchestrator
    orch := startTestOrchestrator(t)
    defer orch.Stop()
    
    // Get baseline memory
    var m1 runtime.MemStats
    runtime.ReadMemStats(&m1)
    baselineAlloc := m1.Alloc
    
    // Process 1000 tasks
    for i := 0; i < 1000; i++ {
        task := &queue.Task{
            ID:       uuid.New().String(),
            FilePath: "/testdata/sample.mp3",
        }
        
        require.NoError(t, orch.Queue.Push(task))
        
        // Wait for processing
        time.Sleep(10 * time.Millisecond)
    }
    
    // Force GC
    runtime.GC()
    time.Sleep(1 * time.Second)
    
    // Check final memory
    var m2 runtime.MemStats
    runtime.ReadMemStats(&m2)
    finalAlloc := m2.Alloc
    
    // Calculate growth
    growth := float64(finalAlloc-baselineAlloc) / float64(baselineAlloc)
    
    // Assert: Memory growth < 20%
    assert.Less(t, growth, 0.20, "Memory grew by %.2f%%", growth*100)
}
```

---

### Memory Leak Tests (Python)

```python
# worker/tests/integration/test_memory.py

import psutil
import pytest
import time
from worker.server.grpc_server import TranscriptionServicer

def test_no_memory_leak_1000_transcriptions():
    """Ensure no memory leaks after 1000 transcriptions."""
    
    servicer = TranscriptionServicer(config)
    process = psutil.Process()
    
    # Baseline
    baseline_mb = process.memory_info().rss / (1024 * 1024)
    
    # Process 1000 requests
    for i in range(1000):
        request = transcription_pb2.TranscribeRequest(
            file_path="/testdata/sample.mp3",
            task_type="transcribe",
            options=transcription_pb2.TranscribeOptions(
                whisper_model="tiny"
            )
        )
        
        context = FakeContext()
        response = servicer.Transcribe(request, context)
        assert response.success
        
        # Periodic GC
        if i % 100 == 0:
            import gc
            gc.collect()
    
    # Final GC
    import gc
    gc.collect()
    time.sleep(2)
    
    # Check memory
    final_mb = process.memory_info().rss / (1024 * 1024)
    growth_mb = final_mb - baseline_mb
    growth_percent = (growth_mb / baseline_mb) * 100
    
    # Assert: Memory growth < 20%
    assert growth_percent < 20, f"Memory grew by {growth_mb}MB ({growth_percent:.1f}%)"
```

---

## Kubernetes Memory Limits

### Pod Resource Limits

```yaml
# deploy/values.yaml

resources:
  orchestrator:
    requests:
      memory: 64Mi
      cpu: 100m
    limits:
      memory: 256Mi  # Hard limit
      cpu: 500m
  
  worker:
    requests:
      memory: 2Gi
      cpu: 500m
    limits:
      memory: 4Gi   # Hard limit (OOM kill if exceeded)
      cpu: 2000m
```

**Rationale:**

- **Orchestrator:** Lightweight, minimal memory needed
- **Worker:** Whisper models + audio processing = 2-4GB

**OOM Behavior:**

- If worker hits 4GB limit → K8s kills pod
- Orchestrator detects worker unavailable
- Requeues task to queue
- K8s restarts worker pod
- Task processed by new worker

---

### Memory Pressure Eviction

**Kubernetes Node Pressure:**

```yaml
# kubelet config (on nodes)
evictionHard:
  memory.available: "500Mi"
evictionSoft:
  memory.available: "1Gi"
evictionSoftGracePeriod:
  memory.available: "1m30s"
```

**Behavior:**

- If node memory < 500Mi → Hard eviction (immediate)
- If node memory < 1Gi → Soft eviction (1m30s grace period)

**Mitigation:**

- Set appropriate memory limits (not too high)
- Use multiple nodes (spread pods)
- Monitor node memory usage

---

## Summary

### Memory Leak Prevention

| Component | Mechanism | Result |
|-----------|-----------|--------|
| **Go Orchestrator** | Bounded queue, no storage, context cleanup, defer | ✅ No leaks |
| **Python Worker** | No global state, context managers, explicit cleanup | ✅ No leaks |
| **Whisper Model** | ModelManager with hard limits, delayed cleanup | ✅ No leaks |
| **Temp Files** | try-finally cleanup | ✅ No leaks |
| **gRPC Connections** | Context-based lifecycle | ✅ No leaks |

---

### Monitoring

- ✅ Prometheus metrics (memory, goroutines, queue size)
- ✅ Health checks (memory threshold)
- ✅ Alerting (memory leaks, stuck models)
- ✅ Kubernetes OOM protection

---

### Testing

- ✅ Memory leak tests (1000+ transcriptions)
- ✅ Integration tests (full pipeline)
- ✅ Load tests (24-hour soak test)

---

**Status:** Ready for implementation  
**Related Epics:** EPIC_01 (Orchestrator), EPIC_02 (Worker), EPIC_03 (Testing)  
**Next Steps:** Implement ModelManager, memory monitoring, integration tests
