"""
Unit tests for subtitle writer module.

Tests extracted logic from subgen.py:1218-1225, 1301-1316
"""

import pytest
import os
from unittest.mock import Mock, patch, mock_open
from pathlib import Path

# Import will fail until we implement the module
try:
    from subtitles.writer import (
        generate_subtitle_path,
        write_lrc,
        write_srt,
        append_line_to_result,
        SubtitleGenerationError,
    )

    SUBTITLE_MODULE_EXISTS = True
except ImportError:
    SUBTITLE_MODULE_EXISTS = False
    pytestmark = pytest.mark.skip(reason="Subtitle module not yet implemented")


@pytest.mark.skipif(not SUBTITLE_MODULE_EXISTS, reason="Subtitle module not yet implemented")
class TestGenerateSubtitlePath:
    """Test generate_subtitle_path function (from subgen.py:1301-1316)."""

    def test_generate_subtitle_path_full(self):
        """Test generating path with all options enabled."""
        with patch("subtitles.writer.LanguageCode") as mock_lang_code:
            mock_lang = Mock()
            mock_lang.to_iso_639_2_b = Mock(return_value="eng")
            mock_lang_code.ENGLISH = mock_lang

            path = generate_subtitle_path(
                "/media/video.mp4",
                mock_lang,
                "medium",
                show_subgen=True,
                show_model=True,
                format="srt",
            )

            assert path == "/media/video.subgen.medium.eng.srt"

    def test_generate_subtitle_path_no_subgen(self):
        """Test generating path without subgen marker."""
        with patch("subtitles.writer.LanguageCode") as mock_lang_code:
            mock_lang = Mock()
            mock_lang.to_iso_639_2_b = Mock(return_value="eng")

            path = generate_subtitle_path(
                "/media/video.mkv",
                mock_lang,
                "tiny",
                show_subgen=False,
                show_model=True,
                format="srt",
            )

            assert path == "/media/video.tiny.eng.srt"

    def test_generate_subtitle_path_no_model(self):
        """Test generating path without model name."""
        with patch("subtitles.writer.LanguageCode") as mock_lang_code:
            mock_lang = Mock()
            mock_lang.to_iso_639_2_b = Mock(return_value="jpn")

            path = generate_subtitle_path(
                "/media/video.avi",
                mock_lang,
                "large",
                show_subgen=True,
                show_model=False,
                format="srt",
            )

            assert path == "/media/video.subgen.jpn.srt"

    def test_generate_subtitle_path_minimal(self):
        """Test generating path with minimal options."""
        with patch("subtitles.writer.LanguageCode") as mock_lang_code:
            mock_lang = Mock()
            mock_lang.to_iso_639_2_b = Mock(return_value="fra")

            path = generate_subtitle_path(
                "/media/movie.mp4",
                mock_lang,
                "medium",
                show_subgen=False,
                show_model=False,
                format="srt",
            )

            assert path == "/media/movie.fra.srt"

    def test_generate_subtitle_path_lrc_format(self):
        """Test generating LRC file path."""
        with patch("subtitles.writer.LanguageCode") as mock_lang_code:
            mock_lang = Mock()
            mock_lang.to_iso_639_2_b = Mock(return_value="eng")

            path = generate_subtitle_path(
                "/media/song.mp3",
                mock_lang,
                "small",
                show_subgen=True,
                show_model=True,
                format="lrc",
            )

            assert path == "/media/song.subgen.small.eng.lrc"

    def test_generate_subtitle_path_preserves_directory(self):
        """Test directory path is preserved."""
        with patch("subtitles.writer.LanguageCode") as mock_lang_code:
            mock_lang = Mock()
            mock_lang.to_iso_639_2_b = Mock(return_value="spa")

            path = generate_subtitle_path(
                "/very/long/path/to/video.mkv",
                mock_lang,
                "base",
                show_subgen=False,
                show_model=False,
                format="srt",
            )

            assert path == "/very/long/path/to/video.spa.srt"


@pytest.mark.skipif(not SUBTITLE_MODULE_EXISTS, reason="Subtitle module not yet implemented")
class TestWriteLRC:
    """Test write_lrc function (from subgen.py:1218-1225)."""

    def test_write_lrc_basic(self):
        """Test writing basic LRC file."""
        segments = [
            Mock(start=0.0, text="Hello world"),
            Mock(start=2.5, text="This is a test"),
            Mock(start=5.0, text="LRC subtitle"),
        ]

        m = mock_open()
        with patch("builtins.open", m), patch("os.replace"):
            write_lrc(segments, "/tmp/test.lrc", append_footer=False)

        # Verify content
        handle = m()
        calls = handle.write.call_args_list

        # Check first segment - extract actual written strings
        written_lines = [call[0][0] for call in calls]
        assert "[00:00.00]Hello world\n" in written_lines
        assert "[00:02.50]This is a test\n" in written_lines
        assert "[00:05.00]LRC subtitle\n" in written_lines

    def test_write_lrc_with_footer(self):
        """Test writing LRC with footer."""
        segments = [Mock(start=0.0, text="Test")]

        m = mock_open()
        with patch("builtins.open", m), patch("os.replace"):
            write_lrc(segments, "/tmp/test.lrc", append_footer=True)

        handle = m()
        calls = handle.write.call_args_list

        # Check footer present
        assert any("Transcribed by Subgen" in str(call) for call in calls)

    def test_write_lrc_removes_newlines(self):
        """Test that embedded newlines are removed."""
        segments = [
            Mock(start=0.0, text="Line with\nnewline"),
            Mock(start=2.0, text="Another\n\ntest"),
        ]

        m = mock_open()
        with patch("builtins.open", m), patch("os.replace"):
            write_lrc(segments, "/tmp/test.lrc", append_footer=False)

        handle = m()
        calls = handle.write.call_args_list

        # Extract actual written strings
        written_lines = [call[0][0] for call in calls]

        # Newlines in text should be replaced with spaces
        assert "[00:00.00]Line with newline\n" in written_lines
        assert "[00:02.00]Another  test\n" in written_lines

    def test_write_lrc_atomic_write(self):
        """Test atomic write using temp file."""
        segments = [Mock(start=0.0, text="Test")]

        m = mock_open()
        with patch("builtins.open", m), patch("os.replace") as mock_replace:
            write_lrc(segments, "/tmp/test.lrc", append_footer=False)

        # Verify opened temp file
        m.assert_called_with("/tmp/test.lrc.tmp", "w", encoding="utf-8")

        # Verify atomic rename
        mock_replace.assert_called_once_with("/tmp/test.lrc.tmp", "/tmp/test.lrc")

    def test_write_lrc_error_cleanup(self):
        """Test temp file cleanup on error."""
        segments = [Mock(start=0.0, text="Test")]

        m = mock_open()
        m.side_effect = IOError("Disk full")

        with (
            patch("builtins.open", m),
            patch("os.path.exists", return_value=True),
            patch("os.remove") as mock_remove,
        ):
            with pytest.raises(SubtitleGenerationError):
                write_lrc(segments, "/tmp/test.lrc", append_footer=False)

            # Verify temp file cleanup
            mock_remove.assert_called_once_with("/tmp/test.lrc.tmp")

    def test_write_lrc_minutes_seconds_format(self):
        """Test correct formatting of minutes and seconds."""
        segments = [
            Mock(start=65.25, text="After one minute"),  # 01:05.25
            Mock(start=125.5, text="After two minutes"),  # 02:05.50
        ]

        m = mock_open()
        with patch("builtins.open", m), patch("os.replace"):
            write_lrc(segments, "/tmp/test.lrc", append_footer=False)

        handle = m()
        calls = handle.write.call_args_list

        # Extract actual written strings
        written_lines = [call[0][0] for call in calls]

        assert "[01:05.25]After one minute\n" in written_lines
        assert "[02:05.50]After two minutes\n" in written_lines


@pytest.mark.skipif(not SUBTITLE_MODULE_EXISTS, reason="Subtitle module not yet implemented")
class TestWriteSRT:
    """Test write_srt function."""

    def test_write_srt_basic(self):
        """Test writing basic SRT file."""
        mock_result = Mock()
        mock_result.to_srt_vtt = Mock()

        with patch("os.replace"):
            write_srt(mock_result, "/tmp/test.srt", word_level_highlight=False, append_footer=False)

        # Verify stable-whisper called with temp file
        call_args = mock_result.to_srt_vtt.call_args
        assert call_args[0][0] == "/tmp/test.srt.tmp"
        assert call_args[1]["word_level"] == False

    def test_write_srt_word_level(self):
        """Test writing SRT with word-level timestamps."""
        mock_result = Mock()
        mock_result.to_srt_vtt = Mock()

        with patch("os.replace"):
            write_srt(mock_result, "/tmp/test.srt", word_level_highlight=True, append_footer=False)

        call_args = mock_result.to_srt_vtt.call_args
        assert call_args[1]["word_level"] == True

    def test_write_srt_with_footer(self):
        """Test appending footer to SRT."""
        mock_result = Mock()
        mock_result.to_srt_vtt = Mock()

        m = mock_open()
        with patch("builtins.open", m), patch("os.replace"):
            write_srt(mock_result, "/tmp/test.srt", word_level_highlight=False, append_footer=True)

        # Verify footer appended
        handle = m()
        calls = handle.write.call_args_list
        written_text = "".join(str(call) for call in calls)

        assert "Transcribed by Subgen" in written_text

    def test_write_srt_atomic_write(self):
        """Test atomic write using temp file."""
        mock_result = Mock()
        mock_result.to_srt_vtt = Mock()

        with patch("os.replace") as mock_replace:
            write_srt(mock_result, "/tmp/test.srt", word_level_highlight=False, append_footer=False)

        mock_replace.assert_called_once_with("/tmp/test.srt.tmp", "/tmp/test.srt")

    def test_write_srt_error_cleanup(self):
        """Test temp file cleanup on error."""
        mock_result = Mock()
        mock_result.to_srt_vtt = Mock(side_effect=RuntimeError("Error"))

        with patch("os.path.exists", return_value=True), patch("os.remove") as mock_remove:
            with pytest.raises(SubtitleGenerationError):
                write_srt(
                    mock_result, "/tmp/test.srt", word_level_highlight=False, append_footer=False
                )

            mock_remove.assert_called_once_with("/tmp/test.srt.tmp")


@pytest.mark.skipif(not SUBTITLE_MODULE_EXISTS, reason="Subtitle module not yet implemented")
class TestAppendLineToResult:
    """Test append_line_to_result function."""

    def test_append_line_to_segments(self):
        """Test appending newline to segments."""
        mock_result = Mock()
        mock_result.segments = [
            Mock(text="Hello world"),
            Mock(text="Second line"),
            Mock(text="Third line"),
        ]

        append_line_to_result(mock_result)

        assert mock_result.segments[0].text == "Hello world\n"
        assert mock_result.segments[1].text == "Second line\n"
        assert mock_result.segments[2].text == "Third line\n"

    def test_append_line_no_duplicate(self):
        """Test does not add duplicate newlines."""
        mock_result = Mock()
        mock_result.segments = [Mock(text="Already has newline\n"), Mock(text="No newline")]

        append_line_to_result(mock_result)

        assert mock_result.segments[0].text == "Already has newline\n"
        assert mock_result.segments[1].text == "No newline\n"

    def test_append_line_empty_segments(self):
        """Test with empty segments list."""
        mock_result = Mock()
        mock_result.segments = []

        # Should not raise error
        append_line_to_result(mock_result)

        assert len(mock_result.segments) == 0
