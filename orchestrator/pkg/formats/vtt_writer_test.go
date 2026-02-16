package formats

import (
	"bytes"
	"strings"
	"testing"
)

// TestVTTWriter_HappyPath tests normal VTT output
func TestVTTWriter_HappyPath(t *testing.T) {
	writer := &VTTWriter{}
	var buf bytes.Buffer

	err := writer.Write(&buf, testSegments, testMetadata)
	if err != nil {
		t.Fatalf("VTTWriter.Write() failed: %v", err)
	}

	output := buf.String()

	// Check for WebVTT header
	if !strings.HasPrefix(output, "WEBVTT\n\n") {
		t.Errorf("VTT output should start with 'WEBVTT\\n\\n', got: %q", output[:min(20, len(output))])
	}

	// Check that all segments are present
	for _, seg := range testSegments {
		if !strings.Contains(output, seg.Text) {
			t.Errorf("VTT output missing segment text: %q", seg.Text)
		}
	}

	// Check for timestamp format (HH:MM:SS.mmm --> HH:MM:SS.mmm)
	if !strings.Contains(output, "00:00:00.000 --> 00:00:03.200") {
		t.Errorf("VTT output missing expected timestamp format")
	}
}

// TestVTTWriter_EmptySegments tests VTT with no segments
func TestVTTWriter_EmptySegments(t *testing.T) {
	writer := &VTTWriter{}
	var buf bytes.Buffer

	err := writer.Write(&buf, []Segment{}, testMetadata)
	if err != nil {
		t.Fatalf("VTTWriter.Write() should not error on empty segments: %v", err)
	}

	output := buf.String()

	// Should still have header
	if !strings.HasPrefix(output, "WEBVTT\n\n") {
		t.Errorf("VTT output should have header even with no segments")
	}
}

// TestVTTWriter_SpecialCharacters tests VTT with special characters
func TestVTTWriter_SpecialCharacters(t *testing.T) {
	writer := &VTTWriter{}
	var buf bytes.Buffer

	segments := []Segment{
		{Start: 0.0, End: 2.0, Text: "Text with <b>HTML</b> tags"},
		{Start: 2.5, End: 4.5, Text: "Text with & ampersand"},
		{Start: 5.0, End: 7.0, Text: "Text with \"quotes\" and 'apostrophes'"},
	}

	err := writer.Write(&buf, segments, testMetadata)
	if err != nil {
		t.Fatalf("VTTWriter.Write() failed: %v", err)
	}

	output := buf.String()

	// All text should be preserved (VTT supports HTML-like tags)
	for _, seg := range segments {
		if !strings.Contains(output, seg.Text) {
			t.Errorf("VTT output missing segment text: %q", seg.Text)
		}
	}
}

// TestVTTWriter_MultilineText tests VTT with multiline text in segments
func TestVTTWriter_MultilineText(t *testing.T) {
	writer := &VTTWriter{}
	var buf bytes.Buffer

	segments := []Segment{
		{Start: 0.0, End: 2.0, Text: "Line one\nLine two"},
		{Start: 2.5, End: 4.5, Text: "Single line"},
	}

	err := writer.Write(&buf, segments, testMetadata)
	if err != nil {
		t.Fatalf("VTTWriter.Write() failed: %v", err)
	}

	output := buf.String()

	// Multiline text should be preserved
	if !strings.Contains(output, "Line one\nLine two") {
		t.Errorf("VTT output should preserve multiline text")
	}
}

// TestVTTWriter_TimestampFormatting tests correct timestamp formatting
func TestVTTWriter_TimestampFormatting(t *testing.T) {
	writer := &VTTWriter{}
	var buf bytes.Buffer

	segments := []Segment{
		{Start: 0.0, End: 1.0, Text: "Zero start"},
		{Start: 61.5, End: 125.750, Text: "Over one minute"},
		{Start: 3661.123, End: 3665.456, Text: "Over one hour"},
	}

	err := writer.Write(&buf, segments, testMetadata)
	if err != nil {
		t.Fatalf("VTTWriter.Write() failed: %v", err)
	}

	output := buf.String()

	// Check specific timestamp formats
	expectedTimestamps := []string{
		"00:00:00.000 --> 00:00:01.000",
		"00:01:01.500 --> 00:02:05.750",
		"01:01:01.123 --> 01:01:05.456",
	}

	for _, expected := range expectedTimestamps {
		if !strings.Contains(output, expected) {
			t.Errorf("VTT output should contain timestamp %q, got:\n%s", expected, output)
		}
	}
}

// TestVTTWriter_NilWriter tests error handling for nil writer
func TestVTTWriter_NilWriter(t *testing.T) {
	writer := &VTTWriter{}

	err := writer.Write(nil, testSegments, testMetadata)
	if err == nil {
		t.Errorf("VTTWriter.Write() should return error for nil writer")
	}
}

// Helper function for min (for Go < 1.21)
func min(a, b int) int {
	if a < b {
		return a
	}
	return b
}
