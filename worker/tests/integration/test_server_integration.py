"""
Integration tests for gRPC server.

Tests the full gRPC client -> server flow with real gRPC connections.
"""

import time
from concurrent import futures

import grpc
import pytest

from pb import transcription_pb2, transcription_pb2_grpc
from grpc_server.server import create_grpc_server
from config.settings import WorkerSettings


@pytest.fixture
def test_config():
    """Test configuration."""
    config = WorkerSettings(
        grpc_host="localhost",
        grpc_port=50052,  # Different port for testing
        whisper_threads=2,
        memory_threshold_mb=3000,
        version="1.0.0-test",
    )
    return config


@pytest.fixture
def grpc_server(test_config):
    """Start gRPC server for integration testing."""
    server, servicer = create_grpc_server(test_config)
    servicer.start_time = time.time()

    server.add_insecure_port(f"[::]:{test_config.grpc_port}")
    server.start()

    yield server, servicer

    server.stop(grace=1)


@pytest.fixture
def grpc_client(test_config, grpc_server):
    """Create gRPC client for testing."""
    channel = grpc.insecure_channel(f"localhost:{test_config.grpc_port}")

    # Wait for server to be ready
    grpc.channel_ready_future(channel).result(timeout=5)

    client = transcription_pb2_grpc.TranscriptionServiceStub(channel)
    yield client
    channel.close()


# ============================================================================
# Integration Tests
# ============================================================================


def test_health_check_integration(grpc_client):
    """Test HealthCheck RPC end-to-end."""
    request = transcription_pb2.HealthCheckRequest()

    response = grpc_client.HealthCheck(request)

    assert response.status == transcription_pb2.HealthCheckResponse.HEALTHY
    assert response.memory_mb > 0
    assert response.version == "1.0.0-test"
    assert response.uptime_seconds >= 0
    assert response.jobs_processed == 0
    assert response.jobs_active == 0


def test_health_check_tracks_uptime(grpc_client):
    """Test HealthCheck tracks uptime correctly."""
    # First call
    response1 = grpc_client.HealthCheck(transcription_pb2.HealthCheckRequest())
    uptime1 = response1.uptime_seconds

    # Wait a bit
    time.sleep(1.1)  # Sleep more than 1 second to ensure uptime increases

    # Second call
    response2 = grpc_client.HealthCheck(transcription_pb2.HealthCheckRequest())
    uptime2 = response2.uptime_seconds

    # Uptime should increase
    assert uptime2 > uptime1


def test_detect_language_integration(grpc_client):
    """Test DetectLanguage RPC (stub implementation returns error)."""
    request = transcription_pb2.DetectLanguageRequest(file_path="/test/audio.mp3", sample_length=30)

    response = grpc_client.DetectLanguage(request)

    # Should return error for now (not implemented)
    assert response.success is False
    assert "not yet implemented" in response.error_message.lower()


def test_detect_language_requires_audio_source_integration(grpc_client):
    """Test DetectLanguage requires audio source."""
    request = transcription_pb2.DetectLanguageRequest()
    # No file_path or audio_content

    with pytest.raises(grpc.RpcError) as exc_info:
        grpc_client.DetectLanguage(request)

    assert exc_info.value.code() == grpc.StatusCode.INVALID_ARGUMENT
    assert "required" in exc_info.value.details().lower()


def test_transcribe_integration(grpc_client):
    """Test Transcribe RPC (stub implementation returns error)."""
    request = transcription_pb2.TranscribeRequest(
        file_path="/test/video.mp4",
        task_type="transcribe",
        options=transcription_pb2.TranscribeOptions(whisper_model="tiny"),
    )

    response = grpc_client.Transcribe(request)

    # Should return error for now (not implemented)
    assert response.success is False
    assert "not yet implemented" in response.error_message.lower()


def test_transcribe_validates_file_path_integration(grpc_client):
    """Test Transcribe validates file_path."""
    request = transcription_pb2.TranscribeRequest(
        file_path="",  # Empty
        task_type="transcribe",
    )

    with pytest.raises(grpc.RpcError) as exc_info:
        grpc_client.Transcribe(request)

    assert exc_info.value.code() == grpc.StatusCode.INVALID_ARGUMENT
    assert "file_path" in exc_info.value.details().lower()


def test_multiple_concurrent_health_checks(grpc_client):
    """Test server can handle concurrent requests."""
    import concurrent.futures

    def call_health_check():
        request = transcription_pb2.HealthCheckRequest()
        return grpc_client.HealthCheck(request)

    # Make 10 concurrent requests
    with concurrent.futures.ThreadPoolExecutor(max_workers=10) as executor:
        futures_list = [executor.submit(call_health_check) for _ in range(10)]
        results = [f.result() for f in futures_list]

    # All should succeed
    assert len(results) == 10
    for response in results:
        assert response.status == transcription_pb2.HealthCheckResponse.HEALTHY


def test_server_handles_invalid_requests_gracefully(grpc_client):
    """Test server handles malformed requests without crashing."""
    # Empty transcribe request
    with pytest.raises(grpc.RpcError):
        grpc_client.Transcribe(transcription_pb2.TranscribeRequest())

    # Empty detect language request
    with pytest.raises(grpc.RpcError):
        grpc_client.DetectLanguage(transcription_pb2.DetectLanguageRequest())

    # Health check should still work after errors
    response = grpc_client.HealthCheck(transcription_pb2.HealthCheckRequest())
    assert response.status == transcription_pb2.HealthCheckResponse.HEALTHY
