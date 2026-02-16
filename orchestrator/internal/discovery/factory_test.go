package discovery_test

import (
	"testing"

	"github.com/mccloud/subgen/orchestrator/internal/config"
	"github.com/mccloud/subgen/orchestrator/internal/discovery"
	"github.com/sirupsen/logrus"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestNewDiscovery_Localhost tests factory creates localhost discovery
func TestNewDiscovery_Localhost(t *testing.T) {
	cfg := &config.Config{
		Worker: config.WorkerConfig{
			Discovery: "localhost",
			Address:   "localhost:50051",
		},
	}

	log := logrus.New()
	disc, err := discovery.NewDiscovery(cfg, log)

	require.NoError(t, err)
	assert.NotNil(t, disc)
}

// TestNewDiscovery_Kubernetes tests factory creates kubernetes discovery
func TestNewDiscovery_Kubernetes(t *testing.T) {
	t.Skip("Kubernetes discovery requires in-cluster config - skipping for unit tests")
}

// TestNewDiscovery_InvalidMode tests factory rejects invalid discovery mode
func TestNewDiscovery_InvalidMode(t *testing.T) {
	cfg := &config.Config{
		Worker: config.WorkerConfig{
			Discovery: "invalid-mode",
			Address:   "localhost:50051",
		},
	}

	log := logrus.New()
	disc, err := discovery.NewDiscovery(cfg, log)

	assert.Error(t, err)
	assert.Nil(t, disc)
	assert.Contains(t, err.Error(), "unknown worker discovery")
}
