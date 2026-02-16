# Story 04: Recursive Watching

**Epic**: EPIC_07 - File System Monitoring & Automated Processing  
**Status**: In Progress  
**Assignee**: Delegation Agent  
**Effort**: 4-6 hours  
**Priority**: MEDIUM

---

## User Story

As a Subgen operator,
I want the file watcher to monitor all subdirectories recursively,
So that new files added to any subfolder are automatically detected and processed.

---

## Acceptance Criteria

- [ ] Automatically add subdirectories to watcher during initialization
- [ ] Handle directory creation (add new subdirs to watcher dynamically)
- [ ] Handle directory deletion (remove from watcher gracefully)
- [ ] Performance: Efficient with deep directory hierarchies (100+ subdirectories)
- [ ] Support for symlinks (optional, with safety checks)
- [ ] Comprehensive unit tests (happy and unhappy paths)
- [ ] Integration tests with FileWatcher from STORY_01
- [ ] Work log created documenting implementation

---

## Problem Statement

**Current State:**
- FileWatcher only watches top-level directories specified in config
- Files added to subdirectories are NOT detected
- Users must explicitly list every subdirectory to watch

**Challenge:**
fsnotify doesn't watch subdirectories automatically. Each subdirectory must be manually added to the watcher using `watcher.Add()`.

**Use Cases:**
1. **TV Shows**: `/tv/ShowName/Season01/` - Need to watch all show/season folders
2. **Movies**: `/movies/Action/`, `/movies/Comedy/` - Need to watch all genre folders
3. **Dynamic Structure**: New folders created by download clients or media servers

**Example:**
```bash
# Without recursive watching
/movies/
  Action/
    movie1.mkv     # NOT detected
  Comedy/
    movie2.mkv     # NOT detected

# With recursive watching
/movies/
  Action/
    movie1.mkv     # ✅ Detected
  Comedy/
    movie2.mkv     # ✅ Detected
  Drama/           # ✅ New directory detected
    movie3.mkv     # ✅ File in new directory detected
```

---

## Technical Design

### Approach

Extend FileWatcher to:
1. **Initialization**: Walk directory tree and add all subdirectories to watcher
2. **Runtime**: Detect new directory creation events and add them to watcher
3. **Runtime**: Detect directory deletion events and remove them gracefully
4. **Safety**: Handle symlinks with cycle detection

**Key Components:**
1. **addRecursive()** - Recursively add subdirectories to watcher
2. **handleDirectoryCreated()** - Add new directories to watcher
3. **handleDirectoryDeleted()** - Remove directories from watcher
4. **isDirectory()** - Check if path is a directory (handles deleted paths)

**Design Decisions:**
- Use `filepath.Walk` for initial recursive addition (same as scanner)
- Check `fsnotify.Create` events to detect if created path is a directory
- Use `os.Stat()` to determine if path is directory vs file
- Gracefully handle `os.ErrNotExist` for deleted directories
- Log all directory additions/removals for debugging
- Skip symlinks by default (safety first, can be configurable later)

**Algorithm (Initialization):**
```
1. For each configured folder:
   a. Walk directory tree using filepath.Walk
   b. For each subdirectory found:
      - Add to fsnotify watcher
      - Log success/failure
   c. Track total directories watched
```

**Algorithm (Runtime - CREATE event):**
```
1. Receive fsnotify.Create event for path
2. Check if path still exists (os.Stat)
3. If path is directory:
   a. Add directory to watcher (watcher.Add)
   b. Recursively add subdirectories (addRecursive)
   c. Log directory addition
4. Else if path is file:
   a. Process as file (existing handleFileCreated)
```

**Algorithm (Runtime - REMOVE event):**
```
1. Receive fsnotify.Remove event for path
2. Gracefully handle removal (no need to explicitly remove from watcher)
3. Log directory removal
4. fsnotify automatically stops watching deleted paths
```

### Files to Create/Modify

**Create:**
- `orchestrator/internal/monitor/recursive.go` - Recursive watching implementation
- `orchestrator/internal/monitor/recursive_test.go` - Comprehensive tests

**Modify:**
- `orchestrator/internal/monitor/watcher.go`:
  - Call `addRecursive()` during Watch() initialization
  - Add directory detection to `handleFileCreated()`
  - Add `handleDirectoryCreated()` method
  - Add `isDirectory()` helper method
  - Update event handling to check for directory CREATE events

### Integration Points

- **FileWatcher (STORY_01)**: Core watcher that needs recursive capability
- **File Stability (STORY_02)**: Only applies to files, not directories
- **Scanner (STORY_03)**: Uses same recursive traversal pattern
- **Orchestrator Main**: No changes needed, transparent to caller

---

## Testing Strategy

### Unit Tests (orchestrator/internal/monitor/recursive_test.go)

**Happy Path Tests:**
1. `TestAddRecursive_SingleLevel` - Add single directory with no subdirs
2. `TestAddRecursive_MultiLevel` - Add directory with deep hierarchy
3. `TestAddRecursive_EmptyDirectory` - Handle directory with no subdirs
4. `TestFileWatcher_RecursiveInitialization` - All subdirs added on startup
5. `TestFileWatcher_NewDirectoryCreated` - New directory added to watcher
6. `TestFileWatcher_FileInSubdirectoryDetected` - File in subdir triggers callback

**Unhappy Path Tests:**
1. `TestAddRecursive_NonExistentDirectory` - Gracefully handle missing directory
2. `TestAddRecursive_PermissionDenied` - Skip inaccessible subdirectories
3. `TestFileWatcher_DirectoryDeleted` - Handle directory deletion gracefully
4. `TestAddRecursive_TooManyDirectories` - Handle 1000+ subdirectories

**Edge Cases:**
1. `TestAddRecursive_SymlinkDirectory` - Skip symlinks by default
2. `TestAddRecursive_CircularSymlink` - Prevent infinite loops
3. `TestFileWatcher_DirectoryCreatedThenDeleted` - Rapid create/delete
4. `TestFileWatcher_FileAndDirectorySameName` - Handle name collisions

### Integration Tests

```go
func TestFileWatcher_RecursiveIntegration(t *testing.T) {
    // Create test directory structure
    // /tmp/test/
    //   subdir1/
    //     subdir2/
    //       test.mkv
    
    // Start watcher
    // Verify all directories watched
    // Create file in deep subdir
    // Verify callback invoked
}

func TestFileWatcher_DynamicDirectoryCreation(t *testing.T) {
    // Start watcher on /tmp/test
    // Create new directory /tmp/test/newdir
    // Create file in newdir
    // Verify file detected
}
```

### Manual Testing

```bash
# Test setup
mkdir -p /tmp/subgen_recursive_test/{movies,tv}/{subfolder1,subfolder2}
cd orchestrator

# Run tests
go test ./internal/monitor/... -v -run TestRecursive

# Manual integration test
go run cmd/orchestrator/main.go

# In another terminal:
mkdir -p /tmp/subgen_recursive_test/movies/action
touch /tmp/subgen_recursive_test/movies/action/movie.mkv
# Expected: File detected and callback invoked

mkdir -p /tmp/subgen_recursive_test/movies/action/bluray
touch /tmp/subgen_recursive_test/movies/action/bluray/movie2.mkv
# Expected: File detected in deeply nested directory
```

---

## Implementation Details

### Recursive Directory Addition (recursive.go)

```go
// orchestrator/internal/monitor/recursive.go
package monitor

import (
    "os"
    "path/filepath"
)

// addRecursive recursively adds all subdirectories to the watcher
func (fw *FileWatcher) addRecursive(rootPath string) error {
    dirCount := 0
    
    err := filepath.Walk(rootPath, func(path string, info os.FileInfo, err error) error {
        if err != nil {
            fw.log.WithError(err).Warnf("Error accessing path: %s", path)
            // Continue walking despite errors
            return nil
        }
        
        // Only add directories
        if !info.IsDir() {
            return nil
        }
        
        // Skip symlinks to prevent infinite loops
        if info.Mode()&os.ModeSymlink != 0 {
            fw.log.Debugf("Skipping symlink directory: %s", path)
            return filepath.SkipDir
        }
        
        // Add directory to watcher
        if err := fw.watcher.Add(path); err != nil {
            fw.log.WithError(err).Warnf("Failed to watch subdirectory: %s", path)
        } else {
            dirCount++
            fw.log.Debugf("Watching subdirectory: %s", path)
        }
        
        return nil
    })
    
    if err != nil {
        return err
    }
    
    fw.log.WithField("directories", dirCount).Infof("Added %d directories to watcher", dirCount)
    return nil
}

// isDirectory checks if a path is a directory
// Returns false if path doesn't exist or is not a directory
func (fw *FileWatcher) isDirectory(path string) bool {
    info, err := os.Stat(path)
    if err != nil {
        return false
    }
    return info.IsDir()
}

// handleDirectoryCreated processes a directory creation event
func (fw *FileWatcher) handleDirectoryCreated(dirPath string) {
    fw.log.WithField("directory", dirPath).Info("Directory created")
    
    // Add the new directory and all its subdirectories to watcher
    if err := fw.addRecursive(dirPath); err != nil {
        fw.log.WithError(err).Warnf("Failed to add recursive watch for: %s", dirPath)
    } else {
        fw.log.WithField("directory", dirPath).Info("Added new directory to watcher")
    }
}
```

### Update FileWatcher (watcher.go modifications)

```go
// In Watch() method, after adding configured folders:
func (fw *FileWatcher) Watch(ctx context.Context) error {
    // ... existing watcher creation ...
    
    // Add all configured folders with recursive subdirectories
    for _, folder := range fw.folders {
        if err := fw.addRecursive(folder); err != nil {
            fw.log.WithError(err).Warnf("Failed to watch folder recursively: %s", folder)
            // Continue watching other folders even if one fails
        } else {
            fw.log.Infof("Watching folder recursively: %s", folder)
        }
    }
    
    // ... existing event loop ...
}

// Update handleFileCreated to detect directories:
func (fw *FileWatcher) handleFileCreated(filePath string) {
    // Check if this is a directory
    if fw.isDirectory(filePath) {
        fw.handleDirectoryCreated(filePath)
        return
    }
    
    // Existing file handling logic
    fw.log.WithField("file", filePath).Info("File created")
    
    if !fw.WaitForStability(filePath) {
        fw.log.WithField("file", filePath).Warn("File failed stability check, skipping")
        return
    }
    
    if fw.callback != nil {
        fw.callback(filePath)
    }
}
```

### Test Structure (recursive_test.go)

```go
// orchestrator/internal/monitor/recursive_test.go
package monitor_test

import (
    "context"
    "os"
    "path/filepath"
    "testing"
    "time"
    
    "github.com/stretchr/testify/assert"
    "github.com/stretchr/testify/require"
    
    "github.com/mccloud/subgen/orchestrator/internal/monitor"
)

func TestAddRecursive_SingleLevel(t *testing.T) {
    // Test adding directory with no subdirectories
}

func TestAddRecursive_MultiLevel(t *testing.T) {
    // Test adding directory with deep hierarchy
}

func TestFileWatcher_RecursiveInitialization(t *testing.T) {
    // Test all subdirectories added on startup
}

func TestFileWatcher_NewDirectoryCreated(t *testing.T) {
    // Test new directory detection and addition
}

func TestFileWatcher_FileInSubdirectoryDetected(t *testing.T) {
    // Test file detection in subdirectories
}

// ... additional tests ...
```

---

## Definition of Done

- [ ] Story file created with complete acceptance criteria
- [ ] recursive.go implemented with addRecursive(), isDirectory(), handleDirectoryCreated()
- [ ] watcher.go modified to use recursive watching
- [ ] All unit tests written FIRST (TDD)
- [ ] All unit tests passing (happy + unhappy + edge cases)
- [ ] Integration tests passing
- [ ] Performance test: 100+ subdirectories handled efficiently
- [ ] Code follows Go best practices
- [ ] Type safety maintained
- [ ] Comprehensive error handling
- [ ] Structured logging for debugging
- [ ] Work log created: `0021_2026-02-16_epic07_story04_recursive_watching.md`
- [ ] Code committed and pushed

---

## Success Criteria

- All subdirectories automatically added to watcher on initialization
- New directories detected and added to watcher dynamically
- Files in subdirectories (any depth) detected correctly
- Handles 100+ subdirectories with <100ms overhead
- Graceful handling of directory deletion, permission errors
- Symlinks skipped by default for safety
- All tests passing with >85% coverage

---

## Performance Characteristics

**Expected Performance:**
- 100 subdirectories: < 100ms to add all to watcher
- 1000 subdirectories: < 1 second to add all to watcher
- Directory create event: < 50ms to add to watcher
- Memory overhead: ~1KB per directory watched
- CPU overhead: Negligible (event-driven)

**Optimization Opportunities (Future):**
- Parallel directory addition with worker pool
- Configurable symlink following with cycle detection
- Directory depth limit configuration

---

## References

- **Epic README**: `/home/mikekao/personal/subgen/docs/BACKLOG/EPIC_07/README.md` lines 263-311
- **Primary Doc**: `/home/mikekao/personal/subgen/README-LLM.md`
- **STORY_01**: Basic FileWatcher implementation
- **STORY_03**: Scanner uses similar recursive pattern
- **fsnotify Documentation**: https://github.com/fsnotify/fsnotify
- **Original Python Implementation**: Uses watchdog's recursive=True parameter

---

**Created**: 2026-02-16  
**Last Updated**: 2026-02-16
