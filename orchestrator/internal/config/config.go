package config

import (
	"fmt"
	"os"
	"strings"
	"time"

	"github.com/sirupsen/logrus"
	"github.com/spf13/viper"
)

// Config holds all orchestrator configuration
type Config struct {
	// Server Configuration
	WebhookPort int
	MetricsPort int
	LogLevel    string

	// Media Server Configuration
	Plex     PlexConfig
	Jellyfin JellyfinConfig

	// Worker Configuration
	Worker WorkerConfig

	// Queue Configuration
	Queue QueueConfig

	// Transcription Options (passed to worker)
	Transcription TranscriptionConfig

	// Whisper Advanced Configuration
	Whisper WhisperConfig

	// Processing Control
	ProcessAddedMedia  bool
	ProcessMediaOnPlay bool

	// Skip Configuration
	Skip SkipConfig

	// Path Mapping Configuration
	PathMapping PathMappingConfig

	// Monitoring Configuration
	Monitor MonitorConfig

	// ASR Configuration
	ASR ASRConfig
}

type PlexConfig struct {
	Token   string
	Server  string
	Enabled bool

	// Episode queueing
	QueueNextEpisode bool
	QueueSeason      bool
	QueueSeries      bool
}

type JellyfinConfig struct {
	Token   string
	Server  string
	Enabled bool
}

type WorkerConfig struct {
	Discovery string
	Address   string
	Timeout   int // seconds

	// Kubernetes-specific fields
	Namespace   string
	ServiceName string
	Port        int32
}

type QueueConfig struct {
	MaxSize             int
	MaxAudioContentSize int64 // Maximum size in bytes for ASR audio uploads (default 100MB)
}

type TranscriptionConfig struct {
	WhisperModel         string
	WhisperThreads       int
	Device               string
	WordLevelHighlight   bool
	CustomRegroup        string
	LRCForAudioFiles     bool
	SubtitleLanguageName string
	AppendFooter         bool
	ModelCleanupDelay    int // seconds
}

type SkipConfig struct {
	IfExternalSubtitlesExist      bool
	IfTargetSubtitlesExist        bool
	IfInternalSubtitlesLang       string
	SubtitleLanguages             []string
	AudioLanguages                []string
	OnlySubgenSubtitles           bool
	PreferredAudioLanguages       []string
	LimitToPreferredAudioLanguage bool
}

type PathMappingConfig struct {
	Enabled bool   // USE_PATH_MAPPING
	From    string // PATH_MAPPING_FROM - comma-separated source paths
	To      string // PATH_MAPPING_TO - comma-separated destination paths
}

type MonitorConfig struct {
	Enabled           bool
	TranscribeFolders []string
	ScanOnStartup     bool
	StabilityChecks   int
	StabilityWait     int // seconds
	StabilityTimeout  int // seconds
	BatchScanLimit    int // Maximum files to scan in batch mode (0 = unlimited)
}

type ASRConfig struct {
	Timeout time.Duration // Timeout for ASR requests (default 30s)
}

// Load reads configuration from environment variables
func Load() (*Config, error) {
	v := viper.New()

	// Set defaults
	setDefaults(v)

	// Read from environment
	v.AutomaticEnv()

	// Allow .env file in development (optional)
	if _, err := os.Stat(".env"); err == nil {
		v.SetConfigFile(".env")
		v.SetConfigType("env")
		if err := v.ReadInConfig(); err != nil {
			return nil, fmt.Errorf("error reading config file: %w", err)
		}
	}

	// Build config struct
	config := &Config{
		WebhookPort: v.GetInt("WEBHOOK_PORT"),
		MetricsPort: v.GetInt("METRICS_PORT"),
		LogLevel:    v.GetString("LOG_LEVEL"),

		Plex: PlexConfig{
			Token:            v.GetString("PLEX_TOKEN"),
			Server:           v.GetString("PLEX_SERVER"),
			Enabled:          v.GetBool("PLEX_ENABLED"),
			QueueNextEpisode: v.GetBool("PLEX_QUEUE_NEXT_EPISODE"),
			QueueSeason:      v.GetBool("PLEX_QUEUE_SEASON"),
			QueueSeries:      v.GetBool("PLEX_QUEUE_SERIES"),
		},

		Jellyfin: JellyfinConfig{
			Token:   v.GetString("JELLYFIN_TOKEN"),
			Server:  v.GetString("JELLYFIN_SERVER"),
			Enabled: v.GetBool("JELLYFIN_ENABLED"),
		},

		Worker: WorkerConfig{
			Discovery:   v.GetString("WORKER_DISCOVERY"),
			Address:     v.GetString("WORKER_ADDRESS"),
			Timeout:     v.GetInt("WORKER_TIMEOUT"),
			Namespace:   v.GetString("WORKER_NAMESPACE"),
			ServiceName: v.GetString("WORKER_SERVICE_NAME"),
			Port:        int32(v.GetInt("WORKER_PORT")),
		},

		Queue: QueueConfig{
			MaxSize:             v.GetInt("QUEUE_MAX_SIZE"),
			MaxAudioContentSize: v.GetInt64("QUEUE_MAX_AUDIO_CONTENT_SIZE"),
		},

		Transcription: TranscriptionConfig{
			WhisperModel:         v.GetString("WHISPER_MODEL"),
			WhisperThreads:       v.GetInt("WHISPER_THREADS"),
			Device:               v.GetString("TRANSCRIBE_DEVICE"),
			WordLevelHighlight:   v.GetBool("WORD_LEVEL_HIGHLIGHT"),
			CustomRegroup:        v.GetString("CUSTOM_REGROUP"),
			LRCForAudioFiles:     v.GetBool("LRC_FOR_AUDIO_FILES"),
			SubtitleLanguageName: v.GetString("SUBTITLE_LANGUAGE_NAME"),
			AppendFooter:         v.GetBool("APPEND_FOOTER"),
			ModelCleanupDelay:    v.GetInt("MODEL_CLEANUP_DELAY"),
		},

		ProcessAddedMedia:  v.GetBool("PROCESS_ADDED_MEDIA"),
		ProcessMediaOnPlay: v.GetBool("PROCESS_MEDIA_ON_PLAY"),

		Skip: SkipConfig{
			IfExternalSubtitlesExist:      v.GetBool("SKIP_IF_EXTERNAL_SUBTITLES_EXIST"),
			IfTargetSubtitlesExist:        v.GetBool("SKIP_IF_TARGET_SUBTITLES_EXIST"),
			IfInternalSubtitlesLang:       v.GetString("SKIP_IF_INTERNAL_SUBTITLES_LANGUAGE"),
			SubtitleLanguages:             parseStringList(v.GetString("SKIP_SUBTITLE_LANGUAGES")),
			AudioLanguages:                parseStringList(v.GetString("SKIP_IF_AUDIO_LANGUAGES")),
			OnlySubgenSubtitles:           v.GetBool("SKIP_ONLY_SUBGEN_SUBTITLES"),
			PreferredAudioLanguages:       parseStringList(v.GetString("PREFERRED_AUDIO_LANGUAGES")),
			LimitToPreferredAudioLanguage: v.GetBool("LIMIT_TO_PREFERRED_AUDIO_LANGUAGE"),
		},

		PathMapping: PathMappingConfig{
			Enabled: v.GetBool("USE_PATH_MAPPING"),
			From:    v.GetString("PATH_MAPPING_FROM"),
			To:      v.GetString("PATH_MAPPING_TO"),
		},

		Monitor: MonitorConfig{
			Enabled:           v.GetBool("MONITOR"),
			TranscribeFolders: parseStringListPipe(v.GetString("TRANSCRIBE_FOLDERS")),
			ScanOnStartup:     v.GetBool("SCAN_ON_STARTUP"),
			StabilityChecks:   v.GetInt("FILE_STABILITY_CHECKS"),
			StabilityWait:     v.GetInt("FILE_STABILITY_WAIT"),
			StabilityTimeout:  v.GetInt("FILE_STABILITY_TIMEOUT"),
			BatchScanLimit:    v.GetInt("BATCH_SCAN_LIMIT"),
		},

		ASR: ASRConfig{
			Timeout: time.Duration(v.GetInt("ASR_TIMEOUT")) * time.Second,
		},
	}

	// Parse and validate WhisperConfig (advanced options)
	whisperConfig, err := loadWhisperConfig(v)
	if err != nil {
		return nil, fmt.Errorf("failed to load Whisper configuration: %w", err)
	}
	config.Whisper = *whisperConfig

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
	v.SetDefault("PLEX_QUEUE_NEXT_EPISODE", false)
	v.SetDefault("PLEX_QUEUE_SEASON", false)
	v.SetDefault("PLEX_QUEUE_SERIES", false)

	// Jellyfin
	v.SetDefault("JELLYFIN_ENABLED", false)
	v.SetDefault("JELLYFIN_SERVER", "http://localhost:8096")

	// Worker
	v.SetDefault("WORKER_DISCOVERY", "localhost")
	v.SetDefault("WORKER_ADDRESS", "localhost:50051")
	v.SetDefault("WORKER_TIMEOUT", 18000) // 5 hours
	v.SetDefault("WORKER_NAMESPACE", "media")
	v.SetDefault("WORKER_SERVICE_NAME", "subgen-worker")
	v.SetDefault("WORKER_PORT", 50051)

	// Queue
	v.SetDefault("QUEUE_MAX_SIZE", 1000)
	v.SetDefault("QUEUE_MAX_AUDIO_CONTENT_SIZE", 100*1024*1024) // 100MB default

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

	// Path Mapping
	v.SetDefault("USE_PATH_MAPPING", false)
	v.SetDefault("PATH_MAPPING_FROM", "")
	v.SetDefault("PATH_MAPPING_TO", "")

	// Monitoring
	v.SetDefault("MONITOR", false)
	v.SetDefault("TRANSCRIBE_FOLDERS", "")
	v.SetDefault("SCAN_ON_STARTUP", true)
	v.SetDefault("FILE_STABILITY_CHECKS", 3)
	v.SetDefault("FILE_STABILITY_WAIT", 2)
	v.SetDefault("FILE_STABILITY_TIMEOUT", 60)
	v.SetDefault("BATCH_SCAN_LIMIT", 1000) // Default limit of 1000 files (0 = unlimited)

	// ASR
	v.SetDefault("ASR_TIMEOUT", 300) // 300 seconds (5 minutes) for longer audio files

	// Whisper Advanced Options
	v.SetDefault("SUBGEN_KWARGS", "")       // Empty JSON by default
	v.SetDefault("USE_MODEL_PROMPT", false) // Disabled by default
	v.SetDefault("CUSTOM_MODEL_PROMPT", "") // No custom prompt by default
	v.SetDefault("COMPUTE_TYPE", "auto")    // Auto-detect compute type
}

// validate performs validation on the config struct
func validate(config *Config) error {
	// Validate port ranges
	if config.WebhookPort < 1 || config.WebhookPort > 65535 {
		return fmt.Errorf("WEBHOOK_PORT must be between 1 and 65535, got %d", config.WebhookPort)
	}
	if config.MetricsPort < 1 || config.MetricsPort > 65535 {
		return fmt.Errorf("METRICS_PORT must be between 1 and 65535, got %d", config.MetricsPort)
	}

	// Validate log level
	validLogLevels := []string{"debug", "info", "warn", "error"}
	if !contains(validLogLevels, config.LogLevel) {
		return fmt.Errorf("LOG_LEVEL must be one of [debug, info, warn, error], got '%s'", config.LogLevel)
	}

	// Required: At least one media server enabled
	if !config.Plex.Enabled && !config.Jellyfin.Enabled {
		return fmt.Errorf("at least one media server must be enabled (PLEX_ENABLED or JELLYFIN_ENABLED)")
	}

	// Required: Plex token if Plex enabled
	if config.Plex.Enabled && config.Plex.Token == "" {
		return fmt.Errorf("PLEX_TOKEN is required when PLEX_ENABLED=true")
	}

	// Validate Plex queue configuration
	if err := validatePlexQueueConfig(&config.Plex); err != nil {
		return err
	}

	// Required: Jellyfin token if Jellyfin enabled
	if config.Jellyfin.Enabled && config.Jellyfin.Token == "" {
		return fmt.Errorf("JELLYFIN_TOKEN is required when JELLYFIN_ENABLED=true")
	}

	// Validate worker discovery mode
	validDiscoveryModes := []string{"localhost", "kubernetes"}
	if !contains(validDiscoveryModes, config.Worker.Discovery) {
		return fmt.Errorf("WORKER_DISCOVERY must be one of [localhost, kubernetes], got '%s', check if invalid worker discovery mode", config.Worker.Discovery)
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

// validatePlexQueueConfig validates Plex queue configuration
func validatePlexQueueConfig(config *PlexConfig) error {
	// Count how many queue modes are enabled
	count := 0
	if config.QueueNextEpisode {
		count++
	}
	if config.QueueSeason {
		count++
	}
	if config.QueueSeries {
		count++
	}

	// Only one mode can be enabled at a time
	if count > 1 {
		return fmt.Errorf("only one Plex queue mode can be enabled at a time (PLEX_QUEUE_NEXT_EPISODE, PLEX_QUEUE_SEASON, PLEX_QUEUE_SERIES)")
	}

	return nil
}

// logConfig logs the configuration (with secrets redacted)
func logConfig(config *Config) {
	logrus.WithFields(logrus.Fields{
		"webhook_port":          config.WebhookPort,
		"metrics_port":          config.MetricsPort,
		"log_level":             config.LogLevel,
		"plex_enabled":          config.Plex.Enabled,
		"plex_server":           config.Plex.Server,
		"plex_token":            redact(config.Plex.Token),
		"jellyfin_enabled":      config.Jellyfin.Enabled,
		"jellyfin_server":       config.Jellyfin.Server,
		"jellyfin_token":        redact(config.Jellyfin.Token),
		"worker_discovery":      config.Worker.Discovery,
		"worker_address":        config.Worker.Address,
		"worker_timeout":        config.Worker.Timeout,
		"queue_max_size":        config.Queue.MaxSize,
		"whisper_model":         config.Transcription.WhisperModel,
		"whisper_threads":       config.Transcription.WhisperThreads,
		"process_added_media":   config.ProcessAddedMedia,
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

// parseStringList parses a comma-separated string into a slice
func parseStringList(s string) []string {
	if s == "" {
		return []string{}
	}
	parts := strings.Split(s, ",")
	result := make([]string, 0, len(parts))
	for _, part := range parts {
		trimmed := strings.TrimSpace(part)
		if trimmed != "" {
			result = append(result, trimmed)
		}
	}
	return result
}

// parseStringListPipe parses a pipe-separated string into a slice
func parseStringListPipe(s string) []string {
	if s == "" {
		return []string{}
	}
	parts := strings.Split(s, "|")
	result := make([]string, 0, len(parts))
	for _, part := range parts {
		trimmed := strings.TrimSpace(part)
		if trimmed != "" {
			result = append(result, trimmed)
		}
	}
	return result
}

// contains checks if a string slice contains a value
func contains(slice []string, value string) bool {
	for _, item := range slice {
		if item == value {
			return true
		}
	}
	return false
}
