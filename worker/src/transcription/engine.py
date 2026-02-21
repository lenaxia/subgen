"""
Main transcription orchestration engine.

Extracted from subgen.py:
- gen_subtitles (lines 1227-1274)
"""

import os
import logging
import sys
import time
from typing import Optional, Any, List
from dataclasses import dataclass
from pathlib import Path

# Add parent directory to path to import language_code
worker_root = Path(__file__).parent.parent.parent.parent
sys.path.insert(0, str(worker_root))

from language_code import LanguageCode

from audio.extractor import has_audio, handle_multiple_audio_tracks, AudioExtractionError
from language.detector import (
    detect_language_from_file,
    detect_language_from_bytes,
    LanguageDetectionError,
)
from subtitles.writer import (
    generate_subtitle_path,
    write_lrc,
    write_srt,
    append_line_to_result,
    SubtitleGenerationError,
)

logger = logging.getLogger(__name__)


@dataclass
class TranscribeOptions:
    """Options for transcription."""

    whisper_model: str = "medium"
    whisper_threads: int = 4
    word_level_highlight: bool = False
    custom_regroup: str = "cm_sl=84_sl=42++++++1"
    lrc_for_audio: bool = True
    custom_prompt: str = ""
    append_footer: bool = False
    subtitle_language_name: str = "aa"
    show_model_in_filename: bool = True
    show_subgen_in_filename: bool = True


@dataclass
class TranscriptionResult:
    """Result of transcription."""

    success: bool
    subtitle_path: str
    detected_language: str
    error_message: Optional[str] = None
    duration_seconds: float = 0.0
    segment_count: int = 0
    transcription_time_ms: int = 0
    peak_memory_mb: int = 0
    segments: Optional[List[Any]] = None  # Whisper segments


class TranscriptionEngine:
    """
    Core transcription engine.

    Extracted logic from: subgen.py:1227-1274 (gen_subtitles)
    """

    def __init__(self, config: Any) -> None:
        """Initialize transcription engine with configuration."""
        self.config = config
        # Model manager will be injected in STORY_03
        self.model: Any = None

    def transcribe(
        self,
        source: Any,  # str (file path) or bytes
        task_type: str,
        force_language: Optional[str],
        options: TranscribeOptions,
    ) -> TranscriptionResult:
        """
        Transcribe audio/video file to subtitles.

        This is the refactored version of gen_subtitles() from subgen.py:1227-1274

        Args:
            source: File path or audio bytes
            task_type: "transcribe" or "translate"
            force_language: Forced language code (ISO 639-1) or None
            options: Transcription options

        Returns:
            TranscriptionResult with subtitle path and metadata
        """
        start_time = time.time()

        # Handle byte content by writing to temp file
        temp_files = []
        try:
            if isinstance(source, bytes):
                # Write bytes to temp file
                import tempfile

                temp_file = tempfile.NamedTemporaryFile(suffix=".wav", delete=False)
                temp_file.write(source)
                temp_file.close()
                file_path = temp_file.name
                temp_files.append(temp_file.name)
                logger.debug(f"Written {len(source)} bytes to temp file: {file_path}")
            else:
                # It's already a file path
                file_path = source

            # Validate file
            if not os.path.exists(file_path):
                raise FileNotFoundError(f"File not found: {file_path}")

            if not has_audio(file_path):
                raise AudioExtractionError(f"No valid audio in file: {file_path}")

            # Check if audio or video file
            file_name, file_extension = os.path.splitext(file_path)
            is_audio_file = file_extension.lower() in (
                ".mp3",
                ".wav",
                ".aac",
                ".flac",
                ".ogg",
                ".m4a",
            )

            # Load model (will be ModelManager in STORY_03)
            # For now, assume self.model is loaded
            if not self.model:
                raise RuntimeError("Model not loaded")

            # Handle multiple audio tracks
            data = file_path
            extracted_audio = handle_multiple_audio_tracks(
                file_path, force_language if force_language else None
            )
            if extracted_audio:
                try:
                    # Write audio bytes to temporary file
                    import tempfile

                    temp_file = tempfile.NamedTemporaryFile(suffix=".wav", delete=False)
                    temp_file.write(extracted_audio.read())
                    temp_file.close()
                    data = temp_file.name

                    # Track temp file for cleanup
                    temp_files = [temp_file.name]
                finally:
                    extracted_audio.close()

            # Prepare transcription args
            args = {}

            # Note: regroup is a stable-whisper feature, not available in faster-whisper
            # We're using stable-whisper wrapper around faster-whisper, so check if regroup is supported
            # For now, skip custom_regroup since we're using load_faster_whisper
            # if options.custom_regroup and options.custom_regroup.lower() != "default":
            #     args["regroup"] = options.custom_regroup

            # Transcribe
            lang_code = force_language if force_language else None
            # faster-whisper returns (segments_generator, info)
            segments_generator, info = self.model.transcribe(
                data, language=lang_code, task=task_type, **args
            )

            # Convert generator to list to get segments
            segments = list(segments_generator)

            # Create a result object compatible with stable-whisper expectations
            class FasterWhisperResult:
                def __init__(self, segments, language):
                    self.segments = segments
                    self.language = language

            result = FasterWhisperResult(segments, info.language)

            # Append newlines to segments
            append_line_to_result(result)

            # Determine detected language
            detected_lang: LanguageCode
            if force_language:
                # Convert string to LanguageCode if provided
                if isinstance(force_language, str):
                    detected_lang = LanguageCode.from_iso_639_1(force_language)
                else:
                    detected_lang = force_language  # Already a LanguageCode
            else:
                detected_lang = LanguageCode.from_string(result.language)

            # Generate subtitle file
            if is_audio_file and options.lrc_for_audio:
                # Write LRC for audio files
                subtitle_path = file_name + ".lrc"
                write_lrc(result.segments, subtitle_path, append_footer=options.append_footer)
            else:
                # Write SRT for video files
                subtitle_path = generate_subtitle_path(
                    file_path,
                    detected_lang,
                    options.whisper_model,
                    show_subgen=options.show_subgen_in_filename,
                    show_model=options.show_model_in_filename,
                    format="srt",
                )
                write_srt(
                    result,
                    subtitle_path,
                    word_level_highlight=options.word_level_highlight,
                    append_footer=options.append_footer,
                )

            # Calculate stats
            duration = time.time() - start_time

            return TranscriptionResult(
                success=True,
                subtitle_path=subtitle_path,
                detected_language=detected_lang.to_iso_639_1(),
                duration_seconds=duration,
                segment_count=len(result.segments),
                transcription_time_ms=int(duration * 1000),
                segments=result.segments,
            )

        except Exception as e:
            logger.exception(f"Transcription failed: {e}")
            return TranscriptionResult(
                success=False, subtitle_path="", detected_language="", error_message=str(e)
            )
        finally:
            # Clean up temp files if we created any
            for temp_file_path in temp_files:
                if os.path.exists(temp_file_path):
                    try:
                        os.unlink(temp_file_path)
                        logger.debug(f"Cleaned up temp file: {temp_file_path}")
                    except Exception as e:
                        logger.warning(f"Failed to clean up temp file {temp_file_path}: {e}")

    def detect_language(
        self,
        source: Any,  # str (file path) or bytes
        sample_length: int,
        sample_offset: int,
    ) -> Any:  # LanguageDetectionResult
        """
        Detect language from audio source.

        Args:
            source: File path or audio bytes
            sample_length: Sample duration in seconds
            sample_offset: Start offset in seconds

        Returns:
            LanguageDetectionResult

        Raises:
            RuntimeError: If model not loaded
        """
        if not self.model:
            raise RuntimeError("Model not loaded")

        if isinstance(source, str):
            return detect_language_from_file(
                source, self.model, sample_offset=sample_offset, sample_length=sample_length
            )
        else:
            return detect_language_from_bytes(source, self.model)

    def is_model_loaded(self) -> bool:
        """Check if model is loaded."""
        return self.model is not None
