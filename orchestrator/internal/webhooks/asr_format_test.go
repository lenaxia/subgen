package webhooks

import (
	"bytes"
	"io"
	"mime/multipart"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/gofiber/fiber/v2"
	"github.com/mccloud/subgen/orchestrator/internal/config"
	"github.com/sirupsen/logrus"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestASRFormat_SRT tests SRT format output (default)
func TestASRFormat_SRT(t *testing.T) {
	mockQueue := newMockQueueForBlocking()
	mockQueue.simulateDelay = 50 * time.Millisecond

	cfg := &config.Config{
		ASR: config.ASRConfig{
			Timeout: 5 * time.Second,
		},
		Queue: config.QueueConfig{
			MaxAudioContentSize: 10 * 1024 * 1024, // 10MB
		},
	}

	log := logrus.New()
	log.SetLevel(logrus.ErrorLevel)

	server := NewServer(cfg, mockQueue, log)
	app := server.App()

	// Create multipart request with audio file
	body := &bytes.Buffer{}
	writer := multipart.NewWriter(body)
	part, err := writer.CreateFormFile("audio_file", "test.mp3")
	require.NoError(t, err)
	_, err = part.Write([]byte("fake audio content"))
	require.NoError(t, err)
	writer.Close()

	// Test SRT format (default, no output parameter)
	req := httptest.NewRequest("POST", "/asr?task=transcribe&language=en", body)
	req.Header.Set("Content-Type", writer.FormDataContentType())

	resp, err := app.Test(req, 10000) // 10 second timeout
	require.NoError(t, err)
	defer resp.Body.Close()

	// Verify response
	assert.Equal(t, fiber.StatusOK, resp.StatusCode)
	assert.Equal(t, "text/plain; charset=utf-8", resp.Header.Get("Content-Type"))

	// Read and verify SRT format
	bodyBytes, err := io.ReadAll(resp.Body)
	require.NoError(t, err)
	bodyStr := string(bodyBytes)

	// SRT format validation
	assert.Contains(t, bodyStr, "1\n")                       // Sequence number
	assert.Contains(t, bodyStr, "00:00:00,000 --> 00:00:03") // Timestamp with commas
	assert.Contains(t, bodyStr, "Test segment 1")            // Text content
	assert.Contains(t, bodyStr, "2\n")                       // Second sequence
	assert.Contains(t, bodyStr, "Test segment 2")
}

// TestASRFormat_VTT tests VTT format output
func TestASRFormat_VTT(t *testing.T) {
	mockQueue := newMockQueueForBlocking()
	mockQueue.simulateDelay = 50 * time.Millisecond

	cfg := &config.Config{
		ASR: config.ASRConfig{
			Timeout: 5 * time.Second,
		},
		Queue: config.QueueConfig{
			MaxAudioContentSize: 10 * 1024 * 1024,
		},
	}

	log := logrus.New()
	log.SetLevel(logrus.ErrorLevel)

	server := NewServer(cfg, mockQueue, log)
	app := server.App()

	// Create multipart request
	body := &bytes.Buffer{}
	writer := multipart.NewWriter(body)
	part, err := writer.CreateFormFile("audio_file", "test.mp3")
	require.NoError(t, err)
	_, err = part.Write([]byte("fake audio content"))
	require.NoError(t, err)
	writer.Close()

	// Test VTT format
	req := httptest.NewRequest("POST", "/asr?task=transcribe&language=en&output=vtt", body)
	req.Header.Set("Content-Type", writer.FormDataContentType())

	resp, err := app.Test(req, 10000)
	require.NoError(t, err)
	defer resp.Body.Close()

	// Verify response
	assert.Equal(t, fiber.StatusOK, resp.StatusCode)
	assert.Equal(t, "text/vtt; charset=utf-8", resp.Header.Get("Content-Type"))

	// Read and verify VTT format
	bodyBytes, err := io.ReadAll(resp.Body)
	require.NoError(t, err)
	bodyStr := string(bodyBytes)

	// VTT format validation
	assert.True(t, strings.HasPrefix(bodyStr, "WEBVTT"), "Should start with WEBVTT header")
	assert.Contains(t, bodyStr, "00:00:00.000 --> 00:00:03") // Timestamp with dots
	assert.Contains(t, bodyStr, "Test segment 1")
	assert.Contains(t, bodyStr, "Test segment 2")
}

// TestASRFormat_LRC tests LRC format output
func TestASRFormat_LRC(t *testing.T) {
	mockQueue := newMockQueueForBlocking()
	mockQueue.simulateDelay = 50 * time.Millisecond

	cfg := &config.Config{
		ASR: config.ASRConfig{
			Timeout: 5 * time.Second,
		},
		Queue: config.QueueConfig{
			MaxAudioContentSize: 10 * 1024 * 1024,
		},
	}

	log := logrus.New()
	log.SetLevel(logrus.ErrorLevel)

	server := NewServer(cfg, mockQueue, log)
	app := server.App()

	// Create multipart request
	body := &bytes.Buffer{}
	writer := multipart.NewWriter(body)
	part, err := writer.CreateFormFile("audio_file", "test.mp3")
	require.NoError(t, err)
	_, err = part.Write([]byte("fake audio content"))
	require.NoError(t, err)
	writer.Close()

	// Test LRC format
	req := httptest.NewRequest("POST", "/asr?task=transcribe&language=en&output=lrc", body)
	req.Header.Set("Content-Type", writer.FormDataContentType())

	resp, err := app.Test(req, 10000)
	require.NoError(t, err)
	defer resp.Body.Close()

	// Verify response
	assert.Equal(t, fiber.StatusOK, resp.StatusCode)
	assert.Equal(t, "text/plain; charset=utf-8", resp.Header.Get("Content-Type"))

	// Read and verify LRC format
	bodyBytes, err := io.ReadAll(resp.Body)
	require.NoError(t, err)
	bodyStr := string(bodyBytes)

	// LRC format validation
	assert.Contains(t, bodyStr, "[00:00.00]Test segment 1") // LRC timestamp format
	assert.Contains(t, bodyStr, "[00:03.3")                 // Allow for rounding (3.39 or 3.40)
	assert.Contains(t, bodyStr, "Test segment 2")
}

// TestASRFormat_TXT tests plain text format output
func TestASRFormat_TXT(t *testing.T) {
	mockQueue := newMockQueueForBlocking()
	mockQueue.simulateDelay = 50 * time.Millisecond

	cfg := &config.Config{
		ASR: config.ASRConfig{
			Timeout: 5 * time.Second,
		},
		Queue: config.QueueConfig{
			MaxAudioContentSize: 10 * 1024 * 1024,
		},
	}

	log := logrus.New()
	log.SetLevel(logrus.ErrorLevel)

	server := NewServer(cfg, mockQueue, log)
	app := server.App()

	// Create multipart request
	body := &bytes.Buffer{}
	writer := multipart.NewWriter(body)
	part, err := writer.CreateFormFile("audio_file", "test.mp3")
	require.NoError(t, err)
	_, err = part.Write([]byte("fake audio content"))
	require.NoError(t, err)
	writer.Close()

	// Test TXT format
	req := httptest.NewRequest("POST", "/asr?task=transcribe&language=en&output=txt", body)
	req.Header.Set("Content-Type", writer.FormDataContentType())

	resp, err := app.Test(req, 10000)
	require.NoError(t, err)
	defer resp.Body.Close()

	// Verify response
	assert.Equal(t, fiber.StatusOK, resp.StatusCode)
	assert.Equal(t, "text/plain; charset=utf-8", resp.Header.Get("Content-Type"))

	// Read and verify TXT format (no timestamps)
	bodyBytes, err := io.ReadAll(resp.Body)
	require.NoError(t, err)
	bodyStr := string(bodyBytes)

	// TXT format validation - just text, no timestamps
	assert.Contains(t, bodyStr, "Test segment 1")
	assert.Contains(t, bodyStr, "Test segment 2")
	assert.NotContains(t, bodyStr, "00:00") // No timestamps
	assert.NotContains(t, bodyStr, "-->")   // No SRT arrows
	assert.NotContains(t, bodyStr, "[")     // No LRC brackets
}

// TestASRFormat_TSV tests tab-separated values format output
func TestASRFormat_TSV(t *testing.T) {
	mockQueue := newMockQueueForBlocking()
	mockQueue.simulateDelay = 50 * time.Millisecond

	cfg := &config.Config{
		ASR: config.ASRConfig{
			Timeout: 5 * time.Second,
		},
		Queue: config.QueueConfig{
			MaxAudioContentSize: 10 * 1024 * 1024,
		},
	}

	log := logrus.New()
	log.SetLevel(logrus.ErrorLevel)

	server := NewServer(cfg, mockQueue, log)
	app := server.App()

	// Create multipart request
	body := &bytes.Buffer{}
	writer := multipart.NewWriter(body)
	part, err := writer.CreateFormFile("audio_file", "test.mp3")
	require.NoError(t, err)
	_, err = part.Write([]byte("fake audio content"))
	require.NoError(t, err)
	writer.Close()

	// Test TSV format
	req := httptest.NewRequest("POST", "/asr?task=transcribe&language=en&output=tsv", body)
	req.Header.Set("Content-Type", writer.FormDataContentType())

	resp, err := app.Test(req, 10000)
	require.NoError(t, err)
	defer resp.Body.Close()

	// Verify response
	assert.Equal(t, fiber.StatusOK, resp.StatusCode)
	assert.Equal(t, "text/plain; charset=utf-8", resp.Header.Get("Content-Type"))

	// Read and verify TSV format
	bodyBytes, err := io.ReadAll(resp.Body)
	require.NoError(t, err)
	bodyStr := string(bodyBytes)

	// TSV format validation - tab-separated columns
	lines := strings.Split(strings.TrimSpace(bodyStr), "\n")
	assert.GreaterOrEqual(t, len(lines), 3, "Should have header + at least 2 data rows")

	// Check header
	assert.Contains(t, lines[0], "start\tend\ttext")

	// Check data rows
	assert.Contains(t, bodyStr, "\t") // Should have tabs
	assert.Contains(t, bodyStr, "0.000")
	assert.Contains(t, bodyStr, "Test segment 1")
}

// TestASRFormat_JSON tests JSON format output
func TestASRFormat_JSON(t *testing.T) {
	mockQueue := newMockQueueForBlocking()
	mockQueue.simulateDelay = 50 * time.Millisecond

	cfg := &config.Config{
		ASR: config.ASRConfig{
			Timeout: 5 * time.Second,
		},
		Queue: config.QueueConfig{
			MaxAudioContentSize: 10 * 1024 * 1024,
		},
	}

	log := logrus.New()
	log.SetLevel(logrus.ErrorLevel)

	server := NewServer(cfg, mockQueue, log)
	app := server.App()

	// Create multipart request
	body := &bytes.Buffer{}
	writer := multipart.NewWriter(body)
	part, err := writer.CreateFormFile("audio_file", "test.mp3")
	require.NoError(t, err)
	_, err = part.Write([]byte("fake audio content"))
	require.NoError(t, err)
	writer.Close()

	// Test JSON format
	req := httptest.NewRequest("POST", "/asr?task=transcribe&language=en&output=json", body)
	req.Header.Set("Content-Type", writer.FormDataContentType())

	resp, err := app.Test(req, 10000)
	require.NoError(t, err)
	defer resp.Body.Close()

	// Verify response
	assert.Equal(t, fiber.StatusOK, resp.StatusCode)
	assert.Equal(t, "application/json; charset=utf-8", resp.Header.Get("Content-Type"))

	// Read and verify JSON format
	bodyBytes, err := io.ReadAll(resp.Body)
	require.NoError(t, err)
	bodyStr := string(bodyBytes)

	// JSON format validation
	assert.Contains(t, bodyStr, `"language"`)
	assert.Contains(t, bodyStr, `"duration"`)
	assert.Contains(t, bodyStr, `"segments"`)
	assert.Contains(t, bodyStr, `"start"`)
	assert.Contains(t, bodyStr, `"end"`)
	assert.Contains(t, bodyStr, `"text"`)
	assert.Contains(t, bodyStr, "Test segment 1")
	assert.Contains(t, bodyStr, "Test segment 2")
}

// TestASRFormat_Invalid tests invalid format returns error
func TestASRFormat_Invalid(t *testing.T) {
	mockQueue := newMockQueueForBlocking()

	cfg := &config.Config{
		ASR: config.ASRConfig{
			Timeout: 5 * time.Second,
		},
		Queue: config.QueueConfig{
			MaxAudioContentSize: 10 * 1024 * 1024,
		},
	}

	log := logrus.New()
	log.SetLevel(logrus.ErrorLevel)

	server := NewServer(cfg, mockQueue, log)
	app := server.App()

	// Create multipart request
	body := &bytes.Buffer{}
	writer := multipart.NewWriter(body)
	part, err := writer.CreateFormFile("audio_file", "test.mp3")
	require.NoError(t, err)
	_, err = part.Write([]byte("fake audio content"))
	require.NoError(t, err)
	writer.Close()

	// Test invalid format
	req := httptest.NewRequest("POST", "/asr?task=transcribe&language=en&output=invalid", body)
	req.Header.Set("Content-Type", writer.FormDataContentType())

	resp, err := app.Test(req, 10000)
	require.NoError(t, err)
	defer resp.Body.Close()

	// Verify error response
	assert.Equal(t, fiber.StatusBadRequest, resp.StatusCode)

	bodyBytes, err := io.ReadAll(resp.Body)
	require.NoError(t, err)
	bodyStr := string(bodyBytes)

	assert.Contains(t, bodyStr, "invalid format")
	assert.Contains(t, bodyStr, "supported")
}

// TestASRFormat_CaseInsensitive tests format is case-insensitive
func TestASRFormat_CaseInsensitive(t *testing.T) {
	mockQueue := newMockQueueForBlocking()
	mockQueue.simulateDelay = 50 * time.Millisecond

	cfg := &config.Config{
		ASR: config.ASRConfig{
			Timeout: 5 * time.Second,
		},
		Queue: config.QueueConfig{
			MaxAudioContentSize: 10 * 1024 * 1024,
		},
	}

	log := logrus.New()
	log.SetLevel(logrus.ErrorLevel)

	server := NewServer(cfg, mockQueue, log)
	app := server.App()

	// Test with uppercase format
	body := &bytes.Buffer{}
	writer := multipart.NewWriter(body)
	part, err := writer.CreateFormFile("audio_file", "test.mp3")
	require.NoError(t, err)
	_, err = part.Write([]byte("fake audio content"))
	require.NoError(t, err)
	writer.Close()

	req := httptest.NewRequest("POST", "/asr?task=transcribe&language=en&output=VTT", body)
	req.Header.Set("Content-Type", writer.FormDataContentType())

	resp, err := app.Test(req, 10000)
	require.NoError(t, err)
	defer resp.Body.Close()

	// Should succeed with uppercase format
	assert.Equal(t, fiber.StatusOK, resp.StatusCode)
	assert.Equal(t, "text/vtt; charset=utf-8", resp.Header.Get("Content-Type"))

	bodyBytes, err := io.ReadAll(resp.Body)
	require.NoError(t, err)
	bodyStr := string(bodyBytes)

	assert.True(t, strings.HasPrefix(bodyStr, "WEBVTT"))
}
