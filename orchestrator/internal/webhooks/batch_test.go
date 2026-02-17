package webhooks

import (
	"encoding/json"
	"io"
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"

	"github.com/gofiber/fiber/v2"
	"github.com/mccloud/subgen/orchestrator/internal/config"
	"github.com/mccloud/subgen/orchestrator/internal/monitor"
	"github.com/sirupsen/logrus"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// MockScanner implements monitor.Scanner for testing
type MockScanner struct {
	result *monitor.ScanResult
	err    error
}

func (ms *MockScanner) ScanDirectory(directory string, recursive bool, language string) (*monitor.ScanResult, error) {
	if ms.err != nil {
		return nil, ms.err
	}
	return ms.result, nil
}

// setupBatchTestServer creates a test server with batch endpoint
func setupBatchTestServer(scanner monitor.Scanner) *Server {
	cfg := &config.Config{
		WebhookPort: 9000,
	}
	queue := &MockQueue{}
	log := logrus.New()
	log.SetOutput(io.Discard) // Suppress logs during tests

	server := NewServer(cfg, queue, log)
	server.SetScanner(scanner) // Inject mock scanner
	return server
}

// Helper to create test directory structure
func setupBatchTestDir(t *testing.T) string {
	tmpDir, err := os.MkdirTemp("", "batch_test")
	require.NoError(t, err)
	return tmpDir
}

func cleanupBatchTestDir(t *testing.T, dir string) {
	err := os.RemoveAll(dir)
	require.NoError(t, err)
}

func TestHandleBatch_Success(t *testing.T) {
	// Setup
	testDir := setupBatchTestDir(t)
	defer cleanupBatchTestDir(t, testDir)

	// Create test files
	err := os.WriteFile(filepath.Join(testDir, "movie1.mkv"), []byte("test"), 0644)
	require.NoError(t, err)
	err = os.WriteFile(filepath.Join(testDir, "movie2.mp4"), []byte("test"), 0644)
	require.NoError(t, err)

	mockScanner := &MockScanner{
		result: &monitor.ScanResult{
			Scanned:     2,
			Queued:      2,
			Skipped:     0,
			SkipReasons: make(map[string]int),
		},
	}
	server := setupBatchTestServer(mockScanner)

	// Execute
	req := httptest.NewRequest("POST", "/batch?directory="+testDir, nil)
	resp, err := server.App().Test(req)

	// Verify
	require.NoError(t, err)
	assert.Equal(t, fiber.StatusOK, resp.StatusCode)

	var result map[string]interface{}
	err = json.NewDecoder(resp.Body).Decode(&result)
	require.NoError(t, err)

	assert.Equal(t, "success", result["status"])
	assert.Equal(t, float64(2), result["scanned"])
	assert.Equal(t, float64(2), result["queued"])
	assert.Equal(t, float64(0), result["skipped"])
}

func TestHandleBatch_DirectoryNotFound(t *testing.T) {
	// Setup
	mockScanner := &MockScanner{
		err: os.ErrNotExist,
	}
	server := setupBatchTestServer(mockScanner)

	// Execute
	req := httptest.NewRequest("POST", "/batch?directory=/nonexistent", nil)
	resp, err := server.App().Test(req)

	// Verify
	require.NoError(t, err)
	assert.Equal(t, fiber.StatusBadRequest, resp.StatusCode)

	var result map[string]interface{}
	err = json.NewDecoder(resp.Body).Decode(&result)
	require.NoError(t, err)

	assert.Equal(t, "error", result["status"])
	assert.Contains(t, result["error"], "directory")
}

func TestHandleBatch_MissingDirectoryParameter(t *testing.T) {
	// Setup
	mockScanner := &MockScanner{}
	server := setupBatchTestServer(mockScanner)

	// Execute - missing directory parameter
	req := httptest.NewRequest("POST", "/batch", nil)
	resp, err := server.App().Test(req)

	// Verify
	require.NoError(t, err)
	assert.Equal(t, fiber.StatusBadRequest, resp.StatusCode)

	var result map[string]interface{}
	err = json.NewDecoder(resp.Body).Decode(&result)
	require.NoError(t, err)

	assert.Equal(t, "error", result["status"])
	assert.Contains(t, result["error"], "directory parameter required")
}

func TestHandleBatch_RecursiveParameter(t *testing.T) {
	// Setup
	testDir := setupBatchTestDir(t)
	defer cleanupBatchTestDir(t, testDir)

	mockScanner := &MockScanner{
		result: &monitor.ScanResult{
			Scanned:     5,
			Queued:      3,
			Skipped:     2,
			SkipReasons: map[string]int{"subtitle_exists": 2},
		},
	}
	server := setupBatchTestServer(mockScanner)

	// Execute with recursive=true
	req := httptest.NewRequest("POST", "/batch?directory="+testDir+"&recursive=true", nil)
	resp, err := server.App().Test(req)

	// Verify
	require.NoError(t, err)
	assert.Equal(t, fiber.StatusOK, resp.StatusCode)

	var result map[string]interface{}
	err = json.NewDecoder(resp.Body).Decode(&result)
	require.NoError(t, err)

	assert.Equal(t, "success", result["status"])
	assert.Equal(t, float64(5), result["scanned"])
}

func TestHandleBatch_LanguageParameter(t *testing.T) {
	// Setup
	testDir := setupBatchTestDir(t)
	defer cleanupBatchTestDir(t, testDir)

	mockScanner := &MockScanner{
		result: &monitor.ScanResult{
			Scanned:     3,
			Queued:      3,
			Skipped:     0,
			SkipReasons: make(map[string]int),
		},
	}
	server := setupBatchTestServer(mockScanner)

	// Execute with language parameter
	req := httptest.NewRequest("POST", "/batch?directory="+testDir+"&language=en", nil)
	resp, err := server.App().Test(req)

	// Verify
	require.NoError(t, err)
	assert.Equal(t, fiber.StatusOK, resp.StatusCode)

	var result map[string]interface{}
	err = json.NewDecoder(resp.Body).Decode(&result)
	require.NoError(t, err)

	assert.Equal(t, "success", result["status"])
}

func TestHandleBatch_EmptyDirectory(t *testing.T) {
	// Setup
	testDir := setupBatchTestDir(t)
	defer cleanupBatchTestDir(t, testDir)

	mockScanner := &MockScanner{
		result: &monitor.ScanResult{
			Scanned:     0,
			Queued:      0,
			Skipped:     0,
			SkipReasons: make(map[string]int),
		},
	}
	server := setupBatchTestServer(mockScanner)

	// Execute
	req := httptest.NewRequest("POST", "/batch?directory="+testDir, nil)
	resp, err := server.App().Test(req)

	// Verify
	require.NoError(t, err)
	assert.Equal(t, fiber.StatusOK, resp.StatusCode)

	var result map[string]interface{}
	err = json.NewDecoder(resp.Body).Decode(&result)
	require.NoError(t, err)

	assert.Equal(t, "success", result["status"])
	assert.Equal(t, float64(0), result["scanned"])
}

func TestHandleBatch_SkipReasons(t *testing.T) {
	// Setup
	testDir := setupBatchTestDir(t)
	defer cleanupBatchTestDir(t, testDir)

	mockScanner := &MockScanner{
		result: &monitor.ScanResult{
			Scanned: 10,
			Queued:  3,
			Skipped: 7,
			SkipReasons: map[string]int{
				"subtitle_exists":         5,
				"audio_language_mismatch": 2,
			},
		},
	}
	server := setupBatchTestServer(mockScanner)

	// Execute
	req := httptest.NewRequest("POST", "/batch?directory="+testDir, nil)
	resp, err := server.App().Test(req)

	// Verify
	require.NoError(t, err)
	assert.Equal(t, fiber.StatusOK, resp.StatusCode)

	var result map[string]interface{}
	err = json.NewDecoder(resp.Body).Decode(&result)
	require.NoError(t, err)

	assert.Equal(t, "success", result["status"])
	skipReasons := result["skip_reasons"].(map[string]interface{})
	assert.Equal(t, float64(5), skipReasons["subtitle_exists"])
	assert.Equal(t, float64(2), skipReasons["audio_language_mismatch"])
}

func TestHandleBatch_GetRequestReturnsError(t *testing.T) {
	// Setup
	mockScanner := &MockScanner{}
	server := setupBatchTestServer(mockScanner)

	// Execute - GET instead of POST
	req := httptest.NewRequest("GET", "/batch?directory=/tmp", nil)
	resp, err := server.App().Test(req)

	// Verify - should return method not allowed or not found
	require.NoError(t, err)
	assert.NotEqual(t, fiber.StatusOK, resp.StatusCode)
}

func TestHandleBatch_PermissionDenied(t *testing.T) {
	// Skip on systems where we can't properly test permissions
	if os.Getuid() == 0 {
		t.Skip("Skipping permission test when running as root")
	}

	// Setup - create directory with no read permissions
	testDir := setupBatchTestDir(t)
	defer cleanupBatchTestDir(t, testDir)

	err := os.Chmod(testDir, 0000)
	require.NoError(t, err)
	defer func() {
		_ = os.Chmod(testDir, 0755) // Restore for cleanup
	}()

	mockScanner := &MockScanner{
		err: os.ErrPermission,
	}
	server := setupBatchTestServer(mockScanner)

	// Execute
	req := httptest.NewRequest("POST", "/batch?directory="+testDir, nil)
	resp, err := server.App().Test(req)

	// Verify
	require.NoError(t, err)
	assert.Equal(t, fiber.StatusBadRequest, resp.StatusCode)

	var result map[string]interface{}
	err = json.NewDecoder(resp.Body).Decode(&result)
	require.NoError(t, err)

	assert.Equal(t, "error", result["status"])
}
