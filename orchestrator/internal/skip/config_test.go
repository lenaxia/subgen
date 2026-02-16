package skip

import (
	"os"
	"testing"
)

// TestNewConfig_Default tests default configuration values
func TestNewConfig_Default(t *testing.T) {
	// Clear environment variable
	os.Unsetenv("SKIP_IF_TARGET_SUBTITLES_EXIST")

	config, err := NewConfig()
	if err != nil {
		t.Fatalf("NewConfig() failed: %v", err)
	}

	if config == nil {
		t.Fatal("Expected non-nil config")
	}

	// Default should be true
	if !config.SkipIfTargetSubtitleExists {
		t.Error("Expected default SkipIfTargetSubtitleExists=true, got false")
	}
}

// TestNewConfig_ExplicitTrue tests explicit true configuration
func TestNewConfig_ExplicitTrue(t *testing.T) {
	os.Setenv("SKIP_IF_TARGET_SUBTITLES_EXIST", "true")
	defer os.Unsetenv("SKIP_IF_TARGET_SUBTITLES_EXIST")

	config, err := NewConfig()
	if err != nil {
		t.Fatalf("NewConfig() failed: %v", err)
	}

	if !config.SkipIfTargetSubtitleExists {
		t.Error("Expected SkipIfTargetSubtitleExists=true, got false")
	}
}

// TestNewConfig_ExplicitFalse tests explicit false configuration
func TestNewConfig_ExplicitFalse(t *testing.T) {
	os.Setenv("SKIP_IF_TARGET_SUBTITLES_EXIST", "false")
	defer os.Unsetenv("SKIP_IF_TARGET_SUBTITLES_EXIST")

	config, err := NewConfig()
	if err != nil {
		t.Fatalf("NewConfig() failed: %v", err)
	}

	if config.SkipIfTargetSubtitleExists {
		t.Error("Expected SkipIfTargetSubtitleExists=false, got true")
	}
}

// TestNewConfig_Variations tests various boolean string formats
func TestNewConfig_Variations(t *testing.T) {
	tests := []struct {
		name  string
		value string
		want  bool
	}{
		{"true", "true", true},
		{"True", "True", true},
		{"TRUE", "TRUE", true},
		{"1", "1", true},
		{"false", "false", false},
		{"False", "False", false},
		{"FALSE", "FALSE", false},
		{"0", "0", false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			os.Setenv("SKIP_IF_TARGET_SUBTITLES_EXIST", tt.value)
			defer os.Unsetenv("SKIP_IF_TARGET_SUBTITLES_EXIST")

			config, err := NewConfig()
			if err != nil {
				t.Fatalf("NewConfig() failed for value %q: %v", tt.value, err)
			}

			if config.SkipIfTargetSubtitleExists != tt.want {
				t.Errorf("For value %q: got %v, want %v",
					tt.value, config.SkipIfTargetSubtitleExists, tt.want)
			}
		})
	}
}

// TestNewConfig_InvalidValue tests error handling for invalid values
func TestNewConfig_InvalidValue(t *testing.T) {
	invalidValues := []string{"yes", "no", "maybe", "invalid", ""}

	for _, value := range invalidValues {
		t.Run(value, func(t *testing.T) {
			if value == "" {
				os.Unsetenv("SKIP_IF_TARGET_SUBTITLES_EXIST")
			} else {
				os.Setenv("SKIP_IF_TARGET_SUBTITLES_EXIST", value)
				defer os.Unsetenv("SKIP_IF_TARGET_SUBTITLES_EXIST")
			}

			config, err := NewConfig()

			// Empty string should use default (true), not error
			if value == "" {
				if err != nil {
					t.Errorf("Expected no error for empty value, got: %v", err)
				}
				if config == nil || !config.SkipIfTargetSubtitleExists {
					t.Error("Expected default config with SkipIfTargetSubtitleExists=true")
				}
			} else {
				// Invalid values should error
				if err == nil {
					t.Errorf("Expected error for invalid value %q, got nil", value)
				}
			}
		})
	}
}

// TestConfig_Validate tests configuration validation
func TestConfig_Validate(t *testing.T) {
	tests := []struct {
		name    string
		config  *Config
		wantErr bool
	}{
		{
			name: "ValidTrue",
			config: &Config{
				SkipIfTargetSubtitleExists: true,
			},
			wantErr: false,
		},
		{
			name: "ValidFalse",
			config: &Config{
				SkipIfTargetSubtitleExists: false,
			},
			wantErr: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := tt.config.Validate()
			if (err != nil) != tt.wantErr {
				t.Errorf("Validate() error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}

// TestConfig_Structure tests Config struct fields
func TestConfig_Structure(t *testing.T) {
	config := &Config{
		SkipIfTargetSubtitleExists: true,
	}

	// Verify field exists and has correct value
	if !config.SkipIfTargetSubtitleExists {
		t.Error("SkipIfTargetSubtitleExists field not set correctly")
	}

	// Verify field is accessible
	config.SkipIfTargetSubtitleExists = false
	if config.SkipIfTargetSubtitleExists {
		t.Error("SkipIfTargetSubtitleExists field not mutable")
	}
}

// TestNewConfig_PreferredAudioLanguages tests PREFERRED_AUDIO_LANGUAGES configuration (STORY_05)
func TestNewConfig_PreferredAudioLanguages(t *testing.T) {
	tests := []struct {
		name     string
		envValue string
		expected []string
	}{
		{
			name:     "default (empty)",
			envValue: "",
			expected: nil,
		},
		{
			name:     "single language",
			envValue: "eng",
			expected: []string{"eng"},
		},
		{
			name:     "multiple languages",
			envValue: "eng|jpn|kor",
			expected: []string{"eng", "jpn", "kor"},
		},
		{
			name:     "with whitespace",
			envValue: "eng | jpn | kor",
			expected: []string{"eng", "jpn", "kor"},
		},
		{
			name:     "mixed case",
			envValue: "ENG|JPN",
			expected: []string{"eng", "jpn"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if tt.envValue == "" {
				os.Unsetenv("PREFERRED_AUDIO_LANGUAGES")
			} else {
				os.Setenv("PREFERRED_AUDIO_LANGUAGES", tt.envValue)
				defer os.Unsetenv("PREFERRED_AUDIO_LANGUAGES")
			}

			config, err := NewConfig()
			if err != nil {
				t.Fatalf("NewConfig() failed: %v", err)
			}

			if len(config.PreferredAudioLanguages) != len(tt.expected) {
				t.Errorf("PreferredAudioLanguages length = %d, want %d",
					len(config.PreferredAudioLanguages), len(tt.expected))
				return
			}

			for i := range config.PreferredAudioLanguages {
				if config.PreferredAudioLanguages[i] != tt.expected[i] {
					t.Errorf("PreferredAudioLanguages[%d] = %q, want %q",
						i, config.PreferredAudioLanguages[i], tt.expected[i])
				}
			}
		})
	}
}

// TestNewConfig_LimitToPreferredAudioLanguage tests LIMIT_TO_PREFERRED_AUDIO_LANGUAGE configuration (STORY_05)
func TestNewConfig_LimitToPreferredAudioLanguage(t *testing.T) {
	tests := []struct {
		name     string
		envValue string
		expected bool
		wantErr  bool
	}{
		{
			name:     "default (false)",
			envValue: "",
			expected: false,
			wantErr:  false,
		},
		{
			name:     "explicit true",
			envValue: "true",
			expected: true,
			wantErr:  false,
		},
		{
			name:     "explicit false",
			envValue: "false",
			expected: false,
			wantErr:  false,
		},
		{
			name:     "case insensitive true",
			envValue: "TRUE",
			expected: true,
			wantErr:  false,
		},
		{
			name:     "numeric true",
			envValue: "1",
			expected: true,
			wantErr:  false,
		},
		{
			name:     "numeric false",
			envValue: "0",
			expected: false,
			wantErr:  false,
		},
		{
			name:     "invalid value",
			envValue: "maybe",
			expected: false,
			wantErr:  true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if tt.envValue == "" {
				os.Unsetenv("LIMIT_TO_PREFERRED_AUDIO_LANGUAGE")
			} else {
				os.Setenv("LIMIT_TO_PREFERRED_AUDIO_LANGUAGE", tt.envValue)
				defer os.Unsetenv("LIMIT_TO_PREFERRED_AUDIO_LANGUAGE")
			}

			config, err := NewConfig()

			if tt.wantErr {
				if err == nil {
					t.Error("Expected error, got nil")
				}
				return
			}

			if err != nil {
				t.Fatalf("NewConfig() failed: %v", err)
			}

			if config.LimitToPreferredAudioLanguage != tt.expected {
				t.Errorf("LimitToPreferredAudioLanguage = %v, want %v",
					config.LimitToPreferredAudioLanguage, tt.expected)
			}
		})
	}
}

// TestConfig_PreferredAudioLanguagesIntegration tests integration of preferred audio filtering (STORY_05)
func TestConfig_PreferredAudioLanguagesIntegration(t *testing.T) {
	// Set both env vars
	os.Setenv("PREFERRED_AUDIO_LANGUAGES", "eng|jpn")
	os.Setenv("LIMIT_TO_PREFERRED_AUDIO_LANGUAGE", "true")
	defer os.Unsetenv("PREFERRED_AUDIO_LANGUAGES")
	defer os.Unsetenv("LIMIT_TO_PREFERRED_AUDIO_LANGUAGE")

	config, err := NewConfig()
	if err != nil {
		t.Fatalf("NewConfig() failed: %v", err)
	}

	// Verify preferred languages
	if len(config.PreferredAudioLanguages) != 2 {
		t.Errorf("PreferredAudioLanguages length = %d, want 2", len(config.PreferredAudioLanguages))
	}

	if config.PreferredAudioLanguages[0] != "eng" || config.PreferredAudioLanguages[1] != "jpn" {
		t.Errorf("PreferredAudioLanguages = %v, want [eng jpn]", config.PreferredAudioLanguages)
	}

	// Verify limit flag
	if !config.LimitToPreferredAudioLanguage {
		t.Error("LimitToPreferredAudioLanguage = false, want true")
	}
}

// TestNewConfig_SkipUnknownLanguage tests SKIP_UNKNOWN_LANGUAGE configuration (STORY_06)
func TestNewConfig_SkipUnknownLanguage(t *testing.T) {
	tests := []struct {
		name     string
		envValue string
		want     bool
		wantErr  bool
	}{
		{
			name:     "default (false)",
			envValue: "",
			want:     false,
			wantErr:  false,
		},
		{
			name:     "explicit true",
			envValue: "true",
			want:     true,
			wantErr:  false,
		},
		{
			name:     "explicit false",
			envValue: "false",
			want:     false,
			wantErr:  false,
		},
		{
			name:     "1 means true",
			envValue: "1",
			want:     true,
			wantErr:  false,
		},
		{
			name:     "0 means false",
			envValue: "0",
			want:     false,
			wantErr:  false,
		},
		{
			name:     "invalid value",
			envValue: "maybe",
			want:     false,
			wantErr:  true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if tt.envValue != "" {
				os.Setenv("SKIP_UNKNOWN_LANGUAGE", tt.envValue)
				defer os.Unsetenv("SKIP_UNKNOWN_LANGUAGE")
			}

			config, err := NewConfig()
			if (err != nil) != tt.wantErr {
				t.Errorf("NewConfig() error = %v, wantErr %v", err, tt.wantErr)
				return
			}

			if !tt.wantErr && config.SkipUnknownLanguage != tt.want {
				t.Errorf("SkipUnknownLanguage = %v, want %v", config.SkipUnknownLanguage, tt.want)
			}
		})
	}
}

// TestNewConfig_SkipIfNoLanguageButSubtitlesExist tests SKIP_IF_NO_LANGUAGE_BUT_SUBTITLES_EXIST configuration (STORY_06)
func TestNewConfig_SkipIfNoLanguageButSubtitlesExist(t *testing.T) {
	tests := []struct {
		name     string
		envValue string
		want     bool
		wantErr  bool
	}{
		{
			name:     "default (false)",
			envValue: "",
			want:     false,
			wantErr:  false,
		},
		{
			name:     "explicit true",
			envValue: "true",
			want:     true,
			wantErr:  false,
		},
		{
			name:     "explicit false",
			envValue: "false",
			want:     false,
			wantErr:  false,
		},
		{
			name:     "1 means true",
			envValue: "1",
			want:     true,
			wantErr:  false,
		},
		{
			name:     "0 means false",
			envValue: "0",
			want:     false,
			wantErr:  false,
		},
		{
			name:     "invalid value",
			envValue: "yes",
			want:     false,
			wantErr:  true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if tt.envValue != "" {
				os.Setenv("SKIP_IF_NO_LANGUAGE_BUT_SUBTITLES_EXIST", tt.envValue)
				defer os.Unsetenv("SKIP_IF_NO_LANGUAGE_BUT_SUBTITLES_EXIST")
			}

			config, err := NewConfig()
			if (err != nil) != tt.wantErr {
				t.Errorf("NewConfig() error = %v, wantErr %v", err, tt.wantErr)
				return
			}

			if !tt.wantErr && config.SkipIfNoLanguageButSubtitlesExist != tt.want {
				t.Errorf("SkipIfNoLanguageButSubtitlesExist = %v, want %v", config.SkipIfNoLanguageButSubtitlesExist, tt.want)
			}
		})
	}
}

// TestConfig_AdvancedSkipConditionsIntegration tests integration of advanced skip conditions (STORY_06)
func TestConfig_AdvancedSkipConditionsIntegration(t *testing.T) {
	// Set both advanced skip env vars
	os.Setenv("SKIP_UNKNOWN_LANGUAGE", "true")
	os.Setenv("SKIP_IF_NO_LANGUAGE_BUT_SUBTITLES_EXIST", "true")
	defer os.Unsetenv("SKIP_UNKNOWN_LANGUAGE")
	defer os.Unsetenv("SKIP_IF_NO_LANGUAGE_BUT_SUBTITLES_EXIST")

	config, err := NewConfig()
	if err != nil {
		t.Fatalf("NewConfig() failed: %v", err)
	}

	// Verify both flags are enabled
	if !config.SkipUnknownLanguage {
		t.Error("SkipUnknownLanguage = false, want true")
	}

	if !config.SkipIfNoLanguageButSubtitlesExist {
		t.Error("SkipIfNoLanguageButSubtitlesExist = false, want true")
	}
}
