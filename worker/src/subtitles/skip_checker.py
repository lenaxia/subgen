"""
Skip logic module for worker.

Implements comprehensive skip decisions based on configuration and file inspection.
EPIC_10: Added support for output_language-specific skip checking.
"""

import os
import glob as glob_module
import logging
from typing import Optional, Tuple
from enum import Enum
from dataclasses import dataclass

from config.settings import WorkerSettings
from subtitles.detector import (
    scan_external_subtitles,
    get_embedded_subtitles,
    has_subtitle_language,
    is_audio_file,
    is_video_file,
)
from audio.extractor import get_audio_tracks

logger = logging.getLogger(__name__)


class SkipReason(Enum):
    """Reasons for skipping transcription."""

    SUBTITLE_EXISTS = "subtitle_file_exists"
    LRC_EXISTS = "lrc_file_exists"
    EMBEDDED_SUBTITLE = "embedded_subtitle_exists"
    EXTERNAL_SUBTITLE = "external_subtitle_exists"
    SUBTITLE_LANGUAGE_SKIP = "subtitle_language_in_skip_list"
    AUDIO_LANGUAGE_SKIP = "audio_language_in_skip_list"
    AUDIO_LANGUAGE_MISMATCH = "audio_language_mismatch"
    UNKNOWN_LANGUAGE = "unknown_language"


@dataclass
class SkipResult:
    """Result of skip check."""

    should_skip: bool
    reason: Optional[SkipReason]
    details: str


class SkipChecker:
    """Implements comprehensive skip logic for transcription."""

    def __init__(self, config: WorkerSettings):
        """Initialize skip checker."""
        self.config = config

    def check(self, file_path: str, output_language: Optional[str] = None) -> SkipResult:
        """Check if file should be skipped."""
        if self.config.skip.skip_if_target_subtitles_exist:
            result = self._check_target_subtitles_exist(file_path, output_language)
            if result.should_skip:
                return result

        if self.config.skip.check_embedded_subtitles and is_video_file(file_path):
            if self.config.skip.skip_if_internal_subtitles_language:
                result = self._check_embedded_subtitles(file_path)
                if result.should_skip:
                    return result

        if self.config.skip.skip_if_external_subtitles_exist:
            result = self._check_external_subtitles(file_path)
            if result.should_skip:
                return result

        if self.config.skip.get_skip_audio_languages() and is_video_file(file_path):
            result = self._check_audio_language_skip(file_path)
            if result.should_skip:
                return result

        if (
            self.config.skip.limit_to_preferred_audio_language
            and self.config.skip.get_preferred_audio_languages()
            and is_video_file(file_path)
        ):
            result = self._check_preferred_audio_language(file_path)
            if result.should_skip:
                return result

        return SkipResult(should_skip=False, reason=None, details="No skip condition met")

    def _check_target_subtitles_exist(
        self, file_path: str, output_language: Optional[str] = None
    ) -> SkipResult:
        """Check if target subtitle files already exist (EPIC_10: supports output_language)."""
        base = os.path.splitext(file_path)[0]

        # EPIC_10: If output_language specified, check for that specific language only
        if output_language:
            output_lang_lower = output_language.lower()

            # Check for subgen-generated subtitle with specific output language (case-insensitive)
            subgen_pattern_lower = f"{base}.subgen.*.{output_lang_lower}.srt"
            subgen_pattern_upper = f"{base}.subgen.*.{output_language.upper()}.srt"
            subgen_matches = glob_module.glob(subgen_pattern_lower) + glob_module.glob(
                subgen_pattern_upper
            )
            if subgen_matches:
                return SkipResult(
                    should_skip=True,
                    reason=SkipReason.SUBTITLE_EXISTS,
                    details=f"Subgen subtitle already exists for {output_language}: {subgen_matches[0]}",
                )

            # Check for simple subtitle with output language (case-insensitive)
            simple_srt_lower = f"{base}.{output_lang_lower}.srt"
            simple_srt_upper = f"{base}.{output_language.upper()}.srt"
            if os.path.exists(simple_srt_lower) or os.path.exists(simple_srt_upper):
                return SkipResult(
                    should_skip=True,
                    reason=SkipReason.SUBTITLE_EXISTS,
                    details=f"SRT file exists for {output_language}",
                )

            simple_lrc_lower = f"{base}.{output_lang_lower}.lrc"
            simple_lrc_upper = f"{base}.{output_language.upper()}.lrc"
            if os.path.exists(simple_lrc_lower) or os.path.exists(simple_lrc_upper):
                return SkipResult(
                    should_skip=True,
                    reason=SkipReason.LRC_EXISTS,
                    details=f"LRC file exists for {output_language}",
                )

            return SkipResult(
                should_skip=False,
                reason=None,
                details=f"No target subtitles found for {output_language}",
            )

        # Original behavior: check for any subgen subtitle
        subgen_srt = glob_module.glob(f"{base}.subgen.*.*.srt")
        subgen_lrc = glob_module.glob(f"{base}.subgen.*.*.lrc")
        if subgen_srt or subgen_lrc:
            existing_path = subgen_srt[0] if subgen_srt else subgen_lrc[0]
            return SkipResult(
                should_skip=True,
                reason=SkipReason.SUBTITLE_EXISTS if subgen_srt else SkipReason.LRC_EXISTS,
                details=f"Subgen subtitle already exists: {existing_path}",
            )

        if is_audio_file(file_path):
            lrc_path = f"{base}.lrc"
            if os.path.exists(lrc_path):
                return SkipResult(
                    should_skip=True,
                    reason=SkipReason.LRC_EXISTS,
                    details=f"LRC file exists: {lrc_path}",
                )

        if is_video_file(file_path):
            srt_path = f"{base}.srt"
            if os.path.exists(srt_path):
                return SkipResult(
                    should_skip=True,
                    reason=SkipReason.SUBTITLE_EXISTS,
                    details=f"SRT file exists: {srt_path}",
                )

        return SkipResult(should_skip=False, reason=None, details="No target subtitles found")

    def _check_embedded_subtitles(self, file_path: str) -> SkipResult:
        """Check if file has embedded subtitles in target language."""
        target_lang = self.config.skip.skip_if_internal_subtitles_language
        if not target_lang:
            return SkipResult(
                should_skip=False, reason=None, details="No target language configured"
            )

        try:
            embedded_subs = get_embedded_subtitles(file_path)
            if has_subtitle_language(embedded_subs, target_lang):
                return SkipResult(
                    should_skip=True,
                    reason=SkipReason.EMBEDDED_SUBTITLE,
                    details=f"Embedded subtitle found: language={target_lang}",
                )
        except Exception as e:
            logger.debug(f"Error checking embedded subtitles: {e}")

        return SkipResult(should_skip=False, reason=None, details="No matching embedded subtitles")

    def _check_external_subtitles(self, file_path: str) -> SkipResult:
        """Check if external subtitles exist in target language."""
        logger.debug(f"_check_external_subtitles: checking {file_path}")

        try:
            external_subs = scan_external_subtitles(file_path)
            logger.debug(
                f"  Found {len(external_subs)} external subtitles: {[s.path for s in external_subs]}"
            )

            if self.config.skip.skip_only_subgen_subtitles:
                external_subs = [s for s in external_subs if s.is_subgen_generated]

            target_lang = self.config.skip.skip_if_internal_subtitles_language

            if has_subtitle_language(external_subs, target_lang):
                details = f"External subtitle found: language={target_lang}"
                if self.config.skip.skip_only_subgen_subtitles:
                    details += " (subgen-generated only)"

                return SkipResult(
                    should_skip=True,
                    reason=SkipReason.EXTERNAL_SUBTITLE,
                    details=details,
                )
        except Exception as e:
            logger.debug(f"Error checking external subtitles: {e}")

        return SkipResult(should_skip=False, reason=None, details="No matching external subtitles")

    def _check_audio_language_skip(self, file_path: str) -> SkipResult:
        """Check if audio language is in skip list."""
        skip_languages = self.config.skip.get_skip_audio_languages()
        if not skip_languages:
            return SkipResult(
                should_skip=False, reason=None, details="No audio skip languages configured"
            )

        try:
            audio_tracks = get_audio_tracks(file_path)
            for track in audio_tracks:
                if track.language.lower() in [lang.lower() for lang in skip_languages]:
                    return SkipResult(
                        should_skip=True,
                        reason=SkipReason.AUDIO_LANGUAGE_SKIP,
                        details=f"Audio track language matches skip list: {track.language}",
                    )
        except Exception as e:
            logger.debug(f"Error checking audio language: {e}")

        return SkipResult(should_skip=False, reason=None, details="Audio language not in skip list")

    def _check_preferred_audio_language(self, file_path: str) -> SkipResult:
        """Check if audio language matches preferred list."""
        preferred_languages = self.config.skip.get_preferred_audio_languages()
        if not preferred_languages:
            return SkipResult(
                should_skip=False, reason=None, details="No preferred audio languages configured"
            )

        try:
            audio_tracks = get_audio_tracks(file_path)
            for track in audio_tracks:
                if track.language.lower() in [lang.lower() for lang in preferred_languages]:
                    return SkipResult(
                        should_skip=False,
                        reason=None,
                        details=f"Audio language matches preferred: {track.language}",
                    )

            return SkipResult(
                should_skip=True,
                reason=SkipReason.AUDIO_LANGUAGE_MISMATCH,
                details=f"Audio language doesn't match preferred list: {preferred_languages}",
            )
        except Exception as e:
            logger.debug(f"Error checking preferred audio language: {e}")

        return SkipResult(should_skip=False, reason=None, details="Error checking audio language")
