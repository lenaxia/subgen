# Story 04: Memory Leak Validation

**Epic**: EPIC_03 - Integration & Testing  
**Status**: Not Started  
**Priority**: **CRITICAL** ⚠️  
**Estimated Effort**: 6-8 hours  
**Assignee**: TBD

---

## User Story

As a **production system operator**,  
I want **automated tests that verify no memory leaks exist after 1000+ transcriptions**,  
So that **I can confidently deploy the system without fear of OOM crashes**.

---

## Context

Memory leaks are the PRIMARY reason for rewriting Subgen. The legacy `subgen.py` has 3 confirmed memory leaks that cause OOM kills after weeks of operation:

1. **Unbounded `task_results` dictionary** (~10-100MB/year)
2. **Whisper model not reliably unloading** (1.5GB permanent allocation)
3. **BytesIO objects not closed** (accumulates during heavy load)

**Why This Is CRITICAL:**
- Memory leaks cause production outages (OOM kills)
- Kubernetes restarts pods when memory limits exceeded
- Data loss if transcription in progress during OOM
- User trust depends on system stability

**Current State:**
- Legacy subgen.py has confirmed leaks
- New architecture designed to prevent leaks (see [02_MEMORY_MANAGEMENT.md](../../DESIGN/02_MEMORY_MANAGEMENT.md))
- NO validation that leaks are actually fixed

**Target State:**
- Automated memory leak tests for both Go and Python
- 1000+ transcription stress test with < 20% memory growth
- Memory profiling integrated into tests
- Prometheus metrics track memory usage
- CI/CD fails if memory growth exceeds threshold

---

## Acceptance Criteria

- [ ] Go memory leak test: `test/memory/memory_test.go`
- [ ] Python memory leak test: `test/memory/test_memory.py`
- [ ] Test: 1000 transcriptions, Go orchestrator memory growth < 20%
- [ ] Test: 1000 transcriptions, Python worker memory growth < 20%
- [ ] Test: Model cleanup after idle timeout (30s)
- [ ] Test: Immediate model cleanup on memory threshold
- [ ] Test: No leaked file handles
- [ ] Test: No leaked gRPC connections
- [ ] Test: Goroutine count stable (Go)
- [ ] Memory profiling with pprof (Go)
- [ ] Memory profiling with memory_profiler (Python)
- [ ] Prometheus metrics validation
- [ ] CI/CD integration (fail build on leak)
- [ ] Documentation: Memory leak testing guide
- [ ] Work log created

---

## Technical Design

### Memory Leak Test Strategy

```
┌────────────────────────────────────────────────────────────────┐
│  Memory Leak Test Phases                                       │
├────────────────────────────────────────────────────────────────┤
│                                                                 │
│  Phase 1: Baseline Measurement                                │
│    ├─ Start services (orchestrator + worker)                  │
│    ├─ Wait for warmup (60s)                                   │
│    ├─ Measure initial memory                                  │
│    └─ Record baseline                                          │
│                                                                 │
│  Phase 2: Load Generation (1000 tasks)                        │
│    ├─ Send 1000 transcription requests                        │
│    ├─ Use same small audio file (fast)                        │
│    ├─ Measure memory every 100 tasks                          │
│    └─ Wait for all tasks to complete                          │
│                                                                 │
│  Phase 3: Cleanup & Stabilization                             │
│    ├─ Stop sending requests                                   │
│    ├─ Wait for queue to drain                                 │
│    ├─ Wait for model cleanup (30s)                            │
│    ├─ Force garbage collection (both Go and Python)           │
│    └─ Wait for memory to stabilize (60s)                      │
│                                                                 │
│  Phase 4: Final Measurement                                   │
│    ├─ Measure final memory                                    │
│    ├─ Calculate growth percentage                             │
│    ├─ Assert: growth < 20%                                    │
│    └─ Generate memory profile report                          │
│                                                                 │
└────────────────────────────────────────────────────────────────┘
```

### File Structure

```
test/
├── memory/
│   ├── memory_test.go                      # Go memory tests
│   ├── test_memory.py                      # Python memory tests
│   ├── memory_profiler.go                  # Go pprof utilities
│   └── memory_profiler.py                  # Python profiling utilities
├── scripts/
│   ├── run_memory_tests.sh                 # Test runner
│   └── analyze_memory_profile.sh           # Profile analyzer
└── reports/
    ├── go_memory_profile.pb.gz             # pprof output
    ├── python_memory_profile.dat           # memory_profiler output
    └── memory_test_report.md               # Human-readable report
```

---

## Implementation Steps

### Step 1: Go Memory Leak Test

**File: `/home/mikekao/personal/subgen/test/memory/memory_test.go`**

```go
package memory

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"runtime"
	"runtime/pprof"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/mccloud/subgen/orchestrator/internal/queue"
	"github.com/mccloud/subgen/orchestrator/internal/grpc_client"
	"github.com/mccloud/subgen/orchestrator/internal/config"
)

const (
	numTranscriptions = 1000
	testAudioFile     = "../testdata/short_audio.mp3"
	workerAddr        = "localhost:50051"
	maxMemoryGrowth   = 0.20 // 20%
)

// Test 1: Orchestrator Memory Leak Test (1000 Transcriptions)
func TestOrchestrator_NoMemoryLeak_1000Transcriptions(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping memory leak test in short mode")
	}

	// Create config
	cfg := &config.Config{
		Queue: config.QueueConfig{
			MaxSize: 2000,
		},
	}

	// Create queue
	q := queue.NewPriorityQueue(cfg.Queue.MaxSize)

	// Create gRPC client
	grpcClient := grpc_client.NewClient(
		5*time.Hour,
		5*time.Second,
		3,
		1*time.Second,
		nil,
		logrus.New(),
	)
	defer grpcClient.Close()

	// Warmup - let system stabilize
	t.Log("Warmup: processing 10 tasks...")
	for i := 0; i < 10; i++ {
		task := &queue.Task{
			ID:           fmt.Sprintf("warmup-%d", i),
			FilePath:     testAudioFile,
			TaskType:     "transcribe",
			Priority:     queue.PriorityNormal,
		}
		
		err := q.Push(task)
		require.NoError(t, err)

		// Process task
		poppedTask := q.Pop()
		require.NotNil(t, poppedTask)

		_, err = grpcClient.Transcribe(context.Background(), workerAddr, poppedTask)
		// Ignore errors for warmup
	}

	// Wait for warmup to settle
	time.Sleep(5 * time.Second)

	// Force GC before baseline
	runtime.GC()
	runtime.GC() // Run twice for thorough collection
	time.Sleep(2 * time.Second)

	// Phase 1: Baseline Measurement
	var m1 runtime.MemStats
	runtime.ReadMemStats(&m1)
	baselineAlloc := m1.Alloc
	baselineHeapAlloc := m1.HeapAlloc
	baselineHeapInuse := m1.HeapInuse
	baselineNumGC := m1.NumGC
	baselineGoroutines := runtime.NumGoroutine()

	t.Logf("Baseline Memory:")
	t.Logf("  Alloc:       %d MB", baselineAlloc/(1024*1024))
	t.Logf("  HeapAlloc:   %d MB", baselineHeapAlloc/(1024*1024))
	t.Logf("  HeapInuse:   %d MB", baselineHeapInuse/(1024*1024))
	t.Logf("  Goroutines:  %d", baselineGoroutines)

	// Start memory profiling
	heapProfile, err := os.Create("../../reports/go_memory_baseline.prof")
	require.NoError(t, err)
	pprof.WriteHeapProfile(heapProfile)
	heapProfile.Close()

	// Phase 2: Load Generation (1000 tasks)
	t.Logf("Processing %d transcriptions...", numTranscriptions)
	start := time.Now()

	for i := 0; i < numTranscriptions; i++ {
		task := &queue.Task{
			ID:           fmt.Sprintf("task-%d", i),
			FilePath:     testAudioFile,
			TaskType:     "transcribe",
			Priority:     queue.PriorityNormal,
		}

		err := q.Push(task)
		require.NoError(t, err)

		// Process immediately (simulates dispatcher)
		poppedTask := q.Pop()
		require.NotNil(t, poppedTask)

		_, err = grpcClient.Transcribe(context.Background(), workerAddr, poppedTask)
		// Ignore individual errors, we're testing memory

		// Log progress
		if (i+1)%100 == 0 {
			var m runtime.MemStats
			runtime.ReadMemStats(&m)
			currentAlloc := m.Alloc
			growth := float64(currentAlloc-baselineAlloc) / float64(baselineAlloc) * 100

			t.Logf("  Progress: %d/%d tasks, Memory: %d MB (%.1f%% growth)",
				i+1, numTranscriptions, currentAlloc/(1024*1024), growth)
		}
	}

	processingDuration := time.Since(start)
	t.Logf("All %d tasks processed in %.1fs", numTranscriptions, processingDuration.Seconds())

	// Phase 3: Cleanup & Stabilization
	t.Log("Waiting for queue to drain and model cleanup...")
	time.Sleep(35 * time.Second) // Model cleanup delay + buffer

	// Force multiple GC cycles
	t.Log("Forcing garbage collection...")
	runtime.GC()
	time.Sleep(1 * time.Second)
	runtime.GC()
	time.Sleep(1 * time.Second)
	runtime.GC()
	time.Sleep(2 * time.Second)

	// Phase 4: Final Measurement
	var m2 runtime.MemStats
	runtime.ReadMemStats(&m2)
	finalAlloc := m2.Alloc
	finalHeapAlloc := m2.HeapAlloc
	finalHeapInuse := m2.HeapInuse
	finalNumGC := m2.NumGC
	finalGoroutines := runtime.NumGoroutine()

	t.Logf("Final Memory:")
	t.Logf("  Alloc:       %d MB", finalAlloc/(1024*1024))
	t.Logf("  HeapAlloc:   %d MB", finalHeapAlloc/(1024*1024))
	t.Logf("  HeapInuse:   %d MB", finalHeapInuse/(1024*1024))
	t.Logf("  Goroutines:  %d", finalGoroutines)
	t.Logf("  GC Runs:     %d (delta: %d)", finalNumGC, finalNumGC-baselineNumGC)

	// Save final memory profile
	heapProfile, err = os.Create("../../reports/go_memory_final.prof")
	require.NoError(t, err)
	pprof.WriteHeapProfile(heapProfile)
	heapProfile.Close()

	// Calculate growth
	allocGrowth := float64(finalAlloc-baselineAlloc) / float64(baselineAlloc)
	heapAllocGrowth := float64(finalHeapAlloc-baselineHeapAlloc) / float64(baselineHeapAlloc)
	heapInuseGrowth := float64(finalHeapInuse-baselineHeapInuse) / float64(baselineHeapInuse)
	goroutineGrowth := finalGoroutines - baselineGoroutines

	t.Logf("Memory Growth:")
	t.Logf("  Alloc:       %.2f%%", allocGrowth*100)
	t.Logf("  HeapAlloc:   %.2f%%", heapAllocGrowth*100)
	t.Logf("  HeapInuse:   %.2f%%", heapInuseGrowth*100)
	t.Logf("  Goroutines:  %+d", goroutineGrowth)

	// Assertions - FAIL test if memory grows too much
	assert.Less(t, allocGrowth, maxMemoryGrowth,
		"Memory growth %.1f%% exceeds threshold %.1f%%",
		allocGrowth*100, maxMemoryGrowth*100)

	assert.Less(t, heapAllocGrowth, maxMemoryGrowth,
		"Heap growth %.1f%% exceeds threshold %.1f%%",
		heapAllocGrowth*100, maxMemoryGrowth*100)

	// Goroutines should be stable (allow ±5 variance)
	assert.InDelta(t, baselineGoroutines, finalGoroutines, 5,
		"Goroutine count changed by %d (possible goroutine leak)", goroutineGrowth)

	t.Log("✅ PASS: No memory leak detected in Go orchestrator")
}

// Test 2: Queue Memory Leak Test
func TestQueue_NoMemoryLeak_PushPop(t *testing.T) {
	q := queue.NewPriorityQueue(10000)

	// Baseline
	runtime.GC()
	time.Sleep(1 * time.Second)
	var m1 runtime.MemStats
	runtime.ReadMemStats(&m1)
	baseline := m1.Alloc

	// Push and pop 10000 tasks
	for i := 0; i < 10000; i++ {
		task := &queue.Task{
			ID:       fmt.Sprintf("task-%d", i),
			FilePath: "/test/file.mp3",
			Priority: queue.PriorityNormal,
		}
		q.Push(task)
		q.Pop() // Immediately pop
	}

	// GC and measure
	runtime.GC()
	time.Sleep(1 * time.Second)
	var m2 runtime.MemStats
	runtime.ReadMemStats(&m2)
	final := m2.Alloc

	growth := float64(final-baseline) / float64(baseline)

	t.Logf("Queue Memory Test:")
	t.Logf("  Baseline: %d MB", baseline/(1024*1024))
	t.Logf("  Final:    %d MB", final/(1024*1024))
	t.Logf("  Growth:   %.2f%%", growth*100)

	assert.Less(t, growth, 0.10, "Queue memory should grow < 10%%")
}

// Test 3: Connection Pool Memory Leak
func TestConnectionPool_NoLeak(t *testing.T) {
	pool := grpc_client.NewConnectionPool(10)
	defer pool.Close()

	ctx := context.Background()

	// Baseline
	runtime.GC()
	var m1 runtime.MemStats
	runtime.ReadMemStats(&m1)
	baseline := m1.Alloc

	// Get and put connections 1000 times
	for i := 0; i < 1000; i++ {
		conn, err := pool.Get(ctx, workerAddr)
		if err != nil {
			continue // Ignore connection errors
		}
		pool.Put(workerAddr, conn)
	}

	// Cleanup
	pool.Close()

	runtime.GC()
	time.Sleep(1 * time.Second)
	var m2 runtime.MemStats
	runtime.ReadMemStats(&m2)
	final := m2.Alloc

	growth := float64(final-baseline) / float64(baseline)

	t.Logf("Connection Pool Memory:")
	t.Logf("  Growth: %.2f%%", growth*100)

	assert.Less(t, growth, 0.05, "Connection pool should not leak")
}

// Test 4: Goroutine Leak Detection
func TestNoGoroutineLeak(t *testing.T) {
	baseline := runtime.NumGoroutine()

	// Create and close 100 clients
	for i := 0; i < 100; i++ {
		client := grpc_client.NewClient(
			5*time.Hour,
			5*time.Second,
			3,
			1*time.Second,
			nil,
			logrus.New(),
		)
		
		// Make a call
		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		_, _ = client.HealthCheck(ctx, workerAddr)
		cancel()

		// Close immediately
		client.Close()
	}

	// Wait for goroutines to finish
	time.Sleep(2 * time.Second)

	final := runtime.NumGoroutine()
	growth := final - baseline

	t.Logf("Goroutines: %d → %d (delta: %+d)", baseline, final, growth)

	// Allow small variance (±10)
	assert.InDelta(t, baseline, final, 10, "Goroutine count should be stable")
}
```

---

### Step 2: Python Memory Leak Test

**File: `/home/mikekao/personal/subgen/test/memory/test_memory.py`**

```python
"""
Python worker memory leak tests.

Tests that the worker doesn't leak memory after processing many tasks.
Uses psutil for memory measurement and memory_profiler for detailed analysis.
"""

import gc
import os
import sys
import time
import psutil
import pytest
from pathlib import Path

# Add worker to path
worker_root = Path(__file__).parent.parent.parent / "worker"
sys.path.insert(0, str(worker_root))

from src.grpc_server.service import TranscriptionServicer
from src.config.settings import WorkerSettings
from pb import transcription_pb2


# Constants
NUM_TRANSCRIPTIONS = 1000
TEST_AUDIO_FILE = str(Path(__file__).parent.parent / "testdata" / "short_audio.mp3")
MAX_MEMORY_GROWTH = 0.20  # 20%


class FakeContext:
    """Mock gRPC context for testing."""
    
    def abort(self, code, details):
        raise RuntimeError(f"gRPC abort: {code} - {details}")


@pytest.fixture(scope="module")
def config():
    """Create test configuration."""
    return WorkerSettings(
        grpc_port=50051,
        whisper_model="tiny",
        whisper_threads=2,
        memory_threshold_mb=3000,
        model_cleanup_delay=5,
        clear_vram_on_complete=False,
        device="cpu",
    )


@pytest.fixture(scope="module")
def servicer(config):
    """Create TranscriptionServicer for testing."""
    servicer = TranscriptionServicer(config)
    servicer.start_time = time.time()
    return servicer


def get_memory_mb():
    """Get current memory usage in MB."""
    process = psutil.Process()
    return process.memory_info().rss / (1024 * 1024)


def get_memory_info():
    """Get detailed memory info."""
    process = psutil.Process()
    mem = process.memory_info()
    return {
        "rss_mb": mem.rss / (1024 * 1024),
        "vms_mb": mem.vms / (1024 * 1024),
        "shared_mb": mem.shared / (1024 * 1024) if hasattr(mem, "shared") else 0,
    }


# Test 1: Worker Memory Leak Test (1000 Transcriptions)
@pytest.mark.slow
def test_worker_no_memory_leak_1000_transcriptions(servicer):
    """
    CRITICAL TEST: Verify no memory leaks after 1000 transcriptions.
    
    This test validates that the worker doesn't accumulate memory
    after processing many transcription tasks.
    """
    
    # Warmup - let system stabilize
    print("\n=== Warmup Phase ===")
    for i in range(10):
        request = transcription_pb2.TranscribeRequest(
            file_path=TEST_AUDIO_FILE,
            task_type="transcribe",
            options=transcription_pb2.TranscribeOptions(whisper_model="tiny"),
        )
        context = FakeContext()
        try:
            servicer.Transcribe(request, context)
        except Exception:
            pass  # Ignore errors during warmup

    # Wait for stabilization
    time.sleep(10)
    
    # Force GC
    gc.collect()
    gc.collect()
    time.sleep(2)

    # Phase 1: Baseline Measurement
    print("\n=== Baseline Measurement ===")
    baseline = get_memory_info()
    print(f"Baseline RSS:    {baseline['rss_mb']:.1f} MB")
    print(f"Baseline VMS:    {baseline['vms_mb']:.1f} MB")
    print(f"Baseline Shared: {baseline['shared_mb']:.1f} MB")

    # Phase 2: Load Generation (1000 tasks)
    print(f"\n=== Processing {NUM_TRANSCRIPTIONS} Transcriptions ===")
    start_time = time.time()
    memory_samples = []

    for i in range(NUM_TRANSCRIPTIONS):
        request = transcription_pb2.TranscribeRequest(
            file_path=TEST_AUDIO_FILE,
            task_type="transcribe",
            force_language="",  # Auto-detect
            options=transcription_pb2.TranscribeOptions(
                whisper_model="tiny",
                whisper_threads=2,
            ),
        )

        context = FakeContext()
        
        try:
            response = servicer.Transcribe(request, context)
            # Clean up generated subtitle
            if response.success and os.path.exists(response.subtitle_path):
                os.remove(response.subtitle_path)
        except Exception as e:
            print(f"Task {i} failed: {e}")

        # Sample memory every 100 tasks
        if (i + 1) % 100 == 0:
            current = get_memory_mb()
            memory_samples.append(current)
            growth = (current - baseline['rss_mb']) / baseline['rss_mb'] * 100
            print(f"  Progress: {i+1}/{NUM_TRANSCRIPTIONS}, Memory: {current:.1f} MB ({growth:+.1f}%)")

    processing_duration = time.time() - start_time
    print(f"\nProcessing completed in {processing_duration:.1f}s")

    # Phase 3: Cleanup & Stabilization
    print("\n=== Cleanup Phase ===")
    time.Sleep(10)  # Wait for model cleanup

    # Force aggressive GC
    print("Forcing garbage collection...")
    for _ in range(5):
        gc.collect()
        time.sleep(1)

    # Phase 4: Final Measurement
    print("\n=== Final Measurement ===")
    final = get_memory_info()
    print(f"Final RSS:    {final['rss_mb']:.1f} MB")
    print(f"Final VMS:    {final['vms_mb']:.1f} MB")
    print(f"Final Shared: {final['shared_mb']:.1f} MB")

    # Calculate growth
    rss_growth = (final['rss_mb'] - baseline['rss_mb']) / baseline['rss_mb']
    vms_growth = (final['vms_mb'] - baseline['vms_mb']) / baseline['vms_mb']

    print("\n=== Memory Growth Analysis ===")
    print(f"RSS Growth:  {rss_growth*100:+.2f}%")
    print(f"VMS Growth:  {vms_growth*100:+.2f}%")
    print(f"Growth Threshold: {MAX_MEMORY_GROWTH*100:.1f}%")

    # Plot memory over time
    if memory_samples:
        print("\nMemory Samples (every 100 tasks):")
        for idx, mem in enumerate(memory_samples):
            growth = (mem - baseline['rss_mb']) / baseline['rss_mb'] * 100
            bar = "█" * int(growth) if growth > 0 else ""
            print(f"  {(idx+1)*100:4d}: {mem:6.1f} MB {bar} ({growth:+.1f}%)")

    # CRITICAL ASSERTION - Test FAILS if memory grows > 20%
    assert rss_growth < MAX_MEMORY_GROWTH, (
        f"MEMORY LEAK DETECTED: Memory grew by {rss_growth*100:.1f}% "
        f"(threshold: {MAX_MEMORY_GROWTH*100:.1f}%)"
    )

    print("\n✅ PASS: No memory leak detected in Python worker")


# Test 2: Model Cleanup Test
@pytest.mark.slow
def test_model_cleanup_after_idle(servicer):
    """Verify model is unloaded after idle timeout."""
    
    # Process one transcription (loads model)
    request = transcription_pb2.TranscribeRequest(
        file_path=TEST_AUDIO_FILE,
        task_type="transcribe",
        options=transcription_pb2.TranscribeOptions(whisper_model="tiny"),
    )
    context = FakeContext()
    response = servicer.Transcribe(request, context)
    
    if response.success:
        os.remove(response.subtitle_path)

    # Check model is loaded
    health_req = transcription_pb2.HealthCheckRequest()
    health_resp = servicer.HealthCheck(health_req, context)
    
    model_loaded_before = health_resp.model_loaded
    memory_before = health_resp.memory_mb

    print(f"\nAfter transcription:")
    print(f"  Model Loaded: {model_loaded_before}")
    print(f"  Memory: {memory_before} MB")

    # Wait for cleanup (model_cleanup_delay + buffer)
    print("\nWaiting for model cleanup (35s)...")
    time.Sleep(35)

    # Check model is unloaded
    health_resp = servicer.HealthCheck(health_req, context)
    model_loaded_after = health_resp.model_loaded
    memory_after = health_resp.memory_mb

    print(f"\nAfter cleanup:")
    print(f"  Model Loaded: {model_loaded_after}")
    print(f"  Memory: {memory_after} MB")
    print(f"  Memory Released: {memory_before - memory_after} MB")

    # Assertions
    if model_loaded_before:
        # If model was loaded, it should be unloaded now
        assert not model_loaded_after, "Model should be unloaded after idle timeout"
        assert memory_after < memory_before, "Memory should decrease after model cleanup"

    print("✅ PASS: Model cleanup works correctly")


# Test 3: Memory Threshold Test
def test_memory_threshold_enforcement(servicer):
    """Verify health check reports unhealthy when memory exceeds threshold."""
    
    request = transcription_pb2.HealthCheckRequest()
    context = FakeContext()
    
    response = servicer.HealthCheck(request, context)

    # Check if memory is within threshold
    threshold = servicer.config.system.memory_threshold_mb
    current_memory = response.memory_mb

    print(f"\nMemory Threshold Test:")
    print(f"  Current Memory: {current_memory} MB")
    print(f"  Threshold:      {threshold} MB")
    print(f"  Status:         {response.status}")

    # Status should match memory threshold
    if current_memory > threshold:
        assert response.status == transcription_pb2.HealthCheckResponse.UNHEALTHY
        print("  Status: UNHEALTHY (as expected)")
    else:
        assert response.status == transcription_pb2.HealthCheckResponse.HEALTHY
        print("  Status: HEALTHY (as expected)")

    print("✅ PASS: Memory threshold enforced correctly")


# Test 4: File Handle Leak Test
def test_no_file_handle_leak(servicer):
    """Verify file handles are closed after transcription."""
    
    process = psutil.Process()
    baseline_fds = process.num_fds() if hasattr(process, "num_fds") else len(process.open_files())

    print(f"\nBaseline file descriptors: {baseline_fds}")

    # Process 100 transcriptions
    for i in range(100):
        request = transcription_pb2.TranscribeRequest(
            file_path=TEST_AUDIO_FILE,
            task_type="transcribe",
            options=transcription_pb2.TranscribeOptions(whisper_model="tiny"),
        )
        context = FakeContext()
        
        try:
            response = servicer.Transcribe(request, context)
            if response.success and os.path.exists(response.subtitle_path):
                os.remove(response.subtitle_path)
        except Exception:
            pass

    # Check file descriptors
    final_fds = process.num_fds() if hasattr(process, "num_fds") else len(process.open_files())
    growth = final_fds - baseline_fds

    print(f"Final file descriptors: {final_fds}")
    print(f"Growth: {growth:+d}")

    # Allow small growth (±5 descriptors)
    assert abs(growth) < 5, f"File descriptor leak detected: {growth:+d}"

    print("✅ PASS: No file handle leak")
```

---

### Step 3: Memory Profiling Script

**File: `/home/mikekao/personal/subgen/test/scripts/run_memory_tests.sh`**

```bash
#!/bin/bash
# Run comprehensive memory leak tests with profiling

set -e

echo "========================================="
echo "  Subgen Memory Leak Test Suite"
echo "========================================="

# Create reports directory
mkdir -p ../reports

# Start Docker Compose
echo ""
echo "Starting Docker Compose environment..."
cd ../
docker-compose -f docker-compose.integration.yml up -d

echo "Waiting for services to be healthy..."
sleep 30

# Check services are running
docker-compose -f docker-compose.integration.yml ps

echo ""
echo "========================================="
echo "  Phase 1: Go Orchestrator Memory Tests"
echo "========================================="

cd memory

# Run Go memory tests with profiling
go test -v -run TestOrchestrator_NoMemoryLeak \
  -timeout 30m \
  -memprofile=../reports/go_memory.prof \
  -cpuprofile=../reports/go_cpu.prof \
  2>&1 | tee ../reports/go_memory_test.log

# Analyze Go memory profile
if [ -f "../reports/go_memory.prof" ]; then
    echo ""
    echo "Generating Go memory profile report..."
    go tool pprof -text ../reports/go_memory.prof > ../reports/go_memory_analysis.txt
    go tool pprof -pdf ../reports/go_memory.prof > ../reports/go_memory_graph.pdf 2>/dev/null || true
    echo "Profile saved: ../reports/go_memory_analysis.txt"
fi

echo ""
echo "========================================="
echo "  Phase 2: Python Worker Memory Tests"
echo "========================================="

# Run Python memory tests
pytest test_memory.py -v -s \
  --maxfail=1 \
  2>&1 | tee ../reports/python_memory_test.log

echo ""
echo "========================================="
echo "  Phase 3: Memory Profile Analysis"
echo "========================================="

# Generate summary report
cat > ../reports/memory_test_report.md <<EOF
# Memory Leak Test Report

**Date:** $(date +%Y-%m-%d)
**Duration:** $(date +%s) seconds

---

## Go Orchestrator Results

\`\`\`
$(grep "Memory Growth:" ../reports/go_memory_test.log -A 5 || echo "No results")
\`\`\`

**Status:** $(grep "PASS: No memory leak" ../reports/go_memory_test.log > /dev/null && echo "✅ PASS" || echo "❌ FAIL")

---

## Python Worker Results

\`\`\`
$(grep "Memory Growth Analysis" ../reports/python_memory_test.log -A 5 || echo "No results")
\`\`\`

**Status:** $(grep "PASS: No memory leak" ../reports/python_memory_test.log > /dev/null && echo "✅ PASS" || echo "❌ FAIL")

---

## Memory Profiles

- Go Memory Profile: [go_memory.prof](./go_memory.prof)
- Go Memory Analysis: [go_memory_analysis.txt](./go_memory_analysis.txt)
- Python Test Log: [python_memory_test.log](./python_memory_test.log)

---

## Conclusion

$(if grep -q "PASS: No memory leak" ../reports/go_memory_test.log && \
     grep -q "PASS: No memory leak" ../reports/python_memory_test.log; then
    echo "✅ **ALL MEMORY TESTS PASSED** - No leaks detected"
  else
    echo "❌ **MEMORY TESTS FAILED** - Leaks detected, see logs above"
  fi)
EOF

cat ../reports/memory_test_report.md

echo ""
echo "========================================="
echo "  Test Complete!"
echo "========================================="
echo ""
echo "Reports saved in: test/reports/"
echo "  - memory_test_report.md"
echo "  - go_memory_test.log"
echo "  - python_memory_test.log"
echo "  - go_memory.prof (pprof format)"
echo ""

# Cleanup
echo "Stopping Docker Compose..."
cd ../
docker-compose -f docker-compose.integration.yml down

echo ""
echo "Done!"
```

**Usage:**
```bash
cd test/scripts
chmod +x run_memory_tests.sh
./run_memory_tests.sh
```

---

### Step 4: Prometheus Metrics Validation

**File: `/home/mikekao/personal/subgen/test/memory/metrics_test.go`**

```go
package memory

import (
	"fmt"
	"io"
	"net/http"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

const (
	metricsURL = "http://localhost:9090/metrics"
)

// Test: Validate Prometheus Metrics
func TestPrometheusMetrics_MemoryTracking(t *testing.T) {
	// Fetch metrics
	resp, err := http.Get(metricsURL)
	require.NoError(t, err)
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	require.NoError(t, err)

	metrics := string(body)

	// Verify memory metrics exist
	requiredMetrics := []string{
		"subgen_orchestrator_memory_bytes",
		"subgen_orchestrator_goroutines",
		"subgen_queue_size",
		"subgen_worker_memory_mb",
		"subgen_worker_jobs_total",
	}

	for _, metric := range requiredMetrics {
		assert.Contains(t, metrics, metric, "Metric %s should be exported", metric)
	}

	t.Log("✅ All memory metrics exported to Prometheus")
}

// Test: Memory Metrics Increase During Load
func TestMetrics_MemoryIncreaseDuringLoad(t *testing.T) {
	// Get baseline metrics
	baseline := fetchMetricValue(t, "subgen_worker_memory_mb")

	// Send some transcription requests
	// (Assumes orchestrator is running)
	for i := 0; i < 5; i++ {
		payload := fmt.Sprintf(`data={"Event":"library.new","Item":{"Path":"%s"}}`, testAudioFile)
		http.Post("http://localhost:9000/emby", "application/x-www-form-urlencoded", strings.NewReader(payload))
	}

	// Wait for processing
	time.Sleep(30 * time.Second)

	// Check metrics increased
	current := fetchMetricValue(t, "subgen_worker_memory_mb")

	t.Logf("Worker Memory: %d MB → %d MB", baseline, current)

	// Memory should increase during processing
	// (Or stay same if model was already loaded)
	assert.GreaterOrEqual(t, current, baseline, "Memory should not decrease unexpectedly")

	t.Log("✅ Memory metrics tracking correctly")
}

// fetchMetricValue parses a Prometheus metric value
func fetchMetricValue(t *testing.T, metricName string) int {
	resp, err := http.Get(metricsURL)
	require.NoError(t, err)
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	require.NoError(t, err)

	lines := strings.Split(string(body), "\n")
	for _, line := range lines {
		if strings.HasPrefix(line, metricName) && !strings.HasPrefix(line, "#") {
			// Parse value
			parts := strings.Fields(line)
			if len(parts) >= 2 {
				var value int
				fmt.Sscanf(parts[len(parts)-1], "%d", &value)
				return value
			}
		}
	}

	return 0
}
```

---

## Definition of Done

- [ ] All acceptance criteria met
- [ ] Go memory leak test passes (< 20% growth after 1000 tasks)
- [ ] Python memory leak test passes (< 20% growth after 1000 tasks)
- [ ] Model cleanup test passes
- [ ] Memory threshold test passes
- [ ] File handle leak test passes
- [ ] Goroutine leak test passes
- [ ] Memory profiling reports generated (pprof + memory_profiler)
- [ ] Prometheus metrics validated
- [ ] Test runner script created and documented
- [ ] Memory test report generated
- [ ] Work log created: `docs/WORKLOGS/NNNN_YYYY-MM-DD_EPIC_03_story_04.md`
- [ ] Code committed and pushed

---

## Validation Commands

```bash
# Run complete memory test suite
cd test/scripts
./run_memory_tests.sh

# Run Go memory tests only
cd test/memory
go test -v -run TestOrchestrator -timeout 30m

# Run Python memory tests only
cd test/memory
pytest test_memory.py -v -s --maxfail=1

# Analyze Go memory profile
go tool pprof -text ../reports/go_memory.prof
go tool pprof -web ../reports/go_memory.prof  # Opens browser

# View memory test report
cat ../reports/memory_test_report.md
```

---

## Dependencies

**Requires:**
- STORY_01 (gRPC Integration Tests) - Docker Compose setup
- STORY_03 (End-to-End Tests) - Complete pipeline working

**Blocks:**
- STORY_05 (Load Testing) - needs stable memory behavior
- EPIC_04 (K8s Deployment) - production deployment depends on leak validation

---

## Notes

### Memory Leak Detection Strategy

**Go Orchestrator:**
- Use `runtime.ReadMemStats()` for measurement
- Track `Alloc`, `HeapAlloc`, `HeapInuse`
- Monitor goroutine count
- Use pprof for detailed analysis

**Python Worker:**
- Use `psutil.Process().memory_info().rss` for measurement
- Track RSS (Resident Set Size)
- Force garbage collection between measurements
- Use `memory_profiler` for line-by-line analysis

### Why 1000 Transcriptions?

- Legacy leak: ~10MB per 100 transcriptions = 100MB at 1000
- Small leaks become visible at this scale
- Comparable to 1 month of production use (30 episodes/day)
- Fast enough to run in CI (~30-60 minutes)

### Memory Growth Threshold: 20%

**Why 20%?**
- Python GC may not immediately free memory
- OS memory caching behavior
- Model cache (legitimate memory use)
- Buffer for test variance

**Beyond 20%** = likely leak (investigate immediately)

### Profiling Tools

**Go pprof:**
```bash
# Heap profile
go tool pprof -text memory.prof
go tool pprof -web memory.prof

# Compare profiles
go tool pprof -base=baseline.prof final.prof
```

**Python memory_profiler:**
```python
from memory_profiler import profile

@profile
def transcribe_audio(...):
    # Function is profiled line-by-line
    pass
```

### CI/CD Integration

```yaml
# .github/workflows/memory-tests.yml
name: Memory Leak Tests

on:
  pull_request:
    branches: [main]
  schedule:
    - cron: '0 2 * * 0'  # Weekly on Sunday 2am

jobs:
  memory-tests:
    runs-on: ubuntu-latest
    timeout-minutes: 60
    
    steps:
      - uses: actions/checkout@v4
      
      - name: Start Docker Compose
        run: |
          cd test
          docker-compose -f docker-compose.integration.yml up -d
          sleep 30
      
      - name: Run memory tests
        run: |
          cd test/scripts
          ./run_memory_tests.sh
      
      - name: Upload reports
        uses: actions/upload-artifact@v4
        with:
          name: memory-test-reports
          path: test/reports/
      
      - name: Fail if memory leak detected
        run: |
          if ! grep -q "PASS: No memory leak" test/reports/*.log; then
            echo "❌ Memory leak detected!"
            exit 1
          fi
```

---

## References

- [02_MEMORY_MANAGEMENT.md](../../DESIGN/02_MEMORY_MANAGEMENT.md) - Memory leak prevention design
- Go pprof: https://go.dev/blog/pprof
- Python memory_profiler: https://pypi.org/project/memory-profiler/
- psutil: https://psutil.readthedocs.io/

---

**Created**: 2026-02-15  
**Last Updated**: 2026-02-15
