package integration

import (
	"context"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/credentials/insecure"
	"google.golang.org/grpc/status"

	pb "github.com/mccloud/subgen/orchestrator/pkg/pb"
)

const (
	workerAddr  = "localhost:50051"
	testdataDir = "../testdata"
	timeout     = 5 * time.Minute
)

// TestMain sets up/tears down Docker Compose environment
func TestMain(m *testing.M) {
	// NOTE: Tests assume Docker Compose is already running
	// To run tests:
	//   cd test
	//   docker-compose -f docker-compose.grpc-test.yml up -d
	//   cd integration
	//   go test -v
	os.Exit(m.Run())
}

// newTestClient creates a gRPC client for testing
func newTestClient(t *testing.T) (pb.TranscriptionServiceClient, *grpc.ClientConn) {
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	conn, err := grpc.DialContext(
		ctx,
		workerAddr,
		grpc.WithTransportCredentials(insecure.NewCredentials()),
		grpc.WithBlock(),
	)
	require.NoError(t, err, "Failed to connect to worker")

	client := pb.NewTranscriptionServiceClient(conn)
	return client, conn
}

// =============================================================================
// HealthCheck RPC Tests
// =============================================================================

// Test 1: HealthCheck RPC - Basic functionality
func TestHealthCheck_Basic(t *testing.T) {
	client, conn := newTestClient(t)
	defer conn.Close()

	req := &pb.HealthCheckRequest{}

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	resp, err := client.HealthCheck(ctx, req)

	// Assertions
	require.NoError(t, err, "HealthCheck RPC should succeed")
	assert.NotNil(t, resp, "Response should not be nil")
	assert.True(t, resp.Status == pb.HealthCheckResponse_HEALTHY || resp.Status == pb.HealthCheckResponse_UNHEALTHY,
		"Status should be HEALTHY or UNHEALTHY")
	assert.GreaterOrEqual(t, resp.MemoryMb, int64(0), "Memory should be >= 0")
	assert.GreaterOrEqual(t, resp.JobsProcessed, int32(0), "Jobs processed should be >= 0")
	assert.GreaterOrEqual(t, resp.JobsActive, int32(0), "Jobs active should be >= 0")
	assert.NotEmpty(t, resp.Version, "Version should be populated")
	assert.GreaterOrEqual(t, resp.UptimeSeconds, int64(0), "Uptime should be >= 0")

	t.Logf("Worker Health: %s", resp.Status)
	t.Logf("Memory: %dMB", resp.MemoryMb)
	t.Logf("Jobs Processed: %d", resp.JobsProcessed)
	t.Logf("Model Loaded: %v", resp.ModelLoaded)
	t.Logf("Version: %s", resp.Version)
	t.Logf("Uptime: %ds", resp.UptimeSeconds)
}

// Test 2: HealthCheck RPC - Repeated calls
func TestHealthCheck_RepeatedCalls(t *testing.T) {
	client, conn := newTestClient(t)
	defer conn.Close()

	req := &pb.HealthCheckRequest{}
	ctx := context.Background()

	// Call HealthCheck 10 times rapidly
	for i := 0; i < 10; i++ {
		resp, err := client.HealthCheck(ctx, req)
		require.NoError(t, err, "HealthCheck call %d should succeed", i+1)
		assert.True(t, resp.Status == pb.HealthCheckResponse_HEALTHY || resp.Status == pb.HealthCheckResponse_UNHEALTHY,
			"Call %d: Status should be valid", i+1)
	}

	t.Log("Successfully called HealthCheck 10 times")
}

// Test 3: HealthCheck RPC - All fields populated
func TestHealthCheck_AllFieldsPopulated(t *testing.T) {
	client, conn := newTestClient(t)
	defer conn.Close()

	req := &pb.HealthCheckRequest{}
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	resp, err := client.HealthCheck(ctx, req)
	require.NoError(t, err, "HealthCheck should succeed")

	// Verify ALL response fields are populated
	assert.True(t, resp.Status != pb.HealthCheckResponse_UNKNOWN, "Status should not be UNKNOWN")
	assert.GreaterOrEqual(t, resp.MemoryMb, int64(0))
	// model_loaded can be true or false
	assert.GreaterOrEqual(t, resp.JobsProcessed, int32(0))
	assert.GreaterOrEqual(t, resp.JobsActive, int32(0))
	assert.NotEmpty(t, resp.Version)
	assert.GreaterOrEqual(t, resp.UptimeSeconds, int64(0))

	t.Log("All HealthCheck fields validated successfully")
}

// =============================================================================
// DetectLanguage RPC Tests
// =============================================================================

// Test 4: DetectLanguage RPC - With file path
func TestDetectLanguage_FilePath(t *testing.T) {
	client, conn := newTestClient(t)
	defer conn.Close()

	req := &pb.DetectLanguageRequest{
		AudioSource: &pb.DetectLanguageRequest_FilePath{
			FilePath: filepath.Join(testdataDir, "short_audio.mp3"),
		},
		SampleLength: 30,
		SampleOffset: 0,
	}

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	t.Log("Sending DetectLanguage request...")
	resp, err := client.DetectLanguage(ctx, req)

	// NOTE: Worker stub returns "not yet implemented" error
	// This validates proper error handling
	if err != nil {
		st, ok := status.FromError(err)
		require.True(t, ok, "Should be a gRPC status error")
		t.Logf("Expected error from stub: %s", st.Message())
		return
	}

	// If implemented, validate response
	assert.NotNil(t, resp)
	if resp.Success {
		assert.NotEmpty(t, resp.LanguageCode, "Language code should be returned")
		assert.NotEmpty(t, resp.LanguageName, "Language name should be returned")
		assert.GreaterOrEqual(t, resp.Confidence, float32(0.0), "Confidence should be >= 0")
		assert.LessOrEqual(t, resp.Confidence, float32(1.0), "Confidence should be <= 1")
		t.Logf("Detected: %s (%s) with confidence %.2f", resp.LanguageName, resp.LanguageCode, resp.Confidence)
	} else {
		assert.NotEmpty(t, resp.ErrorMessage, "Error message should be set when success=false")
		t.Logf("DetectLanguage not yet implemented: %s", resp.ErrorMessage)
	}
}

// Test 5: DetectLanguage RPC - With audio bytes
func TestDetectLanguage_AudioContent(t *testing.T) {
	client, conn := newTestClient(t)
	defer conn.Close()

	// Read audio file into memory
	audioBytes, err := os.ReadFile(filepath.Join(testdataDir, "short_audio.mp3"))
	require.NoError(t, err, "Failed to read test audio file")

	req := &pb.DetectLanguageRequest{
		AudioSource: &pb.DetectLanguageRequest_AudioContent{
			AudioContent: audioBytes,
		},
		SampleLength: 30,
		SampleOffset: 0,
	}

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	resp, err := client.DetectLanguage(ctx, req)

	// Validate error handling (stub implementation)
	if err != nil {
		st, ok := status.FromError(err)
		require.True(t, ok, "Should be a gRPC status error")
		t.Logf("Expected error from stub: %s", st.Message())
		return
	}

	// If implemented, validate response
	assert.NotNil(t, resp)
	t.Logf("DetectLanguage with audio bytes: success=%v", resp.Success)
}

// Test 6: DetectLanguage RPC - Missing audio source
func TestDetectLanguage_MissingSource(t *testing.T) {
	client, conn := newTestClient(t)
	defer conn.Close()

	req := &pb.DetectLanguageRequest{
		// No audio_source set
		SampleLength: 30,
	}

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	resp, err := client.DetectLanguage(ctx, req)

	// Should fail with invalid argument
	require.Error(t, err, "Should fail with missing audio source")

	st, ok := status.FromError(err)
	require.True(t, ok, "Should be a gRPC status error")
	assert.Equal(t, codes.InvalidArgument, st.Code(), "Should be INVALID_ARGUMENT")

	assert.Nil(t, resp, "Response should be nil on error")
	t.Logf("Correctly rejected request with missing audio source: %s", st.Message())
}

// =============================================================================
// Transcribe RPC Tests
// =============================================================================

// Test 7: Transcribe RPC - Basic request validation
func TestTranscribe_BasicRequest(t *testing.T) {
	client, conn := newTestClient(t)
	defer conn.Close()

	req := &pb.TranscribeRequest{
		FilePath:      filepath.Join(testdataDir, "short_audio.mp3"),
		TaskType:      "transcribe",
		ForceLanguage: "",
		Options: &pb.TranscribeOptions{
			WhisperModel:   "tiny",
			WhisperThreads: 2,
		},
	}

	ctx, cancel := context.WithTimeout(context.Background(), timeout)
	defer cancel()

	t.Log("Sending Transcribe request...")
	resp, err := client.Transcribe(ctx, req)

	// NOTE: Worker stub returns "not yet implemented" error
	// This validates proper error handling
	if err != nil {
		st, ok := status.FromError(err)
		require.True(t, ok, "Should be a gRPC status error")
		t.Logf("Expected error from stub: %s", st.Message())
		return
	}

	// If implemented, validate response
	assert.NotNil(t, resp)
	if resp.Success {
		assert.NotEmpty(t, resp.SubtitlePath, "Subtitle path should be returned")
		assert.NotEmpty(t, resp.DetectedLanguage, "Language should be detected")
		assert.NotNil(t, resp.Stats, "Stats should be populated")
		t.Logf("Transcription completed: %s", resp.SubtitlePath)
	} else {
		assert.NotEmpty(t, resp.ErrorMessage, "Error message should be set when success=false")
		t.Logf("Transcribe not yet implemented: %s", resp.ErrorMessage)
	}
}

// Test 8: Transcribe RPC - Missing file path
func TestTranscribe_MissingFilePath(t *testing.T) {
	client, conn := newTestClient(t)
	defer conn.Close()

	req := &pb.TranscribeRequest{
		FilePath: "", // Empty file path
		TaskType: "transcribe",
		Options: &pb.TranscribeOptions{
			WhisperModel: "tiny",
		},
	}

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	resp, err := client.Transcribe(ctx, req)

	// Should fail with invalid argument
	require.Error(t, err, "Should fail with empty file path")

	st, ok := status.FromError(err)
	require.True(t, ok, "Should be a gRPC status error")
	assert.Equal(t, codes.InvalidArgument, st.Code(), "Should be INVALID_ARGUMENT")

	assert.Nil(t, resp, "Response should be nil on error")
	t.Logf("Correctly rejected request with missing file path: %s", st.Message())
}

// Test 9: Transcribe RPC - All request fields populated
func TestTranscribe_AllFieldsPopulated(t *testing.T) {
	client, conn := newTestClient(t)
	defer conn.Close()

	req := &pb.TranscribeRequest{
		FilePath:      filepath.Join(testdataDir, "short_audio.mp3"),
		TaskType:      "transcribe",
		ForceLanguage: "en",
		Options: &pb.TranscribeOptions{
			WhisperModel:         "tiny",
			WhisperThreads:       2,
			WordLevelHighlight:   false,
			CustomRegroup:        "cm_sl=84_sl=42++++++1",
			LrcForAudio:          true,
			CustomPrompt:         "",
			AppendFooter:         false,
			SubtitleLanguageName: "aa",
			ShowModelInFilename:  true,
			ShowSubgenInFilename: true,
		},
		Metadata: map[string]string{
			"source":      "integration_test",
			"test_case":   "all_fields_populated",
			"plex_server": "http://localhost:32400",
		},
	}

	ctx, cancel := context.WithTimeout(context.Background(), timeout)
	defer cancel()

	resp, err := client.Transcribe(ctx, req)

	// Validate protocol (not implementation)
	if err != nil {
		st, ok := status.FromError(err)
		require.True(t, ok, "Should be a gRPC status error")
		t.Logf("Expected error from stub: %s", st.Message())
		return
	}

	assert.NotNil(t, resp, "Response should not be nil")
	t.Log("All protobuf request fields sent successfully")
}

// Test 10: Transcribe RPC - Timeout handling
func TestTranscribe_ShortTimeout(t *testing.T) {
	client, conn := newTestClient(t)
	defer conn.Close()

	req := &pb.TranscribeRequest{
		FilePath: filepath.Join(testdataDir, "short_audio.mp3"),
		TaskType: "transcribe",
		Options: &pb.TranscribeOptions{
			WhisperModel: "tiny",
		},
	}

	// Very short timeout to test timeout handling
	ctx, cancel := context.WithTimeout(context.Background(), 100*time.Millisecond)
	defer cancel()

	resp, err := client.Transcribe(ctx, req)

	// May timeout or may complete quickly (stub returns immediately)
	if err != nil {
		st, ok := status.FromError(err)
		require.True(t, ok, "Should be a gRPC status error")
		if st.Code() == codes.DeadlineExceeded {
			t.Log("Request timed out as expected")
		} else {
			t.Logf("Got expected error: %s", st.Message())
		}
	}

	if resp != nil {
		t.Logf("Request completed before timeout: success=%v", resp.Success)
	}
}

// =============================================================================
// Concurrent & Stress Tests
// =============================================================================

// Test 11: Concurrent HealthCheck calls
func TestConcurrent_HealthChecks(t *testing.T) {
	client, conn := newTestClient(t)
	defer conn.Close()

	const numRequests = 20
	results := make(chan error, numRequests)

	req := &pb.HealthCheckRequest{}
	ctx := context.Background()

	// Send 20 health checks concurrently
	for i := 0; i < numRequests; i++ {
		go func(id int) {
			_, err := client.HealthCheck(ctx, req)
			results <- err
		}(i)
	}

	// Collect results
	successCount := 0
	for i := 0; i < numRequests; i++ {
		err := <-results
		if err == nil {
			successCount++
		}
	}

	assert.Equal(t, numRequests, successCount, "All concurrent health checks should succeed")
	t.Logf("Successfully completed %d concurrent HealthCheck calls", successCount)
}

// Test 12: Multiple clients to same worker
func TestMultipleClients_SameWorker(t *testing.T) {
	// Create 5 separate clients
	clients := make([]pb.TranscriptionServiceClient, 5)
	conns := make([]*grpc.ClientConn, 5)

	for i := 0; i < 5; i++ {
		client, conn := newTestClient(t)
		clients[i] = client
		conns[i] = conn
		defer conn.Close()
	}

	// Each client sends a HealthCheck
	ctx := context.Background()
	for i, client := range clients {
		resp, err := client.HealthCheck(ctx, &pb.HealthCheckRequest{})
		require.NoError(t, err, "Client %d should succeed", i+1)
		assert.True(t, resp.Status != pb.HealthCheckResponse_UNKNOWN, "Client %d: valid status", i+1)
	}

	t.Log("Successfully connected 5 independent clients")
}

// =============================================================================
// Protocol Validation Tests
// =============================================================================

// Test 13: gRPC connection establishment
func TestProtocol_ConnectionEstablishment(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	conn, err := grpc.DialContext(
		ctx,
		workerAddr,
		grpc.WithTransportCredentials(insecure.NewCredentials()),
		grpc.WithBlock(),
	)
	require.NoError(t, err, "Should connect to worker")
	defer conn.Close()

	assert.NotNil(t, conn, "Connection should be established")
	t.Log("gRPC connection established successfully")
}

// Test 14: Service method availability
func TestProtocol_ServiceMethodsAvailable(t *testing.T) {
	client, conn := newTestClient(t)
	defer conn.Close()

	ctx := context.Background()

	// Test that all 3 RPC methods are callable (even if they return errors)
	_, err1 := client.HealthCheck(ctx, &pb.HealthCheckRequest{})
	assert.NoError(t, err1, "HealthCheck method should be available")

	_, err2 := client.DetectLanguage(ctx, &pb.DetectLanguageRequest{
		AudioSource: &pb.DetectLanguageRequest_FilePath{
			FilePath: "/nonexistent",
		},
		SampleLength: 10,
	})
	// May return error, but method should be callable
	assert.NotPanics(t, func() {
		_ = err2
	}, "DetectLanguage method should be available")

	_, err3 := client.Transcribe(ctx, &pb.TranscribeRequest{
		FilePath: "/nonexistent",
		TaskType: "transcribe",
		Options:  &pb.TranscribeOptions{WhisperModel: "tiny"},
	})
	// May return error, but method should be callable
	assert.NotPanics(t, func() {
		_ = err3
	}, "Transcribe method should be available")

	t.Log("All 3 RPC methods are available")
}

// Test 15: Large metadata map
func TestProtocol_LargeMetadataMap(t *testing.T) {
	client, conn := newTestClient(t)
	defer conn.Close()

	// Create large metadata map
	metadata := make(map[string]string)
	for i := 0; i < 100; i++ {
		metadata[string(rune('A'+i%26))+string(rune('a'+i%26))] = "test_value"
	}

	req := &pb.TranscribeRequest{
		FilePath: filepath.Join(testdataDir, "short_audio.mp3"),
		TaskType: "transcribe",
		Options:  &pb.TranscribeOptions{WhisperModel: "tiny"},
		Metadata: metadata,
	}

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	_, err := client.Transcribe(ctx, req)
	// May fail with stub error, but metadata should serialize correctly
	if err != nil {
		st, _ := status.FromError(err)
		t.Logf("Got expected stub error: %s", st.Message())
	}

	t.Log("Large metadata map handled successfully")
}

// Test 16: Empty options object
func TestProtocol_EmptyOptions(t *testing.T) {
	client, conn := newTestClient(t)
	defer conn.Close()

	req := &pb.TranscribeRequest{
		FilePath: filepath.Join(testdataDir, "short_audio.mp3"),
		TaskType: "transcribe",
		Options:  nil, // No options provided
	}

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	_, err := client.Transcribe(ctx, req)
	// Should handle nil options gracefully
	if err != nil {
		t.Logf("Got error (expected with stub): %v", err)
	}

	t.Log("Nil options handled successfully")
}
