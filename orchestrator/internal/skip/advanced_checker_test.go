package skip

import (
	"testing"
)

// TestAdvancedChecker_CheckUnknownLanguage_HappyPaths tests successful unknown language detection
func TestAdvancedChecker_CheckUnknownLanguage_HappyPaths(t *testing.T) {
	tests := []struct {
		name         string
		detectedLang string
		skipEnabled  bool
		wantSkip     bool
		wantContains string
	}{
		{
			name:         "skip when language is empty string",
			detectedLang: "",
			skipEnabled:  true,
			wantSkip:     true,
			wantContains: "unknown",
		},
		{
			name:         "skip when language is 'unknown'",
			detectedLang: "unknown",
			skipEnabled:  true,
			wantSkip:     true,
			wantContains: "unknown",
		},
		{
			name:         "skip when language is 'undefined'",
			detectedLang: "undefined",
			skipEnabled:  true,
			wantSkip:     true,
			wantContains: "unknown",
		},
		{
			name:         "skip when language is 'und' (ISO 639-2)",
			detectedLang: "und",
			skipEnabled:  true,
			wantSkip:     true,
			wantContains: "unknown",
		},
		{
			name:         "don't skip when language is valid",
			detectedLang: "eng",
			skipEnabled:  true,
			wantSkip:     false,
			wantContains: "",
		},
		{
			name:         "don't skip when disabled even with unknown language",
			detectedLang: "",
			skipEnabled:  false,
			wantSkip:     false,
			wantContains: "",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			config := &Config{
				SkipUnknownLanguage: tt.skipEnabled,
			}

			checker, err := NewAdvancedChecker(config)
			if err != nil {
				t.Fatalf("NewAdvancedChecker() error = %v", err)
			}

			gotSkip, gotDetails := checker.CheckUnknownLanguage(tt.detectedLang)

			if gotSkip != tt.wantSkip {
				t.Errorf("CheckUnknownLanguage() gotSkip = %v, want %v", gotSkip, tt.wantSkip)
			}

			if tt.wantSkip && tt.wantContains != "" {
				if gotDetails == "" {
					t.Errorf("CheckUnknownLanguage() expected details containing %q, got empty string", tt.wantContains)
				}
			}

			if !tt.wantSkip && gotDetails != "" {
				t.Errorf("CheckUnknownLanguage() expected empty details when not skipping, got %q", gotDetails)
			}
		})
	}
}

// TestAdvancedChecker_CheckNoLanguageButSubtitlesExist_HappyPaths tests the no language but subs exist condition
func TestAdvancedChecker_CheckNoLanguageButSubtitlesExist_HappyPaths(t *testing.T) {
	tests := []struct {
		name         string
		detectedLang string
		hasSubtitles bool
		skipEnabled  bool
		wantSkip     bool
		wantContains string
	}{
		{
			name:         "skip when no language and has subtitles",
			detectedLang: "",
			hasSubtitles: true,
			skipEnabled:  true,
			wantSkip:     true,
			wantContains: "no language detected but subtitles exist",
		},
		{
			name:         "skip when unknown language and has subtitles",
			detectedLang: "unknown",
			hasSubtitles: true,
			skipEnabled:  true,
			wantSkip:     true,
			wantContains: "no language detected but subtitles exist",
		},
		{
			name:         "skip when undefined language and has subtitles",
			detectedLang: "undefined",
			hasSubtitles: true,
			skipEnabled:  true,
			wantSkip:     true,
			wantContains: "no language detected but subtitles exist",
		},
		{
			name:         "skip when und language and has subtitles",
			detectedLang: "und",
			hasSubtitles: true,
			skipEnabled:  true,
			wantSkip:     true,
			wantContains: "no language detected but subtitles exist",
		},
		{
			name:         "don't skip when has valid language",
			detectedLang: "eng",
			hasSubtitles: true,
			skipEnabled:  true,
			wantSkip:     false,
			wantContains: "",
		},
		{
			name:         "don't skip when no subtitles",
			detectedLang: "",
			hasSubtitles: false,
			skipEnabled:  true,
			wantSkip:     false,
			wantContains: "",
		},
		{
			name:         "don't skip when disabled",
			detectedLang: "",
			hasSubtitles: true,
			skipEnabled:  false,
			wantSkip:     false,
			wantContains: "",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			config := &Config{
				SkipIfNoLanguageButSubtitlesExist: tt.skipEnabled,
			}

			checker, err := NewAdvancedChecker(config)
			if err != nil {
				t.Fatalf("NewAdvancedChecker() error = %v", err)
			}

			gotSkip, gotDetails := checker.CheckNoLanguageButSubtitlesExist(tt.detectedLang, tt.hasSubtitles)

			if gotSkip != tt.wantSkip {
				t.Errorf("CheckNoLanguageButSubtitlesExist() gotSkip = %v, want %v", gotSkip, tt.wantSkip)
			}

			if tt.wantSkip && tt.wantContains != "" {
				if gotDetails == "" {
					t.Errorf("CheckNoLanguageButSubtitlesExist() expected details containing %q, got empty string", tt.wantContains)
				}
			}

			if !tt.wantSkip && gotDetails != "" {
				t.Errorf("CheckNoLanguageButSubtitlesExist() expected empty details when not skipping, got %q", gotDetails)
			}
		})
	}
}

// TestAdvancedChecker_CheckNoLanguageButSubtitlesExist_EdgeCases tests edge cases
func TestAdvancedChecker_CheckNoLanguageButSubtitlesExist_EdgeCases(t *testing.T) {
	tests := []struct {
		name         string
		detectedLang string
		hasSubtitles bool
		skipEnabled  bool
		wantSkip     bool
		description  string
	}{
		{
			name:         "both conditions required",
			detectedLang: "eng",
			hasSubtitles: false,
			skipEnabled:  true,
			wantSkip:     false,
			description:  "has language, no subtitles - don't skip",
		},
		{
			name:         "empty language empty subtitles",
			detectedLang: "",
			hasSubtitles: false,
			skipEnabled:  true,
			wantSkip:     false,
			description:  "no language, no subtitles - don't skip (attempt transcription)",
		},
		{
			name:         "case sensitive unknown check",
			detectedLang: "UNKNOWN",
			hasSubtitles: true,
			skipEnabled:  true,
			wantSkip:     false,
			description:  "UPPERCASE 'UNKNOWN' should not match (case sensitive)",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			config := &Config{
				SkipIfNoLanguageButSubtitlesExist: tt.skipEnabled,
			}

			checker, err := NewAdvancedChecker(config)
			if err != nil {
				t.Fatalf("NewAdvancedChecker() error = %v", err)
			}

			gotSkip, _ := checker.CheckNoLanguageButSubtitlesExist(tt.detectedLang, tt.hasSubtitles)

			if gotSkip != tt.wantSkip {
				t.Errorf("CheckNoLanguageButSubtitlesExist() %s: gotSkip = %v, want %v", tt.description, gotSkip, tt.wantSkip)
			}
		})
	}
}

// TestAdvancedChecker_NewAdvancedChecker_Validation tests constructor validation
func TestAdvancedChecker_NewAdvancedChecker_Validation(t *testing.T) {
	tests := []struct {
		name    string
		config  *Config
		wantErr bool
	}{
		{
			name:    "nil config returns error",
			config:  nil,
			wantErr: true,
		},
		{
			name: "valid config succeeds",
			config: &Config{
				SkipUnknownLanguage:               true,
				SkipIfNoLanguageButSubtitlesExist: true,
			},
			wantErr: false,
		},
		{
			name:    "empty config succeeds",
			config:  &Config{},
			wantErr: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, err := NewAdvancedChecker(tt.config)
			if (err != nil) != tt.wantErr {
				t.Errorf("NewAdvancedChecker() error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}

// TestIsUnknownLanguage tests the helper function
func TestIsUnknownLanguage(t *testing.T) {
	tests := []struct {
		name string
		lang string
		want bool
	}{
		{
			name: "empty string is unknown",
			lang: "",
			want: true,
		},
		{
			name: "unknown is unknown",
			lang: "unknown",
			want: true,
		},
		{
			name: "undefined is unknown",
			lang: "undefined",
			want: true,
		},
		{
			name: "und is unknown",
			lang: "und",
			want: true,
		},
		{
			name: "eng is not unknown",
			lang: "eng",
			want: false,
		},
		{
			name: "en is not unknown",
			lang: "en",
			want: false,
		},
		{
			name: "jpn is not unknown",
			lang: "jpn",
			want: false,
		},
		{
			name: "UNKNOWN uppercase is not unknown (case sensitive)",
			lang: "UNKNOWN",
			want: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := IsUnknownLanguage(tt.lang)
			if got != tt.want {
				t.Errorf("IsUnknownLanguage(%q) = %v, want %v", tt.lang, got, tt.want)
			}
		})
	}
}

// TestAdvancedChecker_Integration tests integration scenarios
func TestAdvancedChecker_Integration(t *testing.T) {
	t.Run("both checks can be enabled together", func(t *testing.T) {
		config := &Config{
			SkipUnknownLanguage:               true,
			SkipIfNoLanguageButSubtitlesExist: true,
		}

		checker, err := NewAdvancedChecker(config)
		if err != nil {
			t.Fatalf("NewAdvancedChecker() error = %v", err)
		}

		// Test SKIP_UNKNOWN_LANGUAGE triggers first
		shouldSkip, details := checker.CheckUnknownLanguage("")
		if !shouldSkip {
			t.Error("Expected CheckUnknownLanguage to skip with empty language")
		}
		if details == "" {
			t.Error("Expected details to be non-empty")
		}

		// Test SKIP_IF_NO_LANGUAGE_BUT_SUBTITLES_EXIST works independently
		shouldSkip, details = checker.CheckNoLanguageButSubtitlesExist("", true)
		if !shouldSkip {
			t.Error("Expected CheckNoLanguageButSubtitlesExist to skip")
		}
		if details == "" {
			t.Error("Expected details to be non-empty")
		}
	})

	t.Run("both checks can be disabled together", func(t *testing.T) {
		config := &Config{
			SkipUnknownLanguage:               false,
			SkipIfNoLanguageButSubtitlesExist: false,
		}

		checker, err := NewAdvancedChecker(config)
		if err != nil {
			t.Fatalf("NewAdvancedChecker() error = %v", err)
		}

		// Neither check should trigger
		shouldSkip, _ := checker.CheckUnknownLanguage("")
		if shouldSkip {
			t.Error("Expected CheckUnknownLanguage to NOT skip when disabled")
		}

		shouldSkip, _ = checker.CheckNoLanguageButSubtitlesExist("", true)
		if shouldSkip {
			t.Error("Expected CheckNoLanguageButSubtitlesExist to NOT skip when disabled")
		}
	})

	t.Run("selective enabling works", func(t *testing.T) {
		// Only enable SKIP_UNKNOWN_LANGUAGE
		config := &Config{
			SkipUnknownLanguage:               true,
			SkipIfNoLanguageButSubtitlesExist: false,
		}

		checker, err := NewAdvancedChecker(config)
		if err != nil {
			t.Fatalf("NewAdvancedChecker() error = %v", err)
		}

		// First check should trigger
		shouldSkip, _ := checker.CheckUnknownLanguage("")
		if !shouldSkip {
			t.Error("Expected CheckUnknownLanguage to skip")
		}

		// Second check should not trigger
		shouldSkip, _ = checker.CheckNoLanguageButSubtitlesExist("", true)
		if shouldSkip {
			t.Error("Expected CheckNoLanguageButSubtitlesExist to NOT skip when disabled")
		}
	})
}
