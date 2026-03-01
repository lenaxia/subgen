"""
Unit tests for transcription engine module.

Tests extracted logic from subgen.py:1227-1274 (gen_subtitles)
"""

import pytest
import os
from unittest.mock import Mock, patch, MagicMock
from pathlib import Path

# Import will fail until we implement the module
try:
    from transcription.engine import TranscriptionEngine, TranscribeOptions, TranscriptionResult

    TRANSCRIPTION_MODULE_EXISTS = True
except ImportError:
    TRANSCRIPTION_MODULE_EXISTS = False
    pytestmark = pytest.mark.skip(reason="Transcription module not yet implemented")


@pytest.fixture
def mock_config():
    """Mock configuration."""
    config = Mock()
    config.whisper_model = "medium"
    config.whisper_threads = 4
    return config


@pytest.fixture
def transcribe_options():
    """Standard transcribe options."""
    return TranscribeOptions(
        whisper_model="medium",
        whisper_threads=4,
        word_level_highlight=False,
        custom_regroup="cm_sl=84_sl=42++++++1",
        lrc_for_audio=True,
        custom_prompt="",
        append_footer=False,
        subtitle_language_name="aa",
        show_model_in_filename=True,
        show_subgen_in_filename=True,
    )


def make_mock_model(segments=None, language="English"):
    """
    Return a mock model whose transcribe() returns (generator, info) as
    faster-whisper does.  `segments` defaults to a single segment.
    """
    if segments is None:
        segments = [Mock(start=0.0, end=2.0, text="Test")]

    info = Mock()
    info.language = language

    mock_model = Mock()
    mock_model.transcribe = Mock(return_value=(iter(segments), info))
    return mock_model


@pytest.mark.skipif(
    not TRANSCRIPTION_MODULE_EXISTS, reason="Transcription module not yet implemented"
)
class TestTranscriptionEngine:
    """Test TranscriptionEngine class (from subgen.py:1227-1274)."""

    def test_engine_initialization(self, mock_config):
        """Test engine initialises correctly."""
        engine = TranscriptionEngine(mock_config)

        assert engine.config == mock_config
        assert engine.model is None

    def test_is_model_loaded_false(self, mock_config):
        """Test is_model_loaded returns False when no model."""
        engine = TranscriptionEngine(mock_config)

        assert engine.is_model_loaded() is False

    def test_is_model_loaded_true(self, mock_config):
        """Test is_model_loaded returns True when model exists."""
        engine = TranscriptionEngine(mock_config)
        engine.model = Mock()

        assert engine.is_model_loaded() is True


@pytest.mark.skipif(
    not TRANSCRIPTION_MODULE_EXISTS, reason="Transcription module not yet implemented"
)
class TestTranscribeVideoFile:
    """Test transcribing video files."""

    def test_transcribe_video_success(self, mock_config, transcribe_options):
        """Test successful video transcription returns success result."""
        segments = [Mock(start=0.0, end=2.0, text="Test")]

        with (
            patch("transcription.engine.has_audio", return_value=True),
            patch("transcription.engine.handle_multiple_audio_tracks", return_value=None),
            patch("transcription.engine.write_srt", return_value=1),
            patch(
                "transcription.engine.generate_subtitle_path", return_value="/test/video.eng.srt"
            ),
            patch("os.path.exists", return_value=True),
            patch("os.path.splitext", return_value=("/test/video", ".mp4")),
        ):
            engine = TranscriptionEngine(mock_config)
            engine.model = make_mock_model(segments)

            with patch("transcription.engine.LanguageCode") as mock_lang_code:
                mock_lang = Mock()
                mock_lang.to_iso_639_1 = Mock(return_value="en")
                mock_lang_code.from_string = Mock(return_value=mock_lang)

                result = engine.transcribe(
                    "/test/video.mp4", "transcribe", None, transcribe_options
                )

        assert result.success is True
        assert result.subtitle_path == "/test/video.eng.srt"
        assert result.detected_language == "en"
        assert result.segment_count == 1

    def test_transcribe_video_model_returns_tuple(self, mock_config, transcribe_options):
        """Engine unpacks (generator, info) from model.transcribe() correctly."""
        with (
            patch("transcription.engine.has_audio", return_value=True),
            patch("transcription.engine.handle_multiple_audio_tracks", return_value=None),
            patch("transcription.engine.write_srt", return_value=0),
            patch("transcription.engine.generate_subtitle_path", return_value="/test/v.srt"),
            patch("os.path.exists", return_value=True),
            patch("os.path.splitext", return_value=("/test/v", ".mp4")),
        ):
            engine = TranscriptionEngine(mock_config)
            engine.model = make_mock_model([], language="Japanese")

            with patch("transcription.engine.LanguageCode") as mock_lang_code:
                mock_lang = Mock()
                mock_lang.to_iso_639_1 = Mock(return_value="ja")
                mock_lang_code.from_string = Mock(return_value=mock_lang)

                result = engine.transcribe("/test/v.mp4", "transcribe", None, transcribe_options)

        assert result.success is True
        # Language comes from info.language, not from a segments list
        mock_lang_code.from_string.assert_called_once_with("Japanese")

    def test_transcribe_video_file_not_found(self, mock_config, transcribe_options):
        """Test transcription with non-existent file."""
        with patch("os.path.exists", return_value=False):
            engine = TranscriptionEngine(mock_config)
            engine.model = Mock()

            result = engine.transcribe(
                "/nonexistent/video.mp4", "transcribe", None, transcribe_options
            )

            assert result.success is False
            assert "not found" in result.error_message.lower()

    def test_transcribe_video_no_audio(self, mock_config, transcribe_options):
        """Test transcription with file that has no audio."""
        with (
            patch("os.path.exists", return_value=True),
            patch("transcription.engine.has_audio", return_value=False),
        ):
            engine = TranscriptionEngine(mock_config)
            engine.model = Mock()

            result = engine.transcribe("/test/noaudio.mp4", "transcribe", None, transcribe_options)

            assert result.success is False
            assert "audio" in result.error_message.lower()

    def test_transcribe_video_model_not_loaded(self, mock_config, transcribe_options):
        """Test transcription fails when model not loaded."""
        with (
            patch("os.path.exists", return_value=True),
            patch("transcription.engine.has_audio", return_value=True),
        ):
            engine = TranscriptionEngine(mock_config)
            # No model loaded

            result = engine.transcribe("/test/video.mp4", "transcribe", None, transcribe_options)

            assert result.success is False
            assert "model not loaded" in result.error_message.lower()

    def test_transcribe_video_forced_language(self, mock_config, transcribe_options):
        """Test transcription with forced language passes it to model."""
        with (
            patch("transcription.engine.has_audio", return_value=True),
            patch("transcription.engine.handle_multiple_audio_tracks", return_value=None),
            patch("transcription.engine.write_srt", return_value=0),
            patch(
                "transcription.engine.generate_subtitle_path", return_value="/test/video.jpn.srt"
            ),
            patch("os.path.exists", return_value=True),
            patch("os.path.splitext", return_value=("/test/video", ".mp4")),
        ):
            engine = TranscriptionEngine(mock_config)
            mock_model = make_mock_model([], language="Japanese")
            engine.model = mock_model

            engine.transcribe(
                "/test/video.mp4",
                "transcribe",
                "ja",  # Forced language
                transcribe_options,
            )

            call_args = mock_model.transcribe.call_args
            assert call_args[1]["language"] == "ja"

    def test_transcribe_passes_generator_to_write_srt(self, mock_config, transcribe_options):
        """Engine passes the segments generator directly to write_srt without list()."""
        captured = {}

        def capture_write_srt(segments, *args, **kwargs):
            captured["segments"] = segments
            return 2

        with (
            patch("transcription.engine.has_audio", return_value=True),
            patch("transcription.engine.handle_multiple_audio_tracks", return_value=None),
            patch("transcription.engine.write_srt", side_effect=capture_write_srt),
            patch("transcription.engine.generate_subtitle_path", return_value="/t.srt"),
            patch("os.path.exists", return_value=True),
            patch("os.path.splitext", return_value=("/t", ".mp4")),
        ):
            engine = TranscriptionEngine(mock_config)
            engine.model = make_mock_model()

            with patch("transcription.engine.LanguageCode"):
                engine.transcribe("/t.mp4", "transcribe", None, transcribe_options)

        # The object passed to write_srt must be a generator/iterator, not a list
        import types

        assert not isinstance(captured["segments"], list), (
            "segments must not be materialised as a list before passing to write_srt"
        )
        assert hasattr(captured["segments"], "__iter__"), "segments must be iterable"

    def test_transcribe_video_multiple_audio_tracks_appends_temp(
        self, mock_config, transcribe_options
    ):
        """Multi-track extraction appends to temp_files, not reassigns."""
        import tempfile

        extracted_audio = Mock()
        extracted_audio.read = Mock(return_value=b"audio_data")
        extracted_audio.close = Mock()

        created_temps = []

        real_named_temp = tempfile.NamedTemporaryFile

        def patched_named_temp(*args, **kwargs):
            tf = real_named_temp(*args, **kwargs)
            created_temps.append(tf.name)
            return tf

        with (
            patch("transcription.engine.has_audio", return_value=True),
            patch(
                "transcription.engine.handle_multiple_audio_tracks", return_value=extracted_audio
            ),
            patch("transcription.engine.write_srt", return_value=0),
            patch("transcription.engine.generate_subtitle_path", return_value="/test/video.srt"),
            patch("os.path.exists", return_value=True),
            patch("os.path.splitext", return_value=("/test/video", ".mkv")),
            patch("tempfile.NamedTemporaryFile", side_effect=patched_named_temp),
        ):
            engine = TranscriptionEngine(mock_config)
            engine.model = make_mock_model()

            with patch("transcription.engine.LanguageCode"):
                engine.transcribe("/test/video.mkv", "transcribe", None, transcribe_options)

        # Both temp files should have been created and cleaned up (not orphaned)
        # The test verifies no NamedTemporaryFile path is left on disk
        for p in created_temps:
            assert not os.path.exists(p), f"Temp file was not cleaned up: {p}"


@pytest.mark.skipif(
    not TRANSCRIPTION_MODULE_EXISTS, reason="Transcription module not yet implemented"
)
class TestTranscribeAudioFile:
    """Test transcribing audio files (generates LRC)."""

    def test_transcribe_audio_generates_lrc(self, mock_config, transcribe_options):
        """Test audio file generates LRC file."""
        with (
            patch("transcription.engine.has_audio", return_value=True),
            patch("transcription.engine.handle_multiple_audio_tracks", return_value=None),
            patch("transcription.engine.write_lrc", return_value=0),
            patch("os.path.exists", return_value=True),
            patch("os.path.splitext", return_value=("/test/audio", ".mp3")),
        ):
            engine = TranscriptionEngine(mock_config)
            engine.model = make_mock_model(language="English")

            with patch("transcription.engine.LanguageCode") as mock_lang_code:
                mock_lang = Mock()
                mock_lang.to_iso_639_1 = Mock(return_value="en")
                mock_lang_code.from_string = Mock(return_value=mock_lang)

                result = engine.transcribe(
                    "/test/audio.mp3", "transcribe", None, transcribe_options
                )

        assert result.success is True
        assert result.subtitle_path == "/test/audio.lrc"

    def test_transcribe_audio_various_extensions(self, mock_config, transcribe_options):
        """Test various audio file extensions produce LRC output."""
        audio_extensions = [".mp3", ".wav", ".aac", ".flac", ".ogg", ".m4a"]

        for ext in audio_extensions:
            with (
                patch("transcription.engine.has_audio", return_value=True),
                patch("transcription.engine.handle_multiple_audio_tracks", return_value=None),
                patch("transcription.engine.write_lrc", return_value=0),
                patch("os.path.exists", return_value=True),
                patch("os.path.splitext", return_value=(f"/test/audio", ext)),
            ):
                engine = TranscriptionEngine(mock_config)
                engine.model = make_mock_model()

                with patch("transcription.engine.LanguageCode") as mock_lang_code:
                    mock_lang = Mock()
                    mock_lang.to_iso_639_1 = Mock(return_value="en")
                    mock_lang_code.from_string = Mock(return_value=mock_lang)

                    result = engine.transcribe(
                        f"/test/audio{ext}", "transcribe", None, transcribe_options
                    )

            assert result.subtitle_path == "/test/audio.lrc", f"Failed for extension {ext}"

    def test_transcribe_audio_passes_generator_to_write_lrc(self, mock_config, transcribe_options):
        """Engine passes the segments generator to write_lrc without list()."""
        captured = {}

        def capture_write_lrc(segments, *args, **kwargs):
            captured["segments"] = segments
            return 3

        with (
            patch("transcription.engine.has_audio", return_value=True),
            patch("transcription.engine.handle_multiple_audio_tracks", return_value=None),
            patch("transcription.engine.write_lrc", side_effect=capture_write_lrc),
            patch("os.path.exists", return_value=True),
            patch("os.path.splitext", return_value=("/test/audio", ".mp3")),
        ):
            engine = TranscriptionEngine(mock_config)
            engine.model = make_mock_model()

            with patch("transcription.engine.LanguageCode"):
                engine.transcribe("/test/audio.mp3", "transcribe", None, transcribe_options)

        import types

        assert not isinstance(captured["segments"], list), (
            "segments must not be materialised as a list before passing to write_lrc"
        )


@pytest.mark.skipif(
    not TRANSCRIPTION_MODULE_EXISTS, reason="Transcription module not yet implemented"
)
class TestTranscribeOptions:
    """Test TranscribeOptions usage in transcription."""

    def test_transcribe_with_word_level_highlight(self, mock_config, transcribe_options):
        """Test word-level highlight option is forwarded to write_srt."""
        transcribe_options.word_level_highlight = True

        with (
            patch("transcription.engine.has_audio", return_value=True),
            patch("transcription.engine.handle_multiple_audio_tracks", return_value=None),
            patch("transcription.engine.write_srt", return_value=0) as mock_write_srt,
            patch("transcription.engine.generate_subtitle_path", return_value="/test/video.srt"),
            patch("os.path.exists", return_value=True),
            patch("os.path.splitext", return_value=("/test/video", ".mp4")),
        ):
            engine = TranscriptionEngine(mock_config)
            engine.model = make_mock_model()

            with patch("transcription.engine.LanguageCode") as mock_lang_code:
                mock_lang = Mock()
                mock_lang.to_iso_639_1 = Mock(return_value="en")
                mock_lang_code.from_string = Mock(return_value=mock_lang)

                engine.transcribe("/test/video.mp4", "transcribe", None, transcribe_options)

        call_args = mock_write_srt.call_args
        assert call_args[1]["word_level_highlight"] is True


@pytest.mark.skipif(
    not TRANSCRIPTION_MODULE_EXISTS, reason="Transcription module not yet implemented"
)
class TestDetectLanguage:
    """Test detect_language method."""

    def test_detect_language_from_file_path(self, mock_config):
        """Test language detection from file path."""
        with patch("transcription.engine.detect_language_from_file") as mock_detect:
            from language.detector import LanguageDetectionResult

            mock_detect.return_value = LanguageDetectionResult(
                language_code="fr", language_name="French", confidence=1.0
            )

            engine = TranscriptionEngine(mock_config)
            engine.model = Mock()

            result = engine.detect_language("/test/video.mp4", sample_length=30, sample_offset=0)

            assert result.language_code == "fr"
            assert result.language_name == "French"
            mock_detect.assert_called_once()

    def test_detect_language_from_bytes(self, mock_config):
        """Test language detection from audio bytes."""
        with patch("transcription.engine.detect_language_from_bytes") as mock_detect:
            from language.detector import LanguageDetectionResult

            mock_detect.return_value = LanguageDetectionResult(
                language_code="de", language_name="German", confidence=1.0
            )

            engine = TranscriptionEngine(mock_config)
            engine.model = Mock()

            audio_bytes = b"fake_audio_data"
            result = engine.detect_language(audio_bytes, sample_length=30, sample_offset=0)

            assert result.language_code == "de"
            mock_detect.assert_called_once_with(audio_bytes, engine.model)

    def test_detect_language_model_not_loaded(self, mock_config):
        """Test language detection fails when model not loaded."""
        engine = TranscriptionEngine(mock_config)
        # No model loaded

        with pytest.raises(RuntimeError, match="Model not loaded"):
            engine.detect_language("/test/video.mp4", sample_length=30, sample_offset=0)


@pytest.mark.skipif(
    not TRANSCRIPTION_MODULE_EXISTS, reason="Transcription module not yet implemented"
)
class TestTranscriptionResult:
    """Test TranscriptionResult dataclass."""

    def test_transcription_result_success(self):
        """Test successful transcription result."""
        result = TranscriptionResult(
            success=True,
            subtitle_path="/test/video.srt",
            detected_language="en",
            error_message=None,
            duration_seconds=45.2,
            segment_count=10,
            transcription_time_ms=45200,
            peak_memory_mb=512,
        )

        assert result.success is True
        assert result.subtitle_path == "/test/video.srt"
        assert result.detected_language == "en"
        assert result.segment_count == 10

    def test_transcription_result_failure(self):
        """Test failed transcription result."""
        result = TranscriptionResult(
            success=False, subtitle_path="", detected_language="", error_message="File not found"
        )

        assert result.success is False
        assert result.error_message == "File not found"

    def test_transcription_result_segments_defaults_to_none(self):
        """For the file-based path, segments defaults to None (not materialised)."""
        result = TranscriptionResult(success=True, subtitle_path="/t.srt", detected_language="en")
        assert result.segments is None, (
            "segments must be None by default — only populated when return_segments=True (ASR path)"
        )

    def test_transcription_result_segments_populated_for_asr_path(self):
        """For the ASR/bytes path (return_segments=True), segments is a list."""
        seg = object()
        result = TranscriptionResult(
            success=True,
            subtitle_path="/t.srt",
            detected_language="en",
            segments=[seg],
        )
        assert isinstance(result.segments, list)
        assert result.segments[0] is seg
