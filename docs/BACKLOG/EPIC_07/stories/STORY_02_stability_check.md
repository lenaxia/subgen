# Story 02: File Stability Checking

**Epic**: EPIC_07 - File System Monitoring & Automated Processing  
**Status**: In Progress  
**Assignee**: Delegation Agent  
**Effort**: 6-8 hours  
**Priority**: HIGH

---

## User Story

As a Subgen operator,
I want the file watcher to wait for file uploads/copies to complete before processing,
So that partial files are not transcribed, avoiding failures and wasted processing.

---

## Acceptance Criteria

- [x] Story file created with complete details
- [ ] 3-check stability algorithm implemented (check file size 3 times)
- [ ] 2-second intervals between checks (configurable)
- [ ] Only process files when size stops changing
- [ ] Handle partial uploads gracefully
- [ ] Configuration: `FILE_STABILITY_WAIT` (default: 2s), `FILE_STABILITY_CHECKS` (default: 3)
- [ ] Timeout protection (max 60s wait)
- [ ] Integration with FileWatcher from STORY_01
- [ ] Comprehensive unit tests (happy and unhappy paths)
- [ ] All tests passing
- [ ] Work log created

---

## Problem Statement

When files are being copied or uploaded to watched directories, the file creation event fires immediately but the file content is incomplete. If transcription starts before the upload completes:

1. File read errors occur (incomplete media file)
2. Transcription fails or produces incorrect results
3. Processing resources are wasted
4. User has to manually retry

**Original Python Implementation:**
```python
# Check file size 3 times with 2-second intervals
for i in range(3):
    size = os.path.getsize(file_path)
    time.sleep(2)
    new_size = os.path.getsize(file_path)
    if size == new_size:
        break  # Stable
    size = new_size
```

---

## Technical Design

### Approach

Implement a stability checking algorithm that monitors file size changes over time. The file is considered "stable" when its size remains unchanged for a configured number of checks at configured intervals.

**Key Components:**
1. **waitForStability()** - Main stability checking function
2. **Config fields** - StabilityChecks, StabilityWait, StabilityTimeout
3. **Integration** - Call stability check before invoking callback in handleFileCreated()

**Design Decisions:**
- Use configurable checks (default: 3) and intervals (default: 2s)
- Timeout protection prevents infinite waiting (default: 60s)
- Reset stable count when size changes (ensures consecutive stable checks)
- Return bool indicating success/failure
- Log stability status for debugging

**Algorithm:**
```
1. Initialize: lastSize = -1, stableCount = 0
2. Loop until stableCount >= checks OR timeout:
   a. Get current file size via os.Stat()
   b. If size == lastSize: increment stableCount
   c. If size != lastSize: reset stableCount to 0, update lastSize
   d. Sleep for interval duration
3. Return true if stable, false if timeout
```

### Files to Create

- `orchestrator/internal/monitor/config.go` - Add stability config fields (already exists from STORY_01)
- `orchestrator/internal/monitor/stability.go` - Stability checking implementation
- `orchestrator/internal/monitor/stability_test.go` - Comprehensive tests

### Files to Modify

- `orchestrator/internal/monitor/watcher.go` - Integrate stability check in handleFileCreated()

### Integration Points

- **FileWatcher.handleFileCreated()** - Call waitForStability() before callback
- **Config struct** - Add StabilityChecks, StabilityWait, StabilityTimeout fields
- **Logging** - Structured logs for stability events

---

## Testing Strategy

### Unit Tests (orchestrator/internal/monitor/stability_test.go)

**Happy Path Tests:**
1. `TestWaitForStability_StableFile` - File size stable, returns true immediately
2. `TestWaitForStability_StableAfterGrowth` - File grows then stabilizes, returns true
3. `TestWaitForStability_MultipleChecks` - Requires all checks to pass
4. `TestWaitForStability_ConfigurableChecks` - Respects StabilityChecks config
5. `TestWaitForStability_ConfigurableInterval` - Respects StabilityWait config

**Unhappy Path Tests:**
1. `TestWaitForStability_Timeout` - File never stabilizes, timeout returns false
2. `TestWaitForStability_FileDisappears` - File deleted during check, returns false
3. `TestWaitForStability_FileNotFound` - Non-existent file returns false immediately
4. `TestWaitForStability_PermissionDenied` - Cannot read file, returns false
5. `TestWaitForStability_ContinuousGrowth` - File keeps growing, timeout returns false

**Integration Tests:**
1. `TestFileWatcher_StabilityIntegration` - Full watcher with stability enabled
2. `TestFileWatcher_StabilityDisabled` - Callback invoked immediately if checks=0
3. `TestFileWatcher_StabilityTimeout` - Timeout during upload, callback not invoked

**Edge Cases:**
1. `TestWaitForStability_ZeroSizeFile` - Empty file is considered stable
2. `TestWaitForStability_VeryLargeFile` - Size > 2GB handled correctly
3. `TestWaitForStability_SimultaneousFiles` - Multiple files checked independently

### Manual Testing

```bash
# Test setup
mkdir -p /tmp/subgen_stability_test
cd orchestrator

# Run unit tests
go test ./internal/monitor/... -v -run TestWaitForStability

# Integration test: slow copy simulation
dd if=/dev/zero of=/tmp/large.mkv bs=1M count=100 &
cp /tmp/large.mkv /tmp/subgen_stability_test/test.mkv

# Expected: Watcher waits for copy completion before callback
# Check logs for: "Waiting for file stability" -> "File is stable"
```

---

## Implementation Details

### Stability Configuration (update config.go)

```go
// orchestrator/internal/monitor/config.go
package monitor

import "time"

// Config holds configuration for file system monitoring
type Config struct {
    // Existing fields from STORY_01
    Enabled bool
    Folders []string
    
    // STORY_02: File stability checking
    // StabilityChecks is the number of consecutive checks required (0 = disabled)
    StabilityChecks int
    
    // StabilityWait is the interval between stability checks
    StabilityWait time.Duration
    
    // StabilityTimeout is the maximum time to wait for stability
    StabilityTimeout time.Duration
}

// DefaultConfig returns a Config with sensible defaults
func DefaultConfig() *Config {
    return &Config{
        Enabled:          false,
        Folders:          []string{},
        StabilityChecks:  3,                    // 3 checks required
        StabilityWait:    2 * time.Second,      // 2s between checks
        StabilityTimeout: 60 * time.Second,     // 60s max wait
    }
}
```

### Stability Implementation (stability.go)

```go
// orchestrator/internal/monitor/stability.go
package monitor

import (
    "os"
    "time"
)

// waitForStability checks if a file has stopped growing/changing.
// Returns true if file is stable, false if timeout or error occurs.
func (fw *FileWatcher) waitForStability(filePath string) bool {
    // Stability checking disabled
    if fw.config.StabilityChecks <= 0 {
        return true
    }
    
    fw.log.WithField("file", filePath).Debug("Waiting for file stability")
    
    checks := fw.config.StabilityChecks
    interval := fw.config.StabilityWait
    timeout := time.After(fw.config.StabilityTimeout)
    
    var lastSize int64 = -1
    stableCount := 0
    
    for stableCount < checks {
        select {
        case <-timeout:
            fw.log.WithField("file", filePath).Warn("Stability check timeout")
            return false
            
        default:
            // Get current file size
            stat, err := os.Stat(filePath)
            if err != nil {
                fw.log.WithError(err).WithField("file", filePath).Error("Failed to stat file during stability check")
                return false
            }
            
            currentSize := stat.Size()
            
            // Check if size is stable
            if currentSize == lastSize {
                stableCount++
                fw.log.WithFields(map[string]interface{}{
                    "file":        filePath,
                    "size":        currentSize,
                    "stableCount": stableCount,
                    "required":    checks,
                }).Debug("File size stable")
            } else {
                // Size changed, reset counter
                if lastSize != -1 {
                    fw.log.WithFields(map[string]interface{}{
                        "file":    filePath,
                        "oldSize": lastSize,
                        "newSize": currentSize,
                    }).Debug("File size changed, resetting stability counter")
                }
                stableCount = 0
                lastSize = currentSize
            }
            
            // Wait before next check (unless this was the last check)
            if stableCount < checks {
                time.Sleep(interval)
            }
        }
    }
    
    fw.log.WithFields(map[string]interface{}{
        "file": filePath,
        "size": lastSize,
    }).Info("File is stable")
    
    return true
}
```

### Integration with FileWatcher (update watcher.go)

```go
// orchestrator/internal/monitor/watcher.go
package monitor

import (
    "context"
    "fmt"
    
    "github.com/fsnotify/fsnotify"
    "github.com/sirupsen/logrus"
)

// FileCallback is called when a new file is detected
type FileCallback func(filePath string)

// FileWatcher monitors directories for new media files
type FileWatcher struct {
    watcher  *fsnotify.Watcher
    folders  []string
    callback FileCallback
    config   *Config  // Add config field
    log      *logrus.Logger
}

// NewFileWatcher creates a new FileWatcher instance
func NewFileWatcher(folders []string, callback FileCallback, config *Config, log *logrus.Logger) (*FileWatcher, error) {
    if log == nil {
        return nil, fmt.Errorf("logger cannot be nil")
    }
    
    if config == nil {
        config = DefaultConfig()
    }
    
    return &FileWatcher{
        folders:  folders,
        callback: callback,
        config:   config,
        log:      log,
    }, nil
}

// Watch starts monitoring configured directories for file creation events.
// It blocks until the context is canceled or an unrecoverable error occurs.
func (fw *FileWatcher) Watch(ctx context.Context) error {
    // ... existing Watch() implementation unchanged ...
}

// handleFileCreated processes a file creation event
func (fw *FileWatcher) handleFileCreated(filePath string) {
    fw.log.WithField("file", filePath).Info("File created")
    
    // Wait for file stability before processing
    if !fw.waitForStability(filePath) {
        fw.log.WithField("file", filePath).Warn("File failed stability check, skipping")
        return
    }
    
    if fw.callback != nil {
        fw.callback(filePath)
    }
}
```

### Test Structure (stability_test.go)

```go
// orchestrator/internal/monitor/stability_test.go
package monitor_test

import (
    "context"
    "os"
    "path/filepath"
    "testing"
    "time"
    
    "github.com/mccloud/subgen/orchestrator/internal/monitor"
    "github.com/sirupsen/logrus"
    "github.com/stretchr/testify/assert"
    "github.com/stretchr/testify/require"
)

// Test helper functions
func setupStabilityTest(t *testing.T) (string, *monitor.Config, *logrus.Logger) {
    t.Helper()
    dir, err := os.MkdirTemp("", "subgen_stability_test_*")
    require.NoError(t, err)
    t.Cleanup(func() { os.RemoveAll(dir) })
    
    config := monitor.DefaultConfig()
    log := logrus.New()
    log.SetLevel(logrus.ErrorLevel)
    
    return dir, config, log
}

// Happy path tests
func TestWaitForStability_StableFile(t *testing.T) { /* ... */ }
func TestWaitForStability_StableAfterGrowth(t *testing.T) { /* ... */ }
func TestWaitForStability_MultipleChecks(t *testing.T) { /* ... */ }
func TestWaitForStability_ConfigurableChecks(t *testing.T) { /* ... */ }
func TestWaitForStability_ConfigurableInterval(t *testing.T) { /* ... */ }

// Unhappy path tests
func TestWaitForStability_Timeout(t *testing.T) { /* ... */ }
func TestWaitForStability_FileDisappears(t *testing.T) { /* ... */ }
func TestWaitForStability_FileNotFound(t *testing.T) { /* ... */ }
func TestWaitForStability_PermissionDenied(t *testing.T) { /* ... */ }
func TestWaitForStability_ContinuousGrowth(t *testing.T) { /* ... */ }

// Integration tests
func TestFileWatcher_StabilityIntegration(t *testing.T) { /* ... */ }
func TestFileWatcher_StabilityDisabled(t *testing.T) { /* ... */ }
func TestFileWatcher_StabilityTimeout(t *testing.T) { /* ... */ }

// Edge cases
func TestWaitForStability_ZeroSizeFile(t *testing.T) { /* ... */ }
func TestWaitForStability_VeryLargeFile(t *testing.T) { /* ... */ }
func TestWaitForStability_SimultaneousFiles(t *testing.T) { /* ... */ }
```

---

## Definition of Done

- [x] Story file created with complete acceptance criteria
- [ ] Config struct updated with stability fields
- [ ] waitForStability() method implemented
- [ ] FileWatcher integrated with stability checking
- [ ] All unit tests written FIRST (TDD)
- [ ] All unit tests passing (happy + unhappy + edge cases)
- [ ] Integration tests passing
- [ ] Code follows Go best practices
- [ ] Type safety maintained
- [ ] Comprehensive error handling
- [ ] Structured logging for debugging
- [ ] Work log created: `0017_2026-02-16_epic07_story02_stability_check.md`
- [ ] Code committed and pushed

---

## Success Criteria

- File size checked 3 times with 2-second intervals (configurable)
- Only process files when size stops changing for all checks
- Timeout protection prevents infinite waiting (max 60s)
- Graceful handling of file disappearance, permission errors
- Configuration via Config struct (no environment variables yet)
- All tests passing with >85% coverage
- Integration with FileWatcher seamless

---

## References

- **Epic README**: `/home/mikekao/personal/subgen/docs/BACKLOG/EPIC_07/README.md` lines 122-189
- **Primary Doc**: `/home/mikekao/personal/subgen/README-LLM.md`
- **STORY_01**: `/home/mikekao/personal/subgen/docs/BACKLOG/EPIC_07/stories/STORY_01_basic_watcher.md`
- **Original Python Implementation**: `subgen.py` lines 2110-2123

---

**Created**: 2026-02-16  
**Last Updated**: 2026-02-16
