package formats

import (
	"fmt"
	"io"
)

// VTTWriter implements Writer interface for WebVTT format
type VTTWriter struct{}

// Write writes segments in WebVTT format
// Format: WEBVTT header followed by cues with timestamps
// Timestamp format: HH:MM:SS.mmm --> HH:MM:SS.mmm
func (w *VTTWriter) Write(writer io.Writer, segments []Segment, metadata Metadata) error {
	if writer == nil {
		return fmt.Errorf("writer cannot be nil")
	}

	// Write WebVTT header
	if _, err := fmt.Fprintf(writer, "WEBVTT\n\n"); err != nil {
		return fmt.Errorf("failed to write VTT header: %w", err)
	}

	// Write each segment as a cue
	for _, seg := range segments {
		// Format: HH:MM:SS.mmm --> HH:MM:SS.mmm
		startTime := formatVTTTimestamp(seg.Start)
		endTime := formatVTTTimestamp(seg.End)

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

// formatVTTTimestamp converts seconds to VTT timestamp format (HH:MM:SS.mmm)
func formatVTTTimestamp(seconds float64) string {
	hours := int(seconds) / 3600
	minutes := (int(seconds) % 3600) / 60
	secs := int(seconds) % 60
	millis := int((seconds - float64(int(seconds))) * 1000)

	return fmt.Sprintf("%02d:%02d:%02d.%03d", hours, minutes, secs, millis)
}
