# Story 05: ASR Format Selection

**Epic**: EPIC_08  
**Status**: Not Started  
**Effort**: 3-4 hours  
**Priority**: MEDIUM  
**Assignee**: Delegation Agent

---

## User Story

As a Bazarr user or ASR API consumer,
I want to select the subtitle output format (SRT, VTT, LRC),
So that I can receive subtitles in the format my player or application requires.

---

## Background

The current ASR endpoint (`/asr`) returns subtitles in SRT format only. Some applications (Bazarr, web players) may prefer VTT (WebVTT) format for better web compatibility or LRC for audio-only content.

This story extends the ASR endpoint to support format selection via query parameter while maintaining backward compatibility (default: SRT).

---

## Acceptance Criteria

- [ ] Query parameter: `?output=srt` (default), `?output=vtt`, `?output=lrc`
- [ ] Return subtitle in requested format
- [ ] Still block until transcription completes
- [ ] Content-Type headers match format:
  - SRT: `text/plain; charset=utf-8`
  - VTT: `text/vtt; charset=utf-8`
  - LRC: `text/plain; charset=utf-8`
- [ ] Works with existing ASR deduplication (by audio hash)
- [ ] Uses format writers from STORY_01
- [ ] Comprehensive error handling (invalid format)
- [ ] Unit tests for format parameter
- [ ] Integration tests with all three formats
- [ ] Type checking passes
- [ ] Work log created

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

1. **Parse Format Parameter** - Extract and validate `output` query param
2. **Call Format Writer** - Use appropriate writer from STORY_01
3. **Set Content-Type** - Return correct MIME type for format
4. **Maintain Deduplication** - Format doesn't affect task hash

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

```go
// orchestrator/internal/webhooks/asr.go

// Add format validation
var validASRFormats = map[string]bool{
	"srt": true,
	"vtt": true,
	"lrc": true,
}

// Modify handleASR function
func (s *Server) handleASR(c *fiber.Ctx) error {
	// ... existing code for task and language ...
	
	// Parse output format parameter
	outputFormat := c.Query("output", "srt")
	outputFormat = strings.ToLower(strings.TrimSpace(outputFormat))
	
	// Validate format
	if !validASRFormats[outputFormat] {
		return c.Status(fiber.StatusBadRequest).JSON(ErrorResponse{
			Status: "error",
			Error:  fmt.Sprintf("invalid output format: %s (must be srt, vtt, or lrc)", outputFormat),
		})
	}
	
	// ... existing code for audio upload, hashing, queueing ...
	
	// Wait for transcription result
	result := <-resultChan
	if result.Error != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(ErrorResponse{
			Status: "error",
			Error:  result.Error.Error(),
		})
	}
	
	// Convert segments to requested format
	var output bytes.Buffer
	var contentType string
	
	switch outputFormat {
	case "vtt":
		writer := formats.NewVTTWriter()
		if err := writer.Write(&output, result.Segments, result.Metadata); err != nil {
			return c.Status(fiber.StatusInternalServerError).JSON(ErrorResponse{
				Status: "error",
				Error:  fmt.Sprintf("failed to generate VTT: %v", err),
			})
		}
		contentType = "text/vtt; charset=utf-8"
		
	case "lrc":
		writer := formats.NewLRCWriter()
		if err := writer.Write(&output, result.Segments, result.Metadata); err != nil {
			return c.Status(fiber.StatusInternalServerError).JSON(ErrorResponse{
				Status: "error",
				Error:  fmt.Sprintf("failed to generate LRC: %v", err),
			})
		}
		contentType = "text/plain; charset=utf-8"
		
	case "srt":
		fallthrough
	default:
		writer := formats.NewSRTWriter()
		if err := writer.Write(&output, result.Segments, result.Metadata); err != nil {
			return c.Status(fiber.StatusInternalServerError).JSON(ErrorResponse{
				Status: "error",
				Error:  fmt.Sprintf("failed to generate SRT: %v", err),
			})
		}
		contentType = "text/plain; charset=utf-8"
	}
	
	// Set Content-Type and return
	c.Set("Content-Type", contentType)
	return c.SendString(output.String())
}
```

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

**Requires:**
- STORY_01 (Multiple Output Formats) - ✅ Must be complete (format writers needed)

**Integration:**
- ASR endpoint (existing)
- Format writers package

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
