package skip

import (
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"testing"
)

// TestMultiAudioTrackDetection_Integration tests detection of multiple audio tracks in a real file
// This test requires FFprobe to be installed on the system
func TestMultiAudioTrackDetection_Integration(t *testing.T) {
	// Skip if running in environments without FFprobe
	if os.Getenv("SKIP_INTEGRATION_TESTS") == "true" {
		t.Skip("Skipping integration test")
	}

	// Path to test file with multiple audio tracks
	testFile := "../../../test/testdata/multi_audio_test/multi_audio_test.mkv"
	absPath, err := filepath.Abs(testFile)
	if err != nil {
		t.Fatalf("Failed to get absolute path: %v", err)
	}

	// Check if test file exists
	if _, err := os.Stat(absPath); os.IsNotExist(err) {
		t.Skipf("Test file not found: %s. Run create_multi_audio_video.sh to generate it.", absPath)
	}

	detector := NewAudioDetector()
	ctx := context.Background()

	// Get audio tracks
	tracks, err := detector.GetAudioTracks(ctx, absPath)
	if err != nil {
		t.Fatalf("GetAudioTracks failed: %v", err)
	}

	// Verify we detected 3 audio tracks
	if len(tracks) != 3 {
		t.Errorf("Expected 3 audio tracks, got %d", len(tracks))
	}

	// Verify track details
	expectedTracks := []struct {
		index    int
		language string
		title    string
		codec    string
		channels int
	}{
		{1, "eng", "English", "aac", 1},
		{2, "spa", "Spanish", "aac", 1},
		{3, "jpn", "Japanese", "aac", 1},
	}

	for i, expected := range expectedTracks {
		if i >= len(tracks) {
			t.Errorf("Missing track %d", i)
			continue
		}

		track := tracks[i]
		if track.Index != expected.index {
			t.Errorf("Track %d: expected index %d, got %d", i, expected.index, track.Index)
		}
		if track.Language != expected.language {
			t.Errorf("Track %d: expected language %s, got %s", i, expected.language, track.Language)
		}
		if track.Title != expected.title {
			t.Errorf("Track %d: expected title %s, got %s", i, expected.title, track.Title)
		}
		if track.Codec != expected.codec {
			t.Errorf("Track %d: expected codec %s, got %s", i, expected.codec, track.Codec)
		}
		if track.Channels != expected.channels {
			t.Errorf("Track %d: expected channels %d, got %d", i, expected.channels, track.Channels)
		}
	}

	t.Logf("Successfully detected %d audio tracks:", len(tracks))
	for i, track := range tracks {
		t.Logf("  Track %d: [%d] %s (%s) - %s, %d channels",
			i, track.Index, track.Title, track.Language, track.Codec, track.Channels)
	}
}

// TestMultiAudioTrackLanguageFiltering_Integration tests language-based filtering with multiple tracks
func TestMultiAudioTrackLanguageFiltering_Integration(t *testing.T) {
	if os.Getenv("SKIP_INTEGRATION_TESTS") == "true" {
		t.Skip("Skipping integration test")
	}

	testFile := "../../../test/testdata/multi_audio_test/multi_audio_test.mkv"
	absPath, err := filepath.Abs(testFile)
	if err != nil {
		t.Fatalf("Failed to get absolute path: %v", err)
	}

	if _, err := os.Stat(absPath); os.IsNotExist(err) {
		t.Skipf("Test file not found: %s", absPath)
	}

	detector := NewAudioDetector()
	ctx := context.Background()

	tracks, err := detector.GetAudioTracks(ctx, absPath)
	if err != nil {
		t.Fatalf("GetAudioTracks failed: %v", err)
	}

	tests := []struct {
		name           string
		preferredLangs []string
		shouldMatch    bool
		description    string
	}{
		{
			name:           "match English",
			preferredLangs: []string{"eng"},
			shouldMatch:    true,
			description:    "Should find English track",
		},
		{
			name:           "match Spanish",
			preferredLangs: []string{"spa"},
			shouldMatch:    true,
			description:    "Should find Spanish track",
		},
		{
			name:           "match Japanese",
			preferredLangs: []string{"jpn"},
			shouldMatch:    true,
			description:    "Should find Japanese track",
		},
		{
			name:           "match multiple (eng|jpn)",
			preferredLangs: []string{"eng", "jpn"},
			shouldMatch:    true,
			description:    "Should find either English or Japanese track",
		},
		{
			name:           "no match French",
			preferredLangs: []string{"fre"},
			shouldMatch:    false,
			description:    "Should not find French track",
		},
		{
			name:           "no match German",
			preferredLangs: []string{"ger"},
			shouldMatch:    false,
			description:    "Should not find German track",
		},
		{
			name:           "match with ISO 639-1 code (en for eng)",
			preferredLangs: []string{"en"},
			shouldMatch:    true,
			description:    "Should match 'eng' track with 'en' preference",
		},
		{
			name:           "match with ISO 639-1 code (ja for jpn)",
			preferredLangs: []string{"ja"},
			shouldMatch:    true,
			description:    "Should match 'jpn' track with 'ja' preference",
		},
		{
			name:           "match with ISO 639-1 code (es for spa)",
			preferredLangs: []string{"es"},
			shouldMatch:    true,
			description:    "Should match 'spa' track with 'es' preference",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := detector.HasAnyPreferredLanguage(tracks, tt.preferredLangs)
			if result != tt.shouldMatch {
				t.Errorf("%s: expected %v, got %v", tt.description, tt.shouldMatch, result)
			}
		})
	}
}

// TestMultiAudioTrackSkipLogic_Integration tests the skip logic with multiple audio tracks
func TestMultiAudioTrackSkipLogic_Integration(t *testing.T) {
	if os.Getenv("SKIP_INTEGRATION_TESTS") == "true" {
		t.Skip("Skipping integration test")
	}

	testFile := "../../../test/testdata/multi_audio_test/multi_audio_test.mkv"
	absPath, err := filepath.Abs(testFile)
	if err != nil {
		t.Fatalf("Failed to get absolute path: %v", err)
	}

	if _, err := os.Stat(absPath); os.IsNotExist(err) {
		t.Skipf("Test file not found: %s", absPath)
	}

	detector := NewAudioDetector()
	ctx := context.Background()

	tracks, err := detector.GetAudioTracks(ctx, absPath)
	if err != nil {
		t.Fatalf("GetAudioTracks failed: %v", err)
	}

	// Test case 1: Skip if audio language is in skip list
	t.Run("skip if audio in skip list", func(t *testing.T) {
		hasSkipLang := detector.HasLanguage(tracks, "eng")
		if !hasSkipLang {
			t.Error("Should detect English track for skipping")
		}
		t.Logf("Detected English track in skip list: %v", hasSkipLang)
	})

	// Test case 2: Process if preferred language found
	t.Run("process if preferred language found", func(t *testing.T) {
		preferredLangs := []string{"jpn", "kor"}
		hasPreferred := detector.HasAnyPreferredLanguage(tracks, preferredLangs)
		if !hasPreferred {
			t.Error("Should find Japanese track in preferred list")
		}
	})

	// Test case 3: Skip if preferred language not found
	t.Run("skip if preferred language not found", func(t *testing.T) {
		preferredLangs := []string{"fre", "ger"}
		hasPreferred := detector.HasAnyPreferredLanguage(tracks, preferredLangs)
		if hasPreferred {
			t.Error("Should not find French or German tracks")
		}
	})
}

// TestFFProbeOutput_RealFile tests parsing real FFprobe output
func TestFFProbeOutput_RealFile(t *testing.T) {
	if os.Getenv("SKIP_INTEGRATION_TESTS") == "true" {
		t.Skip("Skipping integration test")
	}

	// Sample FFprobe JSON output from the multi-audio test file
	sampleJSON := `{
    "streams": [
        {
            "index": 1,
            "codec_name": "aac",
            "codec_type": "audio",
            "channels": 1,
            "tags": {
                "language": "eng",
                "title": "English"
            }
        },
        {
            "index": 2,
            "codec_name": "aac",
            "codec_type": "audio",
            "channels": 1,
            "tags": {
                "language": "spa",
                "title": "Spanish"
            }
        },
        {
            "index": 3,
            "codec_name": "aac",
            "codec_type": "audio",
            "channels": 1,
            "tags": {
                "language": "jpn",
                "title": "Japanese"
            }
        }
    ]
}`

	var probe FFProbeOutput
	err := json.Unmarshal([]byte(sampleJSON), &probe)
	if err != nil {
		t.Fatalf("Failed to parse JSON: %v", err)
	}

	if len(probe.Streams) != 3 {
		t.Errorf("Expected 3 streams, got %d", len(probe.Streams))
	}

	// Verify each stream
	expectedStreams := []struct {
		index    int
		codec    string
		language string
		title    string
	}{
		{1, "aac", "eng", "English"},
		{2, "aac", "spa", "Spanish"},
		{3, "aac", "jpn", "Japanese"},
	}

	for i, expected := range expectedStreams {
		stream := probe.Streams[i]
		if stream.Index != expected.index {
			t.Errorf("Stream %d: expected index %d, got %d", i, expected.index, stream.Index)
		}
		if stream.CodecName != expected.codec {
			t.Errorf("Stream %d: expected codec %s, got %s", i, expected.codec, stream.CodecName)
		}
		if stream.Tags.Language != expected.language {
			t.Errorf("Stream %d: expected language %s, got %s", i, expected.language, stream.Tags.Language)
		}
		if stream.Tags.Title != expected.title {
			t.Errorf("Stream %d: expected title %s, got %s", i, expected.title, stream.Tags.Title)
		}
	}

	// Test extraction
	detector := NewAudioDetector()
	tracks := detector.extractAudioTracks(&probe)

	if len(tracks) != 3 {
		t.Errorf("Expected 3 audio tracks, got %d", len(tracks))
	}

	for i, track := range tracks {
		expected := expectedStreams[i]
		if track.Index != expected.index {
			t.Errorf("Track %d: expected index %d, got %d", i, expected.index, track.Index)
		}
		if track.Language != expected.language {
			t.Errorf("Track %d: expected language %s, got %s", i, expected.language, track.Language)
		}
		if track.Title != expected.title {
			t.Errorf("Track %d: expected title %s, got %s", i, expected.title, track.Title)
		}
		if track.Codec != expected.codec {
			t.Errorf("Track %d: expected codec %s, got %s", i, expected.codec, track.Codec)
		}
	}
}
