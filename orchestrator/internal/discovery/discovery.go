package discovery

import (
	"context"
	"time"
)

// WorkerDiscovery finds available workers
type WorkerDiscovery interface {
	// GetWorkers returns all healthy workers
	GetWorkers(ctx context.Context) ([]Worker, error)

	// Watch for worker changes (add/remove)
	// Returns channel that emits WorkerEvent on changes
	Watch(ctx context.Context) (<-chan WorkerEvent, error)
}

// Worker represents a transcription worker
type Worker struct {
	ID       string    // Unique identifier
	Address  string    // gRPC address (host:port)
	Healthy  bool      // Health check status
	Active   int32     // Active jobs
	LastSeen time.Time // Last health check
}

// WorkerEvent represents a change in worker availability
type WorkerEvent struct {
	Type   EventType
	Worker Worker
}

// EventType represents type of worker change
type EventType string

const (
	EventTypeAdded   EventType = "added"
	EventTypeRemoved EventType = "removed"
	EventTypeUpdated EventType = "updated"
)
