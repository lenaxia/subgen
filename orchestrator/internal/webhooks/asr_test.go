package webhooks

import (
	"bytes"
	"io"
	"mime/multipart"
	"net/http/httptest"
	"testing"

	"github.com/mccloud/subgen/orchestrator/internal/config"
	"github.com/sirupsen/logrus"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestHandleASR_Success(t *testing.T) {
	server, queue := createTestServer(t)

	// Create multipart form with audio file
	body := &bytes.Buffer{}
	writer := multipart.NewWriter(body)

	// Add audio file field
	part, err := writer.CreateFormFile("audio_file", "test.wav")
	require.NoError(t, err)

	audioData := []byte("fake audio content for testing")
	_, err = part.Write(audioData)
	require.NoError(t, err)

	err = writer.Close()
	require.NoError(t, err)

	req := httptest.NewRequest("POST", "/asr?task=transcribe&language=en", body)
	req.Header.Set("Content-Type", writer.FormDataContentType())

	resp, err := server.app.Test(req, -1) // -1 disables timeout for blocking operation
	require.NoError(t, err)
	defer resp.Body.Close()

	assert.Equal(t, 200, resp.StatusCode)

	// Verify task was queued
	tasks := queue.GetTasks()
	assert.Len(t, tasks, 1)
}

func TestHandleASR_EmptyFile(t *testing.T) {
	server, queue := createTestServer(t)

	// Create multipart form with empty audio file
	body := &bytes.Buffer{}
	writer := multipart.NewWriter(body)

	// Add empty audio file field
	part, err := writer.CreateFormFile("audio_file", "empty.wav")
	require.NoError(t, err)

	// Write empty data
	_, err = part.Write([]byte{})
	require.NoError(t, err)

	err = writer.Close()
	require.NoError(t, err)

	req := httptest.NewRequest("POST", "/asr", body)
	req.Header.Set("Content-Type", writer.FormDataContentType())

	resp, err := server.app.Test(req)
	require.NoError(t, err)
	defer resp.Body.Close()

	assert.Equal(t, 400, resp.StatusCode)

	// Verify no task was queued
	tasks := queue.GetTasks()
	assert.Len(t, tasks, 0)
}

func TestHandleASR_MissingFile(t *testing.T) {
	server, queue := createTestServer(t)

	// Create multipart form without audio file
	body := &bytes.Buffer{}
	writer := multipart.NewWriter(body)
	err := writer.Close()
	require.NoError(t, err)

	req := httptest.NewRequest("POST", "/asr", body)
	req.Header.Set("Content-Type", writer.FormDataContentType())

	resp, err := server.app.Test(req)
	require.NoError(t, err)
	defer resp.Body.Close()

	assert.Equal(t, 400, resp.StatusCode)

	// Verify no task was queued
	tasks := queue.GetTasks()
	assert.Len(t, tasks, 0)
}

func TestHandleASR_DifferentOutputFormats(t *testing.T) {
	outputs := []string{"srt", "vtt", "txt", "json", "tsv"}

	for _, output := range outputs {
		t.Run(output, func(t *testing.T) {
			server, queue := createTestServer(t)
			queue.Reset()

			// Create multipart form with audio file
			body := &bytes.Buffer{}
			writer := multipart.NewWriter(body)

			part, err := writer.CreateFormFile("audio_file", "test.wav")
			require.NoError(t, err)

			audioData := []byte("fake audio content")
			_, err = part.Write(audioData)
			require.NoError(t, err)

			err = writer.Close()
			require.NoError(t, err)

			req := httptest.NewRequest("POST", "/asr?output="+output, body)
			req.Header.Set("Content-Type", writer.FormDataContentType())

			resp, err := server.app.Test(req, -1)
			require.NoError(t, err)
			defer resp.Body.Close()

			assert.Equal(t, 200, resp.StatusCode)

			// Verify task was queued
			tasks := queue.GetTasks()
			assert.Len(t, tasks, 1)
		})
	}
}

func TestHandleASR_WithVideoFile(t *testing.T) {
	server, queue := createTestServer(t)

	// Create multipart form with audio file
	body := &bytes.Buffer{}
	writer := multipart.NewWriter(body)

	part, err := writer.CreateFormFile("audio_file", "test.wav")
	require.NoError(t, err)

	audioData := []byte("fake audio content")
	_, err = part.Write(audioData)
	require.NoError(t, err)

	err = writer.Close()
	require.NoError(t, err)

	req := httptest.NewRequest("POST", "/asr?video_file=movie.mkv", body)
	req.Header.Set("Content-Type", writer.FormDataContentType())

	resp, err := server.app.Test(req, -1)
	require.NoError(t, err)
	defer resp.Body.Close()

	assert.Equal(t, 200, resp.StatusCode)

	// Verify task was queued with video file name
	tasks := queue.GetTasks()
	assert.Len(t, tasks, 1)
	// Note: We'd check task.VideoFile if we added that field
}

// TestHandleASR_OversizedFile tests GAP #4: AudioContent size validation
func TestHandleASR_OversizedFile(t *testing.T) {
	// Create a server with smaller max size for faster testing
	cfg := &config.Config{
		WebhookPort:        9000,
		ProcessAddedMedia:  true,
		ProcessMediaOnPlay: true,
		Queue: config.QueueConfig{
			MaxSize:             1000,
			MaxAudioContentSize: 1024, // 1KB limit for testing
		},
	}
	queue := &MockQueue{}
	log := logrus.New()
	log.SetOutput(io.Discard)
	server := NewServer(cfg, queue, log)

	// Create multipart form with a file that exceeds max size
	body := &bytes.Buffer{}
	writer := multipart.NewWriter(body)

	part, err := writer.CreateFormFile("audio_file", "huge.wav")
	require.NoError(t, err)

	// Create 2KB of data (exceeds 1KB limit)
	largeData := make([]byte, 2048)
	for i := range largeData {
		largeData[i] = byte(i % 256)
	}
	_, err = part.Write(largeData)
	require.NoError(t, err)

	err = writer.Close()
	require.NoError(t, err)

	req := httptest.NewRequest("POST", "/asr?video_file=movie.mkv", body)
	req.Header.Set("Content-Type", writer.FormDataContentType())

	resp, err := server.app.Test(req, -1)
	require.NoError(t, err)
	defer resp.Body.Close()

	// Should return 413 Request Entity Too Large
	assert.Equal(t, 413, resp.StatusCode)

	// Verify no task was queued
	tasks := queue.GetTasks()
	assert.Len(t, tasks, 0)
}
