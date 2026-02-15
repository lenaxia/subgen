# Story 02: Modular Refactor - Extract Transcription Logic

**Epic**: EPIC_02 - Python Worker Refactor  
**Status**: Not Started  
**Priority**: Critical  
**Estimated Effort**: 10-12 hours  
**Assignee**: TBD

---

## User Story

As a **Python developer**,  
I want **transcription logic extracted from subgen.py into clean, modular components**,  
So that **the code is testable, maintainable, and integrates with the gRPC server**.

---

## Background

The legacy `subgen.py` is a 2,144-line monolith with tightly coupled functions. This story extracts the core transcription logic into modular components with clear responsibilities:

- **Audio extraction** - Extract audio from video files
- **Language detection** - Detect spoken language
- **Transcription** - Core Whisper transcription
- **Subtitle generation** - Generate SRT/LRC files

**Critical**: This story requires thorough analysis of the legacy code to preserve existing functionality while making it modular and testable.

---

## Acceptance Criteria

- [ ] `worker/transcription/` module created with 5 files
- [ ] `audio.py` - Audio extraction functions extracted
- [ ] `language.py` - Language detection extracted
- [ ] `engine.py` - Core transcription logic extracted
- [ ] `subtitles.py` - SRT/LRC generation extracted
- [ ] `model.py` - Placeholder for model management (STORY_03)
- [ ] All functions have type hints
- [ ] All functions have docstrings
- [ ] No global variables (all state passed as parameters)
- [ ] Unit tests for each module (60+ tests total)
- [ ] Integration test for full transcription pipeline
- [ ] Legacy `gen_subtitles` functionality preserved
- [ ] Work log created

---

## Legacy Code Analysis

### 1. gen_subtitles Function (Lines 1227-1274)

**Location**: `subgen.py:1227-1274`

**Current Implementation**:
```python
def gen_subtitles(file_path: str, transcription_type: str, force_language: LanguageCode = LanguageCode.NONE) -> None:
    """Generates subtitles for a video file."""
    try:
        start_model()
        
        # Check if the file is an audio file
        file_name, file_extension = os.path.splitext(file_path)
        is_audio_file = isAudioFileExtension(file_extension)
        
        data = file_path
        # Extract audio from the file if it has multiple audio tracks
        extracted_audio_file = handle_multiple_audio_tracks(file_path, force_language)
        if extracted_audio_file:
            data = extracted_audio_file.read()
        
        args = {}
        display_name = os.path.basename(file_path)
        args['progress_callback'] = ProgressHandler(display_name)
            
        if custom_regroup and custom_regroup.lower() != 'default':
            args['regroup'] = custom_regroup
            
        args.update(kwargs)
        
        result = model.transcribe(data, language=force_language.to_iso_639_1(), task=transcription_type, verbose=None, **args)

        appendLine(result)

        # If it is an audio file, write the LRC file
        if is_audio_file and lrc_for_audio_files:
            write_lrc(result, file_name + '.lrc')
        else:
            if not force_language: 
                force_language = LanguageCode.from_string(result.language)
            result.to_srt_vtt(name_subtitle(file_path, force_language), word_level=word_level_highlight)

    except Exception as e:
        logging.info(f"Error processing or transcribing {file_path} in {force_language}: {e}")

    finally:
        delete_model()
```

**Analysis**:
- Calls `start_model()` - need to refactor to ModelManager (STORY_03)
- Handles both audio and video files
- Extracts audio if multiple tracks exist
- Passes `custom_regroup` to stable-ts
- Writes LRC for audio files, SRT for video files
- Always calls `delete_model()` in finally

**New Design**:
- Extract into `TranscriptionEngine.transcribe()` method
- Pass model manager as dependency
- Return result object instead of writing files directly
- Use context managers for resource cleanup

---

### 2. handle_multiple_audio_tracks Function (Lines 1318-1350)

**Location**: `subgen.py:1318-1350`

**Current Implementation**:
```python
def handle_multiple_audio_tracks(file_path: str, language: LanguageCode | None = None) -> BytesIO | None:
    """
    Handles the possibility of a media file having multiple audio tracks. 
    
    If the media file has multiple audio tracks, it will extract the audio track of the selected language.
    """
    audio_bytes = None
    audio_tracks = get_audio_tracks(file_path)

    if len(audio_tracks) > 1:
        logging.debug(f"Handling multiple audio tracks from {file_path}")
        logging.debug(
            "Audio tracks:\n"
            + "\n".join([f"  - {track['index']}: {track['codec']} {track['language']} {('default' if track['default'] else '')}" for track in audio_tracks])
        )

        if language is not None:
            audio_track = get_audio_track_by_language(audio_tracks, language)
        if audio_track is None:
            audio_track = audio_tracks[0]
        
        audio_bytes = extract_audio_track_to_memory(file_path, audio_track["index"])
        if audio_bytes is None:
            logging.error(f"Failed to extract audio track {audio_track['index']} from {file_path}")
            return None
    return audio_bytes
```

**Dependencies**:
- `get_audio_tracks()` (lines 1446-1490)
- `get_audio_track_by_language()` (lines 1388-1402)
- `extract_audio_track_to_memory()` (lines 1352-1386)

**New Design**:
- Move to `worker/transcription/audio.py`
- Use context managers for BytesIO
- Add proper error handling with custom exceptions
- Make testable with mock audio tracks

---

### 3. detect_language_task Function (Lines 1050-1098)

**Location**: `subgen.py:1050-1098`

**Current Implementation**:
```python
def detect_language_task(path, original_task_data=None):
    """
    Worker function that detects language for a local file.
    Then queues the actual transcription with the detected language.
    """
    detected_language = LanguageCode.NONE
    
    try:
        logging.info(
            f"Detecting language of file: {path} "
            f"({detect_language_length}s starting at {detect_language_offset}s)"
        )
        
        start_model()
        
        audio_segment = extract_audio_segment_to_memory(
            path, 
            detect_language_offset, 
            int(detect_language_length)
        ).read()
        
        detected_language = LanguageCode.from_name(model.transcribe(audio_segment).language)
        
        logging.info(f"Detected language: {detected_language.to_name()}")

    except Exception as e:
        logging.error(f"Error detecting language for file: {e}", exc_info=True)
        
    finally:
        delete_model()
        
        # Queue transcription with detected language (REMOVED in new design)
```

**Analysis**:
- Extracts audio segment (default: 30 seconds from offset 0)
- Uses Whisper model to detect language
- Uses `LanguageCode` class for language handling
- Old design: queues transcription task (removed in new architecture)

**New Design**:
- Move to `worker/transcription/language.py`
- Return language result, don't queue tasks
- Support both file path and audio bytes
- Add confidence score

---

### 4. has_audio Function (Lines 2016-2038)

**Location**: `subgen.py:2016-2038`

**Current Implementation**:
```python
def has_audio(file_path):
    try:
        if not is_valid_path(file_path):
            return False

        if not (has_video_extension(file_path) or has_audio_extension(file_path)):
            return False

        with av.open(file_path) as container:
            # Check for an audio stream and ensure it has a valid codec
            for stream in container.streams:
                if stream.type == 'audio':
                    # Check if the stream has a codec and if it is valid
                    if stream.codec_context and stream.codec_context.name != 'none':
                        return True
                    else:
                        logging.debug(f"Unsupported or missing codec for audio stream in {file_path}")
            return False

    except (av.FFmpegError, UnicodeDecodeError):
        logging.debug(f"Error processing file {file_path}")
        return False
```

**Analysis**:
- Uses `av` library to check for audio streams
- Validates codec is not 'none'
- Returns boolean

**New Design**:
- Move to `worker/transcription/audio.py`
- Return detailed audio info (codec, channels, etc.)
- Use as validation before transcription

---

### 5. Subtitle Generation Functions

**write_lrc** (Lines 1218-1225):
```python
def write_lrc(result, file_path):
    with open(file_path, "w") as file:
        for segment in result.segments:
            minutes, seconds = divmod(int(segment.start), 60)
            fraction = int((segment.start - int(segment.start)) * 100)
            text = segment.text[:].replace('\n', '')
            file.write(f"[{minutes:02d}:{seconds:02d}.{fraction:02d}]{text}\n")
```

**name_subtitle** (Lines 1301-1316):
```python
def name_subtitle(file_path: str, language: LanguageCode) -> str:
    subgen_part = ".subgen" if show_in_subname_subgen else ""
    model_part = f".{whisper_model}" if show_in_subname_model else ""
    lang_part = define_subtitle_language_naming(language, subtitle_language_naming_type)
    
    return f"{os.path.splitext(file_path)[0]}{subgen_part}{model_part}.{lang_part}.srt"
```

**New Design**:
- Move to `worker/transcription/subtitles.py`
- Support both SRT and LRC formats
- Atomic writes (write to temp, then rename)
- Configurable naming conventions

---

## Technical Design

### Module Structure

```
worker/transcription/
├── __init__.py
├── engine.py              # Main transcription orchestration
├── audio.py               # Audio extraction and validation
├── language.py            # Language detection
├── subtitles.py           # SRT/LRC generation
└── model.py               # Model lifecycle (placeholder for STORY_03)
```

### 1. Audio Module (`worker/transcription/audio.py`)

```python
"""Audio extraction and validation."""

import io
import logging
from typing import Optional, List, Dict
from contextlib import contextmanager
import av
import ffmpeg
from dataclasses import dataclass

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
    try:
        with av.open(file_path) as container:
            for stream in container.streams:
                if stream.type == 'audio':
                    if stream.codec_context and stream.codec_context.name != 'none':
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
    """
    try:
        probe = ffmpeg.probe(file_path, select_streams='a')
        audio_streams = probe.get('streams', [])
        
        tracks = []
        for stream in audio_streams:
            track = AudioTrackInfo(
                index=int(stream.get("index", 0)),
                codec=stream.get("codec_name", "Unknown"),
                channels=int(stream.get("channels", 0)),
                language=stream.get("tags", {}).get("language", "und"),
                is_default=stream.get("disposition", {}).get("default", 0) == 1,
                title=stream.get("tags", {}).get("title")
            )
            tracks.append(track)
        
        return tracks
        
    except ffmpeg.Error as e:
        logger.error(f"FFmpeg error: {e.stderr}")
        raise AudioExtractionError(f"Failed to probe audio tracks: {e}")


@contextmanager
def extract_audio_track(file_path: str, track_index: int):
    """
    Extract specific audio track to memory.
    
    Extracted from: subgen.py:1352-1386
    
    Args:
        file_path: Path to media file
        track_index: Index of audio track to extract
        
    Yields:
        BytesIO object with audio data (16kHz mono WAV)
    """
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
                loglevel="quiet"
            )
            .run(capture_stdout=True, capture_stderr=True)
        )
        
        buffer = io.BytesIO(out)
        yield buffer
        
    except ffmpeg.Error as e:
        logger.error(f"FFmpeg error: {e.stderr.decode()}")
        raise AudioExtractionError(f"Failed to extract audio: {e}")
        
    finally:
        if buffer:
            buffer.close()


def handle_multiple_audio_tracks(file_path: str, preferred_language: Optional[str] = None) -> Optional[io.BytesIO]:
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
        logger.debug(f"  Track {track.index}: {track.codec} {track.language} {'(default)' if track.is_default else ''}")
    
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
def extract_audio_segment(file_path: str, start_time: int, duration: int):
    """
    Extract audio segment from file.
    
    Extracted from: subgen.py:1100-1141
    
    Args:
        file_path: Path to media file
        start_time: Start time in seconds
        duration: Duration in seconds
        
    Yields:
        BytesIO with audio segment
    """
    buffer = None
    try:
        out, _ = (
            ffmpeg
            .input(file_path, ss=start_time, t=duration)
            .output('pipe:1', format='wav', acodec='pcm_s16le', ar=16000)
            .run(capture_stdout=True, capture_stderr=True)
        )
        
        if not out:
            raise AudioExtractionError("FFmpeg output is empty")
        
        buffer = io.BytesIO(out)
        yield buffer
        
    except ffmpeg.Error as e:
        logger.error(f"FFmpeg error: {e.stderr.decode()}")
        raise AudioExtractionError(f"Failed to extract segment: {e}")
        
    finally:
        if buffer:
            buffer.close()
```

### 2. Language Detection Module (`worker/transcription/language.py`)

```python
"""Language detection from audio."""

import logging
from typing import Union, Optional
from dataclasses import dataclass
from language_code import LanguageCode

logger = logging.getLogger(__name__)


@dataclass
class LanguageDetectionResult:
    """Result of language detection."""
    language_code: str  # ISO 639-1 (e.g., "en")
    language_name: str  # English name (e.g., "English")
    confidence: float   # 0.0 - 1.0


class LanguageDetectionError(Exception):
    """Raised when language detection fails."""
    pass


def detect_language_from_file(
    file_path: str,
    model,  # WhisperModel (will be properly typed in STORY_03)
    sample_offset: int = 0,
    sample_length: int = 30
) -> LanguageDetectionResult:
    """
    Detect language from audio file.
    
    Extracted from: subgen.py:1050-1098
    
    Args:
        file_path: Path to media file
        model: Loaded Whisper model
        sample_offset: Start offset in seconds
        sample_length: Sample duration in seconds
        
    Returns:
        LanguageDetectionResult with detected language
    """
    from .audio import extract_audio_segment
    
    try:
        logger.info(f"Detecting language: {file_path} ({sample_length}s from {sample_offset}s)")
        
        # Extract audio segment
        with extract_audio_segment(file_path, sample_offset, sample_length) as audio_buffer:
            audio_bytes = audio_buffer.read()
        
        # Detect language with Whisper
        result = model.transcribe(audio_bytes)
        detected_lang = LanguageCode.from_name(result.language)
        
        logger.info(f"Detected language: {detected_lang.to_name()}")
        
        return LanguageDetectionResult(
            language_code=detected_lang.to_iso_639_1(),
            language_name=detected_lang.to_name(),
            confidence=1.0  # Whisper doesn't provide confidence score
        )
        
    except Exception as e:
        logger.error(f"Language detection failed: {e}", exc_info=True)
        raise LanguageDetectionError(f"Failed to detect language: {e}")


def detect_language_from_bytes(
    audio_bytes: bytes,
    model  # WhisperModel
) -> LanguageDetectionResult:
    """
    Detect language from audio bytes.
    
    Args:
        audio_bytes: Raw audio data
        model: Loaded Whisper model
        
    Returns:
        LanguageDetectionResult with detected language
    """
    try:
        result = model.transcribe(audio_bytes)
        detected_lang = LanguageCode.from_name(result.language)
        
        return LanguageDetectionResult(
            language_code=detected_lang.to_iso_639_1(),
            language_name=detected_lang.to_name(),
            confidence=1.0
        )
        
    except Exception as e:
        logger.error(f"Language detection failed: {e}", exc_info=True)
        raise LanguageDetectionError(f"Failed to detect language: {e}")


def choose_transcription_language(
    file_path: str,
    forced_language: Optional[LanguageCode],
    force_detected_language_to: Optional[LanguageCode],
    preferred_audio_languages: list
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
    from .audio import get_audio_tracks
    
    # Priority 1: User forced
    if forced_language:
        logger.debug(f"Using forced language: {forced_language}")
        return forced_language
    
    # Priority 2: Config override
    if force_detected_language_to:
        logger.debug(f"Using config language override: {force_detected_language_to}")
        return force_detected_language_to
    
    # Priority 3 & 4: From audio tracks
    audio_tracks = get_audio_tracks(file_path)
    
    # Try preferred languages
    for preferred in preferred_audio_languages:
        for track in audio_tracks:
            if track.language == preferred.to_iso_639_2():
                logger.debug(f"Using preferred audio language: {preferred}")
                return preferred
    
    # Use default track language
    default_track = next((t for t in audio_tracks if t.is_default), None)
    if default_track:
        lang = LanguageCode.from_iso_639_2(default_track.language)
        logger.debug(f"Using default track language: {lang}")
        return lang
    
    # Priority 5: Auto-detect
    return LanguageCode.NONE
```

### 3. Subtitle Generation Module (`worker/transcription/subtitles.py`)

```python
"""Subtitle file generation (SRT and LRC)."""

import os
import logging
from typing import Optional
from pathlib import Path
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
    format: str = "srt"
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


def write_lrc(segments, output_path: str, append_footer: bool = False) -> None:
    """
    Write LRC subtitle file.
    
    Extracted from: subgen.py:1218-1225
    
    LRC format: [MM:SS.xx]Text
    
    Args:
        segments: List of transcription segments
        output_path: Path to write LRC file
        append_footer: Whether to append generation footer
    """
    try:
        # Write to temp file first (atomic write)
        temp_path = output_path + ".tmp"
        
        with open(temp_path, "w", encoding="utf-8") as f:
            for segment in segments:
                minutes, seconds = divmod(int(segment.start), 60)
                fraction = int((segment.start - int(segment.start)) * 100)
                
                # Remove embedded newlines
                text = segment.text.strip().replace('\n', ' ')
                
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
    result,  # stable_whisper result object
    output_path: str,
    word_level_highlight: bool = False,
    append_footer: bool = False
) -> None:
    """
    Write SRT subtitle file.
    
    Uses stable-whisper's to_srt_vtt() method.
    
    Args:
        result: Transcription result from stable-whisper
        output_path: Path to write SRT file
        word_level_highlight: Enable word-level timestamps
        append_footer: Whether to append generation footer
    """
    try:
        # Write to temp file
        temp_path = output_path + ".tmp"
        
        # Use stable-whisper's method
        result.to_srt_vtt(temp_path, word_level=word_level_highlight)
        
        # Append footer if requested
        if append_footer:
            with open(temp_path, "a", encoding="utf-8") as f:
                f.write("\n\nTranscribed by Subgen\n")
        
        # Atomic rename
        os.replace(temp_path, output_path)
        logger.info(f"SRT subtitle written: {output_path}")
        
    except Exception as e:
        logger.error(f"Failed to write SRT: {e}", exc_info=True)
        if os.path.exists(temp_path):
            os.remove(temp_path)
        raise SubtitleGenerationError(f"Failed to write SRT: {e}")


def append_line_to_result(result) -> None:
    """
    Append blank line to each segment text.
    
    Extracted from: subgen.py (appendLine function)
    
    Modifies result in-place.
    """
    for segment in result.segments:
        if not segment.text.endswith('\n'):
            segment.text += '\n'
```

### 4. Main Transcription Engine (`worker/transcription/engine.py`)

```python
"""Main transcription orchestration."""

import os
import logging
from typing import Optional
from dataclasses import dataclass
from language_code import LanguageCode

from .audio import (
    has_audio,
    handle_multiple_audio_tracks,
    AudioExtractionError
)
from .language import (
    detect_language_from_file,
    detect_language_from_bytes,
    choose_transcription_language,
    LanguageDetectionError
)
from .subtitles import (
    generate_subtitle_path,
    write_lrc,
    write_srt,
    append_line_to_result,
    SubtitleGenerationError
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


class TranscriptionEngine:
    """
    Core transcription engine.
    
    Extracted logic from: subgen.py:1227-1274 (gen_subtitles)
    """
    
    def __init__(self, config):
        self.config = config
        # Model manager will be injected in STORY_03
        self.model = None
    
    def transcribe(
        self,
        file_path: str,
        task_type: str,
        force_language: Optional[str],
        options: TranscribeOptions
    ) -> TranscriptionResult:
        """
        Transcribe audio/video file to subtitles.
        
        This is the refactored version of gen_subtitles() from subgen.py:1227-1274
        
        Args:
            file_path: Path to media file
            task_type: "transcribe" or "translate"
            force_language: Forced language code (ISO 639-1) or None
            options: Transcription options
            
        Returns:
            TranscriptionResult with subtitle path and metadata
        """
        import time
        start_time = time.time()
        
        try:
            # Validate file
            if not os.path.exists(file_path):
                raise FileNotFoundError(f"File not found: {file_path}")
            
            if not has_audio(file_path):
                raise AudioExtractionError(f"No valid audio in file: {file_path}")
            
            # Check if audio or video file
            file_name, file_extension = os.path.splitext(file_path)
            is_audio_file = file_extension.lower() in (
                ".mp3", ".wav", ".aac", ".flac", ".ogg", ".m4a"
            )
            
            # Load model (will be ModelManager in STORY_03)
            # For now, assume self.model is loaded
            if not self.model:
                raise RuntimeError("Model not loaded")
            
            # Handle multiple audio tracks
            data = file_path
            extracted_audio = handle_multiple_audio_tracks(
                file_path,
                force_language if force_language else None
            )
            if extracted_audio:
                data = extracted_audio.read()
            
            # Prepare transcription args
            args = {}
            
            if options.custom_regroup and options.custom_regroup.lower() != 'default':
                args['regroup'] = options.custom_regroup
            
            # Transcribe
            lang_code = force_language if force_language else None
            result = self.model.transcribe(
                data,
                language=lang_code,
                task=task_type,
                verbose=None,
                **args
            )
            
            # Append newlines to segments
            append_line_to_result(result)
            
            # Determine detected language
            detected_language = force_language
            if not detected_language:
                detected_language = LanguageCode.from_string(result.language)
            
            # Generate subtitle file
            if is_audio_file and options.lrc_for_audio:
                # Write LRC for audio files
                subtitle_path = file_name + '.lrc'
                write_lrc(
                    result.segments,
                    subtitle_path,
                    append_footer=options.append_footer
                )
            else:
                # Write SRT for video files
                subtitle_path = generate_subtitle_path(
                    file_path,
                    detected_language,
                    options.whisper_model,
                    show_subgen=options.show_subgen_in_filename,
                    show_model=options.show_model_in_filename,
                    format="srt"
                )
                write_srt(
                    result,
                    subtitle_path,
                    word_level_highlight=options.word_level_highlight,
                    append_footer=options.append_footer
                )
            
            # Calculate stats
            duration = time.time() - start_time
            
            return TranscriptionResult(
                success=True,
                subtitle_path=subtitle_path,
                detected_language=detected_language.to_iso_639_1(),
                duration_seconds=duration,
                segment_count=len(result.segments),
                transcription_time_ms=int(duration * 1000)
            )
            
        except Exception as e:
            logger.exception(f"Transcription failed: {e}")
            return TranscriptionResult(
                success=False,
                subtitle_path="",
                detected_language="",
                error_message=str(e)
            )
    
    def detect_language(
        self,
        source: str,  # file path or bytes
        sample_length: int,
        sample_offset: int
    ) -> 'LanguageDetectionResult':
        """
        Detect language from audio source.
        
        Args:
            source: File path or audio bytes
            sample_length: Sample duration in seconds
            sample_offset: Start offset in seconds
            
        Returns:
            LanguageDetectionResult
        """
        if not self.model:
            raise RuntimeError("Model not loaded")
        
        if isinstance(source, str):
            return detect_language_from_file(
                source,
                self.model,
                sample_offset=sample_offset,
                sample_length=sample_length
            )
        else:
            return detect_language_from_bytes(source, self.model)
    
    def is_model_loaded(self) -> bool:
        """Check if model is loaded."""
        return self.model is not None
```

---

## Testing Strategy

### Unit Tests (60+ tests)

**Audio Module Tests** (15 tests):
```python
# tests/unit/test_audio.py

def test_has_audio_valid_file():
    """Test has_audio with valid video file."""
    assert has_audio("/test/video.mp4") is True

def test_has_audio_no_codec():
    """Test has_audio with invalid codec."""
    # Mock av.open to return stream with codec 'none'
    assert has_audio("/test/invalid.mp4") is False

def test_get_audio_tracks():
    """Test get_audio_tracks returns correct info."""
    tracks = get_audio_tracks("/test/video.mkv")
    assert len(tracks) == 2
    assert tracks[0].codec == "aac"

def test_extract_audio_track_context_manager():
    """Test audio extraction closes resources."""
    with extract_audio_track("/test/video.mp4", 0) as audio:
        data = audio.read()
        assert len(data) > 0
    # Verify BytesIO closed

def test_handle_multiple_audio_tracks_preferred_language():
    """Test selecting audio track by language."""
    audio = handle_multiple_audio_tracks("/test/video.mkv", "jpn")
    assert audio is not None
```

**Language Module Tests** (12 tests):
```python
# tests/unit/test_language.py

def test_detect_language_from_file(mock_model):
    """Test language detection from file."""
    result = detect_language_from_file(
        "/test/audio.mp3",
        mock_model,
        sample_offset=0,
        sample_length=30
    )
    assert result.language_code == "en"

def test_choose_transcription_language_forced():
    """Test language selection with forced language."""
    lang = choose_transcription_language(
        "/test/video.mp4",
        forced_language=LanguageCode.ENGLISH,
        force_detected_language_to=None,
        preferred_audio_languages=[]
    )
    assert lang == LanguageCode.ENGLISH
```

**Subtitle Module Tests** (18 tests):
```python
# tests/unit/test_subtitles.py

def test_generate_subtitle_path_full():
    """Test subtitle path generation with all options."""
    path = generate_subtitle_path(
        "/media/movie.mkv",
        LanguageCode.ENGLISH,
        "medium",
        show_subgen=True,
        show_model=True,
        format="srt"
    )
    assert path == "/media/movie.subgen.medium.eng.srt"

def test_write_lrc_atomic():
    """Test LRC writing uses atomic rename."""
    segments = [Mock(start=0.0, text="Test")]
    write_lrc(segments, "/tmp/test.lrc", append_footer=False)
    assert os.path.exists("/tmp/test.lrc")
    # Verify temp file not present

def test_write_srt_word_level():
    """Test SRT with word-level timestamps."""
    mock_result = Mock()
    write_srt(mock_result, "/tmp/test.srt", word_level_highlight=True)
    # Verify stable-whisper called with word_level=True
```

**Engine Module Tests** (15 tests):
```python
# tests/unit/test_engine.py

def test_transcribe_video_file(mock_model):
    """Test transcribing video file."""
    engine = TranscriptionEngine(config)
    engine.model = mock_model
    
    result = engine.transcribe(
        "/test/video.mp4",
        "transcribe",
        None,
        TranscribeOptions()
    )
    
    assert result.success
    assert result.subtitle_path.endswith(".srt")

def test_transcribe_audio_file_generates_lrc(mock_model):
    """Test transcribing audio file generates LRC."""
    engine = TranscriptionEngine(config)
    engine.model = mock_model
    
    result = engine.transcribe(
        "/test/audio.mp3",
        "transcribe",
        None,
        TranscribeOptions(lrc_for_audio=True)
    )
    
    assert result.success
    assert result.subtitle_path.endswith(".lrc")

def test_transcribe_file_not_found():
    """Test transcribe with non-existent file."""
    engine = TranscriptionEngine(config)
    result = engine.transcribe("/nonexistent.mp4", "transcribe", None, TranscribeOptions())
    
    assert not result.success
    assert "not found" in result.error_message.lower()
```

### Integration Tests (5 tests)

```python
# tests/integration/test_transcription_pipeline.py

@pytest.mark.slow
def test_full_transcription_pipeline(sample_video, tiny_model):
    """Test complete transcription from video to SRT."""
    engine = TranscriptionEngine(config)
    engine.model = tiny_model
    
    result = engine.transcribe(
        sample_video,
        "transcribe",
        "en",
        TranscribeOptions()
    )
    
    assert result.success
    assert os.path.exists(result.subtitle_path)
    
    # Verify SRT content
    with open(result.subtitle_path) as f:
        content = f.read()
        assert "1\n00:00:00" in content  # SRT format
```

---

## Definition of Done

- [ ] All acceptance criteria met
- [ ] All 4 modules created with complete implementations
- [ ] No global variables (all state passed as parameters)
- [ ] Type hints throughout (mypy --strict passes)
- [ ] Docstrings for all public functions
- [ ] Unit tests passing (60+ tests)
- [ ] Integration tests passing (5+ tests)
- [ ] Code coverage > 80% for transcription module
- [ ] Legacy gen_subtitles functionality preserved
- [ ] Work log created: `docs/WORKLOGS/NNNN_YYYY-MM-DD_EPIC_02_story_02_modular_refactor.md`
- [ ] Code committed and pushed

---

## Validation Commands

```bash
# Run all tests
cd worker
pytest tests/ -v

# Run with coverage
pytest tests/ --cov=transcription --cov-report=term-missing

# Type checking
mypy transcription/ --strict

# Integration tests (slow)
pytest tests/integration/ -v -m slow

# Test specific module
pytest tests/unit/test_audio.py -v
```

---

## Dependencies

**Requires:**
- STORY_01 (gRPC Server Setup) - needs server structure

**Blocks:**
- STORY_03 (Model Lifecycle) - needs engine to integrate model manager
- STORY_04 (Memory Leaks) - needs modules to fix leaks

---

## References

- Legacy code: `subgen.py:1227-1274` (gen_subtitles)
- Legacy code: `subgen.py:1318-1350` (handle_multiple_audio_tracks)
- Legacy code: `subgen.py:1050-1098` (detect_language_task)
- Legacy code: `subgen.py:2016-2038` (has_audio)
- [02_MEMORY_MANAGEMENT.md](../../DESIGN/02_MEMORY_MANAGEMENT.md) - Context managers

---

**Created**: 2026-02-15  
**Last Updated**: 2026-02-15
