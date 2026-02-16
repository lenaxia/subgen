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
	assert.Contains(t, err.Error(), "WEBHOOK_PORT")
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

func TestLoad_InvalidLogLevel(t *testing.T) {
	// Setup
	os.Clearenv()
	os.Setenv("PLEX_TOKEN", "test-token")
	os.Setenv("LOG_LEVEL", "invalid")

	// Test
	config, err := Load()

	// Assert
	require.Error(t, err)
	assert.Nil(t, config)
	assert.Contains(t, err.Error(), "LOG_LEVEL")
}

func TestLoad_InvalidWorkerDiscovery(t *testing.T) {
	// Setup
	os.Clearenv()
	os.Setenv("PLEX_TOKEN", "test-token")
	os.Setenv("WORKER_DISCOVERY", "invalid")

	// Test
	config, err := Load()

	// Assert
	require.Error(t, err)
	assert.Nil(t, config)
	assert.Contains(t, err.Error(), "worker discovery")
}

func TestLoad_LocalhostDiscoveryWithoutAddress(t *testing.T) {
	// Setup
	os.Clearenv()
	os.Setenv("PLEX_TOKEN", "test-token")
	os.Setenv("WORKER_DISCOVERY", "kubernetes") // Use kubernetes mode so address not required
	os.Setenv("WORKER_ADDRESS", "")             // Empty address

	// Test
	config, err := Load()

	// Assert
	require.NoError(t, err) // Kubernetes mode doesn't require address
	assert.Equal(t, "kubernetes", config.Worker.Discovery)
}

func TestLoad_InvalidWorkerAddressFormat(t *testing.T) {
	// Setup
	os.Clearenv()
	os.Setenv("PLEX_TOKEN", "test-token")
	os.Setenv("WORKER_DISCOVERY", "localhost")
	os.Setenv("WORKER_ADDRESS", "localhost") // Missing port

	// Test
	config, err := Load()

	// Assert
	require.Error(t, err)
	assert.Nil(t, config)
	assert.Contains(t, err.Error(), "must include port")
}

func TestLoad_ProcessingFlags(t *testing.T) {
	// Setup
	os.Clearenv()
	os.Setenv("PLEX_TOKEN", "test-token")
	os.Setenv("PROCESS_ADDED_MEDIA", "false")
	os.Setenv("PROCESS_MEDIA_ON_PLAY", "false")

	// Test
	config, err := Load()

	// Assert
	require.NoError(t, err)
	assert.False(t, config.ProcessAddedMedia)
	assert.False(t, config.ProcessMediaOnPlay)
}

func TestLoad_SkipConfiguration(t *testing.T) {
	// Setup
	os.Clearenv()
	os.Setenv("PLEX_TOKEN", "test-token")
	os.Setenv("SKIP_IF_EXTERNAL_SUBTITLES_EXIST", "true")
	os.Setenv("SKIP_IF_TARGET_SUBTITLES_EXIST", "false")
	os.Setenv("SKIP_IF_INTERNAL_SUBTITLES_LANGUAGE", "eng")

	// Test
	config, err := Load()

	// Assert
	require.NoError(t, err)
	assert.True(t, config.Skip.IfExternalSubtitlesExist)
	assert.False(t, config.Skip.IfTargetSubtitlesExist)
	assert.Equal(t, "eng", config.Skip.IfInternalSubtitlesLang)
}

func TestLoad_WithArrayFields(t *testing.T) {
	// Setup
	os.Clearenv()
	os.Setenv("PLEX_TOKEN", "test-token")
	os.Setenv("SKIP_SUBTITLE_LANGUAGES", "eng,spa,fra")
	os.Setenv("SKIP_IF_AUDIO_LANGUAGES", "jpn,kor")

	// Test
	config, err := Load()

	// Assert
	require.NoError(t, err)
	assert.Equal(t, []string{"eng", "spa", "fra"}, config.Skip.SubtitleLanguages)
	assert.Equal(t, []string{"jpn", "kor"}, config.Skip.AudioLanguages)
}

func TestLoad_WithEmptyArrayFields(t *testing.T) {
	// Setup
	os.Clearenv()
	os.Setenv("PLEX_TOKEN", "test-token")
	os.Setenv("SKIP_SUBTITLE_LANGUAGES", "")
	os.Setenv("SKIP_IF_AUDIO_LANGUAGES", "   ")

	// Test
	config, err := Load()

	// Assert
	require.NoError(t, err)
	assert.Empty(t, config.Skip.SubtitleLanguages)
	assert.Empty(t, config.Skip.AudioLanguages)
}

func TestLoad_PlexQueueNextEpisode(t *testing.T) {
	// Setup
	os.Clearenv()
	os.Setenv("PLEX_TOKEN", "test-token")
	os.Setenv("PLEX_QUEUE_NEXT_EPISODE", "true")

	// Test
	config, err := Load()

	// Assert
	require.NoError(t, err)
	assert.True(t, config.Plex.QueueNextEpisode)
	assert.False(t, config.Plex.QueueSeason)
	assert.False(t, config.Plex.QueueSeries)
}

func TestLoad_PlexQueueSeason(t *testing.T) {
	// Setup
	os.Clearenv()
	os.Setenv("PLEX_TOKEN", "test-token")
	os.Setenv("PLEX_QUEUE_SEASON", "true")

	// Test
	config, err := Load()

	// Assert
	require.NoError(t, err)
	assert.False(t, config.Plex.QueueNextEpisode)
	assert.True(t, config.Plex.QueueSeason)
	assert.False(t, config.Plex.QueueSeries)
}

func TestLoad_PlexQueueSeries(t *testing.T) {
	// Setup
	os.Clearenv()
	os.Setenv("PLEX_TOKEN", "test-token")
	os.Setenv("PLEX_QUEUE_SERIES", "true")

	// Test
	config, err := Load()

	// Assert
	require.NoError(t, err)
	assert.False(t, config.Plex.QueueNextEpisode)
	assert.False(t, config.Plex.QueueSeason)
	assert.True(t, config.Plex.QueueSeries)
}

func TestLoad_PlexQueueMultipleModes(t *testing.T) {
	// Setup
	os.Clearenv()
	os.Setenv("PLEX_TOKEN", "test-token")
	os.Setenv("PLEX_QUEUE_NEXT_EPISODE", "true")
	os.Setenv("PLEX_QUEUE_SEASON", "true")

	// Test
	config, err := Load()

	// Assert
	assert.Error(t, err)
	assert.Nil(t, config)
	assert.Contains(t, err.Error(), "only one Plex queue mode")
}
