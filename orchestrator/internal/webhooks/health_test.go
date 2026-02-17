package webhooks

import (
	"encoding/json"
	"errors"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestHealthEndpoint tests the /health endpoint (liveness probe)
// Following TDD: These tests should FAIL initially because /health endpoint doesn't exist yet
func TestHealthEndpoint(t *testing.T) {
	server, _ := createTestServer(t)
	app := server.app

	t.Run("HappyPath_ReturnsHTTP200", func(t *testing.T) {
		req := httptest.NewRequest("GET", "/health", nil)
		resp, err := app.Test(req)
		require.NoError(t, err)
		assert.Equal(t, 200, resp.StatusCode, "/health should return 200 OK")
	})

	t.Run("HappyPath_ReturnsJSON", func(t *testing.T) {
		req := httptest.NewRequest("GET", "/health", nil)
		resp, err := app.Test(req)
		require.NoError(t, err)
		assert.Contains(t, resp.Header.Get("Content-Type"), "application/json")
	})

	t.Run("HappyPath_ContainsStatusAlive", func(t *testing.T) {
		req := httptest.NewRequest("GET", "/health", nil)
		resp, err := app.Test(req)
		require.NoError(t, err)

		var result map[string]interface{}
		err = json.NewDecoder(resp.Body).Decode(&result)
		require.NoError(t, err)

		assert.Equal(t, "alive", result["status"], "status should be 'alive'")
	})

	t.Run("HappyPath_ContainsTimestamp", func(t *testing.T) {
		req := httptest.NewRequest("GET", "/health", nil)
		resp, err := app.Test(req)
		require.NoError(t, err)

		var result map[string]interface{}
		err = json.NewDecoder(resp.Body).Decode(&result)
		require.NoError(t, err)

		timestamp, ok := result["timestamp"].(float64)
		assert.True(t, ok, "timestamp should be a number")
		assert.Greater(t, timestamp, float64(0))
	})

	t.Run("HappyPath_TimestampIsRecent", func(t *testing.T) {
		req := httptest.NewRequest("GET", "/health", nil)
		resp, err := app.Test(req)
		require.NoError(t, err)

		var result map[string]interface{}
		err = json.NewDecoder(resp.Body).Decode(&result)
		require.NoError(t, err)

		timestamp := int64(result["timestamp"].(float64))
		now := time.Now().Unix()
		assert.InDelta(t, now, timestamp, 5, "timestamp should be within 5 seconds of now")
	})

	t.Run("UnhappyPath_RejectsPOST", func(t *testing.T) {
		req := httptest.NewRequest("POST", "/health", nil)
		resp, err := app.Test(req)
		require.NoError(t, err)
		assert.Equal(t, 405, resp.StatusCode, "POST should return 405 Method Not Allowed")
	})

	t.Run("UnhappyPath_RejectsPUT", func(t *testing.T) {
		req := httptest.NewRequest("PUT", "/health", nil)
		resp, err := app.Test(req)
		require.NoError(t, err)
		assert.Equal(t, 405, resp.StatusCode)
	})

	t.Run("UnhappyPath_RejectsDELETE", func(t *testing.T) {
		req := httptest.NewRequest("DELETE", "/health", nil)
		resp, err := app.Test(req)
		require.NoError(t, err)
		assert.Equal(t, 405, resp.StatusCode)
	})
}

// TestReadyEndpoint tests the /ready endpoint (readiness probe)
// Following TDD: These tests should FAIL initially because /ready endpoint doesn't exist yet
func TestReadyEndpoint(t *testing.T) {
	server, mockQueue := createTestServer(t)
	app := server.app

	t.Run("HappyPath_ReturnsHTTP200WhenReady", func(t *testing.T) {
		// Use existing MockWorkerPool from detect_language_test.go
		mockPool := &MockWorkerPool{
			SelectWorkerFunc: func() (*Worker, error) {
				return &Worker{Address: "localhost:50051", Healthy: true}, nil
			},
		}
		server.workerPool = mockPool
		mockQueue.Reset()

		req := httptest.NewRequest("GET", "/ready", nil)
		resp, err := app.Test(req)
		require.NoError(t, err)
		assert.Equal(t, 200, resp.StatusCode, "/ready should return 200 when workers available")
	})

	t.Run("HappyPath_ReturnsJSON", func(t *testing.T) {
		mockPool := &MockWorkerPool{
			SelectWorkerFunc: func() (*Worker, error) {
				return &Worker{Address: "localhost:50051", Healthy: true}, nil
			},
		}
		server.workerPool = mockPool

		req := httptest.NewRequest("GET", "/ready", nil)
		resp, err := app.Test(req)
		require.NoError(t, err)
		assert.Contains(t, resp.Header.Get("Content-Type"), "application/json")
	})

	t.Run("HappyPath_ContainsStatusReady", func(t *testing.T) {
		mockPool := &MockWorkerPool{
			SelectWorkerFunc: func() (*Worker, error) {
				return &Worker{Address: "localhost:50051", Healthy: true}, nil
			},
		}
		server.workerPool = mockPool

		req := httptest.NewRequest("GET", "/ready", nil)
		resp, err := app.Test(req)
		require.NoError(t, err)

		var result map[string]interface{}
		err = json.NewDecoder(resp.Body).Decode(&result)
		require.NoError(t, err)

		assert.Equal(t, "ready", result["status"])
	})

	t.Run("HappyPath_IncludesWorkerCount", func(t *testing.T) {
		mockPool := &MockWorkerPool{
			SelectWorkerFunc: func() (*Worker, error) {
				return &Worker{Address: "localhost:50051", Healthy: true}, nil
			},
		}
		server.workerPool = mockPool

		req := httptest.NewRequest("GET", "/ready", nil)
		resp, err := app.Test(req)
		require.NoError(t, err)

		var result map[string]interface{}
		err = json.NewDecoder(resp.Body).Decode(&result)
		require.NoError(t, err)

		// Should have worker count in response
		_, hasWorkers := result["workers_available"]
		assert.True(t, hasWorkers, "response should include workers_available")
	})

	t.Run("HappyPath_IncludesQueueInfo", func(t *testing.T) {
		mockPool := &MockWorkerPool{
			SelectWorkerFunc: func() (*Worker, error) {
				return &Worker{Address: "localhost:50051", Healthy: true}, nil
			},
		}
		server.workerPool = mockPool

		// Add some tasks to queue
		mockQueue.Enqueue(Task{FilePath: "/test1.mkv"})
		mockQueue.Enqueue(Task{FilePath: "/test2.mkv"})

		req := httptest.NewRequest("GET", "/ready", nil)
		resp, err := app.Test(req)
		require.NoError(t, err)

		var result map[string]interface{}
		err = json.NewDecoder(resp.Body).Decode(&result)
		require.NoError(t, err)

		queueSize, ok := result["queue_size"].(float64)
		assert.True(t, ok)
		assert.Equal(t, float64(2), queueSize)
	})

	t.Run("UnhappyPath_Returns503WhenNoWorkers", func(t *testing.T) {
		mockPool := &MockWorkerPool{
			SelectWorkerFunc: func() (*Worker, error) {
				return nil, errors.New("no workers available")
			},
		}
		server.workerPool = mockPool

		req := httptest.NewRequest("GET", "/ready", nil)
		resp, err := app.Test(req)
		require.NoError(t, err)
		assert.Equal(t, 503, resp.StatusCode)

		var result map[string]interface{}
		err = json.NewDecoder(resp.Body).Decode(&result)
		require.NoError(t, err)

		assert.Equal(t, "not_ready", result["status"])
		assert.Contains(t, result["reason"], "workers")
	})

	t.Run("UnhappyPath_Returns503WhenWorkerPoolNotInitialized", func(t *testing.T) {
		// No worker pool set
		server.workerPool = nil

		req := httptest.NewRequest("GET", "/ready", nil)
		resp, err := app.Test(req)
		require.NoError(t, err)
		assert.Equal(t, 503, resp.StatusCode)

		var result map[string]interface{}
		err = json.NewDecoder(resp.Body).Decode(&result)
		require.NoError(t, err)

		assert.Equal(t, "not_ready", result["status"])
		assert.Contains(t, result["reason"], "not_initialized")
	})

	t.Run("UnhappyPath_Returns503WhenQueueOverloaded", func(t *testing.T) {
		mockPool := &MockWorkerPool{
			SelectWorkerFunc: func() (*Worker, error) {
				return &Worker{Address: "localhost:50051", Healthy: true}, nil
			},
		}
		server.workerPool = mockPool
		mockQueue.Reset()

		// Add many tasks to simulate overload
		for i := 0; i < 10001; i++ {
			mockQueue.Enqueue(Task{FilePath: "/test.mkv"})
		}

		req := httptest.NewRequest("GET", "/ready", nil)
		resp, err := app.Test(req)
		require.NoError(t, err)
		assert.Equal(t, 503, resp.StatusCode)

		var result map[string]interface{}
		err = json.NewDecoder(resp.Body).Decode(&result)
		require.NoError(t, err)

		assert.Equal(t, "not_ready", result["status"])
		assert.Contains(t, result["reason"], "queue")
	})

	t.Run("UnhappyPath_RejectsPOST", func(t *testing.T) {
		req := httptest.NewRequest("POST", "/ready", nil)
		resp, err := app.Test(req)
		require.NoError(t, err)
		assert.Equal(t, 405, resp.StatusCode)
	})
}

// TestLiveEndpoint tests the /live endpoint (alternative liveness probe)
// Following TDD: These tests should FAIL initially because /live endpoint doesn't exist yet
func TestLiveEndpoint(t *testing.T) {
	server, _ := createTestServer(t)
	app := server.app

	t.Run("HappyPath_ReturnsHTTP200", func(t *testing.T) {
		req := httptest.NewRequest("GET", "/live", nil)
		resp, err := app.Test(req)
		require.NoError(t, err)
		assert.Equal(t, 200, resp.StatusCode)
	})

	t.Run("HappyPath_ReturnsJSON", func(t *testing.T) {
		req := httptest.NewRequest("GET", "/live", nil)
		resp, err := app.Test(req)
		require.NoError(t, err)
		assert.Contains(t, resp.Header.Get("Content-Type"), "application/json")
	})

	t.Run("HappyPath_ContainsStatusAlive", func(t *testing.T) {
		req := httptest.NewRequest("GET", "/live", nil)
		resp, err := app.Test(req)
		require.NoError(t, err)

		var result map[string]interface{}
		err = json.NewDecoder(resp.Body).Decode(&result)
		require.NoError(t, err)

		assert.Equal(t, "alive", result["status"])
	})

	t.Run("HappyPath_IncludesUptimeSeconds", func(t *testing.T) {
		req := httptest.NewRequest("GET", "/live", nil)
		resp, err := app.Test(req)
		require.NoError(t, err)

		var result map[string]interface{}
		err = json.NewDecoder(resp.Body).Decode(&result)
		require.NoError(t, err)

		uptime, ok := result["uptime_seconds"].(float64)
		assert.True(t, ok)
		assert.GreaterOrEqual(t, uptime, float64(0))
	})

	t.Run("UnhappyPath_RejectsPOST", func(t *testing.T) {
		req := httptest.NewRequest("POST", "/live", nil)
		resp, err := app.Test(req)
		require.NoError(t, err)
		assert.Equal(t, 405, resp.StatusCode)
	})
}
