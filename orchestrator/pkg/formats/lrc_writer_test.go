package formats

import (
	"bytes"
	"strings"
	"testing"
)

func TestLRCWriter_HappyPath(t *testing.T) {
	writer := &LRCWriter{}
	segments := []Segment{
		{Start: 0.0, End: 3.2, Text: "First lyric line"},
		{Start: 3.5, End: 6.8, Text: "Second lyric line"},
	}
	metadata := Metadata{Language: "en"}

	var buf bytes.Buffer
	err := writer.Write(&buf, segments, metadata)
	if err != nil {
		t.Fatalf("Write failed: %v", err)
	}

	output := buf.String()

	// Check language header
	if !strings.Contains(output, "[la:en]") {
		t.Error("Output should contain language header")
	}

	// Check timestamps (LRC format: [MM:SS.xx])
	if !strings.Contains(output, "[00:00.00]First lyric line") {
		t.Error("Output should contain first lyric with timestamp")
	}
	if !strings.Contains(output, "[00:03.50]Second lyric line") {
		t.Error("Output should contain second lyric with timestamp")
	}
}

func TestLRCWriter_EmptySegments(t *testing.T) {
	writer := &LRCWriter{}
	metadata := Metadata{Language: "en"}

	var buf bytes.Buffer
	err := writer.Write(&buf, []Segment{}, metadata)

	if err != nil {
		t.Errorf("Write should not error on empty segments, got: %v", err)
	}

	output := buf.String()
	// Should still have language header
	if !strings.Contains(output, "[la:en]") {
		t.Error("Output should contain language header even with empty segments")
	}
}

func TestLRCWriter_TimestampFormatting(t *testing.T) {
	writer := &LRCWriter{}
	segments := []Segment{
		{Start: 65.12, End: 70.0, Text: "Timestamp test"}, // 1:05.12
		{Start: 126.0, End: 130.0, Text: "Another line"},  // 2:06.00
	}

	var buf bytes.Buffer
	err := writer.Write(&buf, segments, Metadata{})
	if err != nil {
		t.Fatalf("Write failed: %v", err)
	}

	output := buf.String()
	// 65.12 seconds = 1 minute, 5 seconds, 12 centiseconds
	if !strings.Contains(output, "[01:05.12]Timestamp test") {
		t.Errorf("First timestamp formatting incorrect, got: %s", output)
	}
	// 126.0 seconds = 2 minutes, 6 seconds, 0 centiseconds
	if !strings.Contains(output, "[02:06.00]Another line") {
		t.Errorf("Second timestamp formatting incorrect, got: %s", output)
	}
}

func TestLRCWriter_NoLanguageMetadata(t *testing.T) {
	writer := &LRCWriter{}
	segments := []Segment{
		{Start: 0.0, End: 2.0, Text: "Test line"},
	}

	var buf bytes.Buffer
	err := writer.Write(&buf, segments, Metadata{Language: ""})
	if err != nil {
		t.Fatalf("Write failed: %v", err)
	}

	output := buf.String()
	// Should not have language header if language is empty
	if strings.Contains(output, "[la:") {
		t.Error("Output should not contain language header when language is empty")
	}
	// But should still have the lyric line
	if !strings.Contains(output, "[00:00.00]Test line") {
		t.Error("Output should contain lyric line")
	}
}

func TestLRCWriter_SpecialCharacters(t *testing.T) {
	writer := &LRCWriter{}
	segments := []Segment{
		{Start: 0.0, End: 2.0, Text: "Text with 'quotes' and brackets [like this]"},
		{Start: 2.5, End: 4.0, Text: "Text with special chars: @#$%"},
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
	if !strings.Contains(output, "[like this]") {
		t.Error("Brackets in text should be preserved")
	}
}

func TestLRCWriter_NilWriter(t *testing.T) {
	writer := &LRCWriter{}
	segments := []Segment{{Start: 0.0, End: 1.0, Text: "Test"}}

	err := writer.Write(nil, segments, Metadata{})
	if err == nil {
		t.Error("Write should return error for nil writer")
	}
	if !strings.Contains(err.Error(), "cannot be nil") {
		t.Errorf("Error should mention nil writer, got: %v", err)
	}
}

func TestLRCWriter_Multiline(t *testing.T) {
	writer := &LRCWriter{}
	segments := []Segment{
		{Start: 0.0, End: 2.0, Text: "Line 1\nLine 2"},
	}

	var buf bytes.Buffer
	err := writer.Write(&buf, segments, Metadata{})
	if err != nil {
		t.Fatalf("Write failed: %v", err)
	}

	output := buf.String()
	// LRC typically doesn't support multiline in a single timestamp,
	// but we preserve it as-is
	if !strings.Contains(output, "Line 1\nLine 2") {
		t.Error("Multiline text should be preserved")
	}
}
