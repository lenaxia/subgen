package mediaserver

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"sync"

	"github.com/sirupsen/logrus"
)

// JellyfinClient implements MediaServerClient for Jellyfin
type JellyfinClient struct {
	serverURL  string
	token      string
	httpClient *http.Client
	log        *logrus.Logger

	// Cache admin user ID (fetched once)
	adminUserID      string
	adminUserIDMutex sync.RWMutex
}

// NewJellyfinClient creates a new Jellyfin API client
func NewJellyfinClient(serverURL, token string, config ClientConfig, log *logrus.Logger) *JellyfinClient {
	return &JellyfinClient{
		serverURL: serverURL,
		token:     token,
		httpClient: &http.Client{
			Timeout: config.Timeout,
			Transport: &http.Transport{
				MaxIdleConns:      config.MaxIdleConns,
				IdleConnTimeout:   config.IdleConnTimeout,
				DisableKeepAlives: false,
			},
		},
		log: log,
	}
}

// GetFilePath fetches the file system path for a Jellyfin item ID
func (c *JellyfinClient) GetFilePath(ctx context.Context, itemID string) (string, error) {
	// Get admin user ID (cached after first call)
	adminUserID, err := c.getAdminUserID(ctx)
	if err != nil {
		return "", fmt.Errorf("failed to get admin user: %w", err)
	}

	url := fmt.Sprintf("%s/Users/%s/Items/%s", c.serverURL, adminUserID, itemID)

	req, err := http.NewRequestWithContext(ctx, "GET", url, nil)
	if err != nil {
		return "", fmt.Errorf("failed to create request: %w", err)
	}

	req.Header.Set("Authorization", fmt.Sprintf("MediaBrowser Token=%s", c.token))
	req.Header.Set("Accept", "application/json")

	c.log.WithFields(logrus.Fields{
		"item_id": itemID,
		"url":     url,
	}).Debug("Fetching Jellyfin file path")

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return "", fmt.Errorf("request failed: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return "", fmt.Errorf("jellyfin API returned %d: %s", resp.StatusCode, string(body))
	}

	// Parse JSON response
	var item JellyfinItem
	if err := json.NewDecoder(resp.Body).Decode(&item); err != nil {
		return "", fmt.Errorf("failed to parse JSON response: %w", err)
	}

	if item.Path == "" {
		return "", fmt.Errorf("file path is empty in response")
	}

	c.log.WithFields(logrus.Fields{
		"item_id":   itemID,
		"file_path": item.Path,
	}).Info("Retrieved Jellyfin file path")

	return item.Path, nil
}

// RefreshMetadata triggers Jellyfin to rescan metadata for an item
func (c *JellyfinClient) RefreshMetadata(ctx context.Context, itemID string) error {
	url := fmt.Sprintf("%s/Items/%s/Refresh?MetadataRefreshMode=FullRefresh", c.serverURL, itemID)

	req, err := http.NewRequestWithContext(ctx, "POST", url, nil)
	if err != nil {
		return fmt.Errorf("failed to create request: %w", err)
	}

	req.Header.Set("Authorization", fmt.Sprintf("MediaBrowser Token=%s", c.token))

	c.log.WithFields(logrus.Fields{
		"item_id": itemID,
		"url":     url,
	}).Debug("Refreshing Jellyfin metadata")

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return fmt.Errorf("request failed: %w", err)
	}
	defer resp.Body.Close()

	// Jellyfin returns 204 No Content on success
	if resp.StatusCode != http.StatusNoContent {
		body, _ := io.ReadAll(resp.Body)
		return fmt.Errorf("jellyfin API returned %d: %s", resp.StatusCode, string(body))
	}

	c.log.WithField("item_id", itemID).Info("Jellyfin metadata refresh initiated")

	return nil
}

// getAdminUserID fetches and caches the admin user ID
func (c *JellyfinClient) getAdminUserID(ctx context.Context) (string, error) {
	// Check cache first (read lock)
	c.adminUserIDMutex.RLock()
	if c.adminUserID != "" {
		adminID := c.adminUserID
		c.adminUserIDMutex.RUnlock()
		return adminID, nil
	}
	c.adminUserIDMutex.RUnlock()

	// Not cached, fetch it (write lock)
	c.adminUserIDMutex.Lock()
	defer c.adminUserIDMutex.Unlock()

	// Double-check after acquiring write lock
	if c.adminUserID != "" {
		return c.adminUserID, nil
	}

	url := fmt.Sprintf("%s/Users", c.serverURL)

	req, err := http.NewRequestWithContext(ctx, "GET", url, nil)
	if err != nil {
		return "", fmt.Errorf("failed to create request: %w", err)
	}

	req.Header.Set("Authorization", fmt.Sprintf("MediaBrowser Token=%s", c.token))
	req.Header.Set("Accept", "application/json")

	c.log.Debug("Fetching Jellyfin admin user ID")

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return "", fmt.Errorf("request failed: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return "", fmt.Errorf("jellyfin API returned %d: %s", resp.StatusCode, string(body))
	}

	// Parse users array
	var users []JellyfinUser
	if err := json.NewDecoder(resp.Body).Decode(&users); err != nil {
		return "", fmt.Errorf("failed to parse JSON response: %w", err)
	}

	// Find admin user
	for _, user := range users {
		if user.Policy.IsAdministrator {
			c.adminUserID = user.ID
			c.log.WithField("admin_id", c.adminUserID).Info("Cached Jellyfin admin user ID")
			return c.adminUserID, nil
		}
	}

	return "", fmt.Errorf("no administrator user found in Jellyfin")
}

// JellyfinItem represents a Jellyfin media item
type JellyfinItem struct {
	Path string `json:"Path"`
	Name string `json:"Name"`
	Type string `json:"Type"`
}

// JellyfinUser represents a Jellyfin user
type JellyfinUser struct {
	ID     string `json:"Id"`
	Name   string `json:"Name"`
	Policy struct {
		IsAdministrator bool `json:"IsAdministrator"`
	} `json:"Policy"`
}
