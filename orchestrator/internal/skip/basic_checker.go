package skip

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

// BasicChecker implements basic file existence and embedded subtitle skip checks
type BasicChecker struct {
	config          *Config
	detector        *SubtitleDetector
	externalScanner *ExternalScanner
	audioDetector   *AudioDetector
	advancedChecker *AdvancedChecker
}

// NewBasicChecker creates a new BasicChecker with the given configuration
func NewBasicChecker(config *Config) (*BasicChecker, error) {
	if config == nil {
		return nil, fmt.Errorf("config cannot be nil")
	}

	if err := config.Validate(); err != nil {
		return nil, fmt.Errorf("invalid config: %w", err)
	}

	advancedChecker, err := NewAdvancedChecker(config)
	if err != nil {
		return nil, fmt.Errorf("failed to create advanced checker: %w", err)
	}

	return &BasicChecker{
		config:          config,
		detector:        NewSubtitleDetector(),
		externalScanner: NewExternalScanner(),
		audioDetector:   NewAudioDetector(),
		advancedChecker: advancedChecker,
	}, nil
}

// Check determines if a file should be skipped based on subtitle existence
func (c *BasicChecker) Check(ctx context.Context, filePath string) (*CheckResult, error) {
	if filePath == "" {
		return nil, fmt.Errorf("filePath cannot be empty")
	}

	// Check for SRT file (videos) - only if target subtitle check is enabled
	srtPath := getSubtitlePath(filePath, ".srt")
	if c.config.SkipIfTargetSubtitleExists && exists(srtPath) {
		return &CheckResult{
			ShouldSkip: true,
			Reason:     ReasonSubtitleExists,
			Details:    fmt.Sprintf("subtitle file exists: %s", srtPath),
		}, nil
	}

	// Check for LRC file (audio files) - only if target subtitle check is enabled
	if c.config.SkipIfTargetSubtitleExists && isAudioFile(filePath) {
		lrcPath := getSubtitlePath(filePath, ".lrc")
		if exists(lrcPath) {
			return &CheckResult{
				ShouldSkip: true,
				Reason:     ReasonLRCExists,
				Details:    fmt.Sprintf("LRC file exists: %s", lrcPath),
			}, nil
		}
	}

	// Check for embedded subtitles (if enabled and file is video)
	if c.config.CheckEmbeddedSubtitles && isVideoFile(filePath) && c.config.SkipIfInternalSubtitlesLanguage != "" {
		tracks, err := c.detector.GetEmbeddedSubtitles(ctx, filePath)
		if err != nil {
			// Log error but don't fail the check - FFprobe might not be available
			// or the file might be corrupted. We'll continue with other checks.
			// In production, this should be logged via structured logging.
		} else if c.detector.HasLanguage(tracks, c.config.SkipIfInternalSubtitlesLanguage) {
			return &CheckResult{
				ShouldSkip: true,
				Reason:     ReasonEmbeddedSubtitle,
				Details:    fmt.Sprintf("embedded subtitle found: language=%s", c.config.SkipIfInternalSubtitlesLanguage),
			}, nil
		}
	}

	// Check for external subtitles (if enabled)
	if c.config.SkipIfExternalSubtitlesExist {
		subtitles, err := c.externalScanner.ScanForSubtitles(filePath)
		if err != nil {
			// Log error but don't fail the check - directory might not be accessible
			// Continue with other checks
		} else {
			// Determine target language (use internal language config)
			targetLang := c.config.SkipIfInternalSubtitlesLanguage

			// Filter subtitles if SKIP_ONLY_SUBGEN_SUBTITLES is enabled
			var filteredSubtitles []ExternalSubtitle
			if c.config.SkipOnlySubgenSubtitles {
				for _, sub := range subtitles {
					if sub.IsSubgenGenerated {
						filteredSubtitles = append(filteredSubtitles, sub)
					}
				}
			} else {
				filteredSubtitles = subtitles
			}

			// Check if any filtered subtitle matches target language
			if c.externalScanner.HasLanguage(filteredSubtitles, targetLang) {
				details := fmt.Sprintf("external subtitle found: language=%s", targetLang)
				if c.config.SkipOnlySubgenSubtitles {
					details += " (subgen-generated only)"
				}

				return &CheckResult{
					ShouldSkip: true,
					Reason:     ReasonExternalSubtitle,
					Details:    details,
				}, nil
			}
		}
	}

	// Check audio language filtering (if enabled)
	if len(c.config.SkipIfAudioLanguages) > 0 && isVideoFile(filePath) {
		audioTracks, err := c.audioDetector.GetAudioTracks(ctx, filePath)
		if err != nil {
			// Log error but don't fail the check - FFprobe might not be available
			// Continue with other checks
		} else {
			for _, track := range audioTracks {
				if MatchesAnyLanguage(track.Language, c.config.SkipIfAudioLanguages) {
					return &CheckResult{
						ShouldSkip: true,
						Reason:     ReasonAudioLanguageSkip,
						Details:    fmt.Sprintf("audio track language matches skip list: %s", track.Language),
					}, nil
				}
			}
		}
	}

	// Check preferred audio language filtering (if enabled) - STORY_05
	if c.config.LimitToPreferredAudioLanguage && len(c.config.PreferredAudioLanguages) > 0 && isVideoFile(filePath) {
		audioTracks, err := c.audioDetector.GetAudioTracks(ctx, filePath)
		if err != nil {
			// Log error but don't fail the check - FFprobe might not be available
			// Continue with other checks
		} else {
			// Check if file has any preferred audio language
			hasPreferred := c.audioDetector.HasAnyPreferredLanguage(audioTracks, c.config.PreferredAudioLanguages)
			if !hasPreferred {
				return &CheckResult{
					ShouldSkip: true,
					Reason:     ReasonAudioLanguageMismatch,
					Details:    "no audio tracks match preferred languages",
				}, nil
			}
		}
	}

	// Check subtitle language filtering (if enabled)
	if len(c.config.SkipSubtitleLanguages) > 0 {
		// Check embedded subtitles for language filter
		if c.config.CheckEmbeddedSubtitles && isVideoFile(filePath) {
			tracks, err := c.detector.GetEmbeddedSubtitles(ctx, filePath)
			if err == nil {
				for _, track := range tracks {
					if MatchesAnyLanguage(track.Language, c.config.SkipSubtitleLanguages) {
						return &CheckResult{
							ShouldSkip: true,
							Reason:     ReasonSubtitleLanguageSkip,
							Details:    fmt.Sprintf("embedded subtitle language matches skip list: %s", track.Language),
						}, nil
					}
				}
			}
		}

		// Check external subtitles for language filter
		subtitles, err := c.externalScanner.ScanForSubtitles(filePath)
		if err == nil {
			for _, sub := range subtitles {
				if MatchesAnyLanguage(sub.Language, c.config.SkipSubtitleLanguages) {
					return &CheckResult{
						ShouldSkip: true,
						Reason:     ReasonSubtitleLanguageSkip,
						Details:    fmt.Sprintf("external subtitle language matches skip list: %s", sub.Language),
					}, nil
				}
			}
		}
	}

	// STORY_06: Check unknown language (if enabled)
	// Note: This requires language detection context that would typically be passed in
	// For now, we check if config is enabled. Real implementation would need detected language.
	// This is a placeholder for integration with language detection flow.
	targetLanguage := c.config.SkipIfInternalSubtitlesLanguage
	if shouldSkip, details := c.advancedChecker.CheckUnknownLanguage(targetLanguage); shouldSkip {
		return &CheckResult{ShouldSkip: true, Reason: ReasonUnknownLanguage, Details: details}, nil
	}

	// STORY_06: Check no language but subtitles exist
	// This check is used after language detection in the actual workflow
	// Determine if subtitles exist based on previous checks
	hasSubtitles := false

	// Check if basic subtitle files exist
	if exists(srtPath) {
		hasSubtitles = true
	}
	if isAudioFile(filePath) {
		lrcPath := getSubtitlePath(filePath, ".lrc")
		if exists(lrcPath) {
			hasSubtitles = true
		}
	}

	// Check for embedded subtitles (if enabled)
	if c.config.CheckEmbeddedSubtitles && isVideoFile(filePath) {
		tracks, err := c.detector.GetEmbeddedSubtitles(ctx, filePath)
		if err == nil && len(tracks) > 0 {
			hasSubtitles = true
		}
	}

	// Check for external subtitles
	externalSubs, err := c.externalScanner.ScanForSubtitles(filePath)
	if err == nil && len(externalSubs) > 0 {
		hasSubtitles = true
	}

	if shouldSkip, details := c.advancedChecker.CheckNoLanguageButSubtitlesExist(targetLanguage, hasSubtitles); shouldSkip {
		return &CheckResult{ShouldSkip: true, Reason: ReasonNoLanguageButSubtitlesExist, Details: details}, nil
	}

	return &CheckResult{
		ShouldSkip: false,
		Reason:     ReasonNotApplicable,
		Details:    "no subtitle file found",
	}, nil
}

// GetConfig returns the checker's configuration
func (c *BasicChecker) GetConfig() *Config {
	return c.config
}

// Helper functions

// exists checks if a file exists at the given path
func exists(path string) bool {
	_, err := os.Stat(path)
	return err == nil
}

// isAudioFile determines if a file is an audio file based on extension
func isAudioFile(filePath string) bool {
	ext := strings.ToLower(filepath.Ext(filePath))
	audioExts := []string{".mp3", ".m4a", ".flac", ".wav", ".aac", ".ogg", ".opus", ".wma"}

	for _, audioExt := range audioExts {
		if ext == audioExt {
			return true
		}
	}

	return false
}

// isVideoFile determines if a file is a video file based on extension
func isVideoFile(filePath string) bool {
	ext := strings.ToLower(filepath.Ext(filePath))
	videoExts := []string{".mkv", ".mp4", ".avi", ".mov", ".m4v", ".wmv", ".flv", ".webm", ".ts", ".m2ts", ".mpg", ".mpeg"}

	for _, videoExt := range videoExts {
		if ext == videoExt {
			return true
		}
	}

	return false
}

// getSubtitlePath returns the expected subtitle path for a media file
// It replaces the file extension with the subtitle extension
func getSubtitlePath(filePath string, subtitleExt string) string {
	base := strings.TrimSuffix(filePath, filepath.Ext(filePath))
	return base + subtitleExt
}
