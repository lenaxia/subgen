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

func TestJellyfinGetFilePath_Success(t *testing.T) {
	// Mock Jellyfin server
	callCount := 0
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// First call: get admin user
		if r.URL.Path == "/Users" {
			callCount++
			assert.Equal(t, "MediaBrowser Token=test-token", r.Header.Get("Authorization"))
			assert.Equal(t, "application/json", r.Header.Get("Accept"))
			w.WriteHeader(http.StatusOK)
			w.Write([]byte(`[
				{"Id": "user1", "Name": "Regular", "Policy": {"IsAdministrator": false}},
				{"Id": "admin1", "Name": "Admin", "Policy": {"IsAdministrator": true}}
			]`))
			return
		}

		// Second call: get item
		if r.URL.Path == "/Users/admin1/Items/item123" {
			callCount++
			assert.Equal(t, "MediaBrowser Token=test-token", r.Header.Get("Authorization"))
			w.WriteHeader(http.StatusOK)
			w.Write([]byte(`{
				"Path": "/media/tv/show/S01E01.mkv",
				"Name": "Episode 1",
				"Type": "Episode"
			}`))
			return
		}

		t.Fatalf("Unexpected request: %s", r.URL.Path)
	}))
	defer server.Close()

	client := NewJellyfinClient(server.URL, "test-token", DefaultClientConfig(), logrus.New())
	filePath, err := client.GetFilePath(context.Background(), "item123")

	require.NoError(t, err)
	assert.Equal(t, "/media/tv/show/S01E01.mkv", filePath)
	assert.Equal(t, 2, callCount, "Should make exactly 2 API calls")
}

func TestJellyfinGetFilePath_AdminUserCached(t *testing.T) {
	// Verify admin user ID is cached and not fetched on second call
	usersCallCount := 0
	itemCallCount := 0

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/Users" {
			usersCallCount++
			w.WriteHeader(http.StatusOK)
			w.Write([]byte(`[{"Id": "admin1", "Name": "Admin", "Policy": {"IsAdministrator": true}}]`))
			return
		}

		if r.URL.Path == "/Users/admin1/Items/item123" || r.URL.Path == "/Users/admin1/Items/item456" {
			itemCallCount++
			w.WriteHeader(http.StatusOK)
			w.Write([]byte(`{"Path": "/media/file.mkv", "Name": "File", "Type": "Episode"}`))
			return
		}

		t.Fatalf("Unexpected request: %s", r.URL.Path)
	}))
	defer server.Close()

	client := NewJellyfinClient(server.URL, "test-token", DefaultClientConfig(), logrus.New())

	// First call - should fetch admin user
	_, err := client.GetFilePath(context.Background(), "item123")
	require.NoError(t, err)

	// Second call - should reuse cached admin user
	_, err = client.GetFilePath(context.Background(), "item456")
	require.NoError(t, err)

	assert.Equal(t, 1, usersCallCount, "Should fetch admin user only once")
	assert.Equal(t, 2, itemCallCount, "Should fetch both items")
}

func TestJellyfinGetFilePath_NoAdminUser(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/Users" {
			w.WriteHeader(http.StatusOK)
			w.Write([]byte(`[
				{"Id": "user1", "Name": "Regular", "Policy": {"IsAdministrator": false}}
			]`))
			return
		}
		t.Fatalf("Unexpected request: %s", r.URL.Path)
	}))
	defer server.Close()

	client := NewJellyfinClient(server.URL, "test-token", DefaultClientConfig(), logrus.New())
	filePath, err := client.GetFilePath(context.Background(), "item123")

	require.Error(t, err)
	assert.Contains(t, err.Error(), "no administrator user found")
	assert.Equal(t, "", filePath)
}

func TestJellyfinGetFilePath_InvalidItemID(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/Users" {
			w.WriteHeader(http.StatusOK)
			w.Write([]byte(`[{"Id": "admin1", "Name": "Admin", "Policy": {"IsAdministrator": true}}]`))
			return
		}

		if r.URL.Path == "/Users/admin1/Items/invalid" {
			w.WriteHeader(http.StatusNotFound)
			w.Write([]byte("Not Found"))
			return
		}

		t.Fatalf("Unexpected request: %s", r.URL.Path)
	}))
	defer server.Close()

	client := NewJellyfinClient(server.URL, "test-token", DefaultClientConfig(), logrus.New())
	filePath, err := client.GetFilePath(context.Background(), "invalid")

	require.Error(t, err)
	assert.Contains(t, err.Error(), "404")
	assert.Equal(t, "", filePath)
}

func TestJellyfinGetFilePath_EmptyPath(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/Users" {
			w.WriteHeader(http.StatusOK)
			w.Write([]byte(`[{"Id": "admin1", "Name": "Admin", "Policy": {"IsAdministrator": true}}]`))
			return
		}

		if r.URL.Path == "/Users/admin1/Items/item123" {
			w.WriteHeader(http.StatusOK)
			w.Write([]byte(`{"Path": "", "Name": "File", "Type": "Episode"}`))
			return
		}

		t.Fatalf("Unexpected request: %s", r.URL.Path)
	}))
	defer server.Close()

	client := NewJellyfinClient(server.URL, "test-token", DefaultClientConfig(), logrus.New())
	filePath, err := client.GetFilePath(context.Background(), "item123")

	require.Error(t, err)
	assert.Contains(t, err.Error(), "file path is empty")
	assert.Equal(t, "", filePath)
}

func TestJellyfinGetFilePath_InvalidJSON(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/Users" {
			w.WriteHeader(http.StatusOK)
			w.Write([]byte(`[{"Id": "admin1", "Name": "Admin", "Policy": {"IsAdministrator": true}}]`))
			return
		}

		if r.URL.Path == "/Users/admin1/Items/item123" {
			w.WriteHeader(http.StatusOK)
			w.Write([]byte("Not JSON"))
			return
		}

		t.Fatalf("Unexpected request: %s", r.URL.Path)
	}))
	defer server.Close()

	client := NewJellyfinClient(server.URL, "test-token", DefaultClientConfig(), logrus.New())
	filePath, err := client.GetFilePath(context.Background(), "item123")

	require.Error(t, err)
	assert.Contains(t, err.Error(), "failed to parse JSON response")
	assert.Equal(t, "", filePath)
}

func TestJellyfinRefreshMetadata_Success(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, "POST", r.Method)
		assert.Equal(t, "/Items/item123/Refresh", r.URL.Path)
		assert.Equal(t, "FullRefresh", r.URL.Query().Get("MetadataRefreshMode"))
		assert.Equal(t, "MediaBrowser Token=test-token", r.Header.Get("Authorization"))
		w.WriteHeader(http.StatusNoContent)
	}))
	defer server.Close()

	client := NewJellyfinClient(server.URL, "test-token", DefaultClientConfig(), logrus.New())
	err := client.RefreshMetadata(context.Background(), "item123")

	require.NoError(t, err)
}

func TestJellyfinRefreshMetadata_InvalidItemID(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusNotFound)
		w.Write([]byte("Not Found"))
	}))
	defer server.Close()

	client := NewJellyfinClient(server.URL, "test-token", DefaultClientConfig(), logrus.New())
	err := client.RefreshMetadata(context.Background(), "invalid")

	require.Error(t, err)
	assert.Contains(t, err.Error(), "404")
}

func TestJellyfinRefreshMetadata_ServerError(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
		w.Write([]byte("Internal Server Error"))
	}))
	defer server.Close()

	client := NewJellyfinClient(server.URL, "test-token", DefaultClientConfig(), logrus.New())
	err := client.RefreshMetadata(context.Background(), "item123")

	require.Error(t, err)
	assert.Contains(t, err.Error(), "500")
}

func TestJellyfinGetFilePath_ContextCanceled(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		time.Sleep(100 * time.Millisecond)
		w.WriteHeader(http.StatusOK)
	}))
	defer server.Close()

	ctx, cancel := context.WithCancel(context.Background())
	cancel() // Cancel immediately

	client := NewJellyfinClient(server.URL, "test-token", DefaultClientConfig(), logrus.New())
	filePath, err := client.GetFilePath(ctx, "item123")

	require.Error(t, err)
	assert.Contains(t, err.Error(), "context canceled")
	assert.Equal(t, "", filePath)
}

func TestJellyfinClient_ConnectionPooling(t *testing.T) {
	// Verify that multiple requests reuse the same connection
	callCount := 0
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		callCount++
		if r.URL.Path == "/Users" {
			w.WriteHeader(http.StatusOK)
			w.Write([]byte(`[{"Id": "admin1", "Name": "Admin", "Policy": {"IsAdministrator": true}}]`))
			return
		}

		w.WriteHeader(http.StatusOK)
		w.Write([]byte(`{"Path": "/media/file.mkv", "Name": "File", "Type": "Episode"}`))
	}))
	defer server.Close()

	config := DefaultClientConfig()
	config.MaxIdleConns = 5
	client := NewJellyfinClient(server.URL, "test-token", config, logrus.New())

	// Make multiple requests
	for i := 0; i < 5; i++ {
		_, err := client.GetFilePath(context.Background(), "item123")
		require.NoError(t, err)
	}

	// Should be 6 calls: 1 for admin user + 5 for items
	assert.Equal(t, 6, callCount)
}
