package webhooks

import (
	"bytes"
	"encoding/json"
	"mime/multipart"
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"

	"github.com/mccloud/subgen/orchestrator/internal/config"
	"github.com/mccloud/subgen/orchestrator/internal/queue"
	"github.com/sirupsen/logrus"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// MockQueueWithResult sends a fake result to ResultChan to unblock ASR endpoint
type MockQueueWithResult struct {
	tasks []Task
}

func (m *MockQueueWithResult) Enqueue(task Task) error {
	m.tasks = append(m.tasks, task)
	// If task has ResultChan, send fake result to unblock ASR endpoint
	if task.ResultChan != nil {
		go func() {
			task.ResultChan <- &queue.TranscriptionResult{
				Segments: []queue.Segment{
					{Text: "fake transcription"},
				},
				Metadata: queue.Metadata{
					Language: "en",
					Duration: 1.0,
				},
				Error: nil,
			}
		}()
	}
	return nil
}

func (m *MockQueueWithResult) Size() int                                  { return len(m.tasks) }
func (m *MockQueueWithResult) ProcessingCount() int                       { return 0 }
func (m *MockQueueWithResult) IsIdle() bool                               { return len(m.tasks) == 0 }
func (m *MockQueueWithResult) GetTaskInfo(taskID string) *queue.TaskInfo  { return nil }
func (m *MockQueueWithResult) GetAllProcessingTaskInfo() []queue.TaskInfo { return []queue.TaskInfo{} }
func (m *MockQueueWithResult) GetHistory(limit, offset int) []queue.TaskInfo {
	return []queue.TaskInfo{}
}
func (m *MockQueueWithResult) GetHistoryTotal() int { return 0 }
func (m *MockQueueWithResult) GetTasks() []Task     { return m.tasks }

// TestPathMapping_Emby tests path mapping in Emby webhook handler
func TestPathMapping_Emby(t *testing.T) {
	// Create temporary directory structure
	tmpDir := t.TempDir()
	destDir := filepath.Join(tmpDir, "dest")
	require.NoError(t, os.MkdirAll(filepath.Join(destDir, "movies"), 0755))

	testFile := filepath.Join(destDir, "movies", "test.mkv")
	require.NoError(t, os.WriteFile(testFile, []byte("test"), 0644))

	// Setup config with path mapping
	cfg := &config.Config{
		ProcessAddedMedia: true,
		PathMapping: config.PathMappingConfig{
			Enabled: true,
			From:    "/data",
			To:      destDir,
		},
		Plex: config.PlexConfig{
			Token:   "test-token",
			Enabled: true,
		},
	}

	// Create mock queue
	queue := &MockQueue{}
	log := logrus.New()
	log.SetLevel(logrus.FatalLevel)

	// Create server
	server := NewServer(cfg, queue, log)
	app := server.App()

	// Create test payload as form-encoded data with Emby format
	var buf bytes.Buffer
	buf.WriteString(`data={"Event":"library.new","Item":{"Path":"/data/movies/test.mkv","Type":"Movie"}}`)

	// Create request
	req := httptest.NewRequest("POST", "/emby", &buf)
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")

	// Execute request
	resp, err := app.Test(req)
	require.NoError(t, err)

	// Assert
	assert.Equal(t, 200, resp.StatusCode)
	assert.Equal(t, 1, len(queue.GetTasks()))
	assert.Equal(t, testFile, queue.GetTasks()[0].FilePath) // Should be mapped path
}

// TestPathMapping_ASR tests path mapping in ASR webhook handler
func TestPathMapping_ASR(t *testing.T) {
	// Create temporary directory structure
	tmpDir := t.TempDir()
	destDir := filepath.Join(tmpDir, "dest")
	require.NoError(t, os.MkdirAll(filepath.Join(destDir, "videos"), 0755))

	testFile := filepath.Join(destDir, "videos", "video.mp4")
	require.NoError(t, os.WriteFile(testFile, []byte("test"), 0644))

	// Setup config with path mapping
	cfg := &config.Config{
		Queue: config.QueueConfig{
			MaxAudioContentSize: 100 * 1024 * 1024, // 100MB
		},
		PathMapping: config.PathMappingConfig{
			Enabled: true,
			From:    "/source",
			To:      destDir,
		},
		Plex: config.PlexConfig{
			Token:   "test-token",
			Enabled: true,
		},
	}

	// Create mock queue that sends fake result to unblock ASR endpoint
	queue := &MockQueueWithResult{}
	log := logrus.New()
	log.SetLevel(logrus.FatalLevel)

	// Create server
	server := NewServer(cfg, queue, log)
	app := server.App()

	// Create multipart form with audio file
	var buf bytes.Buffer
	writer := multipart.NewWriter(&buf)

	// Add audio file
	part, err := writer.CreateFormFile("audio_file", "audio.mp3")
	require.NoError(t, err)
	_, err = part.Write([]byte("fake audio content"))
	require.NoError(t, err)

	require.NoError(t, writer.Close())

	// Create request with video_file query parameter
	req := httptest.NewRequest("POST", "/asr?task=transcribe&language=en&video_file=/source/videos/video.mp4", &buf)
	req.Header.Set("Content-Type", writer.FormDataContentType())

	// Execute request
	resp, err := app.Test(req)
	require.NoError(t, err)

	// Assert
	assert.Equal(t, 200, resp.StatusCode)
	tasks := queue.GetTasks()
	assert.Equal(t, 1, len(tasks))
	assert.Equal(t, testFile, tasks[0].FilePath) // Should be mapped path
	assert.Equal(t, "transcribe", tasks[0].TranscriptionType)
	assert.Equal(t, "en", tasks[0].ForceLanguage)
}

// TestPathMapping_Disabled tests that mapping is bypassed when disabled
func TestPathMapping_Disabled(t *testing.T) {
	// Setup config with path mapping DISABLED
	cfg := &config.Config{
		ProcessAddedMedia: true,
		PathMapping: config.PathMappingConfig{
			Enabled: false,
			From:    "/data",
			To:      "/mnt/media",
		},
		Plex: config.PlexConfig{
			Token:   "test-token",
			Enabled: true,
		},
	}

	// Create mock queue
	queue := &MockQueue{}
	log := logrus.New()
	log.SetLevel(logrus.FatalLevel)

	// Create server
	server := NewServer(cfg, queue, log)
	app := server.App()

	// Create test payload (path doesn't need to exist when mapping disabled)
	payload := EmbyPayload{
		Event: "library.new",
		Item: struct {
			Path string `json:"Path"`
		}{
			Path: "/data/movies/test.mkv",
		},
	}
	payloadBytes, err := json.Marshal(payload)
	require.NoError(t, err)

	// Create request
	req := httptest.NewRequest("POST", "/emby", bytes.NewReader([]byte("data="+string(payloadBytes))))
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")

	// Execute request
	resp, err := app.Test(req)
	require.NoError(t, err)

	// Assert
	assert.Equal(t, 200, resp.StatusCode)
	tasks := queue.GetTasks()
	assert.Equal(t, 1, len(tasks))
	assert.Equal(t, "/data/movies/test.mkv", tasks[0].FilePath) // Should be original path (mapping disabled)
}

// TestPathMapping_InvalidPath tests error handling when mapped path doesn't exist
func TestPathMapping_InvalidPath(t *testing.T) {
	// Setup config with path mapping
	cfg := &config.Config{
		ProcessAddedMedia: true,
		PathMapping: config.PathMappingConfig{
			Enabled: true,
			From:    "/data",
			To:      "/nonexistent",
		},
		Plex: config.PlexConfig{
			Token:   "test-token",
			Enabled: true,
		},
	}

	// Create mock queue
	queue := &MockQueue{}
	log := logrus.New()
	log.SetLevel(logrus.FatalLevel)

	// Create server
	server := NewServer(cfg, queue, log)
	app := server.App()

	// Create test payload
	payload := EmbyPayload{
		Event: "library.new",
		Item: struct {
			Path string `json:"Path"`
		}{
			Path: "/data/movies/test.mkv",
		},
	}
	payloadBytes, err := json.Marshal(payload)
	require.NoError(t, err)

	// Create request
	req := httptest.NewRequest("POST", "/emby", bytes.NewReader([]byte("data="+string(payloadBytes))))
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")

	// Execute request
	resp, err := app.Test(req)
	require.NoError(t, err)

	// Assert - should return error
	assert.Equal(t, 400, resp.StatusCode)
	tasks := queue.GetTasks()
	assert.Equal(t, 0, len(tasks)) // Should NOT have queued task
}

// TestPathMapping_MultipleMappings tests multiple path mappings
func TestPathMapping_MultipleMappings(t *testing.T) {
	// Create temporary directory structure
	tmpDir := t.TempDir()
	movieDir := filepath.Join(tmpDir, "movies")
	tvDir := filepath.Join(tmpDir, "tv")

	require.NoError(t, os.MkdirAll(movieDir, 0755))
	require.NoError(t, os.MkdirAll(tvDir, 0755))

	movieFile := filepath.Join(movieDir, "action.mkv")
	tvFile := filepath.Join(tvDir, "show.mkv")

	require.NoError(t, os.WriteFile(movieFile, []byte("test"), 0644))
	require.NoError(t, os.WriteFile(tvFile, []byte("test"), 0644))

	// Setup config with multiple path mappings
	cfg := &config.Config{
		ProcessAddedMedia: true,
		PathMapping: config.PathMappingConfig{
			Enabled: true,
			From:    "/data/movies,/data/tv",
			To:      movieDir + "," + tvDir,
		},
		Plex: config.PlexConfig{
			Token:   "test-token",
			Enabled: true,
		},
	}

	// Create mock queue
	queue := &MockQueue{}
	log := logrus.New()
	log.SetLevel(logrus.FatalLevel)

	// Create server
	server := NewServer(cfg, queue, log)
	app := server.App()

	// Test first mapping (movies)
	payload1 := EmbyPayload{
		Event: "library.new",
		Item: struct {
			Path string `json:"Path"`
		}{
			Path: "/data/movies/action.mkv",
		},
	}
	payloadBytes1, err := json.Marshal(payload1)
	require.NoError(t, err)

	req1 := httptest.NewRequest("POST", "/emby", bytes.NewReader([]byte("data="+string(payloadBytes1))))
	req1.Header.Set("Content-Type", "application/x-www-form-urlencoded")

	resp1, err := app.Test(req1)
	require.NoError(t, err)
	assert.Equal(t, 200, resp1.StatusCode)
	tasks := queue.GetTasks()
	assert.Equal(t, 1, len(tasks))
	assert.Equal(t, movieFile, tasks[0].FilePath)

	// Test second mapping (tv)
	payload2 := EmbyPayload{
		Event: "library.new",
		Item: struct {
			Path string `json:"Path"`
		}{
			Path: "/data/tv/show.mkv",
		},
	}
	payloadBytes2, err := json.Marshal(payload2)
	require.NoError(t, err)

	req2 := httptest.NewRequest("POST", "/emby", bytes.NewReader([]byte("data="+string(payloadBytes2))))
	req2.Header.Set("Content-Type", "application/x-www-form-urlencoded")

	resp2, err := app.Test(req2)
	require.NoError(t, err)
	assert.Equal(t, 200, resp2.StatusCode)
	tasks = queue.GetTasks()
	assert.Equal(t, 2, len(tasks))
	assert.Equal(t, tvFile, tasks[1].FilePath)
}
