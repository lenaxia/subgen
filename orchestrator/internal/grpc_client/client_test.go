package grpc_client

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/prometheus/client_golang/prometheus"
	"github.com/sirupsen/logrus"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"

	"github.com/mccloud/subgen/orchestrator/internal/queue"
	pb "github.com/mccloud/subgen/orchestrator/pkg/pb"
)

// Helper to create isolated metrics for each test
func newTestMetrics() *ClientMetrics {
	registry := prometheus.NewRegistry()
	return NewClientMetricsWithRegistry(registry)
}

// mockTranscriptionClient implements pb.TranscriptionServiceClient
type mockTranscriptionClient struct {
	transcribeFunc     func(ctx context.Context, in *pb.TranscribeRequest, opts ...grpc.CallOption) (*pb.TranscribeResponse, error)
	detectLanguageFunc func(ctx context.Context, in *pb.DetectLanguageRequest, opts ...grpc.CallOption) (*pb.DetectLanguageResponse, error)
	healthCheckFunc    func(ctx context.Context, in *pb.HealthCheckRequest, opts ...grpc.CallOption) (*pb.HealthCheckResponse, error)
}

func (m *mockTranscriptionClient) Transcribe(ctx context.Context, in *pb.TranscribeRequest, opts ...grpc.CallOption) (*pb.TranscribeResponse, error) {
	if m.transcribeFunc != nil {
		return m.transcribeFunc(ctx, in, opts...)
	}
	return &pb.TranscribeResponse{Success: true}, nil
}

func (m *mockTranscriptionClient) DetectLanguage(ctx context.Context, in *pb.DetectLanguageRequest, opts ...grpc.CallOption) (*pb.DetectLanguageResponse, error) {
	if m.detectLanguageFunc != nil {
		return m.detectLanguageFunc(ctx, in, opts...)
	}
	return &pb.DetectLanguageResponse{Success: true}, nil
}

func (m *mockTranscriptionClient) HealthCheck(ctx context.Context, in *pb.HealthCheckRequest, opts ...grpc.CallOption) (*pb.HealthCheckResponse, error) {
	if m.healthCheckFunc != nil {
		return m.healthCheckFunc(ctx, in, opts...)
	}
	return &pb.HealthCheckResponse{Status: pb.HealthCheckResponse_HEALTHY}, nil
}

func TestTranscribe_Success(t *testing.T) {
	// Setup
	log := logrus.New()
	log.SetLevel(logrus.FatalLevel) // Suppress logs during tests
	metrics := newTestMetrics()

	client := NewClient(5*time.Hour, 5*time.Second, 3, 1*time.Second, metrics, log)

	task := &queue.Task{
		FilePath:      "/path/to/video.mp4",
		TaskType:      "transcribe",
		ForceLanguage: "en",
	}

	// Mock gRPC client
	mockClient := &mockTranscriptionClient{
		transcribeFunc: func(ctx context.Context, in *pb.TranscribeRequest, opts ...grpc.CallOption) (*pb.TranscribeResponse, error) {
			assert.Equal(t, "/path/to/video.mp4", in.FilePath)
			assert.Equal(t, "transcribe", in.TaskType)
			assert.Equal(t, "en", in.ForceLanguage)

			return &pb.TranscribeResponse{
				Success:          true,
				SubtitlePath:     "/path/to/video.en.srt",
				DetectedLanguage: "en",
				Stats: &pb.TranscriptionStats{
					DurationSeconds:     120.5,
					SegmentCount:        45,
					TranscriptionTimeMs: 5000,
				},
			}, nil
		},
	}

	// Test
	ctx := context.Background()
	resp, err := client.transcribeWithClient(ctx, mockClient, task)

	// Verify
	require.NoError(t, err)
	assert.True(t, resp.Success)
	assert.Equal(t, "/path/to/video.en.srt", resp.SubtitlePath)
	assert.Equal(t, "en", resp.DetectedLanguage)
	assert.Equal(t, int32(45), resp.Stats.SegmentCount)
}

func TestTranscribe_Failure(t *testing.T) {
	// Setup
	log := logrus.New()
	log.SetLevel(logrus.FatalLevel)
	metrics := newTestMetrics()

	client := NewClient(5*time.Hour, 5*time.Second, 3, 1*time.Second, metrics, log)

	task := &queue.Task{
		FilePath: "/path/to/video.mp4",
		TaskType: "transcribe",
	}

	// Mock failure response
	mockClient := &mockTranscriptionClient{
		transcribeFunc: func(ctx context.Context, in *pb.TranscribeRequest, opts ...grpc.CallOption) (*pb.TranscribeResponse, error) {
			return &pb.TranscribeResponse{
				Success:      false,
				ErrorMessage: "file not found",
			}, nil
		},
	}

	// Test
	ctx := context.Background()
	resp, err := client.transcribeWithClient(ctx, mockClient, task)

	// Verify - should return response but with error
	require.Error(t, err)
	assert.Contains(t, err.Error(), "file not found")
	assert.False(t, resp.Success)
}

func TestTranscribe_RetryOnTransientError(t *testing.T) {
	// Setup
	log := logrus.New()
	log.SetLevel(logrus.FatalLevel)
	metrics := newTestMetrics()

	client := NewClient(5*time.Hour, 5*time.Second, 3, 10*time.Millisecond, metrics, log)

	task := &queue.Task{
		FilePath: "/path/to/video.mp4",
		TaskType: "transcribe",
	}

	// Mock transient error then success
	attempts := 0
	mockClient := &mockTranscriptionClient{
		transcribeFunc: func(ctx context.Context, in *pb.TranscribeRequest, opts ...grpc.CallOption) (*pb.TranscribeResponse, error) {
			attempts++
			if attempts < 3 {
				return nil, status.Error(codes.Unavailable, "temporary network error")
			}
			return &pb.TranscribeResponse{
				Success:      true,
				SubtitlePath: "/path/to/video.en.srt",
			}, nil
		},
	}

	// Test
	ctx := context.Background()
	resp, err := client.transcribeWithClient(ctx, mockClient, task)

	// Verify - should succeed after retries
	require.NoError(t, err)
	assert.True(t, resp.Success)
	assert.Equal(t, 3, attempts, "should retry 2 times before success")
}

func TestTranscribe_MaxRetriesExceeded(t *testing.T) {
	// Setup
	log := logrus.New()
	log.SetLevel(logrus.FatalLevel)
	metrics := newTestMetrics()

	client := NewClient(5*time.Hour, 5*time.Second, 3, 10*time.Millisecond, metrics, log)

	task := &queue.Task{
		FilePath: "/path/to/video.mp4",
		TaskType: "transcribe",
	}

	// Mock persistent error
	attempts := 0
	mockClient := &mockTranscriptionClient{
		transcribeFunc: func(ctx context.Context, in *pb.TranscribeRequest, opts ...grpc.CallOption) (*pb.TranscribeResponse, error) {
			attempts++
			return nil, status.Error(codes.Unavailable, "persistent error")
		},
	}

	// Test
	ctx := context.Background()
	_, err := client.transcribeWithClient(ctx, mockClient, task)

	// Verify - should fail after max retries
	require.Error(t, err)
	assert.Contains(t, err.Error(), "max retries exceeded")
	assert.Equal(t, 4, attempts, "should try 1 initial + 3 retries = 4 total")
}

func TestTranscribe_ContextCancellation(t *testing.T) {
	// Setup
	log := logrus.New()
	log.SetLevel(logrus.FatalLevel)
	metrics := newTestMetrics()

	client := NewClient(5*time.Hour, 5*time.Second, 3, 1*time.Second, metrics, log)

	task := &queue.Task{
		FilePath: "/path/to/video.mp4",
		TaskType: "transcribe",
	}

	// Mock slow response
	mockClient := &mockTranscriptionClient{
		transcribeFunc: func(ctx context.Context, in *pb.TranscribeRequest, opts ...grpc.CallOption) (*pb.TranscribeResponse, error) {
			select {
			case <-ctx.Done():
				return nil, ctx.Err()
			case <-time.After(2 * time.Second):
				return &pb.TranscribeResponse{Success: true}, nil
			}
		},
	}

	// Test with canceled context
	ctx, cancel := context.WithCancel(context.Background())
	cancel() // Cancel immediately

	_, err := client.transcribeWithClient(ctx, mockClient, task)

	// Verify - should fail with context canceled
	require.Error(t, err)
	assert.True(t, errors.Is(err, context.Canceled))
}

func TestDetectLanguage_Success(t *testing.T) {
	// Setup
	log := logrus.New()
	log.SetLevel(logrus.FatalLevel)
	metrics := newTestMetrics()

	client := NewClient(5*time.Hour, 5*time.Second, 3, 1*time.Second, metrics, log)

	// Mock gRPC client
	mockClient := &mockTranscriptionClient{
		detectLanguageFunc: func(ctx context.Context, in *pb.DetectLanguageRequest, opts ...grpc.CallOption) (*pb.DetectLanguageResponse, error) {
			assert.Equal(t, "/path/to/audio.mp3", in.GetFilePath())
			assert.Equal(t, int32(30), in.SampleLength)

			return &pb.DetectLanguageResponse{
				Success:      true,
				LanguageCode: "en",
				LanguageName: "English",
				Confidence:   0.95,
			}, nil
		},
	}

	// Test
	ctx := context.Background()
	resp, err := client.detectLanguageWithClient(ctx, mockClient, "/path/to/audio.mp3", 0.0, 30.0)

	// Verify
	require.NoError(t, err)
	assert.True(t, resp.Success)
	assert.Equal(t, "en", resp.LanguageCode)
	assert.Equal(t, "English", resp.LanguageName)
	assert.Equal(t, float32(0.95), resp.Confidence)
}

func TestDetectLanguage_Failure(t *testing.T) {
	// Setup
	log := logrus.New()
	log.SetLevel(logrus.FatalLevel)
	metrics := newTestMetrics()

	client := NewClient(5*time.Hour, 5*time.Second, 3, 1*time.Second, metrics, log)

	// Mock error
	mockClient := &mockTranscriptionClient{
		detectLanguageFunc: func(ctx context.Context, in *pb.DetectLanguageRequest, opts ...grpc.CallOption) (*pb.DetectLanguageResponse, error) {
			return nil, status.Error(codes.InvalidArgument, "invalid audio format")
		},
	}

	// Test
	ctx := context.Background()
	_, err := client.detectLanguageWithClient(ctx, mockClient, "/path/to/invalid.mp3", 0.0, 30.0)

	// Verify
	require.Error(t, err)
	assert.Contains(t, err.Error(), "invalid audio format")
}

func TestHealthCheck_Success(t *testing.T) {
	// Setup
	log := logrus.New()
	log.SetLevel(logrus.FatalLevel)
	metrics := newTestMetrics()

	client := NewClient(5*time.Hour, 5*time.Second, 3, 1*time.Second, metrics, log)

	// Mock gRPC client
	mockClient := &mockTranscriptionClient{
		healthCheckFunc: func(ctx context.Context, in *pb.HealthCheckRequest, opts ...grpc.CallOption) (*pb.HealthCheckResponse, error) {
			return &pb.HealthCheckResponse{
				Status:        pb.HealthCheckResponse_HEALTHY,
				MemoryMb:      1024,
				ModelLoaded:   true,
				JobsProcessed: 42,
				JobsActive:    2,
				Version:       "v1.0.0",
				UptimeSeconds: 3600,
			}, nil
		},
	}

	// Test
	ctx := context.Background()
	resp, err := client.healthCheckWithClient(ctx, mockClient)

	// Verify
	require.NoError(t, err)
	assert.Equal(t, pb.HealthCheckResponse_HEALTHY, resp.Status)
	assert.Equal(t, int64(1024), resp.MemoryMb)
	assert.True(t, resp.ModelLoaded)
	assert.Equal(t, int32(42), resp.JobsProcessed)
	assert.Equal(t, int32(2), resp.JobsActive)
}

func TestHealthCheck_Unhealthy(t *testing.T) {
	// Setup
	log := logrus.New()
	log.SetLevel(logrus.FatalLevel)
	metrics := newTestMetrics()

	client := NewClient(5*time.Hour, 5*time.Second, 3, 1*time.Second, metrics, log)

	// Mock unhealthy response
	mockClient := &mockTranscriptionClient{
		healthCheckFunc: func(ctx context.Context, in *pb.HealthCheckRequest, opts ...grpc.CallOption) (*pb.HealthCheckResponse, error) {
			return &pb.HealthCheckResponse{
				Status:   pb.HealthCheckResponse_UNHEALTHY,
				MemoryMb: 8192, // High memory
			}, nil
		},
	}

	// Test
	ctx := context.Background()
	resp, err := client.healthCheckWithClient(ctx, mockClient)

	// Verify - no error, but status is unhealthy
	require.NoError(t, err)
	assert.Equal(t, pb.HealthCheckResponse_UNHEALTHY, resp.Status)
}

func TestBuildMetadata_Plex(t *testing.T) {
	// Setup
	log := logrus.New()
	log.SetLevel(logrus.FatalLevel)
	metrics := newTestMetrics()

	client := NewClient(5*time.Hour, 5*time.Second, 3, 1*time.Second, metrics, log)

	task := &queue.Task{
		PlexItemID: "12345",
		PlexServer: "http://plex:32400",
		PlexToken:  "secret-token",
	}

	// Test
	metadata := client.buildMetadata(task)

	// Verify
	assert.Equal(t, "12345", metadata["plex_item_id"])
	assert.Equal(t, "http://plex:32400", metadata["plex_server"])
	assert.Equal(t, "secret-token", metadata["plex_token"])
	assert.Empty(t, metadata["jellyfin_item_id"])
}

func TestBuildMetadata_Jellyfin(t *testing.T) {
	// Setup
	log := logrus.New()
	log.SetLevel(logrus.FatalLevel)
	metrics := newTestMetrics()

	client := NewClient(5*time.Hour, 5*time.Second, 3, 1*time.Second, metrics, log)

	task := &queue.Task{
		JellyfinItemID: "abc-123",
		JellyfinServer: "http://jellyfin:8096",
		JellyfinToken:  "jf-token",
	}

	// Test
	metadata := client.buildMetadata(task)

	// Verify
	assert.Equal(t, "abc-123", metadata["jellyfin_item_id"])
	assert.Equal(t, "http://jellyfin:8096", metadata["jellyfin_server"])
	assert.Equal(t, "jf-token", metadata["jellyfin_token"])
	assert.Empty(t, metadata["plex_item_id"])
}

func TestBuildMetadata_Empty(t *testing.T) {
	// Setup
	log := logrus.New()
	log.SetLevel(logrus.FatalLevel)
	metrics := newTestMetrics()

	client := NewClient(5*time.Hour, 5*time.Second, 3, 1*time.Second, metrics, log)

	task := &queue.Task{}

	// Test
	metadata := client.buildMetadata(task)

	// Verify
	assert.Empty(t, metadata)
}

func TestRetryWithBackoff_ExponentialDelay(t *testing.T) {
	// Setup
	log := logrus.New()
	log.SetLevel(logrus.FatalLevel)
	metrics := newTestMetrics()

	client := NewClient(5*time.Hour, 5*time.Second, 3, 100*time.Millisecond, metrics, log)

	attempts := 0
	var delays []time.Duration
	lastTime := time.Now()

	fn := func() error {
		attempts++
		if attempts > 1 {
			delays = append(delays, time.Since(lastTime))
			lastTime = time.Now()
		}
		if attempts < 4 {
			// Return a retryable gRPC error
			return status.Error(codes.Unavailable, "retry me")
		}
		return nil
	}

	// Test
	ctx := context.Background()
	err := client.retryWithBackoff(ctx, fn)

	// Verify
	require.NoError(t, err)
	assert.Equal(t, 4, attempts)
	assert.Len(t, delays, 3, "should have 3 delays")

	// Verify exponential backoff: ~100ms, ~200ms, ~400ms
	assert.Greater(t, delays[0], 90*time.Millisecond)
	assert.Greater(t, delays[1], 180*time.Millisecond)
	assert.Greater(t, delays[2], 350*time.Millisecond)
}
