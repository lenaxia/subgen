"""Language detection module."""

from .detector import (
    detect_language_from_file,
    detect_language_from_bytes,
    choose_transcription_language,
    LanguageDetectionResult,
    LanguageDetectionError,
)

__all__ = [
    "detect_language_from_file",
    "detect_language_from_bytes",
    "choose_transcription_language",
    "LanguageDetectionResult",
    "LanguageDetectionError",
]
