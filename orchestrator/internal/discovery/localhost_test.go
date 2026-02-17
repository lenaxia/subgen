package discovery_test

import (
	"context"
	"io"
	"testing"
	"time"

	"github.com/mccloud/subgen/orchestrator/internal/discovery"
	"github.com/sirupsen/logrus"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestLocalhostDiscovery_GetWorkers_Success tests successful worker discovery
func TestLocalhostDiscovery_GetWorkers_Success(t *testing.T) {
	// This test will require a mock gRPC server
	// For now, we'll skip it and implement with mock
	t.Skip("Requires mock gRPC server - will implement with mocks")
}

// TestLocalhostDiscovery_GetWorkers_ConnectionFailure tests connection failure
func TestLocalhostDiscovery_GetWorkers_ConnectionFailure(t *testing.T) {
	log := logrus.New()
	log.SetOutput(io.Discard)

	// Create discovery with invalid address
	disc := discovery.NewLocalhostDiscovery("invalid:99999", log)

	ctx, cancel := context.WithTimeout(context.Background(), 100*time.Millisecond)
	defer cancel()

	workers, err := disc.GetWorkers(ctx)

	// Localhost discovery returns unhealthy worker when connection fails (no error)
	// This allows the pool to track the worker and retry later
	assert.NoError(t, err, "GetWorkers should not return error for connection failures")
	assert.NotNil(t, workers, "Should return unhealthy worker")
	assert.Equal(t, 1, len(workers), "Should return one worker")
	assert.False(t, workers[0].Healthy, "Worker should be marked as unhealthy")
	assert.Equal(t, "invalid:99999", workers[0].Address)
}

// TestLocalhostDiscovery_Watch_NoEvents tests that localhost watch returns closed channel
func TestLocalhostDiscovery_Watch_NoEvents(t *testing.T) {
	log := logrus.New()
	log.SetOutput(io.Discard)

	disc := discovery.NewLocalhostDiscovery("localhost:50051", log)

	ctx := context.Background()
	eventCh, err := disc.Watch(ctx)

	require.NoError(t, err)
	require.NotNil(t, eventCh)

	// Channel should be closed (no events for localhost)
	select {
	case _, ok := <-eventCh:
		assert.False(t, ok, "Expected channel to be closed")
	case <-time.After(100 * time.Millisecond):
		t.Fatal("Channel should be immediately closed")
	}
}

// TestLocalhostDiscovery_Creation tests discovery creation
func TestLocalhostDiscovery_Creation(t *testing.T) {
	log := logrus.New()
	disc := discovery.NewLocalhostDiscovery("localhost:50051", log)

	assert.NotNil(t, disc)
}
