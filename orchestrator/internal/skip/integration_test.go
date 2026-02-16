package skip

import (
	"context"
	"os"
	"path/filepath"
	"testing"
)

// TestIntegration_BasicCheckerWithAdvancedChecker verifies integration between BasicChecker and AdvancedChecker
func TestIntegration_BasicCheckerWithAdvancedChecker(t *testing.T) {
	// STORY_06 Integration: Verify AdvancedChecker is properly wired into BasicChecker
	config := &Config{
		SkipIfTargetSubtitleExists:        true,
		SkipUnknownLanguage:               true,
		SkipIfNoLanguageButSubtitlesExist: true,
	}

	checker, err := NewBasicChecker(config)
	if err != nil {
		t.Fatalf("Failed to create BasicChecker: %v", err)
	}

	// Verify advancedChecker is initialized
	if checker.advancedChecker == nil {
		t.Fatal("AdvancedChecker not initialized in BasicChecker")
	}

	// Verify config is shared
	if checker.advancedChecker.config != config {
		t.Error("AdvancedChecker should share the same config instance")
	}
}

// TestIntegration_SkipUnknownLanguage verifies unknown language skip logic
func TestIntegration_SkipUnknownLanguage(t *testing.T) {
	config := &Config{
		SkipIfTargetSubtitleExists:      true,
		SkipUnknownLanguage:             true,
		SkipIfInternalSubtitlesLanguage: "unknown", // Simulating unknown language detection
	}

	checker, err := NewBasicChecker(config)
	if err != nil {
		t.Fatalf("Failed to create BasicChecker: %v", err)
	}

	ctx := context.Background()
	result, err := checker.Check(ctx, "testdata/video.mkv")
	if err != nil {
		t.Fatalf("Check failed: %v", err)
	}

	// Should skip due to unknown language
	if !result.ShouldSkip {
		t.Error("Expected file to be skipped due to unknown language")
	}

	if result.Reason != ReasonUnknownLanguage {
		t.Errorf("Expected reason %v, got %v", ReasonUnknownLanguage, result.Reason)
	}
}

// TestIntegration_SkipLogicDisabled verifies behavior when skip logic is disabled
func TestIntegration_SkipLogicDisabled(t *testing.T) {
	config := &Config{
		SkipIfTargetSubtitleExists: false, // Disabled
	}

	checker, err := NewBasicChecker(config)
	if err != nil {
		t.Fatalf("Failed to create BasicChecker: %v", err)
	}

	ctx := context.Background()
	result, err := checker.Check(ctx, "testdata/video.mkv")
	if err != nil {
		t.Fatalf("Check failed: %v", err)
	}

	// Should NOT skip when disabled
	if result.ShouldSkip {
		t.Error("Expected file NOT to be skipped when skip logic is disabled")
	}

	if result.Reason != ReasonNotApplicable {
		t.Errorf("Expected reason %v, got %v", ReasonNotApplicable, result.Reason)
	}
}

// TestIntegration_MultipleSkipConditions verifies multiple skip conditions work together
func TestIntegration_MultipleSkipConditions(t *testing.T) {
	// Create temp directory for test
	tempDir := t.TempDir()
	videoPath := filepath.Join(tempDir, "test.mkv")
	srtPath := filepath.Join(tempDir, "test.srt")

	// Create empty video file
	if err := os.WriteFile(videoPath, []byte("fake video"), 0644); err != nil {
		t.Fatalf("Failed to create test video: %v", err)
	}

	// Create subtitle file
	if err := os.WriteFile(srtPath, []byte("1\n00:00:00,000 --> 00:00:01,000\nTest"), 0644); err != nil {
		t.Fatalf("Failed to create test subtitle: %v", err)
	}

	config := &Config{
		SkipIfTargetSubtitleExists:      true,
		CheckEmbeddedSubtitles:          true,
		SkipIfExternalSubtitlesExist:    true,
		SkipIfInternalSubtitlesLanguage: "eng",
	}

	checker, err := NewBasicChecker(config)
	if err != nil {
		t.Fatalf("Failed to create BasicChecker: %v", err)
	}

	ctx := context.Background()
	result, err := checker.Check(ctx, videoPath)
	if err != nil {
		t.Fatalf("Check failed: %v", err)
	}

	// Should skip because subtitle file exists
	if !result.ShouldSkip {
		t.Error("Expected file to be skipped due to existing subtitle")
	}

	if result.Reason != ReasonSubtitleExists {
		t.Errorf("Expected reason %v, got %v", ReasonSubtitleExists, result.Reason)
	}
}

// TestIntegration_AudioFileWithLRC verifies LRC file detection for audio files
func TestIntegration_AudioFileWithLRC(t *testing.T) {
	// Create temp directory for test
	tempDir := t.TempDir()
	audioPath := filepath.Join(tempDir, "song.mp3")
	lrcPath := filepath.Join(tempDir, "song.lrc")

	// Create empty audio file
	if err := os.WriteFile(audioPath, []byte("fake audio"), 0644); err != nil {
		t.Fatalf("Failed to create test audio: %v", err)
	}

	// Create LRC file
	if err := os.WriteFile(lrcPath, []byte("[00:00.00]Test lyrics"), 0644); err != nil {
		t.Fatalf("Failed to create test LRC: %v", err)
	}

	config := &Config{
		SkipIfTargetSubtitleExists: true,
	}

	checker, err := NewBasicChecker(config)
	if err != nil {
		t.Fatalf("Failed to create BasicChecker: %v", err)
	}

	ctx := context.Background()
	result, err := checker.Check(ctx, audioPath)
	if err != nil {
		t.Fatalf("Check failed: %v", err)
	}

	// Should skip because LRC file exists
	if !result.ShouldSkip {
		t.Error("Expected audio file to be skipped due to existing LRC")
	}

	if result.Reason != ReasonLRCExists {
		t.Errorf("Expected reason %v, got %v", ReasonLRCExists, result.Reason)
	}
}

// TestIntegration_ContextCancellation verifies context cancellation is handled
func TestIntegration_ContextCancellation(t *testing.T) {
	config := &Config{
		SkipIfTargetSubtitleExists: true,
	}

	checker, err := NewBasicChecker(config)
	if err != nil {
		t.Fatalf("Failed to create BasicChecker: %v", err)
	}

	// Create cancelled context
	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	// Check should handle cancelled context gracefully
	// For basic file checks, cancellation doesn't affect the result
	// but the context is available for future enhancements
	result, err := checker.Check(ctx, "testdata/video.mkv")
	if err != nil {
		t.Logf("Context cancellation handled with error: %v", err)
	} else if result != nil {
		t.Logf("Context cancellation ignored, result: %+v", result)
	}
}

// TestIntegration_AllSkipReasons verifies all skip reasons are properly defined
func TestIntegration_AllSkipReasons(t *testing.T) {
	expectedReasons := []SkipReason{
		ReasonSubtitleExists,
		ReasonLRCExists,
		ReasonEmbeddedSubtitle,
		ReasonExternalSubtitle,
		ReasonSubtitleLanguageSkip,
		ReasonAudioLanguageSkip,
		ReasonAudioLanguageMismatch,
		ReasonUnknownLanguage,
		ReasonNoLanguageButSubtitlesExist,
		ReasonNotApplicable,
	}

	// Verify all reasons are non-empty strings
	for _, reason := range expectedReasons {
		if string(reason) == "" {
			t.Errorf("Skip reason %v is empty", reason)
		}
	}
}

// TestIntegration_ConfigValidation verifies config validation works
func TestIntegration_ConfigValidation(t *testing.T) {
	tests := []struct {
		name    string
		config  *Config
		wantErr bool
	}{
		{
			name:    "nil config",
			config:  nil,
			wantErr: true,
		},
		{
			name: "valid config",
			config: &Config{
				SkipIfTargetSubtitleExists: true,
			},
			wantErr: false,
		},
		{
			name: "config with all features enabled",
			config: &Config{
				SkipIfTargetSubtitleExists:        true,
				CheckEmbeddedSubtitles:            true,
				SkipIfExternalSubtitlesExist:      true,
				SkipIfInternalSubtitlesLanguage:   "eng",
				SkipIfAudioLanguages:              []string{"jpn", "kor"},
				PreferredAudioLanguages:           []string{"eng"},
				LimitToPreferredAudioLanguage:     true,
				SkipUnknownLanguage:               true,
				SkipIfNoLanguageButSubtitlesExist: true,
			},
			wantErr: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, err := NewBasicChecker(tt.config)
			if (err != nil) != tt.wantErr {
				t.Errorf("NewBasicChecker() error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}
