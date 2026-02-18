package webhooks

import (
	"encoding/json"
	"net/http/httptest"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestK8sHealthAliases tests that K8s-standard health check paths work
// These are aliases to the existing /health, /ready, /live endpoints
func TestK8sHealthAliases(t *testing.T) {
	server, _ := createTestServer(t)
	app := server.app

	t.Run("healthz_alias_works", func(t *testing.T) {
		req := httptest.NewRequest("GET", "/healthz", nil)
		resp, err := app.Test(req)
		require.NoError(t, err)
		assert.Equal(t, 200, resp.StatusCode, "/healthz should return 200")

		var result map[string]interface{}
		err = json.NewDecoder(resp.Body).Decode(&result)
		require.NoError(t, err)
		assert.Equal(t, "alive", result["status"], "/healthz should return alive status")
	})

	t.Run("livez_alias_works", func(t *testing.T) {
		req := httptest.NewRequest("GET", "/livez", nil)
		resp, err := app.Test(req)
		require.NoError(t, err)
		assert.Equal(t, 200, resp.StatusCode, "/livez should return 200")

		var result map[string]interface{}
		err = json.NewDecoder(resp.Body).Decode(&result)
		require.NoError(t, err)
		assert.Equal(t, "alive", result["status"], "/livez should return alive status")
	})

	t.Run("readyz_alias_works", func(t *testing.T) {
		// Set up worker pool for readiness check
		mockPool := &MockWorkerPool{
			SelectWorkerFunc: func() (*Worker, error) {
				return &Worker{Address: "localhost:50051", Healthy: true}, nil
			},
		}
		server.workerPool = mockPool

		req := httptest.NewRequest("GET", "/readyz", nil)
		resp, err := app.Test(req)
		require.NoError(t, err)
		assert.Equal(t, 200, resp.StatusCode, "/readyz should return 200 when ready")

		var result map[string]interface{}
		err = json.NewDecoder(resp.Body).Decode(&result)
		require.NoError(t, err)
		assert.Equal(t, "ready", result["status"], "/readyz should return ready status")
	})

	t.Run("healthz_matches_health_response", func(t *testing.T) {
		req1 := httptest.NewRequest("GET", "/health", nil)
		resp1, err := app.Test(req1)
		require.NoError(t, err)

		req2 := httptest.NewRequest("GET", "/healthz", nil)
		resp2, err := app.Test(req2)
		require.NoError(t, err)

		var result1, result2 map[string]interface{}
		json.NewDecoder(resp1.Body).Decode(&result1)
		json.NewDecoder(resp2.Body).Decode(&result2)

		assert.Equal(t, result1["status"], result2["status"], "/healthz should match /health")
	})

	t.Run("livez_matches_live_response", func(t *testing.T) {
		req1 := httptest.NewRequest("GET", "/live", nil)
		resp1, err := app.Test(req1)
		require.NoError(t, err)

		req2 := httptest.NewRequest("GET", "/livez", nil)
		resp2, err := app.Test(req2)
		require.NoError(t, err)

		var result1, result2 map[string]interface{}
		json.NewDecoder(resp1.Body).Decode(&result1)
		json.NewDecoder(resp2.Body).Decode(&result2)

		assert.Equal(t, result1["status"], result2["status"], "/livez should match /live")
	})

	t.Run("readyz_matches_ready_response", func(t *testing.T) {
		mockPool := &MockWorkerPool{
			SelectWorkerFunc: func() (*Worker, error) {
				return &Worker{Address: "localhost:50051", Healthy: true}, nil
			},
		}
		server.workerPool = mockPool

		req1 := httptest.NewRequest("GET", "/ready", nil)
		resp1, err := app.Test(req1)
		require.NoError(t, err)

		req2 := httptest.NewRequest("GET", "/readyz", nil)
		resp2, err := app.Test(req2)
		require.NoError(t, err)

		var result1, result2 map[string]interface{}
		json.NewDecoder(resp1.Body).Decode(&result1)
		json.NewDecoder(resp2.Body).Decode(&result2)

		assert.Equal(t, result1["status"], result2["status"], "/readyz should match /ready")
	})
}
