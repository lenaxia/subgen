# STORY_03: Dynamic Worker Watch

**Epic:** EPIC_09  
**Status:** Not Started  
**Assignee:** TBD  
**Effort:** 6-8 hours

---

## User Story

As an **orchestrator**,  
I want to **watch Kubernetes Endpoints for real-time worker changes**,  
So that **new workers are discovered automatically without polling**.

---

## Acceptance Criteria

- [ ] K8s watch established on Endpoints resource
- [ ] Worker added events detected and processed
- [ ] Worker removed events detected and processed
- [ ] Worker updated events detected and processed
- [ ] Watch reconnects automatically on disconnection
- [ ] Worker pool updated in real-time (<30 seconds)
- [ ] Logs show worker lifecycle events
- [ ] Tests cover all event types

---

## Technical Design

### Implementation

Update `orchestrator/internal/discovery/kubernetes.go`:

```go
func (d *KubernetesDiscovery) Watch(ctx context.Context) (<-chan WorkerEvent, error) {
    // Create K8s watch
    watcher, err := d.client.CoreV1().Endpoints(d.namespace).Watch(ctx, metav1.ListOptions{
        FieldSelector: fmt.Sprintf("metadata.name=%s", d.service),
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
                d.log.Info("Watch context cancelled, stopping")
                return
                
            case event, ok := <-watcher.ResultChan():
                if !ok {
                    d.log.Warn("Watch channel closed, reconnecting...")
                    // Trigger reconnect (handled by pool's watchLoop)
                    return
                }
                
                d.handleEndpointEvent(ctx, event, ch)
            }
        }
    }()
    
    return ch, nil
}
```

### Event Handling

```go
func (d *KubernetesDiscovery) handleEndpointEvent(
    ctx context.Context, 
    event watch.Event, 
    ch chan<- WorkerEvent,
) {
    endpoints, ok := event.Object.(*corev1.Endpoints)
    if !ok {
        d.log.Error("Unexpected object type in watch event")
        return
    }
    
    switch event.Type {
    case watch.Added, watch.Modified:
        // Parse current workers from endpoints
        workers := d.parseWorkers(ctx, endpoints)
        
        // Compare with previous state and generate add/remove events
        for _, worker := range workers {
            ch <- WorkerEvent{
                Type:   EventTypeAdded,
                Worker: worker,
            }
        }
        
    case watch.Deleted:
        // All workers removed
        d.log.Info("Endpoints deleted, all workers removed")
        // Pool will handle by marking all as removed
        
    case watch.Error:
        d.log.WithError(event.Object).Error("Watch error event")
    }
}
```

### Reconnection Logic

```go
// In pool.go watchLoop()
func (p *Pool) watchLoop(ctx context.Context, eventCh <-chan WorkerEvent) {
    for {
        select {
        case <-ctx.Done():
            return
            
        case event, ok := <-eventCh:
            if !ok {
                // Channel closed, watch disconnected
                p.log.Warn("Watch disconnected, reconnecting in 5 seconds...")
                
                time.Sleep(5 * time.Second)
                
                // Re-establish watch
                newCh, err := p.discovery.Watch(ctx)
                if err != nil {
                    p.log.WithError(err).Error("Failed to reconnect watch")
                    // Fall back to periodic refresh
                    return
                }
                
                eventCh = newCh
                continue
            }
            
            p.handleWorkerEvent(event)
        }
    }
}
```

---

## Event Types

### 1. Worker Added

**Trigger**: New pod in Endpoints  
**Action**: Add to worker pool  
**Log**: `INFO: Worker added: subgen-worker-3`

### 2. Worker Removed

**Trigger**: Pod removed from Endpoints  
**Action**: Remove from worker pool, mark unhealthy  
**Log**: `INFO: Worker removed: subgen-worker-3`

### 3. Worker Updated

**Trigger**: Endpoint address changed (health status)  
**Action**: Update worker health in pool  
**Log**: `DEBUG: Worker updated: subgen-worker-1 (healthy=true)`

---

## Testing Strategy

### Unit Tests

```go
// TestWatch_Success
// TestWatch_ContextCancelled
// TestWatch_WatchError
// TestHandleEndpointEvent_Added
// TestHandleEndpointEvent_Modified
// TestHandleEndpointEvent_Deleted
// TestHandleEndpointEvent_Error
```

### Integration Tests

Requires K8s test environment:

```go
func TestWatch_Integration(t *testing.T) {
    // 1. Create test Endpoints with 2 addresses
    // 2. Start watch
    // 3. Add 3rd address to Endpoints
    // 4. Verify EventTypeAdded received
    // 5. Remove 1st address from Endpoints
    // 6. Verify EventTypeRemoved received
}
```

### Manual Testing

```bash
# 1. Deploy Phase 2 (3 workers)
helm install subgen-worker bjw-s/app-template \
  --namespace media \
  --values deploy/values-phase2-workers.yaml \
  --set controllers.main.replicas=3

# 2. Watch orchestrator logs
kubectl logs -n media -l app.kubernetes.io/name=subgen-orchestrator -f

# 3. Scale up to 5 workers
kubectl scale statefulset subgen-worker --replicas=5 -n media

# Expected logs (within 30 seconds):
# INFO: Worker added: subgen-worker-3
# INFO: Worker added: subgen-worker-4
# INFO: Discovered 5 workers from K8s

# 4. Scale down to 3 workers
kubectl scale statefulset subgen-worker --replicas=3 -n media

# Expected logs (within 60 seconds):
# INFO: Worker removed: subgen-worker-4
# INFO: Worker removed: subgen-worker-3
# INFO: Discovered 3 workers from K8s
```

---

## Edge Cases

### 1. Watch Disconnects

**Cause**: Network issue, K8s API restart  
**Handling**: Reconnect with 5-second backoff, fall back to periodic refresh

### 2. Duplicate Events

**Cause**: K8s sends Modified events frequently  
**Handling**: Compare with existing state, only update if changed

### 3. Out-of-Order Events

**Cause**: Network delays  
**Handling**: Use LastSeen timestamp, prefer newer events

### 4. Rapid Scaling

**Cause**: User scales up/down quickly  
**Handling**: Process all events sequentially, pool handles race conditions with mutex

---

## Metrics

```go
// Watch metrics
subgen_worker_watch_events_total{type="added"} 5
subgen_worker_watch_events_total{type="removed"} 2
subgen_worker_watch_events_total{type="updated"} 100
subgen_worker_watch_errors_total 1
subgen_worker_watch_reconnects_total 2
```

---

## Files to Modify

- `orchestrator/internal/discovery/kubernetes.go` - Watch implementation
- `orchestrator/internal/discovery/kubernetes_test.go` - Tests
- `orchestrator/internal/discovery/pool.go` - Reconnection logic
- `orchestrator/internal/discovery/metrics.go` - Watch metrics

---

## Definition of Done

- [ ] Watch implementation complete
- [ ] All event types handled
- [ ] Reconnection logic working
- [ ] Unit tests passing
- [ ] Integration tests passing
- [ ] Manual testing validated (scale up/down)
- [ ] Metrics implemented
- [ ] Logging comprehensive
- [ ] Work log created

---

**Story Owner:** TBD  
**Created:** 2026-02-17
