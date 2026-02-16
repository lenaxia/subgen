# Story 04: Standalone Language Detection

**Epic**: EPIC_08  
**Status**: Not Started  
**Effort**: 3-4 hours  
**Priority**: LOW  
**Assignee**: Delegation Agent

---

## User Story

As a Subgen user or external tool integrator,
I want a standalone language detection endpoint,
So that I can quickly identify audio language without performing full transcription.

---

## Background

The original subgen.py (lines 896-939) had a `/detect-language` endpoint that accepted uploaded audio files and returned language detection results. This is useful for:
- Quick language identification without full transcription
- Testing Whisper model on audio samples
- Integration with external tools that need language detection
- Pre-flight checks before transcription

---

## Acceptance Criteria

- [ ] `POST /detect-language` endpoint accepts uploaded audio file
- [ ] Query parameters: `?offset=0&length=30` for sample selection
- [ ] Bypasses queue (immediate processing, not queued)
- [ ] Returns JSON: `{"language": "English", "code": "en", "confidence": 0.99}`
- [ ] Uses existing DetectLanguage gRPC call to worker
- [ ] Timeout: 30 seconds (configurable)
- [ ] Support multipart form data upload
- [ ] Comprehensive error handling (invalid audio, timeout, etc.)
- [ ] Unit tests for handler
- [ ] Integration tests with mock worker
- [ ] Type checking passes
- [ ] Work log created

---

## Technical Design

### API Endpoint

**Route:** `POST /detect-language`

**Query Parameters:**
- `offset` (optional, default: 0) - Offset in seconds to start detection
- `length` (optional, default: 30) - Length in seconds of audio to analyze

**Request:**
```http
POST /detect-language?offset=0&length=30 HTTP/1.1
Content-Type: multipart/form-data; boundary=----WebKitFormBoundary

------WebKitFormBoundary
Content-Disposition: form-data; name="file"; filename="audio.mp3"
Content-Type: audio/mpeg

[binary audio data]
------WebKitFormBoundary--
```

**Response (Success):**
```json
{
  "language": "English",
  "code": "en",
  "confidence": 0.99
}
```

**Response (Error):**
```json
{
  "status": "error",
  "error": "invalid audio file: unsupported format"
}
```

### Approach

1. **Endpoint Handler** - Create handler in webhooks package
2. **Audio Upload** - Accept multipart form data
3. **Temporary Storage** - Save uploaded audio to temp file
4. **gRPC Call** - Call worker's DetectLanguage RPC
5. **Response** - Return JSON with language info
6. **Cleanup** - Delete temporary file after processing

### Files to Create/Modify

**New Files:**
- `orchestrator/internal/webhooks/detect_language.go` - Handler implementation
- `orchestrator/internal/webhooks/detect_language_test.go` - Unit tests

**Modified Files:**
- `orchestrator/internal/webhooks/server.go` - Add POST /detect-language route

### Implementation

```go
// orchestrator/internal/webhooks/detect_language.go
package webhooks

import (
	"context"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strconv"
	"time"

	"github.com/gofiber/fiber/v2"
	"github.com/sirupsen/logrus"
)

const (
	detectLanguageTimeout = 30 * time.Second
	maxUploadSize         = 500 * 1024 * 1024 // 500MB
)

// DetectLanguageRequest represents the query parameters
type DetectLanguageRequest struct {
	Offset float64 // Start offset in seconds
	Length float64 // Length in seconds to analyze
}

// DetectLanguageResponse represents the JSON response
type DetectLanguageResponse struct {
	Language   string  `json:"language"`
	Code       string  `json:"code"`
	Confidence float64 `json:"confidence"`
}

// ErrorResponse represents an error response
type ErrorResponse struct {
	Status string `json:"status"`
	Error  string `json:"error"`
}

// handleDetectLanguage handles POST /detect-language
func (s *Server) handleDetectLanguage(c *fiber.Ctx) error {
	// Parse query parameters
	offsetStr := c.Query("offset", "0")
	lengthStr := c.Query("length", "30")
	
	offset, err := strconv.ParseFloat(offsetStr, 64)
	if err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(ErrorResponse{
			Status: "error",
			Error:  fmt.Sprintf("invalid offset parameter: %v", err),
		})
	}
	
	length, err := strconv.ParseFloat(lengthStr, 64)
	if err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(ErrorResponse{
			Status: "error",
			Error:  fmt.Sprintf("invalid length parameter: %v", err),
		})
	}
	
	// Validate parameters
	if offset < 0 {
		return c.Status(fiber.StatusBadRequest).JSON(ErrorResponse{
			Status: "error",
			Error:  "offset must be >= 0",
		})
	}
	if length <= 0 || length > 300 {
		return c.Status(fiber.StatusBadRequest).JSON(ErrorResponse{
			Status: "error",
			Error:  "length must be between 0 and 300 seconds",
		})
	}
	
	// Get uploaded file
	file, err := c.FormFile("file")
	if err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(ErrorResponse{
			Status: "error",
			Error:  "no file uploaded or invalid multipart form",
		})
	}
	
	// Check file size
	if file.Size > maxUploadSize {
		return c.Status(fiber.StatusBadRequest).JSON(ErrorResponse{
			Status: "error",
			Error:  fmt.Sprintf("file too large: %d bytes (max: %d)", file.Size, maxUploadSize),
		})
	}
	
	// Open uploaded file
	src, err := file.Open()
	if err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(ErrorResponse{
			Status: "error",
			Error:  fmt.Sprintf("failed to open uploaded file: %v", err),
		})
	}
	defer src.Close()
	
	// Create temporary file
	tmpDir := os.TempDir()
	tmpFile, err := os.CreateTemp(tmpDir, "detect-*.tmp")
	if err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(ErrorResponse{
			Status: "error",
			Error:  fmt.Sprintf("failed to create temp file: %v", err),
		})
	}
	tmpPath := tmpFile.Name()
	defer os.Remove(tmpPath) // Clean up
	
	// Copy uploaded file to temp file
	if _, err := io.Copy(tmpFile, src); err != nil {
		tmpFile.Close()
		return c.Status(fiber.StatusInternalServerError).JSON(ErrorResponse{
			Status: "error",
			Error:  fmt.Sprintf("failed to save uploaded file: %v", err),
		})
	}
	tmpFile.Close()
	
	// Call worker's DetectLanguage RPC
	ctx, cancel := context.WithTimeout(c.Context(), detectLanguageTimeout)
	defer cancel()
	
	s.logger.WithFields(logrus.Fields{
		"file_size": file.Size,
		"offset":    offset,
		"length":    length,
	}).Info("Detecting language")
	
	resp, err := s.workerClient.DetectLanguage(ctx, tmpPath, offset, length)
	if err != nil {
		s.logger.WithError(err).Error("Language detection failed")
		return c.Status(fiber.StatusInternalServerError).JSON(ErrorResponse{
			Status: "error",
			Error:  fmt.Sprintf("language detection failed: %v", err),
		})
	}
	
	s.logger.WithFields(logrus.Fields{
		"language":   resp.LanguageName,
		"code":       resp.LanguageCode,
		"confidence": resp.Confidence,
	}).Info("Language detected")
	
	// Return response
	return c.JSON(DetectLanguageResponse{
		Language:   resp.LanguageName,
		Code:       resp.LanguageCode,
		Confidence: resp.Confidence,
	})
}
```

### Integration Points

- **Worker gRPC** - Calls DetectLanguage RPC (already implemented)
- **Temp File Handling** - Uses os.TempDir() for uploaded files
- **Fiber Router** - Adds POST /detect-language route
- **Logging** - Structured logging with file size, duration

---

## Testing Strategy

### Unit Tests

**detect_language_test.go:**
```go
func TestHandleDetectLanguage_Success(t *testing.T) {
	// Happy path: valid audio file, returns language
}

func TestHandleDetectLanguage_NoFile(t *testing.T) {
	// Error: no file uploaded
}

func TestHandleDetectLanguage_InvalidOffset(t *testing.T) {
	// Error: offset is negative
}

func TestHandleDetectLanguage_InvalidLength(t *testing.T) {
	// Error: length is invalid (negative, too large)
}

func TestHandleDetectLanguage_FileTooLarge(t *testing.T) {
	// Error: file exceeds maxUploadSize
}

func TestHandleDetectLanguage_WorkerTimeout(t *testing.T) {
	// Error: worker takes too long (>30s)
}

func TestHandleDetectLanguage_WorkerError(t *testing.T) {
	// Error: worker returns error (invalid audio format)
}

func TestHandleDetectLanguage_TempFileCleanup(t *testing.T) {
	// Verify temp file is deleted after processing
}
```

### Integration Tests

```go
func TestDetectLanguage_Integration(t *testing.T) {
	// Create test audio file
	// Start mock worker
	// Upload file to endpoint
	// Verify correct language response
}
```

### Manual Testing

```bash
# Test 1: Detect English audio
curl -X POST "http://localhost:9000/detect-language?offset=0&length=30" \
  -F "file=@test_audio_english.mp3"
# Expected: {"language": "English", "code": "en", "confidence": 0.99}

# Test 2: Custom offset and length
curl -X POST "http://localhost:9000/detect-language?offset=10&length=15" \
  -F "file=@test_audio.mp3"
# Expected: Detects language from 10-25 second range

# Test 3: No file uploaded
curl -X POST "http://localhost:9000/detect-language"
# Expected: 400 error "no file uploaded"

# Test 4: Invalid parameters
curl -X POST "http://localhost:9000/detect-language?offset=-5&length=30" \
  -F "file=@test_audio.mp3"
# Expected: 400 error "offset must be >= 0"

# Test 5: File too large
# Create 600MB file: dd if=/dev/zero of=large.bin bs=1M count=600
curl -X POST "http://localhost:9000/detect-language" \
  -F "file=@large.bin"
# Expected: 400 error "file too large"
```

---

## Definition of Done

- [ ] Story file created (this document)
- [ ] Tests written FIRST (TDD)
- [ ] Handler implemented in detect_language.go
- [ ] Route added to server.go
- [ ] Query parameter validation
- [ ] File upload handling
- [ ] Temp file cleanup (defer or panic recovery)
- [ ] gRPC integration with worker
- [ ] All unit tests passing
- [ ] Integration tests passing
- [ ] Manual testing completed
- [ ] Type checking passes
- [ ] Error messages are clear
- [ ] Logging includes request details
- [ ] Work log created (0023_2026-02-16_epic08_story04_detect_language.md)
- [ ] Code committed and pushed

---

## Security Considerations

1. **File Size Limits** - Max 500MB to prevent DoS
2. **Timeout** - 30 second limit to prevent hanging
3. **Temp File Cleanup** - Always delete temp files (use defer)
4. **Input Validation** - Validate offset/length parameters
5. **Content Type** - Validate audio file format (future enhancement)

---

## Performance Considerations

- Immediate processing (bypasses queue) for low latency
- Temp file I/O may be bottleneck for large files
- Consider streaming upload instead of temp file (future optimization)
- Worker model loading time may impact first request

---

## Success Criteria

1. **Accuracy**: Language detection matches full transcription
2. **Speed**: Response within 5 seconds for 30-second audio
3. **Reliability**: No memory leaks from temp files
4. **Error Handling**: Clear errors for all failure modes
5. **Usability**: Simple API for external integrations

---

## References

- **Original Implementation**: subgen.py lines 896-939
- **Worker gRPC**: proto/worker.proto DetectLanguage RPC
- **Fiber Docs**: https://docs.gofiber.io/api/app#formfile
- **Go temp files**: os.CreateTemp, os.TempDir

---

**Story Created**: 2026-02-16  
**Last Updated**: 2026-02-16
