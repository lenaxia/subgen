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
