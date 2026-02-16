package formats

import (
	"fmt"
	"io"
	"strings"
)

// Segment represents a single subtitle segment with timing and text
type Segment struct {
	Start float64 // Start time in seconds
	End   float64 // End time in seconds
	Text  string  // Subtitle text
}

// Metadata contains information about the transcription
type Metadata struct {
	Language string  // ISO language code (e.g., "en")
	Duration float64 // Total duration in seconds
}

// Writer defines the interface for subtitle format writers
type Writer interface {
	// Write writes segments to the provided writer in the specific format
	Write(w io.Writer, segments []Segment, metadata Metadata) error
}

// NewWriter creates a new Writer instance for the specified format
// Supported formats: vtt, txt, tsv, json (case-insensitive)
// Returns error if format is not supported
func NewWriter(format string) (Writer, error) {
	// Normalize format to lowercase for case-insensitive comparison
	normalized := strings.ToLower(strings.TrimSpace(format))

	switch normalized {
	case "vtt":
		return &VTTWriter{}, nil
	case "txt":
		return &TXTWriter{}, nil
	case "tsv":
		return &TSVWriter{}, nil
	case "json":
		return &JSONWriter{}, nil
	default:
		return nil, fmt.Errorf("unsupported format: %s (supported: vtt, txt, tsv, json)", format)
	}
}
