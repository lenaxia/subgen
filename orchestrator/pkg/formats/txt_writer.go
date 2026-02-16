package formats

import (
	"fmt"
	"io"
)

// TXTWriter implements Writer interface for plain text format
type TXTWriter struct{}

// Write writes segments in plain text format (no timestamps)
// Each segment's text is written on its own line
func (w *TXTWriter) Write(writer io.Writer, segments []Segment, metadata Metadata) error {
	if writer == nil {
		return fmt.Errorf("writer cannot be nil")
	}

	// Write each segment's text on its own line
	for _, seg := range segments {
		if _, err := fmt.Fprintf(writer, "%s\n", seg.Text); err != nil {
			return fmt.Errorf("failed to write text: %w", err)
		}
	}

	return nil
}
