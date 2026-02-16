package plex

import (
	"context"
	"fmt"

	"github.com/sirupsen/logrus"
)

// QueueMode defines how episodes should be queued
type QueueMode string

const (
	// QueueModeNext queues only the next episode
	QueueModeNext QueueMode = "next"
	// QueueModeSeason queues all remaining episodes in current season
	QueueModeSeason QueueMode = "season"
	// QueueModeSeries queues all remaining episodes in entire series
	QueueModeSeries QueueMode = "series"
)

// EpisodeQueuer manages episode queueing logic
type EpisodeQueuer struct {
	client *Client
	logger *logrus.Logger
}

// NewEpisodeQueuer creates a new episode queuer
func NewEpisodeQueuer(client *Client, logger *logrus.Logger) *EpisodeQueuer {
	return &EpisodeQueuer{
		client: client,
		logger: logger,
	}
}

// QueueEpisodes queues episodes based on mode
func (eq *EpisodeQueuer) QueueEpisodes(ctx context.Context, itemID string, mode QueueMode) ([]string, error) {
	// Get current episode metadata
	currentEp, err := eq.client.GetMetadata(ctx, itemID)
	if err != nil {
		return nil, fmt.Errorf("get current episode metadata: %w", err)
	}

	switch mode {
	case QueueModeNext:
		return eq.queueNextEpisode(ctx, currentEp)
	case QueueModeSeason:
		return eq.queueSeasonEpisodes(ctx, currentEp)
	case QueueModeSeries:
		return eq.queueSeriesEpisodes(ctx, currentEp)
	default:
		return nil, fmt.Errorf("invalid queue mode: %s", mode)
	}
}

// queueNextEpisode queues only the next episode
func (eq *EpisodeQueuer) queueNextEpisode(ctx context.Context, currentEp *Video) ([]string, error) {
	// Get all episodes in season
	episodes, _, err := eq.client.GetChildren(ctx, currentEp.ParentRatingKey)
	if err != nil {
		return nil, fmt.Errorf("get season episodes: %w", err)
	}

	// Find next episode in same season
	currentIndex := currentEp.Index
	for _, ep := range episodes {
		if ep.Index == currentIndex+1 {
			eq.logger.WithFields(logrus.Fields{
				"series":  currentEp.GrandparentTitle,
				"season":  currentEp.ParentIndex,
				"episode": ep.Index,
				"title":   ep.Title,
			}).Info("Queueing next episode")
			return []string{ep.RatingKey}, nil
		}
	}

	// No next episode in season, try next season
	_, seasons, err := eq.client.GetChildren(ctx, currentEp.GrandparentRatingKey)
	if err != nil {
		return nil, fmt.Errorf("get series seasons: %w", err)
	}

	for _, dir := range seasons {
		if dir.Index == currentEp.ParentIndex+1 {
			// Found next season, get first episode
			nextSeasonEps, _, err := eq.client.GetChildren(ctx, dir.RatingKey)
			if err != nil {
				return nil, fmt.Errorf("get next season episodes: %w", err)
			}
			if len(nextSeasonEps) > 0 {
				firstEp := nextSeasonEps[0]
				eq.logger.WithFields(logrus.Fields{
					"series":  currentEp.GrandparentTitle,
					"season":  dir.Index,
					"episode": firstEp.Index,
					"title":   firstEp.Title,
				}).Info("Queueing first episode of next season")
				return []string{firstEp.RatingKey}, nil
			}
		}
	}

	// No next episode found
	eq.logger.WithFields(logrus.Fields{
		"series":  currentEp.GrandparentTitle,
		"season":  currentEp.ParentIndex,
		"episode": currentEp.Index,
	}).Info("No next episode found (end of series)")
	return []string{}, nil
}

// queueSeasonEpisodes queues all remaining episodes in current season
func (eq *EpisodeQueuer) queueSeasonEpisodes(ctx context.Context, currentEp *Video) ([]string, error) {
	episodes, _, err := eq.client.GetChildren(ctx, currentEp.ParentRatingKey)
	if err != nil {
		return nil, fmt.Errorf("get season episodes: %w", err)
	}

	var itemIDs []string
	currentIndex := currentEp.Index

	for _, ep := range episodes {
		// Queue all episodes from current onwards
		if ep.Index >= currentIndex {
			itemIDs = append(itemIDs, ep.RatingKey)
		}
	}

	eq.logger.WithFields(logrus.Fields{
		"series": currentEp.GrandparentTitle,
		"season": currentEp.ParentIndex,
		"count":  len(itemIDs),
	}).Info("Queueing season episodes")

	return itemIDs, nil
}

// queueSeriesEpisodes queues all episodes in the entire series
func (eq *EpisodeQueuer) queueSeriesEpisodes(ctx context.Context, currentEp *Video) ([]string, error) {
	// Get all seasons
	_, seasons, err := eq.client.GetChildren(ctx, currentEp.GrandparentRatingKey)
	if err != nil {
		return nil, fmt.Errorf("get series seasons: %w", err)
	}

	var allItemIDs []string
	currentSeasonIndex := currentEp.ParentIndex
	currentEpisodeIndex := currentEp.Index

	for _, season := range seasons {
		// Skip special seasons (season 0)
		if season.Index == 0 {
			continue
		}

		// Get episodes for this season
		episodes, _, err := eq.client.GetChildren(ctx, season.RatingKey)
		if err != nil {
			eq.logger.WithError(err).WithField("season", season.Index).Warn("Failed to get season episodes")
			continue
		}

		for _, ep := range episodes {
			// Skip episodes before current
			if season.Index < currentSeasonIndex {
				continue
			}
			if season.Index == currentSeasonIndex && ep.Index < currentEpisodeIndex {
				continue
			}

			allItemIDs = append(allItemIDs, ep.RatingKey)
		}
	}

	eq.logger.WithFields(logrus.Fields{
		"series": currentEp.GrandparentTitle,
		"count":  len(allItemIDs),
	}).Info("Queueing series episodes")

	return allItemIDs, nil
}

// GetFilePath extracts file path from episode metadata
func (eq *EpisodeQueuer) GetFilePath(ctx context.Context, itemID string) (string, error) {
	video, err := eq.client.GetMetadata(ctx, itemID)
	if err != nil {
		return "", fmt.Errorf("get metadata: %w", err)
	}

	if len(video.Media) == 0 || len(video.Media[0].Part) == 0 {
		return "", fmt.Errorf("no media parts found for item %s", itemID)
	}

	return video.Media[0].Part[0].File, nil
}
