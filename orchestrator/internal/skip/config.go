package skip

import (
	"fmt"
	"os"
	"strconv"
)

// Config holds skip checker configuration
type Config struct {
	// SkipIfTargetSubtitleExists determines whether to skip files
	// that already have subtitle files (.srt or .lrc)
	SkipIfTargetSubtitleExists bool
	// CheckEmbeddedSubtitles determines whether to check for embedded subtitles
	// in media containers (default: true)
	CheckEmbeddedSubtitles bool
	// SkipIfInternalSubtitlesLanguage specifies which language to skip if found embedded
	// Empty string disables this check (default: "eng")
	SkipIfInternalSubtitlesLanguage string
	// SkipIfExternalSubtitlesExist determines whether to skip if external subtitle files exist
	// (default: false)
	SkipIfExternalSubtitlesExist bool
	// SkipOnlySubgenSubtitles determines whether to only skip if subtitles are subgen-generated
	// (default: false)
	SkipOnlySubgenSubtitles bool
	// SkipSubtitleLanguages is a list of subtitle languages to skip (pipe-separated)
	// e.g., "eng|jpn|kor" (default: empty)
	SkipSubtitleLanguages []string
	// SkipIfAudioLanguages is a list of audio languages to skip (pipe-separated)
	// e.g., "eng|spa" (default: empty)
	SkipIfAudioLanguages []string
	// PreferredAudioLanguages is a list of preferred audio languages (pipe-separated)
	// e.g., "eng|jpn|kor" (default: empty) - STORY_05
	PreferredAudioLanguages []string
	// LimitToPreferredAudioLanguage determines whether to only process files with preferred audio
	// (default: false) - STORY_05
	LimitToPreferredAudioLanguage bool
	// SkipUnknownLanguage determines whether to skip files with unknown/undefined language
	// (default: false) - STORY_06
	SkipUnknownLanguage bool
	// SkipIfNoLanguageButSubtitlesExist determines whether to skip files when language cannot be detected
	// but subtitles already exist (default: false) - STORY_06
	SkipIfNoLanguageButSubtitlesExist bool
}

// NewConfig creates a Config from environment variables
// Reads SKIP_IF_TARGET_SUBTITLES_EXIST (default: true)
// Reads CHECK_EMBEDDED_SUBTITLES (default: true)
// Reads SKIP_IF_INTERNAL_SUBTITLES_LANGUAGE (default: "eng")
// Reads SKIP_IF_EXTERNAL_SUBTITLES_EXIST (default: false)
// Reads SKIP_ONLY_SUBGEN_SUBTITLES (default: false)
// Reads SKIP_SUBTITLE_LANGUAGES (default: empty)
// Reads SKIP_IF_AUDIO_LANGUAGES (default: empty)
// Reads PREFERRED_AUDIO_LANGUAGES (default: empty) - STORY_05
// Reads LIMIT_TO_PREFERRED_AUDIO_LANGUAGE (default: false) - STORY_05
// Reads SKIP_UNKNOWN_LANGUAGE (default: false) - STORY_06
// Reads SKIP_IF_NO_LANGUAGE_BUT_SUBTITLES_EXIST (default: false) - STORY_06
func NewConfig() (*Config, error) {
	skipStr := os.Getenv("SKIP_IF_TARGET_SUBTITLES_EXIST")
	if skipStr == "" {
		skipStr = "true" // Default to true
	}

	skip, err := strconv.ParseBool(skipStr)
	if err != nil {
		return nil, fmt.Errorf("invalid SKIP_IF_TARGET_SUBTITLES_EXIST value: %w", err)
	}

	// Check embedded subtitles (default: true)
	checkEmbeddedStr := os.Getenv("CHECK_EMBEDDED_SUBTITLES")
	if checkEmbeddedStr == "" {
		checkEmbeddedStr = "true"
	}

	checkEmbedded, err := strconv.ParseBool(checkEmbeddedStr)
	if err != nil {
		return nil, fmt.Errorf("invalid CHECK_EMBEDDED_SUBTITLES value: %w", err)
	}

	// Skip if internal subtitles language (default: "eng")
	skipInternalLang := os.Getenv("SKIP_IF_INTERNAL_SUBTITLES_LANGUAGE")
	if skipInternalLang == "" {
		skipInternalLang = "eng"
	}

	// Skip if external subtitles exist (default: false)
	skipExternalStr := os.Getenv("SKIP_IF_EXTERNAL_SUBTITLES_EXIST")
	if skipExternalStr == "" {
		skipExternalStr = "false"
	}

	skipExternal, err := strconv.ParseBool(skipExternalStr)
	if err != nil {
		return nil, fmt.Errorf("invalid SKIP_IF_EXTERNAL_SUBTITLES_EXIST value: %w", err)
	}

	// Skip only subgen subtitles (default: false)
	skipOnlySubgenStr := os.Getenv("SKIP_ONLY_SUBGEN_SUBTITLES")
	if skipOnlySubgenStr == "" {
		skipOnlySubgenStr = "false"
	}

	skipOnlySubgen, err := strconv.ParseBool(skipOnlySubgenStr)
	if err != nil {
		return nil, fmt.Errorf("invalid SKIP_ONLY_SUBGEN_SUBTITLES value: %w", err)
	}

	// Skip subtitle languages (default: empty)
	skipSubLangStr := os.Getenv("SKIP_SUBTITLE_LANGUAGES")
	skipSubLangs := ParseLanguageList(skipSubLangStr)

	// Skip if audio languages (default: empty)
	skipAudioLangStr := os.Getenv("SKIP_IF_AUDIO_LANGUAGES")
	skipAudioLangs := ParseLanguageList(skipAudioLangStr)

	// Preferred audio languages (default: empty) - STORY_05
	preferredAudioLangStr := os.Getenv("PREFERRED_AUDIO_LANGUAGES")
	preferredAudioLangs := ParseLanguageList(preferredAudioLangStr)

	// Limit to preferred audio language (default: false) - STORY_05
	limitToPreferredStr := os.Getenv("LIMIT_TO_PREFERRED_AUDIO_LANGUAGE")
	if limitToPreferredStr == "" {
		limitToPreferredStr = "false"
	}

	limitToPreferred, err := strconv.ParseBool(limitToPreferredStr)
	if err != nil {
		return nil, fmt.Errorf("invalid LIMIT_TO_PREFERRED_AUDIO_LANGUAGE value: %w", err)
	}

	// Skip unknown language (default: false) - STORY_06
	skipUnknownLangStr := os.Getenv("SKIP_UNKNOWN_LANGUAGE")
	if skipUnknownLangStr == "" {
		skipUnknownLangStr = "false"
	}

	skipUnknownLang, err := strconv.ParseBool(skipUnknownLangStr)
	if err != nil {
		return nil, fmt.Errorf("invalid SKIP_UNKNOWN_LANGUAGE value: %w", err)
	}

	// Skip if no language but subtitles exist (default: false) - STORY_06
	skipNoLangButSubsStr := os.Getenv("SKIP_IF_NO_LANGUAGE_BUT_SUBTITLES_EXIST")
	if skipNoLangButSubsStr == "" {
		skipNoLangButSubsStr = "false"
	}

	skipNoLangButSubs, err := strconv.ParseBool(skipNoLangButSubsStr)
	if err != nil {
		return nil, fmt.Errorf("invalid SKIP_IF_NO_LANGUAGE_BUT_SUBTITLES_EXIST value: %w", err)
	}

	return &Config{
		SkipIfTargetSubtitleExists:        skip,
		CheckEmbeddedSubtitles:            checkEmbedded,
		SkipIfInternalSubtitlesLanguage:   skipInternalLang,
		SkipIfExternalSubtitlesExist:      skipExternal,
		SkipOnlySubgenSubtitles:           skipOnlySubgen,
		SkipSubtitleLanguages:             skipSubLangs,
		SkipIfAudioLanguages:              skipAudioLangs,
		PreferredAudioLanguages:           preferredAudioLangs,
		LimitToPreferredAudioLanguage:     limitToPreferred,
		SkipUnknownLanguage:               skipUnknownLang,
		SkipIfNoLanguageButSubtitlesExist: skipNoLangButSubs,
	}, nil
}

// Validate checks if the configuration is valid
// For basic config, all boolean values are valid, so this always returns nil
func (c *Config) Validate() error {
	// All boolean values are valid for this configuration
	return nil
}
