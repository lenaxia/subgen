# EPIC_07: File System Monitoring & Automated Processing

**Status:** Not Started  
**Estimated Effort:** 28-36 hours  
**Duration:** 4-5 days  
**Priority:** 🟡 HIGH (Core Automation Feature)  
**Can Parallelize:** Partially (stories 1-3 independent)

---

## Overview

Implement automated file system monitoring that watches configured directories for new media files and automatically triggers transcription. This enables "set it and forget it" operation where Subgen processes new files as they're added to the media library without requiring webhooks or manual intervention.

**Impact:** This is a **core feature** of the original subgen.py that enables fully automated subtitle generation for media libraries. Users can point Subgen at their media folders and have subtitles generated automatically.

---

## Problem Statement

**Current State:**
- Webhook-based processing only (requires media server integration)
- No automated folder watching
- No startup scanning for existing files
- Manual intervention required for batch processing

**Original subgen.py Behavior:**
- `MONITOR=true` enables watchdog-based folder monitoring
- `TRANSCRIBE_FOLDERS=/movies|/tv` specifies directories to watch
- Scans folders on startup for existing media files
- Watches for new file creation events
- File stability checking (waits for upload completion)
- Recursive directory traversal

**Use Cases:**
1. **Seedbox/Download Client:** New downloads → automatic subtitles
2. **Network Storage:** Files added from multiple sources
3. **Initial Setup:** Scan entire library and process missing subtitles
4. **Standalone Mode:** No media server integration required

---

## Goals

1. Implement Go-based file system watcher (replace Python watchdog)
2. Startup folder scanning with recursive traversal
3. File stability checking (wait for complete uploads)
4. Configurable watch directories
5. Event filtering (new files only, ignore modifications)
6. Integration with task queue and skip logic
7. Performance: Handle 10,000+ file libraries efficiently

---

## User Stories

### [STORY_01: Basic File Watcher](./stories/STORY_01_basic_watcher.md)
**Status:** Not Started  
**Effort:** 8-10 hours  
**Priority:** HIGH  
**Summary:** Implement basic file system event monitoring using fsnotify

**Acceptance Criteria:**
- [ ] Use `github.com/fsnotify/fsnotify` for cross-platform file watching
- [ ] Watch single directory for file creation events
- [ ] Filter events: only CREATE, ignore WRITE/CHMOD/REMOVE
- [ ] Log file creation events with full path
- [ ] Configuration: `MONITOR` (bool), `TRANSCRIBE_FOLDERS` (pipe-separated)
- [ ] Graceful startup and shutdown

**Implementation:**
```go
// orchestrator/internal/monitor/watcher.go
package monitor

import "github.com/fsnotify/fsnotify"

type FileWatcher struct {
    watcher   *fsnotify.Watcher
    folders   []string
    callback  func(filePath string)
    log       *logrus.Logger
}

func (fw *FileWatcher) Watch(ctx context.Context) error {
    watcher, err := fsnotify.NewWatcher()
    if err != nil {
        return err
    }
    defer watcher.Close()
    
    // Add folders
    for _, folder := range fw.folders {
        if err := watcher.Add(folder); err != nil {
            fw.log.Errorf("Failed to watch %s: %v", folder, err)
        }
    }
    
    // Event loop
    for {
        select {
        case event, ok := <-watcher.Events:
            if !ok {
                return nil
            }
            if event.Op&fsnotify.Create == fsnotify.Create {
                fw.handleFileCreated(event.Name)
            }
        case err, ok := <-watcher.Errors:
            if !ok {
                return nil
            }
            fw.log.Errorf("Watcher error: %v", err)
        case <-ctx.Done():
            return ctx.Err()
        }
    }
}
```

---

### [STORY_02: File Stability Checking](./stories/STORY_02_stability_check.md)
**Status:** Not Started  
**Effort:** 6-8 hours  
**Priority:** HIGH  
**Summary:** Wait for file upload/copy completion before processing

**Acceptance Criteria:**
- [ ] 3-check stability algorithm (check file size 3 times)
- [ ] 2-second intervals between checks
- [ ] Only process files when size stops changing
- [ ] Handle partial uploads gracefully
- [ ] Configuration: `FILE_STABILITY_WAIT` (default: 2s), `FILE_STABILITY_CHECKS` (default: 3)
- [ ] Timeout protection (max 60s wait)

**Problem:**
When files are being copied/uploaded, file creation event fires immediately but file is incomplete. Processing partial files causes failures.

**Solution (from original):**
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

**Go Implementation:**
```go
// orchestrator/internal/monitor/stability.go
func (fw *FileWatcher) waitForStability(filePath string) bool {
    checks := fw.config.StabilityChecks  // default: 3
    interval := fw.config.StabilityWait  // default: 2s
    timeout := time.After(60 * time.Second)
    
    var lastSize int64 = -1
    stableCount := 0
    
    for stableCount < checks {
        select {
        case <-timeout:
            fw.log.Warnf("Stability timeout for %s", filePath)
            return false
        default:
            stat, err := os.Stat(filePath)
            if err != nil {
                return false
            }
            
            currentSize := stat.Size()
            if currentSize == lastSize {
                stableCount++
            } else {
                stableCount = 0
                lastSize = currentSize
            }
            
            time.Sleep(interval)
        }
    }
    
    return true
}
```

---

### [STORY_03: Recursive Directory Scanning](./stories/STORY_03_recursive_scan.md)
**Status:** Not Started  
**Effort:** 6-8 hours  
**Priority:** HIGH  
**Summary:** Scan directories recursively on startup and process existing files

**Acceptance Criteria:**
- [ ] Recursive directory traversal using `filepath.Walk`
- [ ] Filter media files by extension (.mp4, .mkv, .avi, .mp3, .flac, etc.)
- [ ] Queue files that pass skip logic
- [ ] Progress logging (every 100 files scanned)
- [ ] Configuration: `SCAN_ON_STARTUP` (default: true)
- [ ] Performance: Handle 10,000+ files efficiently

**Use Case:**
```bash
# User scenario
docker run -e MONITOR=true -e TRANSCRIBE_FOLDERS=/movies -v /data/movies:/movies subgen

# Expected behavior:
# 1. Startup: Scan /movies recursively
# 2. Find 5000 movie files
# 3. Skip 4800 (already have subtitles)
# 4. Queue 200 for transcription
# 5. Continue watching for new files
```

**Implementation:**
```go
// orchestrator/internal/monitor/scanner.go
func (s *Scanner) ScanDirectory(ctx context.Context, rootPath string) (int, int, error) {
    scanned := 0
    queued := 0
    
    err := filepath.Walk(rootPath, func(path string, info os.FileInfo, err error) error {
        if err != nil {
            return nil  // Skip errors, continue walking
        }
        
        // Skip directories
        if info.IsDir() {
            return nil
        }
        
        // Filter by extension
        if !s.isMediaFile(path) {
            return nil
        }
        
        scanned++
        if scanned%100 == 0 {
            s.log.Infof("Scanned %d files...", scanned)
        }
        
        // Check skip logic
        skipResult, _ := s.skipChecker.Check(ctx, path, s.config.TargetLanguage)
        if skipResult.ShouldSkip {
            return nil
        }
        
        // Queue for transcription
        s.queue.Enqueue(Task{FilePath: path, Priority: 2})
        queued++
        
        return nil
    })
    
    return scanned, queued, err
}
```

---

### [STORY_04: Recursive Watching](./stories/STORY_04_recursive_watching.md)
**Status:** Not Started  
**Effort:** 4-6 hours  
**Priority:** MEDIUM  
**Summary:** Watch all subdirectories, not just top-level folder

**Acceptance Criteria:**
- [ ] Automatically add subdirectories to watcher
- [ ] Handle directory creation (add new subdirs to watcher)
- [ ] Handle directory deletion (remove from watcher)
- [ ] Performance: Efficient with deep directory hierarchies
- [ ] Support for symlinks (optional)

**Challenge:**
fsnotify doesn't watch subdirectories automatically. Must manually add each subdirectory.

**Solution:**
```go
// orchestrator/internal/monitor/recursive.go
func (fw *FileWatcher) addRecursive(rootPath string) error {
    return filepath.Walk(rootPath, func(path string, info os.FileInfo, err error) error {
        if err != nil {
            return nil
        }
        
        if info.IsDir() {
            if err := fw.watcher.Add(path); err != nil {
                fw.log.Warnf("Failed to watch %s: %v", path, err)
            }
        }
        
        return nil
    })
}

// Handle new directories
func (fw *FileWatcher) handleEvent(event fsnotify.Event) {
    if event.Op&fsnotify.Create == fsnotify.Create {
        stat, err := os.Stat(event.Name)
        if err == nil && stat.IsDir() {
            // New directory created, add to watcher
            fw.addRecursive(event.Name)
        }
    }
}
```

---

### [STORY_05: Media File Filtering](./stories/STORY_05_media_filtering.md)
**Status:** Not Started  
**Effort:** 2-4 hours  
**Priority:** LOW  
**Summary:** Filter files by media extensions and characteristics

**Acceptance Criteria:**
- [ ] Extension whitelist for video (.mp4, .mkv, .avi, .mov, .m4v, .webm, .flv)
- [ ] Extension whitelist for audio (.mp3, .flac, .m4a, .wav, .ogg, .opus)
- [ ] Minimum file size filter (default: 1MB)
- [ ] Configuration: `MEDIA_EXTENSIONS`, `MIN_FILE_SIZE`
- [ ] Case-insensitive extension matching

**Configuration:**
```env
# Default extensions
MEDIA_VIDEO_EXTENSIONS=mp4,mkv,avi,mov,m4v,webm,flv,wmv,mpg,mpeg
MEDIA_AUDIO_EXTENSIONS=mp3,flac,m4a,wav,ogg,opus,wma
MIN_FILE_SIZE=1048576  # 1MB in bytes

# Or disable filtering
FILTER_BY_EXTENSION=false  # Process all files
```

---

### [STORY_06: Integration & Performance Testing](./stories/STORY_06_monitoring_integration.md)
**Status:** Not Started  
**Effort:** 4-6 hours  
**Priority:** HIGH  
**Summary:** Integrate monitoring with orchestrator and test at scale

**Acceptance Criteria:**
- [ ] Monitor启动 with orchestrator
- [ ] Multiple watch folders support
- [ ] Integration with skip logic (EPIC_06)
- [ ] Performance test: 10,000 file scan in < 30s
- [ ] Memory efficiency: < 50MB overhead for monitoring
- [ ] CPU efficiency: < 5% CPU when idle
- [ ] Integration tests with real file operations

**Integration Points:**
```go
// orchestrator/cmd/orchestrator/main.go
func main() {
    // ...
    
    if config.Monitor {
        watcher := monitor.NewFileWatcher(
            folders: config.TranscribeFolders,
            queue: taskQueue,
            skipChecker: skipChecker,
            log: logger,
        )
        
        // Startup scan
        if config.ScanOnStartup {
            for _, folder := range config.TranscribeFolders {
                scanned, queued, err := watcher.Scanner.ScanDirectory(ctx, folder)
                logger.Infof("Scanned %s: %d files, %d queued", folder, scanned, queued)
            }
        }
        
        // Start watching
        go watcher.Watch(ctx)
    }
    
    // ...
}
```

---

## Architecture

### Component Structure

```
orchestrator/internal/monitor/
├── watcher.go          # Main FileWatcher struct and Watch() loop
├── stability.go        # File stability checking
├── scanner.go          # Recursive directory scanning
├── filter.go           # Media file extension filtering
├── config.go           # Monitor configuration
└── watcher_test.go     # Comprehensive tests
```

### Data Flow

```
1. Startup
   ├─> Config: MONITOR=true, TRANSCRIBE_FOLDERS=/movies|/tv
   ├─> Scanner: Recursive scan of /movies and /tv
   │   ├─> Find 10,000 files
   │   ├─> Filter by extension → 8,500 media files
   │   ├─> Skip logic check → 8,300 already have subtitles
   │   └─> Queue 200 files for transcription
   └─> Watcher: Start watching /movies and /tv

2. File Creation Event
   ├─> fsnotify: CREATE event for /movies/new_movie.mkv
   ├─> Stability Check: Wait until file size stable (6 seconds)
   ├─> Extension Check: .mkv → valid media file
   ├─> Skip Logic: No subtitle exists → proceed
   └─> Queue: Enqueue transcription task (priority 2)

3. Processing
   ├─> Worker fetches task from queue
   ├─> Transcribe via gRPC worker
   └─> Generate subtitle file
```

---

## Configuration

### Environment Variables

```env
# Enable monitoring
MONITOR=true

# Directories to watch (pipe-separated)
TRANSCRIBE_FOLDERS=/movies|/tv|/anime

# Scan on startup
SCAN_ON_STARTUP=true

# File stability checking
FILE_STABILITY_CHECKS=3      # Number of stability checks
FILE_STABILITY_WAIT=2s       # Interval between checks
FILE_STABILITY_TIMEOUT=60s   # Max wait time

# Media filtering
MEDIA_VIDEO_EXTENSIONS=mp4,mkv,avi,mov,m4v,webm,flv,wmv,mpg,mpeg
MEDIA_AUDIO_EXTENSIONS=mp3,flac,m4a,wav,ogg,opus,wma,aac
MIN_FILE_SIZE=1048576        # 1MB minimum

# Performance
SCAN_WORKERS=4               # Parallel scan workers (default: CPU cores)
```

### Example Configurations

**Basic Monitoring:**
```env
MONITOR=true
TRANSCRIBE_FOLDERS=/data/media
```

**Production (Multiple Folders):**
```env
MONITOR=true
TRANSCRIBE_FOLDERS=/movies|/tv|/anime|/documentaries
SCAN_ON_STARTUP=true
FILE_STABILITY_WAIT=3s       # Slower storage
FILE_STABILITY_CHECKS=4      # Extra safety
```

**High-Performance (Fast Storage):**
```env
MONITOR=true
TRANSCRIBE_FOLDERS=/nvme/media
SCAN_ON_STARTUP=true
FILE_STABILITY_WAIT=1s       # Fast SSD
FILE_STABILITY_CHECKS=2      # Quick checks
SCAN_WORKERS=8               # Parallel scanning
```

---

## Dependencies

**Requires:**
- EPIC_01 (Go Orchestrator) - ✅ Complete
- EPIC_06 (Skip Logic) - ⚠️ Recommended but not blocking

**External Libraries:**
- `github.com/fsnotify/fsnotify` - Cross-platform file watching
- Standard library: `path/filepath`, `os`, `time`

**Blocks:**
- EPIC_05 (Migration) - Monitoring is a key feature for migration

**Parallelizable:**
- STORY_01-05 can be developed independently
- STORY_06 requires all others

---

## Testing Strategy

### Unit Tests
- File stability algorithm
- Extension filtering
- Recursive scanning logic
- Event handling

### Integration Tests
- Create test directories with media files
- Trigger file creation events
- Verify tasks queued correctly
- Test with skip logic enabled

### Performance Tests
- Scan 10,000 files in < 30s
- Watch 100 directories simultaneously
- Memory usage < 50MB
- CPU usage < 5% when idle

### Manual Testing
```bash
# Test setup
mkdir -p /tmp/test_monitor/movies
docker-compose up -d

# Test 1: Startup scan
touch /tmp/test_monitor/movies/movie1.mkv
# Expected: File queued on startup

# Test 2: File stability
cp large_movie.mkv /tmp/test_monitor/movies/movie2.mkv
# Expected: Wait for copy completion before queuing

# Test 3: Recursive directories
mkdir -p /tmp/test_monitor/movies/action
touch /tmp/test_monitor/movies/action/movie3.mkv
# Expected: File detected in subdirectory
```

---

## Risks & Mitigation

| Risk | Impact | Mitigation |
|------|--------|------------|
| fsnotify events missed (buffer overflow) | HIGH | Increase event buffer, add event queue |
| File stability detection fails | MEDIUM | Configurable checks/intervals, timeout protection |
| Recursive watching performance | MEDIUM | Limit depth, add exclude patterns |
| High CPU on large scans | MEDIUM | Parallel scanning with worker pool, rate limiting |
| Permission errors on network storage | LOW | Graceful error handling, skip inaccessible paths |

---

## Success Metrics

- [ ] **Startup scan:** 10,000 files in < 30 seconds
- [ ] **Event detection:** < 5 second latency from file creation to queue
- [ ] **Resource usage:** < 50MB memory, < 5% CPU idle
- [ ] **Reliability:** 99.9% event detection (no missed files)
- [ ] **Stability:** No false positives from incomplete uploads

---

## Timeline

**Day 1:** STORY_01 (Basic watcher with fsnotify)  
**Day 2:** STORY_02 (File stability checking)  
**Day 3:** STORY_03 (Recursive scanning on startup)  
**Day 4:** STORY_04 (Recursive watching) + STORY_05 (Media filtering)  
**Day 5:** STORY_06 (Integration & performance testing)

---

## Definition of Done

- [ ] All 6 stories completed with ✅ status
- [ ] fsnotify-based file watcher implemented
- [ ] File stability checking with configurable parameters
- [ ] Recursive directory scanning on startup
- [ ] Recursive watching of subdirectories
- [ ] Media file filtering by extension
- [ ] Integration with task queue and skip logic
- [ ] Unit tests (>85% coverage)
- [ ] Integration tests with real file operations
- [ ] Performance tests meet targets
- [ ] Documentation complete (configuration guide)
- [ ] Work logs for each story

---

## References

- **Original Implementation:** `/home/mikekao/personal/subgen/subgen.py` lines 2087-2144
- **watchdog Usage:** Lines 45-47 (import), 2087-2144 (implementation)
- **Feature Parity:** `/home/mikekao/personal/subgen/docs/WORKLOGS/FEATURE_PARITY_CHECKLIST.md` section 2
- **Configuration:** Lines 114-118 (MONITOR, TRANSCRIBE_FOLDERS)
- **Key Functions:**
  - `NewFileHandler` class - Lines 2098-2128
  - `transcribe_existing()` - Lines 2131-2137
  - File stability checking - Lines 2110-2123

---

**Epic Owner:** TBD  
**Created:** 2026-02-16  
**Last Updated:** 2026-02-16
