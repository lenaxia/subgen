package mediaserver

import (
	"context"
	"encoding/xml"
	"fmt"
	"io"
	"net/http"

	"github.com/sirupsen/logrus"
)

// PlexClient implements MediaServerClient for Plex Media Server
type PlexClient struct {
	serverURL  string
	token      string
	httpClient *http.Client
	log        *logrus.Logger
}

// NewPlexClient creates a new Plex API client
func NewPlexClient(serverURL, token string, config ClientConfig, log *logrus.Logger) *PlexClient {
	return &PlexClient{
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

// GetFilePath fetches the file system path for a Plex rating key
// Implements MediaServerClient interface
func (c *PlexClient) GetFilePath(ctx context.Context, ratingKey string) (string, error) {
	url := fmt.Sprintf("%s/library/metadata/%s", c.serverURL, ratingKey)

	req, err := http.NewRequestWithContext(ctx, "GET", url, nil)
	if err != nil {
		return "", fmt.Errorf("failed to create request: %w", err)
	}

	req.Header.Set("X-Plex-Token", c.token)
	req.Header.Set("Accept", "application/xml")

	c.log.WithFields(logrus.Fields{
		"rating_key": ratingKey,
		"url":        url,
	}).Debug("Fetching Plex file path")

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return "", fmt.Errorf("request failed: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return "", fmt.Errorf("plex API returned %d: %s", resp.StatusCode, string(body))
	}

	// Parse XML response
	var container PlexMediaContainer
	if err := xml.NewDecoder(resp.Body).Decode(&container); err != nil {
		return "", fmt.Errorf("failed to parse XML response: %w", err)
	}

	// Extract file path from nested structure
	if len(container.Videos) == 0 {
		return "", fmt.Errorf("no video found in response")
	}

	video := container.Videos[0]
	if len(video.Media) == 0 {
		return "", fmt.Errorf("no media found in video")
	}

	media := video.Media[0]
	if len(media.Parts) == 0 {
		return "", fmt.Errorf("no parts found in media")
	}

	filePath := media.Parts[0].File
	if filePath == "" {
		return "", fmt.Errorf("file path is empty")
	}

	c.log.WithFields(logrus.Fields{
		"rating_key": ratingKey,
		"file_path":  filePath,
	}).Info("Retrieved Plex file path")

	return filePath, nil
}

// RefreshMetadata triggers Plex to rescan metadata for an item
func (c *PlexClient) RefreshMetadata(ctx context.Context, ratingKey string) error {
	url := fmt.Sprintf("%s/library/metadata/%s/refresh", c.serverURL, ratingKey)

	req, err := http.NewRequestWithContext(ctx, "PUT", url, nil)
	if err != nil {
		return fmt.Errorf("failed to create request: %w", err)
	}

	req.Header.Set("X-Plex-Token", c.token)

	c.log.WithFields(logrus.Fields{
		"rating_key": ratingKey,
		"url":        url,
	}).Debug("Refreshing Plex metadata")

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return fmt.Errorf("request failed: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return fmt.Errorf("plex API returned %d: %s", resp.StatusCode, string(body))
	}

	c.log.WithField("rating_key", ratingKey).Info("Plex metadata refresh initiated")

	return nil
}

// PlexMediaContainer represents Plex XML response structure
type PlexMediaContainer struct {
	XMLName xml.Name    `xml:"MediaContainer"`
	Videos  []PlexVideo `xml:"Video"`
}

// PlexVideo represents a video element in Plex XML response
type PlexVideo struct {
	XMLName xml.Name    `xml:"Video"`
	Media   []PlexMedia `xml:"Media"`
}

// PlexMedia represents a media element in Plex XML response
type PlexMedia struct {
	XMLName xml.Name   `xml:"Media"`
	Parts   []PlexPart `xml:"Part"`
}

// PlexPart represents a part element with file path in Plex XML response
type PlexPart struct {
	XMLName xml.Name `xml:"Part"`
	File    string   `xml:"file,attr"`
}
