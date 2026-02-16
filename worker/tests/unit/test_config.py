"""
Unit tests for configuration system.

Tests configuration loading, validation, backwards compatibility,
and error handling following TDD principles.
"""

import os
import pytest
from pathlib import Path
from pydantic import ValidationError

from config.settings import (
    ServerConfig,
    WhisperConfig,
    ProcessingConfig,
    SystemConfig,
    TranscriptionConfig,
    SubtitleConfig,
    SkipConfig,
    ModelLifecycleConfig,
    WorkerSettings,
    load_config,
)
from utils.errors import ConfigurationError


class TestServerConfig:
    """Test server integration configuration."""

    def test_default_server_config(self):
        """Test default server configuration loads."""
        config = ServerConfig()

        assert config.plex_server == "http://192.168.1.111:32400"
        assert config.plex_token == ""
        assert config.jellyfin_server == "http://192.168.1.111:8096"
        assert config.jellyfin_token == ""

    def test_server_url_validation_valid(self):
        """Test valid server URLs are accepted."""
        config = ServerConfig(
            plex_server="http://localhost:32400",
            jellyfin_server="https://jellyfin.example.com:8096",
        )

        assert config.plex_server == "http://localhost:32400"
        assert config.jellyfin_server == "https://jellyfin.example.com:8096"

    def test_server_url_validation_invalid(self):
        """Test invalid server URLs are rejected."""
        with pytest.raises(ValidationError) as exc_info:
            ServerConfig(plex_server="invalid-url")

        assert "http://" in str(exc_info.value) or "https://" in str(exc_info.value)

    def test_backwards_compatibility_plextoken(self, monkeypatch):
        """Test legacy PLEXTOKEN environment variable works."""
        monkeypatch.setenv("PLEXTOKEN", "legacy-token")

        config = ServerConfig()

        assert config.plex_token == "legacy-token"

    def test_backwards_compatibility_plexserver(self, monkeypatch):
        """Test legacy PLEXSERVER environment variable works."""
        monkeypatch.setenv("PLEXSERVER", "http://legacy:32400")

        config = ServerConfig()

        assert config.plex_server == "http://legacy:32400"

    def test_new_name_overrides_legacy(self, monkeypatch):
        """Test new environment variable names take precedence."""
        monkeypatch.setenv("PLEXTOKEN", "legacy-token")
        monkeypatch.setenv("PLEX_TOKEN", "new-token")

        config = ServerConfig()

        assert config.plex_token == "new-token"


class TestWhisperConfig:
    """Test Whisper model configuration."""

    def test_default_whisper_config(self):
        """Test default Whisper configuration."""
        config = WhisperConfig()

        assert config.model_name == "medium"
        assert config.device == "cpu"
        assert config.cpu_threads == 4
        assert config.concurrent_transcriptions == 2
        assert config.compute_type == "auto"

    def test_valid_model_names(self):
        """Test all valid model names are accepted."""
        valid_models = [
            "tiny",
            "base",
            "small",
            "medium",
            "large",
            "large-v2",
            "large-v3",
            "distil-small.en",
            "distil-medium.en",
            "distil-large-v2",
            "distil-large-v3",
        ]

        for model in valid_models:
            config = WhisperConfig(model_name=model)
            assert config.model_name == model

    def test_invalid_model_name(self):
        """Test invalid model names are rejected."""
        with pytest.raises(ValidationError) as exc_info:
            WhisperConfig(model_name="invalid-model")

        assert "model_name" in str(exc_info.value)

    def test_cpu_threads_validation_min(self):
        """Test CPU threads minimum validation."""
        with pytest.raises(ValidationError):
            WhisperConfig(cpu_threads=0)

    def test_cpu_threads_validation_max(self):
        """Test CPU threads maximum validation."""
        with pytest.raises(ValidationError):
            WhisperConfig(cpu_threads=100)

    def test_cpu_threads_validation_valid(self):
        """Test valid CPU thread counts."""
        for threads in [1, 4, 8, 16, 32]:
            config = WhisperConfig(cpu_threads=threads)
            assert config.cpu_threads == threads

    def test_concurrent_transcriptions_validation_min(self):
        """Test concurrent transcriptions minimum validation."""
        with pytest.raises(ValidationError):
            WhisperConfig(concurrent_transcriptions=0)

    def test_concurrent_transcriptions_validation_max(self):
        """Test concurrent transcriptions maximum validation."""
        with pytest.raises(ValidationError):
            WhisperConfig(concurrent_transcriptions=20)

    def test_concurrent_transcriptions_validation_valid(self):
        """Test valid concurrent transcription counts."""
        for count in [1, 2, 5, 10]:
            config = WhisperConfig(concurrent_transcriptions=count)
            assert config.concurrent_transcriptions == count

    def test_device_cpu(self):
        """Test CPU device selection."""
        config = WhisperConfig(device="cpu")
        assert config.device == "cpu"

    def test_device_cuda_available(self, monkeypatch):
        """Test CUDA device when available."""

        # Mock torch.cuda.is_available()
        class MockTorch:
            class cuda:
                @staticmethod
                def is_available():
                    return True

        import sys

        sys.modules["torch"] = MockTorch()

        config = WhisperConfig(device="cuda")
        assert config.device == "cuda"

    def test_device_cuda_not_available(self, monkeypatch):
        """Test CUDA device validation when not available."""

        # Mock torch.cuda.is_available() to return False
        class MockTorch:
            class cuda:
                @staticmethod
                def is_available():
                    return False

        import sys

        sys.modules["torch"] = MockTorch()

        with pytest.raises(ValidationError) as exc_info:
            WhisperConfig(device="cuda")

        assert "CUDA" in str(exc_info.value)

    def test_compute_type_validation(self):
        """Test valid compute types."""
        valid_types = ["auto", "int8", "int8_float16", "float16", "float32"]

        for compute_type in valid_types:
            config = WhisperConfig(compute_type=compute_type)
            assert config.compute_type == compute_type

    def test_compute_type_invalid(self):
        """Test invalid compute type is rejected."""
        with pytest.raises(ValidationError):
            WhisperConfig(compute_type="invalid")

    def test_model_path_created(self, tmp_path):
        """Test model path is created if missing."""
        model_path = tmp_path / "models" / "subdir"

        config = WhisperConfig(model_path=str(model_path))

        assert Path(config.model_path).exists()
        assert Path(config.model_path).is_dir()


class TestProcessingConfig:
    """Test media processing configuration."""

    def test_default_processing_config(self):
        """Test default processing configuration."""
        config = ProcessingConfig()

        assert config.process_added_media is True
        assert config.process_media_on_play is True
        assert config.monitor_folders == []

    def test_backwards_compatibility_procaddedmedia(self, monkeypatch):
        """Test legacy PROCADDEDMEDIA environment variable."""
        monkeypatch.setenv("PROCADDEDMEDIA", "false")

        config = ProcessingConfig()

        assert config.process_added_media is False

    def test_backwards_compatibility_procmediaonplay(self, monkeypatch):
        """Test legacy PROCMEDIAONPLAY environment variable."""
        monkeypatch.setenv("PROCMEDIAONPLAY", "false")

        config = ProcessingConfig()

        assert config.process_media_on_play is False

    def test_monitor_folders_comma_separated(self, monkeypatch):
        """Test parsing comma-separated folder list."""
        monkeypatch.setenv("TRANSCRIBE_FOLDERS", "/media/tv,/media/movies, /media/music")

        config = ProcessingConfig()

        assert len(config.monitor_folders) == 3
        assert "/media/tv" in config.monitor_folders
        assert "/media/movies" in config.monitor_folders
        assert "/media/music" in config.monitor_folders

    def test_monitor_folders_pipe_separated(self, monkeypatch):
        """Test parsing pipe-separated folder list (legacy)."""
        monkeypatch.setenv("TRANSCRIBE_FOLDERS", "/media/tv|/media/movies")

        config = ProcessingConfig()

        assert len(config.monitor_folders) == 2


class TestSystemConfig:
    """Test system-level configuration."""

    def test_default_system_config(self):
        """Test default system configuration."""
        config = SystemConfig()

        assert config.grpc_port == 50051
        assert config.webhook_port == 9000
        assert config.max_workers == 4
        assert config.memory_threshold_mb == 3000
        assert config.log_level == "INFO"
        assert config.debug is False

    def test_grpc_port_validation_min(self):
        """Test gRPC port minimum validation."""
        with pytest.raises(ValidationError):
            SystemConfig(grpc_port=100)

    def test_grpc_port_validation_max(self):
        """Test gRPC port maximum validation."""
        with pytest.raises(ValidationError):
            SystemConfig(grpc_port=99999)

    def test_grpc_port_validation_valid(self):
        """Test valid gRPC port numbers."""
        for port in [1024, 8080, 50051, 65535]:
            config = SystemConfig(grpc_port=port)
            assert config.grpc_port == port

    def test_webhook_port_backwards_compatibility(self, monkeypatch):
        """Test legacy WEBHOOKPORT environment variable."""
        monkeypatch.setenv("WEBHOOKPORT", "8080")

        config = SystemConfig()

        assert config.webhook_port == 8080

    def test_path_mapping_disabled(self):
        """Test path mapping disabled by default."""
        config = SystemConfig()

        assert config.use_path_mapping is False

    def test_path_mapping_enabled(self, monkeypatch):
        """Test path mapping can be enabled."""
        monkeypatch.setenv("USE_PATH_MAPPING", "true")
        monkeypatch.setenv("PATH_MAPPING_FROM", "/host/path")
        monkeypatch.setenv("PATH_MAPPING_TO", "/container/path")

        config = SystemConfig()

        assert config.use_path_mapping is True
        assert config.path_mapping_from == "/host/path"
        assert config.path_mapping_to == "/container/path"


class TestTranscriptionConfig:
    """Test transcription configuration."""

    def test_default_transcription_config(self):
        """Test default transcription configuration."""
        config = TranscriptionConfig()

        assert config.task == "transcribe"
        assert config.word_level_highlight is False
        assert config.lrc_for_audio_files is True
        assert config.detect_language_length == 30
        assert config.detect_language_offset == 0
        assert config.asr_timeout == 18000

    def test_task_validation_transcribe(self):
        """Test transcribe task is valid."""
        config = TranscriptionConfig(task="transcribe")
        assert config.task == "transcribe"

    def test_task_validation_translate(self):
        """Test translate task is valid."""
        config = TranscriptionConfig(task="translate")
        assert config.task == "translate"

    def test_task_validation_invalid(self):
        """Test invalid task is rejected."""
        with pytest.raises(ValidationError):
            TranscriptionConfig(task="invalid")

    def test_detect_language_length_validation_min(self):
        """Test language detection length minimum."""
        with pytest.raises(ValidationError):
            TranscriptionConfig(detect_language_length=0)

    def test_detect_language_length_validation_max(self):
        """Test language detection length maximum."""
        with pytest.raises(ValidationError):
            TranscriptionConfig(detect_language_length=500)

    def test_detect_language_offset_validation_min(self):
        """Test language detection offset minimum."""
        with pytest.raises(ValidationError):
            TranscriptionConfig(detect_language_offset=-1)

    def test_asr_timeout_validation_min(self):
        """Test ASR timeout minimum."""
        with pytest.raises(ValidationError):
            TranscriptionConfig(asr_timeout=30)


class TestSubtitleConfig:
    """Test subtitle generation configuration."""

    def test_default_subtitle_config(self):
        """Test default subtitle configuration."""
        config = SubtitleConfig()

        assert config.language_naming_type == "ISO_639_2_B"
        assert config.show_subgen_in_filename is True
        assert config.show_model_in_filename is True
        assert config.append_footer is False

    def test_language_naming_types(self):
        """Test all valid language naming types."""
        valid_types = ["ISO_639_1", "ISO_639_2_T", "ISO_639_2_B", "NAME", "NATIVE"]

        for naming_type in valid_types:
            config = SubtitleConfig(language_naming_type=naming_type)
            assert config.language_naming_type == naming_type

    def test_language_naming_type_invalid(self):
        """Test invalid language naming type is rejected."""
        with pytest.raises(ValidationError):
            SubtitleConfig(language_naming_type="INVALID")

    def test_backwards_compatibility_namesublang(self, monkeypatch):
        """Test legacy NAMESUBLANG environment variable."""
        monkeypatch.setenv("NAMESUBLANG", "custom-lang")

        config = SubtitleConfig()

        assert config.custom_language_name == "custom-lang"


class TestSkipConfig:
    """Test skip logic configuration."""

    def test_default_skip_config(self):
        """Test default skip configuration."""
        config = SkipConfig()

        assert config.skip_if_external_subtitles_exist is False
        assert config.skip_if_target_subtitles_exist is True
        assert config.skip_subtitle_languages == []
        assert config.skip_audio_languages == []
        assert config.skip_only_subgen_subtitles is False
        assert config.skip_unknown_language is False

    def test_backwards_compatibility_skipifexternalsub(self, monkeypatch):
        """Test legacy SKIPIFEXTERNALSUB environment variable."""
        monkeypatch.setenv("SKIPIFEXTERNALSUB", "true")

        config = SkipConfig()

        assert config.skip_if_external_subtitles_exist is True

    def test_skip_languages_pipe_separated(self, monkeypatch):
        """Test parsing pipe-separated skip language list."""
        monkeypatch.setenv("SKIP_SUBTITLE_LANGUAGES", "eng|spa|fra")

        config = SkipConfig()

        assert len(config.skip_subtitle_languages) == 3
        assert "eng" in config.skip_subtitle_languages
        assert "spa" in config.skip_subtitle_languages
        assert "fra" in config.skip_subtitle_languages

    def test_skip_audio_languages_pipe_separated(self, monkeypatch):
        """Test parsing pipe-separated skip audio language list."""
        monkeypatch.setenv("SKIP_IF_AUDIO_LANGUAGES", "eng|jpn")

        config = SkipConfig()

        assert len(config.skip_audio_languages) == 2

    def test_backwards_compatibility_skip_lang_codes(self, monkeypatch):
        """Test legacy SKIP_LANG_CODES environment variable."""
        monkeypatch.setenv("SKIP_LANG_CODES", "eng|spa")

        config = SkipConfig()

        assert len(config.skip_subtitle_languages) == 2


class TestModelLifecycleConfig:
    """Test model lifecycle configuration."""

    def test_default_model_lifecycle_config(self):
        """Test default model lifecycle configuration."""
        config = ModelLifecycleConfig()

        assert config.cleanup_delay == 30
        assert config.clear_vram_on_complete is True

    def test_cleanup_delay_validation_min(self):
        """Test cleanup delay minimum validation."""
        config = ModelLifecycleConfig(cleanup_delay=0)
        assert config.cleanup_delay == 0

    def test_cleanup_delay_validation_negative(self):
        """Test cleanup delay rejects negative values."""
        with pytest.raises(ValidationError):
            ModelLifecycleConfig(cleanup_delay=-1)


class TestWorkerSettings:
    """Test master configuration."""

    def test_default_worker_settings(self):
        """Test default worker settings load successfully."""
        config = WorkerSettings()

        assert config.server is not None
        assert config.whisper is not None
        assert config.processing is not None
        assert config.system is not None
        assert config.transcription is not None
        assert config.subtitle is not None
        assert config.skip is not None
        assert config.model_lifecycle is not None

    def test_nested_config_access(self):
        """Test accessing nested configuration values."""
        config = WorkerSettings()

        assert config.whisper.model_name == "medium"
        assert config.system.grpc_port == 50051
        assert config.transcription.task == "transcribe"

    def test_load_from_env_file(self, tmp_path):
        """Test loading configuration from .env file."""
        env_file = tmp_path / ".env"
        env_file.write_text(
            """
WHISPER_MODEL=small
TRANSCRIBE_DEVICE=cpu
GRPC_PORT=9090
PROCESS_ADDED_MEDIA=false
        """.strip()
        )

        config = load_config(env_file=env_file)

        assert config.whisper.model_name == "small"
        assert config.whisper.device == "cpu"
        assert config.system.grpc_port == 9090
        assert config.processing.process_added_media is False

    def test_load_from_yaml(self, tmp_path):
        """Test loading configuration from YAML file."""
        yaml_file = tmp_path / "config.yaml"
        yaml_file.write_text(
            """
whisper:
  model_name: tiny
  device: cpu
  cpu_threads: 8
system:
  grpc_port: 7070
  debug: true
        """.strip()
        )

        config = load_config(yaml_file=yaml_file)

        assert config.whisper.model_name == "tiny"
        assert config.whisper.cpu_threads == 8
        assert config.system.grpc_port == 7070
        assert config.system.debug is True

    def test_save_to_yaml(self, tmp_path):
        """Test saving configuration to YAML file."""
        config = WorkerSettings()
        yaml_file = tmp_path / "output.yaml"

        config.to_yaml(yaml_file)

        assert yaml_file.exists()

        # Load and verify
        loaded_config = load_config(yaml_file=yaml_file)
        assert loaded_config.whisper.model_name == config.whisper.model_name

    def test_invalid_config_clear_error(self, monkeypatch):
        """Test invalid configuration produces clear error message."""
        monkeypatch.setenv("WHISPER_MODEL", "invalid-model")

        with pytest.raises(ConfigurationError) as exc_info:
            load_config()

        error_msg = str(exc_info.value)
        assert "Configuration validation failed" in error_msg
        assert "whisper.model_name" in error_msg


class TestConfigurationErrorHandling:
    """Test configuration error handling."""

    def test_missing_required_no_error(self):
        """Test missing optional fields don't cause errors."""
        # All fields should have defaults
        config = WorkerSettings()
        assert config is not None

    def test_type_conversion_error(self, monkeypatch):
        """Test type conversion errors are caught."""
        monkeypatch.setenv("WHISPER_THREADS", "not-a-number")

        with pytest.raises(ConfigurationError):
            load_config()

    def test_validation_error_message_format(self, monkeypatch):
        """Test validation error messages are user-friendly."""
        monkeypatch.setenv("GRPC_PORT", "99999")

        with pytest.raises(ConfigurationError) as exc_info:
            load_config()

        error_msg = str(exc_info.value)
        assert "Configuration validation failed" in error_msg
        assert "system.grpc_port" in error_msg
