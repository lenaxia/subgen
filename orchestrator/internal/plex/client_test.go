package plex

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// Mock Plex XML responses
const mockEpisodeXML = `<?xml version="1.0" encoding="UTF-8"?>
<MediaContainer size="1">
  <Video ratingKey="12345"
         parentRatingKey="5678"
         grandparentRatingKey="9999"
         type="episode"
         title="Pilot"
         index="1"
         parentIndex="1"
         grandparentTitle="Test Show">
    <Media>
      <Part file="/media/tv/show/s01e01.mkv"/>
    </Media>
  </Video>
</MediaContainer>`

const mockSeasonEpisodesXML = `<?xml version="1.0" encoding="UTF-8"?>
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
</MediaContainer>`

const mockSeasonsXML = `<?xml version="1.0" encoding="UTF-8"?>
<MediaContainer size="2">
  <Directory ratingKey="5678" type="season" index="1" title="Season 1"/>
  <Directory ratingKey="5679" type="season" index="2" title="Season 2"/>
</MediaContainer>`

const mockEmptyXML = `<?xml version="1.0" encoding="UTF-8"?>
<MediaContainer size="0">
</MediaContainer>`

func TestClient_GetMetadata_Success(t *testing.T) {
	// Setup mock server
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Verify request
		assert.Equal(t, "/library/metadata/12345", r.URL.Path)
		assert.Equal(t, "test-token", r.Header.Get("X-Plex-Token"))

		// Return mock response
		w.Header().Set("Content-Type", "application/xml")
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(mockEpisodeXML))
	}))
	defer server.Close()

	// Create client
	client := NewClient(server.URL, "test-token")

	// Test GetMetadata
	video, err := client.GetMetadata(context.Background(), "12345")

	// Assertions
	require.NoError(t, err)
	assert.Equal(t, "12345", video.RatingKey)
	assert.Equal(t, "5678", video.ParentRatingKey)
	assert.Equal(t, "9999", video.GrandparentRatingKey)
	assert.Equal(t, "episode", video.Type)
	assert.Equal(t, "Pilot", video.Title)
	assert.Equal(t, 1, video.Index)
	assert.Equal(t, 1, video.ParentIndex)
	assert.Equal(t, "Test Show", video.GrandparentTitle)
	require.Len(t, video.Media, 1)
	require.Len(t, video.Media[0].Part, 1)
	assert.Equal(t, "/media/tv/show/s01e01.mkv", video.Media[0].Part[0].File)
}

func TestClient_GetMetadata_NotFound(t *testing.T) {
	// Setup mock server
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusNotFound)
		_, _ = w.Write([]byte("Not Found"))
	}))
	defer server.Close()

	// Create client
	client := NewClient(server.URL, "test-token")

	// Test GetMetadata
	video, err := client.GetMetadata(context.Background(), "99999")

	// Assertions
	assert.Error(t, err)
	assert.Nil(t, video)
	assert.Contains(t, err.Error(), "404")
}

func TestClient_GetMetadata_InvalidXML(t *testing.T) {
	// Setup mock server
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/xml")
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte("not valid xml"))
	}))
	defer server.Close()

	// Create client
	client := NewClient(server.URL, "test-token")

	// Test GetMetadata
	video, err := client.GetMetadata(context.Background(), "12345")

	// Assertions
	assert.Error(t, err)
	assert.Nil(t, video)
	assert.Contains(t, err.Error(), "decode xml")
}

func TestClient_GetMetadata_EmptyResponse(t *testing.T) {
	// Setup mock server
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/xml")
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(mockEmptyXML))
	}))
	defer server.Close()

	// Create client
	client := NewClient(server.URL, "test-token")

	// Test GetMetadata
	video, err := client.GetMetadata(context.Background(), "12345")

	// Assertions
	assert.Error(t, err)
	assert.Nil(t, video)
	assert.Contains(t, err.Error(), "no video found")
}

func TestClient_GetChildren_Episodes(t *testing.T) {
	// Setup mock server
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Verify request
		assert.Equal(t, "/library/metadata/5678/children", r.URL.Path)
		assert.Equal(t, "test-token", r.Header.Get("X-Plex-Token"))

		// Return mock response
		w.Header().Set("Content-Type", "application/xml")
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(mockSeasonEpisodesXML))
	}))
	defer server.Close()

	// Create client
	client := NewClient(server.URL, "test-token")

	// Test GetChildren
	videos, dirs, err := client.GetChildren(context.Background(), "5678")

	// Assertions
	require.NoError(t, err)
	assert.Len(t, videos, 3)
	assert.Empty(t, dirs)

	assert.Equal(t, "12345", videos[0].RatingKey)
	assert.Equal(t, "Episode 1", videos[0].Title)
	assert.Equal(t, 1, videos[0].Index)

	assert.Equal(t, "12346", videos[1].RatingKey)
	assert.Equal(t, "Episode 2", videos[1].Title)
	assert.Equal(t, 2, videos[1].Index)

	assert.Equal(t, "12347", videos[2].RatingKey)
	assert.Equal(t, "Episode 3", videos[2].Title)
	assert.Equal(t, 3, videos[2].Index)
}

func TestClient_GetChildren_Seasons(t *testing.T) {
	// Setup mock server
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Verify request
		assert.Equal(t, "/library/metadata/9999/children", r.URL.Path)

		// Return mock response
		w.Header().Set("Content-Type", "application/xml")
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(mockSeasonsXML))
	}))
	defer server.Close()

	// Create client
	client := NewClient(server.URL, "test-token")

	// Test GetChildren
	videos, dirs, err := client.GetChildren(context.Background(), "9999")

	// Assertions
	require.NoError(t, err)
	assert.Empty(t, videos)
	assert.Len(t, dirs, 2)

	assert.Equal(t, "5678", dirs[0].RatingKey)
	assert.Equal(t, "season", dirs[0].Type)
	assert.Equal(t, 1, dirs[0].Index)
	assert.Equal(t, "Season 1", dirs[0].Title)

	assert.Equal(t, "5679", dirs[1].RatingKey)
	assert.Equal(t, "season", dirs[1].Type)
	assert.Equal(t, 2, dirs[1].Index)
	assert.Equal(t, "Season 2", dirs[1].Title)
}

func TestClient_GetChildren_EmptyResponse(t *testing.T) {
	// Setup mock server
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/xml")
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(mockEmptyXML))
	}))
	defer server.Close()

	// Create client
	client := NewClient(server.URL, "test-token")

	// Test GetChildren
	videos, dirs, err := client.GetChildren(context.Background(), "5678")

	// Assertions
	require.NoError(t, err)
	assert.Empty(t, videos)
	assert.Empty(t, dirs)
}

func TestClient_GetChildren_NetworkError(t *testing.T) {
	// Create client with invalid URL
	client := NewClient("http://invalid-host-that-does-not-exist:99999", "test-token")

	// Test GetChildren
	videos, dirs, err := client.GetChildren(context.Background(), "5678")

	// Assertions
	assert.Error(t, err)
	assert.Nil(t, videos)
	assert.Nil(t, dirs)
}

func TestClient_GetChildren_Unauthorized(t *testing.T) {
	// Setup mock server
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusUnauthorized)
		_, _ = w.Write([]byte("Unauthorized"))
	}))
	defer server.Close()

	// Create client
	client := NewClient(server.URL, "invalid-token")

	// Test GetChildren
	videos, dirs, err := client.GetChildren(context.Background(), "5678")

	// Assertions
	assert.Error(t, err)
	assert.Nil(t, videos)
	assert.Nil(t, dirs)
	assert.Contains(t, err.Error(), "401")
}

func TestClient_GetChildren_ContextCanceled(t *testing.T) {
	// Setup mock server that never responds
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		<-r.Context().Done()
	}))
	defer server.Close()

	// Create client
	client := NewClient(server.URL, "test-token")

	// Create canceled context
	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	// Test GetChildren
	videos, dirs, err := client.GetChildren(ctx, "5678")

	// Assertions
	assert.Error(t, err)
	assert.Nil(t, videos)
	assert.Nil(t, dirs)
}
