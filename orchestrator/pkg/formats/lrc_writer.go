package formats

import (
	"fmt"
	"io"
)

// LRCWriter implements Writer interface for LRC (Lyric) format
type LRCWriter struct{}

// Write writes segments in LRC format
// Format: [MM:SS.xx]text
// Timestamp format: [MM:SS.xx] where xx is centiseconds
func (w *LRCWriter) Write(writer io.Writer, segments []Segment, metadata Metadata) error {
	if writer == nil {
		return fmt.Errorf("writer cannot be nil")
	}

	// Write metadata headers if available
	if metadata.Language != "" {
		if _, err := fmt.Fprintf(writer, "[la:%s]\n", metadata.Language); err != nil {
			return fmt.Errorf("failed to write language header: %w", err)
		}
	}

	// Write each segment
	for _, seg := range segments {
		// Format: [MM:SS.xx]text
		timestamp := formatLRCTimestamp(seg.Start)

		// Write timestamp and text on same line
		if _, err := fmt.Fprintf(writer, "%s%s\n", timestamp, seg.Text); err != nil {
			return fmt.Errorf("failed to write segment: %w", err)
		}
	}

	return nil
}

// formatLRCTimestamp converts seconds to LRC timestamp format [MM:SS.xx]
// where xx is centiseconds (hundredths of a second)
func formatLRCTimestamp(seconds float64) string {
	minutes := int(seconds) / 60
	secs := int(seconds) % 60
	centiseconds := int((seconds - float64(int(seconds))) * 100)

	return fmt.Sprintf("[%02d:%02d.%02d]", minutes, secs, centiseconds)
}
