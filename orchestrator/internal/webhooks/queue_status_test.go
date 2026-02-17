package webhooks

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/mccloud/subgen/orchestrator/internal/config"
	"github.com/mccloud/subgen/orchestrator/internal/queue"
	"github.com/prometheus/client_golang/prometheus"
	"github.com/sirupsen/logrus"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestHandleQueueStatus verifies queue status endpoint
func TestHandleQueueStatus(t *testing.T) {
	// Setup
	cfg := &config.Config{}
	q, _, log := createTestQueue()
	adapter := NewQueueAdapter(q)
	server := NewServer(cfg, adapter, log)

	// Add some tasks to queue and processing
	task1 := queue.NewTask("/media/movie1.mkv", queue.TaskTypeTranscribe)
	task2 := queue.NewTask("/media/movie2.mkv", queue.TaskTypeTranscribe)
	q.Enqueue(task1)
	q.Enqueue(task2)
	q.Dequeue() // Move one to processing

	// Make request
	req := httptest.NewRequest("GET", "/queue/status", nil)
	resp, err := server.App().Test(req)
	require.NoError(t, err)

	// Verify response
	assert.Equal(t, http.StatusOK, resp.StatusCode)

	var result map[string]interface{}
	err = json.NewDecoder(resp.Body).Decode(&result)
	require.NoError(t, err)

	assert.Equal(t, "active", result["status"])
	assert.Equal(t, float64(1), result["queued"])
	assert.Equal(t, float64(1), result["processing"])
	assert.Equal(t, false, result["idle"])
}

// TestHandleQueueStatus_Idle verifies idle state
func TestHandleQueueStatus_Idle(t *testing.T) {
	cfg := &config.Config{}
	q, _, log := createTestQueue()
	adapter := NewQueueAdapter(q)
	server := NewServer(cfg, adapter, log)

	req := httptest.NewRequest("GET", "/queue/status", nil)
	resp, err := server.App().Test(req)
	require.NoError(t, err)

	var result map[string]interface{}
	err = json.NewDecoder(resp.Body).Decode(&result)
	require.NoError(t, err)

	assert.Equal(t, "idle", result["status"])
	assert.Equal(t, float64(0), result["queued"])
	assert.Equal(t, float64(0), result["processing"])
	assert.Equal(t, true, result["idle"])
}

// TestHandleQueueProcessing verifies processing tasks endpoint
func TestHandleQueueProcessing(t *testing.T) {
	cfg := &config.Config{}
	q, _, log := createTestQueue()
	adapter := NewQueueAdapter(q)
	server := NewServer(cfg, adapter, log)

	// Add and dequeue tasks
	task1 := queue.NewTask("/media/movie1.mkv", queue.TaskTypeTranscribe)
	task2 := queue.NewTask("/media/movie2.mkv", queue.TaskTypeASR)
	q.Enqueue(task1)
	q.Enqueue(task2)
	q.Dequeue()
	q.Dequeue()

	req := httptest.NewRequest("GET", "/queue/processing", nil)
	resp, err := server.App().Test(req)
	require.NoError(t, err)

	assert.Equal(t, http.StatusOK, resp.StatusCode)

	var result map[string]interface{}
	err = json.NewDecoder(resp.Body).Decode(&result)
	require.NoError(t, err)

	tasks := result["tasks"].([]interface{})
	assert.Len(t, tasks, 2)
}

// TestHandleQueueHistory verifies history endpoint
func TestHandleQueueHistory(t *testing.T) {
	cfg := &config.Config{}
	q, _, log := createTestQueue()
	adapter := NewQueueAdapter(q)
	server := NewServer(cfg, adapter, log)

	// Complete some tasks
	for i := 1; i <= 5; i++ {
		task := queue.NewTask("/media/movie"+string(rune(i+'0'))+".mkv", queue.TaskTypeTranscribe)
		q.Enqueue(task)
		dequeued, _ := q.Dequeue()
		q.MarkDone(dequeued.ID)
	}

	req := httptest.NewRequest("GET", "/queue/history", nil)
	resp, err := server.App().Test(req)
	require.NoError(t, err)

	assert.Equal(t, http.StatusOK, resp.StatusCode)

	var result map[string]interface{}
	err = json.NewDecoder(resp.Body).Decode(&result)
	require.NoError(t, err)

	tasks := result["tasks"].([]interface{})
	assert.Len(t, tasks, 5)
	assert.Equal(t, float64(5), result["total"])
}

// TestHandleQueueHistory_Pagination verifies pagination
func TestHandleQueueHistory_Pagination(t *testing.T) {
	cfg := &config.Config{}
	q, _, log := createTestQueue()
	adapter := NewQueueAdapter(q)
	server := NewServer(cfg, adapter, log)

	// Complete 10 tasks
	for i := 1; i <= 10; i++ {
		task := queue.NewTask("/media/movie"+string(rune(i+'0'))+".mkv", queue.TaskTypeTranscribe)
		q.Enqueue(task)
		dequeued, _ := q.Dequeue()
		q.MarkDone(dequeued.ID)
	}

	// Request with limit and offset
	req := httptest.NewRequest("GET", "/queue/history?limit=5&offset=5", nil)
	resp, err := server.App().Test(req)
	require.NoError(t, err)

	var result map[string]interface{}
	json.NewDecoder(resp.Body).Decode(&result)

	tasks := result["tasks"].([]interface{})
	assert.Len(t, tasks, 5)
	assert.Equal(t, float64(5), result["limit"])
	assert.Equal(t, float64(5), result["offset"])
}

// TestHandleTaskStatus_Found verifies task lookup
func TestHandleTaskStatus_Found(t *testing.T) {
	cfg := &config.Config{}
	q, _, log := createTestQueue()
	adapter := NewQueueAdapter(q)
	server := NewServer(cfg, adapter, log)

	// Create and enqueue task
	task := queue.NewTask("/media/movie.mkv", queue.TaskTypeTranscribe)
	q.Enqueue(task)

	req := httptest.NewRequest("GET", "/tasks/"+task.ID, nil)
	resp, err := server.App().Test(req)
	require.NoError(t, err)

	assert.Equal(t, http.StatusOK, resp.StatusCode)

	var result map[string]interface{}
	err = json.NewDecoder(resp.Body).Decode(&result)
	require.NoError(t, err)

	// Note: JSON fields are capitalized (no json tags on TaskInfo struct)
	assert.Equal(t, task.ID, result["ID"])
	assert.Equal(t, "/media/movie.mkv", result["FilePath"])
	assert.Equal(t, "queued", result["Status"])
}

// TestHandleTaskStatus_NotFound verifies 404 for missing task
func TestHandleTaskStatus_NotFound(t *testing.T) {
	cfg := &config.Config{}
	q, _, log := createTestQueue()
	adapter := NewQueueAdapter(q)
	server := NewServer(cfg, adapter, log)

	req := httptest.NewRequest("GET", "/tasks/nonexistent", nil)
	resp, err := server.App().Test(req)
	require.NoError(t, err)

	assert.Equal(t, http.StatusNotFound, resp.StatusCode)

	var result map[string]interface{}
	json.NewDecoder(resp.Body).Decode(&result)

	assert.Equal(t, "error", result["status"])
	assert.Contains(t, result["error"], "not found")
}

// Helper function to create test queue
func createTestQueue() (*queue.Queue, *queue.QueueMetrics, *logrus.Logger) {
	registry := prometheus.NewRegistry()
	metrics := queue.NewQueueMetricsWithRegistry(registry)
	log := logrus.New()
	log.SetLevel(logrus.ErrorLevel) // Reduce noise in tests
	q := queue.NewQueue(100, metrics, log)
	return q, metrics, log
}
