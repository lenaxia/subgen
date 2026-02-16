package webhooks

import (
	"encoding/json"
	"io"
	"net/http/httptest"
	"net/url"
	"strings"
	"testing"

	"github.com/gofiber/fiber/v2"
	"github.com/mccloud/subgen/orchestrator/internal/config"
	"github.com/mccloud/subgen/orchestrator/internal/queue"
	"github.com/sirupsen/logrus"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// MockQueue implements QueueInterface for testing
type MockQueue struct {
	tasks []Task
	err   error
}

func (m *MockQueue) Enqueue(task Task) error {
	if m.err != nil {
		return m.err
	}
	m.tasks = append(m.tasks, task)
	return nil
}

func (m *MockQueue) Size() int {
	return len(m.tasks)
}

func (m *MockQueue) ProcessingCount() int {
	return 0
}

func (m *MockQueue) IsIdle() bool {
	return len(m.tasks) == 0
}

func (m *MockQueue) GetTaskInfo(taskID string) *queue.TaskInfo {
	return nil
}

func (m *MockQueue) GetAllProcessingTaskInfo() []queue.TaskInfo {
	return []queue.TaskInfo{}
}

func (m *MockQueue) GetHistory(limit, offset int) []queue.TaskInfo {
	return []queue.TaskInfo{}
}

func (m *MockQueue) GetHistoryTotal() int {
	return 0
}

func (m *MockQueue) GetTasks() []Task {
	return m.tasks
}

func (m *MockQueue) Reset() {
	m.tasks = nil
}

// Helper to create test server
func createTestServer(t *testing.T) (*Server, *MockQueue) {
	cfg := &config.Config{
		WebhookPort:        9000,
		ProcessAddedMedia:  true,
		ProcessMediaOnPlay: true,
		Queue: config.QueueConfig{
			MaxSize:             1000,
			MaxAudioContentSize: 100 * 1024 * 1024, // 100MB
		},
	}
	queue := &MockQueue{}
	log := logrus.New()
	log.SetOutput(io.Discard) // Suppress logs in tests

	server := NewServer(cfg, queue, log)
	require.NotNil(t, server)

	return server, queue
}

func TestNewServer(t *testing.T) {
	cfg := &config.Config{WebhookPort: 9000}
	queue := &MockQueue{}
	log := logrus.New()

	server := NewServer(cfg, queue, log)

	assert.NotNil(t, server)
	assert.NotNil(t, server.app)
	assert.Equal(t, cfg, server.config)
	assert.Equal(t, queue, server.queue)
}

func TestHandleGetError(t *testing.T) {
	server, _ := createTestServer(t)

	endpoints := []string{"/plex", "/webhook", "/jellyfin", "/emby", "/tautulli", "/asr"}

	for _, endpoint := range endpoints {
		t.Run(endpoint, func(t *testing.T) {
			req := httptest.NewRequest("GET", endpoint, nil)
			resp, err := server.app.Test(req)
			require.NoError(t, err)
			defer resp.Body.Close()

			assert.Equal(t, fiber.StatusBadRequest, resp.StatusCode)

			var result map[string]string
			err = json.NewDecoder(resp.Body).Decode(&result)
			require.NoError(t, err)

			assert.Contains(t, result["error"], "GET request")
			assert.Contains(t, result["error"], "github.com/McCloudS/subgen")
		})
	}
}

func TestHandleRoot(t *testing.T) {
	server, _ := createTestServer(t)

	req := httptest.NewRequest("GET", "/", nil)
	resp, err := server.app.Test(req)
	require.NoError(t, err)
	defer resp.Body.Close()

	assert.Equal(t, fiber.StatusOK, resp.StatusCode)

	var result map[string]string
	err = json.NewDecoder(resp.Body).Decode(&result)
	require.NoError(t, err)

	assert.Contains(t, result["message"], "webui for configuration was removed")
	assert.Contains(t, result["message"], "environment variables")
}

func TestHandleStatus(t *testing.T) {
	server, _ := createTestServer(t)

	req := httptest.NewRequest("GET", "/status", nil)
	resp, err := server.app.Test(req)
	require.NoError(t, err)
	defer resp.Body.Close()

	assert.Equal(t, fiber.StatusOK, resp.StatusCode)

	var result map[string]string
	err = json.NewDecoder(resp.Body).Decode(&result)
	require.NoError(t, err)

	assert.Contains(t, result["version"], "Subgen")
	assert.Equal(t, "operational", result["status"])
}

// Plex handler tests
func TestHandlePlex_Success(t *testing.T) {
	server, queue := createTestServer(t)

	payload := map[string]interface{}{
		"event": "library.new",
		"Metadata": map[string]interface{}{
			"ratingKey": "12345",
		},
	}
	payloadJSON, _ := json.Marshal(payload)

	form := url.Values{}
	form.Add("payload", string(payloadJSON))

	req := httptest.NewRequest("POST", "/plex", strings.NewReader(form.Encode()))
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	req.Header.Set("User-Agent", "PlexMediaServer/1.0")

	resp, err := server.app.Test(req)
	require.NoError(t, err)
	defer resp.Body.Close()

	assert.Equal(t, fiber.StatusOK, resp.StatusCode)

	// Verify task was queued
	tasks := queue.GetTasks()
	assert.Len(t, tasks, 1)
	assert.Equal(t, "12345", tasks[0].PlexItemID)
}

func TestHandlePlex_MissingUserAgent(t *testing.T) {
	server, queue := createTestServer(t)

	payload := map[string]interface{}{
		"event": "library.new",
	}
	payloadJSON, _ := json.Marshal(payload)

	form := url.Values{}
	form.Add("payload", string(payloadJSON))

	req := httptest.NewRequest("POST", "/plex", strings.NewReader(form.Encode()))
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")

	resp, err := server.app.Test(req)
	require.NoError(t, err)
	defer resp.Body.Close()

	assert.Equal(t, fiber.StatusBadRequest, resp.StatusCode)

	// Verify no task was queued
	tasks := queue.GetTasks()
	assert.Len(t, tasks, 0)

	var result map[string]string
	err = json.NewDecoder(resp.Body).Decode(&result)
	require.NoError(t, err)
	assert.Contains(t, result["error"], "Plex webhook")
}

func TestHandlePlex_MalformedJSON(t *testing.T) {
	server, queue := createTestServer(t)

	form := url.Values{}
	form.Add("payload", "{invalid json")

	req := httptest.NewRequest("POST", "/plex", strings.NewReader(form.Encode()))
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	req.Header.Set("User-Agent", "PlexMediaServer/1.0")

	resp, err := server.app.Test(req)
	require.NoError(t, err)
	defer resp.Body.Close()

	assert.Equal(t, fiber.StatusBadRequest, resp.StatusCode)

	// Verify no task was queued
	tasks := queue.GetTasks()
	assert.Len(t, tasks, 0)
}

func TestHandlePlex_EventFiltering(t *testing.T) {
	cfg := &config.Config{
		WebhookPort:        9000,
		ProcessAddedMedia:  false, // Disabled
		ProcessMediaOnPlay: false, // Disabled
	}
	queue := &MockQueue{}
	log := logrus.New()
	log.SetOutput(io.Discard)
	server := NewServer(cfg, queue, log)

	payload := map[string]interface{}{
		"event": "library.new",
		"Metadata": map[string]interface{}{
			"ratingKey": "12345",
		},
	}
	payloadJSON, _ := json.Marshal(payload)

	form := url.Values{}
	form.Add("payload", string(payloadJSON))

	req := httptest.NewRequest("POST", "/plex", strings.NewReader(form.Encode()))
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	req.Header.Set("User-Agent", "PlexMediaServer/1.0")

	resp, err := server.app.Test(req)
	require.NoError(t, err)
	defer resp.Body.Close()

	assert.Equal(t, fiber.StatusOK, resp.StatusCode)

	// Verify no task was queued (filtered out)
	tasks := queue.GetTasks()
	assert.Len(t, tasks, 0)
}

// Jellyfin handler tests
func TestHandleJellyfin_Success(t *testing.T) {
	server, queue := createTestServer(t)

	form := url.Values{}
	form.Add("NotificationType", "ItemAdded")
	form.Add("ItemId", "abc123")

	req := httptest.NewRequest("POST", "/jellyfin", strings.NewReader(form.Encode()))
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	req.Header.Set("User-Agent", "Jellyfin-Server/10.8.0")

	resp, err := server.app.Test(req)
	require.NoError(t, err)
	defer resp.Body.Close()

	assert.Equal(t, fiber.StatusOK, resp.StatusCode)

	// Verify task was queued
	tasks := queue.GetTasks()
	assert.Len(t, tasks, 1)
	assert.Equal(t, "abc123", tasks[0].JellyfinItemID)
}

func TestHandleJellyfin_MissingUserAgent(t *testing.T) {
	server, queue := createTestServer(t)

	form := url.Values{}
	form.Add("NotificationType", "ItemAdded")

	req := httptest.NewRequest("POST", "/jellyfin", strings.NewReader(form.Encode()))
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")

	resp, err := server.app.Test(req)
	require.NoError(t, err)
	defer resp.Body.Close()

	assert.Equal(t, fiber.StatusBadRequest, resp.StatusCode)

	// Verify no task was queued
	tasks := queue.GetTasks()
	assert.Len(t, tasks, 0)
}

func TestHandleJellyfin_MissingItemId(t *testing.T) {
	server, queue := createTestServer(t)

	form := url.Values{}
	form.Add("NotificationType", "ItemAdded")

	req := httptest.NewRequest("POST", "/jellyfin", strings.NewReader(form.Encode()))
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	req.Header.Set("User-Agent", "Jellyfin-Server/10.8.0")

	resp, err := server.app.Test(req)
	require.NoError(t, err)
	defer resp.Body.Close()

	assert.Equal(t, fiber.StatusBadRequest, resp.StatusCode)

	// Verify no task was queued
	tasks := queue.GetTasks()
	assert.Len(t, tasks, 0)
}

// Emby handler tests
func TestHandleEmby_Success(t *testing.T) {
	server, queue := createTestServer(t)

	data := map[string]interface{}{
		"Event": "library.new",
		"Item": map[string]interface{}{
			"Path": "/media/movies/test.mkv",
		},
	}
	dataJSON, _ := json.Marshal(data)

	form := url.Values{}
	form.Add("data", string(dataJSON))

	req := httptest.NewRequest("POST", "/emby", strings.NewReader(form.Encode()))
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")

	resp, err := server.app.Test(req)
	require.NoError(t, err)
	defer resp.Body.Close()

	assert.Equal(t, fiber.StatusOK, resp.StatusCode)

	// Verify task was queued
	tasks := queue.GetTasks()
	assert.Len(t, tasks, 1)
	assert.Equal(t, "/media/movies/test.mkv", tasks[0].FilePath)
}

func TestHandleEmby_TestNotification(t *testing.T) {
	server, queue := createTestServer(t)

	data := map[string]interface{}{
		"Event": "system.notificationtest",
	}
	dataJSON, _ := json.Marshal(data)

	form := url.Values{}
	form.Add("data", string(dataJSON))

	req := httptest.NewRequest("POST", "/emby", strings.NewReader(form.Encode()))
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")

	resp, err := server.app.Test(req)
	require.NoError(t, err)
	defer resp.Body.Close()

	assert.Equal(t, fiber.StatusOK, resp.StatusCode)

	// Verify no task was queued (test notification)
	tasks := queue.GetTasks()
	assert.Len(t, tasks, 0)

	var result map[string]string
	err = json.NewDecoder(resp.Body).Decode(&result)
	require.NoError(t, err)
	assert.Contains(t, result["message"], "test")
}

func TestHandleEmby_EmptyData(t *testing.T) {
	server, _ := createTestServer(t)

	req := httptest.NewRequest("POST", "/emby", nil)
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")

	resp, err := server.app.Test(req)
	require.NoError(t, err)
	defer resp.Body.Close()

	assert.Equal(t, fiber.StatusOK, resp.StatusCode)
}

// Tautulli handler tests
func TestHandleTautulli_Success(t *testing.T) {
	server, queue := createTestServer(t)

	form := url.Values{}
	form.Add("event", "added")
	form.Add("file", "/media/movies/test.mkv")

	req := httptest.NewRequest("POST", "/tautulli", strings.NewReader(form.Encode()))
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	req.Header.Set("source", "Tautulli")

	resp, err := server.app.Test(req)
	require.NoError(t, err)
	defer resp.Body.Close()

	assert.Equal(t, fiber.StatusOK, resp.StatusCode)

	// Verify task was queued
	tasks := queue.GetTasks()
	assert.Len(t, tasks, 1)
	assert.Equal(t, "/media/movies/test.mkv", tasks[0].FilePath)
}

func TestHandleTautulli_MissingSource(t *testing.T) {
	server, queue := createTestServer(t)

	form := url.Values{}
	form.Add("event", "added")
	form.Add("file", "/media/movies/test.mkv")

	req := httptest.NewRequest("POST", "/tautulli", strings.NewReader(form.Encode()))
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")

	resp, err := server.app.Test(req)
	require.NoError(t, err)
	defer resp.Body.Close()

	assert.Equal(t, fiber.StatusBadRequest, resp.StatusCode)

	// Verify no task was queued
	tasks := queue.GetTasks()
	assert.Len(t, tasks, 0)
}

func TestHandleTautulli_EventFiltering(t *testing.T) {
	cfg := &config.Config{
		WebhookPort:        9000,
		ProcessAddedMedia:  false, // Disabled
		ProcessMediaOnPlay: false, // Disabled
	}
	queue := &MockQueue{}
	log := logrus.New()
	log.SetOutput(io.Discard)
	server := NewServer(cfg, queue, log)

	form := url.Values{}
	form.Add("event", "added")
	form.Add("file", "/media/movies/test.mkv")

	req := httptest.NewRequest("POST", "/tautulli", strings.NewReader(form.Encode()))
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	req.Header.Set("source", "Tautulli")

	resp, err := server.app.Test(req)
	require.NoError(t, err)
	defer resp.Body.Close()

	assert.Equal(t, fiber.StatusOK, resp.StatusCode)

	// Verify no task was queued (filtered out)
	tasks := queue.GetTasks()
	assert.Len(t, tasks, 0)
}
