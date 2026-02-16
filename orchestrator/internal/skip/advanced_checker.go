package skip

import (
	"fmt"
)

// AdvancedChecker implements advanced skip conditions
type AdvancedChecker struct {
	config *Config
}

// NewAdvancedChecker creates a new AdvancedChecker
func NewAdvancedChecker(config *Config) (*AdvancedChecker, error) {
	if config == nil {
		return nil, fmt.Errorf("config cannot be nil")
	}

	return &AdvancedChecker{
		config: config,
	}, nil
}

// CheckUnknownLanguage checks if file should be skipped due to unknown language
// Returns (shouldSkip, details)
func (c *AdvancedChecker) CheckUnknownLanguage(detectedLang string) (bool, string) {
	// Only apply if SKIP_UNKNOWN_LANGUAGE is enabled
	if !c.config.SkipUnknownLanguage {
		return false, ""
	}

	// Consider empty string, "unknown", "undefined", "und" as unknown
	if IsUnknownLanguage(detectedLang) {
		return true, fmt.Sprintf("language detection returned unknown: %q", detectedLang)
	}

	return false, ""
}

// CheckNoLanguageButSubtitlesExist checks if file should be skipped when:
// - Language cannot be detected (empty/unknown)
// - But subtitles already exist (embedded or external)
// This prevents redundant processing when we can't detect language but subs exist
func (c *AdvancedChecker) CheckNoLanguageButSubtitlesExist(detectedLang string, hasSubtitles bool) (bool, string) {
	// Only apply if flag is enabled
	if !c.config.SkipIfNoLanguageButSubtitlesExist {
		return false, ""
	}

	// Check if language is unknown/empty
	isUnknown := IsUnknownLanguage(detectedLang)

	// Skip if language is unknown AND subtitles exist
	if isUnknown && hasSubtitles {
		return true, fmt.Sprintf("no language detected but subtitles exist (lang=%q)", detectedLang)
	}

	return false, ""
}

// IsUnknownLanguage is a helper to check if a language string represents unknown/undefined
// Returns true for: empty string, "unknown", "undefined", "und"
// Case sensitive (lowercase only)
func IsUnknownLanguage(lang string) bool {
	return lang == "" ||
		lang == "unknown" ||
		lang == "undefined" ||
		lang == "und"
}
