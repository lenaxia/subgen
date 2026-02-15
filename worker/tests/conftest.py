"""
pytest configuration and fixtures for worker tests.
"""

import pytest
from typing import Generator
from unittest.mock import MagicMock


@pytest.fixture
def mock_config() -> MagicMock:
    """Mock configuration object."""
    config = MagicMock()
    config.whisper_model = "tiny"
    config.whisper_threads = 4
    config.device = "cpu"
    config.compute_type = "int8"
    config.model_path = "/tmp/models"
    config.memory_threshold_mb = 3000
    config.model_cleanup_delay = 30
    config.clear_vram_on_complete = True
    return config


@pytest.fixture
def mock_model_manager() -> MagicMock:
    """Mock ModelManager."""
    manager = MagicMock()
    manager.get_model.return_value = MagicMock()
    manager.is_loaded.return_value = False
    manager.schedule_cleanup.return_value = None
    return manager


@pytest.fixture
def sample_audio_path() -> str:
    """Sample audio file path for testing."""
    return "/tmp/test_audio.mp3"


@pytest.fixture
def sample_transcribe_request() -> dict:
    """Sample TranscribeRequest data."""
    return {
        "file_path": "/media/TV/Show/S01E01.mkv",
        "task_type": "transcribe",
        "force_language": "",
        "options": {
            "whisper_model": "medium",
            "whisper_threads": 4,
            "word_level_highlight": False,
            "custom_regroup": "",
            "lrc_for_audio": True,
            "custom_prompt": "",
            "append_footer": False,
            "subtitle_language_name": "aa",
            "show_model_in_filename": True,
            "show_subgen_in_filename": True,
        },
        "metadata": {},
    }


@pytest.fixture
def sample_detect_language_request() -> dict:
    """Sample DetectLanguageRequest data."""
    return {
        "file_path": "/media/TV/Show/S01E01.mkv",
        "sample_length": 30,
        "sample_offset": 0,
    }
