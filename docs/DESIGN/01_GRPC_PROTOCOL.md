# gRPC Protocol Design

**Document Version:** 1.0  
**Last Updated:** 2026-02-15  
**Status:** Draft  
**Related Documents:**
- [00_HYBRID_ARCHITECTURE.md](./00_HYBRID_ARCHITECTURE.md)
- [02_MEMORY_MANAGEMENT.md](./02_MEMORY_MANAGEMENT.md)

---

## Table of Contents

1. [Overview](#overview)
2. [Protocol Definition](#protocol-definition)
3. [RPC Methods](#rpc-methods)
4. [Message Schemas](#message-schemas)
5. [Error Handling](#error-handling)
6. [Streaming vs Unary](#streaming-vs-unary)
7. [Security Considerations](#security-considerations)
8. [Code Generation](#code-generation)

---

## Overview

### Purpose

The gRPC protocol defines the communication contract between the **Go orchestrator** and **Python worker(s)**. This protocol must be:

- **Language-agnostic** (works for Go and Python)
- **Type-safe** (protobuf validation)
- **Efficient** (binary serialization)
- **Extensible** (easy to add new fields without breaking compatibility)
- **Observable** (support for tracing and metrics)

### Protocol Buffer Version

- **Protobuf:** proto3
- **gRPC version:** 1.60+ (Go), grpcio 1.60+ (Python)

### Communication Pattern

```
┌─────────────────────┐                    ┌─────────────────────┐
│  Go Orchestrator    │                    │  Python Worker      │
│                     │                    │                     │
│  gRPC Client        │ ──── Unary ────>  │  gRPC Server        │
│                     │      RPC           │                     │
│  • Transcribe()     │                    │  • Load Model       │
│  • DetectLanguage() │                    │  • Process Audio    │
│  • HealthCheck()    │                    │  • Generate SRT/LRC │
│                     │ <─── Response ───  │  • Return Result    │
└─────────────────────┘                    └─────────────────────┘
```

**Pattern:** Request-response (unary RPC)  
**Rationale:** Transcription is long-running (minutes) but does not require bidirectional streaming. Unary RPCs are simpler and sufficient.

**Alternative Considered:** Server-streaming for progress updates  
**Decision:** Start with unary, add streaming in v2 if needed

---

## Protocol Definition

### File: `api/transcription.proto`

```protobuf
syntax = "proto3";

package subgen.v1;

option go_package = "github.com/your-org/subgen/orchestrator/pkg/api/v1";

// ============================================================================
// Service Definition
// ============================================================================

service TranscriptionService {
  // Transcribe audio to subtitles (SRT or LRC)
  rpc Transcribe(TranscribeRequest) returns (TranscribeResponse);
  
  // Detect language from audio sample
  rpc DetectLanguage(DetectLanguageRequest) returns (DetectLanguageResponse);
  
  // Health check for orchestrator monitoring
  rpc HealthCheck(HealthCheckRequest) returns (HealthCheckResponse);
}
```

---

## RPC Methods

### 1. Transcribe

**Purpose:** Main transcription workload. Converts audio/video file to subtitle file (SRT or LRC).

**Workflow:**

```
Orchestrator                           Worker
     │                                    │
     │  TranscribeRequest                │
     ├───────────────────────────────────>│
     │  (file_path, options, metadata)   │
     │                                    │
     │                                    │  1. Validate request
     │                                    │  2. Load Whisper model (if not loaded)
     │                                    │  3. Extract audio (ffmpeg)
     │                                    │  4. Transcribe (faster-whisper)
     │                                    │  5. Stabilize (stable-ts)
     │                                    │  6. Generate SRT/LRC
     │                                    │  7. Write subtitle file
     │                                    │  8. Cleanup resources
     │                                    │
     │  TranscribeResponse               │
     │<───────────────────────────────────┤
     │  (success, subtitle_path, stats)  │
     │                                    │
```

**Request Fields:**

| Field | Type | Required | Description |
|-------|------|----------|-------------|
| `file_path` | string | ✅ | Absolute path to media file on shared NFS |
| `task_type` | string | ✅ | `"transcribe"` or `"translate"` |
| `force_language` | string | ❌ | ISO 639-1 code (e.g., `"en"`), empty = auto-detect |
| `options` | TranscribeOptions | ✅ | Transcription configuration |
| `metadata` | map<string,string> | ❌ | Key-value pairs (e.g., `plex_item_id`) |

**Response Fields:**

| Field | Type | Description |
|-------|------|-------------|
| `success` | bool | `true` if transcription completed |
| `subtitle_path` | string | Absolute path to generated subtitle file |
| `detected_language` | string | ISO 639-1 language code |
| `error_message` | string | Error details if `success=false` |
| `stats` | TranscriptionStats | Performance metrics |

**Error Scenarios:**

| Error | gRPC Status | Description |
|-------|-------------|-------------|
| File not found | `NOT_FOUND` | `file_path` does not exist on NFS |
| Invalid format | `INVALID_ARGUMENT` | Unsupported audio/video format |
| Model load failed | `INTERNAL` | Whisper model failed to load |
| Out of memory | `RESOURCE_EXHAUSTED` | Worker hit memory limit |
| Transcription timeout | `DEADLINE_EXCEEDED` | Exceeded max processing time |

**Timeout:** 5 hours (18,000 seconds) - configurable via orchestrator

---

### 2. DetectLanguage

**Purpose:** Detect the spoken language in an audio file without full transcription. Used for:
- Pre-flight validation (skip if language already has subtitles)
- Language-specific processing
- Routing to specialized models

**Workflow:**

```
Orchestrator                           Worker
     │                                    │
     │  DetectLanguageRequest            │
     ├───────────────────────────────────>│
     │  (file_path, sample_length=30s)   │
     │                                    │
     │                                    │  1. Extract 30s audio sample
     │                                    │  2. Load Whisper model (tiny)
     │                                    │  3. Detect language
     │                                    │  4. Return result
     │                                    │
     │  DetectLanguageResponse           │
     │<───────────────────────────────────┤
     │  (language_code="en", confidence)  │
     │                                    │
```

**Request Fields:**

| Field | Type | Required | Description |
|-------|------|----------|-------------|
| `audio_source` | oneof | ✅ | Either `file_path` or `audio_content` |
| `sample_length` | int32 | ❌ | Sample duration in seconds (default: 30) |
| `sample_offset` | int32 | ❌ | Start offset in seconds (default: 0) |

**Response Fields:**

| Field | Type | Description |
|-------|------|-------------|
| `success` | bool | `true` if detection completed |
| `language_code` | string | ISO 639-1 code (e.g., `"en"`) |
| `language_name` | string | English name (e.g., `"English"`) |
| `confidence` | float | Confidence score 0.0-1.0 |
| `error_message` | string | Error details if `success=false` |

**Performance Target:** < 10 seconds for 30-second sample

---

### 3. HealthCheck

**Purpose:** Monitor worker health and readiness. Orchestrator uses this to:
- Track worker availability
- Route jobs to healthy workers
- Trigger alerts on degradation

**Workflow:**

```
Orchestrator                           Worker
     │                                    │
     │  HealthCheckRequest               │
     ├───────────────────────────────────>│
     │  (empty)                           │
     │                                    │
     │                                    │  1. Check memory usage
     │                                    │  2. Check model status
     │                                    │  3. Check active jobs
     │                                    │  4. Return metrics
     │                                    │
     │  HealthCheckResponse              │
     │<───────────────────────────────────┤
     │  (status=HEALTHY, memory=1200MB)   │
     │                                    │
```

**Request Fields:**

- Empty (reserved for future use)

**Response Fields:**

| Field | Type | Description |
|-------|------|-------------|
| `status` | enum Status | `HEALTHY`, `UNHEALTHY`, `STARTING`, `UNKNOWN` |
| `memory_mb` | int64 | Current memory usage in MB |
| `model_loaded` | bool | Is Whisper model loaded? |
| `jobs_processed` | int32 | Total jobs since start |
| `jobs_active` | int32 | Currently processing jobs |
| `version` | string | Worker version (e.g., `"1.0.0"`) |
| `uptime_seconds` | int64 | Time since worker started |

**Health Status:**

| Status | Condition |
|--------|-----------|
| `HEALTHY` | Memory < 80% limit, no errors, model loaded |
| `UNHEALTHY` | Memory > 90% limit, repeated errors, model load failed |
| `STARTING` | Worker started, model not loaded yet |
| `UNKNOWN` | Health check failed or timed out |

**Frequency:** Orchestrator polls every 30 seconds (configurable)

---

## Message Schemas

### TranscribeOptions

Configuration for transcription behavior.

```protobuf
message TranscribeOptions {
  // Whisper model: tiny, base, small, medium, large, large-v3
  string whisper_model = 1;
  
  // CPU threads for transcription
  int32 whisper_threads = 2;
  
  // Word-level highlighting (karaoke style)
  bool word_level_highlight = 3;
  
  // Custom regroup algorithm (stable-ts)
  string custom_regroup = 4;
  
  // Generate LRC for audio files (vs SRT)
  bool lrc_for_audio = 5;
  
  // Custom prompt for model
  string custom_prompt = 6;
  
  // Append "Transcribed by..." footer
  bool append_footer = 7;
  
  // Subtitle language name (aa, en, etc.)
  string subtitle_language_name = 8;
  
  // Show model name in subtitle filename
  bool show_model_in_filename = 9;
  
  // Show "subgen" in subtitle filename
  bool show_subgen_in_filename = 10;
}
```

**Defaults:**

| Field | Default Value |
|-------|---------------|
| `whisper_model` | `"medium"` |
| `whisper_threads` | `4` |
| `word_level_highlight` | `false` |
| `custom_regroup` | `"cm_sl=84_sl=42++++++1"` |
| `lrc_for_audio` | `true` |
| `custom_prompt` | `""` |
| `append_footer` | `false` |
| `subtitle_language_name` | `"aa"` |
| `show_model_in_filename` | `false` |
| `show_subgen_in_filename` | `false` |

---

### TranscriptionStats

Performance metrics for observability.

```protobuf
message TranscriptionStats {
  // Total duration in seconds
  float duration_seconds = 1;
  
  // Number of subtitle segments
  int32 segment_count = 2;
  
  // Model load time (milliseconds)
  int64 model_load_time_ms = 3;
  
  // Transcription time (milliseconds)
  int64 transcription_time_ms = 4;
  
  // Peak memory usage (MB)
  int64 peak_memory_mb = 5;
}
```

**Usage:**

```go
// Orchestrator logs stats for monitoring
log.WithFields(logrus.Fields{
    "duration":          stats.DurationSeconds,
    "segment_count":     stats.SegmentCount,
    "transcription_ms":  stats.TranscriptionTimeMs,
    "peak_memory_mb":    stats.PeakMemoryMb,
}).Info("transcription completed")
```

---

## Error Handling

### gRPC Status Codes

| Status Code | Use Case | Example |
|-------------|----------|---------|
| `OK` | Success | Transcription completed |
| `INVALID_ARGUMENT` | Bad request | Invalid file path, unsupported format |
| `NOT_FOUND` | File not found | Media file does not exist on NFS |
| `RESOURCE_EXHAUSTED` | Out of resources | Memory limit exceeded |
| `INTERNAL` | Internal error | Model load failed, unexpected exception |
| `DEADLINE_EXCEEDED` | Timeout | Transcription took > 5 hours |
| `UNAVAILABLE` | Worker down | Worker crashed or unreachable |

### Error Response Pattern

**Orchestrator Handling:**

```go
resp, err := grpcClient.Transcribe(ctx, req)
if err != nil {
    // Check gRPC status code
    st, ok := status.FromError(err)
    if ok {
        switch st.Code() {
        case codes.DeadlineExceeded:
            log.Warn("transcription timeout, requeueing")
            queue.Requeue(task)
        case codes.Unavailable:
            log.Error("worker unavailable, trying next worker")
            workerPool.MarkUnhealthy(workerID)
        case codes.ResourceExhausted:
            log.Error("worker out of memory, restarting worker")
            workerPool.RestartWorker(workerID)
        default:
            log.Errorf("transcription failed: %v", st.Message())
        }
    }
    return err
}

if !resp.Success {
    log.Errorf("transcription failed: %s", resp.ErrorMessage)
    return fmt.Errorf("transcription failed: %s", resp.ErrorMessage)
}
```

**Worker Error Handling:**

```python
try:
    # Transcription logic
    result = transcribe_audio(file_path, options)
    return TranscribeResponse(
        success=True,
        subtitle_path=result.subtitle_path,
        detected_language=result.language,
        stats=result.stats
    )
except FileNotFoundError as e:
    context.abort(grpc.StatusCode.NOT_FOUND, f"File not found: {e}")
except MemoryError as e:
    context.abort(grpc.StatusCode.RESOURCE_EXHAUSTED, f"Out of memory: {e}")
except Exception as e:
    logger.exception("Transcription failed")
    context.abort(grpc.StatusCode.INTERNAL, f"Internal error: {e}")
```

---

## Streaming vs Unary

### Design Decision: Unary RPC

**Rationale:**

1. **Simplicity:** Transcription is fire-and-forget. Orchestrator waits for completion.
2. **State Management:** Orchestrator manages task state, not worker.
3. **Retries:** Unary RPCs are easier to retry (idempotent).
4. **Load Balancing:** Easier to route to different workers on retry.

**Tradeoffs:**

| Aspect | Unary | Streaming |
|--------|-------|-----------|
| Progress Updates | ❌ No | ✅ Yes |
| Complexity | ✅ Low | ❌ High |
| Retry Logic | ✅ Simple | ❌ Complex |
| Backpressure | ✅ Natural | ❌ Manual |
| Observability | ✅ Clear start/end | ❌ Requires correlation IDs |

### Future Consideration: Server-Streaming

If progress updates become important (e.g., UI showing transcription progress):

```protobuf
// Future: Streaming version
rpc TranscribeStream(TranscribeRequest) returns (stream TranscribeProgress);

message TranscribeProgress {
  enum Status {
    QUEUED = 0;
    EXTRACTING_AUDIO = 1;
    TRANSCRIBING = 2;
    GENERATING_SUBTITLES = 3;
    COMPLETED = 4;
  }
  
  Status status = 1;
  float progress_percent = 2;  // 0-100
  string current_step = 3;
  TranscriptionStats partial_stats = 4;
}
```

**Decision:** Not implemented in v1. Add if user demand exists.

---

## Security Considerations

### Authentication

**Phase 1 (Single Pod):** No authentication needed (localhost communication)

**Phase 2 (Separate Pods):** TLS + mTLS

```go
// Orchestrator: TLS client config
creds, err := credentials.NewClientTLSFromFile("certs/ca.crt", "")
conn, err := grpc.Dial(
    workerAddress,
    grpc.WithTransportCredentials(creds),
)
```

```python
# Worker: TLS server config
server_credentials = grpc.ssl_server_credentials(
    [(private_key, certificate_chain)]
)
server.add_secure_port('0.0.0.0:50051', server_credentials)
```

**Certificates:** Managed via cert-manager in Kubernetes

---

### Authorization

**Current:** No authorization needed (orchestrator trusts workers)

**Future:** If exposing worker API externally, add token-based auth:

```protobuf
message TranscribeRequest {
  string auth_token = 100;  // Reserved field number
  // ... other fields
}
```

---

### Data Privacy

**Concern:** Sensitive media files (personal videos, etc.)

**Mitigation:**

1. **Shared NFS:** Both orchestrator and worker access same filesystem (no data transfer over network)
2. **File Paths Only:** gRPC messages only contain file paths, not audio data
3. **Ephemeral Processing:** Worker does not store intermediate files
4. **Logs:** Redact file paths in logs (configurable)

```go
// Orchestrator: Redact file paths in logs
func redactPath(path string) string {
    if os.Getenv("REDACT_LOGS") == "true" {
        return "[REDACTED]"
    }
    return path
}
```

---

## Code Generation

### Generate Go Code

```bash
# From repository root
cd api

protoc \
  --go_out=../orchestrator/pkg/api/v1 \
  --go_opt=paths=source_relative \
  --go-grpc_out=../orchestrator/pkg/api/v1 \
  --go-grpc_opt=paths=source_relative \
  transcription.proto
```

**Output:**
- `orchestrator/pkg/api/v1/transcription.pb.go` (message types)
- `orchestrator/pkg/api/v1/transcription_grpc.pb.go` (service client)

---

### Generate Python Code

```bash
# From repository root
cd api

python -m grpc_tools.protoc \
  -I. \
  --python_out=../worker/generated \
  --pyi_out=../worker/generated \
  --grpc_python_out=../worker/generated \
  transcription.proto
```

**Output:**
- `worker/generated/transcription_pb2.py` (message types)
- `worker/generated/transcription_pb2.pyi` (type stubs)
- `worker/generated/transcription_pb2_grpc.py` (service server)

---

### Makefile Automation

```makefile
# Makefile target
.PHONY: proto
proto:
	@echo "Generating protobuf code..."
	cd api && ./generate.sh
	@echo "Done"
```

**File: `api/generate.sh`**

```bash
#!/bin/bash
set -euo pipefail

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
cd "$SCRIPT_DIR"

# Go
protoc \
  --go_out=../orchestrator/pkg/api/v1 \
  --go_opt=paths=source_relative \
  --go-grpc_out=../orchestrator/pkg/api/v1 \
  --go-grpc_opt=paths=source_relative \
  transcription.proto

# Python
python -m grpc_tools.protoc \
  -I. \
  --python_out=../worker/generated \
  --pyi_out=../worker/generated \
  --grpc_python_out=../worker/generated \
  transcription.proto

echo "✅ Protobuf code generated"
```

---

## Version Compatibility

### Backward Compatibility Rules

1. **Never remove fields** (mark as deprecated instead)
2. **Never change field numbers**
3. **Add new fields with high numbers** (100+)
4. **Use `optional` for nullable fields**

**Example: Adding new field**

```protobuf
// Before
message TranscribeOptions {
  string whisper_model = 1;
  int32 whisper_threads = 2;
}

// After (backward compatible)
message TranscribeOptions {
  string whisper_model = 1;
  int32 whisper_threads = 2;
  bool enable_vad = 3;  // NEW: Voice Activity Detection
}
```

**Old clients:** Send requests without `enable_vad` → worker uses default (false)  
**New clients:** Send requests with `enable_vad=true` → worker uses VAD  
**Result:** ✅ No breaking changes

---

## Testing Strategy

### Unit Tests

**Go (Orchestrator):**

```go
func TestTranscribeRequest_Validation(t *testing.T) {
    req := &v1.TranscribeRequest{
        FilePath: "",
        TaskType: "transcribe",
    }
    
    err := validateRequest(req)
    assert.Error(t, err, "empty file_path should fail")
}
```

**Python (Worker):**

```python
def test_transcribe_request_validation():
    request = transcription_pb2.TranscribeRequest()
    # Missing required fields
    
    with pytest.raises(ValueError):
        validate_request(request)
```

---

### Integration Tests

**Scenario:** Go client → Python server

```go
func TestTranscribeIntegration(t *testing.T) {
    // Start Python worker in test mode
    worker := startTestWorker(t)
    defer worker.Stop()
    
    // gRPC client
    client := v1.NewTranscriptionServiceClient(conn)
    
    // Request
    req := &v1.TranscribeRequest{
        FilePath: "/testdata/sample.mp3",
        TaskType: "transcribe",
        Options: &v1.TranscribeOptions{
            WhisperModel: "tiny",
        },
    }
    
    // Call
    resp, err := client.Transcribe(context.Background(), req)
    require.NoError(t, err)
    
    // Assertions
    assert.True(t, resp.Success)
    assert.NotEmpty(t, resp.SubtitlePath)
    assert.FileExists(t, resp.SubtitlePath)
}
```

---

## Performance Targets

| Metric | Target | Measurement |
|--------|--------|-------------|
| Health check latency | < 100ms | p99 |
| Language detection | < 10s | avg |
| Transcription (1hr audio) | < 10min | avg |
| gRPC message overhead | < 1KB | request size |
| Concurrent requests | 10+ | worker capacity |

---

## Observability

### Metrics

**Orchestrator:**

- `subgen_grpc_requests_total{method, status}` - Total gRPC requests
- `subgen_grpc_request_duration_seconds{method}` - Request latency
- `subgen_grpc_errors_total{method, code}` - Error count by gRPC status code

**Worker:**

- `subgen_worker_requests_total{method}` - Total requests handled
- `subgen_worker_request_duration_seconds{method}` - Processing time
- `subgen_worker_memory_mb` - Current memory usage
- `subgen_worker_model_loaded` - Is model loaded (0/1)

---

### Tracing

**OpenTelemetry Integration:**

```go
// Orchestrator: Inject trace context
import "go.opentelemetry.io/contrib/instrumentation/google.golang.org/grpc/otelgrpc"

conn, err := grpc.Dial(
    address,
    grpc.WithUnaryInterceptor(otelgrpc.UnaryClientInterceptor()),
)
```

```python
# Worker: Extract trace context
from opentelemetry.instrumentation.grpc import GrpcInstrumentorServer

GrpcInstrumentorServer().instrument()
```

**Benefit:** Trace requests end-to-end (webhook → queue → worker → media server)

---

## Summary

**gRPC Protocol:**

- ✅ **3 RPC methods:** Transcribe, DetectLanguage, HealthCheck
- ✅ **Type-safe:** Protobuf schema with validation
- ✅ **Efficient:** Binary serialization, low overhead
- ✅ **Observable:** Metrics, tracing, structured errors
- ✅ **Scalable:** Supports 1+ workers with same protocol
- ✅ **Maintainable:** Backward-compatible evolution

**Next Steps:**

1. Create `api/transcription.proto` (complete schema)
2. Implement Go gRPC client (EPIC_01 STORY_07)
3. Implement Python gRPC server (EPIC_02 STORY_01)
4. Integration tests (EPIC_03 STORY_01)

**Related Documents:**

- [00_HYBRID_ARCHITECTURE.md](./00_HYBRID_ARCHITECTURE.md) - System architecture
- [02_MEMORY_MANAGEMENT.md](./02_MEMORY_MANAGEMENT.md) - Memory leak prevention
- [03_SCALING_STRATEGY.md](./03_SCALING_STRATEGY.md) - Phase 1 → Phase 2 scaling

---

**Status:** Ready for implementation  
**Owner:** TBD  
**Epic:** EPIC_01, EPIC_02, EPIC_03
