package plex

import (
	"context"
	"encoding/xml"
	"fmt"
	"io"
	"net/http"
	"time"
)

// Client provides HTTP access to Plex Media Server XML API
type Client struct {
	baseURL    string
	token      string
	httpClient *http.Client
}

// NewClient creates a new Plex API client
func NewClient(baseURL, token string) *Client {
	return &Client{
		baseURL: baseURL,
		token:   token,
		httpClient: &http.Client{
			Timeout: 30 * time.Second,
		},
	}
}

// GetMetadata retrieves metadata for a Plex item by ratingKey
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
