# STORY_02: Configuration Management

**Status:** Not Started  
**Effort:** 4-6 hours  
**Epic:** EPIC_01 (Go Orchestrator Core)  
**Created:** 2026-02-15

---

## User Story

**As a** developer  
**I want** a centralized configuration system with validation and defaults  
**So that** the orchestrator can be configured via environment variables with type safety

---

## Acceptance Criteria

- [ ] All configuration loaded from environment variables
- [ ] Config struct with validation using struct tags
- [ ] Default values provided for optional settings
- [ ] Required fields validated at startup (fail fast)
- [ ] Config printed to logs (with secrets redacted)
- [ ] Support for `.env` file in development
- [ ] Config hot-reload NOT supported (restart required)
- [ ] 100% test coverage for config loading

---

## Integration Points

### Legacy Configuration (subgen.py:106-175)

**Location:** `/home/mikekao/personal/subgen/subgen.py:106-175`

**Current Implementation:**
```python
# Server Integration
plextoken = get_env_with_fallback('PLEX_TOKEN', 'PLEXTOKEN', 'token here')
plexserver = get_env_with_fallback('PLEX_SERVER', 'PLEXSERVER', 'http://192.168.1.111:32400')
jellyfintoken = get_env_with_fallback('JELLYFIN_TOKEN', 'JELLYFINTOKEN', 'token here')
jellyfinserver = get_env_with_fallback('JELLYFIN_SERVER', 'JELLYFINSERVER', 'http://192.168.1.111:8096')

# Whisper Configuration
whisper_model = os.getenv('WHISPER_MODEL', 'medium')
whisper_threads = int(os.getenv('WHISPER_THREADS', 4))
concurrent_transcriptions = int(os.getenv('CONCURRENT_TRANSCRIPTIONS', 2))
transcribe_device = os.getenv('TRANSCRIBE_DEVICE', 'cpu')

# Processing Control
procaddedmedia = get_env_with_fallback('PROCESS_ADDED_MEDIA', 'PROCADDEDMEDIA', True, convert_to_bool)
procmediaonplay = get_env_with_fallback('PROCESS_MEDIA_ON_PLAY', 'PROCMEDIAONPLAY', True, convert_to_bool)

# Subtitle Configuration
namesublang = get_env_with_fallback('SUBTITLE_LANGUAGE_NAME', 'NAMESUBLANG', '')

# System Configuration
webhookport = get_env_with_fallback('WEBHOOK_PORT', 'WEBHOOKPORT', 9000, int)
word_level_highlight = convert_to_bool(os.getenv('WORD_LEVEL_HIGHLIGHT', False))
debug = convert_to_bool(os.getenv('DEBUG', True))
model_cleanup_delay = int(os.getenv('MODEL_CLEANUP_DELAY', 30))

# Skip Configuration
skipifexternalsub = get_env_with_fallback('SKIP_IF_EXTERNAL_SUBTITLES_EXIST', 'SKIPIFEXTERNALSUB', False, convert_to_bool)
```

**Go Implementation Needs:**
- Preserve all environment variable names (for backward compatibility)
- Type-safe parsing (string → int, bool)
- Validation (required vs optional)
- Struct tags for viper binding

---

## Technical Design

### Config Struct

**File:** `internal/config/config.go`

```go
package config

import (
	"fmt"
	"os"
	"strings"

	"github.com/sirupsen/logrus"
	"github.com/spf13/viper"
)

// Config holds all orchestrator configuration
type Config struct {
	// Server Configuration
	WebhookPort int    `mapstructure:"WEBHOOK_PORT" validate:"required,min=1,max=65535"`
	MetricsPort int    `mapstructure:"METRICS_PORT" validate:"required,min=1,max=65535"`
	LogLevel    string `mapstructure:"LOG_LEVEL" validate:"required,oneof=debug info warn error"`

	// Media Server Configuration
	Plex     PlexConfig     `mapstructure:",squash"`
	Jellyfin JellyfinConfig `mapstructure:",squash"`

	// Worker Configuration
	Worker WorkerConfig `mapstructure:",squash"`

	// Queue Configuration
	Queue QueueConfig `mapstructure:",squash"`

	// Transcription Options (passed to worker)
	Transcription TranscriptionConfig `mapstructure:",squash"`

	// Processing Control
	ProcessAddedMedia bool `mapstructure:"PROCESS_ADDED_MEDIA"`
	ProcessMediaOnPlay bool `mapstructure:"PROCESS_MEDIA_ON_PLAY"`

	// Skip Configuration
	Skip SkipConfig `mapstructure:",squash"`
}

type PlexConfig struct {
	Token  string `mapstructure:"PLEX_TOKEN" validate:"required_if=Enabled true"`
	Server string `mapstructure:"PLEX_SERVER" validate:"required_if=Enabled true,url"`
	Enabled bool  `mapstructure:"PLEX_ENABLED"`
}

type JellyfinConfig struct {
	Token   string `mapstructure:"JELLYFIN_TOKEN" validate:"required_if=Enabled true"`
	Server  string `mapstructure:"JELLYFIN_SERVER" validate:"required_if=Enabled true,url"`
	Enabled bool   `mapstructure:"JELLYFIN_ENABLED"`
}

type WorkerConfig struct {
	Discovery string `mapstructure:"WORKER_DISCOVERY" validate:"required,oneof=localhost kubernetes"`
	Address   string `mapstructure:"WORKER_ADDRESS" validate:"required_if=Discovery localhost"`
	Timeout   int    `mapstructure:"WORKER_TIMEOUT" validate:"min=60,max=18000"` // seconds
}

type QueueConfig struct {
	MaxSize int `mapstructure:"QUEUE_MAX_SIZE" validate:"required,min=10,max=10000"`
}

type TranscriptionConfig struct {
	WhisperModel         string `mapstructure:"WHISPER_MODEL" validate:"required,oneof=tiny base small medium large large-v3"`
	WhisperThreads       int    `mapstructure:"WHISPER_THREADS" validate:"min=1,max=32"`
	Device               string `mapstructure:"TRANSCRIBE_DEVICE" validate:"oneof=cpu cuda"`
	WordLevelHighlight   bool   `mapstructure:"WORD_LEVEL_HIGHLIGHT"`
	CustomRegroup        string `mapstructure:"CUSTOM_REGROUP"`
	LRCForAudioFiles     bool   `mapstructure:"LRC_FOR_AUDIO_FILES"`
	SubtitleLanguageName string `mapstructure:"SUBTITLE_LANGUAGE_NAME"`
	AppendFooter         bool   `mapstructure:"APPEND_FOOTER"`
	ModelCleanupDelay    int    `mapstructure:"MODEL_CLEANUP_DELAY" validate:"min=0,max=300"` // seconds
}

type SkipConfig struct {
	IfExternalSubtitlesExist bool     `mapstructure:"SKIP_IF_EXTERNAL_SUBTITLES_EXIST"`
	IfTargetSubtitlesExist   bool     `mapstructure:"SKIP_IF_TARGET_SUBTITLES_EXIST"`
	IfInternalSubtitlesLang  string   `mapstructure:"SKIP_IF_INTERNAL_SUBTITLES_LANGUAGE"`
	SubtitleLanguages        []string `mapstructure:"SKIP_SUBTITLE_LANGUAGES"`
	AudioLanguages           []string `mapstructure:"SKIP_IF_AUDIO_LANGUAGES"`
	OnlySubgenSubtitles      bool     `mapstructure:"SKIP_ONLY_SUBGEN_SUBTITLES"`
}

// Load reads configuration from environment variables
func Load() (*Config, error) {
	v := viper.New()

	// Set defaults
	setDefaults(v)

	// Read from environment
	v.AutomaticEnv()

	// Allow .env file in development
	v.SetConfigFile(".env")
	v.SetConfigType("env")
	if err := v.ReadInConfig(); err != nil {
		// .env file not found is OK (production uses env vars)
		if _, ok := err.(viper.ConfigFileNotFoundError); !ok {
			return nil, fmt.Errorf("error reading config file: %w", err)
		}
	}

	// Unmarshal into struct
	config := &Config{}
	if err := v.Unmarshal(config); err != nil {
		return nil, fmt.Errorf("error unmarshaling config: %w", err)
	}

	// Validate
	if err := validate(config); err != nil {
		return nil, fmt.Errorf("config validation failed: %w", err)
	}

	// Log config (redact secrets)
	logConfig(config)

	return config, nil
}

// setDefaults sets default values for all configuration
func setDefaults(v *viper.Viper) {
	// Server
	v.SetDefault("WEBHOOK_PORT", 9000)
	v.SetDefault("METRICS_PORT", 9090)
	v.SetDefault("LOG_LEVEL", "info")

	// Plex
	v.SetDefault("PLEX_ENABLED", true)
	v.SetDefault("PLEX_SERVER", "http://localhost:32400")

	// Jellyfin
	v.SetDefault("JELLYFIN_ENABLED", false)
	v.SetDefault("JELLYFIN_SERVER", "http://localhost:8096")

	// Worker
	v.SetDefault("WORKER_DISCOVERY", "localhost")
	v.SetDefault("WORKER_ADDRESS", "localhost:50051")
	v.SetDefault("WORKER_TIMEOUT", 18000) // 5 hours

	// Queue
	v.SetDefault("QUEUE_MAX_SIZE", 1000)

	// Transcription
	v.SetDefault("WHISPER_MODEL", "medium")
	v.SetDefault("WHISPER_THREADS", 4)
	v.SetDefault("TRANSCRIBE_DEVICE", "cpu")
	v.SetDefault("WORD_LEVEL_HIGHLIGHT", false)
	v.SetDefault("CUSTOM_REGROUP", "cm_sl=84_sl=42++++++1")
	v.SetDefault("LRC_FOR_AUDIO_FILES", true)
	v.SetDefault("SUBTITLE_LANGUAGE_NAME", "aa")
	v.SetDefault("APPEND_FOOTER", false)
	v.SetDefault("MODEL_CLEANUP_DELAY", 30)

	// Processing
	v.SetDefault("PROCESS_ADDED_MEDIA", true)
	v.SetDefault("PROCESS_MEDIA_ON_PLAY", true)

	// Skip
	v.SetDefault("SKIP_IF_EXTERNAL_SUBTITLES_EXIST", false)
	v.SetDefault("SKIP_IF_TARGET_SUBTITLES_EXIST", true)
	v.SetDefault("SKIP_IF_INTERNAL_SUBTITLES_LANGUAGE", "")
	v.SetDefault("SKIP_SUBTITLE_LANGUAGES", "")
	v.SetDefault("SKIP_IF_AUDIO_LANGUAGES", "")
	v.SetDefault("SKIP_ONLY_SUBGEN_SUBTITLES", false)
}

// validate performs validation on the config struct
func validate(config *Config) error {
	// Required: At least one media server enabled
	if !config.Plex.Enabled && !config.Jellyfin.Enabled {
		return fmt.Errorf("at least one media server must be enabled (PLEX_ENABLED or JELLYFIN_ENABLED)")
	}

	// Required: Plex token if Plex enabled
	if config.Plex.Enabled && config.Plex.Token == "" {
		return fmt.Errorf("PLEX_TOKEN is required when PLEX_ENABLED=true")
	}

	// Required: Jellyfin token if Jellyfin enabled
	if config.Jellyfin.Enabled && config.Jellyfin.Token == "" {
		return fmt.Errorf("JELLYFIN_TOKEN is required when JELLYFIN_ENABLED=true")
	}

	// Validate worker address format for localhost mode
	if config.Worker.Discovery == "localhost" {
		if config.Worker.Address == "" {
			return fmt.Errorf("WORKER_ADDRESS is required when WORKER_DISCOVERY=localhost")
		}
		if !strings.Contains(config.Worker.Address, ":") {
			return fmt.Errorf("WORKER_ADDRESS must include port (e.g., localhost:50051)")
		}
	}

	return nil
}

// logConfig logs the configuration (with secrets redacted)
func logConfig(config *Config) {
	logrus.WithFields(logrus.Fields{
		"webhook_port":         config.WebhookPort,
		"metrics_port":         config.MetricsPort,
		"log_level":            config.LogLevel,
		"plex_enabled":         config.Plex.Enabled,
		"plex_server":          config.Plex.Server,
		"plex_token":           redact(config.Plex.Token),
		"jellyfin_enabled":     config.Jellyfin.Enabled,
		"jellyfin_server":      config.Jellyfin.Server,
		"jellyfin_token":       redact(config.Jellyfin.Token),
		"worker_discovery":     config.Worker.Discovery,
		"worker_address":       config.Worker.Address,
		"worker_timeout":       config.Worker.Timeout,
		"queue_max_size":       config.Queue.MaxSize,
		"whisper_model":        config.Transcription.WhisperModel,
		"whisper_threads":      config.Transcription.WhisperThreads,
		"process_added_media":  config.ProcessAddedMedia,
		"process_media_on_play": config.ProcessMediaOnPlay,
	}).Info("Configuration loaded")
}

// redact replaces all but first 4 characters with asterisks
func redact(s string) string {
	if len(s) <= 4 {
		return "****"
	}
	return s[:4] + strings.Repeat("*", len(s)-4)
}
```

---

## Test Cases

### Unit Tests

**File:** `internal/config/config_test.go`

```go
package config

import (
	"os"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestLoad_WithDefaults(t *testing.T) {
	// Setup
	os.Clearenv()
	os.Setenv("PLEX_TOKEN", "test-token-12345")

	// Test
	config, err := Load()

	// Assert
	require.NoError(t, err)
	assert.Equal(t, 9000, config.WebhookPort)
	assert.Equal(t, 9090, config.MetricsPort)
	assert.Equal(t, "info", config.LogLevel)
	assert.Equal(t, "test-token-12345", config.Plex.Token)
	assert.Equal(t, "http://localhost:32400", config.Plex.Server)
	assert.True(t, config.Plex.Enabled)
	assert.Equal(t, "localhost", config.Worker.Discovery)
	assert.Equal(t, "localhost:50051", config.Worker.Address)
	assert.Equal(t, 1000, config.Queue.MaxSize)
	assert.Equal(t, "medium", config.Transcription.WhisperModel)
	assert.Equal(t, 4, config.Transcription.WhisperThreads)
}

func TestLoad_WithCustomValues(t *testing.T) {
	// Setup
	os.Clearenv()
	os.Setenv("PLEX_TOKEN", "custom-token")
	os.Setenv("WEBHOOK_PORT", "8000")
	os.Setenv("METRICS_PORT", "8090")
	os.Setenv("LOG_LEVEL", "debug")
	os.Setenv("WHISPER_MODEL", "large")
	os.Setenv("QUEUE_MAX_SIZE", "500")

	// Test
	config, err := Load()

	// Assert
	require.NoError(t, err)
	assert.Equal(t, 8000, config.WebhookPort)
	assert.Equal(t, 8090, config.MetricsPort)
	assert.Equal(t, "debug", config.LogLevel)
	assert.Equal(t, "large", config.Transcription.WhisperModel)
	assert.Equal(t, 500, config.Queue.MaxSize)
}

func TestLoad_MissingPlexToken(t *testing.T) {
	// Setup
	os.Clearenv()
	os.Setenv("PLEX_ENABLED", "true")
	// No PLEX_TOKEN set

	// Test
	config, err := Load()

	// Assert
	require.Error(t, err)
	assert.Nil(t, config)
	assert.Contains(t, err.Error(), "PLEX_TOKEN is required")
}

func TestLoad_InvalidWebhookPort(t *testing.T) {
	// Setup
	os.Clearenv()
	os.Setenv("PLEX_TOKEN", "test-token")
	os.Setenv("WEBHOOK_PORT", "99999") // Invalid port

	// Test
	config, err := Load()

	// Assert
	require.Error(t, err)
	assert.Nil(t, config)
}

func TestLoad_BothMediaServersDisabled(t *testing.T) {
	// Setup
	os.Clearenv()
	os.Setenv("PLEX_ENABLED", "false")
	os.Setenv("JELLYFIN_ENABLED", "false")

	// Test
	config, err := Load()

	// Assert
	require.Error(t, err)
	assert.Nil(t, config)
	assert.Contains(t, err.Error(), "at least one media server must be enabled")
}

func TestLoad_JellyfinEnabled(t *testing.T) {
	// Setup
	os.Clearenv()
	os.Setenv("PLEX_ENABLED", "false")
	os.Setenv("JELLYFIN_ENABLED", "true")
	os.Setenv("JELLYFIN_TOKEN", "jellyfin-token-12345")
	os.Setenv("JELLYFIN_SERVER", "http://192.168.1.100:8096")

	// Test
	config, err := Load()

	// Assert
	require.NoError(t, err)
	assert.False(t, config.Plex.Enabled)
	assert.True(t, config.Jellyfin.Enabled)
	assert.Equal(t, "jellyfin-token-12345", config.Jellyfin.Token)
	assert.Equal(t, "http://192.168.1.100:8096", config.Jellyfin.Server)
}

func TestLoad_KubernetesDiscovery(t *testing.T) {
	// Setup
	os.Clearenv()
	os.Setenv("PLEX_TOKEN", "test-token")
	os.Setenv("WORKER_DISCOVERY", "kubernetes")

	// Test
	config, err := Load()

	// Assert
	require.NoError(t, err)
	assert.Equal(t, "kubernetes", config.Worker.Discovery)
	// WORKER_ADDRESS not required for kubernetes mode
}

func TestRedact(t *testing.T) {
	tests := []struct {
		input    string
		expected string
	}{
		{"", "****"},
		{"abc", "****"},
		{"1234", "****"},
		{"12345", "1234*"},
		{"token-12345-abcdef", "toke**************"},
	}

	for _, tt := range tests {
		t.Run(tt.input, func(t *testing.T) {
			result := redact(tt.input)
			assert.Equal(t, tt.expected, result)
		})
	}
}
```

---

## Implementation Steps

### Step 1: Create Config Package
```bash
mkdir -p internal/config
touch internal/config/config.go
touch internal/config/config_test.go
```

### Step 2: Install Dependencies
```bash
go get github.com/spf13/viper@v1.18.2
go get github.com/sirupsen/logrus@v1.9.3
```

### Step 3: Implement Config Struct
- Copy struct definitions from design above
- Add all fields matching legacy environment variables

### Step 4: Implement Load Function
- Use viper to read environment variables
- Set defaults
- Validate required fields
- Return error for missing required config

### Step 5: Implement Validation
- Check at least one media server enabled
- Check required tokens present
- Validate port ranges
- Validate enum values

### Step 6: Write Tests
- Test with defaults
- Test with custom values
- Test validation errors
- Test redaction
- Aim for 100% coverage

### Step 7: Integrate with Main
```go
// cmd/orchestrator/main.go
package main

import (
	"github.com/your-org/subgen/orchestrator/internal/config"
	"github.com/sirupsen/logrus"
)

func main() {
	// Load config
	cfg, err := config.Load()
	if err != nil {
		logrus.Fatalf("Failed to load configuration: %v", err)
	}

	// Set log level
	level, _ := logrus.ParseLevel(cfg.LogLevel)
	logrus.SetLevel(level)

	logrus.Info("Orchestrator starting...")
	// ... rest of initialization
}
```

---

## Dependencies

**Requires:**
- STORY_01 (Project Setup)

**Blocks:**
- STORY_03 (Webhook Handlers) - needs config
- STORY_04 (Queue Management) - needs config
- All other stories

---

## Notes

- Configuration is immutable after load (no hot-reload)
- Restart required for config changes
- All secrets redacted in logs
- Validation failures cause immediate exit (fail fast)
- Support for `.env` file in development only

---

**Owner:** TBD  
**Created:** 2026-02-15  
**Last Updated:** 2026-02-15
