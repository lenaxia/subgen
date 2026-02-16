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

// TestBasicChecker_Check_Disabled tests that skip checking is disabled when configured
func TestBasicChecker_Check_Disabled(t *testing.T) {
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
		SkipIfTargetSubtitleExists: false, // Disabled
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
		t.Error("Expected ShouldSkip=false when skip checking disabled, got true")
	}

	if result.Reason != ReasonNotApplicable {
		t.Errorf("Expected Reason=%v, got %v", ReasonNotApplicable, result.Reason)
	}
}

// TestBasicChecker_HasSubtitlesDetection_WithExternalSRT tests hasSubtitles detection with external SRT
// This test verifies that when SkipIfExternalSubtitlesExist is enabled and an external subtitle
// with matching language is found, the file should be skipped
func TestBasicChecker_HasSubtitlesDetection_WithExternalSRT(t *testing.T) {
	tmpDir := t.TempDir()
	videoPath := filepath.Join(tmpDir, "video.mkv")
	srtPath := filepath.Join(tmpDir, "video.en.srt")

	// Create test files
	if err := os.WriteFile(videoPath, []byte("fake video"), 0644); err != nil {
		t.Fatalf("Failed to create test video: %v", err)
	}
	if err := os.WriteFile(srtPath, []byte("fake subtitle"), 0644); err != nil {
		t.Fatalf("Failed to create test subtitle: %v", err)
	}

	config := &Config{
		SkipIfTargetSubtitleExists:        true,  // Master switch must be enabled
		SkipIfExternalSubtitlesExist:      true,  // Enable external subtitle detection
		SkipIfInternalSubtitlesLanguage:   "en",  // Target language "en"
		SkipIfNoLanguageButSubtitlesExist: false, // Don't use advanced check for this test
	}
	checker, err := NewBasicChecker(config)
	if err != nil {
		t.Fatalf("Failed to create checker: %v", err)
	}

	result, err := checker.Check(context.Background(), videoPath)
	if err != nil {
		t.Fatalf("Check failed: %v", err)
	}

	// Should detect external subtitle and skip
	if !result.ShouldSkip {
		t.Errorf("Expected ShouldSkip=true when external subtitle exists, got false (reason: %v, details: %s)",
			result.Reason, result.Details)
	}

	if result.Reason != ReasonExternalSubtitle {
		t.Errorf("Expected Reason=%v, got %v (details: %s)", ReasonExternalSubtitle, result.Reason, result.Details)
	}
}

// TestBasicChecker_HasSubtitlesDetection_NoSubtitles tests hasSubtitles detection without any subtitles
func TestBasicChecker_HasSubtitlesDetection_NoSubtitles(t *testing.T) {
	tmpDir := t.TempDir()
	videoPath := filepath.Join(tmpDir, "video.mkv")

	// Create only video file, no subtitles
	if err := os.WriteFile(videoPath, []byte("fake video"), 0644); err != nil {
		t.Fatalf("Failed to create test video: %v", err)
	}

	config := &Config{
		SkipIfTargetSubtitleExists:        false, // Skip checking disabled - will return early
		SkipIfExternalSubtitlesExist:      false, // Don't check external
		CheckEmbeddedSubtitles:            false, // Don't check embedded
		SkipIfNoLanguageButSubtitlesExist: true,  // Enable the advanced check
	}
	checker, err := NewBasicChecker(config)
	if err != nil {
		t.Fatalf("Failed to create checker: %v", err)
	}

	result, err := checker.Check(context.Background(), videoPath)
	if err != nil {
		t.Fatalf("Check failed: %v", err)
	}

	// Should NOT skip because SkipIfTargetSubtitleExists=false (master switch is off)
	if result.ShouldSkip {
		t.Errorf("Expected ShouldSkip=false when no subtitles exist, got true (reason: %v, details: %s)",
			result.Reason, result.Details)
	}
}

// TestBasicChecker_HasSubtitlesDetection_WithBasicSRT tests hasSubtitles properly detects basic .srt
func TestBasicChecker_HasSubtitlesDetection_WithBasicSRT(t *testing.T) {
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
		SkipIfTargetSubtitleExists:        true, // Enable basic .srt check (will trigger early return)
		SkipIfNoLanguageButSubtitlesExist: true, // This check won't be reached
	}
	checker, err := NewBasicChecker(config)
	if err != nil {
		t.Fatalf("Failed to create checker: %v", err)
	}

	result, err := checker.Check(context.Background(), videoPath)
	if err != nil {
		t.Fatalf("Check failed: %v", err)
	}

	// Should skip due to basic .srt check (before hasSubtitles advanced check)
	if !result.ShouldSkip {
		t.Error("Expected ShouldSkip=true when basic .srt exists")
	}

	if result.Reason != ReasonSubtitleExists {
		t.Errorf("Expected Reason=%v, got %v", ReasonSubtitleExists, result.Reason)
	}
}
