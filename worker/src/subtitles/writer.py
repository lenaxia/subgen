"""
Subtitle file generation module (SRT and LRC).

Extracted from subgen.py:
- write_lrc (lines 1218-1225)
- name_subtitle (lines 1301-1316)
- appendLine (mentioned in gen_subtitles)
"""

import os
import logging
import sys
from typing import Optional, Any
from pathlib import Path

# Add parent directory to path to import language_code
worker_root = Path(__file__).parent.parent.parent.parent
sys.path.insert(0, str(worker_root))

from language_code import LanguageCode

logger = logging.getLogger(__name__)


class SubtitleGenerationError(Exception):
    """Raised when subtitle generation fails."""

    pass


def generate_subtitle_path(
    media_path: str,
    language: LanguageCode,
    model_name: str,
    show_subgen: bool = True,
    show_model: bool = True,
    format: str = "srt",
) -> str:
    """
    Generate subtitle file path following naming convention.

    Extracted from: subgen.py:1301-1316

    Format: <filename>[.subgen][.<model>].<language>.<format>
    Example: "movie.subgen.medium.eng.srt"

    Args:
        media_path: Path to source media file
        language: Subtitle language
        model_name: Whisper model name (tiny, small, medium, etc.)
        show_subgen: Include ".subgen" in filename
        show_model: Include model name in filename
        format: Subtitle format (srt or lrc)

    Returns:
        Path to subtitle file
    """
    base_path = os.path.splitext(media_path)[0]

    parts = [base_path]

    if show_subgen:
        parts.append(".subgen")

    if show_model:
        parts.append(f".{model_name}")

    # Language code (use ISO 639-2 B)
    lang_code = language.to_iso_639_2_b()
    parts.append(f".{lang_code}")

    parts.append(f".{format}")

    return "".join(parts)


def write_lrc(segments: Any, output_path: str, append_footer: bool = False) -> None:
    """
    Write LRC subtitle file.

    Extracted from: subgen.py:1218-1225

    LRC format: [MM:SS.xx]Text

    Args:
        segments: List of transcription segments
        output_path: Path to write LRC file
        append_footer: Whether to append generation footer

    Raises:
        SubtitleGenerationError: If writing fails
    """
    temp_path = output_path + ".tmp"

    try:
        with open(temp_path, "w", encoding="utf-8") as f:
            for segment in segments:
                minutes, seconds = divmod(int(segment.start), 60)
                fraction = int((segment.start - int(segment.start)) * 100)

                # Remove embedded newlines
                text = segment.text.strip().replace("\n", " ")

                f.write(f"[{minutes:02d}:{seconds:02d}.{fraction:02d}]{text}\n")

            if append_footer:
                f.write("\n[99:99.99]Transcribed by Subgen\n")

        # Atomic rename
        os.replace(temp_path, output_path)
        logger.info(f"LRC subtitle written: {output_path}")

    except Exception as e:
        logger.error(f"Failed to write LRC: {e}", exc_info=True)
        # Cleanup temp file
        if os.path.exists(temp_path):
            os.remove(temp_path)
        raise SubtitleGenerationError(f"Failed to write LRC: {e}")


def write_srt(
    result: Any,  # Transcription result with segments
    output_path: str,
    word_level_highlight: bool = False,
    append_footer: bool = False,
) -> None:
    """
    Write SRT subtitle file.

    Manually generates SRT format from segments (compatible with faster-whisper).

    Args:
        result: Transcription result with segments attribute
        output_path: Path to write SRT file
        word_level_highlight: Enable word-level timestamps (not implemented for faster-whisper)
        append_footer: Whether to append generation footer

    Raises:
        SubtitleGenerationError: If writing fails
    """
    temp_path = output_path + ".tmp"

    try:

        def format_timestamp(seconds: float) -> str:
            """Convert seconds to SRT timestamp format: HH:MM:SS,mmm"""
            hours = int(seconds // 3600)
            minutes = int((seconds % 3600) // 60)
            secs = int(seconds % 60)
            millis = int((seconds % 1) * 1000)
            return f"{hours:02d}:{minutes:02d}:{secs:02d},{millis:03d}"

        with open(temp_path, "w", encoding="utf-8") as f:
            for i, segment in enumerate(result.segments, start=1):
                # Write segment index
                f.write(f"{i}\n")

                # Write timestamps
                start_time = format_timestamp(segment.start)
                end_time = format_timestamp(segment.end)
                f.write(f"{start_time} --> {end_time}\n")

                # Write text (remove embedded newlines)
                text = segment.text.strip()
                f.write(f"{text}\n\n")

            # Append footer if requested
            if append_footer:
                f.write(f"{len(result.segments) + 1}\n")
                f.write("99:59:59,999 --> 99:59:59,999\n")
                f.write("Transcribed by Subgen\n")

        # Atomic rename
        os.replace(temp_path, output_path)
        logger.info(f"SRT subtitle written: {output_path}")

    except Exception as e:
        logger.error(f"Failed to write SRT: {e}", exc_info=True)
        if os.path.exists(temp_path):
            os.remove(temp_path)
        raise SubtitleGenerationError(f"Failed to write SRT: {e}")


def append_line_to_result(result: Any) -> None:
    """
    Append blank line to each segment text.

    Extracted from: subgen.py (appendLine function)

    Modifies result in-place.

    Args:
        result: Transcription result with segments
    """
    for segment in result.segments:
        if not segment.text.endswith("\n"):
            segment.text += "\n"
