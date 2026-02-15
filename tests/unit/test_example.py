"""
Example test file to verify pytest infrastructure is working.

This file contains simple tests that should pass immediately
to prove that pytest, fixtures, and mocks are configured correctly.
"""

import pytest
from language_code import LanguageCode


def test_pytest_is_working():
    """
    Simplest possible test - just checks that pytest runs.

    If this fails, pytest itself is broken.
    """
    assert True


def test_language_code_import():
    """
    Test that we can import LanguageCode from language_code.py.

    This verifies that the module is importable and the Enum works.
    """
    assert LanguageCode.ENGLISH is not None
    assert LanguageCode.ENGLISH.to_iso_639_1() == "en"


def test_mock_whisper_model_fixture(mock_whisper_model):
    """
    Test that mock Whisper model fixture works.

    Args:
        mock_whisper_model: Fixture from conftest.py

    This verifies that:
    1. The fixture loads
    2. The mock has transcribe() method
    3. The mock returns expected structure
    """
    result = mock_whisper_model.transcribe("fake_audio_data")

    assert result is not None
    assert result.language == "English"
    assert len(result.segments) == 2
    assert result.segments[0].text == "This is a test transcription"


def test_sample_audio_bytes_fixture(sample_audio_bytes):
    """
    Test that sample audio fixture generates correct data.

    Args:
        sample_audio_bytes: Fixture from conftest.py

    Verifies that audio data is correct length (1 second at 16kHz)
    """
    # 1 second at 16kHz, 16-bit (2 bytes per sample) = 32000 bytes
    expected_length = 16000 * 2
    assert len(sample_audio_bytes) == expected_length


def test_plex_webhook_payload_fixture(plex_webhook_payload):
    """
    Test that Plex webhook payload fixture has correct structure.

    Args:
        plex_webhook_payload: Fixture from conftest.py

    Verifies the payload matches Plex's webhook format.
    """
    assert "event" in plex_webhook_payload
    assert plex_webhook_payload["event"] == "library.new"
    assert "Metadata" in plex_webhook_payload
    assert "ratingKey" in plex_webhook_payload["Metadata"]
