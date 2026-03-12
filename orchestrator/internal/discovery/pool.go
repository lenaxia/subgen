package discovery

import (
	"context"
	"errors"
	"fmt"
	"sync"
	"time"

	"github.com/sirupsen/logrus"
)

var (
	ErrNoWorkersAvailable = errors.New("no workers available")
	ErrNoHealthyWorkers   = errors.New("no healthy workers")
)

// LoadBalanceStrategy determines how to select workers
type LoadBalanceStrategy string

const (
	RoundRobin  LoadBalanceStrategy = "round_robin"
	LeastLoaded LoadBalanceStrategy = "least_loaded"
)

// Pool manages a pool of workers with health checking
type Pool struct {
	discovery WorkerDiscovery
	strategy  LoadBalanceStrategy
	log       *logrus.Logger

	mu      sync.RWMutex
	workers []Worker
	next    int // For round-robin

	// Health check interval
	healthCheckInterval time.Duration
}

// NewPool creates a worker pool
func NewPool(discovery WorkerDiscovery, strategy LoadBalanceStrategy, log *logrus.Logger) *Pool {
	return &Pool{
		discovery:           discovery,
		strategy:            strategy,
		log:                 log,
		workers:             []Worker{},
		healthCheckInterval: 30 * time.Second,
	}
}

// Start begins worker discovery and health checking
func (p *Pool) Start(ctx context.Context) error {
	// Initial discovery - don't fail if workers aren't ready yet
	if err := p.Refresh(ctx); err != nil {
		p.log.WithError(err).Warn("Initial worker discovery failed, will retry in background")
	}

	// Log worker status
	p.mu.RLock()
	healthyCount := len(p.filterHealthy())
	totalCount := len(p.workers)
	p.mu.RUnlock()

	if healthyCount == 0 {
		p.log.Warn("No healthy workers available at startup, will continue checking")
	} else {
		p.log.WithFields(logrus.Fields{
			"healthy": healthyCount,
			"total":   totalCount,
		}).Info("Worker pool started with healthy workers")
	}

	// Start health check loop
	go p.healthCheckLoop(ctx)

	// Watch for changes (if supported)
	eventCh, err := p.discovery.Watch(ctx)
	if err != nil {
		p.log.WithError(err).Warn("Worker watch not supported")
	} else {
		go p.watchLoop(ctx, eventCh)
	}

	return nil
}

// SelectWorker chooses a worker based on load balancing strategy.
// It also increments the chosen worker's Active count optimistically so
// subsequent calls see up-to-date load before the discovery refresh fires.
func (p *Pool) SelectWorker() (*Worker, error) {
	p.mu.Lock()
	defer p.mu.Unlock()

	// Build indices of healthy workers in p.workers (not a copy).
	var healthyIdx []int
	for i, w := range p.workers {
		if w.Healthy {
			healthyIdx = append(healthyIdx, i)
		}
	}
	if len(healthyIdx) == 0 {
		return nil, ErrNoHealthyWorkers
	}

	var idx int

	switch p.strategy {
	case RoundRobin:
		idx = healthyIdx[p.next%len(healthyIdx)]
		p.next++
		WorkerSelectionTotal.WithLabelValues("round_robin").Inc()

	case LeastLoaded:
		idx = p.findLeastLoadedIdx(healthyIdx)
		WorkerSelectionTotal.WithLabelValues("least_loaded").Inc()

	default:
		return nil, fmt.Errorf("unknown strategy: %s", p.strategy)
	}

	// Optimistically increment so the next SelectWorker call sees this job.
	p.workers[idx].Active++

	// Return a pointer into the live slice so callers see the current address.
	return &p.workers[idx], nil
}

// IncrementActive increments the active job count for a worker by ID.
// It is a no-op if the worker is not found.
func (p *Pool) IncrementActive(workerID string) {
	p.mu.Lock()
	defer p.mu.Unlock()
	for i := range p.workers {
		if p.workers[i].ID == workerID {
			p.workers[i].Active++
			return
		}
	}
}

// DecrementActive decrements the active job count for a worker by ID,
// clamping at zero. It is a no-op if the worker is not found.
func (p *Pool) DecrementActive(workerID string) {
	p.mu.Lock()
	defer p.mu.Unlock()
	for i := range p.workers {
		if p.workers[i].ID == workerID {
			if p.workers[i].Active > 0 {
				p.workers[i].Active--
			}
			return
		}
	}
}

// Refresh re-discovers all workers.
// It preserves the orchestrator's live Active counts for existing workers so
// that load-balancing decisions remain accurate between discovery cycles.
func (p *Pool) Refresh(ctx context.Context) error {
	workers, err := p.discovery.GetWorkers(ctx)
	if err != nil {
		WorkerDiscoveryErrors.Inc()
		return err
	}

	p.mu.Lock()
	// Carry over the orchestrator-tracked Active counts for workers that are
	// already known.  Discovery backends return Active == 0 (or a stale value
	// from the remote pod); we prefer our own bookkeeping.
	activeByID := make(map[string]int32, len(p.workers))
	for _, w := range p.workers {
		activeByID[w.ID] = w.Active
	}
	for i := range workers {
		if live, ok := activeByID[workers[i].ID]; ok {
			workers[i].Active = live
		}
	}
	p.workers = workers
	p.mu.Unlock()

	// Update metrics
	UpdateWorkerMetrics(workers)

	p.log.WithField("count", len(workers)).Info("Workers refreshed")

	return nil
}

// healthCheckLoop periodically refreshes worker list
func (p *Pool) healthCheckLoop(ctx context.Context) {
	ticker := time.NewTicker(p.healthCheckInterval)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			if err := p.Refresh(ctx); err != nil {
				p.log.WithError(err).Error("Failed to refresh workers")
			}
		}
	}
}

// watchLoop handles worker change events from discovery watch
// Implements automatic reconnection with exponential backoff and max retries
func (p *Pool) watchLoop(ctx context.Context, eventCh <-chan WorkerEvent) {
	backoff := time.Second // Initial backoff: 1 second
	maxBackoff := 30 * time.Second
	maxRetries := 10 // Max reconnection attempts before falling back to periodic refresh
	retryCount := 0

	for {
		select {
		case <-ctx.Done():
			p.log.Info("Watch loop context cancelled, stopping")
			return

		case event, ok := <-eventCh:
			if !ok {
				// Watch channel closed - connection lost
				retryCount++

				if retryCount > maxRetries {
					p.log.WithField("retries", retryCount).Error("Max watch reconnection attempts exceeded, falling back to periodic refresh only")
					return
				}

				p.log.WithFields(logrus.Fields{
					"backoff": backoff,
					"retry":   retryCount,
				}).Warn("Watch channel closed, reconnecting after backoff")

				// Wait before reconnecting (exponential backoff)
				select {
				case <-time.After(backoff):
					// Increase backoff for next attempt (exponential)
					backoff = backoff * 2
					if backoff > maxBackoff {
						backoff = maxBackoff
					}
				case <-ctx.Done():
					return
				}

				// Attempt to re-establish watch
				WorkerWatchReconnectsTotal.Inc()
				newCh, err := p.discovery.Watch(ctx)
				if err != nil {
					p.log.WithError(err).Error("Failed to reconnect watch, will retry")
					// Continue loop to retry with increased backoff
					continue
				}

				p.log.WithField("retry", retryCount).Info("Successfully reconnected watch")

				// Reset backoff on successful reconnection (but keep retry count)
				backoff = time.Second
				eventCh = newCh
				continue
			}

			// Process the event
			p.handleWorkerEvent(event)

			// Reset backoff and retry count on successful event (connection is healthy)
			backoff = time.Second
			retryCount = 0
		}
	}
}

// handleWorkerEvent processes worker add/remove/update events
func (p *Pool) handleWorkerEvent(event WorkerEvent) {
	p.mu.Lock()
	defer p.mu.Unlock()

	switch event.Type {
	case EventTypeAdded:
		// Use slice replacement to avoid race conditions with Refresh()
		newWorkers := make([]Worker, len(p.workers)+1)
		copy(newWorkers, p.workers)
		newWorkers[len(p.workers)] = event.Worker
		p.workers = newWorkers

		UpdateWorkerMetrics(p.workers) // Update Prometheus metrics
		p.log.WithField("worker_id", event.Worker.ID).Info("Worker added")

	case EventTypeRemoved:
		p.workers = removeWorker(p.workers, event.Worker.ID)
		UpdateWorkerMetrics(p.workers) // Update Prometheus metrics
		p.log.WithField("worker_id", event.Worker.ID).Info("Worker removed")

	case EventTypeUpdated:
		p.workers = updateWorker(p.workers, event.Worker)
		UpdateWorkerMetrics(p.workers) // Update Prometheus metrics
		p.log.WithField("worker_id", event.Worker.ID).Debug("Worker updated")
	}
}

// filterHealthy returns only healthy workers
func (p *Pool) filterHealthy() []Worker {
	var healthy []Worker
	for _, w := range p.workers {
		if w.Healthy {
			healthy = append(healthy, w)
		}
	}
	return healthy
}

// findLeastLoadedIdx returns the index into p.workers of the healthy worker
// with the fewest active jobs. healthyIdx must be non-empty.
// On a tie the worker with the lowest index is chosen, which together with
// RoundRobin-style tie-breaking keeps distribution even.
func (p *Pool) findLeastLoadedIdx(healthyIdx []int) int {
	best := healthyIdx[0]
	for _, idx := range healthyIdx[1:] {
		if p.workers[idx].Active < p.workers[best].Active {
			best = idx
		}
	}
	return best
}

// Helper functions for worker list manipulation
func removeWorker(workers []Worker, id string) []Worker {
	for i, w := range workers {
		if w.ID == id {
			return append(workers[:i], workers[i+1:]...)
		}
	}
	return workers
}

func updateWorker(workers []Worker, updated Worker) []Worker {
	for i, w := range workers {
		if w.ID == updated.ID {
			workers[i] = updated
			return workers
		}
	}
	return append(workers, updated)
}

// IsWorkerHealthy checks if a worker with the given address is healthy
func (p *Pool) IsWorkerHealthy(address string) bool {
	p.mu.RLock()
	defer p.mu.RUnlock()

	for _, w := range p.workers {
		if w.Address == address {
			return w.Healthy
		}
	}
	return false
}
