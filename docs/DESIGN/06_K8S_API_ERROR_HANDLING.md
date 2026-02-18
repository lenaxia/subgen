# Kubernetes API Error Handling & Resilience

**Document Version:** 2.0  
**Last Updated:** 2026-02-17  
**Status:** ✅ Reconciled with Actual Code  
**Related Documents:**
- [05_WORKER_POOL_CONCURRENCY.md](./05_WORKER_POOL_CONCURRENCY.md)
- [03_SCALING_STRATEGY.md](./03_SCALING_STRATEGY.md)
- [EPIC_09 README](../BACKLOG/EPIC_09/README.md)
- [Design Audit Report](../BACKLOG/EPIC_09/DESIGN_AUDIT_2026-02-17.md)

---

## 🔄 Document Status

**Version 2.0** - Updated to clarify error return patterns and match actual implementation expectations.

**Changes from v1.0:**
- Clarified standardized error return pattern: Always return `([]Worker{}, error)` not `(nil, error)`
- Added note about K8s client field requirement in KubernetesDiscovery struct
- Clarified that retry logic is recommendation, not required for Phase 2 MVP
- Updated proto field name references (jobs_active not JobsActive)

---

## Error Return Pattern (STANDARDIZED)

**Critical Decision:** Throughout this document and all implementations, use this pattern:

```go
// ✅ CORRECT: Always return empty slice with error
func (d *KubernetesDiscovery) GetWorkers(ctx context.Context) ([]Worker, error) {
    if err != nil {
        return []Worker{}, fmt.Errorf("error message: %w", err)
    }
    return workers, nil
}

// ❌ WRONG: Don't return nil slice
func (d *KubernetesDiscovery) GetWorkers(ctx context.Context) ([]Worker, error) {
    if err != nil {
        return nil, fmt.Errorf("error message: %w", err)  // Don't do this
    }
    return workers, nil
}
```

**Rationale:**
- `Pool.Refresh()` expects non-nil slice, even on error
- Consistent pattern easier to reason about
- Prevents nil pointer dereferences

**Implementation Note:**
```go
// In pool.go
func (p *Pool) Refresh(ctx context.Context) error {
    workers, err := p.discovery.GetWorkers(ctx)
    if err != nil {
        // workers is []Worker{}, not nil - safe to log
        p.log.WithField("count", len(workers)).Warn("Discovery failed")
        return err
    }
    p.workers = workers  // Safe assignment
    return nil
}
```

---

## Table of Contents

1. [Overview](#overview)
2. [Error Catalog](#error-catalog)
3. [Retry Strategy](#retry-strategy)
4. [Fallback Mechanisms](#fallback-mechanisms)
5. [RBAC Error Handling](#rbac-error-handling)
6. [Watch Disconnection Handling](#watch-disconnection-handling)
7. [Circuit Breaker Pattern](#circuit-breaker-pattern)
8. [Metrics & Monitoring](#metrics--monitoring)

---

## Overview

### Purpose

Define comprehensive error handling for all Kubernetes API interactions, ensuring the orchestrator remains operational even when K8s API is temporarily unavailable or misconfigured.

### Design Principles

1. **Fail gracefully**: Continue operating with reduced functionality
2. **Fast failure detection**: Detect issues within 30 seconds
3. **Automatic recovery**: Reconnect/retry without manual intervention
4. **Clear error messages**: Log enough context for debugging
5. **No cascading failures**: One component failure shouldn't crash entire system

### Implementation Status

**Phase 1 (Complete):** No K8s API interactions (localhost discovery only)  
**Phase 2 (In Progress):** K8s API integration begins with STORY_01

**Prerequisites for STORY_01:**
1. Add `client *kubernetes.Clientset` field to `KubernetesDiscovery` struct
2. Initialize K8s in-cluster client in `NewKubernetesDiscovery()`
3. Implement basic error handling (this document)
4. Advanced error handling (retry, circuit breaker) can be added incrementally

**Minimum Viable Implementation (Phase 2 MVP):**
- ✅ Handle RBAC errors (403 Forbidden)
- ✅ Handle NotFound errors (404)
- ✅ Return empty slice on error
- ⏳ Retry logic (recommended but not blocking)
- ⏳ Circuit breaker (future enhancement)
- ⏳ Cache fallback (future enhancement)

---

## Error Catalog

### 1. RBAC Permission Errors

#### Error: Forbidden (403)

```
Error: endpoints "subgen-worker" is forbidden: User "system:serviceaccount:media:subgen-orchestrator" 
cannot get resource "endpoints" in API group "" in the namespace "media"
```

**Cause**: ServiceAccount lacks RBAC permissions  
**Impact**: **CRITICAL** - Worker discovery completely broken  
**Recovery**: Requires manual RBAC fix (not automatic)

**Handling**:
```go
func (d *KubernetesDiscovery) GetWorkers(ctx context.Context) ([]Worker, error) {
	endpoints, err := d.client.CoreV1().Endpoints(d.namespace).Get(
		ctx, d.service, metav1.GetOptions{},
	)
	
	if err != nil {
		if k8sErrors.IsForbidden(err) {
			// RBAC issue - log detailed error and disable K8s discovery
			d.log.WithFields(logrus.Fields{
				"namespace": d.namespace,
				"service":   d.service,
				"error":     err,
			}).Error("RBAC permissions missing - check ServiceAccount, Role, and RoleBinding")
			
			// Emit metric for alerting
			d.metrics.RBACErrorsTotal.Inc()
			
			// CRITICAL: Cannot recover automatically
			// Return empty list to allow graceful degradation
			return []Worker{}, fmt.Errorf("RBAC permission denied (see docs/DEPLOYMENT/rbac.yaml)")
		}
		
		// Handle other errors...
	}
	
	return workers, nil
}
```

**User Action Required**:
```bash
# Verify RBAC setup
kubectl auth can-i get endpoints --as=system:serviceaccount:media:subgen-orchestrator -n media

# If "no", apply RBAC:
kubectl apply -f deploy/rbac.yaml
```

---

#### Error: Unauthorized (401)

```
Error: Unauthorized
```

**Cause**: Invalid or missing ServiceAccount token  
**Impact**: **CRITICAL** - K8s client cannot authenticate  
**Recovery**: Requires pod restart

**Handling**:
```go
func NewKubernetesDiscovery(...) (*KubernetesDiscovery, error) {
	config, err := rest.InClusterConfig()
	if err != nil {
		if strings.Contains(err.Error(), "unable to load in-cluster configuration") {
			// Not running in K8s cluster (e.g., local development)
			return nil, fmt.Errorf("not running in Kubernetes cluster: %w", err)
		}
		return nil, fmt.Errorf("failed to get in-cluster config: %w", err)
	}
	
	clientset, err := kubernetes.NewForConfig(config)
	if err != nil {
		return nil, fmt.Errorf("failed to create K8s client: %w", err)
	}
	
	// Test authentication immediately
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	
	_, err = clientset.CoreV1().Endpoints(namespace).List(ctx, metav1.ListOptions{Limit: 1})
	if err != nil {
		if k8sErrors.IsUnauthorized(err) {
			return nil, fmt.Errorf("K8s authentication failed - check ServiceAccount configuration: %w", err)
		}
		// Other errors are non-fatal (might be transient)
		log.WithError(err).Warn("K8s client test failed, but continuing")
	}
	
	return &KubernetesDiscovery{...}, nil
}
```

---

### 2. Resource Not Found Errors

#### Error: NotFound (404)

```
Error: endpoints "subgen-worker" not found
```

**Cause**: Worker service doesn't exist (not deployed yet, wrong name)  
**Impact**: **HIGH** - No workers discovered  
**Recovery**: Automatic (once service created)

**Handling**:
```go
func (d *KubernetesDiscovery) GetWorkers(ctx context.Context) ([]Worker, error) {
	endpoints, err := d.client.CoreV1().Endpoints(d.namespace).Get(
		ctx, d.service, metav1.GetOptions{},
	)
	
	if err != nil {
		if k8sErrors.IsNotFound(err) {
			d.log.WithFields(logrus.Fields{
				"namespace": d.namespace,
				"service":   d.service,
			}).Warn("Endpoints not found - worker service may not be deployed yet")
			
			// Return empty list (not an error - service might be deploying)
			return []Worker{}, nil
		}
		
		// Handle other errors...
	}
	
	// Check if endpoints has any addresses
	if len(endpoints.Subsets) == 0 {
		d.log.WithFields(logrus.Fields{
			"namespace": d.namespace,
			"service":   d.service,
		}).Debug("Endpoints exist but no ready pods yet")
		return []Worker{}, nil
	}
	
	// Parse workers from subsets...
}
```

---

### 3. API Rate Limiting

#### Error: TooManyRequests (429)

```
Error: the server has received too many requests and has asked us to try again later
Retry-After: 30
```

**Cause**: Too many requests to K8s API (throttled by API server)  
**Impact**: **MEDIUM** - Temporary inability to discover workers  
**Recovery**: Automatic after backoff

**Handling**:
```go
func (d *KubernetesDiscovery) GetWorkersWithRetry(ctx context.Context) ([]Worker, error) {
	var lastErr error
	
	for attempt := 0; attempt < 5; attempt++ {
		workers, err := d.GetWorkers(ctx)
		
		if err == nil {
			return workers, nil
		}
		
		lastErr = err
		
		// Check if rate limited
		if k8sErrors.IsTooManyRequests(err) {
			// Extract Retry-After header if available
			retryAfter := 1 * time.Second
			if statusErr, ok := err.(*k8sErrors.StatusError); ok {
				if ra := statusErr.Status().Details.RetryAfterSeconds; ra > 0 {
					retryAfter = time.Duration(ra) * time.Second
				}
			}
			
			d.log.WithFields(logrus.Fields{
				"attempt":     attempt + 1,
				"retry_after": retryAfter,
			}).Warn("K8s API rate limited, backing off")
			
			time.Sleep(retryAfter)
			continue
		}
		
		// For other errors, use exponential backoff
		if isRetryable(err) {
			backoff := time.Duration(1<<attempt) * time.Second
			if backoff > 30*time.Second {
				backoff = 30 * time.Second
			}
			
			d.log.WithFields(logrus.Fields{
				"attempt": attempt + 1,
				"backoff": backoff,
				"error":   err,
			}).Warn("K8s API request failed, retrying")
			
			time.Sleep(backoff)
			continue
		}
		
		// Non-retryable error
		return nil, fmt.Errorf("K8s API request failed: %w", err)
	}
	
	return nil, fmt.Errorf("K8s API request failed after 5 attempts: %w", lastErr)
}

func isRetryable(err error) bool {
	// Retryable errors
	if k8sErrors.IsTimeout(err) {
		return true
	}
	if k8sErrors.IsServerTimeout(err) {
		return true
	}
	if k8sErrors.IsInternalError(err) {
		return true
	}
	if k8sErrors.IsServiceUnavailable(err) {
		return true
	}
	
	// Check for network errors
	if strings.Contains(err.Error(), "connection refused") {
		return true
	}
	if strings.Contains(err.Error(), "i/o timeout") {
		return true
	}
	
	return false
}
```

---

### 4. Network Errors

#### Error: Connection Refused

```
Error: dial tcp 10.96.0.1:443: connect: connection refused
```

**Cause**: K8s API server unreachable (network partition, server down)  
**Impact**: **CRITICAL** - Cannot discover workers  
**Recovery**: Automatic reconnection, fallback to cached workers

**Handling**: See "Fallback Mechanisms" section below

---

#### Error: Context Deadline Exceeded

```
Error: context deadline exceeded
```

**Cause**: K8s API request timeout (slow network, overloaded API server)  
**Impact**: **MEDIUM** - Delayed worker discovery  
**Recovery**: Automatic retry with exponential backoff

**Handling**:
```go
func (d *KubernetesDiscovery) GetWorkers(ctx context.Context) ([]Worker, error) {
	// Set reasonable timeout for K8s API call
	ctx, cancel := context.WithTimeout(ctx, 10*time.Second)
	defer cancel()
	
	endpoints, err := d.client.CoreV1().Endpoints(d.namespace).Get(
		ctx, d.service, metav1.GetOptions{},
	)
	
	if err != nil {
		if ctx.Err() == context.DeadlineExceeded {
			d.log.Warn("K8s API request timed out after 10s")
			d.metrics.TimeoutsTotal.Inc()
			
			// Return cached workers if available
			if cached := d.getCachedWorkers(); len(cached) > 0 {
				d.log.Debug("Using cached worker list due to timeout")
				return cached, nil
			}
		}
		
		return nil, fmt.Errorf("failed to get endpoints: %w", err)
	}
	
	// Success - update cache
	workers := d.parseWorkers(endpoints)
	d.updateCache(workers)
	
	return workers, nil
}
```

---

### 5. Watch-Specific Errors

#### Error: Watch Closed Unexpectedly

```
Watch closed with error: too old resource version
```

**Cause**: Resource version too old (K8s cleared history)  
**Impact**: **LOW** - Watch needs restart  
**Recovery**: Automatic full resync + new watch

**Handling**:
```go
func (d *KubernetesDiscovery) Watch(ctx context.Context) (<-chan WorkerEvent, error) {
	// Get current resource version
	endpoints, err := d.client.CoreV1().Endpoints(d.namespace).Get(
		ctx, d.service, metav1.GetOptions{},
	)
	if err != nil {
		return nil, fmt.Errorf("failed to get initial endpoints: %w", err)
	}
	
	resourceVersion := endpoints.ResourceVersion
	
	// Start watch from this version
	watcher, err := d.client.CoreV1().Endpoints(d.namespace).Watch(ctx, metav1.ListOptions{
		FieldSelector:   fmt.Sprintf("metadata.name=%s", d.service),
		ResourceVersion: resourceVersion,
	})
	if err != nil {
		return nil, fmt.Errorf("failed to watch endpoints: %w", err)
	}
	
	ch := make(chan WorkerEvent)
	
	go func() {
		defer close(ch)
		defer watcher.Stop()
		
		for {
			select {
			case <-ctx.Done():
				return
			case event, ok := <-watcher.ResultChan():
				if !ok {
					// Watch closed, return to trigger reconnect
					d.log.Warn("Watch channel closed, reconnecting")
					return
				}
				
				// Check for error events
				if event.Type == watch.Error {
					if status, ok := event.Object.(*metav1.Status); ok {
						// Check if "too old resource version"
						if k8sErrors.IsResourceExpired(status) {
							d.log.Warn("Resource version expired, full resync needed")
							// Full resync will happen on reconnect
							return
						}
						
						d.log.WithField("status", status).Error("Watch error event")
					}
					return
				}
				
				// Process normal events...
				d.handleWatchEvent(ctx, event, ch)
			}
		}
	}()
	
	return ch, nil
}
```

---

## Retry Strategy

### Exponential Backoff Parameters

```go
type RetryConfig struct {
	InitialDelay  time.Duration  // 1 second
	MaxDelay      time.Duration  // 30 seconds
	Multiplier    float64        // 2.0 (exponential)
	MaxAttempts   int            // 5 attempts
	Jitter        bool           // Add randomness
}

var DefaultRetryConfig = RetryConfig{
	InitialDelay: 1 * time.Second,
	MaxDelay:     30 * time.Second,
	Multiplier:   2.0,
	MaxAttempts:  5,
	Jitter:       true,
}
```

### Retry Implementation

```go
func (c *RetryConfig) ExecuteWithRetry(ctx context.Context, fn func() error) error {
	var lastErr error
	
	for attempt := 0; attempt < c.MaxAttempts; attempt++ {
		err := fn()
		
		if err == nil {
			return nil
		}
		
		lastErr = err
		
		// Check if retryable
		if !isRetryable(err) {
			return fmt.Errorf("non-retryable error: %w", err)
		}
		
		// Calculate backoff
		delay := time.Duration(float64(c.InitialDelay) * math.Pow(c.Multiplier, float64(attempt)))
		if delay > c.MaxDelay {
			delay = c.MaxDelay
		}
		
		// Add jitter (±25%)
		if c.Jitter {
			jitter := time.Duration(rand.Float64() * float64(delay) * 0.5)
			delay = delay - jitter/2 + jitter
		}
		
		log.WithFields(logrus.Fields{
			"attempt": attempt + 1,
			"delay":   delay,
			"error":   err,
		}).Warn("Operation failed, retrying")
		
		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-time.After(delay):
			// Continue to next attempt
		}
	}
	
	return fmt.Errorf("operation failed after %d attempts: %w", c.MaxAttempts, lastErr)
}
```

### Usage

```go
err := retryConfig.ExecuteWithRetry(ctx, func() error {
	_, err := d.client.CoreV1().Endpoints(d.namespace).Get(ctx, d.service, metav1.GetOptions{})
	return err
})
```

---

## Fallback Mechanisms

### 1. Cached Worker List

**Problem**: K8s API temporarily unavailable  
**Solution**: Return last known good worker list

```go
type KubernetesDiscovery struct {
	// ... other fields ...
	
	// Cache
	cacheMu       sync.RWMutex
	cachedWorkers []Worker
	cacheTime     time.Time
	cacheTTL      time.Duration  // 5 minutes
}

func (d *KubernetesDiscovery) GetWorkers(ctx context.Context) ([]Worker, error) {
	// Try to get fresh data from K8s API
	workers, err := d.getWorkersFromK8s(ctx)
	
	if err != nil {
		// K8s API failed, try cache
		if cached := d.getCachedWorkers(); len(cached) > 0 {
			d.log.WithError(err).Warn("K8s API unavailable, using cached worker list")
			d.metrics.CacheHitsTotal.Inc()
			return cached, nil
		}
		
		// Cache empty or expired
		return nil, fmt.Errorf("K8s API unavailable and no cached workers: %w", err)
	}
	
	// Success - update cache
	d.updateCache(workers)
	
	return workers, nil
}

func (d *KubernetesDiscovery) getCachedWorkers() []Worker {
	d.cacheMu.RLock()
	defer d.cacheMu.RUnlock()
	
	// Check if cache is fresh
	if time.Since(d.cacheTime) > d.cacheTTL {
		return nil
	}
	
	// Return copy to prevent external modification
	result := make([]Worker, len(d.cachedWorkers))
	copy(result, d.cachedWorkers)
	return result
}

func (d *KubernetesDiscovery) updateCache(workers []Worker) {
	d.cacheMu.Lock()
	defer d.cacheMu.Unlock()
	
	d.cachedWorkers = workers
	d.cacheTime = time.Now()
}
```

---

### 2. Periodic Fallback Polling

**Problem**: Watch disconnected, events missed  
**Solution**: Poll GetWorkers() every 30s as backup

```go
func (p *WorkerPool) StartDiscovery(ctx context.Context) error {
	// Start watch-based discovery
	p.wg.Add(1)
	go p.watchLoop(ctx)
	
	// Start fallback polling (runs in parallel)
	p.wg.Add(1)
	go p.pollLoop(ctx)
	
	return nil
}

func (p *WorkerPool) pollLoop(ctx context.Context) {
	defer p.wg.Done()
	
	ticker := time.NewTicker(30 * time.Second)
	defer ticker.Stop()
	
	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			// Get fresh worker list
			workers, err := p.discovery.GetWorkers(ctx)
			if err != nil {
				p.log.WithError(err).Debug("Periodic worker refresh failed")
				continue
			}
			
			// Reconcile with current pool
			p.reconcileWorkers(ctx, workers)
		}
	}
}

func (p *WorkerPool) reconcileWorkers(ctx context.Context, discovered []Worker) {
	p.mu.Lock()
	defer p.mu.Unlock()
	
	// Build set of discovered worker IDs
	discoveredIDs := make(map[string]bool)
	for _, w := range discovered {
		discoveredIDs[w.ID] = true
		
		// Add or update worker
		if existing, exists := p.workers[w.ID]; exists {
			// Update health
			atomic.StoreInt32(&existing.healthy, w.healthy)
			existing.LastSeen = time.Now()
		} else {
			// Add new worker
			p.workers[w.ID] = &w
			p.log.WithField("worker", w.ID).Info("Discovered new worker via polling")
		}
	}
	
	// Remove workers no longer discovered
	for id := range p.workers {
		if !discoveredIDs[id] {
			delete(p.workers, id)
			p.log.WithField("worker", id).Info("Removed stale worker via polling")
		}
	}
}
```

---

### 3. Graceful Degradation

**Problem**: All K8s API interactions failing  
**Solution**: Continue with last known workers, emit alerts

```go
func (d *KubernetesDiscovery) getWorkersWithDegradation(ctx context.Context) ([]Worker, error) {
	// Attempt 1: Fresh K8s API call
	workers, err := d.getWorkersFromK8s(ctx)
	if err == nil {
		d.degradedMode = false
		return workers, nil
	}
	
	// Attempt 2: Cached workers
	if cached := d.getCachedWorkers(); len(cached) > 0 {
		if !d.degradedMode {
			d.log.Error("Entering degraded mode - K8s API unavailable, using cached workers")
			d.metrics.DegradedMode.Set(1)
			d.degradedMode = true
		}
		return cached, nil
	}
	
	// Attempt 3: No workers available
	d.log.Error("No workers available - K8s API down and no cache")
	d.metrics.DegradedMode.Set(1)
	
	return nil, fmt.Errorf("worker discovery failed: %w", err)
}
```

---

## RBAC Error Handling

### Pre-flight RBAC Check

**Run during initialization to fail fast**:

```go
func (d *KubernetesDiscovery) verifyRBAC(ctx context.Context) error {
	// Test if we can read endpoints
	_, err := d.client.CoreV1().Endpoints(d.namespace).List(ctx, metav1.ListOptions{Limit: 1})
	if err != nil {
		if k8sErrors.IsForbidden(err) {
			return fmt.Errorf(`
RBAC permission denied. Orchestrator cannot read Endpoints.

Required permissions:
  apiGroups: [""]
  resources: ["endpoints"]
  verbs: ["get", "list", "watch"]

Fix:
  kubectl apply -f deploy/rbac.yaml

Error: %w`, err)
		}
		
		// Other errors (timeout, etc.) are warnings, not fatal
		d.log.WithError(err).Warn("RBAC verification failed (non-fatal)")
	}
	
	return nil
}

func NewKubernetesDiscovery(...) (*KubernetesDiscovery, error) {
	// ... create client ...
	
	// Verify RBAC permissions
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	
	if err := discovery.verifyRBAC(ctx); err != nil {
		return nil, err  // Fatal error
	}
	
	return discovery, nil
}
```

### RBAC Permission Set

**File**: `deploy/rbac.yaml`

```yaml
apiVersion: v1
kind: ServiceAccount
metadata:
  name: subgen-orchestrator
  namespace: media

---
apiVersion: rbac.authorization.k8s.io/v1
kind: Role
metadata:
  name: subgen-orchestrator
  namespace: media
rules:
- apiGroups: [""]
  resources: ["endpoints"]
  verbs: ["get", "list", "watch"]

---
apiVersion: rbac.authorization.k8s.io/v1
kind: RoleBinding
metadata:
  name: subgen-orchestrator
  namespace: media
subjects:
- kind: ServiceAccount
  name: subgen-orchestrator
  namespace: media
roleRef:
  kind: Role
  name: subgen-orchestrator
  apiGroup: rbac.authorization.k8s.io
```

---

## Watch Disconnection Handling

### Watch Lifecycle

```
[Start] → [Connected] → [Event Processing]
                ↓
            [Disconnected] → [Reconnect with backoff] → [Full Resync] → [Connected]
```

### Implementation

```go
func (p *WorkerPool) watchLoop(ctx context.Context) {
	defer p.wg.Done()
	
	backoff := &ExponentialBackoff{
		InitialDelay: 1 * time.Second,
		MaxDelay:     30 * time.Second,
		Multiplier:   2.0,
	}
	
	for {
		select {
		case <-ctx.Done():
			return
		default:
		}
		
		// Start watch
		eventCh, err := p.discovery.Watch(ctx)
		if err != nil {
			delay := backoff.Next()
			p.log.WithError(err).Warnf("Watch failed, retrying in %s", delay)
			p.metrics.WatchErrorsTotal.Inc()
			
			time.Sleep(delay)
			continue
		}
		
		// Watch connected successfully, reset backoff
		backoff.Reset()
		p.log.Info("Watch connected to K8s API")
		
		// Process events until watch closes
		p.processWatchEvents(ctx, eventCh)
		
		// Watch closed (normal or error), loop will reconnect
		p.log.Warn("Watch disconnected, reconnecting...")
		p.metrics.WatchDisconnectsTotal.Inc()
		
		// Do immediate full resync before reconnecting
		if err := p.fullResync(ctx); err != nil {
			p.log.WithError(err).Error("Full resync failed")
		}
	}
}

func (p *WorkerPool) fullResync(ctx context.Context) error {
	p.log.Info("Performing full worker resync...")
	
	workers, err := p.discovery.GetWorkers(ctx)
	if err != nil {
		return fmt.Errorf("failed to get workers: %w", err)
	}
	
	p.reconcileWorkers(ctx, workers)
	
	p.log.WithField("count", len(workers)).Info("Full resync complete")
	return nil
}
```

---

## Circuit Breaker Pattern

### When to Use

If K8s API is consistently failing (>80% error rate over 1 minute), temporarily stop making requests to avoid overwhelming the API server.

### Implementation

```go
type CircuitBreaker struct {
	mu            sync.RWMutex
	state         CircuitState
	failures      int
	successes     int
	lastFailTime  time.Time
	failThreshold int           // 5 failures
	resetTimeout  time.Duration // 30 seconds
}

type CircuitState string

const (
	StateClosed   CircuitState = "closed"    // Normal operation
	StateOpen     CircuitState = "open"      // Failing, reject requests
	StateHalfOpen CircuitState = "half_open" // Testing recovery
)

func (cb *CircuitBreaker) Call(fn func() error) error {
	// Check if circuit is open
	if cb.isOpen() {
		return fmt.Errorf("circuit breaker open (K8s API unavailable)")
	}
	
	// Execute function
	err := fn()
	
	if err != nil {
		cb.recordFailure()
		return err
	}
	
	cb.recordSuccess()
	return nil
}

func (cb *CircuitBreaker) isOpen() bool {
	cb.mu.RLock()
	defer cb.mu.RUnlock()
	
	if cb.state == StateOpen {
		// Check if we should transition to half-open
		if time.Since(cb.lastFailTime) > cb.resetTimeout {
			cb.state = StateHalfOpen
			return false
		}
		return true
	}
	
	return false
}

func (cb *CircuitBreaker) recordFailure() {
	cb.mu.Lock()
	defer cb.mu.Unlock()
	
	cb.failures++
	cb.lastFailTime = time.Now()
	
	if cb.failures >= cb.failThreshold {
		cb.state = StateOpen
		log.Error("Circuit breaker opened - K8s API consistently failing")
	}
}

func (cb *CircuitBreaker) recordSuccess() {
	cb.mu.Lock()
	defer cb.mu.Unlock()
	
	cb.successes++
	
	if cb.state == StateHalfOpen && cb.successes >= 3 {
		cb.state = StateClosed
		cb.failures = 0
		log.Info("Circuit breaker closed - K8s API recovered")
	}
}
```

**Usage**:
```go
circuitBreaker := &CircuitBreaker{
	failThreshold: 5,
	resetTimeout:  30 * time.Second,
}

err := circuitBreaker.Call(func() error {
	_, err := d.client.CoreV1().Endpoints(d.namespace).Get(ctx, d.service, metav1.GetOptions{})
	return err
})
```

---

## Metrics & Monitoring

### Prometheus Metrics

```go
// Kubernetes API metrics
var (
	K8sRequestsTotal = prometheus.NewCounterVec(
		prometheus.CounterOpts{
			Name: "subgen_k8s_requests_total",
			Help: "Total K8s API requests by operation and status",
		},
		[]string{"operation", "status"},  // operation=get|list|watch, status=success|error
	)
	
	K8sRequestDuration = prometheus.NewHistogramVec(
		prometheus.HistogramOpts{
			Name:    "subgen_k8s_request_duration_seconds",
			Help:    "K8s API request duration",
			Buckets: []float64{0.01, 0.05, 0.1, 0.5, 1.0, 5.0, 10.0},
		},
		[]string{"operation"},
	)
	
	K8sWatchDisconnectsTotal = prometheus.NewCounter(
		prometheus.CounterOpts{
			Name: "subgen_k8s_watch_disconnects_total",
			Help: "Total K8s watch disconnections",
		},
	)
	
	K8sRBACErrorsTotal = prometheus.NewCounter(
		prometheus.CounterOpts{
			Name: "subgen_k8s_rbac_errors_total",
			Help: "Total RBAC permission denied errors",
		},
	)
	
	K8sCacheHitsTotal = prometheus.NewCounter(
		prometheus.CounterOpts{
			Name: "subgen_k8s_cache_hits_total",
			Help: "Total times cached worker list was used",
		},
	)
	
	K8sDegradedMode = prometheus.NewGauge(
		prometheus.GaugeOpts{
			Name: "subgen_k8s_degraded_mode",
			Help: "1 if K8s discovery in degraded mode, 0 if normal",
		},
	)
)
```

### Alerts

**Prometheus AlertManager rules**:

```yaml
groups:
- name: subgen-k8s
  rules:
  - alert: K8sDiscoveryFailing
    expr: rate(subgen_k8s_requests_total{status="error"}[5m]) > 0.8
    for: 2m
    annotations:
      summary: "Subgen K8s discovery failing"
      description: "More than 80% of K8s API requests failing for 2 minutes"
  
  - alert: K8sWatchDisconnected
    expr: increase(subgen_k8s_watch_disconnects_total[5m]) > 3
    for: 1m
    annotations:
      summary: "K8s watch frequently disconnecting"
      description: "Watch disconnected 3+ times in 5 minutes"
  
  - alert: K8sRBACPermissionDenied
    expr: increase(subgen_k8s_rbac_errors_total[1m]) > 0
    for: 1m
    annotations:
      summary: "RBAC permissions missing"
      description: "Orchestrator lacks K8s API permissions (check ServiceAccount/Role/RoleBinding)"
  
  - alert: K8sDegradedMode
    expr: subgen_k8s_degraded_mode == 1
    for: 5m
    annotations:
      summary: "Subgen in degraded mode"
      description: "K8s API unavailable, using cached worker list"
```

---

## Testing Strategy

### Unit Tests

**Mock K8s client for error scenarios**:

```go
func TestGetWorkers_RBACForbidden(t *testing.T) {
	mockClient := &MockK8sClient{
		GetFunc: func() (*v1.Endpoints, error) {
			return nil, k8sErrors.NewForbidden(schema.GroupResource{}, "endpoints", fmt.Errorf("forbidden"))
		},
	}
	
	discovery := &KubernetesDiscovery{client: mockClient}
	
	workers, err := discovery.GetWorkers(context.Background())
	
	// Should return empty list, not error (graceful degradation)
	assert.NoError(t, err)
	assert.Empty(t, workers)
	
	// Metric should be incremented
	assert.Equal(t, 1, testutil.ToFloat64(metrics.RBACErrorsTotal))
}

func TestGetWorkers_CacheFallback(t *testing.T) {
	mockClient := &MockK8sClient{
		GetFunc: func() (*v1.Endpoints, error) {
			return nil, fmt.Errorf("connection refused")
		},
	}
	
	discovery := &KubernetesDiscovery{
		client: mockClient,
		cachedWorkers: []Worker{{ID: "worker-1"}},
		cacheTime: time.Now(),
	}
	
	workers, err := discovery.GetWorkers(context.Background())
	
	// Should return cached workers
	assert.NoError(t, err)
	assert.Len(t, workers, 1)
	assert.Equal(t, "worker-1", workers[0].ID)
}
```

### Integration Tests

**Test with real K8s (Kind cluster)**:

```bash
# Create Kind cluster with API server configured to fail
kind create cluster --config test/kind-config-flaky.yaml

# Test RBAC errors
kubectl delete rolebinding subgen-orchestrator -n media
# Verify orchestrator handles gracefully

# Test rate limiting
# (Difficult to trigger - may need to mock)

# Test watch disconnection
kubectl delete endpoints subgen-worker -n media
kubectl apply -f deploy/worker-service.yaml
# Verify watch reconnects and discovers workers
```

---

## Summary

### Key Decisions

1. **Graceful degradation**: Continue with cached workers when K8s API unavailable
2. **Exponential backoff**: 1s → 30s max for retries
3. **Circuit breaker**: Stop requests after 5 consecutive failures
4. **Dual discovery**: Watch (primary) + Polling (fallback every 30s)
5. **Pre-flight RBAC check**: Fail fast if permissions missing

### Error Priorities

| Error | Impact | Recovery | User Action |
|-------|--------|----------|-------------|
| RBAC Forbidden | CRITICAL | Manual | Apply RBAC config |
| Unauthorized | CRITICAL | Restart pod | Fix ServiceAccount |
| NotFound | HIGH | Automatic | Wait for service |
| Rate Limited | MEDIUM | Automatic | None |
| Network Error | CRITICAL | Automatic (cache) | Check network |
| Watch Closed | LOW | Automatic | None |

---

**Document Status**: ✅ Final  
**Ready for Implementation**: Yes  
**Estimated Implementation Time**: Included in STORY_01 (8-10h)
