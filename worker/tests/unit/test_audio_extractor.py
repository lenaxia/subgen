"""
Unit tests for audio extraction module.

Tests extracted logic from subgen.py:1318-1350, 1352-1386, 1446-1490, 2016-2038
"""

import pytest
from unittest.mock import Mock, patch, MagicMock
from io import BytesIO

# Import will fail until we implement the module
try:
    from audio.extractor import (
        has_audio,
        get_audio_tracks,
        extract_audio_track,
        handle_multiple_audio_tracks,
        extract_audio_segment,
        AudioTrackInfo,
        AudioExtractionError,
    )

    AUDIO_MODULE_EXISTS = True
except ImportError:
    AUDIO_MODULE_EXISTS = False
    pytestmark = pytest.mark.skip(reason="Audio module not yet implemented")


@pytest.mark.skipif(not AUDIO_MODULE_EXISTS, reason="Audio module not yet implemented")
class TestHasAudio:
    """Test has_audio function (from subgen.py:2016-2038)."""

    def test_has_audio_with_valid_codec(self):
        """Test file with valid audio codec returns True."""
        with patch("av.open") as mock_av:
            mock_container = MagicMock()
            mock_stream = Mock()
            mock_stream.type = "audio"
            mock_stream.codec_context = Mock()
            mock_stream.codec_context.name = "aac"
            mock_container.streams = [mock_stream]
            mock_container.__enter__ = Mock(return_value=mock_container)
            mock_container.__exit__ = Mock(return_value=False)
            mock_av.return_value = mock_container

            assert has_audio("/test/video.mp4") is True

    def test_has_audio_with_none_codec(self):
        """Test file with 'none' codec returns False."""
        with patch("av.open") as mock_av:
            mock_container = MagicMock()
            mock_stream = Mock()
            mock_stream.type = "audio"
            mock_stream.codec_context = Mock()
            mock_stream.codec_context.name = "none"
            mock_container.streams = [mock_stream]
            mock_container.__enter__ = Mock(return_value=mock_container)
            mock_container.__exit__ = Mock(return_value=False)
            mock_av.return_value = mock_container

            assert has_audio("/test/invalid.mp4") is False

    def test_has_audio_no_audio_stream(self):
        """Test file with no audio stream returns False."""
        with patch("av.open") as mock_av:
            mock_container = MagicMock()
            mock_stream = Mock()
            mock_stream.type = "video"  # Video stream, not audio
            mock_container.streams = [mock_stream]
            mock_container.__enter__ = Mock(return_value=mock_container)
            mock_container.__exit__ = Mock(return_value=False)
            mock_av.return_value = mock_container

            assert has_audio("/test/novideo.mp4") is False

    def test_has_audio_ffmpeg_error(self):
        """Test FFmpeg error returns False."""
        with patch("av.open") as mock_av:
            import av

            mock_av.side_effect = av.FFmpegError(-1, "error")

            assert has_audio("/test/error.mp4") is False

    def test_has_audio_unicode_error(self):
        """Test Unicode decode error returns False."""
        with patch("av.open") as mock_av:
            mock_av.side_effect = UnicodeDecodeError("utf-8", b"", 0, 1, "error")

            assert has_audio("/test/unicode_error.mp4") is False


@pytest.mark.skipif(not AUDIO_MODULE_EXISTS, reason="Audio module not yet implemented")
class TestGetAudioTracks:
    """Test get_audio_tracks function (from subgen.py:1446-1490)."""

    def test_get_audio_tracks_single_track(self):
        """Test extracting single audio track info."""
        with patch("ffmpeg.probe") as mock_probe:
            mock_probe.return_value = {
                "streams": [
                    {
                        "index": 0,
                        "codec_name": "aac",
                        "channels": 2,
                        "tags": {"language": "eng"},
                        "disposition": {"default": 1},
                    }
                ]
            }

            tracks = get_audio_tracks("/test/video.mp4")

            assert len(tracks) == 1
            assert tracks[0].index == 0
            assert tracks[0].codec == "aac"
            assert tracks[0].channels == 2
            assert tracks[0].language == "eng"
            assert tracks[0].is_default is True

    def test_get_audio_tracks_multiple_tracks(self):
        """Test extracting multiple audio track info."""
        with patch("ffmpeg.probe") as mock_probe:
            mock_probe.return_value = {
                "streams": [
                    {
                        "index": 0,
                        "codec_name": "aac",
                        "channels": 2,
                        "tags": {"language": "eng", "title": "English"},
                        "disposition": {"default": 1},
                    },
                    {
                        "index": 1,
                        "codec_name": "ac3",
                        "channels": 6,
                        "tags": {"language": "jpn", "title": "Japanese"},
                        "disposition": {"default": 0},
                    },
                ]
            }

            tracks = get_audio_tracks("/test/video.mkv")

            assert len(tracks) == 2
            assert tracks[0].language == "eng"
            assert tracks[0].is_default is True
            assert tracks[1].language == "jpn"
            assert tracks[1].is_default is False
            assert tracks[1].title == "Japanese"

    def test_get_audio_tracks_missing_tags(self):
        """Test handling missing language tags."""
        with patch("ffmpeg.probe") as mock_probe:
            mock_probe.return_value = {
                "streams": [
                    {
                        "index": 0,
                        "codec_name": "aac",
                        "channels": 2,
                        "tags": {},  # No language tag
                        "disposition": {},
                    }
                ]
            }

            tracks = get_audio_tracks("/test/video.mp4")

            assert len(tracks) == 1
            assert tracks[0].language == "und"  # Undefined
            assert tracks[0].is_default is False

    def test_get_audio_tracks_ffmpeg_error(self):
        """Test FFmpeg error raises AudioExtractionError."""
        with patch("ffmpeg.probe") as mock_probe:
            import ffmpeg

            mock_error = ffmpeg.Error("cmd", b"", b"error message")
            mock_probe.side_effect = mock_error

            with pytest.raises(AudioExtractionError):
                get_audio_tracks("/test/error.mp4")


@pytest.mark.skipif(not AUDIO_MODULE_EXISTS, reason="Audio module not yet implemented")
class TestExtractAudioTrack:
    """Test extract_audio_track context manager (from subgen.py:1352-1386)."""

    def test_extract_audio_track_success(self):
        """Test successful audio track extraction."""
        fake_audio = b"RIFF" + b"\x00" * 100

        with patch("ffmpeg.input") as mock_input:
            mock_stream = Mock()
            mock_output = Mock()
            mock_output.run = Mock(return_value=(fake_audio, b""))
            mock_stream.output = Mock(return_value=mock_output)
            mock_input.return_value = mock_stream

            with extract_audio_track("/test/video.mp4", 0) as audio_buffer:
                data = audio_buffer.read()
                assert len(data) == 104
                assert data[:4] == b"RIFF"

    def test_extract_audio_track_specified_index(self):
        """Test extracting specific track index."""
        fake_audio = b"audio_data"

        with patch("ffmpeg.input") as mock_input:
            mock_stream = Mock()
            mock_output = Mock()
            mock_output.run = Mock(return_value=(fake_audio, b""))
            mock_stream.output = Mock(return_value=mock_output)
            mock_input.return_value = mock_stream

            with extract_audio_track("/test/video.mkv", 1) as audio_buffer:
                data = audio_buffer.read()
                assert data == fake_audio

            # Verify correct track index was used
            mock_stream.output.assert_called_once()
            call_args = mock_stream.output.call_args
            assert "map" in call_args[1]
            assert call_args[1]["map"] == "0:1"

    def test_extract_audio_track_ffmpeg_error(self):
        """Test FFmpeg error raises AudioExtractionError."""
        with patch("ffmpeg.input") as mock_input:
            import ffmpeg

            mock_stream = Mock()
            mock_output = Mock()
            mock_error = ffmpeg.Error("cmd", b"", b"error")
            mock_output.run = Mock(side_effect=mock_error)
            mock_stream.output = Mock(return_value=mock_output)
            mock_input.return_value = mock_stream

            with pytest.raises(AudioExtractionError):
                with extract_audio_track("/test/error.mp4", 0) as audio_buffer:
                    pass


@pytest.mark.skipif(not AUDIO_MODULE_EXISTS, reason="Audio module not yet implemented")
class TestHandleMultipleAudioTracks:
    """Test handle_multiple_audio_tracks function (from subgen.py:1318-1350)."""

    def test_single_audio_track_returns_none(self):
        """Test single audio track returns None (no extraction needed)."""
        with patch("audio.extractor.get_audio_tracks") as mock_get:
            mock_get.return_value = [
                AudioTrackInfo(index=0, codec="aac", channels=2, language="eng", is_default=True)
            ]

            result = handle_multiple_audio_tracks("/test/video.mp4")

            assert result is None

    def test_multiple_tracks_extracts_default(self):
        """Test multiple tracks extracts default track."""
        with (
            patch("audio.extractor.get_audio_tracks") as mock_get,
            patch("audio.extractor.extract_audio_track") as mock_extract,
        ):
            mock_get.return_value = [
                AudioTrackInfo(0, "aac", 2, "eng", True),
                AudioTrackInfo(1, "ac3", 6, "jpn", False),
            ]

            fake_audio = b"audio_data"
            mock_buffer = BytesIO(fake_audio)
            mock_extract.return_value.__enter__ = Mock(return_value=mock_buffer)
            mock_extract.return_value.__exit__ = Mock(return_value=False)

            result = handle_multiple_audio_tracks("/test/video.mkv")

            assert result is not None
            assert result.read() == fake_audio

    def test_multiple_tracks_preferred_language(self):
        """Test selecting track by preferred language."""
        with (
            patch("audio.extractor.get_audio_tracks") as mock_get,
            patch("audio.extractor.extract_audio_track") as mock_extract,
        ):
            mock_get.return_value = [
                AudioTrackInfo(0, "aac", 2, "eng", True),
                AudioTrackInfo(1, "ac3", 6, "jpn", False),
            ]

            fake_audio = b"japanese_audio"
            mock_buffer = BytesIO(fake_audio)
            mock_extract.return_value.__enter__ = Mock(return_value=mock_buffer)
            mock_extract.return_value.__exit__ = Mock(return_value=False)

            result = handle_multiple_audio_tracks("/test/video.mkv", "jpn")

            # Should extract Japanese track (index 1)
            mock_extract.assert_called_once_with("/test/video.mkv", 1)

    def test_multiple_tracks_fallback_to_first(self):
        """Test fallback to first track if no default."""
        with (
            patch("audio.extractor.get_audio_tracks") as mock_get,
            patch("audio.extractor.extract_audio_track") as mock_extract,
        ):
            mock_get.return_value = [
                AudioTrackInfo(0, "aac", 2, "eng", False),
                AudioTrackInfo(1, "ac3", 6, "jpn", False),
            ]

            fake_audio = b"audio"
            mock_buffer = BytesIO(fake_audio)
            mock_extract.return_value.__enter__ = Mock(return_value=mock_buffer)
            mock_extract.return_value.__exit__ = Mock(return_value=False)

            result = handle_multiple_audio_tracks("/test/video.mkv")

            # Should extract first track (index 0)
            mock_extract.assert_called_once_with("/test/video.mkv", 0)


@pytest.mark.skipif(not AUDIO_MODULE_EXISTS, reason="Audio module not yet implemented")
class TestExtractAudioSegment:
    """Test extract_audio_segment context manager (from subgen.py:1100-1141)."""

    def test_extract_audio_segment_success(self):
        """Test successful audio segment extraction."""
        fake_audio = b"RIFF" + b"\x00" * 100

        with patch("ffmpeg.input") as mock_input:
            mock_stream = Mock()
            mock_output = Mock()
            mock_output.run = Mock(return_value=(fake_audio, b""))
            mock_stream.output = Mock(return_value=mock_output)
            mock_input.return_value = mock_stream

            with extract_audio_segment("/test/video.mp4", 10, 30) as audio_buffer:
                data = audio_buffer.read()
                assert len(data) > 0

            # Verify start time and duration were passed
            mock_input.assert_called_once_with("/test/video.mp4", ss=10, t=30)

    def test_extract_audio_segment_empty_output(self):
        """Test extraction with empty FFmpeg output raises error."""
        with patch("ffmpeg.input") as mock_input:
            mock_stream = Mock()
            mock_output = Mock()
            mock_output.run = Mock(return_value=(b"", b""))  # Empty output
            mock_stream.output = Mock(return_value=mock_output)
            mock_input.return_value = mock_stream

            with pytest.raises(AudioExtractionError, match="empty"):
                with extract_audio_segment("/test/video.mp4", 0, 30):
                    pass

    def test_extract_audio_segment_ffmpeg_error(self):
        """Test FFmpeg error raises AudioExtractionError."""
        with patch("ffmpeg.input") as mock_input:
            import ffmpeg

            mock_stream = Mock()
            mock_output = Mock()
            mock_error = ffmpeg.Error("cmd", b"", b"stderr")
            mock_output.run = Mock(side_effect=mock_error)
            mock_stream.output = Mock(return_value=mock_output)
            mock_input.return_value = mock_stream

            with pytest.raises(AudioExtractionError):
                with extract_audio_segment("/test/error.mp4", 0, 30):
                    pass


@pytest.mark.skipif(not AUDIO_MODULE_EXISTS, reason="Audio module not yet implemented")
class TestAudioTrackInfo:
    """Test AudioTrackInfo dataclass."""

    def test_audio_track_info_creation(self):
        """Test creating AudioTrackInfo with all fields."""
        track = AudioTrackInfo(
            index=0, codec="aac", channels=2, language="eng", is_default=True, title="English Audio"
        )

        assert track.index == 0
        assert track.codec == "aac"
        assert track.channels == 2
        assert track.language == "eng"
        assert track.is_default is True
        assert track.title == "English Audio"

    def test_audio_track_info_optional_title(self):
        """Test AudioTrackInfo with optional title."""
        track = AudioTrackInfo(index=1, codec="ac3", channels=6, language="jpn", is_default=False)

        assert track.title is None
