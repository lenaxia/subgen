package skip

import "context"

// SkipReason represents why a file should be skipped
type SkipReason string

const (
	// ReasonSubtitleExists indicates a subtitle file already exists
	ReasonSubtitleExists SkipReason = "subtitle_file_exists"
	// ReasonLRCExists indicates an LRC file already exists for audio
	ReasonLRCExists SkipReason = "lrc_file_exists"
	// ReasonNotApplicable indicates skip logic doesn't apply
	ReasonNotApplicable SkipReason = "not_applicable"
)

// CheckResult contains the result of a skip check
type CheckResult struct {
	// ShouldSkip indicates whether the file should be skipped
	ShouldSkip bool
	// Reason provides the skip reason constant
	Reason SkipReason
	// Details provides human-readable details about the skip decision
	Details string
}

// Checker defines the interface for skip logic implementations
type Checker interface {
	// Check determines if a file should be skipped
	// Returns CheckResult with skip decision and reason, or error if check fails
	Check(ctx context.Context, filePath string) (*CheckResult, error)

	// GetConfig returns the checker's configuration
	GetConfig() *Config
}
