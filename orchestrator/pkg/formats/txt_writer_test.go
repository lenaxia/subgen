package formats

import (
	"bytes"
	"strings"
	"testing"
)

// TestTXTWriter_HappyPath tests normal TXT output
func TestTXTWriter_HappyPath(t *testing.T) {
	writer := &TXTWriter{}
	var buf bytes.Buffer

	err := writer.Write(&buf, testSegments, testMetadata)
	if err != nil {
		t.Fatalf("TXTWriter.Write() failed: %v", err)
	}

	output := buf.String()

	// Check that all segments are present
	for _, seg := range testSegments {
		if !strings.Contains(output, seg.Text) {
			t.Errorf("TXT output missing segment text: %q", seg.Text)
		}
	}

	// Check no timestamps present
	if strings.Contains(output, "00:00:") || strings.Contains(output, "0.0") {
		t.Errorf("TXT output should not contain timestamps, got: %q", output)
	}

	// Each segment should be on its own line
	lines := strings.Split(strings.TrimSpace(output), "\n")
	if len(lines) != len(testSegments) {
		t.Errorf("Expected %d lines, got %d", len(testSegments), len(lines))
	}
}

// TestTXTWriter_EmptySegments tests TXT with no segments
func TestTXTWriter_EmptySegments(t *testing.T) {
	writer := &TXTWriter{}
	var buf bytes.Buffer

	err := writer.Write(&buf, []Segment{}, testMetadata)
	if err != nil {
		t.Fatalf("TXTWriter.Write() should not error on empty segments: %v", err)
	}

	output := buf.String()

	// Should be empty or just whitespace
	if strings.TrimSpace(output) != "" {
		t.Errorf("TXT output with empty segments should be empty, got: %q", output)
	}
}

// TestTXTWriter_SpecialCharacters tests TXT with special characters
func TestTXTWriter_SpecialCharacters(t *testing.T) {
	writer := &TXTWriter{}
	var buf bytes.Buffer

	segments := []Segment{
		{Start: 0.0, End: 2.0, Text: "Text with <b>HTML</b> tags"},
		{Start: 2.5, End: 4.5, Text: "Text with & ampersand"},
		{Start: 5.0, End: 7.0, Text: "Text with \"quotes\" and 'apostrophes'"},
		{Start: 7.5, End: 9.5, Text: "Unicode: émojis 😀 中文"},
	}

	err := writer.Write(&buf, segments, testMetadata)
	if err != nil {
		t.Fatalf("TXTWriter.Write() failed: %v", err)
	}

	output := buf.String()

	// All text should be preserved exactly
	for _, seg := range segments {
		if !strings.Contains(output, seg.Text) {
			t.Errorf("TXT output missing segment text: %q", seg.Text)
		}
	}
}

// TestTXTWriter_MultilineText tests TXT with multiline text
func TestTXTWriter_MultilineText(t *testing.T) {
	writer := &TXTWriter{}
	var buf bytes.Buffer

	segments := []Segment{
		{Start: 0.0, End: 2.0, Text: "Line one\nLine two"},
		{Start: 2.5, End: 4.5, Text: "Single line"},
		{Start: 5.0, End: 7.0, Text: "Another\nmultiline\ntext"},
	}

	err := writer.Write(&buf, segments, testMetadata)
	if err != nil {
		t.Fatalf("TXTWriter.Write() failed: %v", err)
	}

	output := buf.String()

	// Multiline text should be preserved
	for _, seg := range segments {
		if !strings.Contains(output, seg.Text) {
			t.Errorf("TXT output should preserve multiline text: %q", seg.Text)
		}
	}
}

// TestTXTWriter_EmptyTextSegments tests TXT with empty text in some segments
func TestTXTWriter_EmptyTextSegments(t *testing.T) {
	writer := &TXTWriter{}
	var buf bytes.Buffer

	segments := []Segment{
		{Start: 0.0, End: 2.0, Text: "First line"},
		{Start: 2.5, End: 4.5, Text: ""}, // Empty text
		{Start: 5.0, End: 7.0, Text: "Third line"},
	}

	err := writer.Write(&buf, segments, testMetadata)
	if err != nil {
		t.Fatalf("TXTWriter.Write() failed: %v", err)
	}

	output := buf.String()

	// Should contain non-empty segments
	if !strings.Contains(output, "First line") {
		t.Errorf("TXT output missing 'First line'")
	}
	if !strings.Contains(output, "Third line") {
		t.Errorf("TXT output missing 'Third line'")
	}

	// Empty text segments should result in empty lines
	lines := strings.Split(strings.TrimSpace(output), "\n")
	hasEmptyLine := false
	for _, line := range lines {
		if strings.TrimSpace(line) == "" {
			hasEmptyLine = true
			break
		}
	}
	if !hasEmptyLine {
		t.Errorf("TXT output should include empty line for empty text segment")
	}
}

// TestTXTWriter_NilWriter tests error handling for nil writer
func TestTXTWriter_NilWriter(t *testing.T) {
	writer := &TXTWriter{}

	err := writer.Write(nil, testSegments, testMetadata)
	if err == nil {
		t.Errorf("TXTWriter.Write() should return error for nil writer")
	}
}

// TestTXTWriter_OrderPreserved tests that segment order is preserved
func TestTXTWriter_OrderPreserved(t *testing.T) {
	writer := &TXTWriter{}
	var buf bytes.Buffer

	segments := []Segment{
		{Start: 0.0, End: 1.0, Text: "First"},
		{Start: 1.0, End: 2.0, Text: "Second"},
		{Start: 2.0, End: 3.0, Text: "Third"},
		{Start: 3.0, End: 4.0, Text: "Fourth"},
	}

	err := writer.Write(&buf, segments, testMetadata)
	if err != nil {
		t.Fatalf("TXTWriter.Write() failed: %v", err)
	}

	output := buf.String()
	lines := strings.Split(strings.TrimSpace(output), "\n")

	if len(lines) != 4 {
		t.Fatalf("Expected 4 lines, got %d", len(lines))
	}

	expected := []string{"First", "Second", "Third", "Fourth"}
	for i, line := range lines {
		if strings.TrimSpace(line) != expected[i] {
			t.Errorf("Line %d: expected %q, got %q", i, expected[i], line)
		}
	}
}
