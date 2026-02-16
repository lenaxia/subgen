package skip

import (
	"context"
	"encoding/json"
	"os"
	"os/exec"
	"testing"
)

// TestFFProbeOutput_ParseValidJSON tests parsing a valid FFprobe JSON response
func TestFFProbeOutput_ParseValidJSON(t *testing.T) {
	jsonData := `{
		"streams": [
			{
				"index": 0,
				"codec_type": "video",
				"codec_name": "h264"
			},
			{
				"index": 1,
				"codec_type": "subtitle",
				"codec_name": "subrip",
				"tags": {
					"language": "eng",
					"title": "English"
				}
			}
		]
	}`

	var probe FFProbeOutput
	err := json.Unmarshal([]byte(jsonData), &probe)
	if err != nil {
		t.Fatalf("Failed to parse valid JSON: %v", err)
	}

	if len(probe.Streams) != 2 {
		t.Errorf("Expected 2 streams, got %d", len(probe.Streams))
	}

	// Check subtitle stream
	subStream := probe.Streams[1]
	if subStream.CodecType != "subtitle" {
		t.Errorf("Expected codec_type 'subtitle', got '%s'", subStream.CodecType)
	}
	if subStream.CodecName != "subrip" {
		t.Errorf("Expected codec_name 'subrip', got '%s'", subStream.CodecName)
	}
	if subStream.Tags.Language != "eng" {
		t.Errorf("Expected language 'eng', got '%s'", subStream.Tags.Language)
	}
	if subStream.Tags.Title != "English" {
		t.Errorf("Expected title 'English', got '%s'", subStream.Tags.Title)
	}
}

// TestFFProbeOutput_ParseMultipleSubtitles tests parsing JSON with multiple subtitle streams
func TestFFProbeOutput_ParseMultipleSubtitles(t *testing.T) {
	data, err := os.ReadFile("testdata/ffprobe_multiple_subtitles.json")
	if err != nil {
		t.Fatalf("Failed to read test data: %v", err)
	}

	var probe FFProbeOutput
	err = json.Unmarshal(data, &probe)
	if err != nil {
		t.Fatalf("Failed to parse JSON: %v", err)
	}

	// Count subtitle streams
	subtitleCount := 0
	for _, stream := range probe.Streams {
		if stream.CodecType == "subtitle" {
			subtitleCount++
		}
	}

	if subtitleCount != 3 {
		t.Errorf("Expected 3 subtitle streams, got %d", subtitleCount)
	}
}

// TestFFProbeOutput_ParseVariousCodecs tests parsing different subtitle codec types
func TestFFProbeOutput_ParseVariousCodecs(t *testing.T) {
	data, err := os.ReadFile("testdata/ffprobe_multiple_subtitles.json")
	if err != nil {
		t.Fatalf("Failed to read test data: %v", err)
	}

	var probe FFProbeOutput
	err = json.Unmarshal(data, &probe)
	if err != nil {
		t.Fatalf("Failed to parse JSON: %v", err)
	}

	expectedCodecs := map[string]bool{
		"ass":               false,
		"subrip":            false,
		"hdmv_pgs_subtitle": false,
	}

	for _, stream := range probe.Streams {
		if stream.CodecType == "subtitle" {
			if _, exists := expectedCodecs[stream.CodecName]; exists {
				expectedCodecs[stream.CodecName] = true
			}
		}
	}

	for codec, found := range expectedCodecs {
		if !found {
			t.Errorf("Expected to find codec '%s', but didn't", codec)
		}
	}
}

// TestFFProbeOutput_ParseMissingFields tests graceful handling of missing tags
func TestFFProbeOutput_ParseMissingFields(t *testing.T) {
	data, err := os.ReadFile("testdata/ffprobe_subtitle_no_language.json")
	if err != nil {
		t.Fatalf("Failed to read test data: %v", err)
	}

	var probe FFProbeOutput
	err = json.Unmarshal(data, &probe)
	if err != nil {
		t.Fatalf("Failed to parse JSON: %v", err)
	}

	// Find subtitle stream
	var subStream *FFProbeStream
	for i, stream := range probe.Streams {
		if stream.CodecType == "subtitle" {
			subStream = &probe.Streams[i]
			break
		}
	}

	if subStream == nil {
		t.Fatal("Expected to find subtitle stream")
	}

	// Language should be empty string, not error
	if subStream.Tags.Language != "" {
		t.Errorf("Expected empty language, got '%s'", subStream.Tags.Language)
	}

	// Title should still be present
	if subStream.Tags.Title != "Unknown Language" {
		t.Errorf("Expected title 'Unknown Language', got '%s'", subStream.Tags.Title)
	}
}

// TestFFProbeOutput_ParseInvalidJSON tests error handling for malformed JSON
func TestFFProbeOutput_ParseInvalidJSON(t *testing.T) {
	invalidJSON := `{"streams": [{"invalid": }]}`

	var probe FFProbeOutput
	err := json.Unmarshal([]byte(invalidJSON), &probe)
	if err == nil {
		t.Error("Expected error for invalid JSON, got nil")
	}
}

// TestSubtitleDetector_GetEmbeddedSubtitles_SingleSubtitle tests detecting a single subtitle
// NOTE: This test requires FFprobe to be installed and is skipped if not available
func TestSubtitleDetector_GetEmbeddedSubtitles_SingleSubtitle(t *testing.T) {
	// Skip if FFprobe is not available
	if _, err := exec.LookPath("ffprobe"); err != nil {
		t.Skip("FFprobe not found in PATH, skipping integration test")
	}

	// This test would require an actual video file with subtitles
	// For unit testing, we test the parse/extract methods directly
	t.Skip("Integration test requires real video file with embedded subtitles")
}

// TestSubtitleDetector_GetEmbeddedSubtitles_MultipleSubtitles tests detecting multiple subtitles
// NOTE: This test requires FFprobe to be installed and is skipped if not available
func TestSubtitleDetector_GetEmbeddedSubtitles_MultipleSubtitles(t *testing.T) {
	// Skip if FFprobe is not available
	if _, err := exec.LookPath("ffprobe"); err != nil {
		t.Skip("FFprobe not found in PATH, skipping integration test")
	}

	// This test would require an actual video file with multiple subtitles
	t.Skip("Integration test requires real video file with multiple embedded subtitles")
}

// TestSubtitleDetector_GetEmbeddedSubtitles_NoSubtitles tests handling files with no subtitles
// NOTE: This test requires FFprobe to be installed and is skipped if not available
func TestSubtitleDetector_GetEmbeddedSubtitles_NoSubtitles(t *testing.T) {
	// Skip if FFprobe is not available
	if _, err := exec.LookPath("ffprobe"); err != nil {
		t.Skip("FFprobe not found in PATH, skipping integration test")
	}

	// This test would require an actual video file with no subtitles
	t.Skip("Integration test requires real video file without embedded subtitles")
}

// TestSubtitleDetector_GetEmbeddedSubtitles_EmptyFilePath tests error handling for empty path
func TestSubtitleDetector_GetEmbeddedSubtitles_EmptyFilePath(t *testing.T) {
	detector := NewSubtitleDetector()
	ctx := context.Background()

	_, err := detector.GetEmbeddedSubtitles(ctx, "")
	if err == nil {
		t.Error("Expected error for empty filePath, got nil")
	}
}

// TestSubtitleDetector_GetEmbeddedSubtitles_FFprobeCommandFails tests FFprobe execution failure
func TestSubtitleDetector_GetEmbeddedSubtitles_FFprobeCommandFails(t *testing.T) {
	detector := NewSubtitleDetector()
	ctx := context.Background()

	// Non-existent file should cause FFprobe to fail
	_, err := detector.GetEmbeddedSubtitles(ctx, "/nonexistent/file.mkv")
	if err == nil {
		t.Error("Expected error for non-existent file, got nil")
	}
}

// TestSubtitleDetector_HasLanguage tests language matching
func TestSubtitleDetector_HasLanguage(t *testing.T) {
	detector := NewSubtitleDetector()

	tracks := []SubtitleTrack{
		{Index: 0, Language: "eng", Title: "English", Codec: "subrip"},
		{Index: 1, Language: "jpn", Title: "Japanese", Codec: "ass"},
	}

	tests := []struct {
		name     string
		language string
		expected bool
	}{
		{"Match English", "eng", true},
		{"Match Japanese", "jpn", true},
		{"No match French", "fre", false},
		{"Empty language", "", false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := detector.HasLanguage(tracks, tt.language)
			if result != tt.expected {
				t.Errorf("HasLanguage(%s) = %v, expected %v", tt.language, result, tt.expected)
			}
		})
	}
}

// TestSubtitleDetector_ExtractSubtitleTracks tests track extraction logic
func TestSubtitleDetector_ExtractSubtitleTracks(t *testing.T) {
	detector := NewSubtitleDetector()

	probe := &FFProbeOutput{
		Streams: []FFProbeStream{
			{Index: 0, CodecType: "video", CodecName: "h264"},
			{Index: 1, CodecType: "audio", CodecName: "aac"},
			{Index: 2, CodecType: "subtitle", CodecName: "subrip", Tags: FFProbeStreamTags{Language: "eng", Title: "English"}},
			{Index: 3, CodecType: "subtitle", CodecName: "ass", Tags: FFProbeStreamTags{Language: "jpn", Title: "Japanese"}},
		},
	}

	tracks := detector.extractSubtitleTracks(probe)

	if len(tracks) != 2 {
		t.Fatalf("Expected 2 subtitle tracks, got %d", len(tracks))
	}

	// Check first track
	if tracks[0].Index != 2 {
		t.Errorf("Expected track 0 index 2, got %d", tracks[0].Index)
	}
	if tracks[0].Language != "eng" {
		t.Errorf("Expected track 0 language 'eng', got '%s'", tracks[0].Language)
	}

	// Check second track
	if tracks[1].Index != 3 {
		t.Errorf("Expected track 1 index 3, got %d", tracks[1].Index)
	}
	if tracks[1].Language != "jpn" {
		t.Errorf("Expected track 1 language 'jpn', got '%s'", tracks[1].Language)
	}
}

// TestSubtitleDetector_ParseFFprobeOutput tests JSON parsing logic
func TestSubtitleDetector_ParseFFprobeOutput(t *testing.T) {
	detector := NewSubtitleDetector()

	data, err := os.ReadFile("testdata/ffprobe_with_subtitle.json")
	if err != nil {
		t.Fatalf("Failed to read test data: %v", err)
	}

	probe, err := detector.parseFFprobeOutput(data)
	if err != nil {
		t.Fatalf("Failed to parse output: %v", err)
	}

	if len(probe.Streams) != 3 {
		t.Errorf("Expected 3 streams, got %d", len(probe.Streams))
	}
}

// TestSubtitleDetector_ParseFFprobeOutput_InvalidJSON tests error handling
func TestSubtitleDetector_ParseFFprobeOutput_InvalidJSON(t *testing.T) {
	detector := NewSubtitleDetector()

	invalidJSON := []byte(`{"invalid": }`)

	_, err := detector.parseFFprobeOutput(invalidJSON)
	if err == nil {
		t.Error("Expected error for invalid JSON, got nil")
	}
}
