package webhooks

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/gofiber/fiber/v2"
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
	q, metrics, log := createTestQueue()
	server := NewServer(cfg, q, metrics, log)

	// Add some tasks to queue and processing
	task1 := queue.NewTask("/media/movie1.mkv", queue.TaskTypeTranscribe)
	task2 := queue.NewTask("/media/movie2.mkv", queue.TaskTypeTranscribe)
	q.Enqueue(task1)
	q.Enqueue(task2)
	q.Dequeue() // Move one to processing

	// Make request
	app := fiber.New()
	app.Get("/queue/status", server.handleQueueStatus)

	req := httptest.NewRequest("GET", "/queue/status", nil)
	resp, err := app.Test(req)
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
	q, metrics, log := createTestQueue()
	server := NewServer(cfg, q, metrics, log)

	app := fiber.New()
	app.Get("/queue/status", server.handleQueueStatus)

	req := httptest.NewRequest("GET", "/queue/status", nil)
	resp, err := app.Test(req)
	require.NoError(t, err)

	var result map[string]interface{}
	json.NewDecoder(resp.Body).Decode(&result)

	assert.Equal(t, "idle", result["status"])
	assert.Equal(t, float64(0), result["queued"])
	assert.Equal(t, float64(0), result["processing"])
	assert.Equal(t, true, result["idle"])
}

// TestHandleQueueProcessing verifies processing tasks endpoint
func TestHandleQueueProcessing(t *testing.T) {
	cfg := &config.Config{}
	q, metrics, log := createTestQueue()
	server := NewServer(cfg, q, metrics, log)

	// Add and dequeue tasks
	task1 := queue.NewTask("/media/movie1.mkv", queue.TaskTypeTranscribe)
	task2 := queue.NewTask("/media/movie2.mkv", queue.TaskTypeASR)
	q.Enqueue(task1)
	q.Enqueue(task2)
	q.Dequeue()
	q.Dequeue()

	app := fiber.New()
	app.Get("/queue/processing", server.handleQueueProcessing)

	req := httptest.NewRequest("GET", "/queue/processing", nil)
	resp, err := app.Test(req)
	require.NoError(t, err)

	assert.Equal(t, http.StatusOK, resp.StatusCode)

	var result map[string]interface{}
	json.NewDecoder(resp.Body).Decode(&result)

	tasks := result["tasks"].([]interface{})
	assert.Len(t, tasks, 2)
}

// TestHandleQueueHistory verifies history endpoint
func TestHandleQueueHistory(t *testing.T) {
	cfg := &config.Config{}
	q, metrics, log := createTestQueue()
	server := NewServer(cfg, q, metrics, log)

	// Complete some tasks
	for i := 1; i <= 5; i++ {
		task := queue.NewTask("/media/movie"+string(rune(i+'0'))+".mkv", queue.TaskTypeTranscribe)
		q.Enqueue(task)
		dequeued, _ := q.Dequeue()
		q.MarkDone(dequeued.ID)
	}

	app := fiber.New()
	app.Get("/queue/history", server.handleQueueHistory)

	req := httptest.NewRequest("GET", "/queue/history", nil)
	resp, err := app.Test(req)
	require.NoError(t, err)

	assert.Equal(t, http.StatusOK, resp.StatusCode)

	var result map[string]interface{}
	json.NewDecoder(resp.Body).Decode(&result)

	tasks := result["tasks"].([]interface{})
	assert.Len(t, tasks, 5)
	assert.Equal(t, float64(5), result["total"])
}

// TestHandleQueueHistory_Pagination verifies pagination
func TestHandleQueueHistory_Pagination(t *testing.T) {
	cfg := &config.Config{}
	q, metrics, log := createTestQueue()
	server := NewServer(cfg, q, metrics, log)

	// Complete 10 tasks
	for i := 1; i <= 10; i++ {
		task := queue.NewTask("/media/movie"+string(rune(i+'0'))+".mkv", queue.TaskTypeTranscribe)
		q.Enqueue(task)
		dequeued, _ := q.Dequeue()
		q.MarkDone(dequeued.ID)
	}

	app := fiber.New()
	app.Get("/queue/history", server.handleQueueHistory)

	// Request with limit and offset
	req := httptest.NewRequest("GET", "/queue/history?limit=5&offset=5", nil)
	resp, err := app.Test(req)
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
	q, metrics, log := createTestQueue()
	server := NewServer(cfg, q, metrics, log)

	// Create and enqueue task
	task := queue.NewTask("/media/movie.mkv", queue.TaskTypeTranscribe)
	q.Enqueue(task)

	app := fiber.New()
	app.Get("/tasks/:id", server.handleTaskStatus)

	req := httptest.NewRequest("GET", "/tasks/"+task.ID, nil)
	resp, err := app.Test(req)
	require.NoError(t, err)

	assert.Equal(t, http.StatusOK, resp.StatusCode)

	var result map[string]interface{}
	json.NewDecoder(resp.Body).Decode(&result)

	assert.Equal(t, task.ID, result["id"])
	assert.Equal(t, "/media/movie.mkv", result["file_path"])
	assert.Equal(t, "queued", result["status"])
}

// TestHandleTaskStatus_NotFound verifies 404 for missing task
func TestHandleTaskStatus_NotFound(t *testing.T) {
	cfg := &config.Config{}
	q, metrics, log := createTestQueue()
	server := NewServer(cfg, q, metrics, log)

	app := fiber.New()
	app.Get("/tasks/:id", server.handleTaskStatus)

	req := httptest.NewRequest("GET", "/tasks/nonexistent", nil)
	resp, err := app.Test(req)
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
