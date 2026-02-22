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

// MockDiscovery implements WorkerDiscovery for testing
type MockDiscovery struct {
	workers []discovery.Worker
	err     error
}

func (m *MockDiscovery) GetWorkers(ctx context.Context) ([]discovery.Worker, error) {
	if m.err != nil {
		return nil, m.err
	}
	return m.workers, nil
}

func (m *MockDiscovery) Watch(ctx context.Context) (<-chan discovery.WorkerEvent, error) {
	ch := make(chan discovery.WorkerEvent)
	close(ch)
	return ch, nil
}

// TestPool_SelectWorker_RoundRobin tests round-robin worker selection
func TestPool_SelectWorker_RoundRobin(t *testing.T) {
	log := logrus.New()
	log.SetOutput(io.Discard)

	workers := []discovery.Worker{
		{ID: "worker-1", Address: "10.0.0.1:50051", Healthy: true, Active: 0},
		{ID: "worker-2", Address: "10.0.0.2:50051", Healthy: true, Active: 0},
		{ID: "worker-3", Address: "10.0.0.3:50051", Healthy: true, Active: 0},
	}

	mockDisc := &MockDiscovery{workers: workers}
	pool := discovery.NewPool(mockDisc, discovery.RoundRobin, log)

	ctx := context.Background()
	err := pool.Start(ctx)
	require.NoError(t, err)

	// Select workers in round-robin fashion
	w1, err := pool.SelectWorker()
	require.NoError(t, err)
	assert.Equal(t, "worker-1", w1.ID)

	w2, err := pool.SelectWorker()
	require.NoError(t, err)
	assert.Equal(t, "worker-2", w2.ID)

	w3, err := pool.SelectWorker()
	require.NoError(t, err)
	assert.Equal(t, "worker-3", w3.ID)

	// Should wrap around
	w4, err := pool.SelectWorker()
	require.NoError(t, err)
	assert.Equal(t, "worker-1", w4.ID)
}

// TestPool_SelectWorker_LeastLoaded tests least-loaded worker selection
func TestPool_SelectWorker_LeastLoaded(t *testing.T) {
	log := logrus.New()
	log.SetOutput(io.Discard)

	workers := []discovery.Worker{
		{ID: "worker-1", Address: "10.0.0.1:50051", Healthy: true, Active: 5},
		{ID: "worker-2", Address: "10.0.0.2:50051", Healthy: true, Active: 2}, // Least loaded
		{ID: "worker-3", Address: "10.0.0.3:50051", Healthy: true, Active: 10},
	}

	mockDisc := &MockDiscovery{workers: workers}
	pool := discovery.NewPool(mockDisc, discovery.LeastLoaded, log)

	ctx := context.Background()
	err := pool.Start(ctx)
	require.NoError(t, err)

	// Should select worker-2 (least loaded) and increment its Active count.
	worker, err := pool.SelectWorker()
	require.NoError(t, err)
	assert.Equal(t, "worker-2", worker.ID)
	assert.Equal(t, int32(3), worker.Active) // incremented from 2 → 3 after selection
}

// TestPool_SelectWorker_NoHealthyWorkers tests error when no workers are healthy
func TestPool_SelectWorker_NoHealthyWorkers(t *testing.T) {
	log := logrus.New()
	log.SetOutput(io.Discard)

	workers := []discovery.Worker{
		{ID: "worker-1", Address: "10.0.0.1:50051", Healthy: false, Active: 0},
		{ID: "worker-2", Address: "10.0.0.2:50051", Healthy: false, Active: 0},
	}

	mockDisc := &MockDiscovery{workers: workers}
	pool := discovery.NewPool(mockDisc, discovery.RoundRobin, log)

	ctx := context.Background()
	err := pool.Start(ctx)
	require.NoError(t, err)

	worker, err := pool.SelectWorker()
	assert.Error(t, err)
	assert.Nil(t, worker)
	assert.Equal(t, discovery.ErrNoHealthyWorkers, err)
}

// TestPool_SelectWorker_SkipsUnhealthy tests that unhealthy workers are skipped
func TestPool_SelectWorker_SkipsUnhealthy(t *testing.T) {
	log := logrus.New()
	log.SetOutput(io.Discard)

	workers := []discovery.Worker{
		{ID: "worker-1", Address: "10.0.0.1:50051", Healthy: false, Active: 0},
		{ID: "worker-2", Address: "10.0.0.2:50051", Healthy: true, Active: 0},
		{ID: "worker-3", Address: "10.0.0.3:50051", Healthy: false, Active: 0},
	}

	mockDisc := &MockDiscovery{workers: workers}
	pool := discovery.NewPool(mockDisc, discovery.RoundRobin, log)

	ctx := context.Background()
	err := pool.Start(ctx)
	require.NoError(t, err)

	// Should only select worker-2 (the only healthy one)
	for i := 0; i < 5; i++ {
		worker, err := pool.SelectWorker()
		require.NoError(t, err)
		assert.Equal(t, "worker-2", worker.ID)
	}
}

// TestPool_Refresh tests worker refresh
func TestPool_Refresh(t *testing.T) {
	log := logrus.New()
	log.SetOutput(io.Discard)

	initialWorkers := []discovery.Worker{
		{ID: "worker-1", Address: "10.0.0.1:50051", Healthy: true, Active: 0},
	}

	mockDisc := &MockDiscovery{workers: initialWorkers}
	pool := discovery.NewPool(mockDisc, discovery.RoundRobin, log)

	ctx := context.Background()
	err := pool.Start(ctx)
	require.NoError(t, err)

	// Update workers in mock
	mockDisc.workers = []discovery.Worker{
		{ID: "worker-1", Address: "10.0.0.1:50051", Healthy: true, Active: 0},
		{ID: "worker-2", Address: "10.0.0.2:50051", Healthy: true, Active: 0},
	}

	// Refresh pool
	err = pool.Refresh(ctx)
	require.NoError(t, err)

	// Should now have access to both workers
	w1, _ := pool.SelectWorker()
	w2, _ := pool.SelectWorker()

	ids := []string{w1.ID, w2.ID}
	assert.Contains(t, ids, "worker-1")
	assert.Contains(t, ids, "worker-2")
}

// TestPool_ConcurrentSelection tests concurrent SelectWorker calls (thread safety)
func TestPool_ConcurrentSelection(t *testing.T) {
	log := logrus.New()
	log.SetOutput(io.Discard)

	workers := []discovery.Worker{
		{ID: "worker-1", Address: "10.0.0.1:50051", Healthy: true, Active: 0},
		{ID: "worker-2", Address: "10.0.0.2:50051", Healthy: true, Active: 0},
	}

	mockDisc := &MockDiscovery{workers: workers}
	pool := discovery.NewPool(mockDisc, discovery.RoundRobin, log)

	ctx := context.Background()
	err := pool.Start(ctx)
	require.NoError(t, err)

	// Launch 100 goroutines selecting workers concurrently
	done := make(chan bool, 100)
	for i := 0; i < 100; i++ {
		go func() {
			_, err := pool.SelectWorker()
			assert.NoError(t, err)
			done <- true
		}()
	}

	// Wait for all goroutines
	for i := 0; i < 100; i++ {
		select {
		case <-done:
			// Success
		case <-time.After(5 * time.Second):
			t.Fatal("Timeout waiting for concurrent selections")
		}
	}
}
