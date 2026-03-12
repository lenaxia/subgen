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

    def test_generate_subtitle_path_with_target_language(self):
        """Test generating path with target_language for translations (EPIC_10)."""
        with patch("subtitles.writer.LanguageCode") as mock_lang_code:
            mock_lang = Mock()
            mock_lang.to_iso_639_2_b = Mock(return_value="jpn")  # Detected Japanese

            path = generate_subtitle_path(
                "/media/anime.mkv",
                mock_lang,  # Detected language (Japanese)
                "medium",
                show_subgen=True,
                show_model=True,
                format="srt",
                target_language="eng",  # Target English for translation
            )

            # Should use target_language in filename, not detected language
            assert path == "/media/anime.subgen.medium.eng.srt"

    def test_generate_subtitle_path_with_target_language_zho_tw(self):
        """Test target_language for Chinese Traditional (EPIC_10)."""
        with patch("subtitles.writer.LanguageCode") as mock_lang_code:
            mock_lang = Mock()
            mock_lang.to_iso_639_2_b = Mock(return_value="jpn")

            path = generate_subtitle_path(
                "/media/anime.mkv",
                mock_lang,
                "medium",
                show_subgen=True,
                show_model=True,
                format="srt",
                target_language="zho-tw",
            )

            assert path == "/media/anime.subgen.medium.zho-tw.srt"

    def test_generate_subtitle_path_no_target_uses_detected(self):
        """Test without target_language, uses detected language (transcribe mode)."""
        with patch("subtitles.writer.LanguageCode") as mock_lang_code:
            mock_lang = Mock()
            mock_lang.to_iso_639_2_b = Mock(return_value="eng")

            path = generate_subtitle_path(
                "/media/movie.mkv",
                mock_lang,
                "small",
                show_subgen=True,
                show_model=True,
                format="srt",
                target_language=None,  # No target language
            )

            # Should use detected language
            assert path == "/media/movie.subgen.small.eng.srt"

    def test_generate_subtitle_path_target_overrides_detected(self):
        """Test target_language overrides detected language in filename."""
        with patch("subtitles.writer.LanguageCode") as mock_lang_code:
            mock_lang = Mock()
            mock_lang.to_iso_639_2_b = Mock(return_value="kor")  # Detected Korean

            path = generate_subtitle_path(
                "/media/kdrama.mkv",
                mock_lang,
                "medium",
                show_subgen=True,
                show_model=True,
                format="srt",
                target_language="eng",  # Translate to English
            )

            # Should NOT contain 'kor', should contain 'eng'
            assert "kor" not in path
            assert path == "/media/kdrama.subgen.medium.eng.srt"


@pytest.mark.skipif(not SUBTITLE_MODULE_EXISTS, reason="Subtitle module not yet implemented")
class TestWriteLRC:
    """Test write_lrc function (from subgen.py:1218-1225)."""

    def test_write_lrc_basic(self):
        """Test writing basic LRC file and returns correct segment count."""
        segments = [
            Mock(start=0.0, text="Hello world"),
            Mock(start=2.5, text="This is a test"),
            Mock(start=5.0, text="LRC subtitle"),
        ]

        m = mock_open()
        with patch("builtins.open", m), patch("os.replace"):
            count = write_lrc(segments, "/tmp/test.lrc", append_footer=False)

        assert count == 3

        # Verify content
        handle = m()
        calls = handle.write.call_args_list
        written_lines = [call[0][0] for call in calls]
        assert "[00:00.00]Hello world\n" in written_lines
        assert "[00:02.50]This is a test\n" in written_lines
        assert "[00:05.00]LRC subtitle\n" in written_lines

    def test_write_lrc_accepts_generator(self):
        """Test write_lrc consumes a generator (not just a list) without error."""

        def segment_gen():
            yield Mock(start=0.0, text="First")
            yield Mock(start=1.0, text="Second")

        m = mock_open()
        with patch("builtins.open", m), patch("os.replace"):
            count = write_lrc(segment_gen(), "/tmp/test.lrc", append_footer=False)

        assert count == 2

    def test_write_lrc_returns_zero_for_empty(self):
        """Test returns 0 when no segments are provided."""
        m = mock_open()
        with patch("builtins.open", m), patch("os.replace"):
            count = write_lrc(iter([]), "/tmp/test.lrc", append_footer=False)

        assert count == 0

    def test_write_lrc_with_footer(self):
        """Test writing LRC with footer."""
        segments = [Mock(start=0.0, text="Test")]

        m = mock_open()
        with patch("builtins.open", m), patch("os.replace"):
            write_lrc(segments, "/tmp/test.lrc", append_footer=True)

        handle = m()
        calls = handle.write.call_args_list
        assert any("Transcribed by Subgen" in str(call) for call in calls)

    def test_write_lrc_removes_newlines(self):
        """Test that embedded newlines in text are replaced with spaces."""
        segments = [
            Mock(start=0.0, text="Line with\nnewline"),
            Mock(start=2.0, text="Another\n\ntest"),
        ]

        m = mock_open()
        with patch("builtins.open", m), patch("os.replace"):
            write_lrc(segments, "/tmp/test.lrc", append_footer=False)

        handle = m()
        calls = handle.write.call_args_list
        written_lines = [call[0][0] for call in calls]

        assert "[00:00.00]Line with newline\n" in written_lines
        assert "[00:02.00]Another  test\n" in written_lines

    def test_write_lrc_atomic_write(self):
        """Test atomic write using temp file."""
        segments = [Mock(start=0.0, text="Test")]

        m = mock_open()
        with patch("builtins.open", m), patch("os.replace") as mock_replace:
            write_lrc(segments, "/tmp/test.lrc", append_footer=False)

        m.assert_called_with("/tmp/test.lrc.tmp", "w", encoding="utf-8")
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
        written_lines = [call[0][0] for call in calls]

        assert "[01:05.25]After one minute\n" in written_lines
        assert "[02:05.50]After two minutes\n" in written_lines


@pytest.mark.skipif(not SUBTITLE_MODULE_EXISTS, reason="Subtitle module not yet implemented")
class TestWriteSRT:
    """Test write_srt function."""

    def _make_segments(self, n=3):
        segs = []
        for i in range(n):
            s = Mock()
            s.start = float(i * 2)
            s.end = float(i * 2 + 1)
            s.text = f"Segment {i + 1}"
            segs.append(s)
        return segs

    def test_write_srt_basic(self):
        """Test writing SRT file and returns correct segment count."""
        segments = self._make_segments(3)

        m = mock_open()
        with patch("builtins.open", m), patch("os.replace"):
            count = write_srt(
                segments, "/tmp/test.srt", word_level_highlight=False, append_footer=False
            )

        assert count == 3

        handle = m()
        calls = handle.write.call_args_list
        written = "".join(call[0][0] for call in calls)

        # Verify SRT structure: index, timestamps, text, blank line
        assert "1\n" in written
        assert "2\n" in written
        assert "00:00:00,000 --> 00:00:01,000\n" in written
        assert "Segment 1\n" in written

    def test_write_srt_accepts_generator(self):
        """Test write_srt streams from a generator without materialising all segments."""

        def segment_gen():
            for i in range(5):
                s = Mock()
                s.start = float(i)
                s.end = float(i) + 0.9
                s.text = f"Line {i}"
                yield s

        m = mock_open()
        with patch("builtins.open", m), patch("os.replace"):
            count = write_srt(segment_gen(), "/tmp/test.srt")

        assert count == 5

    def test_write_srt_returns_zero_for_empty(self):
        """Test returns 0 when no segments."""
        m = mock_open()
        with patch("builtins.open", m), patch("os.replace"):
            count = write_srt(iter([]), "/tmp/test.srt")

        assert count == 0

    def test_write_srt_with_footer(self):
        """Test appending footer to SRT."""
        segments = self._make_segments(1)

        m = mock_open()
        with patch("builtins.open", m), patch("os.replace"):
            write_srt(segments, "/tmp/test.srt", word_level_highlight=False, append_footer=True)

        handle = m()
        calls = handle.write.call_args_list
        written = "".join(call[0][0] for call in calls)

        assert "Transcribed by Subgen" in written

    def test_write_srt_atomic_write(self):
        """Test atomic write using temp file."""
        segments = self._make_segments(1)

        m = mock_open()
        with patch("builtins.open", m), patch("os.replace") as mock_replace:
            write_srt(segments, "/tmp/test.srt", word_level_highlight=False, append_footer=False)

        m.assert_called_with("/tmp/test.srt.tmp", "w", encoding="utf-8")
        mock_replace.assert_called_once_with("/tmp/test.srt.tmp", "/tmp/test.srt")

    def test_write_srt_error_cleanup(self):
        """Test temp file cleanup on error."""
        segments = self._make_segments(1)

        m = mock_open()
        m.side_effect = IOError("Disk full")

        with (
            patch("builtins.open", m),
            patch("os.path.exists", return_value=True),
            patch("os.remove") as mock_remove,
        ):
            with pytest.raises(SubtitleGenerationError):
                write_srt(segments, "/tmp/test.srt")

            mock_remove.assert_called_once_with("/tmp/test.srt.tmp")

    def test_write_srt_timestamp_format(self):
        """Test SRT timestamp formatting: HH:MM:SS,mmm."""
        s = Mock()
        s.start = 3723.456  # 01:02:03,456
        s.end = 3724.0
        s.text = "Timestamp test"

        m = mock_open()
        with patch("builtins.open", m), patch("os.replace"):
            write_srt([s], "/tmp/test.srt")

        handle = m()
        calls = handle.write.call_args_list
        written = "".join(call[0][0] for call in calls)

        assert "01:02:03,456 --> " in written

    def test_write_srt_footer_uses_final_count(self):
        """Test footer index is one past the final segment count."""
        segments = self._make_segments(4)

        m = mock_open()
        with patch("builtins.open", m), patch("os.replace"):
            write_srt(segments, "/tmp/test.srt", append_footer=True)

        handle = m()
        calls = handle.write.call_args_list
        written = "".join(call[0][0] for call in calls)

        # Footer should use index 5 (4 + 1)
        assert "5\n" in written
        assert "99:59:59,999 --> 99:59:59,999\n" in written


@pytest.mark.skipif(not SUBTITLE_MODULE_EXISTS, reason="Subtitle module not yet implemented")
class TestAppendLineToResult:
    """Test append_line_to_result function (in-place mutation for legacy callers)."""

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

        append_line_to_result(mock_result)

        assert len(mock_result.segments) == 0
