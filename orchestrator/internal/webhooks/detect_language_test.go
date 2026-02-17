package webhooks

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"mime/multipart"
	"net/http/httptest"
	"os"
	"testing"

	"github.com/gofiber/fiber/v2"
	"github.com/mccloud/subgen/orchestrator/internal/config"
	pb "github.com/mccloud/subgen/orchestrator/pkg/pb"
	"github.com/sirupsen/logrus"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// MockGRPCClient is a simple mock for testing
type MockGRPCClient struct {
	DetectLanguageFunc func(ctx context.Context, workerAddr string, filePath string, offset float64, length float64) (*pb.DetectLanguageResponse, error)
}

func (m *MockGRPCClient) DetectLanguage(ctx context.Context, workerAddr string, filePath string, offset float64, length float64) (*pb.DetectLanguageResponse, error) {
	if m.DetectLanguageFunc != nil {
		return m.DetectLanguageFunc(ctx, workerAddr, filePath, offset, length)
	}
	return &pb.DetectLanguageResponse{
		LanguageName: "English",
		LanguageCode: "en",
		Confidence:   0.99,
	}, nil
}

// MockWorkerPool is a simple mock for testing
type MockWorkerPool struct {
	SelectWorkerFunc func() (*Worker, error)
}

func (m *MockWorkerPool) SelectWorker() (*Worker, error) {
	if m.SelectWorkerFunc != nil {
		return m.SelectWorkerFunc()
	}
	return &Worker{
		Address: "worker1:50051",
		Healthy: true,
	}, nil
}

// createTestServerWithMocks creates a test server with mock dependencies
func createTestServerWithMocks(grpcClient GRPCClientInterface, workerPool WorkerPoolInterface) *Server {
	cfg := &config.Config{
		WebhookPort: 9000,
	}
	log := logrus.New()
	log.SetOutput(io.Discard)

	// Create app directly to avoid queue validation issues
	app := fiber.New(fiber.Config{
		DisableStartupMessage: true,
	})

	server := &Server{
		app:        app,
		config:     cfg,
		queue:      nil, // Not needed for detect-language endpoint
		grpcClient: grpcClient,
		workerPool: workerPool,
		log:        log,
	}

	// Setup routes manually
	app.Post("/detect-language", server.handleDetectLanguage)

	return server
}

// TestHandleDetectLanguage_Success tests the happy path
func TestHandleDetectLanguage_Success(t *testing.T) {
	mockClient := &MockGRPCClient{
		DetectLanguageFunc: func(ctx context.Context, workerAddr string, filePath string, offset float64, length float64) (*pb.DetectLanguageResponse, error) {
			assert.Equal(t, "worker1:50051", workerAddr)
			assert.Equal(t, 0.0, offset)
			assert.Equal(t, 30.0, length)
			// Verify temp file exists
			_, err := os.Stat(filePath)
			assert.NoError(t, err)

			return &pb.DetectLanguageResponse{
				LanguageName: "English",
				LanguageCode: "en",
				Confidence:   0.99,
			}, nil
		},
	}

	mockWorkerPool := &MockWorkerPool{}
	server := createTestServerWithMocks(mockClient, mockWorkerPool)

	// Create test audio file
	audioData := []byte("fake audio data for testing")
	body := &bytes.Buffer{}
	writer := multipart.NewWriter(body)
	part, err := writer.CreateFormFile("file", "test_audio.mp3")
	require.NoError(t, err)
	_, err = part.Write(audioData)
	require.NoError(t, err)
	writer.Close()

	// Create test request
	req := httptest.NewRequest("POST", "/detect-language?offset=0&length=30", body)
	req.Header.Set("Content-Type", writer.FormDataContentType())

	// Execute request
	resp, err := server.app.Test(req, -1)
	require.NoError(t, err)
	defer resp.Body.Close()

	// Verify response
	assert.Equal(t, fiber.StatusOK, resp.StatusCode)

	// Parse response body
	respBody, err := io.ReadAll(resp.Body)
	require.NoError(t, err)

	// Verify JSON response
	assert.Contains(t, string(respBody), `"language":"English"`)
	assert.Contains(t, string(respBody), `"code":"en"`)
	assert.Contains(t, string(respBody), `"confidence":0.99`)
}

// TestHandleDetectLanguage_NoFile tests error when no file uploaded
func TestHandleDetectLanguage_NoFile(t *testing.T) {
	server := createTestServerWithMocks(&MockGRPCClient{}, &MockWorkerPool{})

	// Create test request without file
	req := httptest.NewRequest("POST", "/detect-language", nil)

	// Execute request
	resp, err := server.app.Test(req, -1)
	require.NoError(t, err)
	defer resp.Body.Close()

	// Verify error response
	assert.Equal(t, fiber.StatusBadRequest, resp.StatusCode)

	respBody, err := io.ReadAll(resp.Body)
	require.NoError(t, err)
	assert.Contains(t, string(respBody), "no file uploaded")
}

// TestHandleDetectLanguage_InvalidOffset tests error for negative offset
func TestHandleDetectLanguage_InvalidOffset(t *testing.T) {
	server := createTestServerWithMocks(&MockGRPCClient{}, &MockWorkerPool{})

	// Create test audio file
	audioData := []byte("fake audio data")
	body := &bytes.Buffer{}
	writer := multipart.NewWriter(body)
	part, err := writer.CreateFormFile("file", "test.mp3")
	require.NoError(t, err)
	_, err = part.Write(audioData)
	require.NoError(t, err)
	writer.Close()

	// Create test request with invalid offset
	req := httptest.NewRequest("POST", "/detect-language?offset=-5&length=30", body)
	req.Header.Set("Content-Type", writer.FormDataContentType())

	// Execute request
	resp, err := server.app.Test(req, -1)
	require.NoError(t, err)
	defer resp.Body.Close()

	// Verify error response
	assert.Equal(t, fiber.StatusBadRequest, resp.StatusCode)

	var respJSON ErrorResponse
	err = json.NewDecoder(resp.Body).Decode(&respJSON)
	require.NoError(t, err)
	assert.Contains(t, respJSON.Error, "offset must be >= 0")
}

// TestHandleDetectLanguage_InvalidLength tests error for invalid length
func TestHandleDetectLanguage_InvalidLength(t *testing.T) {
	testCases := []struct {
		name   string
		length string
		errMsg string
	}{
		{
			name:   "negative length",
			length: "-10",
			errMsg: "length must be between 0 and 300 seconds",
		},
		{
			name:   "zero length",
			length: "0",
			errMsg: "length must be between 0 and 300 seconds",
		},
		{
			name:   "too long",
			length: "400",
			errMsg: "length must be between 0 and 300 seconds",
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			server := createTestServerWithMocks(&MockGRPCClient{}, &MockWorkerPool{})

			// Create test audio file
			audioData := []byte("fake audio data")
			body := &bytes.Buffer{}
			writer := multipart.NewWriter(body)
			part, err := writer.CreateFormFile("file", "test.mp3")
			require.NoError(t, err)
			_, err = part.Write(audioData)
			require.NoError(t, err)
			writer.Close()

			// Create test request
			req := httptest.NewRequest("POST", fmt.Sprintf("/detect-language?offset=0&length=%s", tc.length), body)
			req.Header.Set("Content-Type", writer.FormDataContentType())

			// Execute request
			resp, err := server.app.Test(req, -1)
			require.NoError(t, err)
			defer resp.Body.Close()

			// Verify error response
			assert.Equal(t, fiber.StatusBadRequest, resp.StatusCode)

			respBody, err := io.ReadAll(resp.Body)
			require.NoError(t, err)
			assert.Contains(t, string(respBody), tc.errMsg)
		})
	}
}

// TestHandleDetectLanguage_WorkerError tests error when worker returns error
func TestHandleDetectLanguage_WorkerError(t *testing.T) {
	mockClient := &MockGRPCClient{
		DetectLanguageFunc: func(ctx context.Context, workerAddr string, filePath string, offset float64, length float64) (*pb.DetectLanguageResponse, error) {
			return nil, errors.New("invalid audio format")
		},
	}

	mockWorkerPool := &MockWorkerPool{}
	server := createTestServerWithMocks(mockClient, mockWorkerPool)

	// Create test audio file
	audioData := []byte("fake audio data")
	body := &bytes.Buffer{}
	writer := multipart.NewWriter(body)
	part, err := writer.CreateFormFile("file", "test.mp3")
	require.NoError(t, err)
	_, err = part.Write(audioData)
	require.NoError(t, err)
	writer.Close()

	// Create test request
	req := httptest.NewRequest("POST", "/detect-language?offset=0&length=30", body)
	req.Header.Set("Content-Type", writer.FormDataContentType())

	// Execute request
	resp, err := server.app.Test(req, -1)
	require.NoError(t, err)
	defer resp.Body.Close()

	// Verify error response
	assert.Equal(t, fiber.StatusInternalServerError, resp.StatusCode)

	respBody, err := io.ReadAll(resp.Body)
	require.NoError(t, err)
	assert.Contains(t, string(respBody), "language detection failed")
}

// TestHandleDetectLanguage_NoWorkerAvailable tests error when no worker available
func TestHandleDetectLanguage_NoWorkerAvailable(t *testing.T) {
	mockWorkerPool := &MockWorkerPool{
		SelectWorkerFunc: func() (*Worker, error) {
			return nil, errors.New("no workers available")
		},
	}

	server := createTestServerWithMocks(&MockGRPCClient{}, mockWorkerPool)

	// Create test audio file
	audioData := []byte("fake audio data")
	body := &bytes.Buffer{}
	writer := multipart.NewWriter(body)
	part, err := writer.CreateFormFile("file", "test.mp3")
	require.NoError(t, err)
	_, err = part.Write(audioData)
	require.NoError(t, err)
	writer.Close()

	// Create test request
	req := httptest.NewRequest("POST", "/detect-language?offset=0&length=30", body)
	req.Header.Set("Content-Type", writer.FormDataContentType())

	// Execute request
	resp, err := server.app.Test(req, -1)
	require.NoError(t, err)
	defer resp.Body.Close()

	// Verify error response
	assert.Equal(t, fiber.StatusServiceUnavailable, resp.StatusCode)

	respBody, err := io.ReadAll(resp.Body)
	require.NoError(t, err)
	assert.Contains(t, string(respBody), "no workers available")
}

// TestHandleDetectLanguage_TempFileCleanup verifies temp files are cleaned up
func TestHandleDetectLanguage_TempFileCleanup(t *testing.T) {
	var capturedFilePath string

	mockClient := &MockGRPCClient{
		DetectLanguageFunc: func(ctx context.Context, workerAddr string, filePath string, offset float64, length float64) (*pb.DetectLanguageResponse, error) {
			capturedFilePath = filePath
			// Verify file exists during RPC call
			_, err := os.Stat(capturedFilePath)
			assert.NoError(t, err, "temp file should exist during RPC call")

			return &pb.DetectLanguageResponse{
				LanguageName: "English",
				LanguageCode: "en",
				Confidence:   0.95,
			}, nil
		},
	}

	mockWorkerPool := &MockWorkerPool{}
	server := createTestServerWithMocks(mockClient, mockWorkerPool)

	// Create test audio file
	audioData := []byte("fake audio data")
	body := &bytes.Buffer{}
	writer := multipart.NewWriter(body)
	part, err := writer.CreateFormFile("file", "test.mp3")
	require.NoError(t, err)
	_, err = part.Write(audioData)
	require.NoError(t, err)
	writer.Close()

	// Create test request
	req := httptest.NewRequest("POST", "/detect-language?offset=0&length=30", body)
	req.Header.Set("Content-Type", writer.FormDataContentType())

	// Execute request
	resp, err := server.app.Test(req, -1)
	require.NoError(t, err)
	defer resp.Body.Close()

	// Verify file was cleaned up after request
	_, err = os.Stat(capturedFilePath)
	assert.True(t, os.IsNotExist(err), "temp file should be deleted after request")
}

// TestHandleDetectLanguage_DefaultParameters tests default offset and length
func TestHandleDetectLanguage_DefaultParameters(t *testing.T) {
	mockClient := &MockGRPCClient{
		DetectLanguageFunc: func(ctx context.Context, workerAddr string, filePath string, offset float64, length float64) (*pb.DetectLanguageResponse, error) {
			// Verify defaults are used
			assert.Equal(t, 0.0, offset, "default offset should be 0")
			assert.Equal(t, 30.0, length, "default length should be 30")

			return &pb.DetectLanguageResponse{
				LanguageName: "Spanish",
				LanguageCode: "es",
				Confidence:   0.88,
			}, nil
		},
	}

	mockWorkerPool := &MockWorkerPool{}
	server := createTestServerWithMocks(mockClient, mockWorkerPool)

	// Create test audio file
	audioData := []byte("fake audio data")
	body := &bytes.Buffer{}
	writer := multipart.NewWriter(body)
	part, err := writer.CreateFormFile("file", "test.mp3")
	require.NoError(t, err)
	_, err = part.Write(audioData)
	require.NoError(t, err)
	writer.Close()

	// Create test request WITHOUT offset/length parameters
	req := httptest.NewRequest("POST", "/detect-language", body)
	req.Header.Set("Content-Type", writer.FormDataContentType())

	// Execute request
	resp, err := server.app.Test(req, -1)
	require.NoError(t, err)
	defer resp.Body.Close()

	// Verify response
	assert.Equal(t, fiber.StatusOK, resp.StatusCode)
}

// TestHandleDetectLanguage_CustomParameters tests custom offset and length
func TestHandleDetectLanguage_CustomParameters(t *testing.T) {
	mockClient := &MockGRPCClient{
		DetectLanguageFunc: func(ctx context.Context, workerAddr string, filePath string, offset float64, length float64) (*pb.DetectLanguageResponse, error) {
			// Verify custom values are used
			assert.Equal(t, 10.0, offset)
			assert.Equal(t, 15.0, length)

			return &pb.DetectLanguageResponse{
				LanguageName: "French",
				LanguageCode: "fr",
				Confidence:   0.92,
			}, nil
		},
	}

	mockWorkerPool := &MockWorkerPool{}
	server := createTestServerWithMocks(mockClient, mockWorkerPool)

	// Create test audio file
	audioData := []byte("fake audio data")
	body := &bytes.Buffer{}
	writer := multipart.NewWriter(body)
	part, err := writer.CreateFormFile("file", "test.mp3")
	require.NoError(t, err)
	_, err = part.Write(audioData)
	require.NoError(t, err)
	writer.Close()

	// Create test request with custom parameters
	req := httptest.NewRequest("POST", "/detect-language?offset=10&length=15", body)
	req.Header.Set("Content-Type", writer.FormDataContentType())

	// Execute request
	resp, err := server.app.Test(req, -1)
	require.NoError(t, err)
	defer resp.Body.Close()

	// Verify response
	assert.Equal(t, fiber.StatusOK, resp.StatusCode)

	respBody, err := io.ReadAll(resp.Body)
	require.NoError(t, err)
	assert.Contains(t, string(respBody), `"language":"French"`)
	assert.Contains(t, string(respBody), `"code":"fr"`)
}

// TestHandleDetectLanguage_InvalidParameterFormat tests non-numeric parameters
func TestHandleDetectLanguage_InvalidParameterFormat(t *testing.T) {
	testCases := []struct {
		name     string
		queryStr string
		errMsg   string
	}{
		{
			name:     "invalid offset format",
			queryStr: "offset=abc&length=30",
			errMsg:   "invalid offset parameter",
		},
		{
			name:     "invalid length format",
			queryStr: "offset=0&length=xyz",
			errMsg:   "invalid length parameter",
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			server := createTestServerWithMocks(&MockGRPCClient{}, &MockWorkerPool{})

			// Create test audio file
			audioData := []byte("fake audio data")
			body := &bytes.Buffer{}
			writer := multipart.NewWriter(body)
			part, err := writer.CreateFormFile("file", "test.mp3")
			require.NoError(t, err)
			_, err = part.Write(audioData)
			require.NoError(t, err)
			writer.Close()

			// Create test request
			req := httptest.NewRequest("POST", "/detect-language?"+tc.queryStr, body)
			req.Header.Set("Content-Type", writer.FormDataContentType())

			// Execute request
			resp, err := server.app.Test(req, -1)
			require.NoError(t, err)
			defer resp.Body.Close()

			// Verify error response
			assert.Equal(t, fiber.StatusBadRequest, resp.StatusCode)

			respBody, err := io.ReadAll(resp.Body)
			require.NoError(t, err)
			assert.Contains(t, string(respBody), tc.errMsg)
		})
	}
}
