package monitor

import (
	"context"
	"os"
	"path/filepath"
	"testing"

	"github.com/mccloud/subgen/orchestrator/internal/skip"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// MockQueue implements QueueInterface for testing
type MockQueue struct {
	enqueuedTasks []interface{}
}

func (mq *MockQueue) Enqueue(task interface{}) error {
	mq.enqueuedTasks = append(mq.enqueuedTasks, task)
	return nil
}

func (mq *MockQueue) Count() int {
	return len(mq.enqueuedTasks)
}

// MockSkipChecker implements skip.Checker for testing
type MockSkipChecker struct {
	shouldSkip bool
	skipReason skip.SkipReason
}

func (msc *MockSkipChecker) Check(ctx context.Context, filePath string) (*skip.CheckResult, error) {
	return &skip.CheckResult{
		ShouldSkip: msc.shouldSkip,
		Reason:     msc.skipReason,
		Details:    "mock skip check",
	}, nil
}

func (msc *MockSkipChecker) GetConfig() *skip.Config {
	return nil
}

// Helper function to create test directory structure
func setupTestDir(t *testing.T) string {
	tmpDir, err := os.MkdirTemp("", "scanner_test")
	require.NoError(t, err)
	return tmpDir
}

func cleanupTestDir(t *testing.T, dir string) {
	err := os.RemoveAll(dir)
	require.NoError(t, err)
}

func TestNewScanner(t *testing.T) {
	queue := &MockQueue{}
	skipChecker := &MockSkipChecker{}

	scanner := NewScanner(queue, skipChecker)
	assert.NotNil(t, scanner, "Scanner should not be nil")
}

func TestScanner_ScanDirectory_SingleFile(t *testing.T) {
	// Setup
	testDir := setupTestDir(t)
	defer cleanupTestDir(t, testDir)

	// Create a single media file
	mediaFile := filepath.Join(testDir, "movie.mkv")
	err := os.WriteFile(mediaFile, []byte("test"), 0644)
	require.NoError(t, err)

	queue := &MockQueue{}
	skipChecker := &MockSkipChecker{shouldSkip: false}
	scanner := NewScanner(queue, skipChecker)

	// Execute
	result, err := scanner.ScanDirectory(testDir, false, "")

	// Verify
	require.NoError(t, err)
	assert.Equal(t, 1, result.Scanned, "Should scan 1 file")
	assert.Equal(t, 1, result.Queued, "Should queue 1 file")
	assert.Equal(t, 0, result.Skipped, "Should skip 0 files")
}

func TestScanner_ScanDirectory_MultipleFiles(t *testing.T) {
	// Setup
	testDir := setupTestDir(t)
	defer cleanupTestDir(t, testDir)

	// Create multiple media files
	mediaFiles := []string{"movie1.mkv", "movie2.mp4", "movie3.avi"}
	for _, filename := range mediaFiles {
		filePath := filepath.Join(testDir, filename)
		err := os.WriteFile(filePath, []byte("test"), 0644)
		require.NoError(t, err)
	}

	queue := &MockQueue{}
	skipChecker := &MockSkipChecker{shouldSkip: false}
	scanner := NewScanner(queue, skipChecker)

	// Execute
	result, err := scanner.ScanDirectory(testDir, false, "")

	// Verify
	require.NoError(t, err)
	assert.Equal(t, 3, result.Scanned, "Should scan 3 files")
	assert.Equal(t, 3, result.Queued, "Should queue 3 files")
	assert.Equal(t, 0, result.Skipped, "Should skip 0 files")
}

func TestScanner_ScanDirectory_Recursive(t *testing.T) {
	// Setup
	testDir := setupTestDir(t)
	defer cleanupTestDir(t, testDir)

	// Create directory structure with files
	// /testDir/movie1.mkv
	// /testDir/subdir/movie2.mp4
	// /testDir/subdir/nested/movie3.avi
	err := os.WriteFile(filepath.Join(testDir, "movie1.mkv"), []byte("test"), 0644)
	require.NoError(t, err)

	subdir := filepath.Join(testDir, "subdir")
	err = os.Mkdir(subdir, 0755)
	require.NoError(t, err)
	err = os.WriteFile(filepath.Join(subdir, "movie2.mp4"), []byte("test"), 0644)
	require.NoError(t, err)

	nested := filepath.Join(subdir, "nested")
	err = os.Mkdir(nested, 0755)
	require.NoError(t, err)
	err = os.WriteFile(filepath.Join(nested, "movie3.avi"), []byte("test"), 0644)
	require.NoError(t, err)

	queue := &MockQueue{}
	skipChecker := &MockSkipChecker{shouldSkip: false}
	scanner := NewScanner(queue, skipChecker)

	// Execute - non-recursive
	result, err := scanner.ScanDirectory(testDir, false, "")
	require.NoError(t, err)
	assert.Equal(t, 1, result.Scanned, "Non-recursive should scan only top-level")

	// Execute - recursive
	result, err = scanner.ScanDirectory(testDir, true, "")
	require.NoError(t, err)
	assert.Equal(t, 3, result.Scanned, "Recursive should scan all subdirectories")
	assert.Equal(t, 3, result.Queued, "Should queue all files")
}

func TestScanner_ScanDirectory_FilterNonMediaFiles(t *testing.T) {
	// Setup
	testDir := setupTestDir(t)
	defer cleanupTestDir(t, testDir)

	// Create media and non-media files
	files := map[string]bool{
		"movie.mkv":    true,  // media
		"movie.mp4":    true,  // media
		"audio.mp3":    true,  // media (audio)
		"readme.txt":   false, // not media
		"image.jpg":    false, // not media
		"subtitle.srt": false, // not media
	}

	for filename := range files {
		filePath := filepath.Join(testDir, filename)
		err := os.WriteFile(filePath, []byte("test"), 0644)
		require.NoError(t, err)
	}

	queue := &MockQueue{}
	skipChecker := &MockSkipChecker{shouldSkip: false}
	scanner := NewScanner(queue, skipChecker)

	// Execute
	result, err := scanner.ScanDirectory(testDir, false, "")

	// Verify - should only scan media files
	require.NoError(t, err)
	assert.Equal(t, 3, result.Scanned, "Should scan only 3 media files")
	assert.Equal(t, 3, result.Queued, "Should queue 3 media files")
}

func TestScanner_ScanDirectory_SkipLogicIntegration(t *testing.T) {
	// Setup
	testDir := setupTestDir(t)
	defer cleanupTestDir(t, testDir)

	// Create media files
	for i := 1; i <= 5; i++ {
		filename := filepath.Join(testDir, filepath.Base(testDir)+".mkv")
		err := os.WriteFile(filename, []byte("test"), 0644)
		require.NoError(t, err)
	}

	queue := &MockQueue{}
	// Skip checker that skips all files
	skipChecker := &MockSkipChecker{
		shouldSkip: true,
		skipReason: skip.ReasonSubtitleExists,
	}
	scanner := NewScanner(queue, skipChecker)

	// Execute
	result, err := scanner.ScanDirectory(testDir, false, "")

	// Verify
	require.NoError(t, err)
	assert.Greater(t, result.Scanned, 0, "Should scan files")
	assert.Equal(t, 0, result.Queued, "Should queue 0 files (all skipped)")
	assert.Equal(t, result.Scanned, result.Skipped, "All scanned files should be skipped")
}

func TestScanner_ScanDirectory_DirectoryNotFound(t *testing.T) {
	queue := &MockQueue{}
	skipChecker := &MockSkipChecker{}
	scanner := NewScanner(queue, skipChecker)

	// Execute with non-existent directory
	result, err := scanner.ScanDirectory("/nonexistent/directory", false, "")

	// Verify
	assert.Error(t, err, "Should return error for non-existent directory")
	assert.Nil(t, result, "Result should be nil on error")
}

func TestScanner_ScanDirectory_EmptyDirectory(t *testing.T) {
	// Setup
	testDir := setupTestDir(t)
	defer cleanupTestDir(t, testDir)

	queue := &MockQueue{}
	skipChecker := &MockSkipChecker{}
	scanner := NewScanner(queue, skipChecker)

	// Execute
	result, err := scanner.ScanDirectory(testDir, false, "")

	// Verify
	require.NoError(t, err)
	assert.Equal(t, 0, result.Scanned, "Should scan 0 files in empty directory")
	assert.Equal(t, 0, result.Queued, "Should queue 0 files")
}

func TestScanner_ScanDirectory_SkipReasonTracking(t *testing.T) {
	// Setup
	testDir := setupTestDir(t)
	defer cleanupTestDir(t, testDir)

	// Create multiple media files
	for i := 1; i <= 3; i++ {
		filename := filepath.Join(testDir, filepath.Base(testDir)+".mkv")
		err := os.WriteFile(filename, []byte("test"), 0644)
		require.NoError(t, err)
	}

	queue := &MockQueue{}
	skipChecker := &MockSkipChecker{
		shouldSkip: true,
		skipReason: skip.ReasonSubtitleExists,
	}
	scanner := NewScanner(queue, skipChecker)

	// Execute
	result, err := scanner.ScanDirectory(testDir, false, "")

	// Verify
	require.NoError(t, err)
	assert.NotNil(t, result.SkipReasons, "SkipReasons map should not be nil")
	assert.Greater(t, result.SkipReasons[string(skip.ReasonSubtitleExists)], 0, "Should track subtitle_exists reason")
}

func TestScanner_ScanDirectory_LanguageParameter(t *testing.T) {
	// Setup
	testDir := setupTestDir(t)
	defer cleanupTestDir(t, testDir)

	mediaFile := filepath.Join(testDir, "movie.mkv")
	err := os.WriteFile(mediaFile, []byte("test"), 0644)
	require.NoError(t, err)

	queue := &MockQueue{}
	skipChecker := &MockSkipChecker{shouldSkip: false}
	scanner := NewScanner(queue, skipChecker)

	// Execute with language parameter
	result, err := scanner.ScanDirectory(testDir, false, "en")

	// Verify
	require.NoError(t, err)
	assert.Equal(t, 1, result.Queued, "Should queue file with language parameter")
	// Note: Actual verification of language parameter passing would require
	// checking the queued task structure in a real implementation
}
