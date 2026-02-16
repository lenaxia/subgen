# Story 09: Enhanced Logging & Error Messages

**Epic**: EPIC_08  
**Status**: Complete  
**Effort**: 2-4 hours  
**Priority**: LOW  
**Assignee**: OpenCode Agent  
**Completed**: 2026-02-16

---

## User Story

As a Subgen administrator troubleshooting issues,
I want clear, structured logs with actionable error messages,
So that I can quickly diagnose problems and understand system behavior.

---

## Background

Good logging is critical for:
- Troubleshooting production issues
- Understanding system behavior
- Performance monitoring
- Audit trails

This story enhances logging throughout the orchestrator with:
- Structured logging with consistent fields
- Clear error messages with solutions
- Log level filtering
- Request IDs for tracing
- Performance metrics
- Startup banner with configuration summary

---

## Acceptance Criteria

- [x] Structured logging with consistent fields (file_path, task_id, duration, etc.)
- [x] Clear error messages with actionable solutions
- [x] Log level filtering (DEBUG, INFO, WARN, ERROR)
- [x] Request IDs for HTTP request tracing
- [x] Performance metrics in logs (processing time, queue depth)
- [x] Startup banner with version and configuration summary
- [x] Log correlation across components (orchestrator → worker)
- [x] No sensitive data in logs (tokens, passwords)
- [x] JSON log format option (for log aggregation)
- [x] Unit tests for logging utilities
- [x] Manual verification of log output
- [x] Work log created

---

## Technical Design

### Logging Architecture

```
┌─────────────────────────────────────────────────────────────┐
│                     Logrus (Structured Logger)               │
│  ├─ Levels: DEBUG, INFO, WARN, ERROR                        │
│  ├─ Fields: file_path, task_id, duration, worker_id         │
│  ├─ Formatters: Text (development), JSON (production)       │
│  └─ Hooks: Request ID injection, performance metrics        │
└─────────────────────────────────────────────────────────────┘
```

### Log Levels

| Level | Purpose | Examples |
|-------|---------|----------|
| DEBUG | Detailed debugging info | "Loaded configuration from env", "Queue state: 15 queued" |
| INFO  | Normal operation events | "Transcription started", "File queued" |
| WARN  | Warning conditions | "Mapped path doesn't exist", "Queue depth high" |
| ERROR | Error conditions | "Transcription failed", "Worker timeout" |

### Structured Fields

**Standard Fields (all logs):**
- `timestamp` - ISO 8601 timestamp
- `level` - Log level (DEBUG/INFO/WARN/ERROR)
- `component` - "orchestrator", "worker", "webhook", "queue"
- `version` - Subgen version

**Event-specific Fields:**
- `file_path` - Media file path
- `task_id` - Unique task identifier
- `duration` - Operation duration (milliseconds)
- `worker_id` - Worker number (1, 2, etc.)
- `request_id` - HTTP request ID (for tracing)
- `queue_depth` - Current queue size
- `error` - Error message (for ERROR level)

### Startup Banner

```
╔════════════════════════════════════════════════════════════════╗
║              Subgen Orchestrator v2026.02.16                   ║
╚════════════════════════════════════════════════════════════════╝

Configuration:
  Whisper Model:      medium
  Device:             cuda
  Compute Type:       float16
  Concurrent Workers: 2
  
  Path Mapping:       enabled (/data → /mnt/media)
  Skip Logic:         enabled (7 conditions)
  File Monitoring:    enabled (3 folders)
  
  Plex Integration:   enabled (queue next episode)
  Jellyfin:           enabled
  Emby:               enabled

Webhooks:
  Listening on:       http://0.0.0.0:9000
  Endpoints:          /plex, /jellyfin, /emby, /asr, /batch
  
Metrics:
  Prometheus:         http://0.0.0.0:9091/metrics
  
Ready to process transcriptions!
```

### Enhanced Error Messages

**Before:**
```
ERROR: transcription failed
```

**After:**
```
ERROR [2026-02-16 12:34:56] task_id=abc123 file=/movies/broken.mkv event=task_failed error="audio extraction failed: no audio tracks found"
  Troubleshooting:
    • Verify file has audio tracks: ffprobe /movies/broken.mkv
    • Check file is not corrupted
    • Try re-encoding with: ffmpeg -i broken.mkv -c copy fixed.mkv
    • Check Subgen has read access to file
```

### Implementation

```go
// orchestrator/internal/util/logger.go
package util

import (
	"fmt"
	"os"
	"time"
	
	"github.com/sirupsen/logrus"
)

// SetupLogger configures the global logger
func SetupLogger(level string, format string) *logrus.Logger {
	logger := logrus.New()
	
	// Set log level
	switch level {
	case "DEBUG":
		logger.SetLevel(logrus.DebugLevel)
	case "INFO":
		logger.SetLevel(logrus.InfoLevel)
	case "WARN":
		logger.SetLevel(logrus.WarnLevel)
	case "ERROR":
		logger.SetLevel(logrus.ErrorLevel)
	default:
		logger.SetLevel(logrus.InfoLevel)
	}
	
	// Set formatter
	if format == "json" {
		logger.SetFormatter(&logrus.JSONFormatter{
			TimestampFormat: time.RFC3339,
		})
	} else {
		logger.SetFormatter(&logrus.TextFormatter{
			FullTimestamp:   true,
			TimestampFormat: "2006-01-02 15:04:05",
		})
	}
	
	logger.SetOutput(os.Stdout)
	
	return logger
}

// LogTaskStart logs when a task starts processing
func LogTaskStart(logger *logrus.Logger, taskID, filePath string, workerID int) {
	logger.WithFields(logrus.Fields{
		"event":     "task_started",
		"task_id":   taskID,
		"file_path": filePath,
		"worker_id": workerID,
	}).Info("Transcription started")
}

// LogTaskComplete logs when a task completes successfully
func LogTaskComplete(logger *logrus.Logger, taskID, filePath, outputFile string, duration time.Duration) {
	logger.WithFields(logrus.Fields{
		"event":        "task_completed",
		"task_id":      taskID,
		"file_path":    filePath,
		"output_file":  outputFile,
		"duration_ms":  duration.Milliseconds(),
	}).Info("Transcription completed")
}

// LogTaskFailed logs when a task fails
func LogTaskFailed(logger *logrus.Logger, taskID, filePath string, err error, troubleshooting []string) {
	entry := logger.WithFields(logrus.Fields{
		"event":     "task_failed",
		"task_id":   taskID,
		"file_path": filePath,
		"error":     err.Error(),
	})
	
	msg := fmt.Sprintf("Transcription failed: %v", err)
	if len(troubleshooting) > 0 {
		msg += "\n  Troubleshooting:"
		for _, tip := range troubleshooting {
			msg += fmt.Sprintf("\n    • %s", tip)
		}
	}
	
	entry.Error(msg)
}

// LogQueueStatus logs current queue statistics
func LogQueueStatus(logger *logrus.Logger, queued, processing, completed int) {
	logger.WithFields(logrus.Fields{
		"event":      "queue_status",
		"queued":     queued,
		"processing": processing,
		"completed":  completed,
	}).Debug("Queue status")
}
```

```go
// orchestrator/internal/util/banner.go
package util

import (
	"fmt"
	"strings"
)

// PrintStartupBanner prints the startup banner
func PrintStartupBanner(config *config.Config, version string) {
	width := 64
	border := strings.Repeat("═", width-2)
	
	fmt.Printf("╔%s╗\n", border)
	fmt.Printf("║%s║\n", center("Subgen Orchestrator "+version, width-2))
	fmt.Printf("╚%s╝\n\n", border)
	
	fmt.Println("Configuration:")
	fmt.Printf("  Whisper Model:      %s\n", config.Whisper.Model)
	fmt.Printf("  Device:             %s\n", config.Whisper.Device)
	fmt.Printf("  Compute Type:       %s\n", config.Whisper.ComputeType)
	fmt.Printf("  Concurrent Workers: %d\n", config.Workers)
	fmt.Println()
	
	if config.PathMapping.Enabled {
		fmt.Printf("  Path Mapping:       enabled (%s → %s)\n", 
			config.PathMapping.From, config.PathMapping.To)
	} else {
		fmt.Println("  Path Mapping:       disabled")
	}
	
	fmt.Println()
	fmt.Println("Webhooks:")
	fmt.Printf("  Listening on:       http://0.0.0.0:%d\n", config.Port)
	fmt.Println("  Endpoints:          /plex, /jellyfin, /emby, /asr, /batch")
	fmt.Println()
	
	if config.Metrics.Enabled {
		fmt.Printf("Metrics:\n")
		fmt.Printf("  Prometheus:         http://0.0.0.0:%d/metrics\n", config.Metrics.Port)
		fmt.Println()
	}
	
	fmt.Println("Ready to process transcriptions!")
	fmt.Println()
}

func center(text string, width int) string {
	padding := (width - len(text)) / 2
	return strings.Repeat(" ", padding) + text + strings.Repeat(" ", width-padding-len(text))
}
```

### Error Message Templates

```go
// orchestrator/internal/util/errors.go
package util

import "fmt"

// Common error messages with troubleshooting tips

func ErrAudioExtractionFailed(filePath string) (string, []string) {
	msg := fmt.Sprintf("audio extraction failed for %s", filePath)
	tips := []string{
		fmt.Sprintf("Verify file has audio tracks: ffprobe %s", filePath),
		"Check file is not corrupted",
		fmt.Sprintf("Try re-encoding: ffmpeg -i %s -c copy fixed.mkv", filePath),
	}
	return msg, tips
}

func ErrFileNotFound(filePath string) (string, []string) {
	msg := fmt.Sprintf("file not found: %s", filePath)
	tips := []string{
		"Verify file path is correct",
		"Check Subgen has read access to file",
		"If using path mapping, verify PATH_MAPPING_TO is correct",
	}
	return msg, tips
}

func ErrWorkerTimeout() (string, []string) {
	msg := "worker timeout: transcription took too long"
	tips := []string{
		"Check worker is running and responsive",
		"Increase timeout in configuration",
		"Check GPU/CPU usage on worker",
	}
	return msg, tips
}

func ErrInvalidAudioFormat(format string) (string, []string) {
	msg := fmt.Sprintf("invalid audio format: %s", format)
	tips := []string{
		"Supported formats: mp3, wav, flac, m4a, ogg, opus",
		"Convert file to supported format using ffmpeg",
	}
	return msg, tips
}
```

### Request ID Middleware

```go
// orchestrator/internal/webhooks/middleware.go
package webhooks

import (
	"github.com/gofiber/fiber/v2"
	"github.com/google/uuid"
)

// RequestIDMiddleware adds a unique request ID to each request
func RequestIDMiddleware() fiber.Handler {
	return func(c *fiber.Ctx) error {
		// Generate or extract request ID
		requestID := c.Get("X-Request-ID")
		if requestID == "" {
			requestID = uuid.New().String()
		}
		
		// Store in context
		c.Locals("request_id", requestID)
		
		// Add to response header
		c.Set("X-Request-ID", requestID)
		
		return c.Next()
	}
}

// GetRequestID retrieves request ID from context
func GetRequestID(c *fiber.Ctx) string {
	if id, ok := c.Locals("request_id").(string); ok {
		return id
	}
	return ""
}
```

### Files to Create

1. **orchestrator/internal/util/logger.go**
   - Logger setup and configuration
   - Structured logging helpers

2. **orchestrator/internal/util/logger_test.go**
   - Unit tests for logging functions

3. **orchestrator/internal/util/banner.go**
   - Startup banner generation

4. **orchestrator/internal/util/errors.go**
   - Error message templates with troubleshooting

5. **orchestrator/internal/webhooks/middleware.go**
   - Request ID middleware

### Files to Modify

- **orchestrator/cmd/orchestrator/main.go**
  - Call PrintStartupBanner on startup
  - Setup logger with config

- **All webhook handlers**
  - Add request ID to log entries
  - Use structured logging

- **All queue operations**
  - Add structured logging with task_id

---

## Testing Strategy

### Unit Tests

**logger_test.go:**
```go
func TestSetupLogger_Levels(t *testing.T) {
	// Test all log levels set correctly
}

func TestLogTaskStart(t *testing.T) {
	// Verify log output includes all fields
}

func TestLogTaskComplete(t *testing.T) {
	// Verify duration calculation and output
}

func TestLogTaskFailed(t *testing.T) {
	// Verify error and troubleshooting tips in output
}
```

**banner_test.go:**
```go
func TestPrintStartupBanner(t *testing.T) {
	// Capture output and verify format
}
```

### Manual Testing

```bash
# Test 1: Startup banner
./orchestrator
# Expected: Banner with version and configuration

# Test 2: Log levels
export LOG_LEVEL=DEBUG
./orchestrator
# Expected: DEBUG logs visible

export LOG_LEVEL=ERROR
./orchestrator
# Expected: Only ERROR logs visible

# Test 3: JSON format
export LOG_FORMAT=json
./orchestrator
# Trigger transcription
# Expected: JSON log entries

# Test 4: Request ID tracing
curl -H "X-Request-ID: test-123" http://localhost:9000/status
# Expected: Logs include request_id=test-123

# Test 5: Error messages with troubleshooting
# Trigger transcription of invalid file
# Expected: ERROR log with troubleshooting tips
```

---

## Definition of Done

- [ ] Story file created (this document)
- [ ] Logger setup implemented
- [ ] Startup banner implemented
- [ ] Error templates created
- [ ] Request ID middleware implemented
- [ ] Structured logging added throughout codebase
- [ ] All unit tests passing
- [ ] Manual testing completed
- [ ] No sensitive data in logs (verified)
- [ ] Type checking passes
- [ ] Work log created (0027_2026-02-16_epic08_story09_logging_polish.md)
- [ ] Code committed and pushed

---

## Log Output Examples

### Startup
```
╔════════════════════════════════════════════════════════════════╗
║              Subgen Orchestrator v2026.02.16                   ║
╚════════════════════════════════════════════════════════════════╝

Configuration:
  Whisper Model:      medium
  Device:             cuda
  Concurrent Workers: 2

Ready to process transcriptions!
```

### Task Processing
```
INFO  [2026-02-16 12:34:56] event=task_queued task_id=abc123 file=/movies/action.mkv priority=2
INFO  [2026-02-16 12:34:57] event=task_started task_id=abc123 file=/movies/action.mkv worker_id=1
INFO  [2026-02-16 12:37:21] event=task_completed task_id=abc123 file=/movies/action.mkv output=/movies/action.eng.srt duration_ms=144523
```

### Error with Troubleshooting
```
ERROR [2026-02-16 12:40:15] event=task_failed task_id=def456 file=/movies/broken.mkv error="audio extraction failed: no audio tracks found"
  Troubleshooting:
    • Verify file has audio tracks: ffprobe /movies/broken.mkv
    • Check file is not corrupted
    • Try re-encoding: ffmpeg -i broken.mkv -c copy fixed.mkv
```

### JSON Format
```json
{"level":"info","time":"2026-02-16T12:34:56Z","event":"task_started","task_id":"abc123","file_path":"/movies/action.mkv","worker_id":1,"msg":"Transcription started"}
```

---

## Security Considerations

**Sensitive Data Filtering:**
- Never log Plex/Jellyfin tokens
- Never log authentication credentials
- Sanitize file paths if they contain usernames
- Redact any PII from logs

---

## Success Criteria

1. **Clarity**: Error messages clearly explain the problem
2. **Actionability**: Every error includes troubleshooting steps
3. **Traceability**: Request IDs allow end-to-end tracing
4. **Performance**: Logging adds <1ms overhead per operation
5. **Completeness**: All major operations logged

---

## References

- **Logrus**: https://github.com/sirupsen/logrus
- **Structured Logging Best Practices**: https://www.honeycomb.io/blog/structured-logging-best-practices
- **Go Logging**: https://dave.cheney.net/2015/11/05/lets-talk-about-logging

---

**Story Created**: 2026-02-16  
**Last Updated**: 2026-02-16
