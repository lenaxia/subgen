"""Audio extraction and validation module."""

from .extractor import (
    has_audio,
    get_audio_tracks,
    extract_audio_track,
    handle_multiple_audio_tracks,
    extract_audio_segment,
    AudioTrackInfo,
    AudioExtractionError,
)

__all__ = [
    "has_audio",
    "get_audio_tracks",
    "extract_audio_track",
    "handle_multiple_audio_tracks",
    "extract_audio_segment",
    "AudioTrackInfo",
    "AudioExtractionError",
]
