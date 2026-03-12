"""
Unit tests for skip logic module.

Tests all skip scenarios comprehensively.
"""

import os
import tempfile
import pytest
from pathlib import Path

from subtitles.skip_checker import SkipChecker, SkipReason
from config.settings import WorkerSettings


@pytest.fixture
def config() -> WorkerSettings:
    """Create a default worker config for testing."""
    return WorkerSettings()


@pytest.fixture
def skip_checker(config: WorkerSettings) -> SkipChecker:
    """Create a skip checker instance."""
    return SkipChecker(config)


@pytest.fixture
def temp_dir():
    """Create a temporary directory for test files."""
    with tempfile.TemporaryDirectory() as tmpdir:
        yield tmpdir


class TestSkipCheckerTargetSubtitles:
    """Test skip logic for target subtitle existence."""

    def test_skip_if_subgen_srt_exists(self, temp_dir, skip_checker):
        """Should skip if subgen-generated SRT exists."""
        # Create test video file
        video_path = os.path.join(temp_dir, "test.mkv")
        Path(video_path).touch()

        # Create existing subgen subtitle
        subtitle_path = os.path.join(temp_dir, "test.subgen.medium.eng.srt")
        Path(subtitle_path).touch()

        result = skip_checker.check(video_path)
        assert result.should_skip is True
        assert result.reason == SkipReason.SUBTITLE_EXISTS
        assert "subgen" in result.details.lower()

    def test_skip_if_subgen_lrc_exists(self, temp_dir, skip_checker):
        """Should skip if subgen-generated LRC exists."""
        # Create test audio file
        audio_path = os.path.join(temp_dir, "test.mp3")
        Path(audio_path).touch()

        # Create existing subgen LRC
        lrc_path = os.path.join(temp_dir, "test.subgen.medium.eng.lrc")
        Path(lrc_path).touch()

        result = skip_checker.check(audio_path)
        assert result.should_skip is True
        assert result.reason == SkipReason.LRC_EXISTS

    def test_skip_if_regular_srt_exists(self, temp_dir, skip_checker):
        """Should skip if regular SRT exists."""
        # Create test video file
        video_path = os.path.join(temp_dir, "test.mkv")
        Path(video_path).touch()

        # Create existing subtitle
        subtitle_path = os.path.join(temp_dir, "test.srt")
        Path(subtitle_path).touch()

        result = skip_checker.check(video_path)
        assert result.should_skip is True
        assert result.reason == SkipReason.SUBTITLE_EXISTS

    def test_skip_if_regular_lrc_exists(self, temp_dir, skip_checker):
        """Should skip if regular LRC exists."""
        # Create test audio file
        audio_path = os.path.join(temp_dir, "test.mp3")
        Path(audio_path).touch()

        # Create existing LRC
        lrc_path = os.path.join(temp_dir, "test.lrc")
        Path(lrc_path).touch()

        result = skip_checker.check(audio_path)
        assert result.should_skip is True
        assert result.reason == SkipReason.LRC_EXISTS

    def test_no_skip_if_no_subtitles(self, temp_dir, skip_checker):
        """Should not skip if no subtitles exist."""
        # Create test video file
        video_path = os.path.join(temp_dir, "test.mkv")
        Path(video_path).touch()

        result = skip_checker.check(video_path)
        assert result.should_skip is False
        assert result.reason is None


class TestSkipCheckerFileTypes:
    """Test skip logic for different file types."""

    def test_video_file_checks_subtitles(self, temp_dir, skip_checker):
        """Video files should check for SRT subtitles."""
        # Create test video file
        video_path = os.path.join(temp_dir, "test.mkv")
        Path(video_path).touch()

        result = skip_checker.check(video_path)
        assert result.should_skip is False
        assert result.reason is None

    def test_audio_file_checks_lrc(self, temp_dir, skip_checker):
        """Audio files should check for LRC subtitles."""
        # Create test audio file
        audio_path = os.path.join(temp_dir, "test.mp3")
        Path(audio_path).touch()

        result = skip_checker.check(audio_path)
        assert result.should_skip is False
        assert result.reason is None

    def test_unsupported_file_no_check(self, temp_dir, skip_checker):
        """Unsupported files should not perform skip checks."""
        # Create unsupported file
        unsupported_path = os.path.join(temp_dir, "test.txt")
        Path(unsupported_path).touch()

        result = skip_checker.check(unsupported_path)
        # Should not skip since it's not a media file
        assert result.should_skip is False
        assert result.reason is None
