"""
Unit tests for error handling and custom exceptions.

Tests exception hierarchy, gRPC status code mapping,
and error message formatting.
"""

import pytest

from utils.errors import (
    WorkerError,
    ConfigurationError,
    ModelLoadError,
    TranscriptionError,
    AudioExtractionError,
    LanguageDetectionError,
    SubtitleGenerationError,
    MemoryError,
    GrpcStatusCode,
    format_validation_errors,
    get_field_suggestion,
)


class TestWorkerError:
    """Test base WorkerError exception."""

    def test_worker_error_message(self):
        """Test WorkerError with message only."""
        error = WorkerError("Test error")

        assert str(error) == "Test error"
        assert error.message == "Test error"
        assert error.grpc_code == GrpcStatusCode.INTERNAL

    def test_worker_error_with_details(self):
        """Test WorkerError with details."""
        error = WorkerError("Test error", details="Additional info")

        assert "Test error" in str(error)
        assert "Additional info" in str(error)

    def test_worker_error_grpc_status(self):
        """Test gRPC status conversion."""
        error = WorkerError("Test error", grpc_code=GrpcStatusCode.INVALID_ARGUMENT)

        code, message = error.to_grpc_status()

        assert code == 3  # INVALID_ARGUMENT
        assert message == "Test error"


class TestConfigurationError:
    """Test ConfigurationError exception."""

    def test_configuration_error_grpc_code(self):
        """Test ConfigurationError has correct gRPC code."""
        error = ConfigurationError("Invalid config")

        assert error.grpc_code == GrpcStatusCode.FAILED_PRECONDITION
        code, _ = error.to_grpc_status()
        assert code == 9  # FAILED_PRECONDITION

    def test_configuration_error_with_details(self):
        """Test ConfigurationError with validation details."""
        error = ConfigurationError(
            "Configuration validation failed", details="Field 'model_name' has invalid value"
        )

        assert "Configuration validation failed" in str(error)
        assert "model_name" in str(error)


class TestModelLoadError:
    """Test ModelLoadError exception."""

    def test_model_load_error_basic(self):
        """Test basic ModelLoadError."""
        error = ModelLoadError("Failed to load")

        assert "Failed to load" in str(error)
        assert error.grpc_code == GrpcStatusCode.UNAVAILABLE

    def test_model_load_error_with_model_name(self):
        """Test ModelLoadError with model name."""
        error = ModelLoadError("CUDA out of memory", model_name="large-v3")

        assert "large-v3" in str(error)
        assert "CUDA out of memory" in str(error)

    def test_model_load_error_grpc_code(self):
        """Test ModelLoadError gRPC status code."""
        error = ModelLoadError("Failed")

        code, _ = error.to_grpc_status()
        assert code == 14  # UNAVAILABLE


class TestTranscriptionError:
    """Test TranscriptionError exception."""

    def test_transcription_error_basic(self):
        """Test basic TranscriptionError."""
        error = TranscriptionError("Transcription failed")

        assert "Transcription failed" in str(error)
        assert error.grpc_code == GrpcStatusCode.INTERNAL

    def test_transcription_error_with_file_path(self):
        """Test TranscriptionError with file path."""
        error = TranscriptionError("Audio too short", file_path="/media/movie.mp4")

        assert "/media/movie.mp4" in str(error)
        assert "Audio too short" in str(error)

    def test_transcription_error_with_details(self):
        """Test TranscriptionError with additional details."""
        error = TranscriptionError(
            "Timeout",
            file_path="/media/movie.mp4",
            details="Transcription took longer than 5 hours",
        )

        assert "Timeout" in str(error)
        assert "/media/movie.mp4" in str(error)
        assert "5 hours" in str(error)


class TestAudioExtractionError:
    """Test AudioExtractionError exception."""

    def test_audio_extraction_error_basic(self):
        """Test basic AudioExtractionError."""
        error = AudioExtractionError("No audio track found")

        assert "No audio track found" in str(error)
        assert error.grpc_code == GrpcStatusCode.INVALID_ARGUMENT

    def test_audio_extraction_error_with_file_path(self):
        """Test AudioExtractionError with file path."""
        error = AudioExtractionError("Unsupported codec", file_path="/media/file.mkv")

        assert "/media/file.mkv" in str(error)
        assert "Unsupported codec" in str(error)

    def test_audio_extraction_error_grpc_code(self):
        """Test AudioExtractionError gRPC code."""
        error = AudioExtractionError("Error")

        code, _ = error.to_grpc_status()
        assert code == 3  # INVALID_ARGUMENT


class TestLanguageDetectionError:
    """Test LanguageDetectionError exception."""

    def test_language_detection_error_basic(self):
        """Test basic LanguageDetectionError."""
        error = LanguageDetectionError("Detection failed")

        assert "Detection failed" in str(error)
        assert error.grpc_code == GrpcStatusCode.INTERNAL

    def test_language_detection_error_with_file_path(self):
        """Test LanguageDetectionError with file path."""
        error = LanguageDetectionError("Silent audio", file_path="/media/audio.mp3")

        assert "/media/audio.mp3" in str(error)
        assert "Silent audio" in str(error)


class TestSubtitleGenerationError:
    """Test SubtitleGenerationError exception."""

    def test_subtitle_generation_error_basic(self):
        """Test basic SubtitleGenerationError."""
        error = SubtitleGenerationError("Failed to write file")

        assert "Failed to write file" in str(error)
        assert error.grpc_code == GrpcStatusCode.INTERNAL

    def test_subtitle_generation_error_with_output_path(self):
        """Test SubtitleGenerationError with output path."""
        error = SubtitleGenerationError("Permission denied", output_path="/media/output.srt")

        assert "/media/output.srt" in str(error)
        assert "Permission denied" in str(error)


class TestMemoryError:
    """Test MemoryError exception."""

    def test_memory_error_basic(self):
        """Test basic MemoryError."""
        error = MemoryError("Memory threshold exceeded")

        assert "Memory threshold exceeded" in str(error)
        assert error.grpc_code == GrpcStatusCode.RESOURCE_EXHAUSTED

    def test_memory_error_with_values(self):
        """Test MemoryError with memory values."""
        error = MemoryError("Memory threshold exceeded", current_mb=3500, threshold_mb=3000)

        assert "3500MB" in str(error)
        assert "3000MB" in str(error)

    def test_memory_error_grpc_code(self):
        """Test MemoryError gRPC code."""
        error = MemoryError("Out of memory")

        code, _ = error.to_grpc_status()
        assert code == 8  # RESOURCE_EXHAUSTED


class TestValidationErrorFormatting:
    """Test validation error formatting helpers."""

    def test_format_validation_errors_single(self):
        """Test formatting single validation error."""
        errors = [{"loc": ("whisper", "model_name"), "msg": "Invalid value", "type": "value_error"}]

        result = format_validation_errors(errors)

        assert "Configuration validation failed" in result
        assert "whisper.model_name" in result
        assert "Invalid value" in result

    def test_format_validation_errors_multiple(self):
        """Test formatting multiple validation errors."""
        errors = [
            {"loc": ("whisper", "model_name"), "msg": "Invalid model", "type": "value_error"},
            {"loc": ("system", "grpc_port"), "msg": "Port out of range", "type": "value_error"},
        ]

        result = format_validation_errors(errors)

        assert "whisper.model_name" in result
        assert "system.grpc_port" in result
        assert "Invalid model" in result
        assert "Port out of range" in result

    def test_format_validation_errors_with_suggestions(self):
        """Test validation errors include suggestions."""
        errors = [{"loc": ("whisper", "model_name"), "msg": "Invalid value", "type": "value_error"}]

        result = format_validation_errors(errors)

        # Should include suggestion for model_name
        assert "Suggestion:" in result
        assert "tiny" in result or "medium" in result


class TestFieldSuggestions:
    """Test field-specific error suggestions."""

    def test_suggestion_for_model_name(self):
        """Test suggestion for whisper.model_name."""
        suggestion = get_field_suggestion("whisper.model_name", "value_error")

        assert suggestion is not None
        assert "tiny" in suggestion
        assert "medium" in suggestion
        assert "large" in suggestion

    def test_suggestion_for_device(self):
        """Test suggestion for whisper.device."""
        suggestion = get_field_suggestion("whisper.device", "value_error")

        assert suggestion is not None
        assert "cpu" in suggestion or "cuda" in suggestion

    def test_suggestion_for_port(self):
        """Test suggestion for port fields."""
        suggestion = get_field_suggestion("system.grpc_port", "value_error")

        assert suggestion is not None
        assert "1024" in suggestion
        assert "65535" in suggestion

    def test_suggestion_for_task(self):
        """Test suggestion for transcription.task."""
        suggestion = get_field_suggestion("transcription.task", "value_error")

        assert suggestion is not None
        assert "transcribe" in suggestion
        assert "translate" in suggestion

    def test_suggestion_for_unknown_field(self):
        """Test suggestion for unknown field returns None."""
        suggestion = get_field_suggestion("unknown.field", "value_error")

        assert suggestion is None


class TestGrpcStatusCodes:
    """Test gRPC status code enum."""

    def test_grpc_status_code_values(self):
        """Test gRPC status code values match spec."""
        assert GrpcStatusCode.OK.value == 0
        assert GrpcStatusCode.INVALID_ARGUMENT.value == 3
        assert GrpcStatusCode.NOT_FOUND.value == 5
        assert GrpcStatusCode.INTERNAL.value == 13
        assert GrpcStatusCode.UNAVAILABLE.value == 14

    def test_all_error_types_have_grpc_code(self):
        """Test all error types have associated gRPC code."""
        error_classes = [
            ConfigurationError,
            ModelLoadError,
            TranscriptionError,
            AudioExtractionError,
            LanguageDetectionError,
            SubtitleGenerationError,
            MemoryError,
        ]

        for error_class in error_classes:
            error = error_class("Test error")
            assert hasattr(error, "grpc_code")
            assert isinstance(error.grpc_code, GrpcStatusCode)
            code, _ = error.to_grpc_status()
            assert isinstance(code, int)
            assert 0 <= code <= 16
