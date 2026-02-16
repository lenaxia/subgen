package discovery_test

import (
	"testing"
	"time"

	"github.com/mccloud/subgen/orchestrator/internal/discovery"
	"github.com/stretchr/testify/assert"
)

// TestWorkerStruct tests Worker struct creation
func TestWorkerStruct(t *testing.T) {
	now := time.Now()
	worker := discovery.Worker{
		ID:       "worker-1",
		Address:  "10.244.1.5:50051",
		Healthy:  true,
		Active:   3,
		LastSeen: now,
	}

	assert.Equal(t, "worker-1", worker.ID)
	assert.Equal(t, "10.244.1.5:50051", worker.Address)
	assert.True(t, worker.Healthy)
	assert.Equal(t, int32(3), worker.Active)
	assert.Equal(t, now, worker.LastSeen)
}

// TestWorkerEvent tests WorkerEvent struct
func TestWorkerEvent(t *testing.T) {
	worker := discovery.Worker{
		ID:      "worker-1",
		Address: "localhost:50051",
		Healthy: true,
	}

	event := discovery.WorkerEvent{
		Type:   discovery.EventTypeAdded,
		Worker: worker,
	}

	assert.Equal(t, discovery.EventTypeAdded, event.Type)
	assert.Equal(t, "worker-1", event.Worker.ID)
}

// TestEventTypes tests all event type constants
func TestEventTypes(t *testing.T) {
	assert.Equal(t, discovery.EventType("added"), discovery.EventTypeAdded)
	assert.Equal(t, discovery.EventType("removed"), discovery.EventTypeRemoved)
	assert.Equal(t, discovery.EventType("updated"), discovery.EventTypeUpdated)
}
