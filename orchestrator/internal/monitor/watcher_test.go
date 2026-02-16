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

// setupTestDir creates a temporary directory for testing
func setupTestDir(t *testing.T) string {
	t.Helper()
	dir, err := os.MkdirTemp("", "subgen_watcher_test_*")
	require.NoError(t, err)
	t.Cleanup(func() {
		os.RemoveAll(dir)
	})
	return dir
}

// setupLogger creates a test logger with minimal output
func setupLogger() *logrus.Logger {
	log := logrus.New()
	log.SetLevel(logrus.ErrorLevel) // Minimize test output
	return log
}

// TestFileWatcher_NewFileWatcher tests the constructor creates a valid watcher
func TestFileWatcher_NewFileWatcher(t *testing.T) {
	log := setupLogger()
	config := monitor.DefaultConfig()
	config.StabilityChecks = 0 // Disable for faster tests
	folders := []string{"/tmp/test1", "/tmp/test2"}
	callback := func(path string) {}

	watcher, err := monitor.NewFileWatcher(folders, callback, config, log)

	require.NoError(t, err)
	assert.NotNil(t, watcher)
}

// TestFileWatcher_NewFileWatcher_NilLogger tests constructor with nil logger
func TestFileWatcher_NewFileWatcher_NilLogger(t *testing.T) {
	config := monitor.DefaultConfig()
	folders := []string{"/tmp/test"}
	callback := func(path string) {}

	watcher, err := monitor.NewFileWatcher(folders, callback, config, nil)

	assert.Error(t, err)
	assert.Nil(t, watcher)
	assert.Contains(t, err.Error(), "logger cannot be nil")
}

// TestFileWatcher_Watch_CreateEvent tests that CREATE events trigger callback
func TestFileWatcher_Watch_CreateEvent(t *testing.T) {
	testDir := setupTestDir(t)
	log := setupLogger()
	config := monitor.DefaultConfig()
	config.StabilityChecks = 0 // Disable for faster tests

	var callbackMu sync.Mutex
	var callbackPath string
	callback := func(path string) {
		callbackMu.Lock()
		defer callbackMu.Unlock()
		callbackPath = path
	}

	watcher, err := monitor.NewFileWatcher([]string{testDir}, callback, config, log)
	require.NoError(t, err)

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	// Start watcher in goroutine
	watcherErrChan := make(chan error, 1)
	go func() {
		watcherErrChan <- watcher.Watch(ctx)
	}()

	// Give watcher time to start
	time.Sleep(100 * time.Millisecond)

	// Create a file
	testFile := filepath.Join(testDir, "test.mkv")
	err = os.WriteFile(testFile, []byte("test content"), 0644)
	require.NoError(t, err)

	// Wait for callback
	time.Sleep(200 * time.Millisecond)

	// Verify callback was called with correct path
	callbackMu.Lock()
	assert.Equal(t, testFile, callbackPath)
	callbackMu.Unlock()

	// Shutdown
	cancel()
	select {
	case err := <-watcherErrChan:
		assert.Equal(t, context.Canceled, err)
	case <-time.After(2 * time.Second):
		t.Fatal("Watcher did not shutdown in time")
	}
}

// TestFileWatcher_Watch_MultipleFiles tests handling multiple CREATE events
func TestFileWatcher_Watch_MultipleFiles(t *testing.T) {
	testDir := setupTestDir(t)
	log := setupLogger()
	config := monitor.DefaultConfig()
	config.StabilityChecks = 0 // Disable for faster tests

	var callbackMu sync.Mutex
	callbackPaths := []string{}
	callback := func(path string) {
		callbackMu.Lock()
		defer callbackMu.Unlock()
		callbackPaths = append(callbackPaths, path)
	}

	watcher, err := monitor.NewFileWatcher([]string{testDir}, callback, config, log)
	require.NoError(t, err)

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	// Start watcher
	go func() {
		watcher.Watch(ctx)
	}()

	time.Sleep(100 * time.Millisecond)

	// Create multiple files
	files := []string{"test1.mkv", "test2.mp4", "test3.avi"}
	for _, filename := range files {
		testFile := filepath.Join(testDir, filename)
		err = os.WriteFile(testFile, []byte("test content"), 0644)
		require.NoError(t, err)
		time.Sleep(50 * time.Millisecond) // Stagger creation
	}

	// Wait for callbacks
	time.Sleep(200 * time.Millisecond)

	// Verify all callbacks were called
	callbackMu.Lock()
	assert.Len(t, callbackPaths, len(files))
	callbackMu.Unlock()

	cancel()
	time.Sleep(100 * time.Millisecond)
}

// TestFileWatcher_Watch_GracefulShutdown tests context cancellation stops watcher
func TestFileWatcher_Watch_GracefulShutdown(t *testing.T) {
	testDir := setupTestDir(t)
	log := setupLogger()
	config := monitor.DefaultConfig()
	config.StabilityChecks = 0 // Disable for faster tests

	callback := func(path string) {}

	watcher, err := monitor.NewFileWatcher([]string{testDir}, callback, config, log)
	require.NoError(t, err)

	ctx, cancel := context.WithCancel(context.Background())

	watcherErrChan := make(chan error, 1)
	go func() {
		watcherErrChan <- watcher.Watch(ctx)
	}()

	time.Sleep(100 * time.Millisecond)

	// Cancel context
	cancel()

	// Verify watcher returns context.Canceled error
	select {
	case err := <-watcherErrChan:
		assert.Equal(t, context.Canceled, err)
	case <-time.After(2 * time.Second):
		t.Fatal("Watcher did not shutdown gracefully")
	}
}

// TestFileWatcher_Watch_MultipleFolders tests watching multiple directories
func TestFileWatcher_Watch_MultipleFolders(t *testing.T) {
	testDir1 := setupTestDir(t)
	testDir2 := setupTestDir(t)
	log := setupLogger()
	config := monitor.DefaultConfig()
	config.StabilityChecks = 0 // Disable for faster tests

	var callbackMu sync.Mutex
	callbackPaths := []string{}
	callback := func(path string) {
		callbackMu.Lock()
		defer callbackMu.Unlock()
		callbackPaths = append(callbackPaths, path)
	}

	watcher, err := monitor.NewFileWatcher([]string{testDir1, testDir2}, callback, config, log)
	require.NoError(t, err)

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	go func() {
		watcher.Watch(ctx)
	}()

	time.Sleep(100 * time.Millisecond)

	// Create file in first directory
	testFile1 := filepath.Join(testDir1, "test1.mkv")
	err = os.WriteFile(testFile1, []byte("test content"), 0644)
	require.NoError(t, err)

	// Create file in second directory
	testFile2 := filepath.Join(testDir2, "test2.mkv")
	err = os.WriteFile(testFile2, []byte("test content"), 0644)
	require.NoError(t, err)

	time.Sleep(200 * time.Millisecond)

	// Verify both callbacks were called
	callbackMu.Lock()
	assert.Len(t, callbackPaths, 2)
	callbackMu.Unlock()

	cancel()
	time.Sleep(100 * time.Millisecond)
}

// TestFileWatcher_Watch_InvalidFolder tests handling of non-existent folders
func TestFileWatcher_Watch_InvalidFolder(t *testing.T) {
	log := setupLogger()
	config := monitor.DefaultConfig()
	config.StabilityChecks = 0 // Disable for faster tests
	nonExistentFolder := "/tmp/does_not_exist_" + time.Now().Format("20060102150405")

	callback := func(path string) {}

	watcher, err := monitor.NewFileWatcher([]string{nonExistentFolder}, callback, config, log)
	require.NoError(t, err)

	ctx, cancel := context.WithTimeout(context.Background(), 1*time.Second)
	defer cancel()

	// Watch should not fail, but log warning
	err = watcher.Watch(ctx)
	// Should timeout or cancel, not crash
	assert.True(t, err == context.DeadlineExceeded || err == context.Canceled)
}

// TestFileWatcher_Watch_WriteEventIgnored tests WRITE events are ignored
func TestFileWatcher_Watch_WriteEventIgnored(t *testing.T) {
	testDir := setupTestDir(t)
	log := setupLogger()

	callbackCount := 0
	var callbackMu sync.Mutex
	callback := func(path string) {
		callbackMu.Lock()
		defer callbackMu.Unlock()
		callbackCount++
	}

	config := monitor.DefaultConfig()
	config.StabilityChecks = 0 // Disable for faster tests
	watcher, err := monitor.NewFileWatcher([]string{testDir}, callback, config, log)
	require.NoError(t, err)

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	go func() {
		watcher.Watch(ctx)
	}()

	time.Sleep(100 * time.Millisecond)

	// Create file (should trigger callback)
	testFile := filepath.Join(testDir, "test.mkv")
	err = os.WriteFile(testFile, []byte("test content"), 0644)
	require.NoError(t, err)

	time.Sleep(100 * time.Millisecond)

	// Verify CREATE callback was triggered
	callbackMu.Lock()
	initialCount := callbackCount
	callbackMu.Unlock()
	assert.Equal(t, 1, initialCount, "Expected CREATE event to trigger callback")

	// Modify file (WRITE event - should NOT trigger callback)
	err = os.WriteFile(testFile, []byte("modified content"), 0644)
	require.NoError(t, err)

	time.Sleep(100 * time.Millisecond)

	// Should still have only 1 callback (CREATE only, not WRITE)
	callbackMu.Lock()
	assert.Equal(t, 1, callbackCount, "Expected only CREATE event to trigger callback, not WRITE")
	callbackMu.Unlock()

	cancel()
	time.Sleep(100 * time.Millisecond)
}

// TestFileWatcher_Watch_ChmodEventIgnored tests CHMOD events are ignored
func TestFileWatcher_Watch_ChmodEventIgnored(t *testing.T) {
	testDir := setupTestDir(t)
	log := setupLogger()

	callbackCount := 0
	var callbackMu sync.Mutex
	callback := func(path string) {
		callbackMu.Lock()
		defer callbackMu.Unlock()
		callbackCount++
	}

	config := monitor.DefaultConfig()
	config.StabilityChecks = 0 // Disable for faster tests
	watcher, err := monitor.NewFileWatcher([]string{testDir}, callback, config, log)
	require.NoError(t, err)

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	go func() {
		watcher.Watch(ctx)
	}()

	time.Sleep(100 * time.Millisecond)

	// Create file (should trigger callback)
	testFile := filepath.Join(testDir, "test.mkv")
	err = os.WriteFile(testFile, []byte("test content"), 0644)
	require.NoError(t, err)

	time.Sleep(100 * time.Millisecond)

	// Verify CREATE callback was triggered
	callbackMu.Lock()
	initialCount := callbackCount
	callbackMu.Unlock()
	assert.Equal(t, 1, initialCount, "Expected CREATE event to trigger callback")

	// Change permissions (CHMOD event - should NOT trigger callback)
	err = os.Chmod(testFile, 0755)
	require.NoError(t, err)

	time.Sleep(100 * time.Millisecond)

	// Should still have only 1 callback (CREATE only, not CHMOD)
	callbackMu.Lock()
	assert.Equal(t, 1, callbackCount, "Expected only CREATE event to trigger callback, not CHMOD")
	callbackMu.Unlock()

	cancel()
	time.Sleep(100 * time.Millisecond)
}

// TestFileWatcher_Watch_RemoveEventIgnored tests REMOVE events are ignored
func TestFileWatcher_Watch_RemoveEventIgnored(t *testing.T) {
	testDir := setupTestDir(t)
	log := setupLogger()
	config := monitor.DefaultConfig()
	config.StabilityChecks = 0 // Disable for faster tests

	callbackCount := 0
	var callbackMu sync.Mutex
	callback := func(path string) {
		callbackMu.Lock()
		defer callbackMu.Unlock()
		callbackCount++
	}

	watcher, err := monitor.NewFileWatcher([]string{testDir}, callback, config, log)
	require.NoError(t, err)

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	go func() {
		watcher.Watch(ctx)
	}()

	time.Sleep(100 * time.Millisecond)

	// Create file (should trigger callback)
	testFile := filepath.Join(testDir, "test.mkv")
	err = os.WriteFile(testFile, []byte("test content"), 0644)
	require.NoError(t, err)

	time.Sleep(100 * time.Millisecond)

	// Verify CREATE callback was triggered
	callbackMu.Lock()
	initialCount := callbackCount
	callbackMu.Unlock()
	assert.Equal(t, 1, initialCount, "Expected CREATE event to trigger callback")

	// Remove file (REMOVE event - should NOT trigger callback)
	err = os.Remove(testFile)
	require.NoError(t, err)

	time.Sleep(100 * time.Millisecond)

	// Should still have only 1 callback (CREATE only, not REMOVE)
	callbackMu.Lock()
	assert.Equal(t, 1, callbackCount, "Expected only CREATE event to trigger callback, not REMOVE")
	callbackMu.Unlock()

	cancel()
	time.Sleep(100 * time.Millisecond)
}

// TestFileWatcher_Watch_NilCallback tests graceful handling of nil callback
func TestFileWatcher_Watch_NilCallback(t *testing.T) {
	testDir := setupTestDir(t)
	log := setupLogger()
	config := monitor.DefaultConfig()
	config.StabilityChecks = 0 // Disable for faster tests

	watcher, err := monitor.NewFileWatcher([]string{testDir}, nil, config, log)
	require.NoError(t, err)

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	go func() {
		watcher.Watch(ctx)
	}()

	time.Sleep(100 * time.Millisecond)

	// Create file (should not crash)
	testFile := filepath.Join(testDir, "test.mkv")
	err = os.WriteFile(testFile, []byte("test content"), 0644)
	require.NoError(t, err)

	time.Sleep(100 * time.Millisecond)

	// If we get here, watcher didn't panic
	cancel()
	time.Sleep(100 * time.Millisecond)
}

// TestFileWatcher_Watch_ContextCanceledBeforeStart tests pre-canceled context
func TestFileWatcher_Watch_ContextCanceledBeforeStart(t *testing.T) {
	testDir := setupTestDir(t)
	log := setupLogger()
	config := monitor.DefaultConfig()
	config.StabilityChecks = 0 // Disable for faster tests

	callback := func(path string) {}

	watcher, err := monitor.NewFileWatcher([]string{testDir}, callback, config, log)
	require.NoError(t, err)

	// Create already-canceled context
	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	// Watch should return immediately with context.Canceled
	err = watcher.Watch(ctx)
	assert.Equal(t, context.Canceled, err)
}

// TestFileWatcher_Watch_EmptyFolderList tests behavior with no folders
func TestFileWatcher_Watch_EmptyFolderList(t *testing.T) {
	log := setupLogger()
	config := monitor.DefaultConfig()
	config.StabilityChecks = 0 // Disable for faster tests
	callback := func(path string) {}

	watcher, err := monitor.NewFileWatcher([]string{}, callback, config, log)
	require.NoError(t, err)

	ctx, cancel := context.WithTimeout(context.Background(), 500*time.Millisecond)
	defer cancel()

	// Should not crash, just wait for context
	err = watcher.Watch(ctx)
	assert.Equal(t, context.DeadlineExceeded, err)
}

// TestFileWatcher_Watch_DuplicateFolder tests same folder added twice
func TestFileWatcher_Watch_DuplicateFolder(t *testing.T) {
	testDir := setupTestDir(t)
	log := setupLogger()
	config := monitor.DefaultConfig()
	config.StabilityChecks = 0 // Disable for faster tests

	callbackCount := 0
	var callbackMu sync.Mutex
	callback := func(path string) {
		callbackMu.Lock()
		defer callbackMu.Unlock()
		callbackCount++
	}

	// Add same folder twice
	watcher, err := monitor.NewFileWatcher([]string{testDir, testDir}, callback, config, log)
	require.NoError(t, err)

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	go func() {
		watcher.Watch(ctx)
	}()

	time.Sleep(100 * time.Millisecond)

	// Create file
	testFile := filepath.Join(testDir, "test.mkv")
	err = os.WriteFile(testFile, []byte("test content"), 0644)
	require.NoError(t, err)

	time.Sleep(200 * time.Millisecond)

	// fsnotify deduplicates, so should only get 1 callback
	callbackMu.Lock()
	assert.Equal(t, 1, callbackCount, "Expected only 1 callback despite duplicate folder")
	callbackMu.Unlock()

	cancel()
	time.Sleep(100 * time.Millisecond)
}
