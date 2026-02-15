"""
Shared pytest fixtures for Subgen tests.

This file is automatically discovered by pytest and makes fixtures
available to all test files without needing to import them.
"""

import pytest
from unittest.mock import Mock, MagicMock, patch
from fastapi.testclient import TestClient
from language_code import LanguageCode
import numpy as np
import io


@pytest.fixture
def mock_whisper_model():
    """
    Mock Whisper model that doesn't load real model weights.

    Returns a mock object with transcribe() method that returns
    fake transcription results without actually running inference.

    Usage in tests:
        def test_something(mock_whisper_model):
            result = mock_whisper_model.transcribe("audio_data")
            assert result.language == "English"
    """
    model = Mock()

    # Mock the transcribe method
    mock_result = Mock()
    mock_result.language = "English"
    mock_result.segments = [
        Mock(start=0.0, end=2.0, text="This is a test transcription", id=0),
        Mock(start=2.0, end=4.5, text="with multiple segments", id=1),
    ]
    mock_result.to_srt_vtt = Mock(return_value=None)

    model.transcribe = Mock(return_value=mock_result)

    return model


@pytest.fixture
def test_client():
    """
    FastAPI TestClient for testing HTTP endpoints.

    Returns a client that can make HTTP requests to the app
    without starting a real server.

    Usage in tests:
        def test_webhook(test_client):
            response = test_client.post("/plex", ...)
            assert response.status_code == 200
    """
    # Import app here to avoid circular imports
    from subgen import app

    return TestClient(app)


@pytest.fixture
def sample_audio_bytes():
    """
    Generate 1 second of silence as audio data.

    Returns 16kHz, 16-bit PCM audio (standard for Whisper).
    Useful for testing audio processing without real audio files.

    Usage in tests:
        def test_audio_processing(sample_audio_bytes):
            result = process_audio(sample_audio_bytes)
            assert len(result) > 0
    """
    # 1 second of silence at 16kHz, 16-bit PCM
    audio_array = np.zeros(16000, dtype=np.int16)
    return audio_array.tobytes()


@pytest.fixture
def plex_webhook_payload():
    """
    Sample Plex webhook JSON payload.

    Returns a dictionary matching Plex's webhook format for
    a "library.new" event (new episode added).

    Usage in tests:
        def test_plex_webhook(test_client, plex_webhook_payload):
            import json
            response = test_client.post(
                "/plex",
                data={"payload": json.dumps(plex_webhook_payload)},
                headers={"User-Agent": "PlexMediaServer/1.0"}
            )
            assert response.status_code == 200
    """
    return {
        "event": "library.new",
        "Metadata": {
            "ratingKey": "12345",
            "type": "episode",
            "title": "Test Episode",
            "grandparentTitle": "Test Show",
            "Media": [{"Part": [{"file": "/media/TV/Show/Season 01/S01E01.mkv"}]}],
        },
    }


@pytest.fixture
def jellyfin_webhook_payload():
    """
    Sample Jellyfin webhook payload.

    Returns form data matching Jellyfin's webhook format for
    an "ItemAdded" event.

    Usage in tests:
        def test_jellyfin_webhook(test_client, jellyfin_webhook_payload):
            response = test_client.post(
                "/jellyfin",
                data=jellyfin_webhook_payload,
                headers={"User-Agent": "Jellyfin-Server/1.0"}
            )
            assert response.status_code == 200
    """
    return {
        "NotificationType": "ItemAdded",
        "ItemId": "abc123def456",
        "file": "/media/TV/Show/Season 01/S01E01.mkv",
    }


@pytest.fixture
def temp_media_file(tmp_path):
    """
    Create a temporary media file for testing.

    Uses pytest's tmp_path fixture to create a temporary directory
    that's automatically cleaned up after the test.

    Args:
        tmp_path: pytest fixture providing temporary directory

    Returns:
        str: Absolute path to temporary file

    Usage in tests:
        def test_file_processing(temp_media_file):
            result = process_video(temp_media_file)
            assert result is not None
    """
    media_file = tmp_path / "test_video.mp4"
    media_file.write_bytes(b"fake video data")
    return str(media_file)


@pytest.fixture
def mock_language_code():
    """
    Returns a sample LanguageCode for testing.

    Usage in tests:
        def test_language_handling(mock_language_code):
            assert mock_language_code.to_iso_639_1() == "en"
    """
    return LanguageCode.ENGLISH


@pytest.fixture(autouse=True)
def reset_global_state():
    """
    Automatically runs before each test to reset global state.

    The autouse=True means this runs for EVERY test without
    explicitly requesting it.

    This prevents tests from affecting each other by:
    - Clearing the task queue
    - Clearing the task_results dictionary
    - Resetting any other global state

    Usage: Automatic, no need to request in test functions
    """
    # Setup: Clear state before test
    # (In future stories, add code here to clear task_queue, task_results, etc.)

    yield  # Test runs here

    # Teardown: Clean up after test
    # (In future stories, add cleanup code here)
