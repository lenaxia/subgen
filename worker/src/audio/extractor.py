"""
Audio extraction and validation module.

Extracted from subgen.py:
- has_audio (lines 2016-2038)
- get_audio_tracks (lines 1446-1490)
- extract_audio_track_to_memory (lines 1352-1386)
- handle_multiple_audio_tracks (lines 1318-1350)
- extract_audio_segment_to_memory (lines 1100-1141)
"""

import io
import logging
from typing import Optional, List, TYPE_CHECKING, Generator, Any
from contextlib import contextmanager
from dataclasses import dataclass

# Lazy imports to avoid import errors during testing
if TYPE_CHECKING:
    import av
    import ffmpeg

logger = logging.getLogger(__name__)


@dataclass
class AudioTrackInfo:
    """Information about an audio track."""

    index: int
    codec: str
    channels: int
    language: str
    is_default: bool
    title: Optional[str] = None


class AudioExtractionError(Exception):
    """Raised when audio extraction fails."""

    pass


def has_audio(file_path: str) -> bool:
    """
    Check if file has valid audio stream.

    Extracted from: subgen.py:2016-2038

    Args:
        file_path: Path to media file

    Returns:
        True if file has valid audio codec
    """
    import av

    try:
        with av.open(file_path) as container:
            for stream in container.streams:
                if stream.type == "audio":
                    if stream.codec_context and stream.codec_context.name != "none":
                        return True
            return False
    except (av.FFmpegError, UnicodeDecodeError) as e:
        logger.debug(f"Error checking audio in {file_path}: {e}")
        return False


def get_audio_tracks(file_path: str) -> List[AudioTrackInfo]:
    """
    Get all audio tracks from media file.

    Extracted from: subgen.py:1446-1490

    Args:
        file_path: Path to media file

    Returns:
        List of AudioTrackInfo objects

    Raises:
        AudioExtractionError: If FFmpeg probe fails
    """
    import ffmpeg

    try:
        probe = ffmpeg.probe(file_path, select_streams="a")
        audio_streams = probe.get("streams", [])

        tracks = []
        for stream in audio_streams:
            track = AudioTrackInfo(
                index=int(stream.get("index", 0)),
                codec=stream.get("codec_name", "Unknown"),
                channels=int(stream.get("channels", 0)),
                language=stream.get("tags", {}).get("language", "und"),
                is_default=stream.get("disposition", {}).get("default", 0) == 1,
                title=stream.get("tags", {}).get("title"),
            )
            tracks.append(track)

        return tracks

    except ffmpeg.Error as e:
        stderr = e.stderr.decode() if e.stderr else str(e)
        logger.error(f"FFmpeg error: {stderr}")
        raise AudioExtractionError(f"Failed to probe audio tracks: {e}")


@contextmanager
def extract_audio_track(file_path: str, track_index: int) -> Generator[io.BytesIO, None, None]:
    """
    Extract specific audio track to memory.

    Extracted from: subgen.py:1352-1386

    Args:
        file_path: Path to media file
        track_index: Index of audio track to extract

    Yields:
        BytesIO object with audio data (16kHz mono WAV)

    Raises:
        AudioExtractionError: If extraction fails
    """
    import ffmpeg

    buffer = None
    try:
        # Extract audio with ffmpeg
        out, _ = (
            ffmpeg.input(file_path)
            .output(
                "pipe:",
                map=f"0:{track_index}",
                format="wav",
                ac=1,  # Mono
                ar=16000,  # 16kHz for Whisper
                loglevel="quiet",
            )
            .run(capture_stdout=True, capture_stderr=True)
        )

        buffer = io.BytesIO(out)
        yield buffer

    except ffmpeg.Error as e:
        stderr = e.stderr.decode() if e.stderr else str(e)
        logger.error(f"FFmpeg error: {stderr}")
        raise AudioExtractionError(f"Failed to extract audio: {e}")

    finally:
        if buffer:
            buffer.close()


def handle_multiple_audio_tracks(
    file_path: str, preferred_language: Optional[str] = None
) -> Optional[io.BytesIO]:
    """
    Extract audio from file with multiple tracks.

    Extracted from: subgen.py:1318-1350

    Args:
        file_path: Path to media file
        preferred_language: Preferred language code (e.g., "eng", "jpn")

    Returns:
        BytesIO with audio data if multiple tracks exist, None otherwise
    """
    tracks = get_audio_tracks(file_path)

    if len(tracks) <= 1:
        return None

    logger.debug(f"Multiple audio tracks found in {file_path}")
    for track in tracks:
        logger.debug(
            f"  Track {track.index}: {track.codec} {track.language} {'(default)' if track.is_default else ''}"
        )

    # Select track
    selected_track = None
    if preferred_language:
        selected_track = next((t for t in tracks if t.language == preferred_language), None)

    if not selected_track:
        selected_track = next((t for t in tracks if t.is_default), tracks[0])

    # Extract audio
    with extract_audio_track(file_path, selected_track.index) as audio_buffer:
        # Read into new BytesIO (context manager will close original)
        audio_data = audio_buffer.read()
        return io.BytesIO(audio_data)


@contextmanager
def extract_audio_segment(
    file_path: str, start_time: int, duration: int
) -> Generator[io.BytesIO, None, None]:
    """
    Extract audio segment from file.

    Extracted from: subgen.py:1100-1141

    Args:
        file_path: Path to media file
        start_time: Start time in seconds
        duration: Duration in seconds

    Yields:
        BytesIO with audio segment

    Raises:
        AudioExtractionError: If extraction fails
    """
    import ffmpeg

    buffer = None
    try:
        out, _ = (
            ffmpeg.input(file_path, ss=start_time, t=duration)
            .output("pipe:1", format="wav", acodec="pcm_s16le", ar=16000)
            .run(capture_stdout=True, capture_stderr=True)
        )

        if not out:
            raise AudioExtractionError("FFmpeg output is empty")

        buffer = io.BytesIO(out)
        yield buffer

    except ffmpeg.Error as e:
        stderr = e.stderr.decode() if e.stderr else str(e)
        logger.error(f"FFmpeg error: {stderr}")
        raise AudioExtractionError(f"Failed to extract segment: {e}")

    finally:
        if buffer:
            buffer.close()
