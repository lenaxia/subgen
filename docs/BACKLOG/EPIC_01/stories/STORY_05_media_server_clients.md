# STORY_05: Media Server API Clients (Plex & Jellyfin)

**Status:** Not Started  
**Effort:** 8-10 hours  
**Epic:** EPIC_01 (Go Orchestrator Core)  
**Created:** 2026-02-15

---

## User Story

**As a** developer  
**I want** HTTP clients for Plex and Jellyfin APIs  
**So that** the orchestrator can fetch file paths and refresh metadata after transcription

---

## Acceptance Criteria

- [ ] Plex client with 3 methods: GetFilePath, RefreshMetadata, GetNextEpisode
- [ ] Jellyfin client with 3 methods: GetFilePath, RefreshMetadata, GetAdminUser
- [ ] All methods properly authenticated with tokens
- [ ] XML parsing for Plex responses
- [ ] JSON parsing for Jellyfin responses
- [ ] Error handling for network failures and API errors
- [ ] Configurable timeout (30s default)
- [ ] HTTP client reuse (connection pooling)
- [ ] 10+ test cases with mock HTTP responses
- [ ] Integration with config from STORY_02
- [ ] Work log created

---

## Integration Points

### Legacy Plex Client (subgen.py:1891-1944)

**Location:** `/home/mikekao/personal/subgen/subgen.py:1891-1944`

**get_plex_file_name Implementation:**
```python
def get_plex_file_name(itemid: str, server_ip: str, plex_token: str) -> str:
    """Gets the full path to a file from the Plex server."""
    url = f"{server_ip}/library/metadata/{itemid}"
    
    headers = {
        "X-Plex-Token": plex_token,
    }
    
    response = requests.get(url, headers=headers)
    
    if response.status_code == 200:
        root = ET.fromstring(response.content)
        fullpath = root.find(".//Part").attrib['file']
        return fullpath
    else:
        raise Exception(f"Error: {response.status_code}")
```

**Key Details:**
- **Endpoint:** `GET /library/metadata/{itemid}`
- **Headers:** `X-Plex-Token: {token}`
- **Response Format:** XML
- **XPath:** `//Part/@file` to extract file path
- **Example Response:**
```xml
<MediaContainer>
  <Video>
    <Media>
      <Part file="/media/TV/Show/S01E01.mkv"/>
    </Media>
  </Video>
</MediaContainer>
```

**refresh_plex_metadata Implementation (subgen.py:1916-1944):**
```python
def refresh_plex_metadata(itemid: str, server_ip: str, plex_token: str) -> None:
    url = f"{server_ip}/library/metadata/{itemid}/refresh"
    headers = {"X-Plex-Token": plex_token}
    response = requests.put(url, headers=headers)
    
    if response.status_code == 200:
        logging.info("Metadata refresh initiated successfully.")
    else:
        raise Exception(f"Error refreshing metadata: {response.status_code}")
```

**Key Details:**
- **Endpoint:** `PUT /library/metadata/{itemid}/refresh`
- **Method:** PUT (not POST!)
- **Success Code:** 200
- **Side Effect:** Plex rescans file for new subtitles

---

### Legacy Jellyfin Client (subgen.py:1983-2014)

**Location:** `/home/mikekao/personal/subgen/subgen.py:1983-2014`

**get_jellyfin_file_name Implementation:**
```python
def get_jellyfin_file_name(item_id: str, jellyfin_url: str, jellyfin_token: str) -> str:
    headers = {
        "Authorization": f"MediaBrowser Token={jellyfin_token}",
    }
    
    # Get admin user ID first
    users = json.loads(requests.get(f"{jellyfin_url}/Users", headers=headers).content)
    jellyfin_admin = get_jellyfin_admin(users)
    
    response = requests.get(f"{jellyfin_url}/Users/{jellyfin_admin}/Items/{item_id}", headers=headers)
    
    if response.status_code == 200:
        file_name = json.loads(response.content)['Path']
        return file_name
    else:
        raise Exception(f"Error: {response.status_code}")

def get_jellyfin_admin(users):
    for user in users:
        if user["Policy"]["IsAdministrator"]:
            return user["Id"]
    raise Exception("Unable to find administrator user in Jellyfin")
```

**Key Details:**
- **Auth Header:** `Authorization: MediaBrowser Token={token}` (NOT Bearer!)
- **Endpoint:** `GET /Users/{adminUserId}/Items/{itemId}`
- **Response Format:** JSON
- **JSON Path:** `.Path` to extract file path
- **Admin User Required:** Must fetch admin user ID first via `/Users`
- **Example Response:**
```json
{
  "Path": "/media/TV/Show/S01E01.mkv",
  "Name": "Episode 1",
  "Type": "Episode"
}
```

**refresh_jellyfin_metadata Implementation (subgen.py:1946-1980):**
```python
def refresh_jellyfin_metadata(itemid: str, server_ip: str, jellyfin_token: str) -> None:
    url = f"{server_ip}/Items/{itemid}/Refresh?MetadataRefreshMode=FullRefresh"
    headers = {"Authorization": f"MediaBrowser Token={jellyfin_token}"}
    
    # Get admin user (used for some API calls)
    users = json.loads(requests.get(f"{server_ip}/Users", headers=headers).content)
    jellyfin_admin = get_jellyfin_admin(users)
    
    response = requests.post(url, headers=headers)
    
    if response.status_code == 204:
        logging.info("Metadata refresh queued successfully.")
    else:
        raise Exception(f"Error refreshing metadata: {response.status_code}")
```

**Key Details:**
- **Endpoint:** `POST /Items/{itemId}/Refresh?MetadataRefreshMode=FullRefresh`
- **Method:** POST
- **Success Code:** 204 (No Content, not 200!)
- **Query Param:** `MetadataRefreshMode=FullRefresh`

---

## Technical Design

### File Structure

```
internal/mediaserver/
├── client.go           # Common HTTP client interface
├── plex.go             # Plex client implementation
├── plex_test.go        # Plex client tests
├── jellyfin.go         # Jellyfin client implementation
├── jellyfin_test.go    # Jellyfin client tests
└── errors.go           # Custom error types
```

---

### Common Interface (client.go)

**File:** `internal/mediaserver/client.go`

```go
package mediaserver

import (
	"context"
	"time"
)

// MediaServerClient interface for media server operations
type MediaServerClient interface {
	// GetFilePath retrieves the file system path for a media item
	GetFilePath(ctx context.Context, itemID string) (string, error)
	
	// RefreshMetadata triggers a metadata refresh for an item
	// (causes server to rescan for new subtitles)
	RefreshMetadata(ctx context.Context, itemID string) error
}

// ClientConfig holds common HTTP client configuration
type ClientConfig struct {
	Timeout        time.Duration // HTTP request timeout
	MaxIdleConns   int           // Connection pool size
	IdleConnTimeout time.Duration // How long idle connections stay open
}

// DefaultClientConfig returns sensible defaults
func DefaultClientConfig() ClientConfig {
	return ClientConfig{
		Timeout:        30 * time.Second,
		MaxIdleConns:   10,
		IdleConnTimeout: 90 * time.Second,
	}
}
```

---

### Plex Client (plex.go)

**File:** `internal/mediaserver/plex.go`

```go
package mediaserver

import (
	"context"
	"encoding/xml"
	"fmt"
	"io"
	"net/http"
	"time"
	
	"github.com/sirupsen/logrus"
)

// PlexClient implements MediaServerClient for Plex
type PlexClient struct {
	serverURL string
	token     string
	httpClient *http.Client
	log       *logrus.Logger
}

// NewPlexClient creates a new Plex API client
func NewPlexClient(serverURL, token string, config ClientConfig, log *logrus.Logger) *PlexClient {
	return &PlexClient{
		serverURL: serverURL,
		token:     token,
		httpClient: &http.Client{
			Timeout: config.Timeout,
			Transport: &http.Transport{
				MaxIdleConns:        config.MaxIdleConns,
				IdleConnTimeout:     config.IdleConnTimeout,
				DisableKeepAlives:   false,
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

type PlexVideo struct {
	XMLName xml.Name    `xml:"Video"`
	Media   []PlexMedia `xml:"Media"`
}

type PlexMedia struct {
	XMLName xml.Name   `xml:"Media"`
	Parts   []PlexPart `xml:"Part"`
}

type PlexPart struct {
	XMLName xml.Name `xml:"Part"`
	File    string   `xml:"file,attr"`
}
```

---

### Jellyfin Client (jellyfin.go)

**File:** `internal/mediaserver/jellyfin.go`

```go
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
	serverURL string
	token     string
	httpClient *http.Client
	log       *logrus.Logger
	
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
				MaxIdleConns:        config.MaxIdleConns,
				IdleConnTimeout:     config.IdleConnTimeout,
				DisableKeepAlives:   false,
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
```

---

## Test Cases (10+)

### Plex Tests

**File:** `internal/mediaserver/plex_test.go`

```go
package mediaserver

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"
	
	"github.com/sirupsen/logrus"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestPlexGetFilePath_Success(t *testing.T) {
	// Mock Plex server
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, "/library/metadata/12345", r.URL.Path)
		assert.Equal(t, "test-token", r.Header.Get("X-Plex-Token"))
		
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
	
	client := NewPlexClient(server.URL, "test-token", DefaultClientConfig(), logrus.New())
	filePath, err := client.GetFilePath(context.Background(), "12345")
	
	require.NoError(t, err)
	assert.Equal(t, "/media/movie.mkv", filePath)
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

func TestPlexRefreshMetadata_Success(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, "PUT", r.Method)
		assert.Equal(t, "/library/metadata/12345/refresh", r.URL.Path)
		w.WriteHeader(http.StatusOK)
	}))
	defer server.Close()
	
	client := NewPlexClient(server.URL, "test-token", DefaultClientConfig(), logrus.New())
	err := client.RefreshMetadata(context.Background(), "12345")
	
	require.NoError(t, err)
}

// Add 7+ more tests for edge cases...
```

### Jellyfin Tests

**File:** `internal/mediaserver/jellyfin_test.go`

Similar structure testing:
1. GetFilePath success
2. GetFilePath with admin user caching
3. RefreshMetadata success (204 response)
4. Auth header format validation
5. Admin user not found error
6. Network errors
7. JSON parsing errors

---

## Implementation Steps

### Step 1: Create Package Structure (15 min)
```bash
cd /home/mikekao/personal/subgen/orchestrator
mkdir -p internal/mediaserver
touch internal/mediaserver/client.go
touch internal/mediaserver/plex.go
touch internal/mediaserver/jellyfin.go
touch internal/mediaserver/plex_test.go
touch internal/mediaserver/jellyfin_test.go
```

### Step 2: Implement Plex Client (2 hours)
1. Define XML structs for response parsing
2. Implement GetFilePath with XML parsing
3. Implement RefreshMetadata with PUT request
4. Add logging and error handling
5. Test manually with curl against real Plex server

### Step 3: Implement Jellyfin Client (2 hours)
1. Define JSON structs for response parsing
2. Implement getAdminUserID with caching
3. Implement GetFilePath
4. Implement RefreshMetadata
5. Test manually against real Jellyfin server

### Step 4: Write Tests (3 hours)
1. Create httptest mock servers
2. Test success cases
3. Test error cases (404, 500, network errors)
4. Test auth header formats
5. Test XML/JSON parsing errors
6. Run: `go test ./internal/mediaserver -v`

### Step 5: Integration with Queue (1 hour)

Update queue worker to call media server clients after transcription:

```go
// After successful transcription
if task.PlexItemID != "" {
	plexClient := mediaserver.NewPlexClient(cfg.Plex.Server, cfg.Plex.Token, ...)
	err := plexClient.RefreshMetadata(ctx, task.PlexItemID)
	if err != nil {
		log.Errorf("Failed to refresh Plex metadata: %v", err)
	}
}
```

---

## Dependencies

**Requires:**
- STORY_01 (Project Setup) ✅
- STORY_02 (Configuration) ✅

**Blocks:**
- STORY_07 (gRPC Client) - needs media server metadata

---

## Definition of Done

- [ ] All 10+ tests passing
- [ ] Plex GetFilePath works
- [ ] Plex RefreshMetadata works
- [ ] Jellyfin GetFilePath works
- [ ] Jellyfin RefreshMetadata works
- [ ] Admin user ID caching works
- [ ] HTTP client connection pooling
- [ ] Error handling for all API failures
- [ ] Manual testing with real servers
- [ ] Code passes golangci-lint
- [ ] Work log created
- [ ] Coverage > 75% for mediaserver package

---

## Notes

### Authentication Differences

**Plex:**
- Header: `X-Plex-Token: {token}`
- Response: XML
- Simple, no user context needed

**Jellyfin:**
- Header: `Authorization: MediaBrowser Token={token}` (NOT Bearer!)
- Response: JSON
- Requires admin user ID for some endpoints

### Why Cache Admin User ID?

Jellyfin requires admin user ID for many operations. Fetching it every time adds latency (extra HTTP request). Cache it after first fetch to improve performance.

### Connection Pooling

Using `http.Client` with custom `Transport` enables connection reuse. This reduces latency for repeated API calls (important when refreshing metadata for many episodes).

---

**Owner:** TBD  
**Created:** 2026-02-15  
**Last Updated:** 2026-02-15
