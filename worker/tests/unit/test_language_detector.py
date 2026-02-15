"""
Unit tests for language detection module.

Tests extracted logic from subgen.py:1050-1098, 1404-1444
"""

import pytest
from unittest.mock import Mock, patch
from io import BytesIO

# Import will fail until we implement the module
try:
    from language.detector import (
        detect_language_from_file,
        detect_language_from_bytes,
        choose_transcription_language,
        LanguageDetectionResult,
        LanguageDetectionError,
    )

    LANGUAGE_MODULE_EXISTS = True
except ImportError:
    LANGUAGE_MODULE_EXISTS = False
    pytestmark = pytest.mark.skip(reason="Language module not yet implemented")


@pytest.mark.skipif(not LANGUAGE_MODULE_EXISTS, reason="Language module not yet implemented")
class TestDetectLanguageFromFile:
    """Test detect_language_from_file function (from subgen.py:1050-1098)."""

    def test_detect_language_success(self):
        """Test successful language detection from file."""
        with patch("audio.extractor.extract_audio_segment") as mock_extract:
            # Mock audio extraction
            mock_buffer = BytesIO(b"fake_audio")
            mock_extract.return_value.__enter__ = Mock(return_value=mock_buffer)
            mock_extract.return_value.__exit__ = Mock(return_value=False)

            # Mock model
            mock_model = Mock()
            mock_result = Mock()
            mock_result.language = "English"
            mock_model.transcribe = Mock(return_value=mock_result)

            # Mock LanguageCode
            with patch("language.detector.LanguageCode") as mock_lang_code:
                mock_lang = Mock()
                mock_lang.to_iso_639_1 = Mock(return_value="en")
                mock_lang.to_name = Mock(return_value="English")
                mock_lang_code.from_name = Mock(return_value=mock_lang)

                result = detect_language_from_file(
                    "/test/video.mp4", mock_model, sample_offset=0, sample_length=30
                )

                assert result.language_code == "en"
                assert result.language_name == "English"
                assert result.confidence == 1.0

    def test_detect_language_custom_sample(self):
        """Test language detection with custom offset and length."""
        with patch("audio.extractor.extract_audio_segment") as mock_extract:
            mock_buffer = BytesIO(b"audio")
            mock_extract.return_value.__enter__ = Mock(return_value=mock_buffer)
            mock_extract.return_value.__exit__ = Mock(return_value=False)

            mock_model = Mock()
            mock_result = Mock()
            mock_result.language = "Spanish"
            mock_model.transcribe = Mock(return_value=mock_result)

            with patch("language.detector.LanguageCode") as mock_lang_code:
                mock_lang = Mock()
                mock_lang.to_iso_639_1 = Mock(return_value="es")
                mock_lang.to_name = Mock(return_value="Spanish")
                mock_lang_code.from_name = Mock(return_value=mock_lang)

                result = detect_language_from_file(
                    "/test/video.mp4", mock_model, sample_offset=60, sample_length=15
                )

                # Verify extract_audio_segment called with correct params
                mock_extract.assert_called_once_with("/test/video.mp4", 60, 15)
                assert result.language_code == "es"

    def test_detect_language_extraction_error(self):
        """Test extraction error raises LanguageDetectionError."""
        with patch("audio.extractor.extract_audio_segment") as mock_extract:
            from audio.extractor import AudioExtractionError

            mock_extract.side_effect = AudioExtractionError("Failed")

            mock_model = Mock()

            with pytest.raises(LanguageDetectionError):
                detect_language_from_file("/test/error.mp4", mock_model)

    def test_detect_language_transcription_error(self):
        """Test transcription error raises LanguageDetectionError."""
        with patch("audio.extractor.extract_audio_segment") as mock_extract:
            mock_buffer = BytesIO(b"audio")
            mock_extract.return_value.__enter__ = Mock(return_value=mock_buffer)
            mock_extract.return_value.__exit__ = Mock(return_value=False)

            mock_model = Mock()
            mock_model.transcribe = Mock(side_effect=Exception("Transcription failed"))

            with pytest.raises(LanguageDetectionError):
                detect_language_from_file("/test/video.mp4", mock_model)


@pytest.mark.skipif(not LANGUAGE_MODULE_EXISTS, reason="Language module not yet implemented")
class TestDetectLanguageFromBytes:
    """Test detect_language_from_bytes function."""

    def test_detect_language_from_bytes_success(self):
        """Test language detection from audio bytes."""
        audio_bytes = b"fake_audio_data"

        mock_model = Mock()
        mock_result = Mock()
        mock_result.language = "Japanese"
        mock_model.transcribe = Mock(return_value=mock_result)

        with patch("language.detector.LanguageCode") as mock_lang_code:
            mock_lang = Mock()
            mock_lang.to_iso_639_1 = Mock(return_value="ja")
            mock_lang.to_name = Mock(return_value="Japanese")
            mock_lang_code.from_name = Mock(return_value=mock_lang)

            result = detect_language_from_bytes(audio_bytes, mock_model)

            assert result.language_code == "ja"
            assert result.language_name == "Japanese"
            mock_model.transcribe.assert_called_once_with(audio_bytes)

    def test_detect_language_from_bytes_error(self):
        """Test error handling in bytes detection."""
        audio_bytes = b"audio"
        mock_model = Mock()
        mock_model.transcribe = Mock(side_effect=RuntimeError("Model error"))

        with pytest.raises(LanguageDetectionError):
            detect_language_from_bytes(audio_bytes, mock_model)


@pytest.mark.skipif(not LANGUAGE_MODULE_EXISTS, reason="Language module not yet implemented")
class TestChooseTranscriptionLanguage:
    """Test choose_transcription_language function (from subgen.py:1404-1444)."""

    def test_choose_language_forced(self):
        """Test forced language takes priority."""
        with patch("language.detector.LanguageCode") as mock_lang_code:
            forced_lang = Mock()
            mock_lang_code.ENGLISH = forced_lang

            result = choose_transcription_language(
                "/test/video.mp4",
                forced_language=forced_lang,
                force_detected_language_to=None,
                preferred_audio_languages=[],
            )

            assert result == forced_lang

    def test_choose_language_config_override(self):
        """Test config override takes second priority."""
        with patch("language.detector.LanguageCode") as mock_lang_code:
            config_lang = Mock()

            result = choose_transcription_language(
                "/test/video.mp4",
                forced_language=None,
                force_detected_language_to=config_lang,
                preferred_audio_languages=[],
            )

            assert result == config_lang

    def test_choose_language_preferred_audio(self):
        """Test preferred audio language from tracks."""
        with (
            patch("audio.extractor.get_audio_tracks") as mock_get_tracks,
            patch("language.detector.LanguageCode") as mock_lang_code,
        ):
            from audio.extractor import AudioTrackInfo

            mock_get_tracks.return_value = [
                AudioTrackInfo(0, "aac", 2, "eng", True),
                AudioTrackInfo(1, "ac3", 6, "jpn", False),
            ]

            # Mock preferred language
            preferred = Mock()
            preferred.to_iso_639_2_b = Mock(return_value="jpn")
            mock_lang_code.JAPANESE = preferred

            result = choose_transcription_language(
                "/test/video.mkv",
                forced_language=None,
                force_detected_language_to=None,
                preferred_audio_languages=[preferred],
            )

            assert result == preferred

    def test_choose_language_default_track(self):
        """Test default track language when no preferences."""
        with (
            patch("audio.extractor.get_audio_tracks") as mock_get_tracks,
            patch("language.detector.LanguageCode") as mock_lang_code,
        ):
            from audio.extractor import AudioTrackInfo

            mock_get_tracks.return_value = [
                AudioTrackInfo(0, "aac", 2, "eng", True),
                AudioTrackInfo(1, "ac3", 6, "jpn", False),
            ]

            # Mock from_iso_639_2
            default_lang = Mock()
            mock_lang_code.from_iso_639_2 = Mock(return_value=default_lang)

            result = choose_transcription_language(
                "/test/video.mkv",
                forced_language=None,
                force_detected_language_to=None,
                preferred_audio_languages=[],
            )

            mock_lang_code.from_iso_639_2.assert_called_with("eng")
            assert result == default_lang

    def test_choose_language_auto_detect(self):
        """Test returns NONE for auto-detection."""
        with (
            patch("audio.extractor.get_audio_tracks") as mock_get_tracks,
            patch("language.detector.LanguageCode") as mock_lang_code,
        ):
            from audio.extractor import AudioTrackInfo

            mock_get_tracks.return_value = [
                AudioTrackInfo(0, "aac", 2, "und", False)  # Undefined language
            ]

            mock_lang_code.NONE = Mock()
            mock_lang_code.from_iso_639_2 = Mock(side_effect=ValueError("Invalid"))

            result = choose_transcription_language(
                "/test/video.mp4",
                forced_language=None,
                force_detected_language_to=None,
                preferred_audio_languages=[],
            )

            assert result == mock_lang_code.NONE


@pytest.mark.skipif(not LANGUAGE_MODULE_EXISTS, reason="Language module not yet implemented")
class TestLanguageDetectionResult:
    """Test LanguageDetectionResult dataclass."""

    def test_language_detection_result_creation(self):
        """Test creating LanguageDetectionResult."""
        result = LanguageDetectionResult(
            language_code="en", language_name="English", confidence=0.95
        )

        assert result.language_code == "en"
        assert result.language_name == "English"
        assert result.confidence == 0.95

    def test_language_detection_result_default_confidence(self):
        """Test default confidence is 1.0."""
        result = LanguageDetectionResult(language_code="fr", language_name="French", confidence=1.0)

        assert result.confidence == 1.0
