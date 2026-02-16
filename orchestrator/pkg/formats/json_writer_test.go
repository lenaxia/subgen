package formats

import (
	"bytes"
	"encoding/json"
	"strings"
	"testing"
)

// JSONOutput represents the expected JSON structure
type JSONOutput struct {
	Language string        `json:"language"`
	Duration float64       `json:"duration"`
	Segments []JSONSegment `json:"segments"`
}

type JSONSegment struct {
	Start float64 `json:"start"`
	End   float64 `json:"end"`
	Text  string  `json:"text"`
}

// TestJSONWriter_HappyPath tests normal JSON output
func TestJSONWriter_HappyPath(t *testing.T) {
	writer := &JSONWriter{}
	var buf bytes.Buffer

	err := writer.Write(&buf, testSegments, testMetadata)
	if err != nil {
		t.Fatalf("JSONWriter.Write() failed: %v", err)
	}

	output := buf.String()

	// Parse JSON to verify it's valid
	var result JSONOutput
	if err := json.Unmarshal([]byte(output), &result); err != nil {
		t.Fatalf("JSON output is not valid: %v\nOutput: %s", err, output)
	}

	// Check language
	if result.Language != testMetadata.Language {
		t.Errorf("Expected language %q, got %q", testMetadata.Language, result.Language)
	}

	// Check duration
	if result.Duration != testMetadata.Duration {
		t.Errorf("Expected duration %.1f, got %.1f", testMetadata.Duration, result.Duration)
	}

	// Check segments
	if len(result.Segments) != len(testSegments) {
		t.Errorf("Expected %d segments, got %d", len(testSegments), len(result.Segments))
	}

	// Verify segment content
	for i, seg := range testSegments {
		if i >= len(result.Segments) {
			break
		}
		resSeg := result.Segments[i]
		if resSeg.Start != seg.Start {
			t.Errorf("Segment %d: expected start %.1f, got %.1f", i, seg.Start, resSeg.Start)
		}
		if resSeg.End != seg.End {
			t.Errorf("Segment %d: expected end %.1f, got %.1f", i, seg.End, resSeg.End)
		}
		if resSeg.Text != seg.Text {
			t.Errorf("Segment %d: expected text %q, got %q", i, seg.Text, resSeg.Text)
		}
	}
}

// TestJSONWriter_EmptySegments tests JSON with no segments
func TestJSONWriter_EmptySegments(t *testing.T) {
	writer := &JSONWriter{}
	var buf bytes.Buffer

	err := writer.Write(&buf, []Segment{}, testMetadata)
	if err != nil {
		t.Fatalf("JSONWriter.Write() should not error on empty segments: %v", err)
	}

	output := buf.String()

	// Parse JSON
	var result JSONOutput
	if err := json.Unmarshal([]byte(output), &result); err != nil {
		t.Fatalf("JSON output is not valid: %v", err)
	}

	// Should have metadata but empty segments array
	if result.Language != testMetadata.Language {
		t.Errorf("Expected language %q, got %q", testMetadata.Language, result.Language)
	}

	if len(result.Segments) != 0 {
		t.Errorf("Expected 0 segments, got %d", len(result.Segments))
	}
}

// TestJSONWriter_SpecialCharacters tests JSON with special characters
func TestJSONWriter_SpecialCharacters(t *testing.T) {
	writer := &JSONWriter{}
	var buf bytes.Buffer

	segments := []Segment{
		{Start: 0.0, End: 2.0, Text: "Text with \"quotes\""},
		{Start: 2.5, End: 4.5, Text: "Text with \\ backslashes"},
		{Start: 5.0, End: 7.0, Text: "Unicode: émojis 😀 中文"},
		{Start: 7.5, End: 9.5, Text: "Newlines\nand\ttabs"},
	}

	metadata := Metadata{Language: "en", Duration: 10.0}

	err := writer.Write(&buf, segments, metadata)
	if err != nil {
		t.Fatalf("JSONWriter.Write() failed: %v", err)
	}

	output := buf.String()

	// Parse JSON - should handle all special characters correctly
	var result JSONOutput
	if err := json.Unmarshal([]byte(output), &result); err != nil {
		t.Fatalf("JSON output is not valid: %v\nOutput: %s", err, output)
	}

	// Verify all segments were preserved correctly
	if len(result.Segments) != len(segments) {
		t.Errorf("Expected %d segments, got %d", len(segments), len(result.Segments))
	}

	for i, seg := range segments {
		if i >= len(result.Segments) {
			break
		}
		if result.Segments[i].Text != seg.Text {
			t.Errorf("Segment %d: text not preserved correctly.\nExpected: %q\nGot: %q",
				i, seg.Text, result.Segments[i].Text)
		}
	}
}

// TestJSONWriter_EmptyMetadata tests JSON with empty metadata fields
func TestJSONWriter_EmptyMetadata(t *testing.T) {
	writer := &JSONWriter{}
	var buf bytes.Buffer

	metadata := Metadata{Language: "", Duration: 0}

	err := writer.Write(&buf, testSegments, metadata)
	if err != nil {
		t.Fatalf("JSONWriter.Write() failed: %v", err)
	}

	output := buf.String()

	// Should still be valid JSON
	var result JSONOutput
	if err := json.Unmarshal([]byte(output), &result); err != nil {
		t.Fatalf("JSON output is not valid: %v", err)
	}

	// Check empty language
	if result.Language != "" {
		t.Errorf("Expected empty language, got %q", result.Language)
	}

	// Check zero duration
	if result.Duration != 0 {
		t.Errorf("Expected zero duration, got %.1f", result.Duration)
	}
}

// TestJSONWriter_FloatingPointPrecision tests JSON timestamp precision
func TestJSONWriter_FloatingPointPrecision(t *testing.T) {
	writer := &JSONWriter{}
	var buf bytes.Buffer

	segments := []Segment{
		{Start: 0.123456789, End: 1.987654321, Text: "High precision"},
	}

	metadata := Metadata{Language: "en", Duration: 2.0}

	err := writer.Write(&buf, segments, metadata)
	if err != nil {
		t.Fatalf("JSONWriter.Write() failed: %v", err)
	}

	output := buf.String()

	// Parse JSON
	var result JSONOutput
	if err := json.Unmarshal([]byte(output), &result); err != nil {
		t.Fatalf("JSON output is not valid: %v", err)
	}

	// Check that floating point precision is preserved reasonably
	if len(result.Segments) != 1 {
		t.Fatalf("Expected 1 segment")
	}

	seg := result.Segments[0]
	// Allow small floating point differences
	if !almostEqual(seg.Start, 0.123456789, 0.000001) {
		t.Errorf("Start time not preserved with sufficient precision: %.9f", seg.Start)
	}
	if !almostEqual(seg.End, 1.987654321, 0.000001) {
		t.Errorf("End time not preserved with sufficient precision: %.9f", seg.End)
	}
}

// TestJSONWriter_NilWriter tests error handling for nil writer
func TestJSONWriter_NilWriter(t *testing.T) {
	writer := &JSONWriter{}

	err := writer.Write(nil, testSegments, testMetadata)
	if err == nil {
		t.Errorf("JSONWriter.Write() should return error for nil writer")
	}
}

// TestJSONWriter_PrettyFormat tests that JSON output is reasonably formatted
func TestJSONWriter_PrettyFormat(t *testing.T) {
	writer := &JSONWriter{}
	var buf bytes.Buffer

	err := writer.Write(&buf, testSegments, testMetadata)
	if err != nil {
		t.Fatalf("JSONWriter.Write() failed: %v", err)
	}

	output := buf.String()

	// Check that JSON is indented (pretty-printed)
	// Pretty JSON should have newlines and indentation
	if !strings.Contains(output, "\n") {
		t.Errorf("JSON output should be pretty-printed with newlines")
	}

	// Should be valid JSON
	var result map[string]interface{}
	if err := json.Unmarshal([]byte(output), &result); err != nil {
		t.Fatalf("JSON output is not valid: %v", err)
	}
}

// Helper function to compare floats with tolerance
func almostEqual(a, b, epsilon float64) bool {
	diff := a - b
	if diff < 0 {
		diff = -diff
	}
	return diff < epsilon
}
