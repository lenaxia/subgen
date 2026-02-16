# Story 06: Integration & Performance Testing

**Epic**: EPIC_07 - File System Monitoring & Automated Processing  
**Status**: In Progress  
**Assignee**: Delegation Agent  
**Effort**: 4-6 hours  
**Priority**: HIGH

---

## User Story

As a Subgen operator,
I want the file monitoring system integrated with the orchestrator and tested at scale,
So that I can confidently deploy it to production with predictable performance characteristics.

---

## Acceptance Criteria

- [ ] Monitor starts with orchestrator when MONITOR=true
- [ ] Multiple watch folders support (pipe-separated list)
- [ ] Integration with skip logic (EPIC_06)
- [ ] Performance test: 10,000 file scan in < 30s
- [ ] Memory efficiency: < 50MB overhead for monitoring
- [ ] CPU efficiency: < 5% CPU when idle
- [ ] Integration tests with real file operations
- [ ] Integration with orchestrator main (cmd/orchestrator/main.go)
- [ ] Configuration via environment variables
- [ ] Startup scanning optional (SCAN_ON_STARTUP)
- [ ] Work log created with performance benchmarks

---

## Technical Design

### Approach

Integrate the monitoring system (Stories 01-05) with the orchestrator main entry point and add comprehensive integration tests and performance benchmarks.

**Key Components:**
1. **Main Integration** - Start watcher when MONITOR=true
2. **Configuration Loading** - Parse TRANSCRIBE_FOLDERS from environment
3. **Startup Scanning** - Optional scan on startup
4. **Performance Benchmarks** - Measure scan time, memory, CPU
5. **Integration Tests** - End-to-end file detection flow

**Design Decisions:**
- Monitor runs in separate goroutine (non-blocking)
- Graceful shutdown via context cancellation
- Environment variable configuration (backwards compatible)
- Performance monitoring via Go benchmarks
- Integration tests use real filesystem operations

### Files to Create/Modify

**Create:**
- `orchestrator/internal/monitor/benchmark_test.go` - Performance benchmarks
- `orchestrator/internal/monitor/integration_test.go` - Full integration tests

**Modify:**
- `orchestrator/cmd/orchestrator/main.go` - Add monitor startup
- `orchestrator/internal/config/config.go` - Add MONITOR, TRANSCRIBE_FOLDERS, SCAN_ON_STARTUP

### Integration Architecture

```
orchestrator/cmd/orchestrator/main.go
    ├─> Load Config (MONITOR, TRANSCRIBE_FOLDERS, SCAN_ON_STARTUP)
    ├─> Initialize Components (queue, skip checker, scanner)
    ├─> IF MONITOR == true:
    │   ├─> Create FileWatcher with callback
    │   ├─> IF SCAN_ON_STARTUP == true:
    │   │   └─> scanner.ScanDirectory() for each folder
    │   └─> Start watcher.Watch() in goroutine
    └─> Start HTTP server

Callback Flow:
    FileWatcher.callback(filePath)
        ├─> Check skip logic (skip.Checker)
        ├─> If should_skip: log and return
        └─> Enqueue task for transcription
```

---

## Testing Strategy

### Integration Tests (integration_test.go)

**Full End-to-End Tests:**
1. `TestMonitor_Integration_FileDetection` - Create file, verify queued
2. `TestMonitor_Integration_MultipleFolders` - Watch multiple directories
3. `TestMonitor_Integration_StartupScan` - Verify startup scanning
4. `TestMonitor_Integration_SkipLogic` - Verify skip logic applied
5. `TestMonitor_Integration_RecursiveDirectory` - Deep directory detection
6. `TestMonitor_Integration_Stability` - File stability checking works

### Performance Benchmarks (benchmark_test.go)

**Benchmark Tests:**
1. `BenchmarkScanner_10000Files` - Scan 10,000 files
2. `BenchmarkWatcher_100Directories` - Watch 100 directories
3. `BenchmarkWatcher_1000FileEvents` - Handle 1,000 file creation events
4. `BenchmarkStability_Check` - File stability checking overhead

**Performance Targets:**
- 10,000 file scan: < 30 seconds
- 100 directory watch: < 1 second to initialize
- 1,000 file events: < 10 seconds to process callbacks
- Memory overhead: < 50MB for watcher + 1,000 directories
- CPU idle: < 5% when no events

### Manual Testing

```bash
# Test 1: Basic monitoring
export MONITOR=true
export TRANSCRIBE_FOLDERS=/tmp/test_movies
export SCAN_ON_STARTUP=true
go run cmd/orchestrator/main.go

# In another terminal:
touch /tmp/test_movies/test.mkv
# Expected: File detected and queued

# Test 2: Multiple folders
export TRANSCRIBE_FOLDERS=/tmp/movies|/tmp/tv|/tmp/anime
go run cmd/orchestrator/main.go
# Expected: All three folders watched

# Test 3: Performance test
mkdir -p /tmp/perf_test
for i in {1..10000}; do touch /tmp/perf_test/file_$i.mkv; done
export TRANSCRIBE_FOLDERS=/tmp/perf_test
export SCAN_ON_STARTUP=true
time go run cmd/orchestrator/main.go
# Expected: Scan completes in < 30s

# Test 4: Memory monitoring
go run cmd/orchestrator/main.go &
PID=$!
watch -n 1 "ps aux | grep $PID | grep -v grep"
# Expected: Memory < 100MB with monitoring active
```

---

## Implementation Details

### Configuration Changes (config/config.go)

```go
type Config struct {
    // ... existing fields ...
    
    // Monitoring configuration
    Monitor            bool          `env:"MONITOR" envDefault:"false"`
    TranscribeFolders  []string      `env:"TRANSCRIBE_FOLDERS" envSeparator:"|"`
    ScanOnStartup      bool          `env:"SCAN_ON_STARTUP" envDefault:"true"`
    
    // File stability (from STORY_02)
    StabilityChecks    int           `env:"FILE_STABILITY_CHECKS" envDefault:"3"`
    StabilityWait      time.Duration `env:"FILE_STABILITY_WAIT" envDefault:"2s"`
    StabilityTimeout   time.Duration `env:"FILE_STABILITY_TIMEOUT" envDefault:"60s"`
}
```

### Main Integration (cmd/orchestrator/main.go)

```go
func main() {
    // Load configuration
    cfg := config.Load()
    
    // Initialize logger
    log := logrus.New()
    
    // Initialize components
    queue := queue.NewTaskQueue()
    skipChecker := skip.NewChecker(cfg)
    scanner := monitor.NewScannerWithLogger(queue, skipChecker, log)
    
    // Start monitoring if enabled
    if cfg.Monitor {
        log.Info("File monitoring enabled")
        
        // Create file watcher
        monitorConfig := &monitor.Config{
            Enabled:          cfg.Monitor,
            Folders:          cfg.TranscribeFolders,
            StabilityChecks:  cfg.StabilityChecks,
            StabilityWait:    cfg.StabilityWait,
            StabilityTimeout: cfg.StabilityTimeout,
        }
        
        // Callback: check skip logic and enqueue
        callback := func(filePath string) {
            ctx := context.Background()
            result, err := skipChecker.Check(ctx, filePath)
            if err != nil {
                log.WithError(err).Warnf("Skip check failed for: %s", filePath)
                return
            }
            
            if result.ShouldSkip {
                log.WithFields(logrus.Fields{
                    "file":   filePath,
                    "reason": result.Reason,
                }).Debug("Skipping file")
                return
            }
            
            // Enqueue transcription task
            task := map[string]interface{}{
                "file_path": filePath,
                "priority":  2, // Standard priority
            }
            if err := queue.Enqueue(task); err != nil {
                log.WithError(err).Errorf("Failed to enqueue: %s", filePath)
            } else {
                log.WithField("file", filePath).Info("Queued for transcription")
            }
        }
        
        watcher, err := monitor.NewFileWatcher(
            cfg.TranscribeFolders,
            callback,
            monitorConfig,
            log,
        )
        if err != nil {
            log.WithError(err).Fatal("Failed to create file watcher")
        }
        
        // Startup scan
        if cfg.ScanOnStartup {
            log.Info("Performing startup scan...")
            for _, folder := range cfg.TranscribeFolders {
                result, err := scanner.ScanDirectory(folder, true, cfg.TargetLanguage)
                if err != nil {
                    log.WithError(err).Warnf("Startup scan failed: %s", folder)
                    continue
                }
                log.WithFields(logrus.Fields{
                    "folder":  folder,
                    "scanned": result.Scanned,
                    "queued":  result.Queued,
                    "skipped": result.Skipped,
                }).Info("Startup scan completed")
            }
        }
        
        // Start watcher in background
        ctx, cancel := context.WithCancel(context.Background())
        defer cancel()
        
        go func() {
            if err := watcher.Watch(ctx); err != nil && err != context.Canceled {
                log.WithError(err).Error("File watcher error")
            }
        }()
    }
    
    // Start HTTP server
    // ...
}
```

### Performance Benchmark (benchmark_test.go)

```go
package monitor_test

import (
    "context"
    "os"
    "path/filepath"
    "testing"
    
    "github.com/mccloud/subgen/orchestrator/internal/monitor"
    "github.com/sirupsen/logrus"
)

// BenchmarkScanner_10000Files measures scan performance on 10k files
func BenchmarkScanner_10000Files(b *testing.B) {
    // Create test directory with 10,000 files
    tmpDir, err := os.MkdirTemp("", "benchmark_*")
    if err != nil {
        b.Fatal(err)
    }
    defer os.RemoveAll(tmpDir)
    
    // Create 10,000 media files
    for i := 0; i < 10000; i++ {
        filePath := filepath.Join(tmpDir, fmt.Sprintf("file_%05d.mkv", i))
        if err := os.WriteFile(filePath, []byte("test"), 0644); err != nil {
            b.Fatal(err)
        }
    }
    
    // Create scanner
    scanner := monitor.NewScanner(nil, nil) // No queue/skip checker for benchmark
    
    b.ResetTimer()
    for i := 0; i < b.N; i++ {
        _, err := scanner.ScanDirectory(tmpDir, true, "en")
        if err != nil {
            b.Fatal(err)
        }
    }
}

// BenchmarkWatcher_100Directories measures watcher initialization time
func BenchmarkWatcher_100Directories(b *testing.B) {
    // Create 100 nested directories
    tmpDir, err := os.MkdirTemp("", "benchmark_*")
    if err != nil {
        b.Fatal(err)
    }
    defer os.RemoveAll(tmpDir)
    
    for i := 0; i < 100; i++ {
        subDir := filepath.Join(tmpDir, fmt.Sprintf("dir_%03d", i))
        if err := os.MkdirAll(subDir, 0755); err != nil {
            b.Fatal(err)
        }
    }
    
    log := logrus.New()
    log.SetLevel(logrus.ErrorLevel)
    config := monitor.DefaultConfig()
    
    callback := func(path string) {}
    
    b.ResetTimer()
    for i := 0; i < b.N; i++ {
        fw, err := monitor.NewFileWatcher([]string{tmpDir}, callback, config, log)
        if err != nil {
            b.Fatal(err)
        }
        
        ctx, cancel := context.WithCancel(context.Background())
        go fw.Watch(ctx)
        cancel() // Immediately cancel
    }
}

// BenchmarkStability_Check measures stability checking overhead
func BenchmarkStability_Check(b *testing.B) {
    tmpDir, err := os.MkdirTemp("", "benchmark_*")
    if err != nil {
        b.Fatal(err)
    }
    defer os.RemoveAll(tmpDir)
    
    testFile := filepath.Join(tmpDir, "test.mkv")
    if err := os.WriteFile(testFile, []byte("test content"), 0644); err != nil {
        b.Fatal(err)
    }
    
    log := logrus.New()
    log.SetLevel(logrus.ErrorLevel)
    config := monitor.DefaultConfig()
    config.StabilityChecks = 3
    config.StabilityWait = 10 * time.Millisecond // Faster for benchmark
    
    fw, _ := monitor.NewFileWatcher([]string{tmpDir}, nil, config, log)
    
    b.ResetTimer()
    for i := 0; i < b.N; i++ {
        fw.WaitForStability(testFile)
    }
}
```

### Integration Test (integration_test.go)

```go
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

// TestMonitor_Integration_FileDetection tests full end-to-end file detection
func TestMonitor_Integration_FileDetection(t *testing.T) {
    tmpDir, err := os.MkdirTemp("", "integration_*")
    require.NoError(t, err)
    defer os.RemoveAll(tmpDir)
    
    // Track queued files
    queuedFiles := make([]string, 0)
    callback := func(path string) {
        queuedFiles = append(queuedFiles, path)
    }
    
    // Create and start watcher
    log := logrus.New()
    log.SetLevel(logrus.ErrorLevel)
    config := monitor.DefaultConfig()
    config.StabilityChecks = 0 // Disable for faster test
    
    fw, err := monitor.NewFileWatcher([]string{tmpDir}, callback, config, log)
    require.NoError(t, err)
    
    ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
    defer cancel()
    
    go fw.Watch(ctx)
    time.Sleep(200 * time.Millisecond) // Wait for initialization
    
    // Create test files
    file1 := filepath.Join(tmpDir, "movie1.mkv")
    file2 := filepath.Join(tmpDir, "movie2.mp4")
    file3 := filepath.Join(tmpDir, "readme.txt") // Should be ignored
    
    require.NoError(t, os.WriteFile(file1, []byte("test"), 0644))
    time.Sleep(100 * time.Millisecond)
    require.NoError(t, os.WriteFile(file2, []byte("test"), 0644))
    time.Sleep(100 * time.Millisecond)
    require.NoError(t, os.WriteFile(file3, []byte("test"), 0644))
    time.Sleep(200 * time.Millisecond)
    
    // Verify only media files queued
    assert.Len(t, queuedFiles, 2, "Should detect 2 media files")
    assert.Contains(t, queuedFiles, file1, "Should detect mkv file")
    assert.Contains(t, queuedFiles, file2, "Should detect mp4 file")
    assert.NotContains(t, queuedFiles, file3, "Should ignore txt file")
}

// Additional integration tests...
```

---

## Definition of Done

- [ ] Story file created
- [ ] Configuration added to config.go (MONITOR, TRANSCRIBE_FOLDERS, SCAN_ON_STARTUP)
- [ ] Main integration implemented in cmd/orchestrator/main.go
- [ ] Callback integrates with skip logic
- [ ] Startup scanning implemented
- [ ] Integration tests created and passing
- [ ] Performance benchmarks created
- [ ] Benchmark results documented (10k files < 30s)
- [ ] Memory profiling completed (< 50MB overhead)
- [ ] Manual testing completed
- [ ] All tests passing
- [ ] Work log created with performance data

---

## Success Criteria

- Monitor starts when MONITOR=true
- Multiple folders supported (pipe-separated)
- Skip logic integration working
- Performance: 10,000 files scanned in < 30s
- Memory: < 50MB overhead
- CPU: < 5% when idle
- All integration tests passing
- Production-ready deployment

---

## References

- **Epic README**: `/home/mikekao/personal/subgen/docs/BACKLOG/EPIC_07/README.md` lines 338-384
- **Primary Doc**: `/home/mikekao/personal/subgen/README-LLM.md`
- **STORY_01-05**: Previous monitoring stories
- **EPIC_06**: Skip logic integration
- **Config Package**: `orchestrator/internal/config/`
- **Main Entry Point**: `orchestrator/cmd/orchestrator/main.go`

---

**Created**: 2026-02-16  
**Last Updated**: 2026-02-16
