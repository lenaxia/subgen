"""
Language policy module for determining output languages.

Implements the logic for deciding whether to transcribe or translate
based on audio language and user preferences.
"""

import logging
from typing import List, Tuple, Optional, Any
from dataclasses import dataclass
from enum import Enum

logger = logging.getLogger(__name__)


class OutputType(Enum):
    """Type of output for a language."""

    TRANSCRIBE = "transcribe"
    TRANSLATE = "translate"


@dataclass
class OutputLanguage:
    """Represents a single output language task."""

    language: str
    output_type: OutputType


def determine_output_languages(
    audio_language: str,
    target_languages: List[str],
    preferred_languages: List[str],
    transcribe_preferred: bool,
    force_language: Optional[str] = None,
) -> List[OutputLanguage]:
    """
    Determine output languages based on audio language and policy.

    Args:
        audio_language: Detected or forced audio language (ISO 639-1)
        target_languages: List of target output languages
        preferred_languages: List of preferred audio languages
        transcribe_preferred: Whether to transcribe when audio matches preferred
        force_language: Force-detected language (if already known)

    Returns:
        List of OutputLanguage objects representing what to generate

    Policy:
        1. If transcribe_preferred is True and audio in preferred_languages:
           - Add transcribe task for audio language
        2. For each language in target_languages:
           - If different from audio language: add translate task
           - If same as audio language: skip (already covered by transcribe)
        3. If no target_languages specified (backward compat):
           - Single transcribe task for audio language
    """
    outputs: List[OutputLanguage] = []

    audio_lang = force_language if force_language else audio_language
    audio_lang_lower = audio_lang.lower() if audio_lang else ""
    preferred_lower = [lang.lower() for lang in preferred_languages]
    target_lower = [lang.lower() for lang in target_languages]

    if not target_languages:
        if not audio_lang_lower:
            logger.warning("No audio language detected and no target languages specified")
            return []
        outputs.append(OutputLanguage(language=audio_lang_lower, output_type=OutputType.TRANSCRIBE))
        logger.info(f"Backward compat mode: single transcribe for {audio_lang_lower}")
        return outputs

    if transcribe_preferred and audio_lang_lower in preferred_lower:
        outputs.append(OutputLanguage(language=audio_lang_lower, output_type=OutputType.TRANSCRIBE))
        logger.info(f"Audio {audio_lang_lower} is preferred: transcribe")

    for target_lang in target_lower:
        if target_lang == audio_lang_lower:
            if not transcribe_preferred or audio_lang_lower not in preferred_lower:
                outputs.append(
                    OutputLanguage(language=target_lang, output_type=OutputType.TRANSLATE)
                )
                logger.info(f"Target {target_lang} same as audio: translate (for consistency)")
        else:
            outputs.append(OutputLanguage(language=target_lang, output_type=OutputType.TRANSLATE))
            logger.info(f"Target {target_lang} different from audio {audio_lang_lower}: translate")

    if not outputs:
        if audio_lang_lower:
            outputs.append(
                OutputLanguage(language=audio_lang_lower, output_type=OutputType.TRANSCRIBE)
            )
            logger.info(f"No matching policy, defaulting to transcribe {audio_lang_lower}")

    return outputs


def should_skip_output_language(
    skip_checker: Any,
    file_path: str,
    output_language: str,
) -> bool:
    """
    Check if subtitle should be skipped for specific output language.

    Args:
        skip_checker: SkipChecker instance
        file_path: Path to media file
        output_language: Target output language to check

    Returns:
        True if should skip, False otherwise
    """
    import os
    import glob

    base = os.path.splitext(file_path)[0]

    patterns = [
        f"{base}.subgen.*.{output_language}.srt",
        f"{base}.subgen.*.{output_language}.lrc",
        f"{base}.{output_language}.subgen.*.srt",
        f"{base}.{output_language}.srt",
        f"{base}.{output_language}.lrc",
    ]

    for pattern in patterns:
        matches = glob.glob(pattern)
        if matches:
            logger.info(f"Skipping {output_language}: existing subtitle {matches[0]}")
            return True

    return False
