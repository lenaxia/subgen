package webhooks

import (
	"encoding/json"
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"

	"github.com/mccloud/subgen/orchestrator/internal/config"
	"github.com/mccloud/subgen/orchestrator/internal/monitor"
	"github.com/sirupsen/logrus"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestBatchEndpointIntegration tests the full batch endpoint with real scanner
func TestBatchEndpointIntegration(t *testing.T) {
	// Setup test directory
	testDir, err := os.MkdirTemp("", "batch_integration_test")
	require.NoError(t, err)
	defer os.RemoveAll(testDir)

	// Create test files
	testFiles := []string{
		"movie1.mkv",
		"movie2.mp4",
		"movie3.avi",
		"document.txt", // Non-media file (should be filtered)
	}

	for _, filename := range testFiles {
		filePath := filepath.Join(testDir, filename)
		err := os.WriteFile(filePath, []byte("test content"), 0644)
		require.NoError(t, err)
	}

	// Create subdirectory with more files
	subdir := filepath.Join(testDir, "subdir")
	err = os.Mkdir(subdir, 0755)
	require.NoError(t, err)

	err = os.WriteFile(filepath.Join(subdir, "movie4.mp4"), []byte("test"), 0644)
	require.NoError(t, err)

	// Setup server with real scanner (no skip checker for this test)
	cfg := &config.Config{
		WebhookPort: 9000,
	}
	queue := &MockQueue{}
	log := logrus.New()
	log.SetLevel(logrus.ErrorLevel) // Suppress logs

	server := NewServer(cfg, queue, log)
	scanner := monitor.NewScanner(queue, nil) // No skip checker
	server.SetScanner(scanner)

	// Test 1: Non-recursive scan
	t.Run("NonRecursiveScan", func(t *testing.T) {
		req := httptest.NewRequest("POST", "/batch?directory="+testDir, nil)
		resp, err := server.App().Test(req)
		require.NoError(t, err)

		var result map[string]interface{}
		err = json.NewDecoder(resp.Body).Decode(&result)
		require.NoError(t, err)

		assert.Equal(t, "success", result["status"])
		assert.Equal(t, float64(3), result["scanned"], "Should scan 3 media files (not .txt)")
		assert.Equal(t, float64(3), result["queued"], "Should queue all scanned files")
		assert.Equal(t, float64(0), result["skipped"], "Should skip 0 files (no skip checker)")
	})

	// Reset queue for next test
	queue.EnqueuedTasks = nil

	// Test 2: Recursive scan
	t.Run("RecursiveScan", func(t *testing.T) {
		req := httptest.NewRequest("POST", "/batch?directory="+testDir+"&recursive=true", nil)
		resp, err := server.App().Test(req)
		require.NoError(t, err)

		var result map[string]interface{}
		err = json.NewDecoder(resp.Body).Decode(&result)
		require.NoError(t, err)

		assert.Equal(t, "success", result["status"])
		assert.Equal(t, float64(4), result["scanned"], "Should scan 4 media files including subdirectory")
		assert.Equal(t, float64(4), result["queued"], "Should queue all scanned files")
	})

	// Test 3: Language parameter
	t.Run("WithLanguageParameter", func(t *testing.T) {
		queue.EnqueuedTasks = nil
		req := httptest.NewRequest("POST", "/batch?directory="+testDir+"&language=en", nil)
		resp, err := server.App().Test(req)
		require.NoError(t, err)

		var result map[string]interface{}
		err = json.NewDecoder(resp.Body).Decode(&result)
		require.NoError(t, err)

		assert.Equal(t, "success", result["status"])
		assert.Greater(t, len(queue.EnqueuedTasks), 0, "Should queue at least one task")
	})
}

// TestBatchEndpointWithSkipLogic tests batch endpoint with skip logic
func TestBatchEndpointWithSkipLogic(t *testing.T) {
	// Setup test directory
	testDir, err := os.MkdirTemp("", "batch_skip_test")
	require.NoError(t, err)
	defer os.RemoveAll(testDir)

	// Create test files
	mediaFiles := []string{"movie1.mkv", "movie2.mp4", "movie3.avi"}
	for _, filename := range mediaFiles {
		filePath := filepath.Join(testDir, filename)
		err := os.WriteFile(filePath, []byte("test"), 0644)
		require.NoError(t, err)
	}

	// Setup server with mock skip checker that skips all files
	cfg := &config.Config{
		WebhookPort: 9000,
	}
	queue := &MockQueue{}
	log := logrus.New()
	log.SetLevel(logrus.ErrorLevel)

	server := NewServer(cfg, queue, log)
	skipChecker := &MockSkipChecker{
		ShouldSkip: true, // Skip all files
		SkipReason: "subtitle_file_exists",
	}
	scanner := monitor.NewScanner(queue, skipChecker)
	server.SetScanner(scanner)

	// Execute batch scan
	req := httptest.NewRequest("POST", "/batch?directory="+testDir, nil)
	resp, err := server.App().Test(req)
	require.NoError(t, err)

	var result map[string]interface{}
	err = json.NewDecoder(resp.Body).Decode(&result)
	require.NoError(t, err)

	// Verify skip logic was applied
	assert.Equal(t, "success", result["status"])
	assert.Equal(t, float64(3), result["scanned"], "Should scan 3 files")
	assert.Equal(t, float64(0), result["queued"], "Should queue 0 files (all skipped)")
	assert.Equal(t, float64(3), result["skipped"], "Should skip 3 files")

	skipReasons := result["skip_reasons"].(map[string]interface{})
	assert.Greater(t, skipReasons["subtitle_file_exists"], float64(0), "Should track skip reasons")
}

// TestBatchEndpointErrorCases tests various error scenarios
func TestBatchEndpointErrorCases(t *testing.T) {
	cfg := &config.Config{WebhookPort: 9000}
	queue := &MockQueue{}
	log := logrus.New()
	log.SetLevel(logrus.ErrorLevel)

	server := NewServer(cfg, queue, log)
	scanner := monitor.NewScanner(queue, nil)
	server.SetScanner(scanner)

	t.Run("NonexistentDirectory", func(t *testing.T) {
		req := httptest.NewRequest("POST", "/batch?directory=/nonexistent/path", nil)
		resp, err := server.App().Test(req)
		require.NoError(t, err)

		var result map[string]interface{}
		err = json.NewDecoder(resp.Body).Decode(&result)
		require.NoError(t, err)

		assert.Equal(t, "error", result["status"])
		assert.Contains(t, result["error"], "directory not found")
	})

	t.Run("FileInsteadOfDirectory", func(t *testing.T) {
		// Create a temporary file
		tmpFile, err := os.CreateTemp("", "not_a_dir")
		require.NoError(t, err)
		defer os.Remove(tmpFile.Name())
		tmpFile.Close()

		req := httptest.NewRequest("POST", "/batch?directory="+tmpFile.Name(), nil)
		resp, err := server.App().Test(req)
		require.NoError(t, err)

		var result map[string]interface{}
		err = json.NewDecoder(resp.Body).Decode(&result)
		require.NoError(t, err)

		assert.Equal(t, "error", result["status"])
		assert.Contains(t, result["error"], "not a directory")
	})

	t.Run("ScannerNotInitialized", func(t *testing.T) {
		// Create server without scanner
		serverNoScanner := NewServer(cfg, queue, log)
		// Don't call SetScanner()

		req := httptest.NewRequest("POST", "/batch?directory=/tmp", nil)
		resp, err := serverNoScanner.App().Test(req)
		require.NoError(t, err)

		var result map[string]interface{}
		err = json.NewDecoder(resp.Body).Decode(&result)
		require.NoError(t, err)

		assert.Equal(t, "error", result["status"])
		assert.Contains(t, result["error"], "scanner not initialized")
	})
}
