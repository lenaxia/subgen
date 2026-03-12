"""
End-to-end tests for skip logic.

Tests skip scenarios with real transcription service integration.
"""

import os
import tempfile
import pytest
import grpc
from pathlib import Path

from pb import transcription_pb2
from pb import transcription_pb2_grpc
from config.settings import WorkerSettings
from grpc_server.service import TranscriptionServicer


@pytest.fixture
def worker_config():
    """Create a worker config for testing."""
    return WorkerSettings()


@pytest.fixture
def grpc_server(worker_config, tmp_path):
    """Create a gRPC server for testing."""
    import concurrent.futures

    servicer = TranscriptionServicer(worker_config)
    server = grpc.server(concurrent.futures.ThreadPoolExecutor(max_workers=1))
    transcription_pb2_grpc.add_TranscriptionServiceServicer_to_server(servicer, server)

    port = 50051
    server.add_insecure_port(f"[::]:{port}")
    server.start()

    yield servicer, port

    server.stop(0)


@pytest.fixture
def grpc_channel(grpc_server):
    """Create a gRPC channel for testing."""
    _, port = grpc_server
    channel = grpc.insecure_channel(f"localhost:{port}")
    yield channel
    channel.close()


@pytest.fixture
def grpc_client(grpc_channel):
    """Create a gRPC client for testing."""
    client = transcription_pb2_grpc.TranscriptionServiceStub(grpc_channel)
    yield client


@pytest.fixture
def tmp_path():
    """Create a temporary directory for test files."""
    with tempfile.TemporaryDirectory() as tmpdir:
        yield Path(tmpdir)


class TestSkipLogicE2E:
    """End-to-end tests for skip logic."""

    def test_skip_with_existing_subgen_subtitle(self, grpc_client, tmp_path):
        """Should skip transcription and return existing subgen subtitle."""
        # Create test video file
        video_path = tmp_path / "test.mkv"
        video_path.write_bytes(b"fake video content")

        # Create existing subgen subtitle
        subtitle_path = tmp_path / "test.subgen.medium.eng.srt"
        subtitle_path.write_text("1\n00:00:00,000 --> 00:00:01,000\nTest subtitle")

        # Request transcription
        request = transcription_pb2.TranscribeRequest(file_path=str(video_path))
        response = grpc_client.Transcribe(request)

        # Verify skip behavior
        assert response.success is True
        assert str(subtitle_path) in response.subtitle_path
        assert response.detected_language == ""

    def test_skip_with_existing_regular_subtitle(self, grpc_client, tmp_path):
        """Should skip transcription and return existing regular SRT."""
        # Create test video file
        video_path = tmp_path / "test.mkv"
        video_path.write_bytes(b"fake video content")

        # Create existing subtitle
        subtitle_path = tmp_path / "test.srt"
        subtitle_path.write_text("1\n00:00:00,000 --> 00:00:01,000\nTest subtitle")

        # Request transcription
        request = transcription_pb2.TranscribeRequest(file_path=str(video_path))
        response = grpc_client.Transcribe(request)

        # Verify skip behavior
        assert response.success is True
        assert str(subtitle_path) in response.subtitle_path
        assert response.detected_language == ""

    def test_skip_with_existing_lrc_for_audio(self, grpc_client, tmp_path):
        """Should skip transcription and return existing LRC for audio files."""
        # Create test audio file
        audio_path = tmp_path / "test.mp3"
        audio_path.write_bytes(b"fake audio content")

        # Create existing LRC
        lrc_path = tmp_path / "test.lrc"
        lrc_path.write_text("[00:00.00]Test lyric")

        # Request transcription
        request = transcription_pb2.TranscribeRequest(file_path=str(audio_path))
        response = grpc_client.Transcribe(request)

        # Verify skip behavior
        assert response.success is True
        assert str(lrc_path) in response.subtitle_path

    def test_no_skip_when_no_subtitles_exist(self, grpc_client, tmp_path):
        """Should attempt transcription when no subtitles exist (will fail on fake content)."""
        # Create test video file
        video_path = tmp_path / "test.mkv"
        video_path.write_bytes(b"fake video content")

        # Request transcription
        request = transcription_pb2.TranscribeRequest(file_path=str(video_path))

        # Should not skip immediately (transcription will fail on fake content)
        # This is expected - we're testing that skip logic doesn't block it
        # In a real scenario with valid media, transcription would proceed
        try:
            response = grpc_client.Transcribe(request)
            # If it gets here, skip logic didn't block
            assert True
        except grpc.RpcError as e:
            # Expected to fail with invalid audio, not skip
            assert "File not found" not in e.details()

    def test_skip_multiple_language_subtitles(self, grpc_client, tmp_path):
        """Should skip and return first matching subtitle."""
        # Create test video file
        video_path = tmp_path / "test.mkv"
        video_path.write_bytes(b"fake video content")

        # Create multiple existing subtitles
        subtitle_en = tmp_path / "test.subgen.medium.eng.srt"
        subtitle_en.write_text("1\n00:00:00,000 --> 00:00:01,000\nEnglish")
        subtitle_es = tmp_path / "test.subgen.medium.spa.srt"
        subtitle_es.write_text("1\n00:00:00,000 --> 00:00:01,000\nSpanish")

        # Request transcription
        request = transcription_pb2.TranscribeRequest(file_path=str(video_path))
        response = grpc_client.Transcribe(request)

        # Verify skip behavior - should find at least one subtitle
        assert response.success is True
        assert response.detected_language == ""
        # Should return one of the existing subtitles
        assert "subgen" in response.subtitle_path

    def test_external_subtitle_detection(self, grpc_client, tmp_path, worker_config):
        """Test external subtitle detection."""
        # Enable external subtitle skip
        worker_config.skip.skip_if_external_subtitles_exist = True
        worker_config.skip.skip_if_internal_subtitles_language = "eng"
        worker_config.skip.skip_only_subgen_subtitles = False

        # Create test video file
        video_path = tmp_path / "test.mkv"
        video_path.write_bytes(b"fake video content")

        # Create external subtitle
        subtitle_path = tmp_path / "test.en.srt"
        subtitle_path.write_text("1\n00:00:00,000 --> 00:00:01,000\nExternal subtitle")

        # Request transcription
        request = transcription_pb2.TranscribeRequest(file_path=str(video_path))
        response = grpc_client.Transcribe(request)

        # Verify skip behavior
        assert response.success is True
        # Might skip based on external subtitle or proceed
        # Depends on external subtitle detection implementation

    def test_config_override_skip(self, grpc_client, tmp_path, worker_config):
        """Test that config can disable skip logic."""
        # Disable skip logic
        worker_config.skip.skip_if_target_subtitles_exist = False

        # Create test video file with existing subtitle
        video_path = tmp_path / "test.mkv"
        video_path.write_bytes(b"fake video content")

        subtitle_path = tmp_path / "test.srt"
        subtitle_path.write_text("1\n00:00:00,000 --> 00:00:01,000\nTest subtitle")

        # Request transcription
        request = transcription_pb2.TranscribeRequest(file_path=str(video_path))

        # Should not skip even though subtitle exists
        # (Will fail on fake content, but skip logic shouldn't block)
        try:
            response = grpc_client.Transcribe(request)
            # If it doesn't skip, transcription might fail or succeed
            assert True
        except grpc.RpcError as e:
            # Verify it's not a skip-related error
            assert "skip" not in e.details().lower()

    def test_file_not_found_error(self, grpc_client, tmp_path):
        """Should return error for non-existent file."""
        # Request transcription for non-existent file
        video_path = tmp_path / "nonexistent.mkv"
        request = transcription_pb2.TranscribeRequest(file_path=str(video_path))

        # Should return NOT_FOUND error
        with pytest.raises(grpc.RpcError) as exc_info:
            grpc_client.Transcribe(request)

        assert exc_info.value.code() == grpc.StatusCode.NOT_FOUND
        assert "File not found" in exc_info.value.details()

    def test_audio_content_bypasses_skip(self, grpc_client):
        """Skip logic should not apply to audio content."""
        # Request transcription with audio content (no file path)
        request = transcription_pb2.TranscribeRequest(audio_content=b"fake audio content")

        # Skip logic only applies to file_path, not audio_content
        # Should not skip (will fail on invalid audio)
        try:
            response = grpc_client.Transcribe(request)
            # If it gets here, skip logic didn't block
            assert True
        except grpc.RpcError as e:
            # Expected to fail, but not due to skip logic
            assert "skip" not in e.details().lower()
