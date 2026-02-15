"""Subtitle generation module."""

from .writer import (
    generate_subtitle_path,
    write_lrc,
    write_srt,
    append_line_to_result,
    SubtitleGenerationError,
)

__all__ = [
    "generate_subtitle_path",
    "write_lrc",
    "write_srt",
    "append_line_to_result",
    "SubtitleGenerationError",
]
