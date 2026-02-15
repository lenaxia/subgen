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


@pytest.mark.skipif(
    not TRANSCRIPTION_MODULE_EXISTS, reason="Transcription module not yet implemented"
)
class TestTranscriptionEngine:
    """Test TranscriptionEngine class (from subgen.py:1227-1274)."""

    def test_engine_initialization(self, mock_config):
        """Test engine initializes correctly."""
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
        """Test successful video transcription."""
        with (
            patch("transcription.engine.has_audio", return_value=True),
            patch("transcription.engine.handle_multiple_audio_tracks", return_value=None),
            patch("transcription.engine.append_line_to_result"),
            patch("transcription.engine.write_srt"),
            patch(
                "transcription.engine.generate_subtitle_path", return_value="/test/video.eng.srt"
            ),
            patch("os.path.exists", return_value=True),
            patch("os.path.splitext", return_value=("/test/video", ".mp4")),
        ):
            engine = TranscriptionEngine(mock_config)

            # Mock model and result
            mock_model = Mock()
            mock_result = Mock()
            mock_result.language = "English"
            mock_result.segments = [Mock(start=0.0, end=2.0, text="Test")]
            mock_model.transcribe = Mock(return_value=mock_result)
            engine.model = mock_model

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
        """Test transcription with forced language."""
        with (
            patch("transcription.engine.has_audio", return_value=True),
            patch("transcription.engine.handle_multiple_audio_tracks", return_value=None),
            patch("transcription.engine.append_line_to_result"),
            patch("transcription.engine.write_srt"),
            patch(
                "transcription.engine.generate_subtitle_path", return_value="/test/video.jpn.srt"
            ),
            patch("os.path.exists", return_value=True),
            patch("os.path.splitext", return_value=("/test/video", ".mp4")),
        ):
            engine = TranscriptionEngine(mock_config)
            mock_model = Mock()
            mock_result = Mock()
            mock_result.language = "Japanese"
            mock_result.segments = []
            mock_model.transcribe = Mock(return_value=mock_result)
            engine.model = mock_model

            result = engine.transcribe(
                "/test/video.mp4",
                "transcribe",
                "ja",  # Forced language
                transcribe_options,
            )

            # Verify transcribe called with forced language
            call_args = mock_model.transcribe.call_args
            assert call_args[1]["language"] == "ja"

    def test_transcribe_video_multiple_audio_tracks(self, mock_config, transcribe_options):
        """Test video with multiple audio tracks."""
        fake_audio = b"extracted_audio_data"

        with (
            patch("transcription.engine.has_audio", return_value=True),
            patch(
                "transcription.engine.handle_multiple_audio_tracks",
                return_value=Mock(read=Mock(return_value=fake_audio)),
            ),
            patch("transcription.engine.append_line_to_result"),
            patch("transcription.engine.write_srt"),
            patch("transcription.engine.generate_subtitle_path", return_value="/test/video.srt"),
            patch("os.path.exists", return_value=True),
            patch("os.path.splitext", return_value=("/test/video", ".mkv")),
        ):
            engine = TranscriptionEngine(mock_config)
            mock_model = Mock()
            mock_result = Mock()
            mock_result.language = "English"
            mock_result.segments = []
            mock_model.transcribe = Mock(return_value=mock_result)
            engine.model = mock_model

            with patch("transcription.engine.LanguageCode") as mock_lang_code:
                mock_lang = Mock()
                mock_lang.to_iso_639_1 = Mock(return_value="en")
                mock_lang_code.from_string = Mock(return_value=mock_lang)

                result = engine.transcribe(
                    "/test/video.mkv", "transcribe", None, transcribe_options
                )

            # Verify transcribed extracted audio, not file path
            call_args = mock_model.transcribe.call_args
            assert call_args[0][0] == fake_audio


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
            patch("transcription.engine.append_line_to_result"),
            patch("transcription.engine.write_lrc"),
            patch("os.path.exists", return_value=True),
            patch("os.path.splitext", return_value=("/test/audio", ".mp3")),
        ):
            engine = TranscriptionEngine(mock_config)
            mock_model = Mock()
            mock_result = Mock()
            mock_result.language = "English"
            mock_result.segments = []
            mock_model.transcribe = Mock(return_value=mock_result)
            engine.model = mock_model

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
        """Test various audio file extensions generate LRC."""
        audio_extensions = [".mp3", ".wav", ".aac", ".flac", ".ogg", ".m4a"]

        for ext in audio_extensions:
            with (
                patch("transcription.engine.has_audio", return_value=True),
                patch("transcription.engine.handle_multiple_audio_tracks", return_value=None),
                patch("transcription.engine.append_line_to_result"),
                patch("transcription.engine.write_lrc"),
                patch("os.path.exists", return_value=True),
                patch("os.path.splitext", return_value=(f"/test/audio", ext)),
            ):
                engine = TranscriptionEngine(mock_config)
                mock_model = Mock()
                mock_result = Mock()
                mock_result.language = "English"
                mock_result.segments = []
                mock_model.transcribe = Mock(return_value=mock_result)
                engine.model = mock_model

                with patch("transcription.engine.LanguageCode") as mock_lang_code:
                    mock_lang = Mock()
                    mock_lang.to_iso_639_1 = Mock(return_value="en")
                    mock_lang_code.from_string = Mock(return_value=mock_lang)

                    result = engine.transcribe(
                        f"/test/audio{ext}", "transcribe", None, transcribe_options
                    )

                assert result.subtitle_path == "/test/audio.lrc"


@pytest.mark.skipif(
    not TRANSCRIPTION_MODULE_EXISTS, reason="Transcription module not yet implemented"
)
class TestTranscribeOptions:
    """Test TranscribeOptions usage in transcription."""

    def test_transcribe_with_custom_regroup(self, mock_config, transcribe_options):
        """Test custom regroup parameter passed to model."""
        transcribe_options.custom_regroup = "custom_algorithm"

        with (
            patch("transcription.engine.has_audio", return_value=True),
            patch("transcription.engine.handle_multiple_audio_tracks", return_value=None),
            patch("transcription.engine.append_line_to_result"),
            patch("transcription.engine.write_srt"),
            patch("transcription.engine.generate_subtitle_path", return_value="/test/video.srt"),
            patch("os.path.exists", return_value=True),
            patch("os.path.splitext", return_value=("/test/video", ".mp4")),
        ):
            engine = TranscriptionEngine(mock_config)
            mock_model = Mock()
            mock_result = Mock()
            mock_result.language = "English"
            mock_result.segments = []
            mock_model.transcribe = Mock(return_value=mock_result)
            engine.model = mock_model

            with patch("transcription.engine.LanguageCode") as mock_lang_code:
                mock_lang = Mock()
                mock_lang.to_iso_639_1 = Mock(return_value="en")
                mock_lang_code.from_string = Mock(return_value=mock_lang)

                result = engine.transcribe(
                    "/test/video.mp4", "transcribe", None, transcribe_options
                )

            # Verify regroup parameter passed
            call_args = mock_model.transcribe.call_args
            assert "regroup" in call_args[1]
            assert call_args[1]["regroup"] == "custom_algorithm"

    def test_transcribe_with_word_level_highlight(self, mock_config, transcribe_options):
        """Test word-level highlight option."""
        transcribe_options.word_level_highlight = True

        with (
            patch("transcription.engine.has_audio", return_value=True),
            patch("transcription.engine.handle_multiple_audio_tracks", return_value=None),
            patch("transcription.engine.append_line_to_result"),
            patch("transcription.engine.write_srt") as mock_write_srt,
            patch("transcription.engine.generate_subtitle_path", return_value="/test/video.srt"),
            patch("os.path.exists", return_value=True),
            patch("os.path.splitext", return_value=("/test/video", ".mp4")),
        ):
            engine = TranscriptionEngine(mock_config)
            mock_model = Mock()
            mock_result = Mock()
            mock_result.language = "English"
            mock_result.segments = []
            mock_model.transcribe = Mock(return_value=mock_result)
            engine.model = mock_model

            with patch("transcription.engine.LanguageCode") as mock_lang_code:
                mock_lang = Mock()
                mock_lang.to_iso_639_1 = Mock(return_value="en")
                mock_lang_code.from_string = Mock(return_value=mock_lang)

                result = engine.transcribe(
                    "/test/video.mp4", "transcribe", None, transcribe_options
                )

            # Verify write_srt called with word_level_highlight=True
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
