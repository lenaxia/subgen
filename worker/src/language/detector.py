"""
Language detection from audio module.

Extracted from subgen.py:
- detect_language_task (lines 1050-1098)
- choose_transcribe_language (lines 1404-1444)
"""

import logging
import os
import sys
from typing import Union, Optional, Any
from dataclasses import dataclass
from pathlib import Path

# Add parent directory to path to import language_code
worker_root = Path(__file__).parent.parent.parent.parent
sys.path.insert(0, str(worker_root))

from language_code import LanguageCode

logger = logging.getLogger(__name__)


@dataclass
class LanguageDetectionResult:
    """Result of language detection."""

    language_code: str  # ISO 639-1 (e.g., "en")
    language_name: str  # English name (e.g., "English")
    confidence: float  # 0.0 - 1.0


class LanguageDetectionError(Exception):
    """Raised when language detection fails."""

    pass


def detect_language_from_file(
    file_path: str,
    model: Any,  # WhisperModel (will be properly typed in STORY_03)
    sample_offset: int = 0,
    sample_length: int = 30,
) -> LanguageDetectionResult:
    """
    Detect language from a short audio sample extracted from the file.

    Extracted from: subgen.py:1050-1098

    Only a [sample_offset, sample_offset + sample_length] clip is decoded —
    never the full file. faster-whisper detects language from the first
    window of features, so the segments generator is deliberately NOT
    consumed: iterating it would decode the entire sample (and, on a full
    file, the entire movie) which is wasted work and peak memory.

    Args:
        file_path: Path to media file
        model: Loaded Whisper model
        sample_offset: Start offset in seconds
        sample_length: Sample duration in seconds

    Returns:
        LanguageDetectionResult with detected language

    Raises:
        LanguageDetectionError: If detection fails
    """
    import tempfile

    from audio.extractor import extract_audio_segment

    temp_path = None
    try:
        logger.info(f"Detecting language: {file_path} ({sample_length}s from {sample_offset}s)")

        # Extract only the sample we need. The context manager closes the
        # buffer; we need the bytes on disk for faster-whisper.
        with extract_audio_segment(file_path, sample_offset, sample_length) as sample:
            with tempfile.NamedTemporaryFile(suffix=".wav", delete=False) as tf:
                # Capture the path BEFORE writing: a write/read failure still
                # leaves the created file, so finally-cleanup must know it.
                temp_path = tf.name
                tf.write(sample.read())

        segments_generator, info = model.transcribe(temp_path, beam_size=5)

        # Language is detected from the initial features; do NOT iterate the
        # generator — that would decode the whole sample for nothing.
        detected_lang = LanguageCode.from_iso_639_1(info.language)

        logger.info(
            f"Detected language: {detected_lang.to_name()} (confidence: {info.language_probability:.2f})"
        )

        return LanguageDetectionResult(
            language_code=detected_lang.to_iso_639_1(),
            language_name=detected_lang.to_name(),
            confidence=info.language_probability,
        )

    except LanguageDetectionError:
        raise
    except Exception as e:
        logger.error(f"Language detection failed: {e}", exc_info=True)
        raise LanguageDetectionError(f"Failed to detect language: {e}")
    finally:
        if temp_path and os.path.exists(temp_path):
            try:
                os.unlink(temp_path)
            except Exception as cleanup_error:
                logger.warning(f"Failed to clean up temp file {temp_path}: {cleanup_error}")


def detect_language_from_bytes(
    audio_bytes: bytes,
    model: Any,  # WhisperModel
) -> LanguageDetectionResult:
    """
    Detect language from audio bytes.

    Args:
        audio_bytes: Raw audio data
        model: Loaded Whisper model

    Returns:
        LanguageDetectionResult with detected language

    Raises:
        LanguageDetectionError: If detection fails
    """
    import tempfile
    import os

    temp_file = None
    try:
        # Write bytes to temp file (same approach as engine.transcribe)
        temp_file = tempfile.NamedTemporaryFile(suffix=".wav", delete=False)
        temp_file.write(audio_bytes)
        temp_file.close()
        file_path = temp_file.name
        logger.debug(
            f"Written {len(audio_bytes)} bytes to temp file for language detection: {file_path}"
        )

        # Detect on the temp file; language comes from info immediately.
        # Do NOT iterate the segments generator — that decodes all audio
        # for nothing (Bazarr samples are short, but the full decode is
        # still wasted work and memory).
        segments_generator, info = model.transcribe(file_path, beam_size=5)

        detected_lang = LanguageCode.from_iso_639_1(info.language)

        return LanguageDetectionResult(
            language_code=detected_lang.to_iso_639_1(),
            language_name=detected_lang.to_name(),
            confidence=info.language_probability,
        )

    except Exception as e:
        logger.error(f"Language detection failed: {e}", exc_info=True)
        raise LanguageDetectionError(f"Failed to detect language: {e}")
    finally:
        # Clean up temp file
        if temp_file and os.path.exists(temp_file.name):
            try:
                os.unlink(temp_file.name)
                logger.debug(f"Cleaned up temp file: {temp_file.name}")
            except Exception as cleanup_error:
                logger.warning(f"Failed to clean up temp file {temp_file.name}: {cleanup_error}")


def choose_transcription_language(
    file_path: str,
    forced_language: Optional[LanguageCode],
    force_detected_language_to: Optional[LanguageCode],
    preferred_audio_languages: list,
) -> LanguageCode:
    """
    Determine language for transcription.

    Extracted from: subgen.py:1404-1444

    Priority:
    1. forced_language (from user)
    2. force_detected_language_to (from config)
    3. Preferred audio language from tracks
    4. Default audio track language
    5. None (auto-detect during transcription)

    Args:
        file_path: Path to media file
        forced_language: User-specified language
        force_detected_language_to: Config override
        preferred_audio_languages: Preferred languages in order

    Returns:
        LanguageCode to use for transcription
    """
    from audio.extractor import get_audio_tracks

    # Priority 1: User forced
    if forced_language:
        logger.debug(f"Using forced language: {forced_language}")
        return forced_language

    # Priority 2: Config override
    if force_detected_language_to:
        logger.debug(f"Using config language override: {force_detected_language_to}")
        return force_detected_language_to

    # Priority 3 & 4: From audio tracks
    try:
        audio_tracks = get_audio_tracks(file_path)

        # Try preferred languages
        for preferred in preferred_audio_languages:
            for track in audio_tracks:
                if track.language == preferred.to_iso_639_2_b():
                    logger.debug(f"Using preferred audio language: {preferred}")
                    return preferred

        # Use default track language
        default_track = next((t for t in audio_tracks if t.is_default), None)
        if default_track and default_track.language != "und":
            try:
                lang = LanguageCode.from_iso_639_2(default_track.language)
                logger.debug(f"Using default track language: {lang}")
                return lang
            except ValueError:
                # Invalid language code, fall through
                pass
    except Exception as e:
        logger.debug(f"Could not determine language from audio tracks: {e}")

    # Priority 5: Auto-detect
    return LanguageCode.NONE
