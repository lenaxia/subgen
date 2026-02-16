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
	// Initial discovery
	if err := p.Refresh(ctx); err != nil {
		return err
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

// SelectWorker chooses a worker based on load balancing strategy
func (p *Pool) SelectWorker() (*Worker, error) {
	p.mu.Lock()
	defer p.mu.Unlock()

	healthy := p.filterHealthy()
	if len(healthy) == 0 {
		return nil, ErrNoHealthyWorkers
	}

	var worker *Worker

	switch p.strategy {
	case RoundRobin:
		worker = &healthy[p.next%len(healthy)]
		p.next++
		WorkerSelectionTotal.WithLabelValues("round_robin").Inc()

	case LeastLoaded:
		worker = p.findLeastLoaded(healthy)
		WorkerSelectionTotal.WithLabelValues("least_loaded").Inc()

	default:
		return nil, fmt.Errorf("unknown strategy: %s", p.strategy)
	}

	return worker, nil
}

// Refresh re-discovers all workers
func (p *Pool) Refresh(ctx context.Context) error {
	workers, err := p.discovery.GetWorkers(ctx)
	if err != nil {
		WorkerDiscoveryErrors.Inc()
		return err
	}

	p.mu.Lock()
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

// watchLoop handles worker change events
func (p *Pool) watchLoop(ctx context.Context, eventCh <-chan WorkerEvent) {
	for {
		select {
		case <-ctx.Done():
			return
		case event, ok := <-eventCh:
			if !ok {
				return
			}
			p.handleWorkerEvent(event)
		}
	}
}

// handleWorkerEvent processes worker add/remove/update events
func (p *Pool) handleWorkerEvent(event WorkerEvent) {
	p.mu.Lock()
	defer p.mu.Unlock()

	switch event.Type {
	case EventTypeAdded:
		p.workers = append(p.workers, event.Worker)
		p.log.WithField("worker_id", event.Worker.ID).Info("Worker added")

	case EventTypeRemoved:
		p.workers = removeWorker(p.workers, event.Worker.ID)
		p.log.WithField("worker_id", event.Worker.ID).Info("Worker removed")

	case EventTypeUpdated:
		p.workers = updateWorker(p.workers, event.Worker)
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

// findLeastLoaded returns worker with fewest active jobs
func (p *Pool) findLeastLoaded(workers []Worker) *Worker {
	if len(workers) == 0 {
		return nil
	}

	leastLoaded := &workers[0]
	for i := range workers {
		if workers[i].Active < leastLoaded.Active {
			leastLoaded = &workers[i]
		}
	}
	return leastLoaded
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
