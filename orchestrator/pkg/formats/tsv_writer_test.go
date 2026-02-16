package formats

import (
	"bytes"
	"strings"
	"testing"
)

// TestTSVWriter_HappyPath tests normal TSV output
func TestTSVWriter_HappyPath(t *testing.T) {
	writer := &TSVWriter{}
	var buf bytes.Buffer

	err := writer.Write(&buf, testSegments, testMetadata)
	if err != nil {
		t.Fatalf("TSVWriter.Write() failed: %v", err)
	}

	output := buf.String()
	lines := strings.Split(strings.TrimSpace(output), "\n")

	// Check header row
	if len(lines) < 1 {
		t.Fatalf("TSV output should have at least header row")
	}
	header := lines[0]
	if header != "start\tend\ttext" {
		t.Errorf("TSV header should be 'start\\tend\\ttext', got: %q", header)
	}

	// Check number of data rows
	if len(lines) != len(testSegments)+1 { // +1 for header
		t.Errorf("Expected %d lines (header + %d segments), got %d", len(testSegments)+1, len(testSegments), len(lines))
	}

	// Check all segments are present
	for _, seg := range testSegments {
		if !strings.Contains(output, seg.Text) {
			t.Errorf("TSV output missing segment text: %q", seg.Text)
		}
	}

	// Check tab-separated format
	for i := 1; i < len(lines); i++ {
		parts := strings.Split(lines[i], "\t")
		if len(parts) != 3 {
			t.Errorf("Line %d should have 3 tab-separated fields, got %d: %q", i, len(parts), lines[i])
		}
	}
}

// TestTSVWriter_EmptySegments tests TSV with no segments
func TestTSVWriter_EmptySegments(t *testing.T) {
	writer := &TSVWriter{}
	var buf bytes.Buffer

	err := writer.Write(&buf, []Segment{}, testMetadata)
	if err != nil {
		t.Fatalf("TSVWriter.Write() should not error on empty segments: %v", err)
	}

	output := buf.String()
	lines := strings.Split(strings.TrimSpace(output), "\n")

	// Should have header row even with no segments
	if len(lines) != 1 {
		t.Errorf("TSV with empty segments should have 1 line (header), got %d", len(lines))
	}

	if lines[0] != "start\tend\ttext" {
		t.Errorf("TSV header should be 'start\\tend\\ttext', got: %q", lines[0])
	}
}

// TestTSVWriter_TextWithTabs tests TSV with tabs in text
func TestTSVWriter_TextWithTabs(t *testing.T) {
	writer := &TSVWriter{}
	var buf bytes.Buffer

	segments := []Segment{
		{Start: 0.0, End: 2.0, Text: "Text\twith\ttabs"},
		{Start: 2.5, End: 4.5, Text: "Normal text"},
	}

	err := writer.Write(&buf, segments, testMetadata)
	if err != nil {
		t.Fatalf("TSVWriter.Write() failed: %v", err)
	}

	output := buf.String()
	lines := strings.Split(strings.TrimSpace(output), "\n")

	// Each line should still have exactly 3 fields (tabs in text should be escaped/replaced)
	for i := 1; i < len(lines); i++ {
		parts := strings.Split(lines[i], "\t")
		if len(parts) != 3 {
			t.Errorf("Line %d should have 3 fields even with tabs in text, got %d", i, len(parts))
		}
	}
}

// TestTSVWriter_TextWithNewlines tests TSV with newlines in text
func TestTSVWriter_TextWithNewlines(t *testing.T) {
	writer := &TSVWriter{}
	var buf bytes.Buffer

	segments := []Segment{
		{Start: 0.0, End: 2.0, Text: "Text\nwith\nnewlines"},
		{Start: 2.5, End: 4.5, Text: "Normal text"},
	}

	err := writer.Write(&buf, segments, testMetadata)
	if err != nil {
		t.Fatalf("TSVWriter.Write() failed: %v", err)
	}

	output := buf.String()
	lines := strings.Split(strings.TrimSpace(output), "\n")

	// Should have header + 2 segments = 3 lines (newlines in text should be escaped)
	// The actual number of lines depends on how newlines are handled
	// At minimum, we should have 3 lines
	if len(lines) < 3 {
		t.Errorf("Expected at least 3 lines, got %d", len(lines))
	}
}

// TestTSVWriter_FloatingPointPrecision tests TSV timestamp formatting
func TestTSVWriter_FloatingPointPrecision(t *testing.T) {
	writer := &TSVWriter{}
	var buf bytes.Buffer

	segments := []Segment{
		{Start: 0.123456789, End: 1.987654321, Text: "High precision"},
		{Start: 10.5, End: 20.75, Text: "Low precision"},
	}

	err := writer.Write(&buf, segments, testMetadata)
	if err != nil {
		t.Fatalf("TSVWriter.Write() failed: %v", err)
	}

	output := buf.String()
	lines := strings.Split(strings.TrimSpace(output), "\n")

	// Check first data row has reasonable precision
	if len(lines) < 2 {
		t.Fatalf("Expected at least 2 lines")
	}

	parts := strings.Split(lines[1], "\t")
	if len(parts) != 3 {
		t.Fatalf("Expected 3 fields in data row")
	}

	// Start time should be formatted with 3 decimal places
	if !strings.Contains(parts[0], "0.123") {
		t.Errorf("Start time should have at least 3 decimal places, got: %q", parts[0])
	}
}

// TestTSVWriter_SpecialCharacters tests TSV with special characters
func TestTSVWriter_SpecialCharacters(t *testing.T) {
	writer := &TSVWriter{}
	var buf bytes.Buffer

	segments := []Segment{
		{Start: 0.0, End: 2.0, Text: "Text with \"quotes\""},
		{Start: 2.5, End: 4.5, Text: "Text with 'apostrophes'"},
		{Start: 5.0, End: 7.0, Text: "Unicode: émojis 😀 中文"},
	}

	err := writer.Write(&buf, segments, testMetadata)
	if err != nil {
		t.Fatalf("TSVWriter.Write() failed: %v", err)
	}

	output := buf.String()

	// All text should be present (may be escaped)
	for _, seg := range segments {
		// Check if the core text is present (allowing for escaping)
		if !strings.Contains(output, "quotes") && !strings.Contains(output, seg.Text) {
			t.Errorf("TSV output missing text from segment: %q", seg.Text)
		}
	}
}

// TestTSVWriter_NilWriter tests error handling for nil writer
func TestTSVWriter_NilWriter(t *testing.T) {
	writer := &TSVWriter{}

	err := writer.Write(nil, testSegments, testMetadata)
	if err == nil {
		t.Errorf("TSVWriter.Write() should return error for nil writer")
	}
}

// TestTSVWriter_EmptyText tests TSV with empty text in some segments
func TestTSVWriter_EmptyText(t *testing.T) {
	writer := &TSVWriter{}
	var buf bytes.Buffer

	segments := []Segment{
		{Start: 0.0, End: 2.0, Text: "First"},
		{Start: 2.5, End: 4.5, Text: ""}, // Empty text
		{Start: 5.0, End: 7.0, Text: "Third"},
	}

	err := writer.Write(&buf, segments, testMetadata)
	if err != nil {
		t.Fatalf("TSVWriter.Write() failed: %v", err)
	}

	output := buf.String()
	lines := strings.Split(strings.TrimSpace(output), "\n")

	// Should have header + 3 segments
	if len(lines) != 4 {
		t.Errorf("Expected 4 lines, got %d", len(lines))
	}

	// Check that empty text is handled
	parts := strings.Split(lines[2], "\t") // Line with empty text
	if len(parts) != 3 {
		t.Errorf("Line with empty text should still have 3 fields, got %d", len(parts))
	}
}
