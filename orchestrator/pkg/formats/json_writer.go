package formats

import (
	"encoding/json"
	"fmt"
	"io"
)

// JSONWriter implements Writer interface for JSON format
type JSONWriter struct{}

// jsonOutput represents the JSON structure for subtitle output
type jsonOutput struct {
	Language string        `json:"language"`
	Duration float64       `json:"duration"`
	Segments []jsonSegment `json:"segments"`
}

// jsonSegment represents a single segment in JSON output
type jsonSegment struct {
	Start float64 `json:"start"`
	End   float64 `json:"end"`
	Text  string  `json:"text"`
}

// Write writes segments in JSON format
// Format: Structured JSON with language, duration, and segments array
func (w *JSONWriter) Write(writer io.Writer, segments []Segment, metadata Metadata) error {
	if writer == nil {
		return fmt.Errorf("writer cannot be nil")
	}

	// Convert segments to JSON segments
	jsonSegments := make([]jsonSegment, len(segments))
	for i, seg := range segments {
		jsonSegments[i] = jsonSegment{
			Start: seg.Start,
			End:   seg.End,
			Text:  seg.Text,
		}
	}

	// Create output structure
	output := jsonOutput{
		Language: metadata.Language,
		Duration: metadata.Duration,
		Segments: jsonSegments,
	}

	// Marshal to JSON with indentation (pretty print)
	encoder := json.NewEncoder(writer)
	encoder.SetIndent("", "  ")

	if err := encoder.Encode(output); err != nil {
		return fmt.Errorf("failed to encode JSON: %w", err)
	}

	return nil
}
