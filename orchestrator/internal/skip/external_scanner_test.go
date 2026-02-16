package skip

import (
	"os"
	"path/filepath"
	"testing"
)

// TestExternalScanner_ScanForSubtitles_HappyPaths tests successful subtitle scanning
func TestExternalScanner_ScanForSubtitles_HappyPaths(t *testing.T) {
	tests := []struct {
		name          string
		setupFiles    map[string]string // filename -> content
		videoFile     string
		wantCount     int
		wantLanguages []string
		wantSubgen    []bool
		wantFormats   []string
	}{
		{
			name: "detect single English subtitle (ISO 639-2)",
			setupFiles: map[string]string{
				"movie.mkv":     "video",
				"movie.eng.srt": "subtitle",
			},
			videoFile:     "movie.mkv",
			wantCount:     1,
			wantLanguages: []string{"eng"},
			wantSubgen:    []bool{false},
			wantFormats:   []string{".srt"},
		},
		{
			name: "detect single English subtitle (ISO 639-1)",
			setupFiles: map[string]string{
				"movie.mkv":    "video",
				"movie.en.srt": "subtitle",
			},
			videoFile:     "movie.mkv",
			wantCount:     1,
			wantLanguages: []string{"en"},
			wantSubgen:    []bool{false},
			wantFormats:   []string{".srt"},
		},
		{
			name: "detect English subtitle (full name)",
			setupFiles: map[string]string{
				"movie.mkv":         "video",
				"movie.english.srt": "subtitle",
			},
			videoFile:     "movie.mkv",
			wantCount:     1,
			wantLanguages: []string{"english"},
			wantSubgen:    []bool{false},
			wantFormats:   []string{".srt"},
		},
		{
			name: "case insensitive matching",
			setupFiles: map[string]string{
				"movie.mkv":         "video",
				"movie.ENGLISH.srt": "subtitle",
			},
			videoFile:     "movie.mkv",
			wantCount:     1,
			wantLanguages: []string{"english"},
			wantSubgen:    []bool{false},
			wantFormats:   []string{".srt"},
		},
		{
			name: "detect subgen-generated subtitle",
			setupFiles: map[string]string{
				"movie.mkv":            "video",
				"movie.subgen.eng.srt": "subtitle",
			},
			videoFile:     "movie.mkv",
			wantCount:     1,
			wantLanguages: []string{"eng"},
			wantSubgen:    []bool{true},
			wantFormats:   []string{".srt"},
		},
		{
			name: "detect forced subtitle",
			setupFiles: map[string]string{
				"movie.mkv":            "video",
				"movie.forced.eng.srt": "subtitle",
			},
			videoFile:     "movie.mkv",
			wantCount:     1,
			wantLanguages: []string{"eng"},
			wantSubgen:    []bool{false},
			wantFormats:   []string{".srt"},
		},
		{
			name: "detect multiple subtitle formats",
			setupFiles: map[string]string{
				"movie.mkv":     "video",
				"movie.eng.srt": "subtitle1",
				"movie.eng.vtt": "subtitle2",
				"movie.eng.ass": "subtitle3",
			},
			videoFile:     "movie.mkv",
			wantCount:     3,
			wantLanguages: []string{"eng", "eng", "eng"},
			wantSubgen:    []bool{false, false, false},
			wantFormats:   []string{".ass", ".srt", ".vtt"}, // Alphabetical order from os.ReadDir
		},
		{
			name: "detect all 11 subtitle formats",
			setupFiles: map[string]string{
				"movie.mkv":      "video",
				"movie.eng.srt":  "sub1",
				"movie.eng.vtt":  "sub2",
				"movie.eng.sub":  "sub3",
				"movie.eng.ass":  "sub4",
				"movie.eng.ssa":  "sub5",
				"movie.eng.idx":  "sub6",
				"movie.eng.sbv":  "sub7",
				"movie.eng.pgs":  "sub8",
				"movie.eng.ttml": "sub9",
				"movie.eng.lrc":  "sub10",
				"movie.eng.smi":  "sub11",
			},
			videoFile: "movie.mkv",
			wantCount: 11,
			wantLanguages: []string{
				"eng", "eng", "eng", "eng", "eng",
				"eng", "eng", "eng", "eng", "eng", "eng",
			},
			wantSubgen: []bool{
				false, false, false, false, false,
				false, false, false, false, false, false,
			},
			wantFormats: []string{
				// Alphabetical order from os.ReadDir
				".ass", ".idx", ".lrc", ".pgs", ".sbv",
				".smi", ".srt", ".ssa", ".sub", ".ttml", ".vtt",
			},
		},
		{
			name: "detect multiple languages",
			setupFiles: map[string]string{
				"movie.mkv":     "video",
				"movie.eng.srt": "english",
				"movie.jpn.srt": "japanese",
				"movie.spa.srt": "spanish",
			},
			videoFile:     "movie.mkv",
			wantCount:     3,
			wantLanguages: []string{"eng", "jpn", "spa"},
			wantSubgen:    []bool{false, false, false},
			wantFormats:   []string{".srt", ".srt", ".srt"},
		},
		{
			name: "complex filename pattern",
			setupFiles: map[string]string{
				"movie.mkv":                   "video",
				"movie.subgen.forced.eng.srt": "subtitle",
			},
			videoFile:     "movie.mkv",
			wantCount:     1,
			wantLanguages: []string{"eng"},
			wantSubgen:    []bool{true},
			wantFormats:   []string{".srt"},
		},
		{
			name: "video with dots in name",
			setupFiles: map[string]string{
				"my.movie.2024.mkv":     "video",
				"my.movie.2024.eng.srt": "subtitle",
			},
			videoFile:     "my.movie.2024.mkv",
			wantCount:     1,
			wantLanguages: []string{"eng"},
			wantSubgen:    []bool{false},
			wantFormats:   []string{".srt"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Create temp directory
			tmpDir := t.TempDir()

			// Setup test files
			for filename, content := range tt.setupFiles {
				filePath := filepath.Join(tmpDir, filename)
				if err := os.WriteFile(filePath, []byte(content), 0644); err != nil {
					t.Fatalf("Failed to create test file %s: %v", filename, err)
				}
			}

			// Create scanner
			scanner := NewExternalScanner()

			// Scan for subtitles
			videoPath := filepath.Join(tmpDir, tt.videoFile)
			subtitles, err := scanner.ScanForSubtitles(videoPath)

			// Verify no error
			if err != nil {
				t.Fatalf("ScanForSubtitles() error = %v", err)
			}

			// Verify count
			if len(subtitles) != tt.wantCount {
				t.Errorf("ScanForSubtitles() got %d subtitles, want %d", len(subtitles), tt.wantCount)
			}

			// Verify each subtitle
			for i := 0; i < len(subtitles) && i < len(tt.wantLanguages); i++ {
				if subtitles[i].Language != tt.wantLanguages[i] {
					t.Errorf("subtitle[%d].Language = %q, want %q", i, subtitles[i].Language, tt.wantLanguages[i])
				}
				if subtitles[i].IsSubgenGenerated != tt.wantSubgen[i] {
					t.Errorf("subtitle[%d].IsSubgenGenerated = %v, want %v", i, subtitles[i].IsSubgenGenerated, tt.wantSubgen[i])
				}
				if subtitles[i].Format != tt.wantFormats[i] {
					t.Errorf("subtitle[%d].Format = %q, want %q", i, subtitles[i].Format, tt.wantFormats[i])
				}
			}
		})
	}
}

// TestExternalScanner_ScanForSubtitles_UnhappyPaths tests error cases
func TestExternalScanner_ScanForSubtitles_UnhappyPaths(t *testing.T) {
	tests := []struct {
		name      string
		filePath  string
		wantError bool
	}{
		{
			name:      "empty file path",
			filePath:  "",
			wantError: true,
		},
		{
			name:      "non-existent directory",
			filePath:  "/nonexistent/path/video.mkv",
			wantError: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			scanner := NewExternalScanner()
			_, err := scanner.ScanForSubtitles(tt.filePath)

			if tt.wantError && err == nil {
				t.Error("ScanForSubtitles() expected error, got nil")
			}
			if !tt.wantError && err != nil {
				t.Errorf("ScanForSubtitles() unexpected error: %v", err)
			}
		})
	}
}

// TestExternalScanner_ScanForSubtitles_NoSubtitles tests when no subtitles exist
func TestExternalScanner_ScanForSubtitles_NoSubtitles(t *testing.T) {
	tmpDir := t.TempDir()

	// Create only video file, no subtitles
	videoPath := filepath.Join(tmpDir, "movie.mkv")
	if err := os.WriteFile(videoPath, []byte("video"), 0644); err != nil {
		t.Fatalf("Failed to create video file: %v", err)
	}

	scanner := NewExternalScanner()
	subtitles, err := scanner.ScanForSubtitles(videoPath)

	if err != nil {
		t.Fatalf("ScanForSubtitles() error = %v", err)
	}

	if len(subtitles) != 0 {
		t.Errorf("ScanForSubtitles() got %d subtitles, want 0", len(subtitles))
	}
}

// TestExternalScanner_ScanForSubtitles_IgnoreOtherFiles tests filtering
func TestExternalScanner_ScanForSubtitles_IgnoreOtherFiles(t *testing.T) {
	tmpDir := t.TempDir()

	// Create test files
	files := map[string]string{
		"movie.mkv":          "video",
		"movie.eng.srt":      "subtitle",
		"othermovie.eng.srt": "other subtitle", // Should be ignored (different base name)
		"movie.txt":          "text file",      // Should be ignored (not a subtitle format)
		"movie.nfo":          "metadata",       // Should be ignored
		"readme.txt":         "readme",         // Should be ignored
	}

	for filename, content := range files {
		filePath := filepath.Join(tmpDir, filename)
		if err := os.WriteFile(filePath, []byte(content), 0644); err != nil {
			t.Fatalf("Failed to create test file %s: %v", filename, err)
		}
	}

	scanner := NewExternalScanner()
	videoPath := filepath.Join(tmpDir, "movie.mkv")
	subtitles, err := scanner.ScanForSubtitles(videoPath)

	if err != nil {
		t.Fatalf("ScanForSubtitles() error = %v", err)
	}

	// Should only find movie.eng.srt
	if len(subtitles) != 1 {
		t.Errorf("ScanForSubtitles() got %d subtitles, want 1", len(subtitles))
	}

	if len(subtitles) > 0 && subtitles[0].Language != "eng" {
		t.Errorf("subtitle.Language = %q, want %q", subtitles[0].Language, "eng")
	}
}

// TestExternalScanner_ParseLanguageFromFilename tests language parsing
func TestExternalScanner_ParseLanguageFromFilename(t *testing.T) {
	tests := []struct {
		name          string
		filename      string
		videoBaseName string
		wantLanguage  string
		wantFound     bool
	}{
		{
			name:          "ISO 639-2 English",
			filename:      "movie.eng.srt",
			videoBaseName: "movie",
			wantLanguage:  "eng",
			wantFound:     true,
		},
		{
			name:          "ISO 639-1 English",
			filename:      "movie.en.srt",
			videoBaseName: "movie",
			wantLanguage:  "en",
			wantFound:     true,
		},
		{
			name:          "full name English",
			filename:      "movie.english.srt",
			videoBaseName: "movie",
			wantLanguage:  "english",
			wantFound:     true,
		},
		{
			name:          "case insensitive",
			filename:      "movie.ENGLISH.srt",
			videoBaseName: "movie",
			wantLanguage:  "english",
			wantFound:     true,
		},
		{
			name:          "with subgen marker",
			filename:      "movie.subgen.eng.srt",
			videoBaseName: "movie",
			wantLanguage:  "eng",
			wantFound:     true,
		},
		{
			name:          "with forced marker",
			filename:      "movie.forced.eng.srt",
			videoBaseName: "movie",
			wantLanguage:  "eng",
			wantFound:     true,
		},
		{
			name:          "complex pattern",
			filename:      "movie.subgen.forced.eng.cc.srt",
			videoBaseName: "movie",
			wantLanguage:  "eng",
			wantFound:     true,
		},
		{
			name:          "Japanese",
			filename:      "movie.jpn.srt",
			videoBaseName: "movie",
			wantLanguage:  "jpn",
			wantFound:     true,
		},
		{
			name:          "no language code",
			filename:      "movie.srt",
			videoBaseName: "movie",
			wantLanguage:  "",
			wantFound:     false,
		},
		{
			name:          "only modifiers",
			filename:      "movie.forced.srt",
			videoBaseName: "movie",
			wantLanguage:  "",
			wantFound:     false,
		},
		{
			name:          "video with dots in name",
			filename:      "my.movie.2024.eng.srt",
			videoBaseName: "my.movie.2024",
			wantLanguage:  "eng",
			wantFound:     true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			scanner := NewExternalScanner()
			gotLanguage, gotFound := scanner.ParseLanguageFromFilename(tt.filename, tt.videoBaseName)

			if gotLanguage != tt.wantLanguage {
				t.Errorf("ParseLanguageFromFilename() language = %q, want %q", gotLanguage, tt.wantLanguage)
			}
			if gotFound != tt.wantFound {
				t.Errorf("ParseLanguageFromFilename() found = %v, want %v", gotFound, tt.wantFound)
			}
		})
	}
}

// TestExternalScanner_HasLanguage tests language matching
func TestExternalScanner_HasLanguage(t *testing.T) {
	tests := []struct {
		name       string
		subtitles  []ExternalSubtitle
		targetLang string
		want       bool
	}{
		{
			name: "exact match",
			subtitles: []ExternalSubtitle{
				{Language: "eng"},
			},
			targetLang: "eng",
			want:       true,
		},
		{
			name: "ISO 639-1 vs 639-2 match",
			subtitles: []ExternalSubtitle{
				{Language: "en"},
			},
			targetLang: "eng",
			want:       true,
		},
		{
			name: "ISO 639-2 vs 639-1 match",
			subtitles: []ExternalSubtitle{
				{Language: "eng"},
			},
			targetLang: "en",
			want:       true,
		},
		{
			name: "case insensitive match",
			subtitles: []ExternalSubtitle{
				{Language: "ENG"},
			},
			targetLang: "eng",
			want:       true,
		},
		{
			name: "no match",
			subtitles: []ExternalSubtitle{
				{Language: "jpn"},
			},
			targetLang: "eng",
			want:       false,
		},
		{
			name: "multiple subtitles with match",
			subtitles: []ExternalSubtitle{
				{Language: "jpn"},
				{Language: "eng"},
				{Language: "spa"},
			},
			targetLang: "eng",
			want:       true,
		},
		{
			name: "empty target language",
			subtitles: []ExternalSubtitle{
				{Language: "eng"},
			},
			targetLang: "",
			want:       false,
		},
		{
			name:       "empty subtitles list",
			subtitles:  []ExternalSubtitle{},
			targetLang: "eng",
			want:       false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			scanner := NewExternalScanner()
			got := scanner.HasLanguage(tt.subtitles, tt.targetLang)

			if got != tt.want {
				t.Errorf("HasLanguage() = %v, want %v", got, tt.want)
			}
		})
	}
}

// TestExternalScanner_IsSubgenGenerated tests subgen detection
func TestExternalScanner_IsSubgenGenerated(t *testing.T) {
	tests := []struct {
		name     string
		filename string
		want     bool
	}{
		{
			name:     "with subgen",
			filename: "movie.subgen.eng.srt",
			want:     true,
		},
		{
			name:     "with SUBGEN (case insensitive)",
			filename: "movie.SUBGEN.eng.srt",
			want:     true,
		},
		{
			name:     "without subgen",
			filename: "movie.eng.srt",
			want:     false,
		},
		{
			name:     "with forced only",
			filename: "movie.forced.eng.srt",
			want:     false,
		},
		{
			name:     "complex with subgen",
			filename: "movie.subgen.forced.eng.cc.srt",
			want:     true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			scanner := NewExternalScanner()
			got := scanner.IsSubgenGenerated(tt.filename)

			if got != tt.want {
				t.Errorf("IsSubgenGenerated() = %v, want %v", got, tt.want)
			}
		})
	}
}
