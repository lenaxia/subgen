"""
Configuration management using pydantic-settings.
Reads environment variables with type validation.
"""

import os
from functools import lru_cache
from typing import Literal

from pydantic import Field
from pydantic_settings import BaseSettings, SettingsConfigDict


class WorkerSettings(BaseSettings):
    """Configuration for Python transcription worker."""

    model_config = SettingsConfigDict(
        env_file=".env",
        env_file_encoding="utf-8",
        case_sensitive=False,
    )

    # gRPC Server Configuration
    grpc_host: str = Field(default="0.0.0.0", description="gRPC server bind address")
    grpc_port: int = Field(default=50051, description="gRPC server port")

    # Whisper Configuration
    whisper_model: str = Field(default="medium", description="Whisper model size")
    whisper_threads: int = Field(default=4, description="CPU threads for transcription")
    device: Literal["cpu", "cuda"] = Field(default="cpu", description="Device for inference")
    compute_type: str = Field(default="int8", description="Quantization type")
    model_path: str = Field(default="./models", description="Path to model storage")

    # Memory Management
    memory_threshold_mb: int = Field(
        default=3000, description="Memory threshold for immediate model unload (MB)"
    )
    model_cleanup_delay: int = Field(
        default=30, description="Seconds to wait before unloading model after last use"
    )
    clear_vram_on_complete: bool = Field(
        default=True, description="Clear CUDA VRAM after transcription"
    )

    # Transcription Options
    detect_language_length: int = Field(
        default=30, description="Seconds of audio for language detection"
    )
    detect_language_offset: int = Field(
        default=0, description="Offset in seconds before detecting language"
    )

    # Subtitle Configuration
    subtitle_language_name: str = Field(default="aa", description="Subtitle language code")
    show_model_in_filename: bool = Field(
        default=True, description="Include model name in subtitle filename"
    )
    show_subgen_in_filename: bool = Field(
        default=True, description="Include 'subgen' marker in subtitle filename"
    )
    append_footer: bool = Field(
        default=False, description="Append 'Transcribed by whisperAI' footer"
    )

    # Logging
    debug: bool = Field(default=True, description="Enable debug logging")
    log_level: str = Field(default="INFO", description="Log level")

    # Version
    version: str = Field(default="1.0.0", description="Worker version")


@lru_cache()
def get_settings() -> WorkerSettings:
    """Get cached settings instance."""
    return WorkerSettings()
