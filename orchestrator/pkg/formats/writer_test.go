package formats

import (
	"bytes"
	"strings"
	"testing"
)

// Test data shared across all format tests
var testSegments = []Segment{
	{Start: 0.0, End: 3.2, Text: "Hello, this is a test subtitle."},
	{Start: 3.4, End: 6.8, Text: "This is the second line of text."},
	{Start: 7.0, End: 10.5, Text: "The audio continues with more dialogue."},
}

var testMetadata = Metadata{
	Language: "en",
	Duration: 10.5,
}

// TestNewWriter_ValidFormats tests factory function with valid format strings
func TestNewWriter_ValidFormats(t *testing.T) {
	tests := []struct {
		format   string
		expected string // Expected type name
	}{
		{"vtt", "*formats.VTTWriter"},
		{"VTT", "*formats.VTTWriter"},
		{"txt", "*formats.TXTWriter"},
		{"TXT", "*formats.TXTWriter"},
		{"tsv", "*formats.TSVWriter"},
		{"TSV", "*formats.TSVWriter"},
		{"json", "*formats.JSONWriter"},
		{"JSON", "*formats.JSONWriter"},
	}

	for _, tt := range tests {
		t.Run(tt.format, func(t *testing.T) {
			writer, err := NewWriter(tt.format)
			if err != nil {
				t.Fatalf("NewWriter(%q) returned error: %v", tt.format, err)
			}
			if writer == nil {
				t.Fatalf("NewWriter(%q) returned nil writer", tt.format)
			}
			// Type check would be done with reflection if needed
			// For now, just verify it's not nil
		})
	}
}

// TestNewWriter_InvalidFormat tests factory function with invalid format strings
func TestNewWriter_InvalidFormat(t *testing.T) {
	tests := []string{
		"",
		"unknown",
		"srt", // Not implemented yet
		"lrc", // Not implemented yet
		"invalid",
		"123",
	}

	for _, format := range tests {
		t.Run(format, func(t *testing.T) {
			writer, err := NewWriter(format)
			if err == nil {
				t.Fatalf("NewWriter(%q) should return error, got writer: %v", format, writer)
			}
			if writer != nil {
				t.Fatalf("NewWriter(%q) should return nil writer on error", format)
			}
			if !strings.Contains(err.Error(), "unsupported format") {
				t.Errorf("Error message should contain 'unsupported format', got: %v", err)
			}
		})
	}
}

// TestNewWriter_CaseInsensitive tests that format strings are case-insensitive
func TestNewWriter_CaseInsensitive(t *testing.T) {
	formats := []string{"vtt", "VTT", "Vtt", "vTt"}

	for _, format := range formats {
		t.Run(format, func(t *testing.T) {
			writer, err := NewWriter(format)
			if err != nil {
				t.Fatalf("NewWriter(%q) should be case-insensitive, got error: %v", format, err)
			}
			if writer == nil {
				t.Fatalf("NewWriter(%q) returned nil writer", format)
			}
		})
	}
}

// TestAllFormats_Integration tests that all formats can be created and write successfully
func TestAllFormats_Integration(t *testing.T) {
	formats := []string{"vtt", "txt", "tsv", "json"}

	for _, format := range formats {
		t.Run(format, func(t *testing.T) {
			writer, err := NewWriter(format)
			if err != nil {
				t.Fatalf("Failed to create %s writer: %v", format, err)
			}

			var buf bytes.Buffer
			err = writer.Write(&buf, testSegments, testMetadata)
			if err != nil {
				t.Fatalf("Failed to write %s format: %v", format, err)
			}

			output := buf.String()
			if output == "" {
				t.Fatalf("%s writer produced empty output", format)
			}

			// Basic sanity check - output should contain some text from segments
			if !strings.Contains(output, "Hello") {
				t.Errorf("%s output should contain 'Hello', got: %s", format, output)
			}
		})
	}
}

// TestEmptySegments tests that all formats handle empty segment lists gracefully
func TestEmptySegments(t *testing.T) {
	formats := []string{"vtt", "txt", "tsv", "json"}

	for _, format := range formats {
		t.Run(format, func(t *testing.T) {
			writer, err := NewWriter(format)
			if err != nil {
				t.Fatalf("Failed to create %s writer: %v", format, err)
			}

			var buf bytes.Buffer
			emptySegments := []Segment{}
			err = writer.Write(&buf, emptySegments, testMetadata)

			// Should not error on empty segments
			if err != nil {
				t.Errorf("%s writer should handle empty segments, got error: %v", format, err)
			}

			// Output should still be valid (e.g., JSON should be valid, VTT should have header)
			output := buf.String()
			if output == "" && (format == "vtt" || format == "json" || format == "tsv") {
				// VTT needs header, JSON needs structure, TSV needs header
				t.Errorf("%s writer should produce some output even with empty segments", format)
			}
		})
	}
}
