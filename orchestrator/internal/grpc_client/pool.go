package grpc_client

import (
	"context"
	"fmt"
	"sync"
	"time"

	"google.golang.org/grpc"
	"google.golang.org/grpc/connectivity"
	"google.golang.org/grpc/credentials/insecure"
	"google.golang.org/grpc/keepalive"
)

// ConnectionPool manages gRPC connections to workers
type ConnectionPool struct {
	mu    sync.RWMutex
	conns map[string]*grpc.ClientConn

	maxConns int
}

// NewConnectionPool creates a connection pool
func NewConnectionPool(maxConns int) *ConnectionPool {
	return &ConnectionPool{
		conns:    make(map[string]*grpc.ClientConn),
		maxConns: maxConns,
	}
}

// Get retrieves or creates a connection to worker
func (p *ConnectionPool) Get(ctx context.Context, addr string) (*grpc.ClientConn, error) {
	// Try to get existing connection
	p.mu.RLock()
	conn, exists := p.conns[addr]
	p.mu.RUnlock()

	if exists && conn.GetState() != connectivity.Shutdown {
		return conn, nil
	}

	// Create new connection
	p.mu.Lock()
	defer p.mu.Unlock()

	// Double-check after acquiring write lock
	conn, exists = p.conns[addr]
	if exists && conn.GetState() != connectivity.Shutdown {
		return conn, nil
	}

	// Dial new connection
	conn, err := grpc.DialContext(ctx, addr,
		grpc.WithTransportCredentials(insecure.NewCredentials()),
		grpc.WithKeepaliveParams(keepalive.ClientParameters{
			Time:                10 * time.Second,
			Timeout:             5 * time.Second,
			PermitWithoutStream: true,
		}),
	)
	if err != nil {
		return nil, fmt.Errorf("failed to dial worker: %w", err)
	}

	p.conns[addr] = conn

	return conn, nil
}

// Put returns a connection to the pool (no-op, keeps conn alive)
func (p *ConnectionPool) Put(addr string, conn *grpc.ClientConn) {
	// Connection remains in pool for reuse
}

// Close closes all connections
func (p *ConnectionPool) Close() error {
	p.mu.Lock()
	defer p.mu.Unlock()

	for addr, conn := range p.conns {
		if err := conn.Close(); err != nil {
			return fmt.Errorf("failed to close connection to %s: %w", addr, err)
		}
		delete(p.conns, addr)
	}

	return nil
}

// Size returns number of active connections
func (p *ConnectionPool) Size() int {
	p.mu.RLock()
	defer p.mu.RUnlock()

	return len(p.conns)
}
