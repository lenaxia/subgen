"""
Integration tests for multi-language subtitle generation (EPIC_10).

Tests the full multi-language workflow including:
- Language policy decision making
- Skip logic with output languages
- Filename generation with target language

Note: These tests use relative imports and may require the language_code module.
If the module is not available, tests will be skipped.
"""

import os
import sys
import tempfile
import pytest
from pathlib import Path
from unittest.mock import Mock, patch

# Add paths for imports
worker_dir = Path(__file__).parent.parent.parent  # Go up to worker/
sys.path.insert(0, str(worker_dir / "src"))
sys.path.insert(0, str(worker_dir))

# Check if language_code is available
try:
    import language_code  # noqa

    HAS_LANGUAGE_CODE = True
except ImportError:
    HAS_LANGUAGE_CODE = False

# Only import if available
if HAS_LANGUAGE_CODE:
    from language.policy import determine_output_languages, OutputType
    from subtitles.skip_checker import SkipChecker, SkipReason
    from subtitles.writer import generate_subtitle_path
    from config.settings import WorkerSettings

# Mark all tests as requiring language_code
requires_lang_code = pytest.mark.skipif(
    not HAS_LANGUAGE_CODE, reason="language_code module not available"
)


@pytest.fixture
def temp_dir():
    """Create a temporary directory for test files."""
    with tempfile.TemporaryDirectory() as tmpdir:
        yield tmpdir


@pytest.mark.integration
class TestLanguagePolicyIntegration:
    """Integration tests for language policy decision making."""

    @requires_lang_code
    def test_anime_japanese_to_multi_language(self):
        """Test Japanese audio with English and Chinese targets."""
        outputs = determine_output_languages(
            audio_language="jpn",
            target_languages=["eng", "zho-tw"],
            preferred_languages=["jpn", "eng"],
            transcribe_preferred=True,
        )

        assert len(outputs) == 3

        languages_and_types = [(o.language, o.output_type) for o in outputs]
        assert ("jpn", OutputType.TRANSCRIBE) in languages_and_types
        assert ("eng", OutputType.TRANSLATE) in languages_and_types
        assert ("zho-tw", OutputType.TRANSLATE) in languages_and_types

    @requires_lang_code
    def test_foreign_film_to_english_only(self):
        """Test Korean audio with only English target (no Korean in preferred)."""
        outputs = determine_output_languages(
            audio_language="kor",
            target_languages=["eng"],
            preferred_languages=["eng"],
            transcribe_preferred=True,
        )

        assert len(outputs) == 1
        assert outputs[0].language == "eng"
        assert outputs[0].output_type == OutputType.TRANSLATE

    @requires_lang_code
    def test_english_audio_with_multi_targets(self):
        """Test English audio with English and Chinese targets."""
        outputs = determine_output_languages(
            audio_language="eng",
            target_languages=["eng", "zho-tw"],
            preferred_languages=["eng"],
            transcribe_preferred=True,
        )

        assert len(outputs) == 2

        languages_and_types = [(o.language, o.output_type) for o in outputs]
        assert ("eng", OutputType.TRANSCRIBE) in languages_and_types
        assert ("zho-tw", OutputType.TRANSLATE) in languages_and_types


@pytest.mark.integration
class TestSkipCheckerMultiLanguage:
    """Integration tests for skip logic with output languages."""

    @requires_lang_code
    def test_skip_existing_target_language(self, temp_dir):
        """Skip only if specific target language exists."""
        config = WorkerSettings()
        skip_checker = SkipChecker(config)

        video_path = os.path.join(temp_dir, "test.mkv")
        Path(video_path).touch()

        eng_srt = os.path.join(temp_dir, "test.subgen.medium.eng.srt")
        Path(eng_srt).touch()

        # Should skip for English
        result = skip_checker.check(video_path, output_language="eng")
        assert result.should_skip is True

        # Should NOT skip for Chinese (doesn't exist yet)
        result = skip_checker.check(video_path, output_language="zho-tw")
        assert result.should_skip is False

    @requires_lang_code
    def test_generate_multiple_languages_independently(self, temp_dir):
        """Multiple languages can be generated independently."""
        config = WorkerSettings()
        skip_checker = SkipChecker(config)

        video_path = os.path.join(temp_dir, "movie.mkv")
        Path(video_path).touch()

        # No subtitles exist - all languages should be processable
        for lang in ["eng", "jpn", "zho-tw"]:
            result = skip_checker.check(video_path, output_language=lang)
            assert result.should_skip is False, f"Should not skip for {lang}"

        # Create English subtitle
        eng_srt = os.path.join(temp_dir, "movie.subgen.medium.eng.srt")
        Path(eng_srt).touch()

        # Now only English should be skipped
        assert skip_checker.check(video_path, output_language="eng").should_skip is True
        assert skip_checker.check(video_path, output_language="jpn").should_skip is False
        assert skip_checker.check(video_path, output_language="zho-tw").should_skip is False


@pytest.mark.integration
class TestMultiLanguageFilename:
    """Integration tests for subtitle filename generation with target language."""

    @requires_lang_code
    def test_translated_subtitle_uses_target_language(self):
        """Translated subtitle should use target language in filename."""
        with patch("subtitles.writer.LanguageCode") as mock_lang_code:
            mock_lang = Mock()
            mock_lang.to_iso_639_2_b = Mock(return_value="jpn")

            path = generate_subtitle_path(
                "/media/anime.mkv",
                mock_lang,
                "medium",
                target_language="eng",
            )

            assert path.endswith(".eng.srt")
            assert "jpn" not in path

    @requires_lang_code
    def test_transcribed_subtitle_uses_detected_language(self):
        """Transcribed subtitle should use detected language in filename."""
        with patch("subtitles.writer.LanguageCode") as mock_lang_code:
            mock_lang = Mock()
            mock_lang.to_iso_639_2_b = Mock(return_value="jpn")

            path = generate_subtitle_path(
                "/media/anime.mkv",
                mock_lang,
                "medium",
                target_language=None,
            )

            assert path.endswith(".jpn.srt")


@pytest.mark.integration
class TestConfigParsing:
    """Test configuration parsing for multi-language settings."""

    @requires_lang_code
    def test_target_languages_parsing(self):
        """Test TARGET_LANGUAGES is parsed correctly."""
        config = WorkerSettings()
        config.skip.target_languages = "eng,zho-tw"

        langs = config.skip.get_target_languages()
        assert len(langs) == 2
        assert "eng" in langs
        assert "zho-tw" in langs

    @requires_lang_code
    def test_target_languages_pipe_separator(self):
        """Test TARGET_LANGUAGES with pipe separator."""
        config = WorkerSettings()
        config.skip.target_languages = "eng|zho-tw|jpn"

        langs = config.skip.get_target_languages()
        assert len(langs) == 3
        assert "eng" in langs
        assert "zho-tw" in langs
        assert "jpn" in langs

    @requires_lang_code
    def test_target_languages_empty(self):
        """Test empty TARGET_LANGUAGES returns empty list."""
        config = WorkerSettings()
        config.skip.target_languages = ""

        langs = config.skip.get_target_languages()
        assert langs == []

    @requires_lang_code
    def test_transcribe_preferred_default(self):
        """Test TRANSCRIBE_PREFERRED defaults to True."""
        config = WorkerSettings()
        assert config.skip.transcribe_preferred is True
