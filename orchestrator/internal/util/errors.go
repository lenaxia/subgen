// Package util provides utility functions for error handling with troubleshooting hints
package util

import (
	"fmt"
	"strings"
)

// ErrorWithHint wraps an error with troubleshooting hints
type ErrorWithHint struct {
	Err   error
	Hints []string
}

// Error implements the error interface
func (e *ErrorWithHint) Error() string {
	var sb strings.Builder
	sb.WriteString(e.Err.Error())
	if len(e.Hints) > 0 {
		sb.WriteString("\nTroubleshooting:")
		for _, hint := range e.Hints {
			sb.WriteString("\n  - ")
			sb.WriteString(hint)
		}
	}
	return sb.String()
}

// Unwrap returns the wrapped error
func (e *ErrorWithHint) Unwrap() error {
	return e.Err
}

// NewErrorWithHints creates an error with troubleshooting hints
func NewErrorWithHints(err error, hints ...string) error {
	return &ErrorWithHint{
		Err:   err,
		Hints: hints,
	}
}

// AudioExtractionError creates an error for audio extraction failures
func AudioExtractionError(filePath string, err error) error {
	return NewErrorWithHints(
		fmt.Errorf("audio extraction failed: %w", err),
		fmt.Sprintf("Verify file has audio tracks: ffprobe %s", filePath),
		"Check file is not corrupted",
		"Try re-encoding with: ffmpeg -i input.mkv -c copy output.mkv",
	)
}

// WorkerConnectionError creates an error for worker connection failures
func WorkerConnectionError(address string, err error) error {
	return NewErrorWithHints(
		fmt.Errorf("failed to connect to worker at %s: %w", address, err),
		"Verify worker is running and healthy",
		fmt.Sprintf("Check network connectivity: nc -zv %s", address),
		"Review worker logs for startup errors",
		"Ensure gRPC port is not blocked by firewall",
	)
}

// PathMappingError creates an error for path mapping issues
func PathMappingError(originalPath, mappedPath string) error {
	return NewErrorWithHints(
		fmt.Errorf("mapped path does not exist: %s -> %s", originalPath, mappedPath),
		"Verify PATH_MAPPING_FROM and PATH_MAPPING_TO are configured correctly",
		fmt.Sprintf("Check if path exists: ls -la %s", mappedPath),
		"Ensure Docker volume mounts match path mappings",
	)
}

// FileNotFoundError creates an error for missing files
func FileNotFoundError(filePath string) error {
	return NewErrorWithHints(
		fmt.Errorf("file not found: %s", filePath),
		"Verify file path is correct",
		"Check Subgen has read access to file",
		"If using path mapping, verify PATH_MAPPING_TO is correct",
	)
}

// WorkerTimeoutError creates an error for worker timeout
func WorkerTimeoutError(filePath string, duration string) error {
	return NewErrorWithHints(
		fmt.Errorf("worker timeout: transcription of %s exceeded %s", filePath, duration),
		"Check worker is running and responsive",
		"Increase timeout in configuration (WORKER_TIMEOUT)",
		"Check GPU/CPU usage on worker",
		"For long videos, consider increasing MODEL_CLEANUP_DELAY",
	)
}

// QueueFullError creates an error when queue is full
func QueueFullError(maxSize int) error {
	return NewErrorWithHints(
		fmt.Errorf("queue is full: maximum size of %d reached", maxSize),
		fmt.Sprintf("Increase queue size (QUEUE_MAX_SIZE > %d)", maxSize),
		"Add more workers to process tasks faster",
		"Check if workers are healthy and processing tasks",
	)
}

// InvalidLanguageCodeError creates an error for invalid language codes
func InvalidLanguageCodeError(code string) error {
	return NewErrorWithHints(
		fmt.Errorf("invalid language code: %s", code),
		"Use ISO 639-1 (2-letter) or ISO 639-2 (3-letter) codes",
		"Examples: en, eng, es, spa, fr, fra",
		"Check configuration: SUBTITLE_LANGUAGE_NAME",
	)
}
