# EPIC_03: Integration & Testing

**Status:** Not Started  
**Estimated Effort:** 34-44 hours  
**Duration:** 1 week  
**Can Parallelize:** ❌ No (depends on EPIC_01 + EPIC_02)

---

## Overview

Comprehensive integration and end-to-end testing to validate that the **Go orchestrator** and **Python worker** work together correctly. This epic ensures the entire system functions as a cohesive unit, with special focus on memory leak validation and production readiness.

---

## Goals

1. Validate gRPC communication (Go ↔ Python)
2. Test all webhook flows end-to-end
3. Validate memory leak fixes (1000+ transcriptions)
4. Load testing for production readiness
5. End-to-end pipeline validation
6. Document test procedures

---

## Design References

- [00_HYBRID_ARCHITECTURE.md](../../DESIGN/00_HYBRID_ARCHITECTURE.md) - System architecture
- [01_GRPC_PROTOCOL.md](../../DESIGN/01_GRPC_PROTOCOL.md) - gRPC protocol
- [02_MEMORY_MANAGEMENT.md](../../DESIGN/02_MEMORY_MANAGEMENT.md) - Memory leak prevention

---

## User Stories

### [STORY_01: gRPC Integration Tests](./stories/STORY_01_grpc_integration.md)
**Status:** Not Started  
**Effort:** 8-10 hours  
**Summary:** Go client → Python server integration tests for all RPC methods

### [STORY_02: Webhook Integration Tests](./stories/STORY_02_webhook_integration.md)
**Status:** Not Started  
**Effort:** 8-10 hours  
**Summary:** Webhook → Queue → Worker → Result end-to-end tests

### [STORY_03: End-to-End Pipeline Tests](./stories/STORY_03_e2e_tests.md)
**Status:** Not Started  
**Effort:** 8-10 hours  
**Summary:** Complete pipeline with real audio files, all subtitle formats

### [STORY_04: Memory Leak Validation](./stories/STORY_04_memory_leak_tests.md)
**Status:** Not Started  
**Effort:** 6-8 hours  
**Summary:** Run 1000 transcription cycles, verify no memory growth

### [STORY_05: Load Testing](./stories/STORY_05_load_testing.md)
**Status:** Not Started  
**Effort:** 4-6 hours  
**Summary:** Performance benchmarks, 24-hour soak test, stress testing

---

## Acceptance Criteria

- [ ] All 5 stories completed
- [ ] All tests passing (gRPC + webhook + E2E)
- [ ] Memory leak tests pass (< 20% growth after 1000 transcriptions)
- [ ] Load tests meet targets (see below)
- [ ] 24-hour soak test passes with zero crashes
- [ ] All 4 webhook types tested (Plex, Jellyfin, Emby, Tautulli)
- [ ] All 3 gRPC methods tested (Transcribe, DetectLanguage, HealthCheck)
- [ ] Test documentation complete
- [ ] Work logs created for all stories

---

## Dependencies

**Requires:**
- EPIC_01 (Go Orchestrator Core) - **MUST be complete**
- EPIC_02 (Python Worker Refactor) - **MUST be complete**

**Blocks:**
- EPIC_04 (K8s Deployment) - needs validated code
- EPIC_05 (Migration & Cutover) - needs confidence in new system

**Parallelizable With:**
- None (sequential epic)

---

## Testing Pyramid

```
         ╱──────────╲
        ╱  E2E (10)  ╲
       ╱──────────────╲
      ╱ Integration(25)╲
     ╱──────────────────╲
    ╱   Unit Tests (65)  ╲
   ╱──────────────────────╲
```

**Distribution:**
- Unit tests: 65% (already in EPIC_01 + EPIC_02)
- Integration tests: 25% (this epic)
- E2E tests: 10% (this epic)

---

## Performance Targets

| Metric | Target | Measurement |
|--------|--------|-------------|
| **gRPC latency** | < 100ms | p99 for HealthCheck |
| **Language detection** | < 10s | avg for 30s audio sample |
| **Transcription (1hr audio)** | < 10min | avg on CPU (medium model) |
| **Memory growth** | < 20% | After 1000 transcriptions |
| **Queue throughput** | 100+ tasks/sec | Push operations |
| **Concurrent webhooks** | 50+ req/sec | Without errors |
| **24-hour uptime** | Zero crashes | Continuous operation |

---

## Test Infrastructure

### Docker Compose for Testing

**File: `test/docker-compose.test.yml`**

```yaml
version: '3.8'
services:
  orchestrator:
    build:
      context: ../orchestrator
      dockerfile: Dockerfile
    environment:
      WORKER_DISCOVERY: "localhost"
      PYTHON_WORKER_ADDRESS: "worker:50051"
    ports:
      - "9000:9000"
      - "9090:9090"
    depends_on:
      - worker
  
  worker:
    build:
      context: ../worker
      dockerfile: Dockerfile
    environment:
      GRPC_PORT: "50051"
      WHISPER_MODEL: "tiny"  # Fast for testing
    ports:
      - "50051:50051"
    volumes:
      - ./testdata:/testdata:ro
```

**Usage:**
```bash
cd test
docker-compose -f docker-compose.test.yml up
# Run tests against localhost:9000
```

---

### Test Data

**Location:** `test/testdata/`

**Contents:**
- `short_audio.mp3` (30 seconds, English)
- `long_audio.mp3` (5 minutes, English)
- `multilang_audio.mp3` (30 seconds, Spanish)
- `video.mkv` (1 minute, multiple audio tracks)
- `audio_only.m4a` (30 seconds, for LRC generation)

**Generation:**
```bash
# Generate test audio (if needed)
ffmpeg -f lavfi -i "sine=frequency=440:duration=30" -ac 1 -ar 16000 test/testdata/short_audio.mp3
```

---

## Timeline

**Day 1-2:** STORY_01 (gRPC Integration Tests)  
**Day 3:** STORY_02 (Webhook Integration Tests)  
**Day 4:** STORY_03 (E2E Pipeline Tests)  
**Day 5:** STORY_04 (Memory Leak Validation) - **CRITICAL**  
**Day 6:** STORY_05 (Load Testing)  
**Day 7:** Buffer, documentation, fixes

---

## Risks & Mitigation

| Risk | Impact | Mitigation |
|------|--------|------------|
| Memory leaks still present | **CRITICAL** | Run comprehensive tests, use pprof/memory_profiler |
| gRPC compatibility issues | High | Test with multiple gRPC versions |
| Flaky integration tests | Medium | Use retries, proper test isolation |
| Test environment setup complexity | Medium | Docker Compose for reproducibility |
| Slow E2E tests | Low | Use tiny Whisper model, short audio samples |

---

## Key Tests

### 1. gRPC Integration Test

**File:** `test/integration/test_grpc.go`

```go
func TestTranscribe_Integration(t *testing.T) {
    // Start Python worker
    worker := startTestWorker(t)
    defer worker.Stop()
    
    // gRPC client
    client := pb.NewTranscriptionServiceClient(conn)
    
    // Request
    req := &pb.TranscribeRequest{
        FilePath: "/testdata/short_audio.mp3",
        TaskType: "transcribe",
        Options: &pb.TranscribeOptions{
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
    assert.Contains(t, readFile(resp.SubtitlePath), "00:00:00")
}
```

---

### 2. Webhook End-to-End Test

**File:** `test/integration/test_webhooks.go`

```go
func TestPlexWebhook_E2E(t *testing.T) {
    // Start orchestrator + worker
    system := startTestSystem(t)
    defer system.Stop()
    
    // Send Plex webhook
    payload := map[string]interface{}{
        "event": "library.new",
        "Metadata": map[string]interface{}{
            "ratingKey": "12345",
        },
    }
    
    resp := sendWebhook(t, "http://localhost:9000/plex", payload)
    assert.Equal(t, 202, resp.StatusCode)
    
    // Wait for transcription to complete
    time.Sleep(30 * time.Second)
    
    // Verify subtitle file created
    subtitles := system.FindSubtitles("/testdata/short_audio.mp3")
    assert.NotEmpty(t, subtitles)
}
```

---

### 3. Memory Leak Test

**File:** `test/integration/test_memory_leaks.go`

```go
func TestNoMemoryLeak_1000Transcriptions(t *testing.T) {
    system := startTestSystem(t)
    defer system.Stop()
    
    // Baseline memory
    baselineOrch := getMemoryUsage(system.Orchestrator)
    baselineWorker := getMemoryUsage(system.Worker)
    
    // Process 1000 tasks
    for i := 0; i < 1000; i++ {
        sendWebhook(t, "http://localhost:9000/plex", makePayload())
        
        if i%100 == 0 {
            t.Logf("Processed %d tasks", i)
        }
    }
    
    // Wait for all to complete
    waitForQueueEmpty(t, system, 1*time.Hour)
    
    // Force GC
    system.ForceGC()
    time.Sleep(5 * time.Second)
    
    // Final memory
    finalOrch := getMemoryUsage(system.Orchestrator)
    finalWorker := getMemoryUsage(system.Worker)
    
    // Calculate growth
    orchGrowth := (finalOrch - baselineOrch) / baselineOrch
    workerGrowth := (finalWorker - baselineWorker) / baselineWorker
    
    // Assert < 20% growth
    assert.Less(t, orchGrowth, 0.20, "Orchestrator memory grew by %.1f%%", orchGrowth*100)
    assert.Less(t, workerGrowth, 0.20, "Worker memory grew by %.1f%%", workerGrowth*100)
}
```

---

## Testing Checklist

### gRPC Communication
- [ ] Transcribe RPC (success case)
- [ ] Transcribe RPC (file not found error)
- [ ] Transcribe RPC (timeout)
- [ ] DetectLanguage RPC (success case)
- [ ] DetectLanguage RPC (invalid audio)
- [ ] HealthCheck RPC (healthy)
- [ ] HealthCheck RPC (unhealthy - high memory)
- [ ] gRPC connection retry on failure

### Webhook Handlers
- [ ] Plex webhook (library.new event)
- [ ] Plex webhook (media.play event)
- [ ] Jellyfin webhook (ItemAdded event)
- [ ] Jellyfin webhook (PlaybackStart event)
- [ ] Emby webhook (library.new event)
- [ ] Tautulli webhook (added event)
- [ ] Invalid webhook payload (400 error)
- [ ] Missing authentication (401 error)

### End-to-End Pipeline
- [ ] Webhook → Queue → Worker → Subtitle file created
- [ ] SRT generation for video file
- [ ] LRC generation for audio file
- [ ] Multiple audio tracks handled correctly
- [ ] Language detection before transcription
- [ ] Skip conditions work (existing subtitles)
- [ ] Metadata refresh after transcription (Plex/Jellyfin)

### Memory Management
- [ ] 1000 transcriptions, memory growth < 20%
- [ ] Model cleanup after idle (30s delay)
- [ ] Model cleanup on high memory (immediate)
- [ ] No leaked file handles
- [ ] No leaked gRPC connections
- [ ] Goroutine count stable (Go orchestrator)

### Load & Performance
- [ ] 100 concurrent webhook requests
- [ ] Queue handles 1000+ tasks
- [ ] 24-hour soak test (zero crashes)
- [ ] Worker restart handled gracefully
- [ ] Orchestrator restart handled gracefully

---

## Definition of Done

- [ ] All 5 stories completed with ✅ status
- [ ] All integration tests passing
- [ ] All E2E tests passing
- [ ] Memory leak tests pass (< 20% growth)
- [ ] Load tests meet performance targets
- [ ] 24-hour soak test passes
- [ ] Test documentation complete
- [ ] CI/CD runs all tests automatically
- [ ] Work logs created for each story
- [ ] Test coverage report generated
- [ ] No flaky tests (100% pass rate on 10 runs)

---

## Next Epic

**EPIC_04: K8s Deployment (bjw-s)** - Deploy validated system to Kubernetes

---

## References

- README-LLM.md - Development workflow, testing requirements
- [00_HYBRID_ARCHITECTURE.md](../../DESIGN/00_HYBRID_ARCHITECTURE.md)
- [01_GRPC_PROTOCOL.md](../../DESIGN/01_GRPC_PROTOCOL.md)
- [02_MEMORY_MANAGEMENT.md](../../DESIGN/02_MEMORY_MANAGEMENT.md)

---

**Epic Owner:** TBD  
**Created:** 2026-02-15  
**Last Updated:** 2026-02-15
