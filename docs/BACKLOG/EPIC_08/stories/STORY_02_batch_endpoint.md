# Story 02: Batch Processing Endpoint

**Epic**: EPIC_08  
**Status**: In Progress  
**Assignee**: Delegation Agent  
**Effort**: 4-6 hours  
**Priority**: MEDIUM

---

## User Story

As a **Subgen administrator**,  
I want **a batch processing API endpoint**,  
So that **I can bulk transcribe entire directories of media files via API without manual intervention**.

---

## Acceptance Criteria

- [ ] `POST /batch?directory=/path/to/folder` endpoint implemented
- [ ] Optional query parameter: `?language=en` to force target language
- [ ] Optional query parameter: `?recursive=true` for subdirectory traversal
- [ ] Returns JSON with scan statistics: `{"scanned": 150, "queued": 23, "skipped": 127, "skip_reasons": {...}}`
- [ ] Respects skip logic from EPIC_06 (skip checker integration)
- [ ] Authentication optional (basic security consideration)
- [ ] Comprehensive error handling (directory not found, permission denied, etc.)
- [ ] All tests passing (unit + integration tests)
- [ ] Work log created documenting implementation

---

## Technical Design

### API Endpoint

**Route:** `POST /batch`

**Query Parameters:**
- `directory` (required) - Absolute path to directory to scan
- `recursive` (optional, default: false) - Recursively scan subdirectories
- `language` (optional) - Force specific language code for all files

**Request:**
```http
POST /batch?directory=/movies&recursive=true&language=en HTTP/1.1
Content-Type: application/json
```

**Response (Success):**
```json
{
  "status": "success",
  "scanned": 150,
  "queued": 23,
  "skipped": 127,
  "skip_reasons": {
    "subtitle_exists": 120,
    "audio_language_mismatch": 5,
    "unknown_language": 2
  }
}
```

**Response (Error):**
```json
{
  "status": "error",
  "error": "directory not found: /invalid/path"
}
```

### Approach

1. **Scanner Component** - Create minimal scanner stub (EPIC_07 STORY_03 not yet implemented)
   - Interface: `Scanner` with `ScanDirectory()` method
   - Stub implementation for testing
   - TODO: Replace with real scanner when EPIC_07 STORY_03 completes

2. **Batch Handler** - Implement POST /batch endpoint
   - Validate query parameters
   - Check directory exists and is readable
   - Use scanner to find media files
   - Apply skip logic to each file
   - Queue files that should be processed
   - Return statistics

3. **Integration** - Wire into webhook server
   - Add route to server.go
   - Pass queue and skip checker dependencies

### Files to Create/Modify

**New Files:**
- `orchestrator/internal/webhooks/batch.go` - Batch endpoint handler
- `orchestrator/internal/webhooks/batch_test.go` - Comprehensive tests
- `orchestrator/internal/monitor/scanner.go` - Scanner stub (temporary until EPIC_07)
- `orchestrator/internal/monitor/scanner_test.go` - Scanner tests

**Modified Files:**
- `orchestrator/internal/webhooks/server.go` - Add /batch route

### Integration Points

- **Queue Interface** - Uses existing `QueueInterface` to enqueue tasks
- **Skip Checker** - Integrates with EPIC_06 skip logic (if available)
- **Monitor Package** - Creates scanner stub in monitor package for consistency
- **Server Routes** - Adds new POST route to webhook server

---

## Testing Strategy

### Unit Tests

**Scanner Tests** (`monitor/scanner_test.go`):
```go
func TestScanner_ScanDirectory_SingleFile(t *testing.T) {
    // Test scanning single file
}

func TestScanner_ScanDirectory_Recursive(t *testing.T) {
    // Test recursive directory scanning
}

func TestScanner_ScanDirectory_FilterMediaFiles(t *testing.T) {
    // Test media file filtering by extension
}
```

**Batch Handler Tests** (`webhooks/batch_test.go`):
```go
func TestHandleBatch_Success(t *testing.T) {
    // Happy path: valid directory, files found and queued
}

func TestHandleBatch_DirectoryNotFound(t *testing.T) {
    // Error: directory doesn't exist
}

func TestHandleBatch_NoMediaFiles(t *testing.T) {
    // Edge case: directory exists but no media files
}

func TestHandleBatch_PermissionDenied(t *testing.T) {
    // Error: directory not readable
}

func TestHandleBatch_RecursiveScanning(t *testing.T) {
    // Test recursive=true parameter
}

func TestHandleBatch_ForceLanguage(t *testing.T) {
    // Test language parameter passed through
}

func TestHandleBatch_SkipLogicIntegration(t *testing.T) {
    // Test skip checker filters files correctly
}
```

### Integration Tests

1. Create temporary directory structure with test media files
2. Call /batch endpoint with various parameters
3. Verify correct response statistics
4. Verify files queued correctly
5. Clean up test directories

### Manual Testing

```bash
# Setup test directory
mkdir -p /tmp/batch_test/movies
touch /tmp/batch_test/movies/movie1.mkv
touch /tmp/batch_test/movies/movie2.mp4
mkdir -p /tmp/batch_test/movies/action
touch /tmp/batch_test/movies/action/movie3.avi

# Test 1: Non-recursive scan
curl -X POST "http://localhost:9000/batch?directory=/tmp/batch_test/movies"
# Expected: scanned=2, only top-level files

# Test 2: Recursive scan
curl -X POST "http://localhost:9000/batch?directory=/tmp/batch_test/movies&recursive=true"
# Expected: scanned=3, includes subdirectory

# Test 3: Force language
curl -X POST "http://localhost:9000/batch?directory=/tmp/batch_test/movies&language=en"
# Expected: language parameter passed to tasks

# Test 4: Directory not found
curl -X POST "http://localhost:9000/batch?directory=/nonexistent"
# Expected: 400 error with clear message

# Clean up
rm -rf /tmp/batch_test
```

---

## Scanner Interface Design (Temporary Stub)

Since EPIC_07 STORY_03 (scanner implementation) is not yet complete, we create a minimal interface and stub:

```go
// orchestrator/internal/monitor/scanner.go
package monitor

// ScanResult contains statistics from directory scan
type ScanResult struct {
    Scanned     int
    Queued      int
    Skipped     int
    SkipReasons map[string]int
}

// Scanner scans directories for media files
type Scanner interface {
    ScanDirectory(directory string, recursive bool, language string) (*ScanResult, error)
}

// BasicScanner is a stub implementation until EPIC_07 STORY_03 completes
type BasicScanner struct {
    queue       QueueInterface
    skipChecker SkipChecker
}

// NewScanner creates a new scanner instance
func NewScanner(queue QueueInterface, skipChecker SkipChecker) Scanner {
    return &BasicScanner{
        queue:       queue,
        skipChecker: skipChecker,
    }
}

// ScanDirectory scans a directory for media files
// TODO: Replace with full implementation from EPIC_07 STORY_03
func (s *BasicScanner) ScanDirectory(directory string, recursive bool, language string) (*ScanResult, error) {
    // Stub implementation
    // Real implementation will:
    // 1. Walk directory tree (if recursive)
    // 2. Filter by media extensions
    // 3. Apply skip logic
    // 4. Queue files that should be processed
    // 5. Track statistics
    
    return &ScanResult{
        Scanned: 0,
        Queued:  0,
        Skipped: 0,
        SkipReasons: make(map[string]int),
    }, nil
}
```

---

## Definition of Done

- [ ] Story file created (this file)
- [ ] Tests written FIRST (TDD approach)
- [ ] All tests failing initially (red phase)
- [ ] Batch handler implemented (`batch.go`)
- [ ] Scanner stub created (`monitor/scanner.go`)
- [ ] Route added to webhook server
- [ ] All tests passing (green phase)
- [ ] Code refactored for clarity
- [ ] Error handling comprehensive
- [ ] Integration validated manually
- [ ] Work log created in `docs/WORKLOGS/`
- [ ] Code committed and pushed

---

## Dependencies

**Requires:**
- EPIC_01 (Go Orchestrator) - ✅ Complete (webhook server exists)
- EPIC_06 (Skip Logic) - ⚠️ Optional (graceful degradation if not available)

**External Libraries:**
- Standard library: `path/filepath`, `os`
- Fiber for HTTP handling (already used)

**Future Work:**
- Replace scanner stub with full implementation from EPIC_07 STORY_03
- Add authentication/authorization
- Add rate limiting for API endpoint

---

## Security Considerations

1. **Path Traversal** - Validate directory path is absolute and accessible
2. **Permission Checks** - Verify directory is readable before scanning
3. **Resource Limits** - Consider max files scanned in single request
4. **Authentication** - Optional basic auth for production use

---

## Performance Considerations

- Large directories (10,000+ files) may take time to scan
- Consider pagination or streaming response for huge directories
- Skip logic should be efficient (don't re-process already queued files)

---

## Notes

- Scanner is a temporary stub until EPIC_07 STORY_03 is implemented
- Skip logic integration assumes EPIC_06 is complete (graceful fallback if not)
- This endpoint complements webhook-based processing (manual bulk operation)

---

**Created:** 2026-02-15  
**Last Updated:** 2026-02-15
