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

            # Mock model - transcribe() returns (segments_generator, info)
            mock_model = Mock()
            mock_info = Mock()
            mock_info.language = "en"  # faster-whisper uses ISO codes
            mock_info.language_probability = 1.0
            mock_segments = []  # Empty generator
            mock_model.transcribe = Mock(return_value=(mock_segments, mock_info))

            result = detect_language_from_file(
                "/test/video.mp4", mock_model, sample_offset=0, sample_length=30
            )

            assert result.language_code == "en"
            assert result.confidence > 0.0

    def test_detect_language_custom_sample(self):
        """Test language detection with custom offset and length."""
        with patch("audio.extractor.extract_audio_segment") as mock_extract:
            mock_buffer = BytesIO(b"audio")
            mock_extract.return_value.__enter__ = Mock(return_value=mock_buffer)
            mock_extract.return_value.__exit__ = Mock(return_value=False)

            # Mock model - transcribe() returns (segments_generator, info)
            mock_model = Mock()
            mock_info = Mock()
            mock_info.language = "es"  # Spanish ISO code
            mock_info.language_probability = 0.95
            mock_segments = []
            mock_model.transcribe = Mock(return_value=(mock_segments, mock_info))

            result = detect_language_from_file(
                "/test/video.mp4", mock_model, sample_offset=60, sample_length=15
            )

            assert result.language_code == "es"
            assert result.confidence > 0.0

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

    def test_detect_language_samples_segment_not_full_file(self):
        """Regression: detection must decode only the requested sample.

        LEGACY BUG: detect_language_from_file ignored sample_offset and
        sample_length and called model.transcribe(file_path) on the FULL
        file, then consumed the entire segments generator — transcribing a
        whole 2h movie just to read a language code.
        """
        import os

        with patch("audio.extractor.extract_audio_segment") as mock_extract:
            mock_buffer = BytesIO(b"sample_audio")
            mock_extract.return_value.__enter__ = Mock(return_value=mock_buffer)
            mock_extract.return_value.__exit__ = Mock(return_value=False)

            consumed = []

            def infinite_segments(*args, **kwargs):
                # A generator that records consumption; detection must not
                # iterate it (language is available from info immediately).
                i = 0
                while True:
                    consumed.append(i)
                    yield Mock(start=float(i), end=float(i + 1), text="x")
                    i += 1

            mock_info = Mock()
            mock_info.language = "en"
            mock_info.language_probability = 0.9
            mock_model = Mock()
            mock_model.transcribe = Mock(return_value=(infinite_segments(), mock_info))

            result = detect_language_from_file(
                "/test/movie.mkv", mock_model, sample_offset=0, sample_length=30
            )

            assert result.language_code == "en"

            # 1. The extraction must be a 30s sample from offset 0
            mock_extract.assert_called_once_with("/test/movie.mkv", 0, 30)

            # 2. The model must NOT be given the full file path
            (transcribe_arg,), _ = mock_model.transcribe.call_args
            assert transcribe_arg != "/test/movie.mkv"

            # 3. The segments generator must not be consumed
            assert consumed == [], "detection decoded audio segments unnecessarily"

    def test_detect_language_cleans_up_temp_file(self):
        """The sampled temp file must be removed after detection."""
        import os
        import tempfile

        created = []

        real_named_temp = tempfile.NamedTemporaryFile

        def track_temp(*args, **kwargs):
            tf = real_named_temp(*args, **kwargs)
            created.append(tf.name)
            return tf

        with (
            patch("audio.extractor.extract_audio_segment") as mock_extract,
            patch("tempfile.NamedTemporaryFile", side_effect=track_temp),
        ):
            mock_buffer = BytesIO(b"sample_audio")
            mock_extract.return_value.__enter__ = Mock(return_value=mock_buffer)
            mock_extract.return_value.__exit__ = Mock(return_value=False)

            mock_info = Mock()
            mock_info.language = "en"
            mock_info.language_probability = 0.9
            mock_model = Mock()
            mock_model.transcribe = Mock(return_value=(iter([]), mock_info))

            detect_language_from_file("/test/movie.mkv", mock_model)

        for path in created:
            assert not os.path.exists(path), f"temp file leaked: {path}"


@pytest.mark.skipif(not LANGUAGE_MODULE_EXISTS, reason="Language module not yet implemented")
class TestDetectLanguageFromBytes:
    """Test detect_language_from_bytes function."""

    def test_detect_language_from_bytes_success(self):
        """Test language detection from audio bytes."""
        audio_bytes = b"fake_audio_data"

        mock_model = Mock()
        mock_info = Mock()
        mock_info.language = "ja"
        mock_info.language_probability = 0.95
        mock_model.transcribe = Mock(return_value=(iter([]), mock_info))

        with patch("language.detector.LanguageCode") as mock_lang_code:
            mock_lang = Mock()
            mock_lang.to_iso_639_1 = Mock(return_value="ja")
            mock_lang.to_name = Mock(return_value="Japanese")
            mock_lang_code.from_iso_639_1 = Mock(return_value=mock_lang)

            result = detect_language_from_bytes(audio_bytes, mock_model)

            assert result.language_code == "ja"
            assert result.language_name == "Japanese"
            # Bytes are written to a temp file (faster-whisper needs a
            # path/file-like input), so transcribe gets a path, not bytes.
            mock_model.transcribe.assert_called_once()
            transcribe_arg = mock_model.transcribe.call_args[0][0]
            assert isinstance(transcribe_arg, str)
            assert transcribe_arg != audio_bytes

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
