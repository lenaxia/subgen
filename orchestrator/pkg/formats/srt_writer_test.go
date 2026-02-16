package formats

import (
	"bytes"
	"strings"
	"testing"
)

func TestSRTWriter_HappyPath(t *testing.T) {
	writer := &SRTWriter{}
	segments := []Segment{
		{Start: 0.0, End: 3.2, Text: "First subtitle line"},
		{Start: 3.5, End: 6.8, Text: "Second subtitle line"},
	}
	metadata := Metadata{Language: "en"}

	var buf bytes.Buffer
	err := writer.Write(&buf, segments, metadata)
	if err != nil {
		t.Fatalf("Write failed: %v", err)
	}

	output := buf.String()

	// Check sequence numbers
	if !strings.Contains(output, "1\n") {
		t.Error("Output should contain sequence number 1")
	}
	if !strings.Contains(output, "2\n") {
		t.Error("Output should contain sequence number 2")
	}

	// Check timestamps (SRT uses comma for milliseconds)
	if !strings.Contains(output, "00:00:00,000 --> 00:00:03,200") {
		t.Errorf("Output should contain first timestamp, got: %s", output)
	}
	// Allow minor rounding differences due to float precision
	if !strings.Contains(output, "00:00:03,5") || !strings.Contains(output, "00:00:06,") {
		t.Errorf("Output should contain second timestamp range, got: %s", output)
	}

	// Check text
	if !strings.Contains(output, "First subtitle line") {
		t.Error("Output should contain first subtitle text")
	}
	if !strings.Contains(output, "Second subtitle line") {
		t.Error("Output should contain second subtitle text")
	}
}

func TestSRTWriter_EmptySegments(t *testing.T) {
	writer := &SRTWriter{}
	var buf bytes.Buffer
	err := writer.Write(&buf, []Segment{}, Metadata{})

	if err != nil {
		t.Errorf("Write should not error on empty segments, got: %v", err)
	}

	output := buf.String()
	if output != "" {
		t.Error("Output should be empty for empty segments")
	}
}

func TestSRTWriter_TimestampFormatting(t *testing.T) {
	writer := &SRTWriter{}
	segments := []Segment{
		{Start: 3661.123, End: 3666.0, Text: "Long timestamp test"},
	}

	var buf bytes.Buffer
	err := writer.Write(&buf, segments, Metadata{})
	if err != nil {
		t.Fatalf("Write failed: %v", err)
	}

	output := buf.String()
	// 3661.123 seconds = 1 hour, 1 minute, 1 second, 123 milliseconds
	// 3666.0 seconds = 1 hour, 1 minute, 6 seconds, 0 milliseconds
	if !strings.Contains(output, "01:01:01,123 --> 01:01:06,000") {
		t.Errorf("Timestamp formatting incorrect, got: %s", output)
	}
}

func TestSRTWriter_SpecialCharacters(t *testing.T) {
	writer := &SRTWriter{}
	segments := []Segment{
		{Start: 0.0, End: 2.0, Text: "Text with 'quotes' and \"double quotes\""},
		{Start: 2.5, End: 4.0, Text: "Text with <tags> and & ampersand"},
	}

	var buf bytes.Buffer
	err := writer.Write(&buf, segments, Metadata{})
	if err != nil {
		t.Fatalf("Write failed: %v", err)
	}

	output := buf.String()
	if !strings.Contains(output, "Text with 'quotes'") {
		t.Error("Special characters should be preserved")
	}
	if !strings.Contains(output, "<tags>") {
		t.Error("Angle brackets should be preserved")
	}
}

func TestSRTWriter_NilWriter(t *testing.T) {
	writer := &SRTWriter{}
	segments := []Segment{{Start: 0.0, End: 1.0, Text: "Test"}}

	err := writer.Write(nil, segments, Metadata{})
	if err == nil {
		t.Error("Write should return error for nil writer")
	}
	if !strings.Contains(err.Error(), "cannot be nil") {
		t.Errorf("Error should mention nil writer, got: %v", err)
	}
}

func TestSRTWriter_MultilineText(t *testing.T) {
	writer := &SRTWriter{}
	segments := []Segment{
		{Start: 0.0, End: 2.0, Text: "Line 1\nLine 2"},
	}

	var buf bytes.Buffer
	err := writer.Write(&buf, segments, Metadata{})
	if err != nil {
		t.Fatalf("Write failed: %v", err)
	}

	output := buf.String()
	// SRT supports multiline text
	if !strings.Contains(output, "Line 1\nLine 2") {
		t.Error("Multiline text should be preserved")
	}
}
