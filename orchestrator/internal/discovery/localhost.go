package discovery

import (
	"context"
	"fmt"
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
	conn, err := grpc.DialContext(ctx, d.address,
		grpc.WithTransportCredentials(insecure.NewCredentials()),
		grpc.WithBlock(),
	)
	if err != nil {
		return nil, fmt.Errorf("failed to connect to localhost worker: %w", err)
	}
	defer conn.Close()

	// TODO: Implement health check using protobuf-generated client
	// For now, return mock worker for compilation
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
	}).Debug("Localhost worker discovered")

	return []Worker{worker}, nil
}

// Watch returns empty channel (no dynamic discovery for localhost)
func (d *LocalhostDiscovery) Watch(ctx context.Context) (<-chan WorkerEvent, error) {
	ch := make(chan WorkerEvent)
	close(ch) // No events for static localhost
	return ch, nil
}
