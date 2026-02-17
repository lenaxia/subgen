package integration

import (
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"sync"
)

// MockMediaServer simulates Plex/Jellyfin API responses for testing
type MockMediaServer struct {
	server     *httptest.Server
	mu         sync.Mutex
	responses  map[string]interface{} // Key: endpoint, Value: response data or error code
	callCount  map[string]int
	shouldFail map[string]int // Key: endpoint, Value: HTTP status code for failure
}

// NewMockMediaServer creates a new mock media server
func NewMockMediaServer() *MockMediaServer {
	mock := &MockMediaServer{
		responses:  make(map[string]interface{}),
		callCount:  make(map[string]int),
		shouldFail: make(map[string]int),
	}

	mux := http.NewServeMux()

	// Plex endpoints
	mux.HandleFunc("/library/metadata/", mock.handlePlexMetadata)

	// Jellyfin endpoints
	mux.HandleFunc("/Items/", mock.handleJellyfinItem)
	mux.HandleFunc("/Users", mock.handleJellyfinUsers)

	mock.server = httptest.NewServer(mux)
	return mock
}

// URL returns the mock server URL
func (m *MockMediaServer) URL() string {
	return m.server.URL
}

// Close shuts down the mock server
func (m *MockMediaServer) Close() {
	m.server.Close()
}

// SetPlexMetadata sets the response for a Plex metadata request
func (m *MockMediaServer) SetPlexMetadata(ratingKey string, filePath string) {
	m.mu.Lock()
	defer m.mu.Unlock()

	endpoint := fmt.Sprintf("/library/metadata/%s", ratingKey)
	m.responses[endpoint] = map[string]interface{}{
		"MediaContainer": map[string]interface{}{
			"Metadata": []map[string]interface{}{
				{
					"Media": []map[string]interface{}{
						{
							"Part": []map[string]interface{}{
								{
									"file": filePath,
								},
							},
						},
					},
				},
			},
		},
	}
}

// SetJellyfinItem sets the response for a Jellyfin item request
func (m *MockMediaServer) SetJellyfinItem(itemID string, filePath string) {
	m.mu.Lock()
	defer m.mu.Unlock()

	endpoint := fmt.Sprintf("/Items/%s", itemID)
	m.responses[endpoint] = map[string]interface{}{
		"Path": filePath,
		"MediaSources": []map[string]interface{}{
			{
				"Path": filePath,
			},
		},
	}
}

// SetJellyfinUsers sets the admin users response
func (m *MockMediaServer) SetJellyfinUsers(adminUserID string, adminUserName string) {
	m.mu.Lock()
	defer m.mu.Unlock()

	m.responses["/Users"] = []map[string]interface{}{
		{
			"Id":     adminUserID,
			"Name":   adminUserName,
			"Policy": map[string]interface{}{},
		},
	}
}

// GetCallCount returns the number of times an endpoint was called
func (m *MockMediaServer) GetCallCount(endpoint string) int {
	m.mu.Lock()
	defer m.mu.Unlock()
	return m.callCount[endpoint]
}

// SimulateFailure makes the mock server return errors for an endpoint
func (m *MockMediaServer) SimulateFailure(endpoint string, statusCode int) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.shouldFail[endpoint] = statusCode
}

// ResetCallCount resets call counters
func (m *MockMediaServer) ResetCallCount() {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.callCount = make(map[string]int)
}

// handlePlexMetadata handles Plex metadata requests
func (m *MockMediaServer) handlePlexMetadata(w http.ResponseWriter, r *http.Request) {
	m.mu.Lock()
	endpoint := r.URL.Path
	m.callCount[endpoint]++

	// Check if should fail
	if statusCode, shouldFail := m.shouldFail[endpoint]; shouldFail {
		m.mu.Unlock()
		http.Error(w, "Simulated failure", statusCode)
		return
	}

	response, ok := m.responses[endpoint]
	m.mu.Unlock()

	if !ok {
		http.Error(w, "Not found", http.StatusNotFound)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(response)
}

// handleJellyfinItem handles Jellyfin item requests
func (m *MockMediaServer) handleJellyfinItem(w http.ResponseWriter, r *http.Request) {
	m.mu.Lock()
	endpoint := r.URL.Path
	m.callCount[endpoint]++

	// Check if should fail
	if statusCode, shouldFail := m.shouldFail[endpoint]; shouldFail {
		m.mu.Unlock()
		http.Error(w, "Simulated failure", statusCode)
		return
	}

	response, ok := m.responses[endpoint]
	m.mu.Unlock()

	if !ok {
		http.Error(w, "Not found", http.StatusNotFound)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(response)
}

// handleJellyfinUsers handles Jellyfin users requests
func (m *MockMediaServer) handleJellyfinUsers(w http.ResponseWriter, r *http.Request) {
	m.mu.Lock()
	endpoint := "/Users"
	m.callCount[endpoint]++

	response, ok := m.responses[endpoint]
	m.mu.Unlock()

	if !ok {
		http.Error(w, "No users configured", http.StatusNotFound)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(response)
}
