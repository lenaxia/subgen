package skip

import (
	"context"
	"testing"
)

// TestParseLanguageList tests language list parsing
func TestParseLanguageList(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		expected []string
	}{
		{
			name:     "empty string",
			input:    "",
			expected: nil,
		},
		{
			name:     "single language",
			input:    "eng",
			expected: []string{"eng"},
		},
		{
			name:     "multiple languages",
			input:    "eng|jpn|kor",
			expected: []string{"eng", "jpn", "kor"},
		},
		{
			name:     "with whitespace",
			input:    "eng | jpn | kor",
			expected: []string{"eng", "jpn", "kor"},
		},
		{
			name:     "mixed case",
			input:    "ENG|JPN|KOR",
			expected: []string{"eng", "jpn", "kor"},
		},
		{
			name:     "empty parts",
			input:    "eng||kor",
			expected: []string{"eng", "kor"},
		},
		{
			name:     "trailing pipe",
			input:    "eng|jpn|",
			expected: []string{"eng", "jpn"},
		},
		{
			name:     "leading pipe",
			input:    "|eng|jpn",
			expected: []string{"eng", "jpn"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := ParseLanguageList(tt.input)

			if len(result) != len(tt.expected) {
				t.Errorf("ParseLanguageList(%q) length = %d, want %d", tt.input, len(result), len(tt.expected))
				return
			}

			for i := range result {
				if result[i] != tt.expected[i] {
					t.Errorf("ParseLanguageList(%q)[%d] = %q, want %q", tt.input, i, result[i], tt.expected[i])
				}
			}
		})
	}
}

// TestMatchesAnyLanguage tests language matching against a list
func TestMatchesAnyLanguage(t *testing.T) {
	tests := []struct {
		name       string
		targetLang string
		langList   []string
		expected   bool
	}{
		{
			name:       "exact match",
			targetLang: "eng",
			langList:   []string{"eng", "jpn", "kor"},
			expected:   true,
		},
		{
			name:       "no match",
			targetLang: "fre",
			langList:   []string{"eng", "jpn", "kor"},
			expected:   false,
		},
		{
			name:       "ISO 639-1 vs 639-2 match (en -> eng)",
			targetLang: "en",
			langList:   []string{"eng", "jpn", "kor"},
			expected:   true,
		},
		{
			name:       "ISO 639-2 vs 639-1 match (eng -> en)",
			targetLang: "eng",
			langList:   []string{"en", "ja", "ko"},
			expected:   true,
		},
		{
			name:       "case insensitive match",
			targetLang: "ENG",
			langList:   []string{"eng", "jpn", "kor"},
			expected:   true,
		},
		{
			name:       "empty target language",
			targetLang: "",
			langList:   []string{"eng", "jpn", "kor"},
			expected:   false,
		},
		{
			name:       "empty language list",
			targetLang: "eng",
			langList:   []string{},
			expected:   false,
		},
		{
			name:       "nil language list",
			targetLang: "eng",
			langList:   nil,
			expected:   false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := MatchesAnyLanguage(tt.targetLang, tt.langList)

			if result != tt.expected {
				t.Errorf("MatchesAnyLanguage(%q, %v) = %v, want %v", tt.targetLang, tt.langList, result, tt.expected)
			}
		})
	}
}

// TestLanguagesMatch tests ISO 639 code translation
func TestLanguagesMatch(t *testing.T) {
	tests := []struct {
		name     string
		lang1    string
		lang2    string
		expected bool
	}{
		{
			name:     "exact match",
			lang1:    "eng",
			lang2:    "eng",
			expected: true,
		},
		{
			name:     "en <-> eng",
			lang1:    "en",
			lang2:    "eng",
			expected: true,
		},
		{
			name:     "eng <-> en",
			lang1:    "eng",
			lang2:    "en",
			expected: true,
		},
		{
			name:     "ja <-> jpn",
			lang1:    "ja",
			lang2:    "jpn",
			expected: true,
		},
		{
			name:     "fr <-> fre",
			lang1:    "fr",
			lang2:    "fre",
			expected: true,
		},
		{
			name:     "case insensitive",
			lang1:    "ENG",
			lang2:    "en",
			expected: true,
		},
		{
			name:     "no match",
			lang1:    "eng",
			lang2:    "jpn",
			expected: false,
		},
		{
			name:     "empty strings",
			lang1:    "",
			lang2:    "",
			expected: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := languagesMatch(tt.lang1, tt.lang2)

			if result != tt.expected {
				t.Errorf("languagesMatch(%q, %q) = %v, want %v", tt.lang1, tt.lang2, result, tt.expected)
			}
		})
	}
}

// TestAudioDetector_GetAudioTracks tests audio track detection
func TestAudioDetector_GetAudioTracks(t *testing.T) {
	t.Run("empty file path", func(t *testing.T) {
		detector := NewAudioDetector()
		ctx := context.Background()

		_, err := detector.GetAudioTracks(ctx, "")
		if err == nil {
			t.Error("GetAudioTracks with empty filePath should return error")
		}
	})

	// Note: Real FFprobe tests would require mock command execution
	// or test fixtures. For now, we test the error handling.

	t.Run("non-existent file", func(t *testing.T) {
		detector := NewAudioDetector()
		ctx := context.Background()

		_, err := detector.GetAudioTracks(ctx, "/non/existent/file.mkv")
		if err == nil {
			t.Error("GetAudioTracks with non-existent file should return error")
		}
	})
}

// TestAudioDetector_HasLanguage tests audio language detection
func TestAudioDetector_HasLanguage(t *testing.T) {
	detector := NewAudioDetector()

	tests := []struct {
		name     string
		tracks   []AudioTrack
		language string
		expected bool
	}{
		{
			name: "single track match",
			tracks: []AudioTrack{
				{Index: 0, Language: "eng", Codec: "aac"},
			},
			language: "eng",
			expected: true,
		},
		{
			name: "multiple tracks with match",
			tracks: []AudioTrack{
				{Index: 0, Language: "jpn", Codec: "aac"},
				{Index: 1, Language: "eng", Codec: "ac3"},
			},
			language: "eng",
			expected: true,
		},
		{
			name: "no match",
			tracks: []AudioTrack{
				{Index: 0, Language: "jpn", Codec: "aac"},
				{Index: 1, Language: "kor", Codec: "ac3"},
			},
			language: "eng",
			expected: false,
		},
		{
			name: "ISO 639-1 vs 639-2 match",
			tracks: []AudioTrack{
				{Index: 0, Language: "eng", Codec: "aac"},
			},
			language: "en",
			expected: true,
		},
		{
			name: "case insensitive",
			tracks: []AudioTrack{
				{Index: 0, Language: "ENG", Codec: "aac"},
			},
			language: "eng",
			expected: true,
		},
		{
			name: "empty language",
			tracks: []AudioTrack{
				{Index: 0, Language: "eng", Codec: "aac"},
			},
			language: "",
			expected: false,
		},
		{
			name: "track with no language",
			tracks: []AudioTrack{
				{Index: 0, Language: "", Codec: "aac"},
			},
			language: "eng",
			expected: false,
		},
		{
			name:     "empty tracks",
			tracks:   []AudioTrack{},
			language: "eng",
			expected: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := detector.HasLanguage(tt.tracks, tt.language)

			if result != tt.expected {
				t.Errorf("HasLanguage(%v, %q) = %v, want %v", tt.tracks, tt.language, result, tt.expected)
			}
		})
	}
}

// TestAudioDetector_ExtractAudioTracks tests audio track extraction from FFprobe output
func TestAudioDetector_ExtractAudioTracks(t *testing.T) {
	detector := NewAudioDetector()

	tests := []struct {
		name     string
		probe    *FFProbeOutput
		expected []AudioTrack
	}{
		{
			name: "single audio track",
			probe: &FFProbeOutput{
				Streams: []FFProbeStream{
					{Index: 0, CodecType: "video", CodecName: "h264"},
					{Index: 1, CodecType: "audio", CodecName: "aac", Channels: 2, Tags: FFProbeStreamTags{Language: "eng", Title: "English"}},
				},
			},
			expected: []AudioTrack{
				{Index: 1, Language: "eng", Title: "English", Codec: "aac", Channels: 2},
			},
		},
		{
			name: "multiple audio tracks",
			probe: &FFProbeOutput{
				Streams: []FFProbeStream{
					{Index: 0, CodecType: "video", CodecName: "h264"},
					{Index: 1, CodecType: "audio", CodecName: "aac", Channels: 2, Tags: FFProbeStreamTags{Language: "jpn", Title: "Japanese"}},
					{Index: 2, CodecType: "audio", CodecName: "ac3", Channels: 6, Tags: FFProbeStreamTags{Language: "eng", Title: "English"}},
				},
			},
			expected: []AudioTrack{
				{Index: 1, Language: "jpn", Title: "Japanese", Codec: "aac", Channels: 2},
				{Index: 2, Language: "eng", Title: "English", Codec: "ac3", Channels: 6},
			},
		},
		{
			name: "no audio tracks",
			probe: &FFProbeOutput{
				Streams: []FFProbeStream{
					{Index: 0, CodecType: "video", CodecName: "h264"},
				},
			},
			expected: []AudioTrack{},
		},
		{
			name: "audio track with no language",
			probe: &FFProbeOutput{
				Streams: []FFProbeStream{
					{Index: 0, CodecType: "video", CodecName: "h264"},
					{Index: 1, CodecType: "audio", CodecName: "aac", Channels: 2},
				},
			},
			expected: []AudioTrack{
				{Index: 1, Language: "", Title: "", Codec: "aac", Channels: 2},
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := detector.extractAudioTracks(tt.probe)

			if len(result) != len(tt.expected) {
				t.Errorf("extractAudioTracks() length = %d, want %d", len(result), len(tt.expected))
				return
			}

			for i := range result {
				if result[i].Index != tt.expected[i].Index ||
					result[i].Language != tt.expected[i].Language ||
					result[i].Title != tt.expected[i].Title ||
					result[i].Codec != tt.expected[i].Codec ||
					result[i].Channels != tt.expected[i].Channels {
					t.Errorf("extractAudioTracks()[%d] = %+v, want %+v", i, result[i], tt.expected[i])
				}
			}
		})
	}
}

// TestAudioDetector_HasAnyPreferredLanguage tests preferred audio language filtering
// This is the NEW method for STORY_05
func TestAudioDetector_HasAnyPreferredLanguage(t *testing.T) {
	detector := NewAudioDetector()

	tests := []struct {
		name           string
		tracks         []AudioTrack
		preferredLangs []string
		expected       bool
	}{
		{
			name: "single track matches single preferred",
			tracks: []AudioTrack{
				{Index: 1, Language: "eng", Codec: "aac"},
			},
			preferredLangs: []string{"eng"},
			expected:       true,
		},
		{
			name: "single track matches one of multiple preferred",
			tracks: []AudioTrack{
				{Index: 1, Language: "jpn", Codec: "aac"},
			},
			preferredLangs: []string{"eng", "jpn", "kor"},
			expected:       true,
		},
		{
			name: "multiple tracks, one matches",
			tracks: []AudioTrack{
				{Index: 1, Language: "fre", Codec: "aac"},
				{Index: 2, Language: "eng", Codec: "ac3"},
			},
			preferredLangs: []string{"eng"},
			expected:       true,
		},
		{
			name: "multiple tracks, none match",
			tracks: []AudioTrack{
				{Index: 1, Language: "fre", Codec: "aac"},
				{Index: 2, Language: "spa", Codec: "ac3"},
			},
			preferredLangs: []string{"eng", "jpn"},
			expected:       false,
		},
		{
			name: "ISO 639-1 vs 639-2 matching",
			tracks: []AudioTrack{
				{Index: 1, Language: "en", Codec: "aac"},
			},
			preferredLangs: []string{"eng", "jpn", "kor"},
			expected:       true,
		},
		{
			name: "case insensitive matching",
			tracks: []AudioTrack{
				{Index: 1, Language: "ENG", Codec: "aac"},
			},
			preferredLangs: []string{"eng"},
			expected:       true,
		},
		{
			name:           "empty tracks",
			tracks:         []AudioTrack{},
			preferredLangs: []string{"eng"},
			expected:       false,
		},
		{
			name: "empty preferred list",
			tracks: []AudioTrack{
				{Index: 1, Language: "eng", Codec: "aac"},
			},
			preferredLangs: []string{},
			expected:       false,
		},
		{
			name: "nil preferred list",
			tracks: []AudioTrack{
				{Index: 1, Language: "eng", Codec: "aac"},
			},
			preferredLangs: nil,
			expected:       false,
		},
		{
			name: "track with no language metadata",
			tracks: []AudioTrack{
				{Index: 1, Language: "", Codec: "aac"},
			},
			preferredLangs: []string{"eng"},
			expected:       false,
		},
		{
			name: "multiple tracks with no language, one with preferred",
			tracks: []AudioTrack{
				{Index: 1, Language: "", Codec: "aac"},
				{Index: 2, Language: "eng", Codec: "ac3"},
			},
			preferredLangs: []string{"eng"},
			expected:       true,
		},
		{
			name: "whitespace in preferred list (should not happen after ParseLanguageList, but test defensive)",
			tracks: []AudioTrack{
				{Index: 1, Language: "eng", Codec: "aac"},
			},
			preferredLangs: []string{" eng ", "jpn "},
			expected:       false, // Whitespace is not trimmed in this method, should be done by ParseLanguageList
		},
		{
			name: "mixed ISO codes in tracks and preferred",
			tracks: []AudioTrack{
				{Index: 1, Language: "en", Codec: "aac"},
			},
			preferredLangs: []string{"eng", "ja", "ko"},
			expected:       true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := detector.HasAnyPreferredLanguage(tt.tracks, tt.preferredLangs)

			if result != tt.expected {
				t.Errorf("HasAnyPreferredLanguage(tracks=%+v, preferredLangs=%v) = %v, want %v",
					tt.tracks, tt.preferredLangs, result, tt.expected)
			}
		})
	}
}
