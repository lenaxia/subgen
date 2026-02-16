package formats

import (
	"fmt"
	"io"
	"strings"
)

// TSVWriter implements Writer interface for tab-separated values format
type TSVWriter struct{}

// Write writes segments in TSV format (tab-separated values)
// Format: Header row (start, end, text) followed by data rows
// Tabs and newlines in text are replaced with spaces to maintain TSV structure
func (w *TSVWriter) Write(writer io.Writer, segments []Segment, metadata Metadata) error {
	if writer == nil {
		return fmt.Errorf("writer cannot be nil")
	}

	// Write header row
	if _, err := fmt.Fprintf(writer, "start\tend\ttext\n"); err != nil {
		return fmt.Errorf("failed to write TSV header: %w", err)
	}

	// Write each segment as a data row
	for _, seg := range segments {
		// Escape text: replace tabs and newlines with spaces
		text := escapeTSVText(seg.Text)

		// Format timestamps with 3 decimal places
		if _, err := fmt.Fprintf(writer, "%.3f\t%.3f\t%s\n", seg.Start, seg.End, text); err != nil {
			return fmt.Errorf("failed to write TSV row: %w", err)
		}
	}

	return nil
}

// escapeTSVText escapes text for TSV format by replacing tabs and newlines with spaces
func escapeTSVText(text string) string {
	// Replace tabs with spaces
	text = strings.ReplaceAll(text, "\t", " ")
	// Replace newlines with spaces
	text = strings.ReplaceAll(text, "\n", " ")
	text = strings.ReplaceAll(text, "\r", " ")
	return text
}
