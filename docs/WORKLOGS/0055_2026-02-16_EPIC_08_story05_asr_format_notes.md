# Work Log: EPIC_08 STORY_05 - ASR Format Selection (Partial)

**Date**: 2026-02-16
**Author**: AI Assistant (Claude)
**Epic/Story**: EPIC_08 / STORY_05
**Status**: Partial - Infrastructure Ready, Awaiting Full ASR Implementation

---

## Summary

STORY_05 requires adding format selection to the ASR endpoint (`?output=srt|vtt|lrc`). Investigation revealed that the ASR endpoint already captures the output parameter and stores it in task options, but the endpoint currently returns a placeholder response rather than blocking and returning actual subtitle content. Full implementation of STORY_05 is blocked on implementing the complete ASR blocking mechanism.

---

## Current State

### Existing Infrastructure (Already Implemented)

**orchestrator/internal/webhooks/server.go (lines 490-605):**
- `handleASR` function accepts `?output=` query parameter (line 495)
- Output parameter stored in `task.ASROptions["output"]` (line 587-589)
- Default value is "srt" if not specified
- Returns placeholder message: "ASR task queued successfully (placeholder response)" (line 604)

**orchestrator/internal/webhooks/asr_test.go:**
- `TestHandleASR_DifferentOutputFormats` (lines 103-139) tests srt, vtt, txt, json, tsv formats
- Tests verify output parameter is accepted and task is queued
- Tests don't verify actual format conversion (because endpoint doesn't return subtitles yet)

**orchestrator/pkg/formats/ (STORY_01 - Complete):**
- SRT writer implemented
- VTT writer implemented  
- LRC writer implemented
- TXT, TSV, JSON writers implemented
- All writers tested and functional

### What's Missing for STORY_05

1. **Blocking ASR Mechanism**
   - Current implementation queues task and returns immediately
   - Need to implement blocking wait for task completion
   - Need result channel or similar mechanism to wait for worker response

2. **Format Conversion on Response**
   - Worker returns transcription segments
   - Need to convert segments to requested format (srt/vtt/lrc) before returning
   - Need to set appropriate Content-Type header based on format

3. **Integration with Format Writers**
   - Use format writers from `orchestrator/pkg/formats/`
   - Convert worker response to selected format
   - Stream formatted subtitle to client

4. **Format Validation**
   - Validate output parameter is one of: srt, vtt, lrc
   - Return 400 error for invalid formats
   - Document supported formats

---

## Recommended Implementation Approach

### Phase 1: Result Channel Infrastructure

```go
// Add to server.go
type ASRResultChannel struct {
    done   chan struct{}
    result []byte
    err    error
    format string
}

// Map of task ID to result channel
asrResults map[string]*ASRResultChannel
asrResultsMu sync.RWMutex
```

### Phase 2: Update handleASR to Block

```go
func (s *Server) handleASR(c *fiber.Ctx) error {
    // ... existing validation ...
    
    // Validate output format
    output := c.Query("output", "srt")
    validFormats := map[string]bool{"srt": true, "vtt": true, "lrc": true}
    if !validFormats[output] {
        return c.Status(400).JSON(fiber.Map{
            "error": fmt.Sprintf("invalid output format: %s (must be srt, vtt, or lrc)", output),
        })
    }
    
    // Generate task ID
    taskID := generateTaskID(audioContent, taskType, language)
    
    // Create result channel
    resultChan := &ASRResultChannel{
        done:   make(chan struct{}),
        format: output,
    }
    s.storeASRResult(taskID, resultChan)
    defer s.removeASRResult(taskID)
    
    // Queue task
    task := Task{
        ID: taskID,
        // ... existing fields ...
    }
    s.queue.Enqueue(task)
    
    // Block until result ready or timeout
    select {
    case <-resultChan.done:
        if resultChan.err != nil {
            return c.Status(500).JSON(fiber.Map{"error": resultChan.err.Error()})
        }
        
        // Set Content-Type based on format
        contentType := getContentType(output)
        c.Set("Content-Type", contentType)
        
        return c.Send(resultChan.result)
        
    case <-time.After(s.config.ASR.Timeout):
        return c.Status(504).JSON(fiber.Map{"error": "transcription timeout"})
    }
}
```

### Phase 3: Worker Completion Handler

```go
// When worker completes transcription, call this
func (s *Server) completeASRTask(taskID string, segments []Segment, err error) {
    s.asrResultsMu.Lock()
    resultChan, exists := s.asrResults[taskID]
    s.asrResultsMu.Unlock()
    
    if !exists {
        return // Task was cancelled or timed out
    }
    
    if err != nil {
        resultChan.err = err
        close(resultChan.done)
        return
    }
    
    // Convert segments to requested format
    var output bytes.Buffer
    switch resultChan.format {
    case "vtt":
        writer := formats.NewVTTWriter()
        err = writer.Write(&output, segments, metadata)
    case "lrc":
        writer := formats.NewLRCWriter()
        err = writer.Write(&output, segments, metadata)
    default: // srt
        writer := formats.NewSRTWriter()
        err = writer.Write(&output, segments, metadata)
    }
    
    if err != nil {
        resultChan.err = err
    } else {
        resultChan.result = output.Bytes()
    }
    
    close(resultChan.done)
}
```

### Phase 4: Content-Type Headers

```go
func getContentType(format string) string {
    switch format {
    case "vtt":
        return "text/vtt; charset=utf-8"
    case "lrc":
        return "text/plain; charset=utf-8"
    default: // srt
        return "text/plain; charset=utf-8"
    }
}
```

---

## Testing Strategy (When Implemented)

### Unit Tests to Add

```go
func TestHandleASR_OutputFormat_VTT(t *testing.T)
func TestHandleASR_OutputFormat_LRC(t *testing.T)
func TestHandleASR_OutputFormat_Default(t *testing.T)
func TestHandleASR_OutputFormat_Invalid(t *testing.T)
func TestHandleASR_ContentType_VTT(t *testing.T)
func TestHandleASR_ContentType_SRT(t *testing.T)
func TestHandleASR_ContentType_LRC(t *testing.T)
func TestHandleASR_BlocksUntilCompletion(t *testing.T)
func TestHandleASR_Timeout(t *testing.T)
```

### Integration Tests

- Upload audio, request VTT format, verify response is valid WebVTT
- Upload audio, request LRC format, verify response is valid LRC
- Upload same audio twice, verify deduplication works with format parameter
- Test timeout behavior with slow workers

---

## Estimated Effort

**Remaining Work**: 3-4 hours

- Implement result channel infrastructure: 1 hour
- Update handleASR to block and return results: 1 hour  
- Integrate format writers: 30 minutes
- Write/update tests: 1 hour
- Manual testing and validation: 30 minutes

---

## Blockers

1. **ASR Worker Integration**: Need worker to actually perform transcription and call completion handler
2. **Queue Task Processing**: Need task processor to invoke worker and handle results
3. **Deduplication Logic**: Need audio hash-based deduplication (mentioned in original design but not implemented)

---

## Dependencies

**Required:**
- STORY_01 (Multiple Output Formats) - ✅ Complete
- Queue task processing infrastructure - ⚠️ Needs implementation
- Worker gRPC integration - ⚠️ Needs implementation

---

## Next Steps

1. Implement queue task processor that calls workers
2. Implement ASR result channel infrastructure
3. Update handleASR to block and wait for results
4. Integrate format writers based on output parameter
5. Add Content-Type headers
6. Write comprehensive tests
7. Manual testing with real audio files

---

## References

- Story File: docs/BACKLOG/EPIC_08/stories/STORY_05_asr_format_selection.md
- Format Writers: orchestrator/pkg/formats/
- Current ASR Handler: orchestrator/internal/webhooks/server.go:490-605
- ASR Tests: orchestrator/internal/webhooks/asr_test.go

---

## Notes

The infrastructure for format selection is already in place - the output parameter is captured and stored. The main work remaining is implementing the blocking mechanism and format conversion, which depends on having a fully functional task processing pipeline. This is more of a "plumbing" issue than a feature implementation issue.

Consider implementing STORY_05 as part of a larger "Complete ASR Endpoint" story that includes:
- Blocking/result channel mechanism
- Audio hash-based deduplication  
- Format selection (STORY_05)
- Worker integration
- Timeout handling
