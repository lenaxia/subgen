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
}

// NewBasicChecker creates a new BasicChecker with the given configuration
func NewBasicChecker(config *Config) (*BasicChecker, error) {
	if config == nil {
		return nil, fmt.Errorf("config cannot be nil")
	}

	if err := config.Validate(); err != nil {
		return nil, fmt.Errorf("invalid config: %w", err)
	}

	return &BasicChecker{
		config:          config,
		detector:        NewSubtitleDetector(),
		externalScanner: NewExternalScanner(),
	}, nil
}

// Check determines if a file should be skipped based on subtitle existence
func (c *BasicChecker) Check(ctx context.Context, filePath string) (*CheckResult, error) {
	if filePath == "" {
		return nil, fmt.Errorf("filePath cannot be empty")
	}

	// If skip is disabled, never skip
	if !c.config.SkipIfTargetSubtitleExists {
		return &CheckResult{
			ShouldSkip: false,
			Reason:     ReasonNotApplicable,
			Details:    "skip checking disabled",
		}, nil
	}

	// Check for SRT file (videos)
	srtPath := getSubtitlePath(filePath, ".srt")
	if exists(srtPath) {
		return &CheckResult{
			ShouldSkip: true,
			Reason:     ReasonSubtitleExists,
			Details:    fmt.Sprintf("subtitle file exists: %s", srtPath),
		}, nil
	}

	// Check for LRC file (audio files)
	if isAudioFile(filePath) {
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
