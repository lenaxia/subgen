// Package plex provides integration with Plex Media Server XML API.
//
// This package includes:
// - HTTP client for Plex XML API communication
// - Episode queueing logic for auto-queueing next/season/series episodes
// - XML parsing for Plex metadata structures
//
// Usage:
//
//	client := plex.NewClient("http://localhost:32400", "plex-token")
//	queuer := plex.NewEpisodeQueuer(client, logger)
//	itemIDs, err := queuer.QueueEpisodes(ctx, "12345", plex.QueueModeNext)
package plex
