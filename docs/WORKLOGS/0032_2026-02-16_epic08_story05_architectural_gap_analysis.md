# Work Log: EPIC_08 STORY_05 - Architectural Gap Analysis & Fix Plan

**Date**: 2026-02-16
**Author**: AI Assistant (Claude)
**Epic/Story**: EPIC_08 / STORY_05 - ASR Format Selection
**Status**: BLOCKED - Requires Architectural Changes

---

## Executive Summary

STORY_05 (ASR Format Selection) validation revealed **critical architectural gaps** that prevent the story from being completable as currently designed. The ASR endpoint validates the format parameter but returns placeholder text instead of formatted subtitles. This is because the current architecture **queues tasks without blocking for results**.

**Decision**: Document the architectural gap and create a follow-up story for blocking ASR implementation. This is the correct approach rather than attempting a major architectural change within STORY_05's scope.

---

## Validation Findings

### GAP #1: Format Writers NOT Used in ASR Response ❌

**Location**: `orchestrator/internal/webhooks/server.go:629-760` (handleASR function)

**Current Behavior**:
```go
// Line 759: Returns placeholder instead of formatted subtitles
return c.SendString("ASR task queued successfully (placeholder response)")
```

**Expected Behavior** (per STORY_05):
```go
// Should return formatted subtitles like:
c.Set("Content-Type", "text/vtt; charset=utf-8")
return c.SendString(vttFormattedSubtitles)
```

**Root Cause**: No mechanism to wait for transcription completion and retrieve results.

---

### GAP #2: Content-Type Headers NOT Set ❌

**Location**: `orchestrator/internal/webhooks/server.go:792-802` (getContentType helper)

**Current State**:
- Helper function `getContentType()` exists (lines 792-802)
- Function correctly maps format → Content-Type
- **Function is NEVER CALLED** ❌

**Impact**: Clients always receive `text/plain` (Fiber default), even when requesting VTT format.

---

### GAP #3: No Blocking Mechanism

**Current Flow**:
```
Client → POST /asr → Queue Task → Return "queued successfully" → Client Done
                              ↓
                         Worker processes later (async)
                              ↓
                         Result discarded (no listener)
```

**Required Flow**:
```
Client → POST /asr → Queue Task → WAIT for worker → Format result → Return subtitles → Client Done
                              ↓
                         Worker processes
                              ↓
                         Send result back via channel
```

**Architectural Gap**: Task struct has no result channel, workers have no callback mechanism.

---

## Root Cause Analysis

### Why Can't We Just "Fix" It?

The issue is **NOT** a simple bug fix. It requires architectural changes:

1. **Task Struct Modification** (`orchestrator/internal/queue/task.go`):
   ```go
   type Task struct {
       // ... existing fields ...
       
       // NEW: Result channel for blocking operations
       ResultChan chan *TranscriptionResult  // ← Architectural change
   }
   ```

2. **Worker Communication** (`orchestrator/internal/queue/queue.go`):
   - Workers need to send results back
   - Queue processor needs to route results to waiting handlers
   - Timeout handling for slow transcriptions

3. **Server State Management** (`orchestrator/internal/webhooks/server.go`):
   - Track in-flight ASR requests
   - Handle timeouts and cancellations
   - Prevent memory leaks from abandoned channels

**Effort Estimate**: 6-8 hours (double the STORY_05 estimate)

---

## Comparison with Original Python Implementation

### Original subgen.py ASR Endpoint

**Lines 687-692** (original Python code):
```python
@app.route('/asr', methods=['POST'])
def asr():
    # Upload received
    audio_file = request.files['audio_file']
    
    # BLOCKS until transcription completes
    result = model.transcribe(audio_file.filename, task=task, language=language)
    
    # Format and return immediately
    subtitles = format_segments(result['segments'], output_format)
    return subtitles, 200, {'Content-Type': get_content_type(output_format)}
```

**Key Difference**: Python version has synchronous model access, can block directly.

**Go Version Challenge**: Distributed architecture with async workers requires result channel mechanism.

---

## Proposed Solutions

### Option A: Implement Full Blocking ASR (Recommended)

**Scope**: Create new story "EPIC_08 STORY_10: Blocking ASR Implementation"

**Changes Required**:
1. Add `ResultChan` field to Task struct
2. Implement result routing in queue processor
3. Update handleASR to block with timeout
4. Integrate format writers (STORY_05 completion)
5. Add Content-Type headers (STORY_05 completion)

**Effort**: 6-8 hours

**Benefits**:
- ✅ Completes STORY_05 functionality
- ✅ Enables synchronous API pattern for ASR
- ✅ Matches original Python behavior
- ✅ Better user experience (immediate response)

**Risks**:
- Architectural complexity increase
- Need timeout handling to prevent hung requests
- Memory management for result channels

---

### Option B: Document as Future Work (Current Approach)

**Scope**: Update STORY_05 to mark as "Partial Implementation"

**Changes**:
1. Add format validation (already done) ✅
2. Store format in task options (already done) ✅
3. Document blocking requirement as follow-up
4. Create tracking issue for blocking ASR

**Effort**: 1 hour (documentation only)

**Benefits**:
- ✅ Honest about current limitations
- ✅ Avoids scope creep
- ✅ Clear path forward documented

**Drawbacks**:
- ❌ ASR endpoint remains non-functional for real use
- ❌ STORY_05 acceptance criteria not met

---

### Option C: Remove ASR Endpoint (Not Recommended)

Mark ASR endpoint as "experimental" and disable until blocking implementation complete.

**Rejected**: Endpoint is already partially functional, removal would be user-hostile.

---

## Decision: Option A with Phased Implementation

### Phase 1: Document Gap (This Work Log)

**Status**: ✅ COMPLETE

**Deliverables**:
- This work log documenting the architectural gap
- Updated STORY_05 status to "BLOCKED"
- Created follow-up story specification

---

### Phase 2: Implement Blocking ASR Infrastructure (NEW STORY)

**Story**: EPIC_08 STORY_10 - Blocking ASR Infrastructure

**Acceptance Criteria**:
- [ ] Add ResultChan to Task struct
- [ ] Implement worker result routing
- [ ] Add timeout handling (30 seconds default)
- [ ] Handle concurrent ASR requests
- [ ] Memory leak prevention for abandoned requests
- [ ] Unit tests for blocking mechanism
- [ ] Integration tests with real worker

**Effort**: 4-5 hours

---

### Phase 3: Complete STORY_05 Implementation

**Acceptance Criteria** (from original story):
- [ ] Use format writers to convert segments to requested format
- [ ] Set Content-Type headers via getContentType() helper
- [ ] Return formatted subtitles instead of placeholder
- [ ] All three formats work: SRT, VTT, LRC
- [ ] Tests updated to verify actual output format

**Effort**: 1-2 hours (now that blocking infrastructure exists)

---

## Implementation Plan for Blocking ASR (Phase 2)

### Step 1: Add Result Channel to Task Struct

**File**: `orchestrator/internal/queue/task.go`

```go
// TranscriptionResult holds the result of a completed transcription
type TranscriptionResult struct {
    Segments []Segment
    Metadata Metadata
    Error    error
}

// Add to Task struct
type Task struct {
    // ... existing fields ...
    
    // For blocking operations (ASR, detect language)
    ResultChan chan *TranscriptionResult `json:"-"` // Don't serialize channel
}
```

---

### Step 2: Update Server to Block on ASR

**File**: `orchestrator/internal/webhooks/server.go`

```go
func (s *Server) handleASR(c *fiber.Ctx) error {
    // ... existing validation (lines 631-677) ...
    
    // Validate output format (STORY_05)
    output := c.Query("output", "srt")
    validFormats := map[string]bool{
        "srt":  true,
        "vtt":  true,
        "lrc":  true,
        "txt":  true,
        "tsv":  true,
        "json": true,
    }
    if !validFormats[output] {
        return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{
            "error": fmt.Sprintf("invalid format: %s (supported: srt, vtt, lrc, txt, tsv, json)", output),
        })
    }
    
    // ... existing file handling (lines 680-728) ...
    
    // Create result channel for blocking
    resultChan := make(chan *TranscriptionResult, 1)
    
    // Create ASR task
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
        close(resultChan)
        s.log.WithError(err).Error("Failed to enqueue ASR task")
        return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
            "error": "Failed to queue task",
        })
    }
    
    s.log.WithField("video_file", videoFile).Info("ASR task queued, waiting for result")
    
    // Block until result ready or timeout
    timeout := 30 * time.Second
    if s.config.ASR.Timeout > 0 {
        timeout = s.config.ASR.Timeout
    }
    
    select {
    case result := <-resultChan:
        if result.Error != nil {
            return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
                "error": fmt.Sprintf("transcription failed: %v", result.Error),
            })
        }
        
        // Convert segments to requested format using format writers (STORY_05)
        var buffer bytes.Buffer
        writer, err := formats.NewWriter(output)
        if err != nil {
            return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
                "error": fmt.Sprintf("unsupported format: %s", output),
            })
        }
        
        if err := writer.Write(&buffer, result.Segments, result.Metadata); err != nil {
            return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
                "error": fmt.Sprintf("format conversion failed: %v", err),
            })
        }
        
        // Set Content-Type header (STORY_05)
        c.Set("Content-Type", getContentType(output))
        
        // Return formatted subtitles (STORY_05)
        return c.SendString(buffer.String())
        
    case <-time.After(timeout):
        return c.Status(fiber.StatusGatewayTimeout).JSON(fiber.Map{
            "error": fmt.Sprintf("transcription timeout after %v", timeout),
        })
    }
}
```

---

### Step 3: Update Queue Processor to Send Results

**File**: `orchestrator/internal/worker/processor.go` (NEW FILE)

```go
package worker

import (
    "context"
    "github.com/mccloud/subgen/orchestrator/internal/queue"
    pb "github.com/mccloud/subgen/orchestrator/pkg/pb"
)

// Processor handles task execution via worker pool
type Processor struct {
    queue      queue.QueueInterface
    workerPool WorkerPoolInterface
    grpcClient GRPCClientInterface
}

// ProcessTask executes a task and sends result to ResultChan if present
func (p *Processor) ProcessTask(ctx context.Context, task *queue.Task) error {
    // Select worker
    worker, err := p.workerPool.SelectWorker()
    if err != nil {
        if task.ResultChan != nil {
            task.ResultChan <- &queue.TranscriptionResult{Error: err}
            close(task.ResultChan)
        }
        return err
    }
    
    // Call worker gRPC
    resp, err := p.grpcClient.Transcribe(ctx, worker.Address, task.FilePath, task.TaskType, task.ForceLanguage)
    if err != nil {
        if task.ResultChan != nil {
            task.ResultChan <- &queue.TranscriptionResult{Error: err}
            close(task.ResultChan)
        }
        return err
    }
    
    // Send result to result channel (for blocking operations)
    if task.ResultChan != nil {
        task.ResultChan <- &queue.TranscriptionResult{
            Segments: convertSegments(resp.Segments),
            Metadata: convertMetadata(resp.Metadata),
        }
        close(task.ResultChan)
    }
    
    return nil
}
```

---

### Step 4: Add Configuration for ASR Timeout

**File**: `orchestrator/internal/config/config.go`

```go
type Config struct {
    // ... existing fields ...
    
    ASR struct {
        Timeout time.Duration `env:"ASR_TIMEOUT" envDefault:"30s"`
    }
}
```

---

## Testing Strategy

### Unit Tests (Phase 2)

```go
// Test blocking mechanism
func TestHandleASR_BlocksUntilCompletion(t *testing.T)
func TestHandleASR_TimeoutAfter30Seconds(t *testing.T)
func TestHandleASR_HandlesWorkerError(t *testing.T)
func TestHandleASR_ConcurrentRequests(t *testing.T)
```

### Unit Tests (Phase 3 - STORY_05)

```go
// Test format selection
func TestHandleASR_ReturnsVTTFormat(t *testing.T)
func TestHandleASR_ReturnsSRTFormat(t *testing.T)
func TestHandleASR_ReturnsLRCFormat(t *testing.T)
func TestHandleASR_SetsCorrectContentType(t *testing.T)
func TestHandleASR_InvalidFormatReturns400(t *testing.T)
```

### Integration Tests

```bash
# Test with real worker
curl -X POST "http://localhost:9000/asr?output=vtt" \
  -F "audio_file=@test.mp3" \
  -H "Content-Type: multipart/form-data"

# Expected: Valid VTT format with WEBVTT header
# Content-Type: text/vtt; charset=utf-8
```

---

## Success Criteria

### Phase 2 (Blocking Infrastructure)

- [ ] ASR endpoint blocks until transcription completes
- [ ] Timeout handling works (30 second default)
- [ ] Concurrent requests don't interfere with each other
- [ ] No memory leaks from abandoned channels
- [ ] Error handling returns clear messages
- [ ] Tests pass with real worker

### Phase 3 (STORY_05 Completion)

- [ ] Format writers used to convert segments
- [ ] Content-Type headers set correctly
- [ ] All 6 formats work: SRT, VTT, LRC, TXT, TSV, JSON
- [ ] Invalid format returns 400 with clear error
- [ ] Tests verify actual output format (not just queuing)

---

## Definition of Done

**Phase 2 (Blocking ASR):**
- [ ] Task struct has ResultChan field
- [ ] Worker processor sends results to ResultChan
- [ ] handleASR blocks with timeout
- [ ] Configuration for ASR timeout
- [ ] Unit tests for blocking mechanism
- [ ] Integration tests with real worker
- [ ] Work log created

**Phase 3 (STORY_05):**
- [ ] Format validation implemented
- [ ] Format writers integrated
- [ ] Content-Type headers set
- [ ] Tests updated to verify output format
- [ ] Manual testing with all formats
- [ ] STORY_05 marked as complete
- [ ] Work log updated

---

## Timeline

**Phase 1**: ✅ COMPLETE (this work log)
**Phase 2**: 4-5 hours (blocking infrastructure)
**Phase 3**: 1-2 hours (STORY_05 completion)
**Total**: 5-7 hours

---

## References

- **Original Story**: `docs/BACKLOG/EPIC_08/stories/STORY_05_asr_format_selection.md`
- **Current ASR Handler**: `orchestrator/internal/webhooks/server.go:629-760`
- **Format Writers**: `orchestrator/pkg/formats/` (STORY_01)
- **Task Struct**: `orchestrator/internal/queue/task.go`
- **Previous Work Log**: `docs/WORKLOGS/0031_2026-02-16_epic08_story05_asr_format_notes.md`

---

## Conclusion

STORY_05 (ASR Format Selection) **cannot be completed** without first implementing blocking ASR infrastructure. This is an architectural requirement, not a simple feature addition.

**Recommended Path Forward**:
1. ✅ Document the gap (this work log)
2. Create STORY_10 specification for blocking ASR
3. Implement blocking infrastructure (Phase 2)
4. Complete STORY_05 implementation (Phase 3)

This phased approach:
- ✅ Maintains honest about current limitations
- ✅ Provides clear implementation path
- ✅ Avoids rushing architectural changes
- ✅ Delivers working feature at the end

**Status**: STORY_05 marked as BLOCKED, awaiting Phase 2 implementation.

---

**Work Log Created**: 2026-02-16
**Author**: AI Assistant (Claude)
**Epic/Story**: EPIC_08 / STORY_05
