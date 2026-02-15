"""
Unit tests for gRPC server and TranscriptionServicer.

Tests written FIRST following TDD methodology.
These tests will fail initially and pass once implementation is complete.
"""

import time
from unittest.mock import Mock, patch

import grpc
import pytest

from pb import transcription_pb2
from grpc_server.service import TranscriptionServicer


@pytest.fixture
def mock_config():
    """Mock configuration for testing."""
    config = Mock()
    config.whisper_threads = 4
    config.memory_threshold_mb = 3000
    config.model_cleanup_delay = 30
    config.version = "1.0.0"
    return config


@pytest.fixture
def servicer(mock_config):
    """Create TranscriptionServicer instance for testing."""
    return TranscriptionServicer(mock_config)


@pytest.fixture
def mock_context():
    """Mock gRPC context."""
    context = Mock(spec=grpc.ServicerContext)
    context.abort = Mock(side_effect=grpc.RpcError("gRPC abort"))
    return context


# ============================================================================
# HealthCheck RPC Tests
# ============================================================================


def test_health_check_returns_healthy_status(servicer, mock_context):
    """Test HealthCheck returns HEALTHY status by default."""
    request = transcription_pb2.HealthCheckRequest()

    response = servicer.HealthCheck(request, mock_context)

    assert response.status == transcription_pb2.HealthCheckResponse.HEALTHY
    assert response.version == "1.0.0"


def test_health_check_returns_memory_usage(servicer, mock_context):
    """Test HealthCheck returns current memory usage."""
    request = transcription_pb2.HealthCheckRequest()

    response = servicer.HealthCheck(request, mock_context)

    assert response.memory_mb > 0
    assert isinstance(response.memory_mb, int)


def test_health_check_returns_job_statistics(servicer, mock_context):
    """Test HealthCheck returns job processing statistics."""
    request = transcription_pb2.HealthCheckRequest()

    # Initial stats should be zero
    response = servicer.HealthCheck(request, mock_context)

    assert response.jobs_processed >= 0
    assert response.jobs_active >= 0


def test_health_check_returns_uptime(servicer, mock_context):
    """Test HealthCheck returns worker uptime."""
    request = transcription_pb2.HealthCheckRequest()

    # Set start time
    servicer.start_time = time.time() - 10  # Started 10 seconds ago

    response = servicer.HealthCheck(request, mock_context)

    assert response.uptime_seconds >= 10
    assert response.uptime_seconds < 15  # Should be close to 10


def test_health_check_returns_model_loaded_status(servicer, mock_context):
    """Test HealthCheck returns whether model is loaded."""
    request = transcription_pb2.HealthCheckRequest()

    response = servicer.HealthCheck(request, mock_context)

    # Model should not be loaded initially
    assert isinstance(response.model_loaded, bool)


# ============================================================================
# DetectLanguage RPC Tests
# ============================================================================


def test_detect_language_requires_audio_source(servicer, mock_context):
    """Test DetectLanguage requires either file_path or audio_content."""
    request = transcription_pb2.DetectLanguageRequest()
    # No file_path or audio_content set

    with pytest.raises(grpc.RpcError):
        servicer.DetectLanguage(request, mock_context)

    mock_context.abort.assert_called_once()


def test_detect_language_accepts_file_path(servicer, mock_context):
    """Test DetectLanguage accepts file_path as audio source."""
    request = transcription_pb2.DetectLanguageRequest(file_path="/test/audio.mp3", sample_length=30)

    # Should return error response (not implemented yet) but not crash
    response = servicer.DetectLanguage(request, mock_context)

    assert response is not None
    assert isinstance(response, transcription_pb2.DetectLanguageResponse)


def test_detect_language_accepts_audio_content(servicer, mock_context):
    """Test DetectLanguage accepts audio_content as bytes."""
    request = transcription_pb2.DetectLanguageRequest(
        audio_content=b"fake audio data", sample_length=30
    )

    response = servicer.DetectLanguage(request, mock_context)

    assert response is not None
    assert isinstance(response, transcription_pb2.DetectLanguageResponse)


def test_detect_language_uses_default_sample_length(servicer, mock_context):
    """Test DetectLanguage uses default sample length if not specified."""
    request = transcription_pb2.DetectLanguageRequest(file_path="/test/audio.mp3")

    response = servicer.DetectLanguage(request, mock_context)

    # Should not crash with missing sample_length
    assert response is not None


def test_detect_language_returns_not_implemented_for_now(servicer, mock_context):
    """Test DetectLanguage returns error (not yet implemented)."""
    request = transcription_pb2.DetectLanguageRequest(file_path="/test/audio.mp3", sample_length=30)

    response = servicer.DetectLanguage(request, mock_context)

    # Should return error response
    assert response.success is False
    assert "not yet implemented" in response.error_message.lower()


# ============================================================================
# Transcribe RPC Tests
# ============================================================================


def test_transcribe_validates_file_path(servicer, mock_context):
    """Test Transcribe validates file_path is not empty."""
    request = transcription_pb2.TranscribeRequest(
        file_path="",  # Empty path
        task_type="transcribe",
    )

    with pytest.raises(grpc.RpcError):
        servicer.Transcribe(request, mock_context)

    mock_context.abort.assert_called_once()
    args = mock_context.abort.call_args[0]
    assert args[0] == grpc.StatusCode.INVALID_ARGUMENT


def test_transcribe_accepts_valid_request(servicer, mock_context):
    """Test Transcribe accepts valid request structure."""
    request = transcription_pb2.TranscribeRequest(
        file_path="/test/video.mp4",
        task_type="transcribe",
        options=transcription_pb2.TranscribeOptions(whisper_model="tiny"),
    )

    # Should return error response (not implemented yet) but not crash
    response = servicer.Transcribe(request, mock_context)

    assert response is not None
    assert isinstance(response, transcription_pb2.TranscribeResponse)


def test_transcribe_increments_job_statistics(servicer, mock_context):
    """Test Transcribe updates job statistics on completion."""
    initial_processed = servicer.stats["jobs_processed"]
    initial_active = servicer.stats["jobs_active"]

    request = transcription_pb2.TranscribeRequest(
        file_path="/test/video.mp4", task_type="transcribe"
    )

    # Mock successful transcription (will be implemented in STORY_02)
    # For now, this should return an error but still update stats
    try:
        servicer.Transcribe(request, mock_context)
    except Exception:
        pass

    # Active jobs should return to initial state after completion
    assert servicer.stats["jobs_active"] == initial_active


def test_transcribe_returns_not_implemented_for_now(servicer, mock_context):
    """Test Transcribe returns error (not yet implemented)."""
    request = transcription_pb2.TranscribeRequest(
        file_path="/test/video.mp4", task_type="transcribe"
    )

    response = servicer.Transcribe(request, mock_context)

    # Should return error response
    assert response.success is False
    assert "not yet implemented" in response.error_message.lower()


def test_transcribe_handles_optional_force_language(servicer, mock_context):
    """Test Transcribe handles optional force_language parameter."""
    request = transcription_pb2.TranscribeRequest(
        file_path="/test/video.mp4", task_type="transcribe", force_language="en"
    )

    response = servicer.Transcribe(request, mock_context)

    # Should not crash with force_language set
    assert response is not None


def test_transcribe_handles_metadata(servicer, mock_context):
    """Test Transcribe accepts metadata dictionary."""
    request = transcription_pb2.TranscribeRequest(
        file_path="/test/video.mp4", task_type="transcribe", metadata={"plex_item_id": "12345"}
    )

    response = servicer.Transcribe(request, mock_context)

    # Should not crash with metadata
    assert response is not None


# ============================================================================
# Server Creation Tests
# ============================================================================


def test_servicer_initialization(mock_config):
    """Test TranscriptionServicer initializes correctly."""
    servicer = TranscriptionServicer(mock_config)

    assert servicer.config == mock_config
    assert servicer.stats["jobs_processed"] == 0
    assert servicer.stats["jobs_active"] == 0
    assert servicer.start_time is None  # Set externally by server


def test_servicer_has_all_required_methods(mock_config):
    """Test TranscriptionServicer implements all required RPC methods."""
    servicer = TranscriptionServicer(mock_config)

    assert hasattr(servicer, "HealthCheck")
    assert hasattr(servicer, "DetectLanguage")
    assert hasattr(servicer, "Transcribe")
    assert callable(servicer.HealthCheck)
    assert callable(servicer.DetectLanguage)
    assert callable(servicer.Transcribe)
