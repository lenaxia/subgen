package grpc_client

import (
	"context"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"google.golang.org/grpc/connectivity"
)

func TestConnectionPool_NewConnection(t *testing.T) {
	pool := NewConnectionPool(10)

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	// Note: This will fail to connect since localhost:50051 isn't running
	// But we can still test the pool logic
	conn, err := pool.Get(ctx, "localhost:50051")

	// Should attempt connection (will fail in background)
	require.NoError(t, err)
	assert.NotNil(t, conn)
	assert.Equal(t, 1, pool.Size())

	pool.Close()
}

func TestConnectionPool_ReuseConnection(t *testing.T) {
	pool := NewConnectionPool(10)
	defer pool.Close()

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	// Get first connection
	conn1, err := pool.Get(ctx, "localhost:50051")
	require.NoError(t, err)
	assert.Equal(t, 1, pool.Size())

	// Get same address again - should reuse
	conn2, err := pool.Get(ctx, "localhost:50051")
	require.NoError(t, err)
	assert.Equal(t, conn1, conn2, "should reuse same connection")
	assert.Equal(t, 1, pool.Size(), "should still have only 1 connection")
}

func TestConnectionPool_MultipleWorkers(t *testing.T) {
	pool := NewConnectionPool(10)
	defer pool.Close()

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	// Connect to multiple workers
	conn1, err := pool.Get(ctx, "worker1:50051")
	require.NoError(t, err)
	assert.NotNil(t, conn1)

	conn2, err := pool.Get(ctx, "worker2:50051")
	require.NoError(t, err)
	assert.NotNil(t, conn2)

	conn3, err := pool.Get(ctx, "worker3:50051")
	require.NoError(t, err)
	assert.NotNil(t, conn3)

	assert.Equal(t, 3, pool.Size())
	// Use pointer comparison instead of deep equality to avoid races with gRPC internals
	assert.True(t, conn1 != conn2, "conn1 and conn2 should be different pointers")
	assert.True(t, conn1 != conn3, "conn1 and conn3 should be different pointers")
	assert.True(t, conn2 != conn3, "conn2 and conn3 should be different pointers")
}

func TestConnectionPool_CloseAll(t *testing.T) {
	pool := NewConnectionPool(10)

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	// Create some connections
	pool.Get(ctx, "worker1:50051")
	pool.Get(ctx, "worker2:50051")

	assert.Equal(t, 2, pool.Size())

	// Close all
	err := pool.Close()
	require.NoError(t, err)
	assert.Equal(t, 0, pool.Size())
}

func TestConnectionPool_RecreateClosedConnection(t *testing.T) {
	pool := NewConnectionPool(10)
	defer pool.Close()

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	// Get first connection
	conn1, err := pool.Get(ctx, "localhost:50051")
	require.NoError(t, err)

	// Close the connection
	conn1.Close()

	// Verify it's shutdown
	state := conn1.GetState()
	assert.Equal(t, connectivity.Shutdown, state)

	// Get again - should create new connection
	conn2, err := pool.Get(ctx, "localhost:50051")
	require.NoError(t, err)
	// Use pointer comparison instead of deep equality to avoid races with gRPC internals
	assert.True(t, conn1 != conn2, "should create new connection after shutdown")
}
