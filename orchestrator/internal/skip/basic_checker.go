package skip

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

// BasicChecker implements basic file existence skip checks
type BasicChecker struct {
	config *Config
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
		config: config,
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

// getSubtitlePath returns the expected subtitle path for a media file
// It replaces the file extension with the subtitle extension
func getSubtitlePath(filePath string, subtitleExt string) string {
	base := strings.TrimSuffix(filePath, filepath.Ext(filePath))
	return base + subtitleExt
}
