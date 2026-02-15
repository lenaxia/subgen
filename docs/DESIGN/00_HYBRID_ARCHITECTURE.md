# Design Document: Hybrid Go + Python Architecture

**Document ID**: DESIGN-00
**Status**: Approved
**Created**: 2026-02-15
**Last Updated**: 2026-02-15
**Authors**: LLM Development Team

---

## Executive Summary

This document defines the hybrid Go + Python architecture for Subgen, replacing the monolithic Python implementation with a production-grade, scalable system that eliminates memory leaks while preserving Whisper transcription quality.

**Key Decision**: Go orchestrator + Python worker(s) communicating via gRPC

---

## Problem Statement

### Current Architecture Issues

**Memory Leaks (CRITICAL)**:
1. `task_results` dict grows unbounded (~10-100MB/year)
2. Whisper model not reliably unloading (1.5GB permanent allocation)
3. BytesIO objects not explicitly closed (minor, GC handles)

**Architectural Issues**:
- 2,144-line monolith (subgen.py)
- Global state management (model, queue, task_results)
- No tests (zero test coverage)
- Tight coupling (webhooks → transcription in same file)
- Python GIL limits concurrency

**Operational Issues**:
- Difficult to debug in production
- Hard to scale components independently
- No health checks or metrics
- Docker-only deployment (not k8s-native)

---

## Goals & Non-Goals

### Goals
- ✅ Eliminate memory leaks completely
- ✅ Enable safe refactoring and feature additions (via tests)
- ✅ Production-grade k8s deployment
- ✅ Preserve Whisper transcription quality (faster-whisper + stable-ts)
- ✅ Design for horizontal scaling (even if not used initially)
- ✅ Comprehensive observability (metrics, logs, health checks)
- ✅ Type safety enforced at compile time (Go)

### Non-Goals
- ❌ High-performance optimization (load is low: 2 episodes/day)
- ❌ Support for multiple media servers simultaneously (keep current model)
- ❌ Real-time streaming transcription (keep batch processing)
- ❌ Web UI (keep current approach: webhooks only)

---

## Architecture Overview

### High-Level System Design

```
┌─────────────────────────────────────────────────────────────────┐
│                  External Clients                                │
│  • Plex Server (webhooks)                                        │
│  • Jellyfin Server (webhooks)                                    │
│  • Emby Server (webhooks)                                        │
│  • Tautulli (webhooks)                                          │
│  • Prometheus (metrics scraping)                                │
└─────────────────────────────────────────────────────────────────┘
                            ↓ HTTP
┌─────────────────────────────────────────────────────────────────┐
│              Kubernetes Namespace: media                         │
├─────────────────────────────────────────────────────────────────┤
│                                                                  │
│  ┌───────────────────────────────────────────────────────────┐ │
│  │  Service: subgen-main (LoadBalancer)                      │ │
│  │  Ports: 9000 (webhooks), 9090 (metrics)                   │ │
│  └───────────────────────────────────────────────────────────┘ │
│                            ↓                                     │
│  ┌───────────────────────────────────────────────────────────┐ │
│  │  Pod: subgen-0 (Deployment, 1 replica)                    │ │
│  │  ━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━  │ │
│  │                                                             │ │
│  │  ┌─────────────────────────────────────────────────────┐  │ │
│  │  │  Container: orchestrator (Go)                       │  │ │
│  │  │  ━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━  │  │ │
│  │  │  Responsibilities:                                  │  │ │
│  │  │  • Receive webhooks (HTTP server)                  │  │ │
│  │  │  • Parse and validate payloads                     │  │ │
│  │  │  • Manage priority queue (in-memory)               │  │ │
│  │  │  • Communicate with media servers (Plex/Jellyfin)  │  │ │
│  │  │  • Dispatch jobs to workers (gRPC)                 │  │ │
│  │  │  • Monitor worker health                           │  │ │
│  │  │  • Export Prometheus metrics                       │  │ │
│  │  │  • Health check endpoint                           │  │ │
│  │  │                                                     │  │ │
│  │  │  Resources: 64Mi-256Mi RAM, 0.1-0.5 CPU           │  │ │
│  │  │  Language: Go 1.21+                                │  │ │
│  │  └─────────────────────────────────────────────────────┘  │ │
│  │                                                             │ │
│  │              ↓ gRPC (localhost:50051 in Phase 1)           │ │
│  │                                                             │ │
│  │  ┌─────────────────────────────────────────────────────┐  │ │
│  │  │  Container: worker (Python)                         │  │ │
│  │  │  ━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━  │  │ │
│  │  │  Responsibilities:                                  │  │ │
│  │  │  • gRPC server (Transcribe, DetectLanguage, Health)│  │ │
│  │  │  • Load Whisper models (faster-whisper)            │  │ │
│  │  │  • Transcribe audio (stable-ts)                    │  │ │
│  │  │  • Generate subtitles (SRT/LRC)                    │  │ │
│  │  │  • Write to NFS media share                        │  │ │
│  │  │  • Monitor memory usage                            │  │ │
│  │  │  • Report health to orchestrator                   │  │ │
│  │  │                                                     │  │ │
│  │  │  Resources: 2-4Gi RAM, 0.5-2 CPU                  │  │ │
│  │  │  Language: Python 3.11                             │  │ │
│  │  └─────────────────────────────────────────────────────┘  │ │
│  └───────────────────────────────────────────────────────────┘ │
│                                                                  │
│  ┌───────────────────────────────────────────────────────────┐ │
│  │  Shared Storage                                           │ │
│  │  • NFS: /media (RW) - Media files + subtitle output      │ │
│  │  • PVC: /models (5Gi) - Whisper models (persistent)      │ │
│  │  • emptyDir: /cache (1Gi, Memory) - Model loading cache  │ │
│  └───────────────────────────────────────────────────────────┘ │
│                                                                  │
└─────────────────────────────────────────────────────────────────┘
```

---

## Component Responsibilities

### Go Orchestrator (Stateless)

**Primary Responsibilities**:
1. **Webhook Reception**: Receive and parse webhooks from media servers
2. **Payload Validation**: Validate webhook payloads against schemas
3. **Queue Management**: Maintain in-memory priority queue with deduplication
4. **Media Server Integration**: Call Plex/Jellyfin APIs for file paths and metadata refresh
5. **Worker Communication**: Dispatch jobs to Python workers via gRPC
6. **Worker Discovery**: Support localhost (Phase 1) and K8s service discovery (Phase 2)
7. **Health Monitoring**: Monitor worker health and trigger restarts if needed
8. **Observability**: Export Prometheus metrics and provide health endpoint

**Why Go**:
- Zero memory leaks (predictable GC, no unbounded data structures)
- Type safety enforced at compile time
- Excellent k8s ecosystem (client-go)
- Fast HTTP server performance
- Easy to test and maintain
- Single binary deployment

**External Dependencies**:
- Media servers (Plex, Jellyfin, Emby, Tautulli) - HTTP webhooks
- Python workers - gRPC
- Prometheus - metrics scraping
- Kubernetes API - worker discovery (Phase 2 only)

---

### Python Worker (Stateful, Restartable)

**Primary Responsibilities**:
1. **gRPC Server**: Listen for Transcribe, DetectLanguage, HealthCheck RPCs
2. **Model Management**: Load and unload Whisper models efficiently
3. **Audio Processing**: Extract audio from video files via FFmpeg
4. **Transcription**: Transcribe audio using faster-whisper + stable-ts
5. **Subtitle Generation**: Generate SRT or LRC files
6. **File I/O**: Write subtitles to NFS media share (same location as media)
7. **Memory Monitoring**: Track memory usage, report to orchestrator
8. **Language Detection**: Detect audio language from sample

**Why Python**:
- ML ecosystem (faster-whisper, stable-ts, PyTorch)
- Proven transcription quality
- Existing Whisper model ecosystem
- Easy to refactor from existing code

**Design for Restartability**:
- Stateless (no persistent state in worker)
- All job state managed by orchestrator
- Can be killed and restarted anytime
- Memory leaks acceptable (orchestrator restarts on threshold)

**External Dependencies**:
- Whisper models (HuggingFace)
- FFmpeg (audio extraction)
- NFS media share (file I/O)
- Orchestrator - gRPC

---

## Data Flow

### Webhook → Transcription Flow

```
1. Media Server sends webhook
   POST http://subgen-service:9000/plex
   Body: {"event": "library.new", "Metadata": {"ratingKey": "12345"}}
   
2. Go Orchestrator receives webhook
   • Fiber HTTP handler parses payload
   • Validates payload structure
   • Extracts ratingKey
   
3. Orchestrator calls Plex API
   • GET http://plex:32400/library/metadata/12345
   • Extract file path: /media/TV/Show/S01E01.mkv
   • Parse metadata (title, season, episode)
   
4. Orchestrator checks skip conditions
   • Does subtitle already exist? (check NFS)
   • Is audio language in skip list?
   • Other skip logic
   
5. Orchestrator enqueues job
   • Priority: 2 (standard transcription)
   • Task ID: /media/TV/Show/S01E01.mkv (for deduplication)
   • Job data: {file_path, task_type, force_language, options, metadata}
   
6. Orchestrator dispatches job to worker
   • Get available worker (localhost:50051 in Phase 1)
   • Call worker.Transcribe() via gRPC
   • Wait for response (blocking, with timeout)
   
7. Python Worker receives gRPC request
   • Parse TranscribeRequest
   • Validate file path exists on NFS
   
8. Worker loads Whisper model (if not loaded)
   • Check if model already in memory
   • If not: Load from /models/medium.pt
   • Cache in memory for 30 seconds after last use
   
9. Worker extracts audio
   • FFmpeg extracts audio from /media/TV/Show/S01E01.mkv
   • Convert to 16kHz PCM
   • Store in memory (BytesIO with context manager)
   
10. Worker transcribes audio
    • faster-whisper.transcribe(audio)
    • stable-ts post-processing (timestamps, regrouping)
    • Language detection if not forced
    
11. Worker generates subtitle
    • Format as SRT or LRC (based on file type)
    • Add footer if configured
    
12. Worker writes subtitle to NFS
    • Path: /media/TV/Show/S01E01.medium.aa.srt
    • Same directory as media file
    • Atomic write (tmp → rename)
    
13. Worker returns response to orchestrator
    • TranscribeResponse with success=true
    • Subtitle path, detected language, stats
    
14. Orchestrator refreshes media server metadata
    • Plex: PUT http://plex:32400/library/metadata/12345/refresh
    • Jellyfin: POST http://jellyfin:8096/Items/12345/Refresh
    
15. Orchestrator removes job from queue
    • Mark as completed
    • Remove from processing set
    • Job complete!
```

---

## Communication Protocol: gRPC

### Why gRPC?

**Advantages**:
- ✅ Type safety (protobuf schema enforced)
- ✅ Efficient binary protocol (faster than JSON)
- ✅ Built-in health checking (standard gRPC health protocol)
- ✅ Streaming support (future feature: streaming progress)
- ✅ Language agnostic (Go ↔ Python ↔ any language)
- ✅ Code generation (Go + Python clients/servers from single .proto)
- ✅ Bi-directional communication (orchestrator can cancel jobs)

**Alternatives Considered**:
- ❌ REST/JSON: Less efficient, no type safety, more boilerplate
- ❌ Message queue (RabbitMQ/Redis): Over-engineering for low load
- ❌ Subprocess calls: Complex process management, no type safety

### RPC Methods

**1. Transcribe**:
- **Purpose**: Transcribe audio file to subtitles
- **Input**: File path, options, metadata
- **Output**: Subtitle content/path, language, stats
- **Timeout**: 5 hours (for large files)

**2. DetectLanguage**:
- **Purpose**: Detect audio language from sample
- **Input**: File path or audio bytes, sample length/offset
- **Output**: Language code, name, confidence
- **Timeout**: 1 minute

**3. HealthCheck**:
- **Purpose**: Monitor worker health and memory
- **Input**: Empty
- **Output**: Status, memory usage, model loaded, jobs processed
- **Timeout**: 10 seconds

---

## Scaling Strategy

### Phase 1: Single Pod (Current Need)

**Deployment Model**:
- 1 Deployment with 1 replica
- 1 Pod with 2 containers (orchestrator + worker)
- gRPC via localhost:50051 (fastest)

**When to Use**:
- Low load (< 10 episodes/day)
- Simple deployment
- Minimal resource usage

**Limitations**:
- Can't scale orchestrator and worker independently
- Both containers restart together

---

### Phase 2: Separate Deployments (Future Scaling)

**Deployment Model**:
- Orchestrator: 1 Deployment, 1 replica
- Workers: 1 StatefulSet, N replicas (3-10)
- gRPC via K8s Service (subgen-worker.media.svc.cluster.local:50051)

**When to Use**:
- High load (> 20 episodes/day)
- Need concurrent transcriptions
- Want independent scaling

**Migration from Phase 1 → Phase 2**:
1. Change `WORKER_DISCOVERY` from "localhost" to "kubernetes"
2. Deploy orchestrator and workers separately (2 app-template charts)
3. **Zero code changes required** - only configuration

**Worker Discovery**:
```go
// Orchestrator discovers workers dynamically
workers := discovery.GetWorkers(ctx)  // Returns []Worker
for _, worker := range workers {
    if worker.IsHealthy() {
        worker.Transcribe(ctx, request)
        break
    }
}
```

---

## Memory Management Strategy

### Go Orchestrator: Zero Leaks by Design

**Strategy**: Bounded data structures, context-based cleanup, defer statements

**Queue Management**:
```go
type Queue struct {
    items      *PriorityHeap      // Bounded by max size
    queued     map[string]bool    // Cleaned on job completion
    processing map[string]bool    // Cleaned on job completion
    maxSize    int                // Configurable limit (default: 1000)
}

func (q *Queue) Enqueue(job *Job) error {
    if len(q.items) >= q.maxSize {
        return ErrQueueFull
    }
    // Add to queue, mark as queued
}

func (q *Queue) MarkDone(jobID string) {
    delete(q.queued, jobID)
    delete(q.processing, jobID)
    // Map entries immediately removed
}
```

**No task_results Leak**:
- Orchestrator doesn't store results long-term
- Results returned immediately to webhook caller or discarded
- No unbounded maps

**Resource Cleanup**:
```go
func (o *Orchestrator) ProcessWebhook(ctx context.Context, payload []byte) error {
    // Defer ensures cleanup even on panic
    defer func() {
        if r := recover(); r != nil {
            log.Error("Panic in webhook handler", "error", r)
        }
    }()
    
    // Context with timeout
    ctx, cancel := context.WithTimeout(ctx, 30*time.Second)
    defer cancel()
    
    // Process webhook
    return o.handleWebhook(ctx, payload)
}
```

---

### Python Worker: Leak Fixes + Restart Strategy

**Strategy**: Fix known leaks, context managers for resources, orchestrator restarts on threshold

**Fix #1: No task_results**:
- Worker is stateless, doesn't store results
- Results returned immediately via gRPC response
- No unbounded data structures

**Fix #2: Context Managers**:
```python
@contextmanager
def whisper_model(model_name: str, device: str):
    """Context manager ensures model cleanup."""
    model = load_model(model_name, device)
    try:
        yield model
    finally:
        model.unload()
        if device == "cuda":
            torch.cuda.empty_cache()
        gc.collect()

# Usage
with whisper_model("medium", "cpu") as model:
    result = model.transcribe(audio)
# Guaranteed cleanup
```

**Fix #3: BytesIO Cleanup**:
```python
@contextmanager
def extract_audio(file_path: str, track_index: int = 0):
    """Context manager for audio extraction."""
    out = ffmpeg.run(..., capture_stdout=True)
    audio_buffer = BytesIO(out)
    try:
        yield audio_buffer
    finally:
        audio_buffer.close()
        del out  # Explicit cleanup

# Usage
with extract_audio(file_path) as audio:
    result = model.transcribe(audio)
# Guaranteed cleanup
```

**Memory Monitoring**:
```python
class MemoryMonitor:
    def __init__(self, threshold_mb: int = 3000):
        self.threshold_mb = threshold_mb
        self.process = psutil.Process(os.getpid())
        self.initial_memory = self.get_memory_mb()
    
    def get_memory_mb(self) -> float:
        return self.process.memory_info().rss / 1024 / 1024
    
    def is_over_threshold(self) -> bool:
        return self.get_memory_mb() > self.threshold_mb

# In HealthCheck RPC handler
def HealthCheck(self, request, context):
    memory_mb = memory_monitor.get_memory_mb()
    return HealthCheckResponse(
        status=HEALTHY if memory_mb < threshold else UNHEALTHY,
        memory_mb=int(memory_mb),
        model_loaded=model is not None
    )
```

**Orchestrator Restart Logic**:
```go
func (wm *WorkerManager) MonitorWorkers(ctx context.Context) {
    ticker := time.NewTicker(30 * time.Second)
    defer ticker.Stop()
    
    for {
        select {
        case <-ticker.C:
            for _, worker := range wm.workers {
                health, err := worker.HealthCheck(ctx)
                if err != nil || health.MemoryMb > 3000 {
                    log.Warn("Worker unhealthy or high memory, marking for restart",
                        "worker", worker.ID,
                        "memory_mb", health.MemoryMb,
                        "error", err)
                    
                    // In Phase 1: Log only (K8s will restart on OOM)
                    // In Phase 2: Can use K8s API to delete pod
                }
            }
        case <-ctx.Done():
            return
        }
    }
}
```

---

## Deployment Strategy (bjw-s app-template)

### Why bjw-s app-template?

**Benefits**:
- ✅ Battle-tested in k8s-at-home community (960 stars)
- ✅ Supports multiple containers per pod
- ✅ No custom Helm chart maintenance
- ✅ Rich feature set (ingress, services, persistence, probes)
- ✅ Active maintenance (latest: 4.6.2, Jan 2026)

**Trade-offs**:
- ⚠️ Must follow app-template patterns
- ⚠️ Less flexibility than custom chart (but sufficient for our needs)

### Deployment Resources

**Phase 1 Resources Created**:
```
Namespace: media
├── Deployment: subgen (1 replica)
│   └── Pod: subgen-0
│       ├── Container: orchestrator (Go)
│       └── Container: worker (Python)
├── Service: subgen-main (LoadBalancer)
│   ├── Port 9000 → HTTP webhooks
│   └── Port 9090 → Prometheus metrics
├── PersistentVolumeClaim: subgen-models (5Gi)
├── Secret: subgen-secrets (PLEX_TOKEN, JELLYFIN_TOKEN)
├── ConfigMap: subgen-config (optional, for complex config)
└── ServiceMonitor: subgen-main (Prometheus scraping)
```

**Phase 2 Resources** (When Scaling):
```
Namespace: media
├── Deployment: subgen-orchestrator (1 replica)
│   └── Pod: orchestrator only
├── StatefulSet: subgen-worker (3 replicas)
│   ├── Pod: subgen-worker-0
│   ├── Pod: subgen-worker-1
│   └── Pod: subgen-worker-2
├── Service: subgen-main (LoadBalancer, points to orchestrator)
├── Service: subgen-worker (Headless, for worker discovery)
├── PVC: subgen-models (shared or per-worker)
└── [Same secrets, configmaps, servicemonitor]
```

---

## Technology Stack Summary

### Go Orchestrator

| Component | Library | Version | Purpose |
|-----------|---------|---------|---------|
| HTTP Server | fiber/v2 | 2.52+ | Webhook receiver |
| gRPC Client | google.golang.org/grpc | 1.60+ | Worker communication |
| Config | spf13/viper | 1.18+ | Environment variables |
| Logging | sirupsen/logrus | 1.9+ | Structured logging |
| Metrics | prometheus/client_golang | 1.18+ | Prometheus exporter |
| Validation | go-playground/validator | 10.16+ | Payload validation |
| Testing | stretchr/testify | 1.8+ | Test assertions |
| Mocking | golang/mock | 1.6+ | Mock generation |
| K8s Client | k8s.io/client-go | 0.29+ | Worker discovery (Phase 2) |

### Python Worker

| Component | Library | Version | Purpose |
|-----------|---------|---------|---------|
| gRPC Server | grpcio | 1.60+ | RPC server |
| Whisper | faster-whisper | 1.0+ | Transcription engine |
| Stable-TS | stable-ts-whisperless | 2.17+ | Timestamp stability |
| Audio | ffmpeg-python | 0.2+ | Audio extraction |
| Media | av | 11.0+ | Media file inspection |
| Config | pydantic-settings | 2.1+ | Configuration |
| Logging | structlog | 24.1+ | Structured logging |
| Memory | psutil | 5.9+ | Memory monitoring |
| Testing | pytest | 7.4+ | Test framework |

---

## Security Considerations

### Network Security

**Phase 1 (Single Pod)**:
- gRPC communication via localhost (not exposed)
- Only HTTP port 9000 exposed to cluster
- Metrics port 9090 exposed to cluster (Prometheus only)

**Phase 2 (Separate Pods)**:
- gRPC service: ClusterIP (not exposed outside cluster)
- Orchestrator service: LoadBalancer (only HTTP 9000, metrics 9090)
- Worker pods not directly accessible

### Authentication

**Media Server Tokens**:
- Stored in K8s Secret (subgen-secrets)
- Mounted as environment variables
- Not logged or exposed in metrics

**gRPC Authentication** (Future):
- Phase 1: No auth needed (localhost)
- Phase 2: Consider mTLS or token-based auth

### Pod Security

**Security Context**:
```yaml
securityContext:
  runAsNonRoot: true
  runAsUser: 568
  runAsGroup: 568
  fsGroup: 568
  readOnlyRootFilesystem: false  # Need write for /tmp, /cache
  allowPrivilegeEscalation: false
  capabilities:
    drop:
      - ALL
```

**Network Policy** (Optional):
```yaml
apiVersion: networking.k8s.io/v1
kind: NetworkPolicy
metadata:
  name: subgen-netpol
spec:
  podSelector:
    matchLabels:
      app.kubernetes.io/name: subgen
  ingress:
    - from:
        - namespaceSelector:
            matchLabels:
              name: media  # Only from media namespace
      ports:
        - protocol: TCP
          port: 9000
    - from:  # Prometheus scraping
        - namespaceSelector:
            matchLabels:
              name: monitoring
      ports:
        - protocol: TCP
          port: 9090
```

---

## Observability

### Metrics (Prometheus)

**Orchestrator Metrics** (`/metrics` endpoint):
```
# Queue metrics
subgen_queue_size{priority="0|1|2"}                    # Gauge
subgen_queue_processing                                # Gauge

# Job metrics
subgen_jobs_total{status="queued"}                     # Counter
subgen_jobs_total{status="completed"}                  # Counter
subgen_jobs_total{status="failed"}                     # Counter
subgen_job_duration_seconds{type="transcribe"}         # Histogram

# Worker metrics
subgen_workers_available                               # Gauge
subgen_workers_healthy                                 # Gauge
subgen_worker_memory_bytes{worker_id="worker-0"}       # Gauge
subgen_worker_requests_total{worker_id="worker-0"}     # Counter

# Webhook metrics
subgen_webhook_requests_total{source="plex"}           # Counter
subgen_webhook_errors_total{source="plex"}             # Counter
subgen_webhook_duration_seconds{source="plex"}         # Histogram

# Media server metrics
subgen_media_server_calls_total{server="plex",endpoint="metadata"}  # Counter
subgen_media_server_errors_total{server="plex"}        # Counter
```

**Worker Metrics** (reported via HealthCheck):
- Memory usage (current, peak)
- Jobs processed
- Model load count
- Transcription duration

### Logging

**Format**: Structured JSON (for log aggregation)

**Go Orchestrator** (logrus):
```json
{
  "level": "info",
  "msg": "Webhook received",
  "timestamp": "2026-02-15T10:30:00Z",
  "source": "plex",
  "event": "library.new",
  "rating_key": "12345",
  "duration_ms": 123
}
```

**Python Worker** (structlog):
```json
{
  "event": "transcription_complete",
  "level": "info",
  "timestamp": "2026-02-15T10:35:00Z",
  "file_path": "/media/TV/Show/S01E01.mkv",
  "language": "en",
  "duration_seconds": 45.2,
  "model": "medium",
  "segments": 342
}
```

### Health Checks

**Orchestrator** (`/health`):
```json
{
  "status": "healthy",
  "queue_size": 3,
  "queue_processing": 1,
  "workers_healthy": 1,
  "workers_total": 1,
  "uptime_seconds": 3600
}
```

**Worker** (gRPC HealthCheck):
```protobuf
{
  "status": "HEALTHY",
  "memory_mb": 2048,
  "model_loaded": true,
  "jobs_processed": 42,
  "jobs_active": 1,
  "uptime_seconds": 3600
}
```

---

## File System Layout

### Shared NFS Mount: /media

**Structure**:
```
/media/
├── TV/
│   └── Show Name/
│       └── Season 01/
│           ├── S01E01.mkv                    # Original media
│           ├── S01E01.medium.aa.srt          # Generated subtitle
│           ├── S01E02.mkv
│           └── S01E02.medium.aa.srt
└── Movies/
    └── Movie Name (2024)/
        ├── Movie.mkv
        └── Movie.medium.aa.srt
```

**Subtitle Naming Convention**:
```
{filename}.{model}.{language}.{format}
Example: S01E01.medium.aa.srt

Components:
- filename: Original media filename (without extension)
- model: Whisper model used (tiny, base, small, medium, large)
- language: Subtitle language code (aa, en, eng, etc.)
- format: srt or lrc
```

**Configuration**:
- `SHOW_IN_SUBNAME_MODEL=true` → Include model name
- `SHOW_IN_SUBNAME_SUBGEN=true` → Add "subgen" marker (e.g., S01E01.subgen.medium.aa.srt)

### Model Storage: /models (PVC)

**Structure**:
```
/models/
├── tiny/                          # Whisper tiny model
├── base/
├── small/
├── medium/                        # Default
│   ├── model.bin
│   ├── config.json
│   └── tokenizer.json
├── large-v3/
└── .cache/                        # HuggingFace cache
```

**Size Requirements**:
- tiny: ~75MB
- base: ~145MB
- small: ~488MB
- medium: ~1.5GB
- large-v3: ~3GB

**PVC Size**: 5Gi (room for 2-3 models + cache)

---

## Error Handling & Recovery

### Orchestrator Error Scenarios

**1. Webhook Parse Error**:
```
Issue: Invalid JSON payload
Action: Return 400 Bad Request, log error
Recovery: None needed (client error)
```

**2. Media Server API Failure**:
```
Issue: Plex API returns 500 or timeout
Action: Log error, return 503 Service Unavailable
Recovery: Retry with exponential backoff (3 attempts)
```

**3. Worker Unavailable**:
```
Issue: gRPC call to worker fails
Action: Log error, requeue job with backoff
Recovery: Mark worker unhealthy, try other workers (Phase 2)
```

**4. Queue Full**:
```
Issue: Queue at max size (1000 jobs)
Action: Return 503 Service Unavailable
Recovery: Wait for queue to drain, webhook can retry
```

---

### Worker Error Scenarios

**1. File Not Found**:
```
Issue: File path doesn't exist on NFS
Action: Return TranscribeResponse with success=false, error="file not found"
Recovery: Orchestrator logs error, doesn't retry
```

**2. Model Download Failure**:
```
Issue: HuggingFace unreachable or model corrupt
Action: Return error to orchestrator
Recovery: Orchestrator retries 3x with backoff, then fails job
```

**3. Transcription Failure**:
```
Issue: Whisper crashes or audio corrupt
Action: Log error, return failure response
Recovery: Orchestrator marks job as failed, doesn't retry
```

**4. Memory Threshold Exceeded**:
```
Issue: Worker memory > 3GB
Action: Report UNHEALTHY in HealthCheck
Recovery: Orchestrator stops sending new jobs, K8s restarts pod on OOM
```

**5. Disk Full (NFS)**:
```
Issue: Can't write subtitle to NFS
Action: Return error to orchestrator
Recovery: Orchestrator logs error, job fails (manual intervention needed)
```

---

## Performance Characteristics

### Expected Performance (2 episodes/day)

**Orchestrator**:
- HTTP requests: < 10ms (webhook parsing)
- Queue operations: < 1ms (in-memory)
- gRPC dispatch: < 5ms (localhost in Phase 1)
- Memory usage: ~50-100MB (stable)

**Worker**:
- Model loading: 2-5 seconds (first time, cached after)
- Transcription: 1-2x real-time (45 min episode = 45-90 min transcription on CPU)
- Memory usage: 1.5-3GB (with medium model)
- Memory growth: ~0MB/job (all leaks fixed)

**Total Pipeline** (per episode):
- Webhook received → Queue → Worker → Transcribe → Write subtitle
- Time: ~45-90 minutes (dominated by transcription)
- Resource: 2-4GB RAM (worker), <100MB RAM (orchestrator)

---

## Testing Strategy

### Go Orchestrator Tests

**Unit Tests** (60+ tests):
- Config loading and validation
- Webhook parsing (all 4 types)
- Queue operations (enqueue, dequeue, dedup)
- Priority ordering
- Media server API clients (mocked HTTP)
- Worker discovery (both implementations)
- gRPC client (mocked worker)

**Integration Tests** (30+ tests):
- HTTP server → Queue → gRPC client
- All webhook types end-to-end
- Worker health monitoring
- Error scenarios (worker down, timeout)

**Example Test**:
```go
func TestPlexWebhook(t *testing.T) {
    // Setup
    mockWorker := new(MockWorkerClient)
    orchestrator := NewOrchestrator(mockWorker, ...)
    
    // Test
    payload := `{"event": "library.new", "Metadata": {"ratingKey": "123"}}`
    resp, err := orchestrator.HandleWebhook(context.Background(), []byte(payload))
    
    // Assert
    require.NoError(t, err)
    assert.Equal(t, 200, resp.StatusCode)
    assert.Equal(t, 1, orchestrator.queue.Size())
    mockWorker.AssertCalled(t, "Transcribe", mock.Anything, mock.Anything)
}
```

---

### Python Worker Tests

**Unit Tests** (50+ tests):
- gRPC server request handling
- Model lifecycle (load, transcribe, unload)
- Audio extraction (mocked ffmpeg)
- Subtitle generation (SRT, LRC formatting)
- Language detection
- Memory monitoring
- Configuration validation

**Integration Tests** (20+ tests):
- gRPC server → transcription engine
- Model loading with real models (tiny model for speed)
- Audio processing with test files
- File I/O with temporary directories

**Memory Leak Tests**:
```python
def test_no_memory_leak_after_1000_transcriptions():
    """Verify memory doesn't grow unbounded."""
    initial_memory = psutil.Process().memory_info().rss
    
    for i in range(1000):
        with whisper_model("tiny", "cpu") as model:
            # Use same audio file repeatedly
            result = model.transcribe(test_audio_path)
    
    final_memory = psutil.Process().memory_info().rss
    growth_mb = (final_memory - initial_memory) / 1024 / 1024
    
    # Allow 50MB growth (model cache, etc.)
    assert growth_mb < 50, f"Memory grew {growth_mb}MB, possible leak"
```

---

## Migration Path from Legacy

### Phase 0: Current State (Legacy)
- Docker container running subgen.py
- Webhook configured: http://docker-host:9000/plex
- Models stored in Docker volume

### Phase 1: Deploy New Architecture
- Deploy to K8s media namespace
- Configure webhook: http://subgen-service.media:9000/plex
- Import models to PVC
- Run in parallel with legacy for validation

### Phase 2: Cutover
- Update webhook to point to new service
- Monitor for 24-48 hours
- Verify memory stability
- Decommission legacy Docker container

### Rollback Plan
- Keep legacy Docker container stopped but ready
- Update webhook back to legacy
- Restart legacy container
- Investigate issues in new system

---

## Success Criteria

### Functional Requirements
- [ ] All webhook types supported (Plex, Jellyfin, Emby, Tautulli)
- [ ] Transcription quality identical to legacy
- [ ] All subtitle formats supported (SRT, LRC)
- [ ] Language detection works correctly
- [ ] Skip conditions work as expected
- [ ] Metadata refresh works (Plex, Jellyfin)

### Non-Functional Requirements
- [ ] Zero memory leaks (validated with 1000+ transcription test)
- [ ] Memory usage stable over 7 days
- [ ] All tests passing (150+ tests total)
- [ ] Health checks working (K8s probes)
- [ ] Metrics exported to Prometheus
- [ ] Logs structured and aggregatable
- [ ] Deployment via bjw-s app-template
- [ ] Documentation complete

### Performance Requirements
- [ ] Webhook response < 100ms
- [ ] Transcription time: 1-2x real-time (acceptable)
- [ ] Model loading < 10 seconds
- [ ] gRPC latency < 10ms (localhost in Phase 1)

---

## Future Enhancements (Out of Scope)

**Phase 3: Advanced Features** (Future Epics):
- Redis queue persistence (EPIC_06)
- Horizontal worker scaling (auto-scale based on queue size)
- Multiple model support (load different models per request)
- GPU support in K8s (NVIDIA device plugin)
- Streaming progress updates (gRPC server streaming)
- Web UI for queue management
- Advanced retry strategies (exponential backoff, dead letter queue)
- Multi-tenancy (multiple users, different configs)

---

## References

- **Original Implementation**: legacy/subgen.py
- **bjw-s app-template**: https://github.com/bjw-s-labs/helm-charts
- **faster-whisper**: https://github.com/guillaumekln/faster-whisper
- **gRPC**: https://grpc.io/
- **Prometheus**: https://prometheus.io/

---

## Appendix: Architecture Diagrams

### Component Interaction (Phase 1)

```
┌─────────────┐
│ Media Server│
│ (Plex)      │
└──────┬──────┘
       │ Webhook (HTTP)
       ↓
┌──────────────────────────────────────────────┐
│ Pod: subgen-0                                 │
│                                               │
│  ┌────────────────────────────────────────┐  │
│  │ Orchestrator (Go)                      │  │
│  │ Port 9000: HTTP Server                 │  │
│  │                                         │  │
│  │ 1. Parse webhook                       │  │
│  │ 2. Call Plex API → get file path      │  │
│  │ 3. Check skip conditions               │  │
│  │ 4. Enqueue job                         │  │
│  │ 5. Dispatch to worker (gRPC)          │  │
│  └────────────────────────────────────────┘  │
│                 ↓ gRPC                        │
│                 localhost:50051               │
│                 ↓                             │
│  ┌────────────────────────────────────────┐  │
│  │ Worker (Python)                        │  │
│  │ Port 50051: gRPC Server                │  │
│  │                                         │  │
│  │ 1. Receive TranscribeRequest           │  │
│  │ 2. Load model (if needed)              │  │
│  │ 3. Extract audio                       │  │
│  │ 4. Transcribe with Whisper             │  │
│  │ 5. Generate SRT/LRC                    │  │
│  │ 6. Write to NFS                        │  │
│  │ 7. Return TranscribeResponse           │  │
│  └────────────────────────────────────────┘  │
│                                               │
└───────────────────┬───────────────────────────┘
                    ↓
         ┌──────────────────────┐
         │ NFS: /media          │
         │ • Media files (RW)   │
         │ • Subtitles (output) │
         └──────────────────────┘
```

### Component Interaction (Phase 2 - Scaled)

```
┌─────────────┐
│Media Servers│
└──────┬──────┘
       │ Webhooks
       ↓
┌──────────────────────────────────┐
│ Pod: orchestrator-xxx            │
│  ┌────────────────────────────┐  │
│  │ Orchestrator (Go)          │  │
│  │ • Worker discovery         │  │
│  │ • Load balancing           │  │
│  │ • Health monitoring        │  │
│  └────────────────────────────┘  │
└───────────┬──────────────────────┘
            │ gRPC (via Service)
            ↓
  ┌─────────────────────────┐
  │ Service: subgen-worker  │
  │ Endpoints:              │
  │ • worker-0:50051        │
  │ • worker-1:50051        │
  │ • worker-2:50051        │
  └─────────────────────────┘
            ↓
    ┌───────┬───────┬───────┐
    ↓       ↓       ↓       ↓
┌─────┐ ┌─────┐ ┌─────┐
│Pod  │ │Pod  │ │Pod  │
│wrk-0│ │wrk-1│ │wrk-2│
└─────┘ └─────┘ └─────┘
Each pod has Python worker
```

---

**END OF DOCUMENT**

This architecture document serves as the foundation for all implementation work in EPIC_01 through EPIC_05.
