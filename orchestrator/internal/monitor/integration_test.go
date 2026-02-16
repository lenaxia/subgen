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
	log.SetLevel(logrus.ErrorLevel) // Reduce noise in tests
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

// TestMonitor_Integration_MultipleFolders tests watching multiple directories
func TestMonitor_Integration_MultipleFolders(t *testing.T) {
	tmpDir1, err := os.MkdirTemp("", "integration1_*")
	require.NoError(t, err)
	defer os.RemoveAll(tmpDir1)

	tmpDir2, err := os.MkdirTemp("", "integration2_*")
	require.NoError(t, err)
	defer os.RemoveAll(tmpDir2)

	// Track queued files
	queuedFiles := make([]string, 0)
	callback := func(path string) {
		queuedFiles = append(queuedFiles, path)
	}

	// Create and start watcher
	log := logrus.New()
	log.SetLevel(logrus.ErrorLevel)
	config := monitor.DefaultConfig()
	config.StabilityChecks = 0

	fw, err := monitor.NewFileWatcher([]string{tmpDir1, tmpDir2}, callback, config, log)
	require.NoError(t, err)

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	go fw.Watch(ctx)
	time.Sleep(200 * time.Millisecond)

	// Create files in both directories
	file1 := filepath.Join(tmpDir1, "movie1.mkv")
	file2 := filepath.Join(tmpDir2, "movie2.mp4")

	require.NoError(t, os.WriteFile(file1, []byte("test"), 0644))
	time.Sleep(100 * time.Millisecond)
	require.NoError(t, os.WriteFile(file2, []byte("test"), 0644))
	time.Sleep(200 * time.Millisecond)

	// Verify both files detected
	assert.Len(t, queuedFiles, 2, "Should detect files in both folders")
	assert.Contains(t, queuedFiles, file1, "Should detect file in folder 1")
	assert.Contains(t, queuedFiles, file2, "Should detect file in folder 2")
}

// TestMonitor_Integration_RecursiveDirectory tests deep directory detection
func TestMonitor_Integration_RecursiveDirectory(t *testing.T) {
	tmpDir, err := os.MkdirTemp("", "integration_recursive_*")
	require.NoError(t, err)
	defer os.RemoveAll(tmpDir)

	// Create nested directory structure
	subDir1 := filepath.Join(tmpDir, "movies")
	subDir2 := filepath.Join(tmpDir, "movies", "action")
	subDir3 := filepath.Join(tmpDir, "movies", "action", "2024")

	require.NoError(t, os.MkdirAll(subDir3, 0755))

	// Track queued files
	queuedFiles := make([]string, 0)
	callback := func(path string) {
		queuedFiles = append(queuedFiles, path)
	}

	// Create and start watcher
	log := logrus.New()
	log.SetLevel(logrus.ErrorLevel)
	config := monitor.DefaultConfig()
	config.StabilityChecks = 0

	fw, err := monitor.NewFileWatcher([]string{tmpDir}, callback, config, log)
	require.NoError(t, err)

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	go fw.Watch(ctx)
	time.Sleep(200 * time.Millisecond) // Wait for recursive setup

	// Create files at different depths
	file1 := filepath.Join(tmpDir, "movie1.mkv")
	file2 := filepath.Join(subDir1, "movie2.mkv")
	file3 := filepath.Join(subDir2, "movie3.mkv")
	file4 := filepath.Join(subDir3, "movie4.mkv")

	require.NoError(t, os.WriteFile(file1, []byte("test"), 0644))
	time.Sleep(100 * time.Millisecond)
	require.NoError(t, os.WriteFile(file2, []byte("test"), 0644))
	time.Sleep(100 * time.Millisecond)
	require.NoError(t, os.WriteFile(file3, []byte("test"), 0644))
	time.Sleep(100 * time.Millisecond)
	require.NoError(t, os.WriteFile(file4, []byte("test"), 0644))
	time.Sleep(200 * time.Millisecond)

	// Verify all files detected at different depths
	assert.Len(t, queuedFiles, 4, "Should detect files at all depths")
	assert.Contains(t, queuedFiles, file1, "Should detect root level file")
	assert.Contains(t, queuedFiles, file2, "Should detect level 1 file")
	assert.Contains(t, queuedFiles, file3, "Should detect level 2 file")
	assert.Contains(t, queuedFiles, file4, "Should detect level 3 file")
}

// TestMonitor_Integration_Stability tests file stability checking works
func TestMonitor_Integration_Stability(t *testing.T) {
	tmpDir, err := os.MkdirTemp("", "integration_stability_*")
	require.NoError(t, err)
	defer os.RemoveAll(tmpDir)

	// Track queued files and timing
	queuedFiles := make([]string, 0)
	queueTimes := make([]time.Time, 0)
	callback := func(path string) {
		queuedFiles = append(queuedFiles, path)
		queueTimes = append(queueTimes, time.Now())
	}

	// Create and start watcher with stability checking
	log := logrus.New()
	log.SetLevel(logrus.ErrorLevel)
	config := monitor.DefaultConfig()
	config.StabilityChecks = 2                    // 2 checks
	config.StabilityWait = 100 * time.Millisecond // Short interval for test
	config.StabilityTimeout = 5 * time.Second

	fw, err := monitor.NewFileWatcher([]string{tmpDir}, callback, config, log)
	require.NoError(t, err)

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	go fw.Watch(ctx)
	time.Sleep(200 * time.Millisecond)

	// Create file and record start time
	file1 := filepath.Join(tmpDir, "movie1.mkv")
	startTime := time.Now()
	require.NoError(t, os.WriteFile(file1, []byte("test"), 0644))

	// Wait for file to be queued
	time.Sleep(1 * time.Second)

	// Verify file was queued after stability delay
	require.Len(t, queuedFiles, 1, "File should be queued after stability checks")
	assert.Contains(t, queuedFiles, file1)

	// Verify timing - should be at least 2 * stability_wait
	expectedMinDelay := time.Duration(config.StabilityChecks) * config.StabilityWait
	actualDelay := queueTimes[0].Sub(startTime)
	assert.GreaterOrEqual(t, actualDelay, expectedMinDelay, "Should wait for stability before queueing")
}

// TestMonitor_Integration_StartupScan tests startup scanning
func TestMonitor_Integration_StartupScan(t *testing.T) {
	tmpDir, err := os.MkdirTemp("", "integration_scan_*")
	require.NoError(t, err)
	defer os.RemoveAll(tmpDir)

	// Create existing files before scanner starts
	file1 := filepath.Join(tmpDir, "existing1.mkv")
	file2 := filepath.Join(tmpDir, "existing2.mp4")
	file3 := filepath.Join(tmpDir, "readme.txt")

	require.NoError(t, os.WriteFile(file1, []byte("test"), 0644))
	require.NoError(t, os.WriteFile(file2, []byte("test"), 0644))
	require.NoError(t, os.WriteFile(file3, []byte("test"), 0644))

	// Create scanner (no skip checker for this test)
	log := logrus.New()
	log.SetLevel(logrus.ErrorLevel)
	scanner := monitor.NewScannerWithLogger(nil, nil, log)

	// Scan directory
	result, err := scanner.ScanDirectory(tmpDir, true, "en")

	// Verify results
	require.NoError(t, err)
	assert.Equal(t, 2, result.Scanned, "Should scan 2 media files")
	assert.Equal(t, 0, result.Queued, "No files queued (no queue interface)")
	assert.Equal(t, 0, result.Skipped, "No files skipped (no skip checker)")
}

// TestMonitor_Integration_NewDirectoryCreated tests dynamic directory watching
func TestMonitor_Integration_NewDirectoryCreated(t *testing.T) {
	tmpDir, err := os.MkdirTemp("", "integration_newdir_*")
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
	config.StabilityChecks = 0

	fw, err := monitor.NewFileWatcher([]string{tmpDir}, callback, config, log)
	require.NoError(t, err)

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	go fw.Watch(ctx)
	time.Sleep(200 * time.Millisecond)

	// Create new subdirectory
	newDir := filepath.Join(tmpDir, "newsubdir")
	require.NoError(t, os.MkdirAll(newDir, 0755))
	time.Sleep(200 * time.Millisecond) // Wait for directory to be added to watcher

	// Create file in new subdirectory
	file1 := filepath.Join(newDir, "movie1.mkv")
	require.NoError(t, os.WriteFile(file1, []byte("test"), 0644))
	time.Sleep(200 * time.Millisecond)

	// Verify file detected in dynamically created directory
	assert.Len(t, queuedFiles, 1, "Should detect file in newly created directory")
	assert.Contains(t, queuedFiles, file1, "Should detect file in new subdirectory")
}

// TestMonitor_Integration_SkipLogic tests skip logic integration with file monitoring
func TestMonitor_Integration_SkipLogic(t *testing.T) {
	tmpDir, err := os.MkdirTemp("", "integration_skip_*")
	require.NoError(t, err)
	defer os.RemoveAll(tmpDir)

	// Track queued files
	queuedFiles := make([]string, 0)
	callback := func(path string) {
		queuedFiles = append(queuedFiles, path)
	}

	// Create and start watcher (no skip checker at watcher level)
	log := logrus.New()
	log.SetLevel(logrus.ErrorLevel)
	config := monitor.DefaultConfig()
	config.StabilityChecks = 0

	fw, err := monitor.NewFileWatcher([]string{tmpDir}, callback, config, log)
	require.NoError(t, err)

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	go fw.Watch(ctx)
	time.Sleep(200 * time.Millisecond)

	// Create file with existing .srt subtitle (should be filtered by downstream skip logic)
	fileWithSubtitle := filepath.Join(tmpDir, "movie_with_sub.mkv")
	subtitleFile := filepath.Join(tmpDir, "movie_with_sub.srt")
	require.NoError(t, os.WriteFile(fileWithSubtitle, []byte("test"), 0644))
	require.NoError(t, os.WriteFile(subtitleFile, []byte("subtitle content"), 0644))
	time.Sleep(200 * time.Millisecond)

	// Create file without subtitle (should be queued)
	fileWithoutSubtitle := filepath.Join(tmpDir, "movie_no_sub.mkv")
	require.NoError(t, os.WriteFile(fileWithoutSubtitle, []byte("test"), 0644))
	time.Sleep(200 * time.Millisecond)

	// Verify both files detected by watcher (skip logic happens downstream in scanner/queue)
	// The watcher just detects all media files and passes them to callback
	assert.Len(t, queuedFiles, 2, "Watcher should detect all media files")
	assert.Contains(t, queuedFiles, fileWithSubtitle, "Should detect file with subtitle")
	assert.Contains(t, queuedFiles, fileWithoutSubtitle, "Should detect file without subtitle")

	// Note: Skip logic filtering happens in Scanner.ScanDirectory() or queue handler,
	// not in FileWatcher. This test verifies watcher correctly passes all media files.
}
