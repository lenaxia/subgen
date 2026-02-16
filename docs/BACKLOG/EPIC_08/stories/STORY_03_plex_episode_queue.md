# Story 03: Plex Episode Queueing

**Epic**: EPIC_08 - Advanced Features & Polish  
**Status**: In Progress  
**Assignee**: Delegation Agent  
**Effort**: 8-10 hours  
**Priority**: MEDIUM

---

## User Story

As a Plex user watching a TV series,  
I want Subgen to automatically queue additional episodes for transcription,  
So that I have subtitles ready when I watch subsequent episodes without triggering each individually.

---

## Background

In the original subgen.py (lines 582-623, 1790-1889), three queueing modes were supported:
1. **PLEX_QUEUE_NEXT_EPISODE** - Queue the next episode in the series
2. **PLEX_QUEUE_SEASON** - Queue all remaining episodes in the current season
3. **PLEX_QUEUE_SERIES** - Queue all episodes in the entire TV series

This feature significantly improves user experience by proactively generating subtitles for upcoming episodes, especially useful for binge-watching scenarios.

---

## Acceptance Criteria

### Configuration
- [ ] **PLEX_QUEUE_NEXT_EPISODE** (bool, default: false) - Queue next episode
- [ ] **PLEX_QUEUE_SEASON** (bool, default: false) - Queue entire season
- [ ] **PLEX_QUEUE_SERIES** (bool, default: false) - Queue entire series
- [ ] Config validation: Only one mode can be enabled at a time
- [ ] Config loading via viper with defaults

### Plex API Integration
- [ ] **PlexClient struct** - HTTP client for Plex XML API calls
- [ ] **GetMetadata(itemID)** - Retrieve episode/season/series metadata
- [ ] **GetNextEpisode(itemID)** - Find next episode in series
- [ ] **GetSeasonEpisodes(seasonKey)** - Get all episodes in season
- [ ] **GetSeriesEpisodes(seriesKey)** - Get all episodes in series
- [ ] **GetFilePath(itemID)** - Extract file path from metadata
- [ ] XML parsing with proper error handling
- [ ] Authentication via X-Plex-Token header

### Episode Navigation
- [ ] Detect season boundaries (don't queue across seasons for season mode)
- [ ] Handle series end gracefully (no error when no next episode)
- [ ] Skip episodes that fail skip logic (integrate with skip checker)
- [ ] Respect episode index ordering
- [ ] Navigate parent/child relationships (GrandparentKey → ParentKey → Item)

### Integration
- [ ] Called from Plex webhook handler after processing episode
- [ ] Queue items using existing queue.Queue
- [ ] Pass Plex itemID for metadata refresh after transcription
- [ ] Logging: "Queued 10 episodes from Season 1 of Show Name"
- [ ] Error handling with fallback (log error, continue processing)

### Quality Standards
- [ ] Type-safe Go implementation
- [ ] Comprehensive error handling
- [ ] Unit tests with mocked HTTP responses (>80% coverage)
- [ ] Integration test with mock Plex server
- [ ] No external network calls in tests

---

## Technical Design

### Component Structure

```
orchestrator/internal/plex/
├── client.go              # Plex HTTP client
├── client_test.go         # Unit tests with mocked HTTP
├── episode_queue.go       # Episode queueing logic
├── episode_queue_test.go  # Unit tests
├── models.go              # Plex XML response structs
└── doc.go                 # Package documentation
```

### Data Models

```go
// Plex XML response structures
type MediaContainer struct {
	XMLName xml.Name `xml:"MediaContainer"`
	Size    int      `xml:"size,attr"`
	Video   []Video  `xml:"Video"`
	Directory []Directory `xml:"Directory"`
}

type Video struct {
	RatingKey           string `xml:"ratingKey,attr"`
	ParentRatingKey     string `xml:"parentRatingKey,attr"`
	GrandparentRatingKey string `xml:"grandparentRatingKey,attr"`
	Type                string `xml:"type,attr"`
	Title               string `xml:"title,attr"`
	Index               int    `xml:"index,attr"`          // Episode number
	ParentIndex         int    `xml:"parentIndex,attr"`    // Season number
	GrandparentTitle    string `xml:"grandparentTitle,attr"` // Series name
	Media               []Media `xml:"Media"`
}

type Media struct {
	Part []Part `xml:"Part"`
}

type Part struct {
	File string `xml:"file,attr"`
}

type Directory struct {
	RatingKey string `xml:"ratingKey,attr"`
	Type      string `xml:"type,attr"`
	Index     int    `xml:"index,attr"`
	Title     string `xml:"title,attr"`
}
```

### PlexClient Implementation

```go
package plex

import (
	"context"
	"encoding/xml"
	"fmt"
	"io"
	"net/http"
	"time"
)

type Client struct {
	baseURL    string
	token      string
	httpClient *http.Client
}

func NewClient(baseURL, token string) *Client {
	return &Client{
		baseURL: baseURL,
		token:   token,
		httpClient: &http.Client{
			Timeout: 30 * time.Second,
		},
	}
}

// GetMetadata retrieves metadata for a Plex item
func (c *Client) GetMetadata(ctx context.Context, itemID string) (*Video, error) {
	url := fmt.Sprintf("%s/library/metadata/%s", c.baseURL, itemID)
	
	req, err := http.NewRequestWithContext(ctx, "GET", url, nil)
	if err != nil {
		return nil, fmt.Errorf("create request: %w", err)
	}
	req.Header.Set("X-Plex-Token", c.token)
	
	resp, err := c.httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("http request: %w", err)
	}
	defer resp.Body.Close()
	
	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return nil, fmt.Errorf("plex api error: status=%d body=%s", resp.StatusCode, string(body))
	}
	
	var container MediaContainer
	if err := xml.NewDecoder(resp.Body).Decode(&container); err != nil {
		return nil, fmt.Errorf("decode xml: %w", err)
	}
	
	if len(container.Video) == 0 {
		return nil, fmt.Errorf("no video found in response")
	}
	
	return &container.Video[0], nil
}

// GetChildren retrieves child items (episodes of season, seasons of series)
func (c *Client) GetChildren(ctx context.Context, parentKey string) ([]Video, []Directory, error) {
	url := fmt.Sprintf("%s/library/metadata/%s/children", c.baseURL, parentKey)
	
	req, err := http.NewRequestWithContext(ctx, "GET", url, nil)
	if err != nil {
		return nil, nil, fmt.Errorf("create request: %w", err)
	}
	req.Header.Set("X-Plex-Token", c.token)
	
	resp, err := c.httpClient.Do(req)
	if err != nil {
		return nil, nil, fmt.Errorf("http request: %w", err)
	}
	defer resp.Body.Close()
	
	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return nil, nil, fmt.Errorf("plex api error: status=%d body=%s", resp.StatusCode, string(body))
	}
	
	var container MediaContainer
	if err := xml.NewDecoder(resp.Body).Decode(&container); err != nil {
		return nil, nil, fmt.Errorf("decode xml: %w", err)
	}
	
	return container.Video, container.Directory, nil
}
```

### Episode Queue Implementation

```go
package plex

import (
	"context"
	"fmt"
	
	"github.com/sirupsen/logrus"
)

type QueueMode string

const (
	QueueModeNext   QueueMode = "next"
	QueueModeSeason QueueMode = "season"
	QueueModeSeries QueueMode = "series"
)

type EpisodeQueuer struct {
	client *Client
	logger *logrus.Logger
}

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
	seasons, _, err := eq.client.GetChildren(ctx, currentEp.GrandparentRatingKey)
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
		// Queue all episodes after current (including current)
		if ep.Index >= currentIndex {
			itemIDs = append(itemIDs, ep.RatingKey)
		}
	}
	
	eq.logger.WithFields(logrus.Fields{
		"series":  currentEp.GrandparentTitle,
		"season":  currentEp.ParentIndex,
		"count":   len(itemIDs),
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
```

### Configuration Changes

```go
// Add to config.go PlexConfig struct
type PlexConfig struct {
	Token   string
	Server  string
	Enabled bool
	
	// Episode queueing
	QueueNextEpisode bool
	QueueSeason      bool
	QueueSeries      bool
}

// Add to setDefaults()
v.SetDefault("PLEX_QUEUE_NEXT_EPISODE", false)
v.SetDefault("PLEX_QUEUE_SEASON", false)
v.SetDefault("PLEX_QUEUE_SERIES", false)

// Add to Load()
Plex: PlexConfig{
	Token:            v.GetString("PLEX_TOKEN"),
	Server:           v.GetString("PLEX_SERVER"),
	Enabled:          v.GetBool("PLEX_ENABLED"),
	QueueNextEpisode: v.GetBool("PLEX_QUEUE_NEXT_EPISODE"),
	QueueSeason:      v.GetBool("PLEX_QUEUE_SEASON"),
	QueueSeries:      v.GetBool("PLEX_QUEUE_SERIES"),
},

// Add validation
func validatePlexQueueConfig(config *PlexConfig) error {
	count := 0
	if config.QueueNextEpisode { count++ }
	if config.QueueSeason { count++ }
	if config.QueueSeries { count++ }
	
	if count > 1 {
		return fmt.Errorf("only one Plex queue mode can be enabled at a time")
	}
	return nil
}
```

### Integration with Webhook Handler

```go
// In webhooks/server.go handlePlex()
func (s *Server) handlePlex(c *fiber.Ctx) error {
	// ... existing code to process current episode ...
	
	// Queue additional episodes if configured
	if s.config.Plex.QueueNextEpisode || s.config.Plex.QueueSeason || s.config.Plex.QueueSeries {
		mode := determineQueueMode(s.config.Plex)
		
		queuer := plex.NewEpisodeQueuer(s.plexClient, s.logger)
		itemIDs, err := queuer.QueueEpisodes(c.Context(), ratingKey, mode)
		if err != nil {
			s.logger.WithError(err).Warn("Failed to queue additional episodes")
			// Continue processing, don't fail the webhook
		} else {
			// Queue each episode
			for _, itemID := range itemIDs {
				filePath, err := queuer.GetFilePath(c.Context(), itemID)
				if err != nil {
					s.logger.WithError(err).WithField("item_id", itemID).Warn("Failed to get file path")
					continue
				}
				
				// Queue transcription task
				task := queue.Task{
					ID:       fmt.Sprintf("plex-%s", itemID),
					FilePath: filePath,
					Priority: 2, // Standard priority
					PlexItemID: itemID,
				}
				
				if err := s.queue.Enqueue(c.Context(), task); err != nil {
					s.logger.WithError(err).WithField("file_path", filePath).Warn("Failed to queue episode")
				}
			}
		}
	}
	
	return c.SendStatus(fiber.StatusOK)
}

func determineQueueMode(config config.PlexConfig) plex.QueueMode {
	if config.QueueNextEpisode {
		return plex.QueueModeNext
	}
	if config.QueueSeason {
		return plex.QueueModeSeason
	}
	if config.QueueSeries {
		return plex.QueueModeSeries
	}
	return ""
}
```

---

## Testing Strategy

### Unit Tests (TDD - Write First!)

**client_test.go** - Mock HTTP responses
```go
func TestGetMetadata_Success(t *testing.T)
func TestGetMetadata_NotFound(t *testing.T)
func TestGetMetadata_InvalidXML(t *testing.T)
func TestGetMetadata_NetworkError(t *testing.T)
func TestGetChildren_Success(t *testing.T)
func TestGetChildren_EmptyResponse(t *testing.T)
```

**episode_queue_test.go** - Mock Plex client
```go
func TestQueueNextEpisode_SameSeason(t *testing.T)
func TestQueueNextEpisode_NextSeason(t *testing.T)
func TestQueueNextEpisode_EndOfSeries(t *testing.T)
func TestQueueSeasonEpisodes_MidSeason(t *testing.T)
func TestQueueSeasonEpisodes_LastEpisode(t *testing.T)
func TestQueueSeriesEpisodes_FromBeginning(t *testing.T)
func TestQueueSeriesEpisodes_MidSeries(t *testing.T)
func TestGetFilePath_Success(t *testing.T)
func TestGetFilePath_NoMediaParts(t *testing.T)
```

### Integration Tests

**mock_plex_server_test.go** - httptest server
```go
func TestPlexIntegration_QueueNextEpisode(t *testing.T) {
	// Start mock Plex server with XML responses
	// Test full flow: GetMetadata -> GetChildren -> Queue
}

func TestPlexIntegration_QueueSeason(t *testing.T) {
	// Mock 10-episode season
	// Verify all 10 episodes queued
}

func TestPlexIntegration_QueueSeries(t *testing.T) {
	// Mock 3 seasons with varying episode counts
	// Verify all episodes queued in order
}
```

### Manual Testing

```bash
# Test 1: Queue next episode
export PLEX_QUEUE_NEXT_EPISODE=true
# Trigger webhook for S01E01
# Verify S01E02 gets queued

# Test 2: Queue season
export PLEX_QUEUE_SEASON=true
# Trigger webhook for S01E01
# Verify S01E02-S01E10 all get queued

# Test 3: Queue series
export PLEX_QUEUE_SERIES=true
# Trigger webhook for S01E01
# Verify entire series (all seasons) gets queued

# Test 4: End of season boundary
export PLEX_QUEUE_NEXT_EPISODE=true
# Trigger webhook for S01E10 (last episode)
# Verify S02E01 gets queued (not S01E11)

# Test 5: End of series
export PLEX_QUEUE_NEXT_EPISODE=true
# Trigger webhook for S03E12 (last episode of series)
# Verify no error, graceful handling
```

---

## Definition of Done

- [ ] Story file created (this document)
- [ ] Configuration: PLEX_QUEUE_NEXT_EPISODE, PLEX_QUEUE_SEASON, PLEX_QUEUE_SERIES
- [ ] PlexClient implemented with GetMetadata, GetChildren
- [ ] EpisodeQueuer implemented with all three modes
- [ ] XML parsing working correctly
- [ ] Season boundaries detected correctly
- [ ] Series end handled gracefully
- [ ] Integration with webhook handler
- [ ] All unit tests passing (>80% coverage)
- [ ] Integration tests passing
- [ ] Manual testing completed
- [ ] Type checking passes (go vet, staticcheck)
- [ ] Code follows Go best practices
- [ ] Logging with structured fields
- [ ] Error handling comprehensive
- [ ] Work log created (0020_2026-02-16_epic08_story03_plex_episode_queue.md)
- [ ] Code committed and pushed

---

## Integration Points

- **webhooks.Server** - Calls episode queuer after processing current episode
- **queue.Queue** - Receives queued episode tasks
- **config.PlexConfig** - Reads configuration from environment
- **skip.Checker** - Applied to queued episodes (optional integration)

---

## Success Criteria

1. **Accuracy**: Queues correct episodes based on mode
2. **Reliability**: No panics or crashes on edge cases
3. **Performance**: Queues 100 episodes in < 5 seconds
4. **Logging**: Clear logs showing what was queued
5. **Error Handling**: Graceful degradation on API failures

---

## References

- **Original Implementation**: subgen.py lines 582-623, 1790-1889
- **Plex API Documentation**: https://github.com/Arcanemagus/plex-api/wiki
- **XML Parsing**: encoding/xml standard library
- **HTTP Mocking**: net/http/httptest

---

**Story Created**: 2026-02-16  
**Last Updated**: 2026-02-16  
**Target Completion**: 2026-02-17
