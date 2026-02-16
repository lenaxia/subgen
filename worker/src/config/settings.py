"""
Configuration management using pydantic-settings.
Reads environment variables with type validation and backwards compatibility.
"""

import os
from pathlib import Path
from typing import List, Literal, Optional, Any
from functools import lru_cache

import yaml
from pydantic import (
    Field,
    field_validator,
    model_validator,
    HttpUrl,
    ValidationInfo,
    AliasChoices,
)
from pydantic_settings import BaseSettings, SettingsConfigDict

from utils.errors import ConfigurationError


class ServerConfig(BaseSettings):
    """Server integration configuration (Plex/Jellyfin)."""

    model_config = SettingsConfigDict(
        env_prefix="",
        env_file=".env",
        env_file_encoding="utf-8",
        case_sensitive=False,
        extra="allow",
        populate_by_name=True,
    )

    plex_server: str = Field(
        default="http://192.168.1.111:32400",
        validation_alias=AliasChoices("PLEX_SERVER", "PLEXSERVER"),
        description="Plex server URL",
    )
    plex_token: str = Field(
        default="",
        validation_alias=AliasChoices("PLEX_TOKEN", "PLEXTOKEN"),
        description="Plex authentication token",
    )
    jellyfin_server: str = Field(
        default="http://192.168.1.111:8096",
        description="Jellyfin server URL",
    )
    jellyfin_token: str = Field(
        default="",
        description="Jellyfin authentication token",
    )

    @field_validator("plex_server", "jellyfin_server")
    @classmethod
    def validate_server_url(cls, v: str) -> str:
        """Validate server URLs start with http:// or https://."""
        if not v.startswith(("http://", "https://")):
            raise ValueError("Server URL must start with http:// or https://")
        return v


class WhisperConfig(BaseSettings):
    """Whisper model configuration."""

    model_config = SettingsConfigDict(
        env_prefix="",
        env_file=".env",
        env_file_encoding="utf-8",
        case_sensitive=False,
        extra="allow",
        populate_by_name=True,
    )

    model_name: Literal[
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
    ] = Field(
        default="medium",
        validation_alias="WHISPER_MODEL",
        description="Whisper model size",
    )
    device: Literal["cpu", "cuda"] = Field(
        default="cpu",
        validation_alias="TRANSCRIBE_DEVICE",
        description="Device for inference",
    )
    cpu_threads: int = Field(
        default=4,
        validation_alias="WHISPER_THREADS",
        ge=1,
        le=32,
        description="CPU threads for transcription",
    )
    concurrent_transcriptions: int = Field(
        default=2,
        ge=1,
        le=10,
        description="Max concurrent transcriptions",
    )
    compute_type: Literal["auto", "int8", "int8_float16", "float16", "float32"] = Field(
        default="auto",
        description="Quantization type",
    )
    model_path: Path = Field(
        default=Path("./models"),
        description="Path to model storage",
    )

    @field_validator("device")
    @classmethod
    def validate_cuda_device(cls, v: str) -> str:
        """Validate CUDA device is available if selected."""
        if v == "cuda":
            try:
                # Lazy import to avoid dependency in tests
                import torch  # type: ignore[import-untyped]

                if not torch.cuda.is_available():
                    raise ValueError("CUDA device selected but CUDA is not available")
            except ImportError:
                raise ValueError("CUDA device selected but PyTorch is not installed")
        return v

    @field_validator("model_path")
    @classmethod
    def create_model_path(cls, v: Path) -> Path:
        """Create model path directory if it doesn't exist."""
        v = Path(v)
        v.mkdir(parents=True, exist_ok=True)
        return v


class ProcessingConfig(BaseSettings):
    """Media processing configuration."""

    model_config = SettingsConfigDict(
        env_prefix="",
        env_file=".env",
        env_file_encoding="utf-8",
        case_sensitive=False,
        extra="allow",
        populate_by_name=True,
        # Allow custom parsing for list fields from env vars
        str_strip_whitespace=True,
    )

    process_added_media: bool = Field(
        default=True,
        validation_alias=AliasChoices("PROCESS_ADDED_MEDIA", "PROCADDEDMEDIA"),
        description="Process media when added to library",
    )
    process_media_on_play: bool = Field(
        default=True,
        validation_alias=AliasChoices("PROCESS_MEDIA_ON_PLAY", "PROCMEDIAONPLAY"),
        description="Process media when played",
    )
    monitor_folders: str = Field(
        default="",
        validation_alias="TRANSCRIBE_FOLDERS",
        description="Folders to monitor for transcription",
    )

    @field_validator("monitor_folders", mode="after")
    @classmethod
    def parse_folder_list(cls, v: str) -> List[str]:
        """Parse comma or pipe-separated folder list."""
        if not v or not v.strip():
            return []
        # Try pipe first, then comma
        if "|" in v:
            return [f.strip() for f in v.split("|") if f.strip()]
        else:
            return [f.strip() for f in v.split(",") if f.strip()]


class SystemConfig(BaseSettings):
    """System-level configuration."""

    model_config = SettingsConfigDict(
        env_prefix="",
        env_file=".env",
        env_file_encoding="utf-8",
        case_sensitive=False,
        extra="allow",
        populate_by_name=True,
    )

    grpc_host: str = Field(
        default="0.0.0.0",
        description="gRPC server host",
    )
    grpc_port: int = Field(
        default=50051,
        ge=1024,
        le=65535,
        description="gRPC server port",
    )
    webhook_port: int = Field(
        default=9000,
        validation_alias="WEBHOOKPORT",
        ge=1024,
        le=65535,
        description="Webhook server port",
    )
    max_workers: int = Field(
        default=4,
        ge=1,
        description="Maximum concurrent workers",
    )
    memory_threshold_mb: int = Field(
        default=3000,
        ge=100,
        description="Memory threshold for model unload (MB)",
    )
    log_level: Literal["DEBUG", "INFO", "WARNING", "ERROR", "CRITICAL"] = Field(
        default="INFO",
        description="Logging level",
    )
    debug: bool = Field(
        default=False,
        description="Enable debug mode",
    )
    use_path_mapping: bool = Field(
        default=False,
        description="Enable path mapping for Docker/containers",
    )
    path_mapping_from: str = Field(
        default="",
        description="Source path for path mapping",
    )
    path_mapping_to: str = Field(
        default="",
        description="Destination path for path mapping",
    )


class TranscriptionConfig(BaseSettings):
    """Transcription configuration."""

    model_config = SettingsConfigDict(
        env_prefix="",
        env_file=".env",
        env_file_encoding="utf-8",
        case_sensitive=False,
        extra="allow",
        populate_by_name=True,
    )

    task: Literal["transcribe", "translate"] = Field(
        default="transcribe",
        description="Transcription task type",
    )
    word_level_highlight: bool = Field(
        default=False,
        description="Enable word-level highlighting in subtitles",
    )
    lrc_for_audio_files: bool = Field(
        default=True,
        description="Generate LRC format for audio files",
    )
    detect_language_length: int = Field(
        default=30,
        ge=10,
        le=300,
        description="Seconds of audio for language detection",
    )
    detect_language_offset: int = Field(
        default=0,
        ge=0,
        description="Offset in seconds before detecting language",
    )
    asr_timeout: int = Field(
        default=18000,
        ge=60,
        description="ASR timeout in seconds",
    )


class SubtitleConfig(BaseSettings):
    """Subtitle generation configuration."""

    model_config = SettingsConfigDict(
        env_prefix="",
        env_file=".env",
        env_file_encoding="utf-8",
        case_sensitive=False,
        extra="allow",
        populate_by_name=True,
    )

    language_naming_type: Literal["ISO_639_1", "ISO_639_2_T", "ISO_639_2_B", "NAME", "NATIVE"] = (
        Field(
            default="ISO_639_2_B",
            description="Language code format for subtitle filenames",
        )
    )
    custom_language_name: str = Field(
        default="",
        validation_alias="NAMESUBLANG",
        description="Custom language name for subtitles",
    )
    show_subgen_in_filename: bool = Field(
        default=True,
        description="Include 'subgen' marker in subtitle filename",
    )
    show_model_in_filename: bool = Field(
        default=True,
        description="Include model name in subtitle filename",
    )
    append_footer: bool = Field(
        default=False,
        description="Append 'Transcribed by whisperAI' footer",
    )


class SkipConfig(BaseSettings):
    """Skip logic configuration."""

    model_config = SettingsConfigDict(
        env_prefix="",
        env_file=".env",
        env_file_encoding="utf-8",
        case_sensitive=False,
        extra="allow",
        populate_by_name=True,
        str_strip_whitespace=True,
    )

    skip_if_external_subtitles_exist: bool = Field(
        default=False,
        validation_alias="SKIPIFEXTERNALSUB",
        description="Skip if external subtitles exist",
    )
    skip_if_target_subtitles_exist: bool = Field(
        default=True,
        description="Skip if target language subtitles exist",
    )
    skip_subtitle_languages: str = Field(
        default="",
        validation_alias=AliasChoices("SKIP_SUBTITLE_LANGUAGES", "SKIP_LANG_CODES"),
        description="Subtitle languages to skip (pipe or comma-separated)",
    )
    skip_audio_languages: str = Field(
        default="",
        validation_alias=AliasChoices("SKIP_AUDIO_LANGUAGES", "SKIP_IF_AUDIO_LANGUAGES"),
        description="Audio languages to skip (pipe or comma-separated)",
    )
    skip_only_subgen_subtitles: bool = Field(
        default=False,
        description="Only skip subgen-generated subtitles",
    )
    skip_unknown_language: bool = Field(
        default=False,
        description="Skip files with unknown language",
    )

    def get_skip_subtitle_languages(self) -> List[str]:
        """Get skip_subtitle_languages as a list."""
        return self._parse_language_string(self.skip_subtitle_languages)

    def get_skip_audio_languages(self) -> List[str]:
        """Get skip_audio_languages as a list."""
        return self._parse_language_string(self.skip_audio_languages)

    @staticmethod
    def _parse_language_string(v: str) -> List[str]:
        """Parse pipe or comma-separated language list."""
        if not v or not v.strip():
            return []
        if "|" in v:
            return [lang.strip() for lang in v.split("|") if lang.strip()]
        else:
            return [lang.strip() for lang in v.split(",") if lang.strip()]


class ModelLifecycleConfig(BaseSettings):
    """Model lifecycle management configuration."""

    model_config = SettingsConfigDict(
        env_prefix="",
        env_file=".env",
        env_file_encoding="utf-8",
        case_sensitive=False,
        extra="allow",
        populate_by_name=True,
    )

    cleanup_delay: int = Field(
        default=30,
        ge=0,
        description="Seconds to wait before unloading model after last use",
    )
    clear_vram_on_complete: bool = Field(
        default=True,
        description="Clear CUDA VRAM after transcription",
    )


class WorkerSettings(BaseSettings):
    """Master configuration for Python transcription worker."""

    model_config = SettingsConfigDict(
        env_prefix="",
        env_file=".env",
        env_file_encoding="utf-8",
        case_sensitive=False,
        extra="allow",
        populate_by_name=True,
        # Make nested config fields read from environment with section prefixes
        env_nested_delimiter="__",
    )

    version: str = Field(
        default="2026.02.9",
        description="Worker version",
    )
    server: ServerConfig = Field(default_factory=ServerConfig)
    whisper: WhisperConfig = Field(default_factory=WhisperConfig)
    processing: ProcessingConfig = Field(default_factory=ProcessingConfig)
    system: SystemConfig = Field(default_factory=SystemConfig)
    transcription: TranscriptionConfig = Field(default_factory=TranscriptionConfig)
    subtitle: SubtitleConfig = Field(default_factory=SubtitleConfig)
    skip: SkipConfig = Field(default_factory=SkipConfig)
    model_lifecycle: ModelLifecycleConfig = Field(default_factory=ModelLifecycleConfig)

    def to_yaml(self, file_path: Path) -> None:
        """Save configuration to YAML file."""
        data = self.model_dump(mode="python")

        # Convert list fields back to pipe-separated strings for consistency
        if "processing" in data and "monitor_folders" in data["processing"]:
            folders = data["processing"]["monitor_folders"]
            if isinstance(folders, list):
                data["processing"]["monitor_folders"] = "|".join(folders) if folders else ""

        if "skip" in data:
            if "skip_subtitle_languages" in data["skip"]:
                langs = data["skip"]["skip_subtitle_languages"]
                if isinstance(langs, list):
                    data["skip"]["skip_subtitle_languages"] = "|".join(langs) if langs else ""
            if "skip_audio_languages" in data["skip"]:
                langs = data["skip"]["skip_audio_languages"]
                if isinstance(langs, list):
                    data["skip"]["skip_audio_languages"] = "|".join(langs) if langs else ""

        # Convert Path objects to strings
        if "whisper" in data and "model_path" in data["whisper"]:
            model_path = data["whisper"]["model_path"]
            if isinstance(model_path, Path):
                data["whisper"]["model_path"] = str(model_path)

        with open(file_path, "w") as f:
            yaml.dump(data, f, default_flow_style=False, sort_keys=False)


def load_config(
    env_file: Optional[Path] = None, yaml_file: Optional[Path] = None
) -> WorkerSettings:
    """
    Load configuration from environment, .env file, or YAML file.

    Priority (highest to lowest):
    1. Environment variables
    2. YAML file (if provided)
    3. .env file (if provided or default)
    4. Default values

    Args:
        env_file: Path to .env file (optional)
        yaml_file: Path to YAML config file (optional)

    Returns:
        WorkerSettings instance

    Raises:
        ConfigurationError: If configuration is invalid
    """
    try:
        if yaml_file and yaml_file.exists():
            # Load from YAML file
            with open(yaml_file) as f:
                yaml_data = yaml.safe_load(f)

            # Create nested config objects from YAML data
            config_dict = {}
            for section in [
                "server",
                "whisper",
                "processing",
                "system",
                "transcription",
                "subtitle",
                "skip",
                "model_lifecycle",
            ]:
                if section in yaml_data:
                    config_dict[section] = yaml_data[section]

            # Temporarily clear environment variables to prevent them from overriding YAML
            import os

            saved_env = {}
            env_vars_to_clear = [
                "WHISPER_MODEL",
                "TRANSCRIBE_DEVICE",
                "WHISPER_THREADS",
                "GRPC_PORT",
                "WEBHOOKPORT",
                "PLEXTOKEN",
                "PLEXSERVER",
                "PROCADDEDMEDIA",
                "PROCMEDIAONPLAY",
                "TRANSCRIBE_FOLDERS",
                "SKIPIFEXTERNALSUB",
                "SKIP_LANG_CODES",
                "SKIP_IF_AUDIO_LANGUAGES",
                "NAMESUBLANG",
                "PROCESS_ADDED_MEDIA",
                "DEBUG",
            ]

            for var in env_vars_to_clear:
                if var in os.environ:
                    saved_env[var] = os.environ[var]
                    del os.environ[var]

            try:
                result = WorkerSettings(**config_dict)
            finally:
                # Restore environment variables
                for var, value in saved_env.items():
                    os.environ[var] = value

            return result

        elif env_file and env_file.exists():
            # Load from .env file using python-dotenv to set env vars
            from dotenv import load_dotenv

            load_dotenv(env_file, override=True)
            return WorkerSettings()

        else:
            # Load from environment variables
            return WorkerSettings()

    except Exception as e:
        # Convert pydantic validation errors to ConfigurationError
        from pydantic import ValidationError

        if isinstance(e, ValidationError):
            errors = []
            # Map of env var names to config paths for better error messages
            env_to_path_map = {
                "WHISPER_MODEL": "whisper.model_name",
                "TRANSCRIBE_DEVICE": "whisper.device",
                "WHISPER_THREADS": "whisper.cpu_threads",
                "GRPC_PORT": "system.grpc_port",
                "WEBHOOKPORT": "system.webhook_port",
                "PLEXTOKEN": "server.plex_token",
                "PLEXSERVER": "server.plex_server",
                "PROCADDEDMEDIA": "processing.process_added_media",
                "PROCMEDIAONPLAY": "processing.process_media_on_play",
                "TRANSCRIBE_FOLDERS": "processing.monitor_folders",
                "SKIPIFEXTERNALSUB": "skip.skip_if_external_subtitles_exist",
                "SKIP_LANG_CODES": "skip.skip_subtitle_languages",
                "SKIP_IF_AUDIO_LANGUAGES": "skip.skip_audio_languages",
                "NAMESUBLANG": "subtitle.custom_language_name",
            }

            for error in e.errors():
                field_loc = ".".join(str(loc) for loc in error["loc"])
                msg = error["msg"]

                # Try to map env var name to config path
                for env_name, config_path in env_to_path_map.items():
                    if env_name in field_loc or env_name.lower() in field_loc.lower():
                        field_loc = config_path
                        break

                errors.append(f"  - {field_loc}: {msg}")

            error_msg = "Configuration validation failed:\n" + "\n".join(errors)
            raise ConfigurationError(error_msg) from e

        raise ConfigurationError(f"Failed to load configuration: {e}") from e


@lru_cache()
def get_settings() -> WorkerSettings:
    """Get cached settings instance."""
    return load_config()
