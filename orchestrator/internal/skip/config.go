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
}

// NewConfig creates a Config from environment variables
// Reads SKIP_IF_TARGET_SUBTITLES_EXIST (default: true)
// Reads CHECK_EMBEDDED_SUBTITLES (default: true)
// Reads SKIP_IF_INTERNAL_SUBTITLES_LANGUAGE (default: "eng")
// Reads SKIP_IF_EXTERNAL_SUBTITLES_EXIST (default: false)
// Reads SKIP_ONLY_SUBGEN_SUBTITLES (default: false)
// Reads SKIP_SUBTITLE_LANGUAGES (default: empty)
// Reads SKIP_IF_AUDIO_LANGUAGES (default: empty)
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

	return &Config{
		SkipIfTargetSubtitleExists:      skip,
		CheckEmbeddedSubtitles:          checkEmbedded,
		SkipIfInternalSubtitlesLanguage: skipInternalLang,
		SkipIfExternalSubtitlesExist:    skipExternal,
		SkipOnlySubgenSubtitles:         skipOnlySubgen,
		SkipSubtitleLanguages:           skipSubLangs,
		SkipIfAudioLanguages:            skipAudioLangs,
	}, nil
}

// Validate checks if the configuration is valid
// For basic config, all boolean values are valid, so this always returns nil
func (c *Config) Validate() error {
	// All boolean values are valid for this configuration
	return nil
}
