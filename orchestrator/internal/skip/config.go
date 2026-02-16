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
}

// NewConfig creates a Config from environment variables
// Reads SKIP_IF_TARGET_SUBTITLES_EXIST (default: true)
// Reads CHECK_EMBEDDED_SUBTITLES (default: true)
// Reads SKIP_IF_INTERNAL_SUBTITLES_LANGUAGE (default: "eng")
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

	return &Config{
		SkipIfTargetSubtitleExists:      skip,
		CheckEmbeddedSubtitles:          checkEmbedded,
		SkipIfInternalSubtitlesLanguage: skipInternalLang,
	}, nil
}

// Validate checks if the configuration is valid
// For basic config, all boolean values are valid, so this always returns nil
func (c *Config) Validate() error {
	// All boolean values are valid for this configuration
	return nil
}
