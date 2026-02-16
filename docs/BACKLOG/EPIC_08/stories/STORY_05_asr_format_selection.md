# Story 05: ASR Format Selection

**Epic**: EPIC_08  
**Status**: BLOCKED  
**Blocker**: STORY_10 (Blocking ASR Infrastructure) must complete first  
**Effort**: 1-2 hours (after STORY_10 complete)  
**Priority**: MEDIUM  
**Assignee**: TBD

---

## ⚠️ BLOCKED - Architectural Dependency

**Status**: This story is BLOCKED pending implementation of STORY_10 (Blocking ASR Infrastructure).

**Current State**: ASR endpoint validates format parameter but returns placeholder text instead of formatted subtitles because there's no mechanism to block and wait for transcription results.

**Blocker**: [STORY_10: Blocking ASR Infrastructure](./STORY_10_blocking_asr_infrastructure.md)

**See**: [Work Log with Gap Analysis](../../WORKLOGS/0032_2026-02-16_epic08_story05_architectural_gap_analysis.md)

---

## User Story

As a Bazarr user or ASR API consumer,
I want to select the subtitle output format (SRT, VTT, LRC, TXT, TSV, JSON),
So that I can receive subtitles in the format my player or application requires.

---

## Background

The current ASR endpoint (`/asr`) queues transcription tasks but returns a placeholder message instead of actual subtitles. This story adds format selection and subtitle return functionality once the blocking ASR infrastructure (STORY_10) is complete.

**Architectural Context**: The Go orchestrator uses an async queue+worker architecture. To return formatted subtitles, the endpoint must block and wait for worker completion, which requires result channels (implemented in STORY_10).

---

## Acceptance Criteria

**Prerequisites** (must complete first):
- [ ] STORY_10 (Blocking ASR Infrastructure) is complete
- [ ] ASR endpoint can block and retrieve transcription results

**This Story**:
- [ ] Query parameter: `?output=srt` (default), `?output=vtt`, `?output=lrc`, `?output=txt`, `?output=tsv`, `?output=json`
- [ ] Format validation returns 400 for invalid formats
- [ ] Use format writers to convert segments to requested format
- [ ] Return formatted subtitles instead of placeholder text
- [ ] Content-Type headers set correctly:
  - SRT: `text/plain; charset=utf-8`
  - VTT: `text/vtt; charset=utf-8`
  - LRC: `text/plain; charset=utf-8`
  - JSON: `application/json; charset=utf-8`
  - TXT: `text/plain; charset=utf-8`
  - TSV: `text/tab-separated-values; charset=utf-8`
- [ ] Works with blocking mechanism from STORY_10
- [ ] Comprehensive error handling (invalid format, format conversion errors)
- [ ] Unit tests updated to verify actual output format
- [ ] Integration tests with all six formats
- [ ] Type checking passes
- [ ] Work log updated

---

## Technical Design

### API Enhancement

**Route:** `POST /asr` (existing endpoint)

**Query Parameters:**
- `task` (existing) - "transcribe" or "translate"
- `language` (existing) - Target language code
- `output` (new, optional, default: "srt") - Output format: "srt", "vtt", "lrc"

**Request:**
```http
POST /asr?task=transcribe&language=en&output=vtt HTTP/1.1
Content-Type: multipart/form-data

[audio file]
```

**Response (VTT):**
```http
HTTP/1.1 200 OK
Content-Type: text/vtt; charset=utf-8

WEBVTT

00:00:00.000 --> 00:00:03.200
Transcribed text here.

00:00:03.400 --> 00:00:06.800
More dialogue.
```

**Response (SRT - default):**
```http
HTTP/1.1 200 OK
Content-Type: text/plain; charset=utf-8

1
00:00:00,000 --> 00:00:03,200
Transcribed text here.

2
00:00:03,400 --> 00:00:06,800
More dialogue.
```

**Response (LRC):**
```http
HTTP/1.1 200 OK
Content-Type: text/plain; charset=utf-8

[00:00.00]Transcribed text here.
[00:03.40]More dialogue.
```

**Response (Error - invalid format):**
```http
HTTP/1.1 400 Bad Request
Content-Type: application/json

{
  "status": "error",
  "error": "invalid output format: txt (must be srt, vtt, or lrc)"
}
```

### Approach

**Phase 1: Prerequisites** (STORY_10)
1. ✅ Add ResultChan to Task struct
2. ✅ Implement blocking mechanism in handleASR
3. ✅ Worker sends results to ResultChan

**Phase 2: Format Selection** (This Story)
1. **Retrieve Result** - Get segments from ResultChan (implemented in STORY_10)
2. **Validate Format** - Check format parameter is valid
3. **Convert Format** - Use format writer to convert segments
4. **Set Content-Type** - Return correct MIME type
5. **Return Subtitles** - Send formatted content to client

### Files to Modify

**orchestrator/internal/webhooks/asr.go:**
- Add format parameter parsing
- Validate format is one of: srt, vtt, lrc
- Call appropriate format writer based on output parameter
- Set Content-Type header based on format

**orchestrator/internal/webhooks/asr_test.go:**
- Add tests for format parameter
- Test all three formats return correct output
- Test invalid format returns 400 error
- Test default format (when not specified)

### Implementation

**File**: `orchestrator/internal/webhooks/server.go`

```go
func (s *Server) handleASR(c *fiber.Ctx) error {
	// ... existing validation code (STORY_10 implements blocking) ...
	
	// STORY_10 will implement this section:
	// - Create ResultChan
	// - Queue task with ResultChan
	// - Block with select/timeout
	// - Receive result from channel
	
	// Block until result ready (implemented in STORY_10)
	select {
	case result := <-resultChan:
		if result.Error != nil {
			return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
				"error": fmt.Sprintf("transcription failed: %v", result.Error),
			})
		}
		
		// --- THIS STORY STARTS HERE ---
		// Parse output format parameter (validation already done, stored in task.ASROptions)
		outputFormat := c.Query("output", "srt")
		
		// Use format writer to convert segments (THIS IS NEW)
		var buffer bytes.Buffer
		writer, err := formats.NewWriter(outputFormat)
		if err != nil {
			return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
				"error": fmt.Sprintf("unsupported format: %s", outputFormat),
			})
		}
		
		if err := writer.Write(&buffer, result.Segments, result.Metadata); err != nil {
			return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
				"error": fmt.Sprintf("format conversion failed: %v", err),
			})
		}
		
		// Set Content-Type header based on format (THIS IS NEW)
		c.Set("Content-Type", getContentType(outputFormat))
		
		// Return formatted subtitles (THIS IS NEW - replaces placeholder)
		return c.SendString(buffer.String())
		
	case <-time.After(timeout):
		// ... timeout handling (implemented in STORY_10) ...
	}
}

// Helper already exists, just needs to be called (line 792-802)
func getContentType(format string) string {
	switch format {
	case "vtt":
		return "text/vtt; charset=utf-8"
	case "json":
		return "application/json; charset=utf-8"
	default:
		return "text/plain; charset=utf-8"
	}
}
```

**Changes from Current Code**:
1. Replace placeholder return with format conversion (lines 4-6 added)
2. Call `formats.NewWriter()` to get format writer (line 5 new)
3. Convert segments using writer (line 6 new)
4. Set Content-Type header (line 7 new)
5. Return formatted content (line 8 new)

### Content-Type Headers

| Format | Content-Type Header | MIME Type |
|--------|-------------------|-----------|
| SRT | `text/plain; charset=utf-8` | Standard text |
| VTT | `text/vtt; charset=utf-8` | WebVTT spec |
| LRC | `text/plain; charset=utf-8` | Standard text |

### Integration Points

- **Format Writers** - Uses writers from STORY_01 (orchestrator/pkg/formats)
- **ASR Deduplication** - Format parameter doesn't affect task hash
- **Worker gRPC** - Worker returns segments in neutral format
- **Bazarr Integration** - Bazarr can request VTT instead of SRT

---

## Testing Strategy

### Unit Tests

**asr_test.go additions:**
```go
func TestHandleASR_OutputFormat_SRT(t *testing.T) {
	// Test ?output=srt returns SRT format
}

func TestHandleASR_OutputFormat_VTT(t *testing.T) {
	// Test ?output=vtt returns VTT format
}

func TestHandleASR_OutputFormat_LRC(t *testing.T) {
	// Test ?output=lrc returns LRC format
}

func TestHandleASR_OutputFormat_Default(t *testing.T) {
	// Test no format parameter defaults to SRT
}

func TestHandleASR_OutputFormat_Invalid(t *testing.T) {
	// Test ?output=invalid returns 400 error
}

func TestHandleASR_OutputFormat_CaseInsensitive(t *testing.T) {
	// Test ?output=VTT (uppercase) works
}

func TestHandleASR_ContentType_SRT(t *testing.T) {
	// Verify Content-Type: text/plain for SRT
}

func TestHandleASR_ContentType_VTT(t *testing.T) {
	// Verify Content-Type: text/vtt for VTT
}

func TestHandleASR_ContentType_LRC(t *testing.T) {
	// Verify Content-Type: text/plain for LRC
}
```

### Integration Tests

```go
func TestASR_Integration_AllFormats(t *testing.T) {
	// Upload same audio file three times
	// Request SRT, VTT, LRC formats
	// Verify all return valid output
	// Verify content matches (same segments, different format)
}
```

### Manual Testing

```bash
# Test 1: Default format (SRT)
curl -X POST "http://localhost:9000/asr?task=transcribe&language=en" \
  -F "file=@test_audio.mp3" \
  -o output.srt
# Verify SRT format

# Test 2: VTT format
curl -X POST "http://localhost:9000/asr?task=transcribe&language=en&output=vtt" \
  -F "file=@test_audio.mp3" \
  -o output.vtt
# Verify VTT format with "WEBVTT" header

# Test 3: LRC format
curl -X POST "http://localhost:9000/asr?task=transcribe&language=en&output=lrc" \
  -F "file=@test_audio.mp3" \
  -o output.lrc
# Verify LRC format with [mm:ss.xx] timestamps

# Test 4: Invalid format
curl -X POST "http://localhost:9000/asr?task=transcribe&language=en&output=json" \
  -F "file=@test_audio.mp3"
# Expected: 400 error "invalid output format: json"

# Test 5: Case insensitivity
curl -X POST "http://localhost:9000/asr?task=transcribe&language=en&output=VTT" \
  -F "file=@test_audio.mp3"
# Expected: Returns VTT format (case doesn't matter)

# Test 6: Content-Type header
curl -X POST "http://localhost:9000/asr?task=transcribe&language=en&output=vtt" \
  -F "file=@test_audio.mp3" -I
# Expected: Content-Type: text/vtt; charset=utf-8
```

---

## Definition of Done

- [ ] Story file created (this document)
- [ ] Tests written FIRST (TDD)
- [ ] Format parameter parsing implemented
- [ ] Format validation implemented
- [ ] All three formats supported (SRT, VTT, LRC)
- [ ] Content-Type headers correct
- [ ] Default format is SRT (backward compatible)
- [ ] All unit tests passing
- [ ] Integration tests passing
- [ ] Manual testing completed
- [ ] Type checking passes
- [ ] Error messages are clear
- [ ] Work log created (0024_2026-02-16_epic08_story05_asr_format_selection.md)
- [ ] Code committed and pushed

---

## Backward Compatibility

- **Default behavior unchanged**: If no `output` parameter specified, returns SRT (existing behavior)
- **Existing clients unaffected**: Bazarr and other clients continue to work without changes
- **Optional enhancement**: Clients can opt-in to VTT/LRC formats when ready

---

## Dependencies

**BLOCKED BY:**
- ⚠️ STORY_10 (Blocking ASR Infrastructure) - MUST complete first
  - Adds ResultChan to Task struct
  - Implements blocking mechanism in handleASR
  - Worker result routing to ResultChan

**Requires:**
- ✅ STORY_01 (Multiple Output Formats) - Complete (format writers ready)

**Integration:**
- ASR endpoint (STORY_10 adds blocking)
- Format writers package (STORY_01)

---

## Success Criteria

1. **Compatibility**: Existing ASR clients continue to work
2. **Flexibility**: All three formats produce valid output
3. **Performance**: Format conversion adds <100ms overhead
4. **Validation**: Clear errors for invalid format requests
5. **Standards**: VTT output validates against WebVTT spec

---

## Future Enhancements

- Support additional formats (JSON, TSV, TXT)
- Allow multiple formats in single request (Accept header)
- Format-specific options (e.g., VTT styling)

---

## References

- **Original ASR endpoint**: orchestrator/internal/webhooks/asr.go
- **Format writers**: STORY_01 (orchestrator/pkg/formats/)
- **WebVTT spec**: https://www.w3.org/TR/webvtt1/
- **LRC spec**: https://en.wikipedia.org/wiki/LRC_(file_format)

---

**Story Created**: 2026-02-16  
**Last Updated**: 2026-02-16
