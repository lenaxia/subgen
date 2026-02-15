// Package mediaserver provides HTTP clients for media server APIs (Plex, Jellyfin).
//
// This package implements the MediaServerClient interface for communicating with
// Plex and Jellyfin servers to:
//   - Retrieve file system paths for media items
//   - Trigger metadata refresh (causes server to rescan for new subtitles)
//
// Both clients implement connection pooling via http.Client for efficient
// resource usage when making multiple API calls.
package mediaserver

import (
	"context"
	"time"
)

// MediaServerClient interface for media server operations
type MediaServerClient interface {
	// GetFilePath retrieves the file system path for a media item
	GetFilePath(ctx context.Context, itemID string) (string, error)

	// RefreshMetadata triggers a metadata refresh for an item
	// (causes server to rescan for new subtitles)
	RefreshMetadata(ctx context.Context, itemID string) error
}

// ClientConfig holds common HTTP client configuration
type ClientConfig struct {
	Timeout         time.Duration // HTTP request timeout
	MaxIdleConns    int           // Connection pool size
	IdleConnTimeout time.Duration // How long idle connections stay open
}

// DefaultClientConfig returns sensible defaults
func DefaultClientConfig() ClientConfig {
	return ClientConfig{
		Timeout:         30 * time.Second,
		MaxIdleConns:    10,
		IdleConnTimeout: 90 * time.Second,
	}
}
