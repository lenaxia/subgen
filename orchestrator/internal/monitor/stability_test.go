package monitor_test

import (
	"context"
	"os"
	"path/filepath"
	"sync"
	"testing"
	"time"

	"github.com/mccloud/subgen/orchestrator/internal/monitor"
	"github.com/sirupsen/logrus"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// setupStabilityTest creates a test directory and config
func setupStabilityTest(t *testing.T) (string, *monitor.Config, *logrus.Logger) {
	t.Helper()
	dir, err := os.MkdirTemp("", "subgen_stability_test_*")
	require.NoError(t, err)
	t.Cleanup(func() {
		os.RemoveAll(dir)
	})

	config := monitor.DefaultConfig()
	log := logrus.New()
	log.SetLevel(logrus.ErrorLevel) // Minimize test output

	return dir, config, log
}

// ========================================
// HAPPY PATH TESTS
// ========================================

// TestWaitForStability_StableFile tests that a stable file returns true immediately
func TestWaitForStability_StableFile(t *testing.T) {
	testDir, config, log := setupStabilityTest(t)

	// Create a stable file
	testFile := filepath.Join(testDir, "stable.mkv")
	err := os.WriteFile(testFile, []byte("stable content"), 0644)
	require.NoError(t, err)

	// Create watcher
	watcher, err := monitor.NewFileWatcher([]string{testDir}, nil, config, log)
	require.NoError(t, err)

	// Check stability
	start := time.Now()
	stable := watcher.WaitForStability(testFile)
	elapsed := time.Since(start)

	// Should return true
	assert.True(t, stable, "Stable file should return true")

	// Should take approximately 3 checks * 2 seconds = 6 seconds
	expectedDuration := time.Duration(config.StabilityChecks) * config.StabilityWait
	assert.GreaterOrEqual(t, elapsed, expectedDuration-time.Second, "Should wait for all checks")
	assert.Less(t, elapsed, expectedDuration+2*time.Second, "Should not wait too long")
}

// TestWaitForStability_StableAfterGrowth tests file that grows then stabilizes
func TestWaitForStability_StableAfterGrowth(t *testing.T) {
	testDir, config, log := setupStabilityTest(t)

	// Create initial file
	testFile := filepath.Join(testDir, "growing.mkv")
	err := os.WriteFile(testFile, []byte("initial"), 0644)
	require.NoError(t, err)

	// Start stability check in goroutine
	watcher, err := monitor.NewFileWatcher([]string{testDir}, nil, config, log)
	require.NoError(t, err)

	resultChan := make(chan bool, 1)
	go func() {
		resultChan <- watcher.WaitForStability(testFile)
	}()

	// Grow file during first check
	time.Sleep(1 * time.Second)
	err = os.WriteFile(testFile, []byte("initial + more data"), 0644)
	require.NoError(t, err)

	// Let it stabilize after that
	// Wait for result
	select {
	case stable := <-resultChan:
		assert.True(t, stable, "File should eventually stabilize")
	case <-time.After(15 * time.Second):
		t.Fatal("Timeout waiting for stability check")
	}
}

// TestWaitForStability_MultipleChecks tests that all checks must pass
func TestWaitForStability_MultipleChecks(t *testing.T) {
	testDir, config, log := setupStabilityTest(t)

	// Set to 5 checks
	config.StabilityChecks = 5
	config.StabilityWait = 500 * time.Millisecond

	// Create stable file
	testFile := filepath.Join(testDir, "multi.mkv")
	err := os.WriteFile(testFile, []byte("content"), 0644)
	require.NoError(t, err)

	// Create watcher
	watcher, err := monitor.NewFileWatcher([]string{testDir}, nil, config, log)
	require.NoError(t, err)

	// Check stability
	start := time.Now()
	stable := watcher.WaitForStability(testFile)
	elapsed := time.Since(start)

	assert.True(t, stable, "Should return true")

	// Should take approximately 5 checks * 500ms = 2.5 seconds
	expectedDuration := time.Duration(config.StabilityChecks) * config.StabilityWait
	assert.GreaterOrEqual(t, elapsed, expectedDuration-500*time.Millisecond)
	assert.Less(t, elapsed, expectedDuration+time.Second)
}

// TestWaitForStability_ConfigurableInterval tests different check intervals
func TestWaitForStability_ConfigurableInterval(t *testing.T) {
	testDir, config, log := setupStabilityTest(t)

	// Set shorter interval
	config.StabilityChecks = 3
	config.StabilityWait = 500 * time.Millisecond

	// Create stable file
	testFile := filepath.Join(testDir, "interval.mkv")
	err := os.WriteFile(testFile, []byte("content"), 0644)
	require.NoError(t, err)

	// Create watcher
	watcher, err := monitor.NewFileWatcher([]string{testDir}, nil, config, log)
	require.NoError(t, err)

	// Check stability
	start := time.Now()
	stable := watcher.WaitForStability(testFile)
	elapsed := time.Since(start)

	assert.True(t, stable)

	// Should take approximately 3 * 500ms = 1.5 seconds
	expectedDuration := 3 * 500 * time.Millisecond
	assert.GreaterOrEqual(t, elapsed, expectedDuration-200*time.Millisecond)
	assert.Less(t, elapsed, expectedDuration+time.Second)
}

// TestWaitForStability_DisabledChecks tests that checks=0 returns immediately
func TestWaitForStability_DisabledChecks(t *testing.T) {
	testDir, config, log := setupStabilityTest(t)

	// Disable stability checks
	config.StabilityChecks = 0

	// Create file
	testFile := filepath.Join(testDir, "disabled.mkv")
	err := os.WriteFile(testFile, []byte("content"), 0644)
	require.NoError(t, err)

	// Create watcher
	watcher, err := monitor.NewFileWatcher([]string{testDir}, nil, config, log)
	require.NoError(t, err)

	// Check stability
	start := time.Now()
	stable := watcher.WaitForStability(testFile)
	elapsed := time.Since(start)

	assert.True(t, stable, "Should return true immediately when disabled")
	assert.Less(t, elapsed, 100*time.Millisecond, "Should return instantly")
}

// ========================================
// UNHAPPY PATH TESTS
// ========================================

// TestWaitForStability_Timeout tests timeout when file never stabilizes
func TestWaitForStability_Timeout(t *testing.T) {
	testDir, config, log := setupStabilityTest(t)

	// Set short timeout
	config.StabilityTimeout = 2 * time.Second
	config.StabilityWait = 500 * time.Millisecond

	// Create file
	testFile := filepath.Join(testDir, "timeout.mkv")
	err := os.WriteFile(testFile, []byte("initial"), 0644)
	require.NoError(t, err)

	// Create watcher
	watcher, err := monitor.NewFileWatcher([]string{testDir}, nil, config, log)
	require.NoError(t, err)

	// Start stability check in goroutine
	resultChan := make(chan bool, 1)
	go func() {
		resultChan <- watcher.WaitForStability(testFile)
	}()

	// Keep growing file to prevent stabilization
	ticker := time.NewTicker(400 * time.Millisecond)
	defer ticker.Stop()

	go func() {
		counter := 0
		for range ticker.C {
			counter++
			_ = os.WriteFile(testFile, []byte("initial"+string(rune(counter))), 0644)
		}
	}()

	// Wait for result
	select {
	case stable := <-resultChan:
		assert.False(t, stable, "Should timeout and return false")
	case <-time.After(5 * time.Second):
		t.Fatal("Test timeout - stability check hung")
	}
}

// TestWaitForStability_FileDisappears tests file deleted during check
func TestWaitForStability_FileDisappears(t *testing.T) {
	testDir, config, log := setupStabilityTest(t)

	// Create file
	testFile := filepath.Join(testDir, "disappear.mkv")
	err := os.WriteFile(testFile, []byte("content"), 0644)
	require.NoError(t, err)

	// Create watcher
	watcher, err := monitor.NewFileWatcher([]string{testDir}, nil, config, log)
	require.NoError(t, err)

	// Start stability check in goroutine
	resultChan := make(chan bool, 1)
	go func() {
		resultChan <- watcher.WaitForStability(testFile)
	}()

	// Delete file after 1 second
	time.Sleep(1 * time.Second)
	err = os.Remove(testFile)
	require.NoError(t, err)

	// Wait for result
	select {
	case stable := <-resultChan:
		assert.False(t, stable, "Should return false when file disappears")
	case <-time.After(10 * time.Second):
		t.Fatal("Test timeout")
	}
}

// TestWaitForStability_FileNotFound tests non-existent file
func TestWaitForStability_FileNotFound(t *testing.T) {
	testDir, config, log := setupStabilityTest(t)

	// Don't create file
	testFile := filepath.Join(testDir, "nonexistent.mkv")

	// Create watcher
	watcher, err := monitor.NewFileWatcher([]string{testDir}, nil, config, log)
	require.NoError(t, err)

	// Check stability
	start := time.Now()
	stable := watcher.WaitForStability(testFile)
	elapsed := time.Since(start)

	assert.False(t, stable, "Non-existent file should return false")
	assert.Less(t, elapsed, 500*time.Millisecond, "Should fail fast")
}

// TestWaitForStability_PermissionDenied tests file with no read permissions
func TestWaitForStability_PermissionDenied(t *testing.T) {
	if os.Getuid() == 0 {
		t.Skip("Test requires non-root user")
	}

	testDir, config, log := setupStabilityTest(t)

	// Create file with no permissions (0000)
	testFile := filepath.Join(testDir, "noperm.mkv")
	err := os.WriteFile(testFile, []byte("content"), 0644)
	require.NoError(t, err)

	// Remove all permissions
	err = os.Chmod(testFile, 0000)
	require.NoError(t, err)

	// Create watcher
	watcher, err := monitor.NewFileWatcher([]string{testDir}, nil, config, log)
	require.NoError(t, err)

	// Check stability
	// Note: os.Stat() may still succeed with 0000 permissions on some systems
	// because Stat only reads metadata, not file contents.
	// This test may be flaky on different systems/filesystems.
	stable := watcher.WaitForStability(testFile)

	// On most systems, stat should still work even with 0000 permissions
	// So this test might actually return true. Let's just verify it doesn't panic.
	_ = stable
	t.Logf("Stability result for 0000 permissions: %v", stable)

	// Cleanup: restore permissions so cleanup can delete the file
	os.Chmod(testFile, 0644)
}

// TestWaitForStability_ContinuousGrowth tests file that keeps growing
func TestWaitForStability_ContinuousGrowth(t *testing.T) {
	testDir, config, log := setupStabilityTest(t)

	// Set shorter timeout
	config.StabilityTimeout = 3 * time.Second
	config.StabilityWait = 500 * time.Millisecond

	// Create file
	testFile := filepath.Join(testDir, "growing.mkv")
	err := os.WriteFile(testFile, []byte("start"), 0644)
	require.NoError(t, err)

	// Create watcher
	watcher, err := monitor.NewFileWatcher([]string{testDir}, nil, config, log)
	require.NoError(t, err)

	// Start stability check in goroutine
	resultChan := make(chan bool, 1)
	go func() {
		resultChan <- watcher.WaitForStability(testFile)
	}()

	// Keep appending to file
	ticker := time.NewTicker(300 * time.Millisecond)
	defer ticker.Stop()

	stopGrowing := make(chan struct{})
	go func() {
		for {
			select {
			case <-ticker.C:
				f, _ := os.OpenFile(testFile, os.O_APPEND|os.O_WRONLY, 0644)
				if f != nil {
					f.WriteString("more data ")
					f.Close()
				}
			case <-stopGrowing:
				return
			}
		}
	}()
	defer close(stopGrowing)

	// Wait for result
	select {
	case stable := <-resultChan:
		assert.False(t, stable, "Continuously growing file should timeout")
	case <-time.After(6 * time.Second):
		t.Fatal("Test timeout")
	}
}

// ========================================
// INTEGRATION TESTS
// ========================================

// TestFileWatcher_StabilityIntegration tests full watcher with stability
func TestFileWatcher_StabilityIntegration(t *testing.T) {
	testDir, config, log := setupStabilityTest(t)

	// Configure shorter intervals for faster test
	config.StabilityChecks = 2
	config.StabilityWait = 500 * time.Millisecond

	var callbackMu sync.Mutex
	callbackPaths := []string{}
	callback := func(path string) {
		callbackMu.Lock()
		defer callbackMu.Unlock()
		callbackPaths = append(callbackPaths, path)
	}

	// Create watcher
	watcher, err := monitor.NewFileWatcher([]string{testDir}, callback, config, log)
	require.NoError(t, err)

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	// Start watcher
	go func() {
		_ = watcher.Watch(ctx)
	}()

	time.Sleep(100 * time.Millisecond)

	// Create stable file
	testFile := filepath.Join(testDir, "stable.mkv")
	err = os.WriteFile(testFile, []byte("content"), 0644)
	require.NoError(t, err)

	// Wait for stability check + callback
	time.Sleep(2 * time.Second)

	// Verify callback was called
	callbackMu.Lock()
	assert.Len(t, callbackPaths, 1, "Callback should be called once file is stable")
	if len(callbackPaths) > 0 {
		assert.Equal(t, testFile, callbackPaths[0])
	}
	callbackMu.Unlock()

	cancel()
	time.Sleep(100 * time.Millisecond)
}

// TestFileWatcher_StabilityDisabled tests callback invoked immediately when disabled
func TestFileWatcher_StabilityDisabled(t *testing.T) {
	testDir, config, log := setupStabilityTest(t)

	// Disable stability checks
	config.StabilityChecks = 0

	callbackCalled := false
	var callbackMu sync.Mutex
	callback := func(path string) {
		callbackMu.Lock()
		defer callbackMu.Unlock()
		callbackCalled = true
	}

	// Create watcher
	watcher, err := monitor.NewFileWatcher([]string{testDir}, callback, config, log)
	require.NoError(t, err)

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	// Start watcher
	go func() {
		_ = watcher.Watch(ctx)
	}()

	time.Sleep(100 * time.Millisecond)

	// Create file
	testFile := filepath.Join(testDir, "instant.mkv")
	err = os.WriteFile(testFile, []byte("content"), 0644)
	require.NoError(t, err)

	// Callback should be called almost immediately
	time.Sleep(200 * time.Millisecond)

	callbackMu.Lock()
	assert.True(t, callbackCalled, "Callback should be invoked immediately when checks disabled")
	callbackMu.Unlock()

	cancel()
	time.Sleep(100 * time.Millisecond)
}

// TestFileWatcher_StabilityTimeout tests callback NOT invoked on timeout
func TestFileWatcher_StabilityTimeout(t *testing.T) {
	testDir, config, log := setupStabilityTest(t)

	// Configure fast timeout
	config.StabilityTimeout = 1 * time.Second
	config.StabilityWait = 300 * time.Millisecond

	callbackCalled := false
	var callbackMu sync.Mutex
	callback := func(path string) {
		callbackMu.Lock()
		defer callbackMu.Unlock()
		callbackCalled = true
	}

	// Create watcher
	watcher, err := monitor.NewFileWatcher([]string{testDir}, callback, config, log)
	require.NoError(t, err)

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	// Start watcher
	go func() {
		_ = watcher.Watch(ctx)
	}()

	time.Sleep(100 * time.Millisecond)

	// Create file
	testFile := filepath.Join(testDir, "timeout.mkv")
	err = os.WriteFile(testFile, []byte("initial"), 0644)
	require.NoError(t, err)

	// Keep modifying file to prevent stability
	ticker := time.NewTicker(200 * time.Millisecond)
	defer ticker.Stop()

	stopGrowing := make(chan struct{})
	go func() {
		counter := 0
		for {
			select {
			case <-ticker.C:
				counter++
				os.WriteFile(testFile, []byte("initial"+string(rune(counter))), 0644)
			case <-stopGrowing:
				return
			}
		}
	}()

	// Wait past timeout
	time.Sleep(3 * time.Second)
	close(stopGrowing)

	// Callback should NOT be called due to timeout
	callbackMu.Lock()
	assert.False(t, callbackCalled, "Callback should not be invoked when stability times out")
	callbackMu.Unlock()

	cancel()
	time.Sleep(100 * time.Millisecond)
}

// ========================================
// EDGE CASES
// ========================================

// TestWaitForStability_ZeroSizeFile tests empty file is considered stable
func TestWaitForStability_ZeroSizeFile(t *testing.T) {
	testDir, config, log := setupStabilityTest(t)

	// Create empty file
	testFile := filepath.Join(testDir, "empty.mkv")
	err := os.WriteFile(testFile, []byte{}, 0644)
	require.NoError(t, err)

	// Create watcher
	watcher, err := monitor.NewFileWatcher([]string{testDir}, nil, config, log)
	require.NoError(t, err)

	// Check stability
	stable := watcher.WaitForStability(testFile)

	assert.True(t, stable, "Zero-size file should be stable")
}

// TestWaitForStability_VeryLargeFile tests large file size handling
func TestWaitForStability_VeryLargeFile(t *testing.T) {
	testDir, config, log := setupStabilityTest(t)

	// Fast config for test speed
	config.StabilityChecks = 2
	config.StabilityWait = 300 * time.Millisecond

	// Create large file (simulate size > 2GB by creating smaller file)
	// In real scenario this would be a multi-GB file
	testFile := filepath.Join(testDir, "large.mkv")

	// Write 10MB file (enough to test)
	largeData := make([]byte, 10*1024*1024)
	for i := range largeData {
		largeData[i] = byte(i % 256)
	}
	err := os.WriteFile(testFile, largeData, 0644)
	require.NoError(t, err)

	// Create watcher
	watcher, err := monitor.NewFileWatcher([]string{testDir}, nil, config, log)
	require.NoError(t, err)

	// Check stability
	stable := watcher.WaitForStability(testFile)

	assert.True(t, stable, "Large file should stabilize")

	// Verify file size is correct
	stat, err := os.Stat(testFile)
	require.NoError(t, err)
	assert.Equal(t, int64(10*1024*1024), stat.Size())
}

// TestWaitForStability_SimultaneousFiles tests multiple files checked independently
func TestWaitForStability_SimultaneousFiles(t *testing.T) {
	testDir, config, log := setupStabilityTest(t)

	// Fast config
	config.StabilityChecks = 2
	config.StabilityWait = 300 * time.Millisecond

	// Create watcher
	watcher, err := monitor.NewFileWatcher([]string{testDir}, nil, config, log)
	require.NoError(t, err)

	// Create multiple files
	file1 := filepath.Join(testDir, "file1.mkv")
	file2 := filepath.Join(testDir, "file2.mkv")
	file3 := filepath.Join(testDir, "file3.mkv")

	err = os.WriteFile(file1, []byte("content1"), 0644)
	require.NoError(t, err)
	err = os.WriteFile(file2, []byte("content2"), 0644)
	require.NoError(t, err)
	err = os.WriteFile(file3, []byte("content3"), 0644)
	require.NoError(t, err)

	// Check stability simultaneously
	var wg sync.WaitGroup
	results := make([]bool, 3)

	wg.Add(3)
	go func() {
		defer wg.Done()
		results[0] = watcher.WaitForStability(file1)
	}()
	go func() {
		defer wg.Done()
		results[1] = watcher.WaitForStability(file2)
	}()
	go func() {
		defer wg.Done()
		results[2] = watcher.WaitForStability(file3)
	}()

	wg.Wait()

	// All should be stable
	assert.True(t, results[0], "File 1 should be stable")
	assert.True(t, results[1], "File 2 should be stable")
	assert.True(t, results[2], "File 3 should be stable")
}
