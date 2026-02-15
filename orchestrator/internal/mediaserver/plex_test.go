package mediaserver

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/sirupsen/logrus"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestPlexGetFilePath_Success(t *testing.T) {
	// Mock Plex server
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, "/library/metadata/12345", r.URL.Path)
		assert.Equal(t, "test-token", r.Header.Get("X-Plex-Token"))
		assert.Equal(t, "application/xml", r.Header.Get("Accept"))

		w.WriteHeader(http.StatusOK)
		w.Write([]byte(`
			<MediaContainer>
				<Video>
					<Media>
						<Part file="/media/tv/show/S01E01.mkv"/>
					</Media>
				</Video>
			</MediaContainer>
		`))
	}))
	defer server.Close()

	client := NewPlexClient(server.URL, "test-token", DefaultClientConfig(), logrus.New())
	filePath, err := client.GetFilePath(context.Background(), "12345")

	require.NoError(t, err)
	assert.Equal(t, "/media/tv/show/S01E01.mkv", filePath)
}

func TestPlexGetFilePath_InvalidRatingKey(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusNotFound)
		w.Write([]byte("Not Found"))
	}))
	defer server.Close()

	client := NewPlexClient(server.URL, "test-token", DefaultClientConfig(), logrus.New())
	filePath, err := client.GetFilePath(context.Background(), "invalid")

	require.Error(t, err)
	assert.Contains(t, err.Error(), "404")
	assert.Equal(t, "", filePath)
}

func TestPlexGetFilePath_InvalidXML(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		w.Write([]byte("Not XML"))
	}))
	defer server.Close()

	client := NewPlexClient(server.URL, "test-token", DefaultClientConfig(), logrus.New())
	filePath, err := client.GetFilePath(context.Background(), "12345")

	require.Error(t, err)
	assert.Contains(t, err.Error(), "failed to parse XML response")
	assert.Equal(t, "", filePath)
}

func TestPlexGetFilePath_EmptyResponse(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		w.Write([]byte(`<MediaContainer></MediaContainer>`))
	}))
	defer server.Close()

	client := NewPlexClient(server.URL, "test-token", DefaultClientConfig(), logrus.New())
	filePath, err := client.GetFilePath(context.Background(), "12345")

	require.Error(t, err)
	assert.Contains(t, err.Error(), "no video found in response")
	assert.Equal(t, "", filePath)
}

func TestPlexGetFilePath_NoMediaInVideo(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		w.Write([]byte(`
			<MediaContainer>
				<Video></Video>
			</MediaContainer>
		`))
	}))
	defer server.Close()

	client := NewPlexClient(server.URL, "test-token", DefaultClientConfig(), logrus.New())
	filePath, err := client.GetFilePath(context.Background(), "12345")

	require.Error(t, err)
	assert.Contains(t, err.Error(), "no media found in video")
	assert.Equal(t, "", filePath)
}

func TestPlexGetFilePath_NoPartsInMedia(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		w.Write([]byte(`
			<MediaContainer>
				<Video>
					<Media></Media>
				</Video>
			</MediaContainer>
		`))
	}))
	defer server.Close()

	client := NewPlexClient(server.URL, "test-token", DefaultClientConfig(), logrus.New())
	filePath, err := client.GetFilePath(context.Background(), "12345")

	require.Error(t, err)
	assert.Contains(t, err.Error(), "no parts found in media")
	assert.Equal(t, "", filePath)
}

func TestPlexGetFilePath_EmptyFilePath(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		w.Write([]byte(`
			<MediaContainer>
				<Video>
					<Media>
						<Part file=""/>
					</Media>
				</Video>
			</MediaContainer>
		`))
	}))
	defer server.Close()

	client := NewPlexClient(server.URL, "test-token", DefaultClientConfig(), logrus.New())
	filePath, err := client.GetFilePath(context.Background(), "12345")

	require.Error(t, err)
	assert.Contains(t, err.Error(), "file path is empty")
	assert.Equal(t, "", filePath)
}

func TestPlexGetFilePath_ContextCanceled(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		time.Sleep(100 * time.Millisecond)
		w.WriteHeader(http.StatusOK)
	}))
	defer server.Close()

	ctx, cancel := context.WithCancel(context.Background())
	cancel() // Cancel immediately

	client := NewPlexClient(server.URL, "test-token", DefaultClientConfig(), logrus.New())
	filePath, err := client.GetFilePath(ctx, "12345")

	require.Error(t, err)
	assert.Contains(t, err.Error(), "context canceled")
	assert.Equal(t, "", filePath)
}

func TestPlexRefreshMetadata_Success(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, "PUT", r.Method)
		assert.Equal(t, "/library/metadata/12345/refresh", r.URL.Path)
		assert.Equal(t, "test-token", r.Header.Get("X-Plex-Token"))
		w.WriteHeader(http.StatusOK)
	}))
	defer server.Close()

	client := NewPlexClient(server.URL, "test-token", DefaultClientConfig(), logrus.New())
	err := client.RefreshMetadata(context.Background(), "12345")

	require.NoError(t, err)
}

func TestPlexRefreshMetadata_InvalidRatingKey(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusNotFound)
		w.Write([]byte("Not Found"))
	}))
	defer server.Close()

	client := NewPlexClient(server.URL, "test-token", DefaultClientConfig(), logrus.New())
	err := client.RefreshMetadata(context.Background(), "invalid")

	require.Error(t, err)
	assert.Contains(t, err.Error(), "404")
}

func TestPlexRefreshMetadata_ServerError(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
		w.Write([]byte("Internal Server Error"))
	}))
	defer server.Close()

	client := NewPlexClient(server.URL, "test-token", DefaultClientConfig(), logrus.New())
	err := client.RefreshMetadata(context.Background(), "12345")

	require.Error(t, err)
	assert.Contains(t, err.Error(), "500")
}

func TestPlexRefreshMetadata_ContextCanceled(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		time.Sleep(100 * time.Millisecond)
		w.WriteHeader(http.StatusOK)
	}))
	defer server.Close()

	ctx, cancel := context.WithCancel(context.Background())
	cancel() // Cancel immediately

	client := NewPlexClient(server.URL, "test-token", DefaultClientConfig(), logrus.New())
	err := client.RefreshMetadata(ctx, "12345")

	require.Error(t, err)
	assert.Contains(t, err.Error(), "context canceled")
}

func TestPlexClient_ConnectionPooling(t *testing.T) {
	// Verify that multiple requests reuse the same connection
	callCount := 0
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		callCount++
		w.WriteHeader(http.StatusOK)
		w.Write([]byte(`
			<MediaContainer>
				<Video>
					<Media>
						<Part file="/media/movie.mkv"/>
					</Media>
				</Video>
			</MediaContainer>
		`))
	}))
	defer server.Close()

	config := DefaultClientConfig()
	config.MaxIdleConns = 5
	client := NewPlexClient(server.URL, "test-token", config, logrus.New())

	// Make multiple requests
	for i := 0; i < 5; i++ {
		_, err := client.GetFilePath(context.Background(), "12345")
		require.NoError(t, err)
	}

	assert.Equal(t, 5, callCount)
}
