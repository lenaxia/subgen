# Story 10: Blocking ASR Infrastructure

**Epic**: EPIC_08  
**Status**: Not Started  
**Effort**: 4-5 hours  
**Priority**: HIGH (Blocks STORY_05)  
**Assignee**: TBD

---

## User Story

As an ASR endpoint user (Bazarr or direct API consumer),
I want the `/asr` endpoint to block and return formatted subtitles immediately,
So that I receive transcription results in a single synchronous request.

---

## Background

The current ASR endpoint (`/asr`) queues transcription tasks but returns a placeholder message immediately. This prevents STORY_05 (ASR Format Selection) from being completed, as there's no mechanism to:

1. Wait for transcription to complete
2. Retrieve transcription results
3. Format and return results to the client

This story implements the **blocking infrastructure** needed to make ASR endpoint fully functional.

**Architectural Context**: The Go orchestrator uses an async queue+worker architecture, unlike the original Python version which had synchronous model access. This requires adding result channels to enable blocking operations.

---

## Acceptance Criteria

- [ ] Task struct supports result channels for blocking operations
- [ ] ASR endpoint blocks until transcription completes
- [ ] Timeout handling (30 second default, configurable)
- [ ] Concurrent ASR requests don't interfere with each other
- [ ] Worker processor sends results to result channels
- [ ] Memory leak prevention (channels cleaned up properly)
- [ ] Error handling returns clear messages to client
- [ ] Unit tests for blocking mechanism
- [ ] Integration tests with real worker
- [ ] Type checking passes
- [ ] Work log created

---

## Technical Design

### Architecture Change Overview

**Before (Current)**:
```
Client → POST /asr → Queue Task → Return "queued" → Client Done
                              ↓
                         Worker processes async
                              ↓
                         Result discarded (no listener)
```

**After (This Story)**:
```
Client → POST /asr → Queue Task → WAIT on ResultChan → Format → Return subtitles
                              ↓
                         Worker processes
                              ↓
                         Send result to ResultChan
```

---

### Implementation Components

#### 1. Add Result Channel to Task Struct

**File**: `orchestrator/internal/queue/task.go`

```go
// TranscriptionResult holds the result of a completed transcription
type TranscriptionResult struct {
    Segments []Segment    // Transcription segments
    Metadata Metadata     // Language, duration, etc.
    Error    error        // Error if transcription failed
}

// Segment represents a single subtitle segment
type Segment struct {
    Start float64
    End   float64
    Text  string
}

// Metadata holds transcription metadata
type Metadata struct {
    Language string
    Duration float64
    Model    string
}

// Add to Task struct
type Task struct {
    // ... existing fields ...
    
    // For blocking operations (ASR, detect language)
    // When set, worker sends result to this channel instead of writing file
    ResultChan chan *TranscriptionResult `json:"-"` // Don't serialize channel
}
```

**Rationale**: `ResultChan` is optional - nil for normal transcriptions (write to file), non-nil for blocking operations (return to client).

---

#### 2. Update handleASR to Block

**File**: `orchestrator/internal/webhooks/server.go`

```go
func (s *Server) handleASR(c *fiber.Ctx) error {
    // ... existing validation (lines 631-677) ...
    
    // Validate output format (for STORY_05 completion)
    output := c.Query("output", "srt")
    
    // ... existing file handling (lines 680-728) ...
    
    // Create result channel for blocking
    resultChan := make(chan *TranscriptionResult, 1) // Buffered to prevent worker blocking
    
    // Create ASR task with result channel
    task := Task{
        FilePath:          videoFile,
        TranscriptionType: taskType,
        ForceLanguage:     language,
        AudioContent:      audioContent,
        ASROptions: map[string]string{
            "output": output,
        },
        ResultChan: resultChan, // NEW: Enable blocking
    }
    
    // Queue task
    if err := s.queue.Enqueue(task); err != nil {
        close(resultChan) // Clean up channel
        s.log.WithError(err).Error("Failed to enqueue ASR task")
        return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
            "error": "Failed to queue task",
        })
    }
    
    s.log.WithFields(map[string]interface{}{
        "video_file": videoFile,
        "format":     output,
    }).Info("ASR task queued, waiting for result")
    
    // Block until result ready or timeout
    timeout := 30 * time.Second
    if s.config.ASR.Timeout > 0 {
        timeout = s.config.ASR.Timeout
    }
    
    select {
    case result := <-resultChan:
        // Handle transcription error
        if result.Error != nil {
            s.log.WithError(result.Error).Error("ASR transcription failed")
            return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
                "error": fmt.Sprintf("transcription failed: %v", result.Error),
            })
        }
        
        // TODO STORY_05: Convert segments to requested format
        // For now, return placeholder (STORY_05 will implement format conversion)
        s.log.WithFields(map[string]interface{}{
            "segments": len(result.Segments),
            "language": result.Metadata.Language,
            "duration": result.Metadata.Duration,
        }).Info("ASR transcription completed")
        
        return c.SendString(fmt.Sprintf(
            "Transcription completed: %d segments, language: %s (format conversion TODO in STORY_05)",
            len(result.Segments),
            result.Metadata.Language,
        ))
        
    case <-time.After(timeout):
        s.log.WithField("timeout", timeout).Warn("ASR transcription timeout")
        return c.Status(fiber.StatusGatewayTimeout).JSON(fiber.Map{
            "error": fmt.Sprintf("transcription timeout after %v", timeout),
        })
    }
}
```

**Note**: Format conversion (using format writers) will be added in STORY_05 Phase 3. This story focuses on the blocking mechanism only.

---

#### 3. Add Configuration for ASR Timeout

**File**: `orchestrator/internal/config/config.go`

```go
type Config struct {
    // ... existing fields ...
    
    ASR struct {
        Timeout time.Duration `env:"ASR_TIMEOUT" envDefault:"30s"`
    } `envPrefix:"ASR_"`
}
```

**Environment Variable**:
```bash
ASR_TIMEOUT=45s  # Override default 30 second timeout
```

---

#### 4. Update Worker Processor to Send Results

**Option A: Modify Existing Queue Dequeue Loop**

If orchestrator has a queue processor loop that dequeues tasks and sends to workers:

```go
// In queue processor loop
func (p *Processor) processTasks(ctx context.Context) {
    for {
        // Dequeue task
        task, err := p.queue.Dequeue()
        if err != nil {
            time.Sleep(100 * time.Millisecond)
            continue
        }
        
        // Process task
        go p.processTask(ctx, task)
    }
}

func (p *Processor) processTask(ctx context.Context, task *queue.Task) {
    // Select worker
    worker, err := p.workerPool.SelectWorker()
    if err != nil {
        p.sendTaskResult(task, nil, err)
        p.queue.MarkFailed(task.ID, err)
        return
    }
    
    // Call worker gRPC
    resp, err := p.grpcClient.Transcribe(ctx, worker.Address, task.FilePath, task.TaskType, task.ForceLanguage)
    if err != nil {
        p.sendTaskResult(task, nil, err)
        p.queue.MarkFailed(task.ID, err)
        return
    }
    
    // Convert gRPC response to result
    result := &queue.TranscriptionResult{
        Segments: convertSegments(resp.Segments),
        Metadata: queue.Metadata{
            Language: resp.DetectedLanguage,
            Duration: resp.Duration,
            Model:    resp.Model,
        },
    }
    
    // Send result to result channel if blocking operation
    p.sendTaskResult(task, result, nil)
    
    // For non-blocking operations, write subtitle file
    if task.ResultChan == nil {
        p.writeSubtitleFile(task, result)
    }
    
    p.queue.MarkDone(task.ID)
}

// Helper to send result to channel if present
func (p *Processor) sendTaskResult(task *queue.Task, result *queue.TranscriptionResult, err error) {
    if task.ResultChan == nil {
        return // Non-blocking operation, no channel to send to
    }
    
    defer close(task.ResultChan) // Always close channel when done
    
    if err != nil {
        task.ResultChan <- &queue.TranscriptionResult{Error: err}
    } else {
        task.ResultChan <- result
    }
}
```

**Option B: Create New Worker Package** (if no processor exists yet):

See work log for full implementation template.

---

### Error Handling

**Scenario 1: Worker Failure**
```go
// Worker returns error
result := &TranscriptionResult{Error: fmt.Errorf("worker crashed")}
task.ResultChan <- result
close(task.ResultChan)

// Client receives 500 error
{"error": "transcription failed: worker crashed"}
```

**Scenario 2: Timeout**
```go
// After 30 seconds, no result received
// select case <-time.After(timeout) triggers
return c.Status(504).JSON(fiber.Map{
    "error": "transcription timeout after 30s",
})
```

**Scenario 3: Channel Memory Leak Prevention**
```go
// Always close channel in worker, even on error
defer close(task.ResultChan)

// Always read from channel or timeout in handler
select {
case result := <-resultChan: // Will unblock when closed
case <-time.After(timeout):  // Fallback
}
```

---

### Concurrency Safety

**Multiple Concurrent ASR Requests**:
- Each request creates its own result channel ✅
- Channels are independent, no shared state ✅
- Worker sends result to correct channel via task reference ✅

**Channel Cleanup**:
- Worker closes channel after sending result ✅
- Handler reads channel or times out ✅
- Garbage collector cleans up closed channels ✅

---

## Testing Strategy

### Unit Tests

**test/unit/asr_blocking_test.go:**
```go
package webhooks_test

import (
    "testing"
    "time"
    "github.com/mccloud/subgen/orchestrator/internal/queue"
)

func TestHandleASR_BlocksUntilCompletion(t *testing.T) {
    // Setup mock queue that simulates successful transcription
    mockQueue := &MockQueue{
        simulateDelay: 500 * time.Millisecond,
    }
    
    server := NewTestServer(mockQueue)
    
    // Make ASR request
    start := time.Now()
    resp := makeASRRequest(server, "test.mp3")
    elapsed := time.Since(start)
    
    // Verify request blocked for ~500ms
    assert.GreaterOrEqual(t, elapsed, 500*time.Millisecond)
    assert.Equal(t, 200, resp.StatusCode)
}

func TestHandleASR_TimeoutAfter30Seconds(t *testing.T) {
    // Setup mock queue that never completes
    mockQueue := &MockQueue{
        simulateDelay: 60 * time.Second, // Longer than timeout
    }
    
    server := NewTestServer(mockQueue)
    server.config.ASR.Timeout = 1 * time.Second // Fast timeout for test
    
    // Make ASR request
    start := time.Now()
    resp := makeASRRequest(server, "test.mp3")
    elapsed := time.Since(start)
    
    // Verify request timed out after ~1s
    assert.LessOrEqual(t, elapsed, 1500*time.Millisecond)
    assert.Equal(t, 504, resp.StatusCode)
    assert.Contains(t, resp.Body, "timeout")
}

func TestHandleASR_HandlesWorkerError(t *testing.T) {
    // Setup mock queue that returns error
    mockQueue := &MockQueue{
        simulateError: errors.New("worker crashed"),
    }
    
    server := NewTestServer(mockQueue)
    
    // Make ASR request
    resp := makeASRRequest(server, "test.mp3")
    
    // Verify error returned to client
    assert.Equal(t, 500, resp.StatusCode)
    assert.Contains(t, resp.Body, "transcription failed")
    assert.Contains(t, resp.Body, "worker crashed")
}

func TestHandleASR_ConcurrentRequests(t *testing.T) {
    // Setup mock queue
    mockQueue := &MockQueue{
        simulateDelay: 100 * time.Millisecond,
    }
    
    server := NewTestServer(mockQueue)
    
    // Make 10 concurrent ASR requests
    var wg sync.WaitGroup
    results := make([]int, 10)
    
    for i := 0; i < 10; i++ {
        wg.Add(1)
        go func(idx int) {
            defer wg.Done()
            resp := makeASRRequest(server, fmt.Sprintf("test%d.mp3", idx))
            results[idx] = resp.StatusCode
        }(i)
    }
    
    wg.Wait()
    
    // Verify all requests succeeded independently
    for i, statusCode := range results {
        assert.Equal(t, 200, statusCode, "Request %d failed", i)
    }
}

func TestHandleASR_ChannelCleanup(t *testing.T) {
    // This test verifies no channel memory leaks
    // Run with -race detector: go test -race
    
    mockQueue := &MockQueue{
        simulateDelay: 10 * time.Millisecond,
    }
    
    server := NewTestServer(mockQueue)
    
    // Make many requests to detect potential leaks
    for i := 0; i < 100; i++ {
        resp := makeASRRequest(server, "test.mp3")
        assert.Equal(t, 200, resp.StatusCode)
    }
    
    // If there are leaks, race detector will report them
}
```

---

### Integration Tests

**test/integration/asr_blocking_integration_test.go:**
```go
func TestASR_Integration_BlockingWithRealWorker(t *testing.T) {
    // Start real worker
    worker := startWorker(t)
    defer worker.Stop()
    
    // Start orchestrator
    orchestrator := startOrchestrator(t, worker.Address)
    defer orchestrator.Stop()
    
    // Upload real audio file
    audioFile := loadTestAudio(t, "test.mp3")
    
    // Make ASR request
    start := time.Now()
    resp := uploadASR(t, orchestrator, audioFile, "srt")
    elapsed := time.Since(start)
    
    // Verify response
    assert.Equal(t, 200, resp.StatusCode)
    assert.Less(t, elapsed, 30*time.Second, "Should complete within timeout")
    
    // Verify result contains subtitle content (placeholder until STORY_05)
    assert.Contains(t, resp.Body, "segments")
    
    t.Logf("ASR request completed in %v", elapsed)
}

func TestASR_Integration_TimeoutWithSlowWorker(t *testing.T) {
    // Start worker that takes 60 seconds
    worker := startSlowWorker(t, 60*time.Second)
    defer worker.Stop()
    
    // Start orchestrator with 5 second timeout
    cfg := &config.Config{
        ASR: config.ASRConfig{
            Timeout: 5 * time.Second,
        },
    }
    orchestrator := startOrchestratorWithConfig(t, worker.Address, cfg)
    defer orchestrator.Stop()
    
    // Make ASR request
    audioFile := loadTestAudio(t, "test.mp3")
    resp := uploadASR(t, orchestrator, audioFile, "srt")
    
    // Verify timeout
    assert.Equal(t, 504, resp.StatusCode)
    assert.Contains(t, resp.Body, "timeout")
}
```

---

### Manual Testing

```bash
# Test 1: Successful blocking request
curl -X POST "http://localhost:9000/asr?task=transcribe&language=en" \
  -F "audio_file=@test_audio.mp3" \
  --max-time 60
# Expected: Blocks for 5-10 seconds, returns result

# Test 2: Timeout
curl -X POST "http://localhost:9000/asr?task=transcribe&language=en" \
  -F "audio_file=@very_long_audio.mp3" \
  --max-time 35
# Expected: Times out after 30 seconds with 504 error

# Test 3: Concurrent requests
for i in {1..5}; do
  curl -X POST "http://localhost:9000/asr?task=transcribe" \
    -F "audio_file=@test$i.mp3" &
done
wait
# Expected: All 5 requests succeed independently

# Test 4: Custom timeout
ASR_TIMEOUT=60s ./orchestrator &
curl -X POST "http://localhost:9000/asr" -F "audio_file=@long.mp3"
# Expected: Times out after 60 seconds instead of 30
```

---

## Integration with STORY_05

This story provides the **infrastructure** for STORY_05. Once complete, STORY_05 can be finished by:

1. Using format writers to convert `result.Segments` to requested format
2. Setting Content-Type headers via `getContentType()` helper
3. Returning formatted subtitle string instead of placeholder

**STORY_05 Phase 3 Dependencies**:
- ✅ This story (STORY_10) must be complete first
- ✅ Format writers from STORY_01 (already complete)

---

## Performance Considerations

**Blocking Request Limits**:
- ASR endpoint will block HTTP connection for transcription duration
- Typical transcription: 5-15 seconds for 5-minute audio
- With 30s timeout, no request blocks longer than 30s
- Recommendation: Use reverse proxy timeout > 30s (e.g., nginx: 60s)

**Concurrent Request Limits**:
- Each blocking request holds one HTTP connection
- Fiber default: 256k concurrent connections
- Practical limit: Number of workers × concurrent transcriptions per worker
- Example: 2 workers × 1 concurrent = 2 blocking requests max

**Memory Usage**:
- Each blocking request: ~1KB (result channel + context)
- 100 concurrent requests: ~100KB overhead
- Negligible compared to audio file sizes (MB range)

---

## Definition of Done

- [ ] Story file created (this document)
- [ ] Task struct has ResultChan field
- [ ] ASR endpoint blocks with timeout
- [ ] Configuration for ASR timeout (default 30s)
- [ ] Worker processor sends results to result channels
- [ ] Channel cleanup (no memory leaks)
- [ ] Unit tests for blocking mechanism (5 tests)
- [ ] Integration tests with real worker (2 tests)
- [ ] Manual testing completed (4 scenarios)
- [ ] Type checking passes (`go build`)
- [ ] Race detector passes (`go test -race`)
- [ ] Work log created
- [ ] Code committed and pushed

---

## Backward Compatibility

**Non-breaking Change**:
- Regular transcription tasks (Plex, Jellyfin, etc.) have `ResultChan = nil`
- Worker checks `if task.ResultChan != nil` before sending
- Existing flows unaffected ✅

**New Behavior**:
- ASR endpoint now blocks instead of returning immediately
- Existing ASR clients may need timeout adjustments
- Recommendation: Document in changelog/migration guide

---

## Dependencies

**Requires:**
- EPIC_01 (Go Orchestrator) - ✅ Complete
- STORY_01 (Multiple Output Formats) - ✅ Complete (for STORY_05 Phase 3)
- Worker gRPC client - ✅ Complete

**Enables:**
- STORY_05 Phase 3 (ASR Format Selection completion)

---

## Success Criteria

1. **Blocking Works**: ASR endpoint blocks until transcription completes
2. **Timeout Works**: Requests timeout after 30 seconds (configurable)
3. **Concurrency Works**: Multiple concurrent ASR requests succeed independently
4. **No Memory Leaks**: Channels cleaned up properly (race detector passes)
5. **Error Handling**: Clear error messages returned to client
6. **Performance**: Blocking overhead < 1ms (channel communication time)

---

## References

- **Gap Analysis**: `docs/WORKLOGS/0032_2026-02-16_epic08_story05_architectural_gap_analysis.md`
- **STORY_05**: `docs/BACKLOG/EPIC_08/stories/STORY_05_asr_format_selection.md`
- **Current ASR Handler**: `orchestrator/internal/webhooks/server.go:629-760`
- **Task Struct**: `orchestrator/internal/queue/task.go`
- **Original Python ASR**: `subgen.py` lines 687-692 (synchronous blocking behavior)

---

**Story Created**: 2026-02-16  
**Last Updated**: 2026-02-16  
**Status**: Not Started  
**Blocks**: STORY_05 (ASR Format Selection)
