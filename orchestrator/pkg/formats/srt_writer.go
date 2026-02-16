package formats

import (
	"fmt"
	"io"
)

// SRTWriter implements Writer interface for SubRip (SRT) format
type SRTWriter struct{}

// Write writes segments in SRT format
// Format: Sequential numbering followed by timestamp and text
// Timestamp format: HH:MM:SS,mmm --> HH:MM:SS,mmm
func (w *SRTWriter) Write(writer io.Writer, segments []Segment, metadata Metadata) error {
	if writer == nil {
		return fmt.Errorf("writer cannot be nil")
	}

	// Write each segment with sequential numbering
	for i, seg := range segments {
		// Write sequence number (1-based)
		if _, err := fmt.Fprintf(writer, "%d\n", i+1); err != nil {
			return fmt.Errorf("failed to write sequence number: %w", err)
		}

		// Format: HH:MM:SS,mmm --> HH:MM:SS,mmm
		startTime := formatSRTTimestamp(seg.Start)
		endTime := formatSRTTimestamp(seg.End)

		// Write timestamp line
		if _, err := fmt.Fprintf(writer, "%s --> %s\n", startTime, endTime); err != nil {
			return fmt.Errorf("failed to write timestamp: %w", err)
		}

		// Write text line
		if _, err := fmt.Fprintf(writer, "%s\n\n", seg.Text); err != nil {
			return fmt.Errorf("failed to write text: %w", err)
		}
	}

	return nil
}

// formatSRTTimestamp converts seconds to SRT timestamp format (HH:MM:SS,mmm)
// Note: SRT uses comma instead of period for milliseconds
func formatSRTTimestamp(seconds float64) string {
	hours := int(seconds) / 3600
	minutes := (int(seconds) % 3600) / 60
	secs := int(seconds) % 60
	millis := int((seconds - float64(int(seconds))) * 1000)

	return fmt.Sprintf("%02d:%02d:%02d,%03d", hours, minutes, secs, millis)
}
