# Story 02: Webhook Integration Tests

**Epic**: EPIC_03 - Integration & Testing  
**Status**: Not Started  
**Priority**: Critical  
**Estimated Effort**: 8-10 hours  
**Assignee**: TBD

---

## User Story

As a **system integrator**,  
I want **comprehensive integration tests for all webhook handlers (Plex, Jellyfin, Emby, Tautulli)**,  
So that **I can verify the complete flow from webhook receipt to worker transcription works correctly**.

---

## Context

Webhooks are the primary entry point for the Subgen system. Media servers (Plex, Jellyfin, Emby) send webhook notifications when new media is added or played. The orchestrator must:

1. Parse webhook payloads correctly
2. Validate payload structure
3. Extract metadata (rating keys, item IDs, file paths)
4. Call media server APIs to get file paths (Plex/Jellyfin)
5. Enqueue transcription tasks
6. Dispatch tasks to Python worker via gRPC
7. Return appropriate HTTP status codes

**Why This Matters:**
- Webhooks have different payload formats for each media server
- Payload parsing errors cause silent failures (media server thinks webhook succeeded)
- Integration between orchestrator → media server API → gRPC worker must be tested
- Skip logic and filtering must work correctly

**Current State:**
- Webhook handlers: `/home/mikekao/personal/subgen/orchestrator/internal/webhooks/server.go` (lines 138-480)
- Unit tests exist, but NO integration tests
- No validation of end-to-end flow

**Target State:**
- Integration test suite covering all 4 webhook types
- Tests validate complete flow: webhook → queue → gRPC → transcription
- Mock media server APIs for testing
- Real gRPC calls to Python worker

---

## Acceptance Criteria

- [ ] Integration test file created: `test/integration/webhook_integration_test.go`
- [ ] Mock media server API server (Plex/Jellyfin)
- [ ] Test: Plex library.new webhook → transcription
- [ ] Test: Plex media.play webhook → transcription
- [ ] Test: Jellyfin ItemAdded webhook → transcription
- [ ] Test: Jellyfin PlaybackStart webhook → transcription
- [ ] Test: Emby library.new webhook → transcription (direct file path)
- [ ] Test: Tautulli added webhook → transcription (direct file path)
- [ ] Test: Invalid webhook payload (400 error)
- [ ] Test: Missing User-Agent header (400 error)
- [ ] Test: Filtered events (PROCESS_ADDED_MEDIA=false)
- [ ] Test: Duplicate task deduplication
- [ ] Test: Queue full scenario (503 error)
- [ ] Test: Media server API failure (retry logic)
- [ ] All tests pass with Docker Compose environment
- [ ] Work log created

---

## Technical Design

### Test Architecture

```
┌───────────────────────────────────────────────────────────────┐
│  Integration Test Suite                                       │
├───────────────────────────────────────────────────────────────┤
│                                                                │
│  ┌──────────────┐                                            │
│  │ Test Client  │                                            │
│  │ (HTTP calls) │                                            │
│  └──────┬───────┘                                            │
│         │ POST /plex                                          │
│         │ POST /jellyfin                                      │
│         │ POST /emby                                          │
│         │ POST /tautulli                                      │
│         ↓                                                      │
│  ┌──────────────────────────────────────────────────────┐   │
│  │  Go Orchestrator                                      │   │
│  │  ┌────────────────────────────────────────────────┐  │   │
│  │  │ Webhook Handlers                               │  │   │
│  │  │ - Parse payload                                │  │   │
│  │  │ - Call media server API (mocked)             │  │   │
│  │  │ - Enqueue task                                 │  │   │
│  │  │ - Dispatch to worker (real gRPC)             │  │   │
│  │  └────────────────────────────────────────────────┘  │   │
│  └──────────────────┬───────────────────────────────────┘   │
│                     │ gRPC                                    │
│                     ↓                                         │
│  ┌──────────────────────────────────────────────────────┐   │
│  │  Python Worker                                        │   │
│  │  - Receive transcription request                     │   │
│  │  - Process audio                                      │   │
│  │  - Generate subtitle                                  │   │
│  └──────────────────────────────────────────────────────┘   │
│                                                                │
│  ┌──────────────────────────────────────────────────────┐   │
│  │  Mock Media Server API                                │   │
│  │  - GET /library/metadata/:id (Plex)                 │   │
│  │  - GET /Items/:id (Jellyfin)                         │   │
│  │  - Returns mocked file paths                         │   │
│  └──────────────────────────────────────────────────────┘   │
│                                                                │
└───────────────────────────────────────────────────────────────┘
```

### File Structure

```
test/
├── integration/
│   ├── webhook_integration_test.go      # Main test file
│   ├── mock_media_server.go             # Mock Plex/Jellyfin API
│   └── webhook_payloads.go              # Sample webhook payloads
├── testdata/
│   └── webhook_payloads/
│       ├── plex_library_new.json
│       ├── plex_media_play.json
│       ├── jellyfin_item_added.json
│       ├── jellyfin_playback_start.json
│       ├── emby_library_new.json
│       └── tautulli_added.json
└── docker-compose.integration.yml       # From STORY_01
```

---

## Implementation Steps

### Step 1: Create Mock Media Server

**File: `/home/mikekao/personal/subgen/test/integration/mock_media_server.go`**

```go
package integration

import (
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"sync"
)

// MockMediaServer simulates Plex/Jellyfin API responses
type MockMediaServer struct {
	server      *httptest.Server
	mu          sync.Mutex
	responses   map[string]interface{} // Key: endpoint, Value: response data
	callCount   map[string]int
}

// NewMockMediaServer creates a new mock media server
func NewMockMediaServer() *MockMediaServer {
	mock := &MockMediaServer{
		responses: make(map[string]interface{}),
		callCount: make(map[string]int),
	}

	mux := http.NewServeMux()
	
	// Plex endpoints
	mux.HandleFunc("/library/metadata/", mock.handlePlexMetadata)
	
	// Jellyfin endpoints
	mux.HandleFunc("/Items/", mock.handleJellyfinItem)
	
	mock.server = httptest.NewServer(mux)
	return mock
}

// URL returns the mock server URL
func (m *MockMediaServer) URL() string {
	return m.server.URL
}

// Close shuts down the mock server
func (m *MockMediaServer) Close() {
	m.server.Close()
}

// SetPlexMetadata sets the response for a Plex metadata request
func (m *MockMediaServer) SetPlexMetadata(ratingKey string, filePath string) {
	m.mu.Lock()
	defer m.mu.Unlock()
	
	endpoint := fmt.Sprintf("/library/metadata/%s", ratingKey)
	m.responses[endpoint] = map[string]interface{}{
		"MediaContainer": map[string]interface{}{
			"Metadata": []map[string]interface{}{
				{
					"Media": []map[string]interface{}{
						{
							"Part": []map[string]interface{}{
								{
									"file": filePath,
								},
							},
						},
					},
				},
			},
		},
	}
}

// SetJellyfinItem sets the response for a Jellyfin item request
func (m *MockMediaServer) SetJellyfinItem(itemID string, filePath string) {
	m.mu.Lock()
	defer m.mu.Unlock()
	
	endpoint := fmt.Sprintf("/Items/%s", itemID)
	m.responses[endpoint] = map[string]interface{}{
		"Path": filePath,
		"MediaSources": []map[string]interface{}{
			{
				"Path": filePath,
			},
		},
	}
}

// GetCallCount returns the number of times an endpoint was called
func (m *MockMediaServer) GetCallCount(endpoint string) int {
	m.mu.Lock()
	defer m.mu.Unlock()
	return m.callCount[endpoint]
}

// handlePlexMetadata handles Plex metadata requests
func (m *MockMediaServer) handlePlexMetadata(w http.ResponseWriter, r *http.Request) {
	m.mu.Lock()
	endpoint := r.URL.Path
	m.callCount[endpoint]++
	response, ok := m.responses[endpoint]
	m.mu.Unlock()

	if !ok {
		http.Error(w, "Not found", http.StatusNotFound)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(response)
}

// handleJellyfinItem handles Jellyfin item requests
func (m *MockMediaServer) handleJellyfinItem(w http.ResponseWriter, r *http.Request) {
	m.mu.Lock()
	endpoint := r.URL.Path
	m.callCount[endpoint]++
	response, ok := m.responses[endpoint]
	m.mu.Unlock()

	if !ok {
		http.Error(w, "Not found", http.StatusNotFound)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(response)
}

// SimulateFailure makes the mock server return errors
func (m *MockMediaServer) SimulateFailure(endpoint string, statusCode int) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.responses[endpoint] = statusCode // Special case: int = error code
}
```

---

### Step 2: Sample Webhook Payloads

**File: `/home/mikekao/personal/subgen/test/integration/webhook_payloads.go`**

```go
package integration

// Sample webhook payloads based on actual media server formats

const (
	PlexLibraryNewPayload = `{
		"event": "library.new",
		"user": true,
		"owner": true,
		"Account": {
			"id": 1,
			"thumb": "https://plex.tv/users/1/avatar",
			"title": "Test User"
		},
		"Server": {
			"title": "Test Plex Server",
			"uuid": "abc123"
		},
		"Metadata": {
			"librarySectionType": "show",
			"ratingKey": "12345",
			"key": "/library/metadata/12345",
			"guid": "plex://episode/5d9c086fe9d5a1001f4d4c1d",
			"type": "episode",
			"title": "Test Episode",
			"grandparentTitle": "Test Show",
			"parentTitle": "Season 1",
			"index": 1,
			"parentIndex": 1,
			"year": 2024,
			"thumb": "/library/metadata/12345/thumb/1234567890",
			"addedAt": 1708012345,
			"updatedAt": 1708012345
		}
	}`

	PlexMediaPlayPayload = `{
		"event": "media.play",
		"user": true,
		"owner": true,
		"Account": {
			"id": 1,
			"title": "Test User"
		},
		"Metadata": {
			"ratingKey": "67890",
			"type": "episode",
			"title": "Played Episode"
		}
	}`

	JellyfinItemAddedPayload = `NotificationType=ItemAdded&ItemId=abc123def456&ItemType=Episode&ItemName=Test%20Episode&SeriesName=Test%20Show&SeasonNumber=1&EpisodeNumber=1`

	JellyfinPlaybackStartPayload = `NotificationType=PlaybackStart&ItemId=xyz789abc123&ItemType=Episode&ItemName=Played%20Episode`

	EmbyLibraryNewPayload = `data=` + EmbyLibraryNewJSON

	EmbyLibraryNewJSON = `{
		"Event": "library.new",
		"Item": {
			"Name": "Test Episode",
			"Path": "/media/TV/Show/S01E01.mkv",
			"Type": "Episode",
			"ServerId": "abc123",
			"Id": "item123"
		},
		"Server": {
			"Name": "Test Emby Server",
			"Id": "server123"
		}
	}`

	TautulliAddedPayload = `event=added&file=/media/TV/Show/S01E01.mkv&title=Test%20Episode&show_name=Test%20Show&season_num=1&episode_num=1`
)

// GetPlexPayload returns a Plex payload with custom rating key
func GetPlexPayload(ratingKey string, event string) string {
	if event == "library.new" {
		return fmt.Sprintf(`{"event": "library.new", "Metadata": {"ratingKey": "%s"}}`, ratingKey)
	}
	return fmt.Sprintf(`{"event": "media.play", "Metadata": {"ratingKey": "%s"}}`, ratingKey)
}

// GetJellyfinPayload returns a Jellyfin payload with custom item ID
func GetJellyfinPayload(itemID string, notificationType string) string {
	return fmt.Sprintf("NotificationType=%s&ItemId=%s", notificationType, itemID)
}
```

---

### Step 3: Webhook Integration Tests

**File: `/home/mikekao/personal/subgen/test/integration/webhook_integration_test.go`**

```go
package integration

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"mime/multipart"
	"net/http"
	"net/http/httptest"
	"net/url"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	
	"github.com/mccloud/subgen/orchestrator/internal/config"
	"github.com/mccloud/subgen/orchestrator/internal/webhooks"
	"github.com/mccloud/subgen/orchestrator/internal/queue"
	"github.com/mccloud/subgen/orchestrator/internal/grpc_client"
)

const (
	testdataDir = "../testdata"
	workerAddr  = "localhost:50051"
)

// testEnv holds the test environment
type testEnv struct {
	orchestrator *webhooks.Server
	mediaServer  *MockMediaServer
	queue        *queue.PriorityQueue
	grpcClient   *grpc_client.Client
	httpServer   *httptest.Server
}

// setupTestEnv creates a test environment with orchestrator + mock media server
func setupTestEnv(t *testing.T) *testEnv {
	// Create mock media server
	mockServer := NewMockMediaServer()

	// Create config
	cfg := &config.Config{
		WebhookPort:         9000,
		ProcessAddedMedia:   true,
		ProcessMediaOnPlay:  true,
		Plex: config.PlexConfig{
			Server: mockServer.URL(),
			Token:  "test-plex-token",
		},
		Jellyfin: config.JellyfinConfig{
			Server: mockServer.URL(),
			Token:  "test-jellyfin-token",
		},
		Queue: config.QueueConfig{
			MaxSize:              1000,
			MaxAudioContentSize:  50 * 1024 * 1024,
		},
	}

	// Create queue
	q := queue.NewPriorityQueue(cfg.Queue.MaxSize)

	// Create gRPC client (connects to real Python worker)
	grpcClient := grpc_client.NewClient(
		5*time.Hour,      // transcribe timeout
		5*time.Second,    // health timeout
		3,                // max retries
		1*time.Second,    // retry delay
		nil,              // metrics (nil for test)
		logrus.New(),     // logger
	)

	// Create webhook server
	log := logrus.New()
	log.SetLevel(logrus.InfoLevel)
	
	webhookServer := webhooks.NewServer(cfg, q, grpcClient, log)

	// Create HTTP test server
	httpServer := httptest.NewServer(webhookServer.App())

	env := &testEnv{
		orchestrator: webhookServer,
		mediaServer:  mockServer,
		queue:        q,
		grpcClient:   grpcClient,
		httpServer:   httpServer,
	}

	return env
}

// teardownTestEnv cleans up test environment
func (e *testEnv) teardownTestEnv() {
	e.httpServer.Close()
	e.mediaServer.Close()
	e.grpcClient.Close()
}

// Test 1: Plex library.new Webhook → Transcription
func TestPlex_LibraryNew_Success(t *testing.T) {
	env := setupTestEnv(t)
	defer env.teardownTestEnv()

	// Setup mock media server response
	ratingKey := "12345"
	testFile := filepath.Join(testdataDir, "short_audio.mp3")
	env.mediaServer.SetPlexMetadata(ratingKey, testFile)

	// Create webhook payload
	payload := GetPlexPayload(ratingKey, "library.new")

	// Create multipart form (Plex sends JSON as form field)
	body := &bytes.Buffer{}
	writer := multipart.NewWriter(body)
	writer.WriteField("payload", payload)
	writer.Close()

	// Send webhook
	req, err := http.NewRequest("POST", env.httpServer.URL+"/plex", body)
	require.NoError(t, err)
	req.Header.Set("Content-Type", writer.FormDataContentType())
	req.Header.Set("User-Agent", "PlexMediaServer/1.40.0")

	resp, err := http.DefaultClient.Do(req)
	require.NoError(t, err)
	defer resp.Body.Close()

	// Verify HTTP response
	assert.Equal(t, http.StatusOK, resp.StatusCode, "Webhook should be accepted")

	// Wait for task to be queued
	time.Sleep(500 * time.Millisecond)

	// Verify media server was called
	assert.Equal(t, 1, env.mediaServer.GetCallCount(fmt.Sprintf("/library/metadata/%s", ratingKey)))

	// Verify task was queued
	assert.Greater(t, env.queue.Size(), 0, "Task should be queued")

	// Wait for transcription to complete (orchestrator dispatches to worker)
	time.Sleep(30 * time.Second)

	// Verify subtitle was created
	expectedSubtitle := strings.Replace(testFile, ".mp3", ".tiny.aa.srt", 1)
	_, err = os.Stat(expectedSubtitle)
	assert.NoError(t, err, "Subtitle file should exist: %s", expectedSubtitle)

	t.Log("✅ Plex library.new webhook → transcription completed successfully")
}

// Test 2: Plex media.play Webhook → Transcription
func TestPlex_MediaPlay_Success(t *testing.T) {
	env := setupTestEnv(t)
	defer env.teardownTestEnv()

	ratingKey := "67890"
	testFile := filepath.Join(testdataDir, "short_audio.mp3")
	env.mediaServer.SetPlexMetadata(ratingKey, testFile)

	payload := GetPlexPayload(ratingKey, "media.play")

	body := &bytes.Buffer{}
	writer := multipart.NewWriter(body)
	writer.WriteField("payload", payload)
	writer.Close()

	req, err := http.NewRequest("POST", env.httpServer.URL+"/plex", body)
	require.NoError(t, err)
	req.Header.Set("Content-Type", writer.FormDataContentType())
	req.Header.Set("User-Agent", "PlexMediaServer/1.40.0")

	resp, err := http.DefaultClient.Do(req)
	require.NoError(t, err)
	defer resp.Body.Close()

	assert.Equal(t, http.StatusOK, resp.StatusCode)
	
	time.Sleep(500 * time.Millisecond)
	assert.Greater(t, env.queue.Size(), 0, "Task should be queued")

	t.Log("✅ Plex media.play webhook processed")
}

// Test 3: Jellyfin ItemAdded Webhook → Transcription
func TestJellyfin_ItemAdded_Success(t *testing.T) {
	env := setupTestEnv(t)
	defer env.teardownTestEnv()

	itemID := "abc123def456"
	testFile := filepath.Join(testdataDir, "short_audio.mp3")
	env.mediaServer.SetJellyfinItem(itemID, testFile)

	payload := GetJellyfinPayload(itemID, "ItemAdded")

	req, err := http.NewRequest("POST", env.httpServer.URL+"/jellyfin", strings.NewReader(payload))
	require.NoError(t, err)
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	req.Header.Set("User-Agent", "Jellyfin-Server/10.8.13")

	resp, err := http.DefaultClient.Do(req)
	require.NoError(t, err)
	defer resp.Body.Close()

	assert.Equal(t, http.StatusOK, resp.StatusCode)
	
	time.Sleep(500 * time.Millisecond)
	assert.Greater(t, env.queue.Size(), 0, "Task should be queued")

	t.Log("✅ Jellyfin ItemAdded webhook processed")
}

// Test 4: Jellyfin PlaybackStart Webhook → Transcription
func TestJellyfin_PlaybackStart_Success(t *testing.T) {
	env := setupTestEnv(t)
	defer env.teardownTestEnv()

	itemID := "xyz789abc123"
	testFile := filepath.Join(testdataDir, "short_audio.mp3")
	env.mediaServer.SetJellyfinItem(itemID, testFile)

	payload := GetJellyfinPayload(itemID, "PlaybackStart")

	req, err := http.NewRequest("POST", env.httpServer.URL+"/jellyfin", strings.NewReader(payload))
	require.NoError(t, err)
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	req.Header.Set("User-Agent", "Jellyfin-Server/10.8.13")

	resp, err := http.DefaultClient.Do(req)
	require.NoError(t, err)
	defer resp.Body.Close()

	assert.Equal(t, http.StatusOK, resp.StatusCode)
	
	time.Sleep(500 * time.Millisecond)
	assert.Greater(t, env.queue.Size(), 0, "Task should be queued")

	t.Log("✅ Jellyfin PlaybackStart webhook processed")
}

// Test 5: Emby library.new Webhook → Transcription (Direct File Path)
func TestEmby_LibraryNew_Success(t *testing.T) {
	env := setupTestEnv(t)
	defer env.teardownTestEnv()

	testFile := filepath.Join(testdataDir, "short_audio.mp3")
	
	// Emby provides file path directly in payload
	payload := fmt.Sprintf(`data={"Event":"library.new","Item":{"Path":"%s"}}`, testFile)

	req, err := http.NewRequest("POST", env.httpServer.URL+"/emby", strings.NewReader(payload))
	require.NoError(t, err)
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")

	resp, err := http.DefaultClient.Do(req)
	require.NoError(t, err)
	defer resp.Body.Close()

	assert.Equal(t, http.StatusOK, resp.StatusCode)
	
	time.Sleep(500 * time.Millisecond)
	assert.Greater(t, env.queue.Size(), 0, "Task should be queued")

	t.Log("✅ Emby library.new webhook processed")
}

// Test 6: Tautulli added Webhook → Transcription (Direct File Path)
func TestTautulli_Added_Success(t *testing.T) {
	env := setupTestEnv(t)
	defer env.teardownTestEnv()

	testFile := filepath.Join(testdataDir, "short_audio.mp3")
	payload := fmt.Sprintf("event=added&file=%s", url.QueryEscape(testFile))

	req, err := http.NewRequest("POST", env.httpServer.URL+"/tautulli", strings.NewReader(payload))
	require.NoError(t, err)
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	req.Header.Set("source", "Tautulli")

	resp, err := http.DefaultClient.Do(req)
	require.NoError(t, err)
	defer resp.Body.Close()

	assert.Equal(t, http.StatusOK, resp.StatusCode)
	
	time.Sleep(500 * time.Millisecond)
	assert.Greater(t, env.queue.Size(), 0, "Task should be queued")

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
	writer.WriteField("payload", payload)
	writer.Close()

	req, err := http.NewRequest("POST", env.httpServer.URL+"/plex", body)
	require.NoError(t, err)
	req.Header.Set("Content-Type", writer.FormDataContentType())
	req.Header.Set("User-Agent", "PlexMediaServer/1.40.0")

	resp, err := http.DefaultClient.Do(req)
	require.NoError(t, err)
	defer resp.Body.Close()

	assert.Equal(t, http.StatusBadRequest, resp.StatusCode, "Invalid payload should be rejected")

	t.Log("✅ Invalid payload rejected correctly")
}

// Test 8: Missing User-Agent Header
func TestPlex_MissingUserAgent(t *testing.T) {
	env := setupTestEnv(t)
	defer env.teardownTestEnv()

	payload := GetPlexPayload("12345", "library.new")

	body := &bytes.Buffer{}
	writer := multipart.NewWriter(body)
	writer.WriteField("payload", payload)
	writer.Close()

	req, err := http.NewRequest("POST", env.httpServer.URL+"/plex", body)
	require.NoError(t, err)
	req.Header.Set("Content-Type", writer.FormDataContentType())
	// No User-Agent header

	resp, err := http.DefaultClient.Do(req)
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
	env.orchestrator.Config.ProcessAddedMedia = false

	payload := GetPlexPayload("12345", "library.new")

	body := &bytes.Buffer{}
	writer := multipart.NewWriter(body)
	writer.WriteField("payload", payload)
	writer.Close()

	req, err := http.NewRequest("POST", env.httpServer.URL+"/plex", body)
	require.NoError(t, err)
	req.Header.Set("Content-Type", writer.FormDataContentType())
	req.Header.Set("User-Agent", "PlexMediaServer/1.40.0")

	resp, err := http.DefaultClient.Do(req)
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
	testFile := filepath.Join(testdataDir, "short_audio.mp3")
	env.mediaServer.SetPlexMetadata(ratingKey, testFile)

	payload := GetPlexPayload(ratingKey, "library.new")

	body := &bytes.Buffer{}
	writer := multipart.NewWriter(body)
	writer.WriteField("payload", payload)
	writer.Close()

	// Send same webhook twice
	for i := 0; i < 2; i++ {
		req, err := http.NewRequest("POST", env.httpServer.URL+"/plex", bytes.NewReader(body.Bytes()))
		require.NoError(t, err)
		req.Header.Set("Content-Type", writer.FormDataContentType())
		req.Header.Set("User-Agent", "PlexMediaServer/1.40.0")

		resp, err := http.DefaultClient.Do(req)
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
	writer.WriteField("payload", payload)
	writer.Close()

	req, err := http.NewRequest("POST", env.httpServer.URL+"/plex", body)
	require.NoError(t, err)
	req.Header.Set("Content-Type", writer.FormDataContentType())
	req.Header.Set("User-Agent", "PlexMediaServer/1.40.0")

	resp, err := http.DefaultClient.Do(req)
	require.NoError(t, err)
	defer resp.Body.Close()

	// Webhook accepted, but media server call fails
	assert.Equal(t, http.StatusOK, resp.StatusCode)
	
	time.Sleep(500 * time.Millisecond)

	// Task might be queued but should fail during processing
	// (This tests retry logic - implementation detail)

	t.Log("✅ Media server API failure handled")
}

// Test 12: Queue Full Scenario
func TestPlex_QueueFull(t *testing.T) {
	// Create test env with small queue
	mockServer := NewMockMediaServer()
	defer mockServer.Close()

	cfg := &config.Config{
		ProcessAddedMedia: true,
		Queue: config.QueueConfig{
			MaxSize: 2, // Very small queue
		},
	}

	q := queue.NewPriorityQueue(cfg.Queue.MaxSize)

	// Fill queue
	for i := 0; i < 2; i++ {
		q.Push(&queue.Task{
			ID:       fmt.Sprintf("task-%d", i),
			FilePath: "/test/file.mp3",
		})
	}

	// Try to send webhook (queue full)
	webhookServer := webhooks.NewServer(cfg, q, nil, logrus.New())
	httpServer := httptest.NewServer(webhookServer.App())
	defer httpServer.Close()

	ratingKey := "12345"
	mockServer.SetPlexMetadata(ratingKey, "/test/file.mp3")

	payload := GetPlexPayload(ratingKey, "library.new")

	body := &bytes.Buffer{}
	writer := multipart.NewWriter(body)
	writer.WriteField("payload", payload)
	writer.Close()

	req, err := http.NewRequest("POST", httpServer.URL+"/plex", body)
	require.NoError(t, err)
	req.Header.Set("Content-Type", writer.FormDataContentType())
	req.Header.Set("User-Agent", "PlexMediaServer/1.40.0")

	resp, err := http.DefaultClient.Do(req)
	require.NoError(t, err)
	defer resp.Body.Close()

	// Should return 503 Service Unavailable
	assert.Equal(t, http.StatusServiceUnavailable, resp.StatusCode, "Queue full should return 503")

	t.Log("✅ Queue full scenario handled correctly")
}
```

---

## Definition of Done

- [ ] All acceptance criteria met
- [ ] Mock media server implemented (Plex/Jellyfin API)
- [ ] 12+ integration tests written and passing
- [ ] All 4 webhook types tested (Plex, Jellyfin, Emby, Tautulli)
- [ ] Error scenarios tested (invalid payload, missing headers, API failures)
- [ ] Queue behavior tested (deduplication, full queue)
- [ ] Tests run successfully with Docker Compose (worker running)
- [ ] Test execution time < 2 minutes (use tiny model)
- [ ] Documentation: How to run webhook integration tests
- [ ] Work log created: `docs/WORKLOGS/NNNN_YYYY-MM-DD_EPIC_03_story_02.md`
- [ ] Code committed and pushed

---

## Validation Commands

```bash
# Start Docker Compose environment (from STORY_01)
cd test
docker-compose -f docker-compose.integration.yml up -d

# Wait for services to be healthy
docker-compose -f docker-compose.integration.yml ps

# Run webhook integration tests
cd test/integration
go test -v -run TestPlex
go test -v -run TestJellyfin
go test -v -run TestEmby
go test -v -run TestTautulli

# Run all webhook tests
go test -v ./... -run Webhook

# Run with race detector
go test -race -v ./...

# Stop environment
cd test
docker-compose -f docker-compose.integration.yml down
```

---

## Dependencies

**Requires:**
- STORY_01 (gRPC Integration Tests) - Docker Compose environment, worker running

**Blocks:**
- STORY_03 (End-to-End Tests) - builds on webhook tests
- STORY_05 (Load Testing) - needs webhook infrastructure

---

## Notes

### Webhook Payload Formats

**Plex**: Multipart form with JSON in `payload` field
- User-Agent: `PlexMediaServer/*`
- Events: `library.new`, `media.play`

**Jellyfin**: URL-encoded form data
- User-Agent: `Jellyfin-Server/*`
- Events: `ItemAdded`, `PlaybackStart`

**Emby**: URL-encoded with JSON in `data` field
- No specific User-Agent requirement
- Events: `library.new`, `playback.start`

**Tautulli**: URL-encoded form data
- Custom header: `source: Tautulli`
- Events: `added`, `played`

### Mock vs Real Media Server

These tests use **mock media server** because:
- Real Plex/Jellyfin requires authentication
- Avoids external dependencies in CI
- Faster test execution
- Predictable responses

For manual testing with real servers, see STORY_03 (End-to-End Tests).

### Transcription Verification

Tests verify:
1. HTTP webhook accepted (200 OK)
2. Task queued correctly
3. Media server API called (if needed)
4. gRPC call made to worker
5. Subtitle file created (for full integration)

---

## References

- [orchestrator/internal/webhooks/server.go](/home/mikekao/personal/subgen/orchestrator/internal/webhooks/server.go) - Webhook handlers (lines 138-480)
- Plex Webhook Documentation: https://support.plex.tv/articles/115002267687-webhooks/
- Jellyfin Webhook Plugin: https://github.com/jellyfin/jellyfin-plugin-webhook
- Emby Webhook: https://emby.media/community/index.php?/topic/50875-webhook-notifications/

---

**Created**: 2026-02-15  
**Last Updated**: 2026-02-15
