package skip

import (
	"context"
	"testing"
)

// TestCheckerInterface verifies that BasicChecker implements Checker interface
func TestCheckerInterface(t *testing.T) {
	config := &Config{
		SkipIfTargetSubtitleExists: true,
	}

	checker, err := NewBasicChecker(config)
	if err != nil {
		t.Fatalf("Failed to create BasicChecker: %v", err)
	}

	// Verify it implements Checker interface
	var _ Checker = checker

	// Verify GetConfig works
	gotConfig := checker.GetConfig()
	if gotConfig == nil {
		t.Fatal("GetConfig() returned nil")
	}

	if gotConfig.SkipIfTargetSubtitleExists != config.SkipIfTargetSubtitleExists {
		t.Errorf("GetConfig() returned wrong config, got %v, want %v",
			gotConfig.SkipIfTargetSubtitleExists, config.SkipIfTargetSubtitleExists)
	}
}

// TestCheckResultStructure verifies CheckResult structure
func TestCheckResultStructure(t *testing.T) {
	result := &CheckResult{
		ShouldSkip: true,
		Reason:     ReasonSubtitleExists,
		Details:    "test details",
	}

	if !result.ShouldSkip {
		t.Error("ShouldSkip should be true")
	}

	if result.Reason != ReasonSubtitleExists {
		t.Errorf("Reason = %v, want %v", result.Reason, ReasonSubtitleExists)
	}

	if result.Details != "test details" {
		t.Errorf("Details = %v, want %v", result.Details, "test details")
	}
}

// TestSkipReasonConstants verifies skip reason constants are defined
func TestSkipReasonConstants(t *testing.T) {
	tests := []struct {
		name   string
		reason SkipReason
		want   string
	}{
		{"SubtitleExists", ReasonSubtitleExists, "subtitle_file_exists"},
		{"LRCExists", ReasonLRCExists, "lrc_file_exists"},
		{"NotApplicable", ReasonNotApplicable, "not_applicable"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if string(tt.reason) != tt.want {
				t.Errorf("SkipReason %s = %v, want %v", tt.name, tt.reason, tt.want)
			}
		})
	}
}

// TestCheckContextCancellation verifies Check respects context cancellation
func TestCheckContextCancellation(t *testing.T) {
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
	// For basic file existence, we don't need to check cancellation,
	// but the interface accepts context for future use
	_, _ = checker.Check(ctx, "testdata/video.mkv")
	// Error handling depends on implementation
	// For now, we just verify the call doesn't panic
}

// Benchmarks for STORY_07 performance requirements

// BenchmarkBasicChecker_Check_FileExists benchmarks basic file existence check
// Target: < 50ms per operation
func BenchmarkBasicChecker_Check_FileExists(b *testing.B) {
	config := &Config{
		SkipIfTargetSubtitleExists: true,
	}

	checker, err := NewBasicChecker(config)
	if err != nil {
		b.Fatalf("Failed to create BasicChecker: %v", err)
	}

	ctx := context.Background()
	filePath := "testdata/video.mkv"

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_, _ = checker.Check(ctx, filePath)
	}
}

// BenchmarkBasicChecker_Check_EmbeddedDetection benchmarks embedded subtitle detection
// Target: < 100ms per operation
func BenchmarkBasicChecker_Check_EmbeddedDetection(b *testing.B) {
	config := &Config{
		SkipIfTargetSubtitleExists:      true,
		CheckEmbeddedSubtitles:          true,
		SkipIfInternalSubtitlesLanguage: "eng",
	}

	checker, err := NewBasicChecker(config)
	if err != nil {
		b.Fatalf("Failed to create BasicChecker: %v", err)
	}

	ctx := context.Background()
	filePath := "testdata/video_with_subs.mkv"

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_, _ = checker.Check(ctx, filePath)
	}
}

// BenchmarkBasicChecker_Check_ExternalScan benchmarks external subtitle scanning
// Target: < 200ms per operation
func BenchmarkBasicChecker_Check_ExternalScan(b *testing.B) {
	config := &Config{
		SkipIfTargetSubtitleExists:      true,
		SkipIfExternalSubtitlesExist:    true,
		SkipIfInternalSubtitlesLanguage: "eng",
	}

	checker, err := NewBasicChecker(config)
	if err != nil {
		b.Fatalf("Failed to create BasicChecker: %v", err)
	}

	ctx := context.Background()
	filePath := "testdata/video.mkv"

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_, _ = checker.Check(ctx, filePath)
	}
}

// BenchmarkBasicChecker_Check_AllChecks benchmarks full skip check with all features
func BenchmarkBasicChecker_Check_AllChecks(b *testing.B) {
	config := &Config{
		SkipIfTargetSubtitleExists:      true,
		CheckEmbeddedSubtitles:          true,
		SkipIfExternalSubtitlesExist:    true,
		SkipIfInternalSubtitlesLanguage: "eng",
		SkipIfAudioLanguages:            []string{"jpn"},
		SkipUnknownLanguage:             true,
	}

	checker, err := NewBasicChecker(config)
	if err != nil {
		b.Fatalf("Failed to create BasicChecker: %v", err)
	}

	ctx := context.Background()
	filePath := "testdata/video.mkv"

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_, _ = checker.Check(ctx, filePath)
	}
}
