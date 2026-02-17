package discovery

import (
	"context"
	"time"

	"github.com/sirupsen/logrus"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"
)

// LocalhostDiscovery implements WorkerDiscovery for single local worker
type LocalhostDiscovery struct {
	address string
	log     *logrus.Logger
}

// NewLocalhostDiscovery creates a localhost worker discovery
func NewLocalhostDiscovery(address string, log *logrus.Logger) *LocalhostDiscovery {
	return &LocalhostDiscovery{
		address: address,
		log:     log,
	}
}

// GetWorkers returns the single localhost worker (if healthy)
func (d *LocalhostDiscovery) GetWorkers(ctx context.Context) ([]Worker, error) {
	// Create a timeout context for connection attempt (5 seconds)
	connCtx, cancel := context.WithTimeout(ctx, 5*time.Second)
	defer cancel()

	// Try to connect to worker with timeout
	conn, err := grpc.DialContext(connCtx, d.address,
		grpc.WithTransportCredentials(insecure.NewCredentials()),
		grpc.WithBlock(),
	)
	if err != nil {
		// Log warning but don't fail - worker might not be ready yet
		d.log.WithError(err).WithField("address", d.address).Warn("Failed to connect to localhost worker, will retry later")

		// Return unhealthy worker so pool can track it
		worker := Worker{
			ID:       "worker-local",
			Address:  d.address,
			Healthy:  false, // Mark as unhealthy
			Active:   0,
			LastSeen: time.Now(),
		}
		return []Worker{worker}, nil
	}
	defer conn.Close()

	// TODO: Implement health check using protobuf-generated client
	// For now, assume healthy if connection succeeded
	worker := Worker{
		ID:       "worker-local",
		Address:  d.address,
		Healthy:  true,
		Active:   0,
		LastSeen: time.Now(),
	}

	d.log.WithFields(logrus.Fields{
		"address": worker.Address,
		"healthy": worker.Healthy,
		"active":  worker.Active,
	}).Info("Localhost worker discovered and healthy")

	return []Worker{worker}, nil
}

// Watch returns empty channel (no dynamic discovery for localhost)
func (d *LocalhostDiscovery) Watch(ctx context.Context) (<-chan WorkerEvent, error) {
	ch := make(chan WorkerEvent)
	close(ch) // No events for static localhost
	return ch, nil
}
