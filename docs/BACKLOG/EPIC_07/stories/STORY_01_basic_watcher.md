# Story 01: Basic File Watcher

**Epic**: EPIC_07 - File System Monitoring & Automated Processing  
**Status**: Complete  
**Assignee**: Delegation Agent  
**Effort**: 8-10 hours  
**Priority**: HIGH

---

## User Story

As a Subgen operator,
I want the orchestrator to monitor configured directories for new media files,
So that new files are automatically queued for transcription without manual intervention.

---

## Acceptance Criteria

- [x] Use `github.com/fsnotify/fsnotify` for cross-platform file watching
- [x] Watch single directory for file creation events
- [x] Filter events: only CREATE, ignore WRITE/CHMOD/REMOVE
- [x] Log file creation events with full path
- [x] Configuration: `MONITOR` (bool), `TRANSCRIBE_FOLDERS` (pipe-separated)
- [x] Graceful startup and shutdown with context-based cancellation
- [x] Comprehensive unit tests (happy and unhappy paths)
- [x] Type-safe implementation with proper error handling
- [x] Work log created documenting implementation

---

## Technical Design

### Approach

Implement a Go-based file watcher using `github.com/fsnotify/fsnotify` that monitors configured directories for CREATE events. The watcher will run in a goroutine and use context-based cancellation for graceful shutdown.

**Key Components:**
1. **FileWatcher struct** - Main watcher with fsnotify integration
2. **Config struct** - Configuration for monitoring
3. **Watch() method** - Event loop that filters CREATE events
4. **Callback mechanism** - Function to handle file creation

**Design Decisions:**
- Use `fsnotify.Create` flag for filtering (not WRITE/CHMOD/REMOVE)
- Context-based cancellation for clean shutdown
- Callback pattern for decoupling file handling from watching
- Single directory watching (recursive watching in STORY_04)
- Error logging rather than propagation for non-critical errors

### Files to Create

- `orchestrator/internal/monitor/config.go` - Configuration struct
- `orchestrator/internal/monitor/watcher.go` - FileWatcher implementation
- `orchestrator/internal/monitor/watcher_test.go` - Comprehensive tests

### Integration Points

- **Task Queue** (Future): Callback will enqueue files for transcription
- **Skip Logic** (EPIC_06): Callback will check skip logic before queuing
- **Orchestrator Main**: Will be started as goroutine when MONITOR=true

---

## Testing Strategy

### Unit Tests (orchestrator/internal/monitor/watcher_test.go)

**Happy Path Tests:**
1. `TestFileWatcher_NewFileWatcher` - Constructor creates watcher with valid config
2. `TestFileWatcher_Watch_CreateEvent` - CREATE event triggers callback
3. `TestFileWatcher_Watch_MultipleFiles` - Multiple CREATE events handled
4. `TestFileWatcher_Watch_GracefulShutdown` - Context cancellation stops watcher
5. `TestFileWatcher_Watch_MultipleFolders` - Watch multiple directories

**Unhappy Path Tests:**
1. `TestFileWatcher_Watch_InvalidFolder` - Non-existent folder logged as error, continues
2. `TestFileWatcher_Watch_WriteEventIgnored` - WRITE events do not trigger callback
3. `TestFileWatcher_Watch_ChmodEventIgnored` - CHMOD events do not trigger callback
4. `TestFileWatcher_Watch_RemoveEventIgnored` - REMOVE events do not trigger callback
5. `TestFileWatcher_Watch_WatcherError` - Watcher errors logged, processing continues
6. `TestFileWatcher_Watch_NilCallback` - Gracefully handles nil callback
7. `TestFileWatcher_Watch_ContextCanceledBeforeStart` - Handles pre-canceled context

**Edge Cases:**
1. `TestFileWatcher_Watch_EmptyFolderList` - No folders configured
2. `TestFileWatcher_Watch_DuplicateFolder` - Same folder added twice
3. `TestFileWatcher_Watch_SymlinkFolder` - Symlinked directories

### Manual Testing

```bash
# Test setup
mkdir -p /tmp/subgen_test/watch_folder
cd orchestrator

# Run tests
go test ./internal/monitor/... -v

# Manual test with real watcher
go run cmd/orchestrator/main.go

# In another terminal:
touch /tmp/subgen_test/watch_folder/test.mkv
# Expected: Log message "File created: /tmp/subgen_test/watch_folder/test.mkv"
```

---

## Implementation Details

### Configuration Structure

```go
// orchestrator/internal/monitor/config.go
package monitor

import "time"

// Config holds configuration for file system monitoring
type Config struct {
    // Enabled determines if monitoring is active
    Enabled bool
    
    // Folders is a list of directories to watch for new files
    Folders []string
    
    // StabilityChecks is the number of file size checks (STORY_02)
    StabilityChecks int
    
    // StabilityWait is the interval between stability checks (STORY_02)
    StabilityWait time.Duration
    
    // StabilityTimeout is the maximum wait time for stability (STORY_02)
    StabilityTimeout time.Duration
}

// DefaultConfig returns a Config with sensible defaults
func DefaultConfig() *Config {
    return &Config{
        Enabled:          false,
        Folders:          []string{},
        StabilityChecks:  3,
        StabilityWait:    2 * time.Second,
        StabilityTimeout: 60 * time.Second,
    }
}
```

### FileWatcher Implementation

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
    log      *logrus.Logger
}

// NewFileWatcher creates a new FileWatcher instance
func NewFileWatcher(folders []string, callback FileCallback, log *logrus.Logger) (*FileWatcher, error) {
    if log == nil {
        return nil, fmt.Errorf("logger cannot be nil")
    }
    
    return &FileWatcher{
        folders:  folders,
        callback: callback,
        log:      log,
    }, nil
}

// Watch starts monitoring configured directories for file creation events.
// It blocks until the context is canceled or an unrecoverable error occurs.
func (fw *FileWatcher) Watch(ctx context.Context) error {
    // Create fsnotify watcher
    watcher, err := fsnotify.NewWatcher()
    if err != nil {
        return fmt.Errorf("failed to create watcher: %w", err)
    }
    defer watcher.Close()
    
    fw.watcher = watcher
    
    // Add all configured folders
    for _, folder := range fw.folders {
        if err := watcher.Add(folder); err != nil {
            fw.log.WithError(err).Warnf("Failed to watch folder: %s", folder)
            // Continue watching other folders even if one fails
        } else {
            fw.log.Infof("Watching folder: %s", folder)
        }
    }
    
    // Event loop
    for {
        select {
        case event, ok := <-watcher.Events:
            if !ok {
                fw.log.Info("Watcher events channel closed")
                return nil
            }
            
            // Only handle CREATE events
            if event.Op&fsnotify.Create == fsnotify.Create {
                fw.handleFileCreated(event.Name)
            }
            
        case err, ok := <-watcher.Errors:
            if !ok {
                fw.log.Info("Watcher errors channel closed")
                return nil
            }
            fw.log.WithError(err).Error("Watcher error")
            // Continue processing despite errors
            
        case <-ctx.Done():
            fw.log.Info("Watcher shutdown requested")
            return ctx.Err()
        }
    }
}

// handleFileCreated processes a file creation event
func (fw *FileWatcher) handleFileCreated(filePath string) {
    fw.log.WithField("file", filePath).Info("File created")
    
    if fw.callback != nil {
        fw.callback(filePath)
    }
}
```

### Test Implementation Structure

```go
// orchestrator/internal/monitor/watcher_test.go
package monitor_test

import (
    "context"
    "os"
    "path/filepath"
    "testing"
    "time"
    
    "github.com/sirupsen/logrus"
    "github.com/stretchr/testify/assert"
    "github.com/stretchr/testify/require"
    
    "orchestrator/internal/monitor"
)

func TestFileWatcher_NewFileWatcher(t *testing.T) {
    // Test constructor
}

func TestFileWatcher_Watch_CreateEvent(t *testing.T) {
    // Test CREATE event triggers callback
}

// ... additional tests
```

---

## Definition of Done

- [x] Story file created with complete acceptance criteria
- [x] Config struct implemented with type safety
- [x] FileWatcher struct implemented with fsnotify
- [x] Watch() method filters CREATE events only
- [x] Context-based cancellation working
- [x] All unit tests written FIRST (TDD)
- [x] All unit tests passing (happy + unhappy paths)
- [x] Integration points documented
- [x] Code follows Go best practices
- [x] Work log created: `0015_2026-02-15_epic07_story01_basic_watcher.md`
- [x] Code committed and pushed

---

## References

- **Epic README**: `/home/mikekao/personal/subgen/docs/BACKLOG/EPIC_07/README.md`
- **Primary Doc**: `/home/mikekao/personal/subgen/README-LLM.md`
- **fsnotify Documentation**: https://github.com/fsnotify/fsnotify
- **Original Python Implementation**: `subgen.py` lines 2087-2144

---

**Created**: 2026-02-15  
**Last Updated**: 2026-02-15
