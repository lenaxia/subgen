package plex

import (
	"context"
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/sirupsen/logrus"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// Test fixtures
func setupMockPlexServer() *httptest.Server {
	// Mock responses based on rating keys
	return httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/xml")

		switch r.URL.Path {
		// S01E01 metadata
		case "/library/metadata/12345":
			fmt.Fprint(w, `<?xml version="1.0" encoding="UTF-8"?>
<MediaContainer size="1">
  <Video ratingKey="12345" parentRatingKey="5678" grandparentRatingKey="9999" type="episode" title="Episode 1" index="1" parentIndex="1" grandparentTitle="Test Show">
    <Media><Part file="/media/tv/show/s01e01.mkv"/></Media>
  </Video>
</MediaContainer>`)

		// S01E05 metadata (mid-season)
		case "/library/metadata/12349":
			fmt.Fprint(w, `<?xml version="1.0" encoding="UTF-8"?>
<MediaContainer size="1">
  <Video ratingKey="12349" parentRatingKey="5678" grandparentRatingKey="9999" type="episode" title="Episode 5" index="5" parentIndex="1" grandparentTitle="Test Show">
    <Media><Part file="/media/tv/show/s01e05.mkv"/></Media>
  </Video>
</MediaContainer>`)

		// S01E10 metadata (last episode of season 1)
		case "/library/metadata/12354":
			fmt.Fprint(w, `<?xml version="1.0" encoding="UTF-8"?>
<MediaContainer size="1">
  <Video ratingKey="12354" parentRatingKey="5678" grandparentRatingKey="9999" type="episode" title="Episode 10" index="10" parentIndex="1" grandparentTitle="Test Show">
    <Media><Part file="/media/tv/show/s01e10.mkv"/></Media>
  </Video>
</MediaContainer>`)

		// S02E05 metadata (mid-season 2)
		case "/library/metadata/12365":
			fmt.Fprint(w, `<?xml version="1.0" encoding="UTF-8"?>
<MediaContainer size="1">
  <Video ratingKey="12365" parentRatingKey="5679" grandparentRatingKey="9999" type="episode" title="Episode 5" index="5" parentIndex="2" grandparentTitle="Test Show">
    <Media><Part file="/media/tv/show/s02e05.mkv"/></Media>
  </Video>
</MediaContainer>`)

		// S03E08 metadata (last episode of series)
		case "/library/metadata/12378":
			fmt.Fprint(w, `<?xml version="1.0" encoding="UTF-8"?>
<MediaContainer size="1">
  <Video ratingKey="12378" parentRatingKey="5680" grandparentRatingKey="9999" type="episode" title="Episode 8" index="8" parentIndex="3" grandparentTitle="Test Show">
    <Media><Part file="/media/tv/show/s03e08.mkv"/></Media>
  </Video>
</MediaContainer>`)

		// Season 1 episodes (10 episodes)
		case "/library/metadata/5678/children":
			fmt.Fprint(w, `<?xml version="1.0" encoding="UTF-8"?>
<MediaContainer size="10">
  <Video ratingKey="12345" parentRatingKey="5678" grandparentRatingKey="9999" type="episode" title="Episode 1" index="1" parentIndex="1" grandparentTitle="Test Show">
    <Media><Part file="/media/tv/show/s01e01.mkv"/></Media>
  </Video>
  <Video ratingKey="12346" parentRatingKey="5678" grandparentRatingKey="9999" type="episode" title="Episode 2" index="2" parentIndex="1" grandparentTitle="Test Show">
    <Media><Part file="/media/tv/show/s01e02.mkv"/></Media>
  </Video>
  <Video ratingKey="12347" parentRatingKey="5678" grandparentRatingKey="9999" type="episode" title="Episode 3" index="3" parentIndex="1" grandparentTitle="Test Show">
    <Media><Part file="/media/tv/show/s01e03.mkv"/></Media>
  </Video>
  <Video ratingKey="12348" parentRatingKey="5678" grandparentRatingKey="9999" type="episode" title="Episode 4" index="4" parentIndex="1" grandparentTitle="Test Show">
    <Media><Part file="/media/tv/show/s01e04.mkv"/></Media>
  </Video>
  <Video ratingKey="12349" parentRatingKey="5678" grandparentRatingKey="9999" type="episode" title="Episode 5" index="5" parentIndex="1" grandparentTitle="Test Show">
    <Media><Part file="/media/tv/show/s01e05.mkv"/></Media>
  </Video>
  <Video ratingKey="12350" parentRatingKey="5678" grandparentRatingKey="9999" type="episode" title="Episode 6" index="6" parentIndex="1" grandparentTitle="Test Show">
    <Media><Part file="/media/tv/show/s01e06.mkv"/></Media>
  </Video>
  <Video ratingKey="12351" parentRatingKey="5678" grandparentRatingKey="9999" type="episode" title="Episode 7" index="7" parentIndex="1" grandparentTitle="Test Show">
    <Media><Part file="/media/tv/show/s01e07.mkv"/></Media>
  </Video>
  <Video ratingKey="12352" parentRatingKey="5678" grandparentRatingKey="9999" type="episode" title="Episode 8" index="8" parentIndex="1" grandparentTitle="Test Show">
    <Media><Part file="/media/tv/show/s01e08.mkv"/></Media>
  </Video>
  <Video ratingKey="12353" parentRatingKey="5678" grandparentRatingKey="9999" type="episode" title="Episode 9" index="9" parentIndex="1" grandparentTitle="Test Show">
    <Media><Part file="/media/tv/show/s01e09.mkv"/></Media>
  </Video>
  <Video ratingKey="12354" parentRatingKey="5678" grandparentRatingKey="9999" type="episode" title="Episode 10" index="10" parentIndex="1" grandparentTitle="Test Show">
    <Media><Part file="/media/tv/show/s01e10.mkv"/></Media>
  </Video>
</MediaContainer>`)

		// Season 2 episodes (8 episodes)
		case "/library/metadata/5679/children":
			fmt.Fprint(w, `<?xml version="1.0" encoding="UTF-8"?>
<MediaContainer size="8">
  <Video ratingKey="12355" parentRatingKey="5679" grandparentRatingKey="9999" type="episode" title="Episode 1" index="1" parentIndex="2" grandparentTitle="Test Show">
    <Media><Part file="/media/tv/show/s02e01.mkv"/></Media>
  </Video>
  <Video ratingKey="12356" parentRatingKey="5679" grandparentRatingKey="9999" type="episode" title="Episode 2" index="2" parentIndex="2" grandparentTitle="Test Show">
    <Media><Part file="/media/tv/show/s02e02.mkv"/></Media>
  </Video>
  <Video ratingKey="12357" parentRatingKey="5679" grandparentRatingKey="9999" type="episode" title="Episode 3" index="3" parentIndex="2" grandparentTitle="Test Show">
    <Media><Part file="/media/tv/show/s02e03.mkv"/></Media>
  </Video>
  <Video ratingKey="12358" parentRatingKey="5679" grandparentRatingKey="9999" type="episode" title="Episode 4" index="4" parentIndex="2" grandparentTitle="Test Show">
    <Media><Part file="/media/tv/show/s02e04.mkv"/></Media>
  </Video>
  <Video ratingKey="12365" parentRatingKey="5679" grandparentRatingKey="9999" type="episode" title="Episode 5" index="5" parentIndex="2" grandparentTitle="Test Show">
    <Media><Part file="/media/tv/show/s02e05.mkv"/></Media>
  </Video>
  <Video ratingKey="12366" parentRatingKey="5679" grandparentRatingKey="9999" type="episode" title="Episode 6" index="6" parentIndex="2" grandparentTitle="Test Show">
    <Media><Part file="/media/tv/show/s02e06.mkv"/></Media>
  </Video>
  <Video ratingKey="12367" parentRatingKey="5679" grandparentRatingKey="9999" type="episode" title="Episode 7" index="7" parentIndex="2" grandparentTitle="Test Show">
    <Media><Part file="/media/tv/show/s02e07.mkv"/></Media>
  </Video>
  <Video ratingKey="12368" parentRatingKey="5679" grandparentRatingKey="9999" type="episode" title="Episode 8" index="8" parentIndex="2" grandparentTitle="Test Show">
    <Media><Part file="/media/tv/show/s02e08.mkv"/></Media>
  </Video>
</MediaContainer>`)

		// Season 3 episodes (8 episodes)
		case "/library/metadata/5680/children":
			fmt.Fprint(w, `<?xml version="1.0" encoding="UTF-8"?>
<MediaContainer size="8">
  <Video ratingKey="12371" parentRatingKey="5680" grandparentRatingKey="9999" type="episode" title="Episode 1" index="1" parentIndex="3" grandparentTitle="Test Show">
    <Media><Part file="/media/tv/show/s03e01.mkv"/></Media>
  </Video>
  <Video ratingKey="12372" parentRatingKey="5680" grandparentRatingKey="9999" type="episode" title="Episode 2" index="2" parentIndex="3" grandparentTitle="Test Show">
    <Media><Part file="/media/tv/show/s03e02.mkv"/></Media>
  </Video>
  <Video ratingKey="12373" parentRatingKey="5680" grandparentRatingKey="9999" type="episode" title="Episode 3" index="3" parentIndex="3" grandparentTitle="Test Show">
    <Media><Part file="/media/tv/show/s03e03.mkv"/></Media>
  </Video>
  <Video ratingKey="12374" parentRatingKey="5680" grandparentRatingKey="9999" type="episode" title="Episode 4" index="4" parentIndex="3" grandparentTitle="Test Show">
    <Media><Part file="/media/tv/show/s03e04.mkv"/></Media>
  </Video>
  <Video ratingKey="12375" parentRatingKey="5680" grandparentRatingKey="9999" type="episode" title="Episode 5" index="5" parentIndex="3" grandparentTitle="Test Show">
    <Media><Part file="/media/tv/show/s03e05.mkv"/></Media>
  </Video>
  <Video ratingKey="12376" parentRatingKey="5680" grandparentRatingKey="9999" type="episode" title="Episode 6" index="6" parentIndex="3" grandparentTitle="Test Show">
    <Media><Part file="/media/tv/show/s03e06.mkv"/></Media>
  </Video>
  <Video ratingKey="12377" parentRatingKey="5680" grandparentRatingKey="9999" type="episode" title="Episode 7" index="7" parentIndex="3" grandparentTitle="Test Show">
    <Media><Part file="/media/tv/show/s03e07.mkv"/></Media>
  </Video>
  <Video ratingKey="12378" parentRatingKey="5680" grandparentRatingKey="9999" type="episode" title="Episode 8" index="8" parentIndex="3" grandparentTitle="Test Show">
    <Media><Part file="/media/tv/show/s03e08.mkv"/></Media>
  </Video>
</MediaContainer>`)

		// Series seasons (3 seasons)
		case "/library/metadata/9999/children":
			fmt.Fprint(w, `<?xml version="1.0" encoding="UTF-8"?>
<MediaContainer size="3">
  <Directory ratingKey="5678" type="season" index="1" title="Season 1"/>
  <Directory ratingKey="5679" type="season" index="2" title="Season 2"/>
  <Directory ratingKey="5680" type="season" index="3" title="Season 3"/>
</MediaContainer>`)

		default:
			w.WriteHeader(http.StatusNotFound)
		}
	}))
}

func TestQueueNextEpisode_SameSeason(t *testing.T) {
	server := setupMockPlexServer()
	defer server.Close()

	client := NewClient(server.URL, "test-token")
	logger := logrus.New()
	logger.SetLevel(logrus.DebugLevel)
	queuer := NewEpisodeQueuer(client, logger)

	// Queue next episode from S01E01
	itemIDs, err := queuer.QueueEpisodes(context.Background(), "12345", QueueModeNext)

	require.NoError(t, err)
	assert.Len(t, itemIDs, 1)
	assert.Equal(t, "12346", itemIDs[0]) // S01E02
}

func TestQueueNextEpisode_NextSeason(t *testing.T) {
	server := setupMockPlexServer()
	defer server.Close()

	client := NewClient(server.URL, "test-token")
	logger := logrus.New()
	logger.SetLevel(logrus.DebugLevel)
	queuer := NewEpisodeQueuer(client, logger)

	// Queue next episode from S01E10 (last episode of season 1)
	itemIDs, err := queuer.QueueEpisodes(context.Background(), "12354", QueueModeNext)

	require.NoError(t, err)
	assert.Len(t, itemIDs, 1)
	assert.Equal(t, "12355", itemIDs[0]) // S02E01
}

func TestQueueNextEpisode_EndOfSeries(t *testing.T) {
	server := setupMockPlexServer()
	defer server.Close()

	client := NewClient(server.URL, "test-token")
	logger := logrus.New()
	logger.SetLevel(logrus.DebugLevel)
	queuer := NewEpisodeQueuer(client, logger)

	// Queue next episode from S03E08 (last episode of series)
	itemIDs, err := queuer.QueueEpisodes(context.Background(), "12378", QueueModeNext)

	require.NoError(t, err)
	assert.Empty(t, itemIDs) // No next episode
}

func TestQueueSeasonEpisodes_FromBeginning(t *testing.T) {
	server := setupMockPlexServer()
	defer server.Close()

	client := NewClient(server.URL, "test-token")
	logger := logrus.New()
	queuer := NewEpisodeQueuer(client, logger)

	// Queue season from S01E01
	itemIDs, err := queuer.QueueEpisodes(context.Background(), "12345", QueueModeSeason)

	require.NoError(t, err)
	assert.Len(t, itemIDs, 10)           // All 10 episodes in season 1
	assert.Equal(t, "12345", itemIDs[0]) // S01E01
	assert.Equal(t, "12354", itemIDs[9]) // S01E10
}

func TestQueueSeasonEpisodes_MidSeason(t *testing.T) {
	server := setupMockPlexServer()
	defer server.Close()

	client := NewClient(server.URL, "test-token")
	logger := logrus.New()
	queuer := NewEpisodeQueuer(client, logger)

	// Queue season from S01E05 (mid-season)
	itemIDs, err := queuer.QueueEpisodes(context.Background(), "12349", QueueModeSeason)

	require.NoError(t, err)
	assert.Len(t, itemIDs, 6)            // Episodes 5-10
	assert.Equal(t, "12349", itemIDs[0]) // S01E05
	assert.Equal(t, "12354", itemIDs[5]) // S01E10
}

func TestQueueSeasonEpisodes_LastEpisode(t *testing.T) {
	server := setupMockPlexServer()
	defer server.Close()

	client := NewClient(server.URL, "test-token")
	logger := logrus.New()
	queuer := NewEpisodeQueuer(client, logger)

	// Queue season from S01E10 (last episode)
	itemIDs, err := queuer.QueueEpisodes(context.Background(), "12354", QueueModeSeason)

	require.NoError(t, err)
	assert.Len(t, itemIDs, 1)            // Only last episode
	assert.Equal(t, "12354", itemIDs[0]) // S01E10
}

func TestQueueSeriesEpisodes_FromBeginning(t *testing.T) {
	server := setupMockPlexServer()
	defer server.Close()

	client := NewClient(server.URL, "test-token")
	logger := logrus.New()
	queuer := NewEpisodeQueuer(client, logger)

	// Queue series from S01E01
	itemIDs, err := queuer.QueueEpisodes(context.Background(), "12345", QueueModeSeries)

	require.NoError(t, err)
	assert.Len(t, itemIDs, 26)            // 10 + 8 + 8 = 26 episodes total
	assert.Equal(t, "12345", itemIDs[0])  // S01E01
	assert.Equal(t, "12354", itemIDs[9])  // S01E10
	assert.Equal(t, "12355", itemIDs[10]) // S02E01
	assert.Equal(t, "12378", itemIDs[25]) // S03E08
}

func TestQueueSeriesEpisodes_MidSeries(t *testing.T) {
	server := setupMockPlexServer()
	defer server.Close()

	client := NewClient(server.URL, "test-token")
	logger := logrus.New()
	queuer := NewEpisodeQueuer(client, logger)

	// Queue series from S02E05 (mid-series)
	itemIDs, err := queuer.QueueEpisodes(context.Background(), "12365", QueueModeSeries)

	require.NoError(t, err)
	// Should get: S02E05-S02E08 (4) + S03E01-S03E08 (8) = 12 episodes
	assert.Len(t, itemIDs, 12)
	assert.Equal(t, "12365", itemIDs[0])  // S02E05
	assert.Equal(t, "12378", itemIDs[11]) // S03E08
}

func TestGetFilePath_Success(t *testing.T) {
	server := setupMockPlexServer()
	defer server.Close()

	client := NewClient(server.URL, "test-token")
	logger := logrus.New()
	queuer := NewEpisodeQueuer(client, logger)

	// Get file path for S01E01
	filePath, err := queuer.GetFilePath(context.Background(), "12345")

	require.NoError(t, err)
	assert.Equal(t, "/media/tv/show/s01e01.mkv", filePath)
}

func TestGetFilePath_NoMediaParts(t *testing.T) {
	// Setup server with response missing media parts
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		fmt.Fprint(w, `<?xml version="1.0" encoding="UTF-8"?>
<MediaContainer size="1">
  <Video ratingKey="12345" parentRatingKey="5678" grandparentRatingKey="9999" type="episode" title="Episode 1" index="1" parentIndex="1" grandparentTitle="Test Show">
  </Video>
</MediaContainer>`)
	}))
	defer server.Close()

	client := NewClient(server.URL, "test-token")
	logger := logrus.New()
	queuer := NewEpisodeQueuer(client, logger)

	// Get file path - should error
	filePath, err := queuer.GetFilePath(context.Background(), "12345")

	assert.Error(t, err)
	assert.Empty(t, filePath)
	assert.Contains(t, err.Error(), "no media parts found")
}

func TestQueueEpisodes_InvalidMode(t *testing.T) {
	server := setupMockPlexServer()
	defer server.Close()

	client := NewClient(server.URL, "test-token")
	logger := logrus.New()
	queuer := NewEpisodeQueuer(client, logger)

	// Try invalid mode
	itemIDs, err := queuer.QueueEpisodes(context.Background(), "12345", QueueMode("invalid"))

	assert.Error(t, err)
	assert.Nil(t, itemIDs)
	assert.Contains(t, err.Error(), "invalid queue mode")
}
