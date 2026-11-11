"""
pytest configuration and fixtures for worker tests.
"""

import os
import sys
from pathlib import Path
import pytest
from typing import Generator
from unittest.mock import MagicMock

# Add parent directories to Python path for imports
worker_dir = Path(__file__).parent.parent
sys.path.insert(0, str(worker_dir / "src"))
sys.path.insert(0, str(worker_dir))
# pb/transcription_pb2_grpc.py uses a flat `import transcription_pb2`
# (matches the container's PYTHONPATH=/app/src:/app:/app/pb)
sys.path.insert(0, str(worker_dir / "pb"))


@pytest.fixture(autouse=True)
def restore_environment():
    """Snapshot and restore os.environ around every test.

    load_config(env_file=...) uses python-dotenv's override=True, which
    permanently injects the .env values into os.environ. Without this
    fixture one config test's .env (WHISPER_MODEL=small,
    PROCESS_ADDED_MEDIA=false, ...) leaks into every later test in the
    process — same failure class as the sys.modules pollution fixed
    earlier.
    """
    snapshot = dict(os.environ)
    yield
    os.environ.clear()
    os.environ.update(snapshot)


@pytest.fixture
def mock_config() -> MagicMock:
    """Mock configuration object mirroring the real nested settings shape."""
    config = MagicMock()
    config.whisper.model_name = "tiny"
    config.whisper.threads = 4
    config.whisper.device = "cpu"
    config.whisper.compute_type = "int8"
    config.whisper.model_path = "/tmp/models"
    config.memory_threshold_mb = 3000
    config.model_lifecycle.cleanup_delay = 30
    config.model_lifecycle.clear_vram_on_complete = True
    config.model_lifecycle.unload_on_idle = None  # auto
    config.system.max_workers = 10
    config.version = "1.0.0"
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
