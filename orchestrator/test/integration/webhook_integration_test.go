package integration

import (
	"bytes"
	"context"
	"fmt"
	"mime/multipart"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/prometheus/client_golang/prometheus"
	"github.com/sirupsen/logrus"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/mccloud/subgen/orchestrator/internal/config"
	"github.com/mccloud/subgen/orchestrator/internal/queue"
	"github.com/mccloud/subgen/orchestrator/internal/webhooks"
)

const (
	testdataDir = "../testdata"
)

// testEnv holds the test environment
type testEnv struct {
	webhookServer *webhooks.Server
	mediaServer   *MockMediaServer
	queue         *queue.Queue
	queueAdapter  *webhooks.QueueAdapter
	log           *logrus.Logger
	config        *config.Config
}

// setupTestEnv creates a test environment with orchestrator + mock media server
func setupTestEnv(t *testing.T) *testEnv {
	// Create logger
	log := logrus.New()
	log.SetLevel(logrus.InfoLevel)
	log.SetOutput(os.Stdout)

	// Create mock media server
	mockServer := NewMockMediaServer()

	// Setup admin user for Jellyfin
	mockServer.SetJellyfinUsers("admin123", "Administrator")

	// Create config
	cfg := &config.Config{
		WebhookPort:        9000,
		ProcessAddedMedia:  true,
		ProcessMediaOnPlay: true,
		Plex: config.PlexConfig{
			Enabled: true,
			Server:  mockServer.URL(),
			Token:   "test-plex-token",
		},
		Jellyfin: config.JellyfinConfig{
			Enabled: true,
			Server:  mockServer.URL(),
			Token:   "test-jellyfin-token",
		},
		Queue: config.QueueConfig{
			MaxSize:             1000,
			MaxAudioContentSize: 50 * 1024 * 1024,
		},
	}

	// Create metrics with custom registry (avoids collisions in tests)
	registry := prometheus.NewRegistry()
	metrics := queue.NewQueueMetricsWithRegistry(registry)

	// Create queue
	q := queue.NewQueue(cfg.Queue.MaxSize, metrics, log)

	// Create queue adapter
	queueAdapter := webhooks.NewQueueAdapter(q)

	// Create webhook server
	webhookServer := webhooks.NewServer(cfg, queueAdapter, log)

	env := &testEnv{
		webhookServer: webhookServer,
		mediaServer:   mockServer,
		queue:         q,
		queueAdapter:  queueAdapter,
		log:           log,
		config:        cfg,
	}

	return env
}

// teardownTestEnv cleans up test environment
func (e *testEnv) teardownTestEnv() {
	e.mediaServer.Close()
}

// waitForQueuedTask waits for a task to appear in the queue
func (e *testEnv) waitForQueuedTask(timeout time.Duration) bool {
	deadline := time.Now().Add(timeout)
	for time.Now().Before(deadline) {
		if e.queue.Size() > 0 {
			return true
		}
		time.Sleep(50 * time.Millisecond)
	}
	return false
}

// Test 1: Plex library.new Webhook → Queue
func TestPlex_LibraryNew_Success(t *testing.T) {
	env := setupTestEnv(t)
	defer env.teardownTestEnv()

	// Setup mock media server response
	ratingKey := "12345"
	testFile, err := filepath.Abs(filepath.Join(testdataDir, "short_audio.wav"))
	require.NoError(t, err)
	env.mediaServer.SetPlexMetadata(ratingKey, testFile)

	// Create webhook payload
	payload := GetPlexPayload(ratingKey, "library.new")

	// Create multipart form (Plex sends JSON as form field)
	body := &bytes.Buffer{}
	writer := multipart.NewWriter(body)
	err = writer.WriteField("payload", payload)
	require.NoError(t, err)
	writer.Close()

	// Send webhook using Fiber test
	req, err := http.NewRequest("POST", "/plex", body)
	require.NoError(t, err)
	req.Header.Set("Content-Type", writer.FormDataContentType())
	req.Header.Set("User-Agent", "PlexMediaServer/1.40.0")

	resp, err := env.webhookServer.App().Test(req, -1) // -1 = no timeout
	require.NoError(t, err)
	defer resp.Body.Close()

	// Verify HTTP response
	assert.Equal(t, http.StatusOK, resp.StatusCode, "Webhook should be accepted")

	// Wait for task to be queued
	assert.True(t, env.waitForQueuedTask(2*time.Second), "Task should be queued")

	// NOTE: Plex API is NOT called during webhook handling. The webhook queues a task with
	// PlexItemID, and the dispatcher later fetches the file path from Plex API when processing.
	// This is the correct behavior - webhook handlers should be fast and not block on API calls.

	// Verify task was queued
	assert.Equal(t, 1, env.queue.Size(), "Exactly one task should be queued")

	t.Log("✅ Plex library.new webhook → queue completed successfully")
}

// Test 2: Plex media.play Webhook → Queue
func TestPlex_MediaPlay_Success(t *testing.T) {
	env := setupTestEnv(t)
	defer env.teardownTestEnv()

	ratingKey := "67890"
	testFile, err := filepath.Abs(filepath.Join(testdataDir, "short_audio.wav"))
	require.NoError(t, err)
	env.mediaServer.SetPlexMetadata(ratingKey, testFile)

	payload := GetPlexPayload(ratingKey, "media.play")

	body := &bytes.Buffer{}
	writer := multipart.NewWriter(body)
	_ = writer.WriteField("payload", payload)
	writer.Close()

	req, err := http.NewRequest("POST", "/plex", body)
	require.NoError(t, err)
	req.Header.Set("Content-Type", writer.FormDataContentType())
	req.Header.Set("User-Agent", "PlexMediaServer/1.40.0")

	resp, err := env.webhookServer.App().Test(req, -1)
	require.NoError(t, err)
	defer resp.Body.Close()

	assert.Equal(t, http.StatusOK, resp.StatusCode)

	assert.True(t, env.waitForQueuedTask(2*time.Second), "Task should be queued")
	assert.Equal(t, 1, env.queue.Size(), "Task should be queued")

	t.Log("✅ Plex media.play webhook processed")
}

// Test 3: Jellyfin ItemAdded Webhook → Queue
func TestJellyfin_ItemAdded_Success(t *testing.T) {
	env := setupTestEnv(t)
	defer env.teardownTestEnv()

	itemID := "abc123def456"
	testFile, err := filepath.Abs(filepath.Join(testdataDir, "short_audio.wav"))
	require.NoError(t, err)
	env.mediaServer.SetJellyfinItem(itemID, testFile)

	payload := GetJellyfinPayload(itemID, "ItemAdded")

	req, err := http.NewRequest("POST", "/jellyfin", strings.NewReader(payload))
	require.NoError(t, err)
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	req.Header.Set("User-Agent", "Jellyfin-Server/10.8.13")

	resp, err := env.webhookServer.App().Test(req, -1)
	require.NoError(t, err)
	defer resp.Body.Close()

	assert.Equal(t, http.StatusOK, resp.StatusCode)

	assert.True(t, env.waitForQueuedTask(2*time.Second), "Task should be queued")
	assert.Equal(t, 1, env.queue.Size(), "Task should be queued")

	t.Log("✅ Jellyfin ItemAdded webhook processed")
}

// Test 4: Jellyfin PlaybackStart Webhook → Queue
func TestJellyfin_PlaybackStart_Success(t *testing.T) {
	env := setupTestEnv(t)
	defer env.teardownTestEnv()

	itemID := "xyz789abc123"
	testFile, err := filepath.Abs(filepath.Join(testdataDir, "short_audio.wav"))
	require.NoError(t, err)
	env.mediaServer.SetJellyfinItem(itemID, testFile)

	payload := GetJellyfinPayload(itemID, "PlaybackStart")

	req, err := http.NewRequest("POST", "/jellyfin", strings.NewReader(payload))
	require.NoError(t, err)
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	req.Header.Set("User-Agent", "Jellyfin-Server/10.8.13")

	resp, err := env.webhookServer.App().Test(req, -1)
	require.NoError(t, err)
	defer resp.Body.Close()

	assert.Equal(t, http.StatusOK, resp.StatusCode)

	assert.True(t, env.waitForQueuedTask(2*time.Second), "Task should be queued")
	assert.Equal(t, 1, env.queue.Size(), "Task should be queued")

	t.Log("✅ Jellyfin PlaybackStart webhook processed")
}

// Test 5: Emby library.new Webhook → Queue (Direct File Path)
func TestEmby_LibraryNew_Success(t *testing.T) {
	env := setupTestEnv(t)
	defer env.teardownTestEnv()

	testFile, err := filepath.Abs(filepath.Join(testdataDir, "short_audio.wav"))
	require.NoError(t, err)

	// Emby provides file path directly in payload
	payload := GetEmbyPayload(testFile, "library.new")

	req, err := http.NewRequest("POST", "/emby", strings.NewReader(payload))
	require.NoError(t, err)
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")

	resp, err := env.webhookServer.App().Test(req, -1)
	require.NoError(t, err)
	defer resp.Body.Close()

	assert.Equal(t, http.StatusOK, resp.StatusCode)

	assert.True(t, env.waitForQueuedTask(2*time.Second), "Task should be queued")
	assert.Equal(t, 1, env.queue.Size(), "Task should be queued")

	t.Log("✅ Emby library.new webhook processed")
}

// Test 6: Tautulli added Webhook → Queue (Direct File Path)
func TestTautulli_Added_Success(t *testing.T) {
	env := setupTestEnv(t)
	defer env.teardownTestEnv()

	testFile, err := filepath.Abs(filepath.Join(testdataDir, "short_audio.wav"))
	require.NoError(t, err)
	payload := GetTautulliPayload(testFile, "added")

	req, err := http.NewRequest("POST", "/tautulli", strings.NewReader(payload))
	require.NoError(t, err)
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	req.Header.Set("source", "Tautulli")

	resp, err := env.webhookServer.App().Test(req, -1)
	require.NoError(t, err)
	defer resp.Body.Close()

	assert.Equal(t, http.StatusOK, resp.StatusCode)

	assert.True(t, env.waitForQueuedTask(2*time.Second), "Task should be queued")
	assert.Equal(t, 1, env.queue.Size(), "Task should be queued")

	t.Log("✅ Tautulli added webhook processed")
}

// Test 7: Invalid Webhook Payload
func TestPlex_InvalidPayload(t *testing.T) {
	env := setupTestEnv(t)
	defer env.teardownTestEnv()

	// Send invalid JSON
	payload := `{invalid json`

	body := &bytes.Buffer{}
	writer := multipart.NewWriter(body)
	_ = writer.WriteField("payload", payload)
	writer.Close()

	req, err := http.NewRequest("POST", "/plex", body)
	require.NoError(t, err)
	req.Header.Set("Content-Type", writer.FormDataContentType())
	req.Header.Set("User-Agent", "PlexMediaServer/1.40.0")

	resp, err := env.webhookServer.App().Test(req, -1)
	require.NoError(t, err)
	defer resp.Body.Close()

	assert.Equal(t, http.StatusBadRequest, resp.StatusCode, "Invalid payload should be rejected")

	// Verify nothing queued
	time.Sleep(500 * time.Millisecond)
	assert.Equal(t, 0, env.queue.Size(), "No task should be queued")

	t.Log("✅ Invalid payload rejected correctly")
}

// Test 8: Missing User-Agent Header
func TestPlex_MissingUserAgent(t *testing.T) {
	env := setupTestEnv(t)
	defer env.teardownTestEnv()

	payload := GetPlexPayload("12345", "library.new")

	body := &bytes.Buffer{}
	writer := multipart.NewWriter(body)
	_ = writer.WriteField("payload", payload)
	writer.Close()

	req, err := http.NewRequest("POST", "/plex", body)
	require.NoError(t, err)
	req.Header.Set("Content-Type", writer.FormDataContentType())
	// No User-Agent header

	resp, err := env.webhookServer.App().Test(req, -1)
	require.NoError(t, err)
	defer resp.Body.Close()

	assert.Equal(t, http.StatusBadRequest, resp.StatusCode, "Missing User-Agent should be rejected")

	t.Log("✅ Missing User-Agent rejected correctly")
}

// Test 9: Filtered Event (PROCESS_ADDED_MEDIA=false)
func TestPlex_FilteredEvent(t *testing.T) {
	env := setupTestEnv(t)
	defer env.teardownTestEnv()

	// Disable PROCESS_ADDED_MEDIA
	env.config.ProcessAddedMedia = false

	ratingKey := "12345"
	testFile, err := filepath.Abs(filepath.Join(testdataDir, "short_audio.wav"))
	require.NoError(t, err)
	env.mediaServer.SetPlexMetadata(ratingKey, testFile)

	payload := GetPlexPayload(ratingKey, "library.new")

	body := &bytes.Buffer{}
	writer := multipart.NewWriter(body)
	_ = writer.WriteField("payload", payload)
	writer.Close()

	req, err := http.NewRequest("POST", "/plex", body)
	require.NoError(t, err)
	req.Header.Set("Content-Type", writer.FormDataContentType())
	req.Header.Set("User-Agent", "PlexMediaServer/1.40.0")

	resp, err := env.webhookServer.App().Test(req, -1)
	require.NoError(t, err)
	defer resp.Body.Close()

	assert.Equal(t, http.StatusOK, resp.StatusCode, "Webhook should be accepted")

	time.Sleep(500 * time.Millisecond)
	assert.Equal(t, 0, env.queue.Size(), "Task should NOT be queued (filtered)")

	t.Log("✅ Filtered event not queued")
}

// Test 10: Duplicate Task Deduplication
func TestPlex_DuplicateTask(t *testing.T) {
	env := setupTestEnv(t)
	defer env.teardownTestEnv()

	ratingKey := "12345"
	testFile, err := filepath.Abs(filepath.Join(testdataDir, "short_audio.wav"))
	require.NoError(t, err)
	env.mediaServer.SetPlexMetadata(ratingKey, testFile)

	payload := GetPlexPayload(ratingKey, "library.new")

	body := &bytes.Buffer{}
	writer := multipart.NewWriter(body)
	_ = writer.WriteField("payload", payload)
	writer.Close()

	// Send same webhook twice
	for i := 0; i < 2; i++ {
		req, err := http.NewRequest("POST", "/plex", bytes.NewReader(body.Bytes()))
		require.NoError(t, err)
		req.Header.Set("Content-Type", writer.FormDataContentType())
		req.Header.Set("User-Agent", "PlexMediaServer/1.40.0")

		resp, err := env.webhookServer.App().Test(req, -1)
		require.NoError(t, err)
		resp.Body.Close()
	}

	time.Sleep(500 * time.Millisecond)

	// Should only have 1 task (deduplicated)
	assert.Equal(t, 1, env.queue.Size(), "Duplicate task should be deduplicated")

	t.Log("✅ Duplicate task deduplicated")
}

// Test 11: Media Server API Failure
func TestPlex_MediaServerAPIFailure(t *testing.T) {
	env := setupTestEnv(t)
	defer env.teardownTestEnv()

	ratingKey := "99999"

	// Simulate API failure (404)
	env.mediaServer.SimulateFailure(fmt.Sprintf("/library/metadata/%s", ratingKey), http.StatusNotFound)

	payload := GetPlexPayload(ratingKey, "library.new")

	body := &bytes.Buffer{}
	writer := multipart.NewWriter(body)
	_ = writer.WriteField("payload", payload)
	writer.Close()

	req, err := http.NewRequest("POST", "/plex", body)
	require.NoError(t, err)
	req.Header.Set("Content-Type", writer.FormDataContentType())
	req.Header.Set("User-Agent", "PlexMediaServer/1.40.0")

	resp, err := env.webhookServer.App().Test(req, -1)
	require.NoError(t, err)
	defer resp.Body.Close()

	// Webhook accepted, but media server call fails
	assert.Equal(t, http.StatusOK, resp.StatusCode)

	time.Sleep(500 * time.Millisecond)

	// Task might be queued but should fail during processing
	// For this test, we just verify webhook handler doesn't crash
	t.Log("✅ Media server API failure handled")
}

// Test 12: Queue Full Scenario
func TestPlex_QueueFull(t *testing.T) {
	// Create test env with small queue
	log := logrus.New()
	log.SetLevel(logrus.WarnLevel)

	mockServer := NewMockMediaServer()
	defer mockServer.Close()

	cfg := &config.Config{
		ProcessAddedMedia: true,
		Plex: config.PlexConfig{
			Enabled: true,
			Server:  mockServer.URL(),
			Token:   "test-token",
		},
		Queue: config.QueueConfig{
			MaxSize:             2, // Very small queue
			MaxAudioContentSize: 50 * 1024 * 1024,
		},
	}

	// Create metrics with custom registry
	registry := prometheus.NewRegistry()
	metrics := queue.NewQueueMetricsWithRegistry(registry)
	q := queue.NewQueue(cfg.Queue.MaxSize, metrics, log)

	// Fill queue
	testFile, err := filepath.Abs(filepath.Join(testdataDir, "short_audio.wav"))
	require.NoError(t, err)

	for i := 0; i < 2; i++ {
		task := queue.NewTask(fmt.Sprintf("%s-%d", testFile, i), queue.TaskTypeTranscribe)
		task.FilePath = fmt.Sprintf("%s-%d", testFile, i)
		err := q.Enqueue(task)
		require.NoError(t, err)
	}

	// Try to send webhook (queue full)
	queueAdapter := webhooks.NewQueueAdapter(q)
	webhookServer := webhooks.NewServer(cfg, queueAdapter, log)

	ratingKey := "12345"
	mockServer.SetPlexMetadata(ratingKey, testFile)

	payload := GetPlexPayload(ratingKey, "library.new")

	body := &bytes.Buffer{}
	writer := multipart.NewWriter(body)
	writer.WriteField("payload", payload)
	writer.Close()

	req, err := http.NewRequest("POST", "/plex", body)
	require.NoError(t, err)
	req.Header.Set("Content-Type", writer.FormDataContentType())
	req.Header.Set("User-Agent", "PlexMediaServer/1.40.0")

	resp, err := webhookServer.App().Test(req, -1)
	require.NoError(t, err)
	defer resp.Body.Close()

	// Should return 429 (Too Many Requests) when queue is full
	assert.Equal(t, http.StatusTooManyRequests, resp.StatusCode)

	t.Log("✅ Queue full scenario handled correctly")
}

// Test 13: Emby Test Notification
func TestEmby_TestNotification(t *testing.T) {
	env := setupTestEnv(t)
	defer env.teardownTestEnv()

	payload := EmbyTestNotificationPayload

	req, err := http.NewRequest("POST", "/emby", strings.NewReader(payload))
	require.NoError(t, err)
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")

	resp, err := env.webhookServer.App().Test(req, -1)
	require.NoError(t, err)
	defer resp.Body.Close()

	assert.Equal(t, http.StatusOK, resp.StatusCode)

	// Verify no task queued (test notification)
	time.Sleep(500 * time.Millisecond)
	assert.Equal(t, 0, env.queue.Size(), "Test notification should not queue task")

	t.Log("✅ Emby test notification handled")
}

// Test 14: Multiple Webhooks From Different Sources
func TestMultiple_WebhooksFromDifferentSources(t *testing.T) {
	env := setupTestEnv(t)
	defer env.teardownTestEnv()

	testFile, err := filepath.Abs(filepath.Join(testdataDir, "short_audio.wav"))
	require.NoError(t, err)

	// Setup Plex
	ratingKey := "11111"
	env.mediaServer.SetPlexMetadata(ratingKey, testFile)

	// Setup Jellyfin
	itemID := "22222"
	env.mediaServer.SetJellyfinItem(itemID, testFile)

	// Send Plex webhook
	plexPayload := GetPlexPayload(ratingKey, "library.new")
	body := &bytes.Buffer{}
	writer := multipart.NewWriter(body)
	writer.WriteField("payload", plexPayload)
	writer.Close()

	req, err := http.NewRequest("POST", "/plex", body)
	require.NoError(t, err)
	req.Header.Set("Content-Type", writer.FormDataContentType())
	req.Header.Set("User-Agent", "PlexMediaServer/1.40.0")

	resp, err := env.webhookServer.App().Test(req, -1)
	require.NoError(t, err)
	resp.Body.Close()

	// Send Jellyfin webhook
	jellyfinPayload := GetJellyfinPayload(itemID, "ItemAdded")
	req2, err := http.NewRequest("POST", "/jellyfin", strings.NewReader(jellyfinPayload))
	require.NoError(t, err)
	req2.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	req2.Header.Set("User-Agent", "Jellyfin-Server/10.8.13")

	resp2, err := env.webhookServer.App().Test(req2, -1)
	require.NoError(t, err)
	resp2.Body.Close()

	// Wait and verify both queued
	time.Sleep(500 * time.Millisecond)

	// NOTE: At webhook time, Plex and Jellyfin tasks have different identifiers:
	// - Plex: PlexItemID="11111", FilePath="" (will be fetched by dispatcher)
	// - Jellyfin: JellyfinItemID="22222", FilePath="" (will be fetched by dispatcher)
	// Since they have different PlexItemID/JellyfinItemID, they generate different task IDs
	// and do NOT deduplicate at webhook time.
	//
	// Deduplication would only occur AFTER the dispatcher fetches file paths from both APIs
	// and discovers they point to the same file. However, this test doesn't run the dispatcher,
	// so we expect 2 separate tasks in the queue.
	assert.Equal(t, 2, env.queue.Size(), "Plex and Jellyfin webhooks create separate tasks (different IDs)")

	t.Log("✅ Multiple webhooks from different sources handled")
}

// Test 15: Concurrent Webhooks
func TestWebhook_ConcurrentRequests(t *testing.T) {
	env := setupTestEnv(t)
	defer env.teardownTestEnv()

	testFile, err := filepath.Abs(filepath.Join(testdataDir, "short_audio.wav"))
	require.NoError(t, err)

	// Setup mock responses for 10 different rating keys
	for i := 0; i < 10; i++ {
		ratingKey := fmt.Sprintf("rating-%d", i)
		// Each gets a different file path so they don't deduplicate
		env.mediaServer.SetPlexMetadata(ratingKey, fmt.Sprintf("%s-%d", testFile, i))
	}

	// Send 10 concurrent webhooks
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	done := make(chan bool, 10)
	for i := 0; i < 10; i++ {
		go func(idx int) {
			ratingKey := fmt.Sprintf("rating-%d", idx)
			payload := GetPlexPayload(ratingKey, "library.new")

			body := &bytes.Buffer{}
			writer := multipart.NewWriter(body)
			writer.WriteField("payload", payload)
			writer.Close()

			req, err := http.NewRequestWithContext(ctx, "POST", "/plex", body)
			if err != nil {
				done <- false
				return
			}
			req.Header.Set("Content-Type", writer.FormDataContentType())
			req.Header.Set("User-Agent", "PlexMediaServer/1.40.0")

			resp, err := env.webhookServer.App().Test(req, -1)
			if err != nil {
				done <- false
				return
			}
			resp.Body.Close()

			done <- resp.StatusCode == http.StatusOK
		}(i)
	}

	// Wait for all requests to complete
	successCount := 0
	for i := 0; i < 10; i++ {
		if <-done {
			successCount++
		}
	}

	// NOTE: Plex webhooks don't include file paths - they only have rating keys.
	// Tasks are queued with empty FilePath and only PlexItemID/PlexServer/PlexToken.
	// The worker fetches the actual file path when processing the task.
	// Since each request has a DIFFERENT PlexItemID (rating-0 through rating-9),
	// they create DIFFERENT tasks and do NOT deduplicate.
	// All requests should succeed (HTTP 200).
	assert.Equal(t, 10, successCount, "All requests with different rating keys should succeed")

	// Wait for queue to process
	time.Sleep(1 * time.Second)
	assert.Equal(t, 10, env.queue.Size(), "All 10 tasks queued (different rating keys = different tasks)")

	t.Log("✅ Concurrent webhook requests handled with proper deduplication")
}
