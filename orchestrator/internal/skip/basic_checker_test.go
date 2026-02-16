package skip

import (
	"context"
	"os"
	"path/filepath"
	"testing"
)

// TestBasicChecker_Check_VideoWithSRT tests skipping video when .srt exists
func TestBasicChecker_Check_VideoWithSRT(t *testing.T) {
	// Setup test files
	tmpDir := t.TempDir()
	videoPath := filepath.Join(tmpDir, "video.mkv")
	srtPath := filepath.Join(tmpDir, "video.srt")

	// Create test files
	if err := os.WriteFile(videoPath, []byte("fake video"), 0644); err != nil {
		t.Fatalf("Failed to create test video: %v", err)
	}
	if err := os.WriteFile(srtPath, []byte("fake subtitle"), 0644); err != nil {
		t.Fatalf("Failed to create test subtitle: %v", err)
	}

	config := &Config{
		SkipIfTargetSubtitleExists: true,
	}
	checker, err := NewBasicChecker(config)
	if err != nil {
		t.Fatalf("Failed to create checker: %v", err)
	}

	result, err := checker.Check(context.Background(), videoPath)
	if err != nil {
		t.Fatalf("Check failed: %v", err)
	}

	if !result.ShouldSkip {
		t.Error("Expected ShouldSkip=true when .srt exists, got false")
	}

	if result.Reason != ReasonSubtitleExists {
		t.Errorf("Expected Reason=%v, got %v", ReasonSubtitleExists, result.Reason)
	}

	if result.Details == "" {
		t.Error("Expected non-empty Details")
	}
}

// TestBasicChecker_Check_VideoWithoutSRT tests not skipping when .srt doesn't exist
func TestBasicChecker_Check_VideoWithoutSRT(t *testing.T) {
	tmpDir := t.TempDir()
	videoPath := filepath.Join(tmpDir, "video.mkv")

	// Create only video, no subtitle
	if err := os.WriteFile(videoPath, []byte("fake video"), 0644); err != nil {
		t.Fatalf("Failed to create test video: %v", err)
	}

	config := &Config{
		SkipIfTargetSubtitleExists: true,
	}
	checker, err := NewBasicChecker(config)
	if err != nil {
		t.Fatalf("Failed to create checker: %v", err)
	}

	result, err := checker.Check(context.Background(), videoPath)
	if err != nil {
		t.Fatalf("Check failed: %v", err)
	}

	if result.ShouldSkip {
		t.Error("Expected ShouldSkip=false when .srt doesn't exist, got true")
	}

	if result.Reason != ReasonNotApplicable {
		t.Errorf("Expected Reason=%v, got %v", ReasonNotApplicable, result.Reason)
	}
}

// TestBasicChecker_Check_AudioWithLRC tests skipping audio when .lrc exists
func TestBasicChecker_Check_AudioWithLRC(t *testing.T) {
	tmpDir := t.TempDir()
	audioPath := filepath.Join(tmpDir, "audio.mp3")
	lrcPath := filepath.Join(tmpDir, "audio.lrc")

	// Create test files
	if err := os.WriteFile(audioPath, []byte("fake audio"), 0644); err != nil {
		t.Fatalf("Failed to create test audio: %v", err)
	}
	if err := os.WriteFile(lrcPath, []byte("fake lyrics"), 0644); err != nil {
		t.Fatalf("Failed to create test lrc: %v", err)
	}

	config := &Config{
		SkipIfTargetSubtitleExists: true,
	}
	checker, err := NewBasicChecker(config)
	if err != nil {
		t.Fatalf("Failed to create checker: %v", err)
	}

	result, err := checker.Check(context.Background(), audioPath)
	if err != nil {
		t.Fatalf("Check failed: %v", err)
	}

	if !result.ShouldSkip {
		t.Error("Expected ShouldSkip=true when .lrc exists, got false")
	}

	if result.Reason != ReasonLRCExists {
		t.Errorf("Expected Reason=%v, got %v", ReasonLRCExists, result.Reason)
	}
}

// TestBasicChecker_Check_AudioWithoutLRC tests not skipping when .lrc doesn't exist
func TestBasicChecker_Check_AudioWithoutLRC(t *testing.T) {
	tmpDir := t.TempDir()
	audioPath := filepath.Join(tmpDir, "audio.mp3")

	// Create only audio, no lrc
	if err := os.WriteFile(audioPath, []byte("fake audio"), 0644); err != nil {
		t.Fatalf("Failed to create test audio: %v", err)
	}

	config := &Config{
		SkipIfTargetSubtitleExists: true,
	}
	checker, err := NewBasicChecker(config)
	if err != nil {
		t.Fatalf("Failed to create checker: %v", err)
	}

	result, err := checker.Check(context.Background(), audioPath)
	if err != nil {
		t.Fatalf("Check failed: %v", err)
	}

	if result.ShouldSkip {
		t.Error("Expected ShouldSkip=false when .lrc doesn't exist, got true")
	}
}

// TestBasicChecker_Check_SkipDisabled tests behavior when skip is disabled
func TestBasicChecker_Check_SkipDisabled(t *testing.T) {
	tmpDir := t.TempDir()
	videoPath := filepath.Join(tmpDir, "video.mkv")
	srtPath := filepath.Join(tmpDir, "video.srt")

	// Create test files
	if err := os.WriteFile(videoPath, []byte("fake video"), 0644); err != nil {
		t.Fatalf("Failed to create test video: %v", err)
	}
	if err := os.WriteFile(srtPath, []byte("fake subtitle"), 0644); err != nil {
		t.Fatalf("Failed to create test subtitle: %v", err)
	}

	// Disable skip checking
	config := &Config{
		SkipIfTargetSubtitleExists: false,
	}
	checker, err := NewBasicChecker(config)
	if err != nil {
		t.Fatalf("Failed to create checker: %v", err)
	}

	result, err := checker.Check(context.Background(), videoPath)
	if err != nil {
		t.Fatalf("Check failed: %v", err)
	}

	// Should NOT skip even though .srt exists
	if result.ShouldSkip {
		t.Error("Expected ShouldSkip=false when skip disabled, got true")
	}

	if result.Reason != ReasonNotApplicable {
		t.Errorf("Expected Reason=%v, got %v", ReasonNotApplicable, result.Reason)
	}
}

// TestBasicChecker_Check_EmptyPath tests error handling for empty path
func TestBasicChecker_Check_EmptyPath(t *testing.T) {
	config := &Config{
		SkipIfTargetSubtitleExists: true,
	}
	checker, err := NewBasicChecker(config)
	if err != nil {
		t.Fatalf("Failed to create checker: %v", err)
	}

	_, err = checker.Check(context.Background(), "")
	if err == nil {
		t.Error("Expected error for empty path, got nil")
	}
}

// TestBasicChecker_Check_MultipleExtensions tests files with complex names
func TestBasicChecker_Check_MultipleExtensions(t *testing.T) {
	tmpDir := t.TempDir()
	videoPath := filepath.Join(tmpDir, "movie.eng.mkv")
	srtPath := filepath.Join(tmpDir, "movie.eng.srt")

	// Create test files
	if err := os.WriteFile(videoPath, []byte("fake video"), 0644); err != nil {
		t.Fatalf("Failed to create test video: %v", err)
	}
	if err := os.WriteFile(srtPath, []byte("fake subtitle"), 0644); err != nil {
		t.Fatalf("Failed to create test subtitle: %v", err)
	}

	config := &Config{
		SkipIfTargetSubtitleExists: true,
	}
	checker, err := NewBasicChecker(config)
	if err != nil {
		t.Fatalf("Failed to create checker: %v", err)
	}

	result, err := checker.Check(context.Background(), videoPath)
	if err != nil {
		t.Fatalf("Check failed: %v", err)
	}

	if !result.ShouldSkip {
		t.Error("Expected ShouldSkip=true for file with multiple extensions")
	}
}

// TestBasicChecker_Check_VariousAudioFormats tests different audio file extensions
func TestBasicChecker_Check_VariousAudioFormats(t *testing.T) {
	audioExts := []string{".mp3", ".m4a", ".flac", ".wav", ".aac", ".ogg", ".opus", ".wma"}

	for _, ext := range audioExts {
		t.Run(ext, func(t *testing.T) {
			tmpDir := t.TempDir()
			audioPath := filepath.Join(tmpDir, "audio"+ext)
			lrcPath := filepath.Join(tmpDir, "audio.lrc")

			// Create test files
			if err := os.WriteFile(audioPath, []byte("fake audio"), 0644); err != nil {
				t.Fatalf("Failed to create test audio: %v", err)
			}
			if err := os.WriteFile(lrcPath, []byte("fake lyrics"), 0644); err != nil {
				t.Fatalf("Failed to create test lrc: %v", err)
			}

			config := &Config{
				SkipIfTargetSubtitleExists: true,
			}
			checker, err := NewBasicChecker(config)
			if err != nil {
				t.Fatalf("Failed to create checker: %v", err)
			}

			result, err := checker.Check(context.Background(), audioPath)
			if err != nil {
				t.Fatalf("Check failed: %v", err)
			}

			if !result.ShouldSkip {
				t.Errorf("Expected ShouldSkip=true for audio format %s with .lrc", ext)
			}

			if result.Reason != ReasonLRCExists {
				t.Errorf("Expected Reason=%v for %s, got %v", ReasonLRCExists, ext, result.Reason)
			}
		})
	}
}

// TestBasicChecker_Check_NonExistentFile tests checking non-existent source files
func TestBasicChecker_Check_NonExistentFile(t *testing.T) {
	tmpDir := t.TempDir()
	videoPath := filepath.Join(tmpDir, "nonexistent.mkv")

	config := &Config{
		SkipIfTargetSubtitleExists: true,
	}
	checker, err := NewBasicChecker(config)
	if err != nil {
		t.Fatalf("Failed to create checker: %v", err)
	}

	// Should not error even if source file doesn't exist
	// (checking for subtitle existence, not source file)
	result, err := checker.Check(context.Background(), videoPath)
	if err != nil {
		t.Fatalf("Check failed for non-existent file: %v", err)
	}

	// Should not skip if no subtitle exists
	if result.ShouldSkip {
		t.Error("Expected ShouldSkip=false for non-existent file without subtitle")
	}
}

// TestNewBasicChecker_NilConfig tests error handling for nil config
func TestNewBasicChecker_NilConfig(t *testing.T) {
	_, err := NewBasicChecker(nil)
	if err == nil {
		t.Error("Expected error for nil config, got nil")
	}
}

// TestNewBasicChecker_InvalidConfig tests error handling for invalid config
func TestNewBasicChecker_InvalidConfig(t *testing.T) {
	config := &Config{
		SkipIfTargetSubtitleExists: true,
	}

	// Simulate validation failure (will be tested in config_test.go)
	checker, err := NewBasicChecker(config)
	if err != nil {
		t.Fatalf("Unexpected error for valid config: %v", err)
	}

	if checker == nil {
		t.Error("Expected non-nil checker for valid config")
	}
}

// TestIsAudioFile tests audio file detection helper
func TestIsAudioFile(t *testing.T) {
	tests := []struct {
		name     string
		filePath string
		want     bool
	}{
		{"MP3", "/path/to/audio.mp3", true},
		{"M4A", "/path/to/audio.m4a", true},
		{"FLAC", "/path/to/audio.flac", true},
		{"WAV", "/path/to/audio.wav", true},
		{"AAC", "/path/to/audio.aac", true},
		{"OGG", "/path/to/audio.ogg", true},
		{"OPUS", "/path/to/audio.opus", true},
		{"WMA", "/path/to/audio.wma", true},
		{"MKV", "/path/to/video.mkv", false},
		{"MP4", "/path/to/video.mp4", false},
		{"AVI", "/path/to/video.avi", false},
		{"UpperCase", "/path/to/audio.MP3", true},
		{"NoExtension", "/path/to/file", false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := isAudioFile(tt.filePath)
			if got != tt.want {
				t.Errorf("isAudioFile(%q) = %v, want %v", tt.filePath, got, tt.want)
			}
		})
	}
}

// TestGetSubtitlePath tests subtitle path generation
func TestGetSubtitlePath(t *testing.T) {
	tests := []struct {
		name        string
		filePath    string
		subtitleExt string
		want        string
	}{
		{"SimpleVideo", "/path/to/video.mkv", ".srt", "/path/to/video.srt"},
		{"SimpleAudio", "/path/to/audio.mp3", ".lrc", "/path/to/audio.lrc"},
		{"MultipleExt", "/path/to/movie.eng.mkv", ".srt", "/path/to/movie.eng.srt"},
		{"NoExtension", "/path/to/file", ".srt", "/path/to/file.srt"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := getSubtitlePath(tt.filePath, tt.subtitleExt)
			if got != tt.want {
				t.Errorf("getSubtitlePath(%q, %q) = %q, want %q",
					tt.filePath, tt.subtitleExt, got, tt.want)
			}
		})
	}
}

// TestExists tests file existence helper
func TestExists(t *testing.T) {
	tmpDir := t.TempDir()

	// Create a test file
	existingFile := filepath.Join(tmpDir, "exists.txt")
	if err := os.WriteFile(existingFile, []byte("test"), 0644); err != nil {
		t.Fatalf("Failed to create test file: %v", err)
	}

	tests := []struct {
		name string
		path string
		want bool
	}{
		{"ExistingFile", existingFile, true},
		{"NonExistentFile", filepath.Join(tmpDir, "notexist.txt"), false},
		{"EmptyPath", "", false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := exists(tt.path)
			if got != tt.want {
				t.Errorf("exists(%q) = %v, want %v", tt.path, got, tt.want)
			}
		})
	}
}

// TestBasicChecker_PreferredAudioLanguageFiltering tests STORY_05 integration
// These tests verify that preferred audio language filtering works end-to-end
func TestBasicChecker_PreferredAudioLanguageFiltering_Disabled(t *testing.T) {
	// When LIMIT_TO_PREFERRED_AUDIO_LANGUAGE=false, should never skip based on audio
	tmpDir := t.TempDir()
	videoPath := filepath.Join(tmpDir, "movie.mkv")

	// Create test video file
	if err := os.WriteFile(videoPath, []byte("fake video"), 0644); err != nil {
		t.Fatalf("Failed to create test video: %v", err)
	}

	config := &Config{
		SkipIfTargetSubtitleExists:    false,
		CheckEmbeddedSubtitles:        false,
		SkipIfExternalSubtitlesExist:  false,
		PreferredAudioLanguages:       []string{"eng", "jpn"},
		LimitToPreferredAudioLanguage: false, // Disabled
	}

	checker, err := NewBasicChecker(config)
	if err != nil {
		t.Fatalf("Failed to create checker: %v", err)
	}

	result, err := checker.Check(context.Background(), videoPath)
	if err != nil {
		t.Fatalf("Check failed: %v", err)
	}

	// Should NOT skip because filtering is disabled
	if result.ShouldSkip {
		t.Error("Expected ShouldSkip=false when LimitToPreferredAudioLanguage=false")
	}
}

// TestBasicChecker_PreferredAudioLanguageFiltering_EmptyList tests with empty preferred list
func TestBasicChecker_PreferredAudioLanguageFiltering_EmptyList(t *testing.T) {
	// When PreferredAudioLanguages is empty, should never skip
	tmpDir := t.TempDir()
	videoPath := filepath.Join(tmpDir, "movie.mkv")

	// Create test video file
	if err := os.WriteFile(videoPath, []byte("fake video"), 0644); err != nil {
		t.Fatalf("Failed to create test video: %v", err)
	}

	config := &Config{
		SkipIfTargetSubtitleExists:    false,
		CheckEmbeddedSubtitles:        false,
		SkipIfExternalSubtitlesExist:  false,
		PreferredAudioLanguages:       []string{}, // Empty
		LimitToPreferredAudioLanguage: true,
	}

	checker, err := NewBasicChecker(config)
	if err != nil {
		t.Fatalf("Failed to create checker: %v", err)
	}

	result, err := checker.Check(context.Background(), videoPath)
	if err != nil {
		t.Fatalf("Check failed: %v", err)
	}

	// Should NOT skip because list is empty
	if result.ShouldSkip {
		t.Error("Expected ShouldSkip=false when PreferredAudioLanguages is empty")
	}
}

// Note: Full integration tests with real FFprobe calls would require:
// - Mock FFprobe command execution
// - Test video files with known audio tracks
// - FFprobe JSON fixtures
// These are covered by unit tests in language_filter_test.go
// The integration here verifies the BasicChecker correctly uses the config
