# Story 01: gRPC Integration Tests

**Epic**: EPIC_03 - Integration & Testing  
**Status**: Not Started  
**Priority**: Critical  
**Estimated Effort**: 8-10 hours  
**Assignee**: TBD

---

## User Story

As a **system integrator**,  
I want **comprehensive integration tests for gRPC communication between Go orchestrator and Python worker**,  
So that **I can verify all RPC methods work correctly end-to-end**.

---

## Context

The hybrid architecture relies on gRPC as the communication layer between the Go orchestrator and Python worker. This story creates integration tests that validate the gRPC protocol implementation on both sides works correctly with real network calls.

**Why This Matters:**
- gRPC is the critical integration point between Go and Python components
- Protobuf serialization/deserialization can fail silently if not tested
- Network timeouts, retries, and error handling must be validated
- Both sides must agree on message formats and field names

**Current State:**
- Go orchestrator gRPC client: `/home/mikekao/personal/subgen/orchestrator/internal/grpc_client/client.go`
- Python worker gRPC server: `/home/mikekao/personal/subgen/worker/src/grpc_server/service.py`
- Protobuf schema: `/home/mikekao/personal/subgen/api/transcription.proto`
- Unit tests exist for both sides, but NO integration tests

**Target State:**
- Integration test suite validating all 3 RPC methods
- Tests use real gRPC connections (not mocks)
- Docker Compose setup for local testing
- Coverage for happy paths and error scenarios

---

## Acceptance Criteria

- [ ] Docker Compose configuration for orchestrator + worker
- [ ] Go integration test file: `test/integration/grpc_integration_test.go`
- [ ] Python integration test file: `test/integration/test_grpc_integration.py`
- [ ] Test: Transcribe RPC with real file (success)
- [ ] Test: Transcribe RPC with missing file (error handling)
- [ ] Test: Transcribe RPC with timeout
- [ ] Test: DetectLanguage RPC with real audio file
- [ ] Test: DetectLanguage RPC with audio bytes
- [ ] Test: DetectLanguage RPC with invalid input
- [ ] Test: HealthCheck RPC (healthy status)
- [ ] Test: HealthCheck RPC (unhealthy when memory high)
- [ ] Test: gRPC connection retry on worker unavailable
- [ ] Test: protobuf field validation (all fields populated correctly)
- [ ] All tests pass locally with Docker Compose
- [ ] Work log created

---

## Technical Design

### Test Architecture

```
┌─────────────────────────────────────────────────────────────┐
│  Integration Test Environment (Docker Compose)              │
├─────────────────────────────────────────────────────────────┤
│                                                              │
│  ┌───────────────┐         gRPC         ┌──────────────┐   │
│  │               │ ──────────────────> │              │   │
│  │  Go Test      │   localhost:50051   │  Python      │   │
│  │  Client       │ <────────────────── │  Worker      │   │
│  │               │                      │              │   │
│  └───────────────┘                      └──────────────┘   │
│        │                                       │            │
│        │                                       │            │
│        └───────────────┬───────────────────────┘            │
│                        │                                    │
│                        ↓                                    │
│              ┌──────────────────┐                          │
│              │  Shared Test     │                          │
│              │  Data Volume     │                          │
│              │  /testdata       │                          │
│              └──────────────────┘                          │
│                                                              │
└─────────────────────────────────────────────────────────────┘
```

### File Structure

```
test/
├── docker-compose.integration.yml     # Orchestrator + Worker setup
├── integration/
│   ├── grpc_integration_test.go       # Go tests
│   └── test_grpc_integration.py       # Python tests
├── testdata/
│   ├── short_audio.mp3                # 30 sec English audio
│   ├── long_audio.mp3                 # 5 min English audio
│   ├── spanish_audio.mp3              # 30 sec Spanish audio
│   ├── video.mkv                      # 1 min video with audio
│   ├── corrupt_audio.mp3              # Invalid audio file
│   └── README.md                      # Test data documentation
└── scripts/
    ├── generate_test_audio.sh         # Generate synthetic audio
    └── run_integration_tests.sh       # Test runner script
```

---

## Implementation Steps

### Step 1: Create Docker Compose Configuration

**File: `/home/mikekao/personal/subgen/test/docker-compose.integration.yml`**

```yaml
version: '3.8'

services:
  # Python worker service
  worker:
    build:
      context: ../worker
      dockerfile: Dockerfile
    container_name: subgen-worker-integration
    environment:
      GRPC_PORT: "50051"
      WHISPER_MODEL: "tiny"              # Tiny model for fast tests
      WHISPER_THREADS: "2"
      MEMORY_THRESHOLD_MB: "2000"
      LOG_LEVEL: "DEBUG"
      MODEL_CLEANUP_DELAY: "5"           # Short delay for testing
      CLEAR_VRAM_ON_COMPLETE: "false"
    ports:
      - "50051:50051"
    volumes:
      - ./testdata:/testdata:ro          # Read-only test data
      - ../models:/models:rw             # Model storage
    healthcheck:
      test: ["CMD", "python", "-c", "import grpc; grpc.insecure_channel('localhost:50051').channel_ready_future(timeout=5).result()"]
      interval: 5s
      timeout: 10s
      retries: 5
      start_period: 30s
    networks:
      - subgen-integration

  # Go orchestrator (for testing gRPC client)
  orchestrator:
    build:
      context: ../orchestrator
      dockerfile: Dockerfile
    container_name: subgen-orchestrator-integration
    environment:
      WORKER_DISCOVERY: "static"
      PYTHON_WORKER_ADDRESS: "worker:50051"
      WEBHOOK_PORT: "9000"
      METRICS_PORT: "9090"
      LOG_LEVEL: "DEBUG"
    ports:
      - "9000:9000"
      - "9090:9090"
    depends_on:
      worker:
        condition: service_healthy
    volumes:
      - ./testdata:/testdata:ro
    networks:
      - subgen-integration

networks:
  subgen-integration:
    driver: bridge
```

**Validation:**
```bash
cd test
docker-compose -f docker-compose.integration.yml up -d
docker-compose -f docker-compose.integration.yml ps
# Should show both services healthy
```

---

### Step 2: Generate Test Audio Files

**File: `/home/mikekao/personal/subgen/test/scripts/generate_test_audio.sh`**

```bash
#!/bin/bash
# Generate synthetic test audio files using ffmpeg

set -e

TESTDATA_DIR="../testdata"
mkdir -p "$TESTDATA_DIR"

echo "Generating test audio files..."

# 1. Short audio (30 seconds, 440Hz sine wave)
echo "  - short_audio.mp3 (30s)"
ffmpeg -f lavfi -i "sine=frequency=440:duration=30" \
  -ac 1 -ar 16000 -ab 64k \
  "$TESTDATA_DIR/short_audio.mp3" -y 2>/dev/null

# 2. Long audio (5 minutes, 440Hz + 880Hz mix)
echo "  - long_audio.mp3 (5min)"
ffmpeg -f lavfi -i "sine=frequency=440:duration=300" \
  -f lavfi -i "sine=frequency=880:duration=300" \
  -filter_complex "[0:a][1:a]amix=inputs=2:duration=first" \
  -ac 1 -ar 16000 -ab 64k \
  "$TESTDATA_DIR/long_audio.mp3" -y 2>/dev/null

# 3. Spanish audio (silence, for testing - Whisper will detect as 'unknown')
echo "  - spanish_audio.mp3 (30s silence)"
ffmpeg -f lavfi -i "anullsrc=r=16000:cl=mono:d=30" \
  -ab 64k \
  "$TESTDATA_DIR/spanish_audio.mp3" -y 2>/dev/null

# 4. Video with audio (1 minute, color bars + tone)
echo "  - video.mkv (1min)"
ffmpeg -f lavfi -i "testsrc=duration=60:size=640x480:rate=30" \
  -f lavfi -i "sine=frequency=440:duration=60" \
  -c:v libx264 -c:a aac -ab 128k \
  "$TESTDATA_DIR/video.mkv" -y 2>/dev/null

# 5. Corrupt audio (invalid data)
echo "  - corrupt_audio.mp3 (invalid)"
echo "This is not valid audio data" > "$TESTDATA_DIR/corrupt_audio.mp3"

# 6. Audio-only file for LRC testing
echo "  - audio_only.m4a (30s)"
ffmpeg -f lavfi -i "sine=frequency=440:duration=30" \
  -ac 1 -ar 16000 -c:a aac -ab 64k \
  "$TESTDATA_DIR/audio_only.m4a" -y 2>/dev/null

echo "Test audio files generated successfully!"
ls -lh "$TESTDATA_DIR"
```

**Usage:**
```bash
cd test/scripts
chmod +x generate_test_audio.sh
./generate_test_audio.sh
```

---

### Step 3: Go Integration Tests

**File: `/home/mikekao/personal/subgen/test/integration/grpc_integration_test.go`**

```go
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
	"google.golang.org/grpc/credentials/insecure"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"

	pb "github.com/mccloud/subgen/orchestrator/pkg/pb"
)

const (
	workerAddr = "localhost:50051"
	testdataDir = "../testdata"
	timeout = 5 * time.Minute
)

// TestMain sets up/tears down Docker Compose environment
func TestMain(m *testing.M) {
	// Assume Docker Compose is running
	// In CI, would start/stop here
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

// Test 1: Transcribe RPC - Success Path
func TestTranscribe_Success(t *testing.T) {
	client, conn := newTestClient(t)
	defer conn.Close()

	req := &pb.TranscribeRequest{
		FilePath:      filepath.Join(testdataDir, "short_audio.mp3"),
		TaskType:      "transcribe",
		ForceLanguage: "",  // Auto-detect
		Options: &pb.TranscribeOptions{
			WhisperModel:   "tiny",
			WhisperThreads: 2,
		},
	}

	ctx, cancel := context.WithTimeout(context.Background(), timeout)
	defer cancel()

	t.Log("Sending Transcribe request...")
	resp, err := client.Transcribe(ctx, req)

	// Assertions
	require.NoError(t, err, "Transcribe RPC should succeed")
	assert.True(t, resp.Success, "Response should indicate success")
	assert.NotEmpty(t, resp.SubtitlePath, "Subtitle path should be returned")
	assert.NotEmpty(t, resp.DetectedLanguage, "Language should be detected")

	// Verify subtitle file was created
	_, err = os.Stat(resp.SubtitlePath)
	assert.NoError(t, err, "Subtitle file should exist: %s", resp.SubtitlePath)

	// Validate stats
	assert.NotNil(t, resp.Stats, "Stats should be populated")
	assert.Greater(t, resp.Stats.DurationSeconds, float32(0), "Duration should be > 0")
	assert.Greater(t, resp.Stats.SegmentCount, int32(0), "Should have segments")

	t.Logf("Transcription completed: %s", resp.SubtitlePath)
	t.Logf("Detected language: %s", resp.DetectedLanguage)
	t.Logf("Segments: %d, Duration: %.2fs", resp.Stats.SegmentCount, resp.Stats.DurationSeconds)
}

// Test 2: Transcribe RPC - File Not Found
func TestTranscribe_FileNotFound(t *testing.T) {
	client, conn := newTestClient(t)
	defer conn.Close()

	req := &pb.TranscribeRequest{
		FilePath: "/nonexistent/file.mp3",
		TaskType: "transcribe",
		Options: &pb.TranscribeOptions{
			WhisperModel: "tiny",
		},
	}

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	resp, err := client.Transcribe(ctx, req)

	// Should get error response
	require.Error(t, err, "Should fail with file not found")
	
	// Check gRPC status code
	st, ok := status.FromError(err)
	require.True(t, ok, "Should be a gRPC status error")
	assert.Equal(t, codes.NotFound, st.Code(), "Should be NOT_FOUND error")

	assert.Nil(t, resp, "Response should be nil on error")
}

// Test 3: Transcribe RPC - Timeout
func TestTranscribe_Timeout(t *testing.T) {
	client, conn := newTestClient(t)
	defer conn.Close()

	req := &pb.TranscribeRequest{
		FilePath: filepath.Join(testdataDir, "long_audio.mp3"),
		TaskType: "transcribe",
		Options: &pb.TranscribeOptions{
			WhisperModel: "tiny",
		},
	}

	// Very short timeout to force timeout
	ctx, cancel := context.WithTimeout(context.Background(), 1*time.Second)
	defer cancel()

	resp, err := client.Transcribe(ctx, req)

	// Should timeout
	require.Error(t, err, "Should fail with timeout")
	
	st, ok := status.FromError(err)
	require.True(t, ok, "Should be a gRPC status error")
	assert.Equal(t, codes.DeadlineExceeded, st.Code(), "Should be DEADLINE_EXCEEDED")

	assert.Nil(t, resp, "Response should be nil on timeout")
}

// Test 4: Transcribe RPC - Invalid Audio File
func TestTranscribe_InvalidAudio(t *testing.T) {
	client, conn := newTestClient(t)
	defer conn.Close()

	req := &pb.TranscribeRequest{
		FilePath: filepath.Join(testdataDir, "corrupt_audio.mp3"),
		TaskType: "transcribe",
		Options: &pb.TranscribeOptions{
			WhisperModel: "tiny",
		},
	}

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	resp, err := client.Transcribe(ctx, req)

	// Should get error
	require.Error(t, err, "Should fail with invalid audio")
	
	st, ok := status.FromError(err)
	require.True(t, ok, "Should be a gRPC status error")
	assert.Equal(t, codes.Internal, st.Code(), "Should be INTERNAL error")

	assert.Nil(t, resp, "Response should be nil on error")
}

// Test 5: DetectLanguage RPC - With File Path
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

	// Assertions
	require.NoError(t, err, "DetectLanguage RPC should succeed")
	assert.True(t, resp.Success, "Response should indicate success")
	assert.NotEmpty(t, resp.LanguageCode, "Language code should be returned")
	assert.NotEmpty(t, resp.LanguageName, "Language name should be returned")
	assert.GreaterOrEqual(t, resp.Confidence, float32(0.0), "Confidence should be >= 0")
	assert.LessOrEqual(t, resp.Confidence, float32(1.0), "Confidence should be <= 1")

	t.Logf("Detected: %s (%s) with confidence %.2f", resp.LanguageName, resp.LanguageCode, resp.Confidence)
}

// Test 6: DetectLanguage RPC - With Audio Bytes
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

	// Assertions
	require.NoError(t, err, "DetectLanguage RPC should succeed")
	assert.True(t, resp.Success, "Response should indicate success")
	assert.NotEmpty(t, resp.LanguageCode, "Language code should be returned")
}

// Test 7: DetectLanguage RPC - Missing Audio Source
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
}

// Test 8: HealthCheck RPC - Healthy Status
func TestHealthCheck_Healthy(t *testing.T) {
	client, conn := newTestClient(t)
	defer conn.Close()

	req := &pb.HealthCheckRequest{}

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	resp, err := client.HealthCheck(ctx, req)

	// Assertions
	require.NoError(t, err, "HealthCheck RPC should succeed")
	assert.Equal(t, pb.HealthCheckResponse_HEALTHY, resp.Status, "Worker should be healthy")
	assert.Greater(t, resp.MemoryMb, int64(0), "Memory should be > 0")
	assert.GreaterOrEqual(t, resp.JobsProcessed, int32(0), "Jobs processed should be >= 0")
	assert.NotEmpty(t, resp.Version, "Version should be populated")

	t.Logf("Worker Health: %s", resp.Status)
	t.Logf("Memory: %dMB", resp.MemoryMb)
	t.Logf("Jobs Processed: %d", resp.JobsProcessed)
	t.Logf("Model Loaded: %v", resp.ModelLoaded)
}

// Test 9: HealthCheck RPC - Repeated Calls
func TestHealthCheck_RepeatedCalls(t *testing.T) {
	client, conn := newTestClient(t)
	defer conn.Close()

	req := &pb.HealthCheckRequest{}
	ctx := context.Background()

	// Call HealthCheck 10 times rapidly
	for i := 0; i < 10; i++ {
		resp, err := client.HealthCheck(ctx, req)
		require.NoError(t, err, "HealthCheck call %d should succeed", i+1)
		assert.Equal(t, pb.HealthCheckResponse_HEALTHY, resp.Status)
	}

	t.Log("Successfully called HealthCheck 10 times")
}

// Test 10: Protobuf Field Validation - All Fields Populated
func TestTranscribe_AllFieldsPopulated(t *testing.T) {
	client, conn := newTestClient(t)
	defer conn.Close()

	req := &pb.TranscribeRequest{
		FilePath:      filepath.Join(testdataDir, "short_audio.mp3"),
		TaskType:      "transcribe",
		ForceLanguage: "en",
		Options: &pb.TranscribeOptions{
			WhisperModel:          "tiny",
			WhisperThreads:        2,
			WordLevelHighlight:    false,
			CustomRegroup:         "cm_sl=84_sl=42++++++1",
			LrcForAudio:           true,
			CustomPrompt:          "",
			AppendFooter:          false,
			SubtitleLanguageName:  "aa",
			ShowModelInFilename:   true,
			ShowSubgenInFilename:  true,
		},
		Metadata: map[string]string{
			"source": "integration_test",
		},
	}

	ctx, cancel := context.WithTimeout(context.Background(), timeout)
	defer cancel()

	resp, err := client.Transcribe(ctx, req)

	require.NoError(t, err, "Transcribe should succeed")
	
	// Verify ALL response fields are populated
	assert.True(t, resp.Success)
	assert.NotEmpty(t, resp.SubtitlePath)
	assert.NotEmpty(t, resp.DetectedLanguage)
	assert.Empty(t, resp.ErrorMessage, "ErrorMessage should be empty on success")
	
	// Verify stats
	require.NotNil(t, resp.Stats)
	assert.Greater(t, resp.Stats.DurationSeconds, float32(0))
	assert.Greater(t, resp.Stats.SegmentCount, int32(0))
	assert.GreaterOrEqual(t, resp.Stats.ModelLoadTimeMs, int64(0))
	assert.Greater(t, resp.Stats.TranscriptionTimeMs, int64(0))
	assert.GreaterOrEqual(t, resp.Stats.PeakMemoryMb, int64(0))

	t.Log("All protobuf fields validated successfully")
}

// Test 11: Connection Retry - Worker Restart
func TestConnection_RetryOnRestart(t *testing.T) {
	t.Skip("Requires Docker Compose control - manual test only")

	// This test would:
	// 1. Stop worker container
	// 2. Attempt gRPC call (should fail)
	// 3. Start worker container
	// 4. Retry gRPC call (should succeed)
	
	// Implementation left for manual testing or CI environment
}

// Test 12: Concurrent Requests
func TestTranscribe_ConcurrentRequests(t *testing.T) {
	client, conn := newTestClient(t)
	defer conn.Close()

	// Send 5 transcription requests concurrently
	const numRequests = 5
	results := make(chan error, numRequests)

	for i := 0; i < numRequests; i++ {
		go func(id int) {
			req := &pb.TranscribeRequest{
				FilePath: filepath.Join(testdataDir, "short_audio.mp3"),
				TaskType: "transcribe",
				Options: &pb.TranscribeOptions{
					WhisperModel: "tiny",
				},
			}

			ctx, cancel := context.WithTimeout(context.Background(), timeout)
			defer cancel()

			resp, err := client.Transcribe(ctx, req)
			if err != nil {
				results <- err
				return
			}

			if !resp.Success {
				results <- assert.AnError
				return
			}

			results <- nil
		}(i)
	}

	// Collect results
	for i := 0; i < numRequests; i++ {
		err := <-results
		assert.NoError(t, err, "Request %d should succeed", i+1)
	}

	t.Log("All 5 concurrent requests completed successfully")
}
```

---

### Step 4: Python Integration Tests

**File: `/home/mikekao/personal/subgen/test/integration/test_grpc_integration.py`**

```python
"""
Python integration tests for gRPC communication.

Tests the Python worker's gRPC server from a Python client perspective.
Complements the Go integration tests.
"""

import os
import time
import pytest
import grpc
from pathlib import Path

from pb import transcription_pb2
from pb import transcription_pb2_grpc


WORKER_ADDR = "localhost:50051"
TESTDATA_DIR = Path(__file__).parent.parent / "testdata"
TIMEOUT = 300  # 5 minutes


@pytest.fixture(scope="session")
def grpc_channel():
    """Create gRPC channel for testing."""
    channel = grpc.insecure_channel(WORKER_ADDR)
    
    # Wait for channel to be ready
    try:
        grpc.channel_ready_future(channel).result(timeout=10)
    except grpc.FutureTimeoutError:
        pytest.fail(f"Failed to connect to worker at {WORKER_ADDR}")
    
    yield channel
    channel.close()


@pytest.fixture
def client(grpc_channel):
    """Create gRPC client stub."""
    return transcription_pb2_grpc.TranscriptionServiceStub(grpc_channel)


class TestTranscribeRPC:
    """Tests for Transcribe RPC method."""

    def test_transcribe_success(self, client):
        """Test successful transcription."""
        request = transcription_pb2.TranscribeRequest(
            file_path=str(TESTDATA_DIR / "short_audio.mp3"),
            task_type="transcribe",
            force_language="",  # Auto-detect
            options=transcription_pb2.TranscribeOptions(
                whisper_model="tiny",
                whisper_threads=2,
            ),
        )

        response = client.Transcribe(request, timeout=TIMEOUT)

        # Assertions
        assert response.success is True
        assert response.subtitle_path != ""
        assert response.detected_language != ""
        assert response.error_message == ""

        # Verify subtitle file exists
        assert os.path.exists(response.subtitle_path)

        # Validate stats
        assert response.stats.duration_seconds > 0
        assert response.stats.segment_count > 0
        assert response.stats.transcription_time_ms > 0

        print(f"Transcription completed: {response.subtitle_path}")
        print(f"Language: {response.detected_language}")

    def test_transcribe_file_not_found(self, client):
        """Test transcription with missing file."""
        request = transcription_pb2.TranscribeRequest(
            file_path="/nonexistent/file.mp3",
            task_type="transcribe",
            options=transcription_pb2.TranscribeOptions(
                whisper_model="tiny",
            ),
        )

        with pytest.raises(grpc.RpcError) as exc_info:
            client.Transcribe(request, timeout=30)

        # Check status code
        assert exc_info.value.code() == grpc.StatusCode.NOT_FOUND
        assert "not found" in exc_info.value.details().lower()

    def test_transcribe_invalid_task_type(self, client):
        """Test transcription with invalid task type."""
        request = transcription_pb2.TranscribeRequest(
            file_path=str(TESTDATA_DIR / "short_audio.mp3"),
            task_type="invalid_type",
            options=transcription_pb2.TranscribeOptions(
                whisper_model="tiny",
            ),
        )

        with pytest.raises(grpc.RpcError) as exc_info:
            client.Transcribe(request, timeout=30)

        # Should fail with validation error
        assert exc_info.value.code() in [
            grpc.StatusCode.INVALID_ARGUMENT,
            grpc.StatusCode.INTERNAL,
        ]

    def test_transcribe_forced_language(self, client):
        """Test transcription with forced language."""
        request = transcription_pb2.TranscribeRequest(
            file_path=str(TESTDATA_DIR / "short_audio.mp3"),
            task_type="transcribe",
            force_language="en",  # Force English
            options=transcription_pb2.TranscribeOptions(
                whisper_model="tiny",
            ),
        )

        response = client.Transcribe(request, timeout=TIMEOUT)

        assert response.success is True
        assert response.detected_language == "en"


class TestDetectLanguageRPC:
    """Tests for DetectLanguage RPC method."""

    def test_detect_language_file_path(self, client):
        """Test language detection with file path."""
        request = transcription_pb2.DetectLanguageRequest(
            file_path=str(TESTDATA_DIR / "short_audio.mp3"),
            sample_length=30,
            sample_offset=0,
        )

        response = client.DetectLanguage(request, timeout=30)

        # Assertions
        assert response.success is True
        assert response.language_code != ""
        assert response.language_name != ""
        assert 0.0 <= response.confidence <= 1.0
        assert response.error_message == ""

        print(f"Detected: {response.language_name} ({response.language_code})")
        print(f"Confidence: {response.confidence:.2f}")

    def test_detect_language_audio_content(self, client):
        """Test language detection with audio bytes."""
        # Read audio file
        audio_path = TESTDATA_DIR / "short_audio.mp3"
        with open(audio_path, "rb") as f:
            audio_content = f.read()

        request = transcription_pb2.DetectLanguageRequest(
            audio_content=audio_content,
            sample_length=30,
            sample_offset=0,
        )

        response = client.DetectLanguage(request, timeout=30)

        assert response.success is True
        assert response.language_code != ""

    def test_detect_language_missing_source(self, client):
        """Test language detection without audio source."""
        request = transcription_pb2.DetectLanguageRequest(
            sample_length=30,
            # No file_path or audio_content
        )

        with pytest.raises(grpc.RpcError) as exc_info:
            client.DetectLanguage(request, timeout=10)

        assert exc_info.value.code() == grpc.StatusCode.INVALID_ARGUMENT

    def test_detect_language_invalid_file(self, client):
        """Test language detection with corrupt audio."""
        request = transcription_pb2.DetectLanguageRequest(
            file_path=str(TESTDATA_DIR / "corrupt_audio.mp3"),
            sample_length=30,
        )

        with pytest.raises(grpc.RpcError) as exc_info:
            client.DetectLanguage(request, timeout=30)

        # Should fail with internal error
        assert exc_info.value.code() == grpc.StatusCode.INTERNAL


class TestHealthCheckRPC:
    """Tests for HealthCheck RPC method."""

    def test_health_check_healthy(self, client):
        """Test health check returns healthy status."""
        request = transcription_pb2.HealthCheckRequest()

        response = client.HealthCheck(request, timeout=5)

        # Assertions
        assert response.status == transcription_pb2.HealthCheckResponse.HEALTHY
        assert response.memory_mb > 0
        assert response.jobs_processed >= 0
        assert response.jobs_active >= 0
        assert response.version != ""
        assert response.uptime_seconds >= 0

        print(f"Worker Status: {response.status}")
        print(f"Memory: {response.memory_mb}MB")
        print(f"Jobs Processed: {response.jobs_processed}")

    def test_health_check_repeated(self, client):
        """Test health check can be called repeatedly."""
        request = transcription_pb2.HealthCheckRequest()

        for i in range(10):
            response = client.HealthCheck(request, timeout=5)
            assert response.status in [
                transcription_pb2.HealthCheckResponse.HEALTHY,
                transcription_pb2.HealthCheckResponse.UNHEALTHY,
            ]

        print("Successfully called HealthCheck 10 times")

    def test_health_check_memory_tracking(self, client):
        """Test that health check tracks memory over time."""
        request = transcription_pb2.HealthCheckRequest()

        # Get baseline memory
        baseline = client.HealthCheck(request, timeout=5)
        baseline_memory = baseline.memory_mb

        # Perform transcription (increases memory)
        transcribe_req = transcription_pb2.TranscribeRequest(
            file_path=str(TESTDATA_DIR / "short_audio.mp3"),
            task_type="transcribe",
            options=transcription_pb2.TranscribeOptions(whisper_model="tiny"),
        )
        client.Transcribe(transcribe_req, timeout=TIMEOUT)

        # Check memory again (might be higher)
        after = client.HealthCheck(request, timeout=5)
        after_memory = after.memory_mb

        print(f"Memory baseline: {baseline_memory}MB")
        print(f"Memory after transcription: {after_memory}MB")

        # Memory should be tracked (may increase or stay same)
        assert after_memory >= 0


class TestProtobufValidation:
    """Tests for protobuf field validation."""

    def test_all_transcribe_fields_populated(self, client):
        """Test that all TranscribeResponse fields are populated."""
        request = transcription_pb2.TranscribeRequest(
            file_path=str(TESTDATA_DIR / "short_audio.mp3"),
            task_type="transcribe",
            force_language="en",
            options=transcription_pb2.TranscribeOptions(
                whisper_model="tiny",
                whisper_threads=2,
                word_level_highlight=False,
                custom_regroup="cm_sl=84_sl=42++++++1",
                lrc_for_audio=True,
                custom_prompt="",
                append_footer=False,
                subtitle_language_name="aa",
                show_model_in_filename=True,
                show_subgen_in_filename=True,
            ),
            metadata={"source": "integration_test"},
        )

        response = client.Transcribe(request, timeout=TIMEOUT)

        # Verify all fields
        assert response.success is True
        assert response.subtitle_path != ""
        assert response.detected_language != ""
        assert response.error_message == ""

        # Verify stats (all fields)
        assert response.stats.duration_seconds > 0
        assert response.stats.segment_count > 0
        assert response.stats.model_load_time_ms >= 0
        assert response.stats.transcription_time_ms > 0
        assert response.stats.peak_memory_mb >= 0

    def test_all_detect_language_fields_populated(self, client):
        """Test that all DetectLanguageResponse fields are populated."""
        request = transcription_pb2.DetectLanguageRequest(
            file_path=str(TESTDATA_DIR / "short_audio.mp3"),
            sample_length=30,
            sample_offset=0,
        )

        response = client.DetectLanguage(request, timeout=30)

        # Verify all fields
        assert response.success is True
        assert response.language_code != ""
        assert response.language_name != ""
        assert 0.0 <= response.confidence <= 1.0
        assert response.error_message == ""


class TestConcurrency:
    """Tests for concurrent requests."""

    def test_concurrent_transcriptions(self, client):
        """Test multiple concurrent transcription requests."""
        import concurrent.futures

        def transcribe():
            request = transcription_pb2.TranscribeRequest(
                file_path=str(TESTDATA_DIR / "short_audio.mp3"),
                task_type="transcribe",
                options=transcription_pb2.TranscribeOptions(whisper_model="tiny"),
            )
            response = client.Transcribe(request, timeout=TIMEOUT)
            return response.success

        # Run 5 concurrent requests
        with concurrent.futures.ThreadPoolExecutor(max_workers=5) as executor:
            futures = [executor.submit(transcribe) for _ in range(5)]
            results = [f.result() for f in concurrent.futures.as_completed(futures)]

        # All should succeed
        assert all(results), "All concurrent requests should succeed"
        print("All 5 concurrent requests completed successfully")


if __name__ == "__main__":
    pytest.main([__file__, "-v"])
```

---

## Definition of Done

- [ ] All acceptance criteria met
- [ ] Docker Compose configuration created and working
- [ ] Test data generated (6 audio/video files)
- [ ] Go integration tests: 12+ tests passing
- [ ] Python integration tests: 15+ tests passing
- [ ] Tests cover all 3 RPC methods (Transcribe, DetectLanguage, HealthCheck)
- [ ] Tests cover happy paths and error scenarios
- [ ] All protobuf fields validated
- [ ] Concurrent request test passing
- [ ] Tests run successfully in Docker Compose environment
- [ ] Documentation: Test runner script with usage instructions
- [ ] Work log created: `docs/WORKLOGS/NNNN_YYYY-MM-DD_EPIC_03_story_01.md`
- [ ] Code committed and pushed

---

## Validation Commands

```bash
# Generate test audio files
cd test/scripts
./generate_test_audio.sh

# Start Docker Compose environment
cd test
docker-compose -f docker-compose.integration.yml up -d

# Wait for services to be healthy
docker-compose -f docker-compose.integration.yml ps

# Run Go integration tests
cd test/integration
go test -v -run TestTranscribe
go test -v -run TestDetectLanguage
go test -v -run TestHealthCheck

# Run all Go integration tests
go test -v ./...

# Run Python integration tests
cd test/integration
pytest test_grpc_integration.py -v

# Run specific test class
pytest test_grpc_integration.py::TestTranscribeRPC -v

# Stop Docker Compose environment
cd test
docker-compose -f docker-compose.integration.yml down
```

---

## Dependencies

**Requires:**
- EPIC_01 (Go Orchestrator Core) - gRPC client implementation
- EPIC_02 (Python Worker Refactor) - gRPC server implementation

**Blocks:**
- STORY_02 (Webhook Integration Tests) - needs gRPC layer working
- STORY_03 (End-to-End Tests) - builds on gRPC tests
- STORY_04 (Memory Leak Validation) - needs stable gRPC communication

---

## Notes

### Test Data Considerations

- **Tiny Whisper Model**: Use tiny model for fast tests (~75MB, faster transcription)
- **Synthetic Audio**: Use ffmpeg-generated audio to avoid copyright issues
- **Deterministic Tests**: Whisper transcription on synthetic audio may produce empty/minimal output - this is expected

### Integration vs E2E Tests

- **Integration Tests**: Test gRPC layer only (direct client → server)
- **E2E Tests** (STORY_03): Test full pipeline (webhook → queue → gRPC → transcription)

### CI/CD Integration

These integration tests should run in CI with Docker Compose:
```yaml
# .github/workflows/integration-tests.yml
- name: Run integration tests
  run: |
    cd test
    docker-compose -f docker-compose.integration.yml up -d
    sleep 30  # Wait for services
    go test ./integration -v
    pytest test/integration -v
    docker-compose -f docker-compose.integration.yml down
```

---

## References

- [api/transcription.proto](/home/mikekao/personal/subgen/api/transcription.proto) - Protobuf schema (lines 1-181)
- [orchestrator/internal/grpc_client/client.go](/home/mikekao/personal/subgen/orchestrator/internal/grpc_client/client.go) - Go gRPC client (lines 52-147)
- [worker/src/grpc_server/service.py](/home/mikekao/personal/subgen/worker/src/grpc_server/service.py) - Python gRPC server (lines 52-174)
- [01_GRPC_PROTOCOL.md](../../DESIGN/01_GRPC_PROTOCOL.md) - gRPC protocol design
- gRPC Testing Guide: https://grpc.io/docs/guides/testing/

---

**Created**: 2026-02-15  
**Last Updated**: 2026-02-15
