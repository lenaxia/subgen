"""
Custom exception classes for worker.

Provides a hierarchy of exceptions with clear error messages
and gRPC status code mappings for error handling.
"""

from enum import Enum
from typing import Optional


class GrpcStatusCode(Enum):
    """gRPC status codes for error mapping."""

    OK = 0
    CANCELLED = 1
    UNKNOWN = 2
    INVALID_ARGUMENT = 3
    DEADLINE_EXCEEDED = 4
    NOT_FOUND = 5
    ALREADY_EXISTS = 6
    PERMISSION_DENIED = 7
    RESOURCE_EXHAUSTED = 8
    FAILED_PRECONDITION = 9
    ABORTED = 10
    OUT_OF_RANGE = 11
    UNIMPLEMENTED = 12
    INTERNAL = 13
    UNAVAILABLE = 14
    DATA_LOSS = 15
    UNAUTHENTICATED = 16


class WorkerError(Exception):
    """
    Base exception for all worker errors.

    Attributes:
        message: Human-readable error message
        grpc_code: Associated gRPC status code
        details: Optional additional error details
    """

    def __init__(
        self,
        message: str,
        grpc_code: GrpcStatusCode = GrpcStatusCode.INTERNAL,
        details: Optional[str] = None,
    ):
        self.message = message
        self.grpc_code = grpc_code
        self.details = details
        super().__init__(self.message)

    def __str__(self) -> str:
        """Return formatted error message."""
        if self.details:
            return f"{self.message}\nDetails: {self.details}"
        return self.message

    def to_grpc_status(self) -> tuple[int, str]:
        """
        Convert to gRPC status tuple.

        Returns:
            Tuple of (status_code, error_message)
        """
        return (self.grpc_code.value, str(self))


class ConfigurationError(WorkerError):
    """
    Raised when configuration is invalid or incomplete.

    Examples:
        - Invalid environment variable value
        - Missing required configuration
        - Invalid configuration file format
    """

    def __init__(self, message: str, details: Optional[str] = None):
        super().__init__(
            message=message, grpc_code=GrpcStatusCode.FAILED_PRECONDITION, details=details
        )


class ModelLoadError(WorkerError):
    """
    Raised when Whisper model fails to load.

    Examples:
        - Model file not found
        - Insufficient memory
        - Invalid model format
        - CUDA initialization failure
    """

    def __init__(
        self, message: str, model_name: Optional[str] = None, details: Optional[str] = None
    ):
        if model_name:
            message = f"Failed to load model '{model_name}': {message}"

        super().__init__(message=message, grpc_code=GrpcStatusCode.UNAVAILABLE, details=details)


class TranscriptionError(WorkerError):
    """
    Raised when transcription process fails.

    Examples:
        - Transcription timeout
        - Audio processing error
        - Model inference error
    """

    def __init__(
        self, message: str, file_path: Optional[str] = None, details: Optional[str] = None
    ):
        if file_path:
            message = f"Transcription failed for '{file_path}': {message}"

        super().__init__(message=message, grpc_code=GrpcStatusCode.INTERNAL, details=details)


class AudioExtractionError(WorkerError):
    """
    Raised when audio extraction from media file fails.

    Examples:
        - File not found
        - No audio track present
        - Unsupported codec
        - FFmpeg error
    """

    def __init__(
        self, message: str, file_path: Optional[str] = None, details: Optional[str] = None
    ):
        if file_path:
            message = f"Audio extraction failed for '{file_path}': {message}"

        super().__init__(
            message=message, grpc_code=GrpcStatusCode.INVALID_ARGUMENT, details=details
        )


class LanguageDetectionError(WorkerError):
    """
    Raised when language detection fails.

    Examples:
        - Insufficient audio duration
        - Silent audio
        - Unsupported language
    """

    def __init__(
        self, message: str, file_path: Optional[str] = None, details: Optional[str] = None
    ):
        if file_path:
            message = f"Language detection failed for '{file_path}': {message}"

        super().__init__(message=message, grpc_code=GrpcStatusCode.INTERNAL, details=details)


class SubtitleGenerationError(WorkerError):
    """
    Raised when subtitle file generation fails.

    Examples:
        - File write error
        - Permission denied
        - Invalid format
        - Disk full
    """

    def __init__(
        self, message: str, output_path: Optional[str] = None, details: Optional[str] = None
    ):
        if output_path:
            message = f"Subtitle generation failed for '{output_path}': {message}"

        super().__init__(message=message, grpc_code=GrpcStatusCode.INTERNAL, details=details)


class MemoryError(WorkerError):
    """
    Raised when memory constraints are exceeded.

    Examples:
        - Out of memory
        - Memory threshold exceeded
        - VRAM exhausted
    """

    def __init__(
        self,
        message: str,
        current_mb: Optional[int] = None,
        threshold_mb: Optional[int] = None,
        details: Optional[str] = None,
    ):
        if current_mb and threshold_mb:
            message = f"{message} (Current: {current_mb}MB, Threshold: {threshold_mb}MB)"

        super().__init__(
            message=message, grpc_code=GrpcStatusCode.RESOURCE_EXHAUSTED, details=details
        )


def format_validation_errors(errors: list[dict]) -> str:
    """
    Format pydantic validation errors into user-friendly message.

    Args:
        errors: List of pydantic error dicts

    Returns:
        Formatted error message with suggestions
    """
    lines = ["Configuration validation failed:"]

    for error in errors:
        field = ".".join(str(loc) for loc in error["loc"])
        msg = error["msg"]
        error_type = error.get("type", "")

        # Add field-specific suggestions
        suggestion = get_field_suggestion(field, error_type)

        lines.append(f"  - {field}: {msg}")
        if suggestion:
            lines.append(f"    Suggestion: {suggestion}")

    return "\n".join(lines)


def get_field_suggestion(field: str, error_type: str) -> Optional[str]:
    """
    Get helpful suggestion for configuration error.

    Args:
        field: Configuration field path
        error_type: Pydantic error type

    Returns:
        Helpful suggestion or None
    """
    suggestions = {
        "whisper.model_name": "Valid models: tiny, base, small, medium, large, large-v2, large-v3, distil-*",
        "whisper.device": "Valid devices: cpu or cuda (requires CUDA-capable GPU)",
        "whisper.cpu_threads": "Value must be between 1 and 32",
        "whisper.concurrent_transcriptions": "Value must be between 1 and 10",
        "whisper.compute_type": "Valid types: auto, int8, int8_float16, float16, float32",
        "system.grpc_port": "Port must be between 1024 and 65535",
        "system.webhook_port": "Port must be between 1024 and 65535",
        "transcription.task": "Valid tasks: transcribe or translate",
        "transcription.detect_language_length": "Value must be between 1 and 300 seconds",
        "transcription.detect_language_offset": "Value must be 0 or greater",
        "transcription.asr_timeout": "Value must be at least 60 seconds",
        "subtitle.language_naming_type": "Valid types: ISO_639_1, ISO_639_2_T, ISO_639_2_B, NAME, NATIVE",
    }

    # Check for exact match
    if field in suggestions:
        return suggestions[field]

    # Check for partial match
    for key, suggestion in suggestions.items():
        if field.endswith(key.split(".")[-1]):
            return suggestion

    return None
