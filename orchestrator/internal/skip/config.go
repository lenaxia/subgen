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
}

// NewConfig creates a Config from environment variables
// Reads SKIP_IF_TARGET_SUBTITLES_EXIST (default: true)
func NewConfig() (*Config, error) {
	skipStr := os.Getenv("SKIP_IF_TARGET_SUBTITLES_EXIST")
	if skipStr == "" {
		skipStr = "true" // Default to true
	}

	skip, err := strconv.ParseBool(skipStr)
	if err != nil {
		return nil, fmt.Errorf("invalid SKIP_IF_TARGET_SUBTITLES_EXIST value: %w", err)
	}

	return &Config{
		SkipIfTargetSubtitleExists: skip,
	}, nil
}

// Validate checks if the configuration is valid
// For basic config, all boolean values are valid, so this always returns nil
func (c *Config) Validate() error {
	// All boolean values are valid for this configuration
	return nil
}
