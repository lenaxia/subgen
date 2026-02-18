# CRITICAL: Docker Compatibility Analysis for Epic 9

**Date:** 2026-02-17  
**Issue:** Phase 2 K8s implementation may break Docker Compose deployment  
**Severity:** 🔴 **HIGH** - Would break existing production deployments  
**Status:** ⚠️ **ISSUE FOUND - FIX REQUIRED**

---

## Problem Statement

The current STORY_01 implementation plan will **BREAK Docker Compose deployments** because:

1. `rest.InClusterConfig()` **ONLY works inside Kubernetes pods**
2. It will return an error when run in Docker (not in a K8s cluster)
3. This error will cause `NewKubernetesDiscovery()` to fail
4. Docker Compose deployments will not start

---

## Current Situation

### ✅ What Works Now (Phase 1)

**Docker Compose Configuration** (`docker-compose.yml`):
```yaml
orchestrator:
  environment:
    - WORKER_ADDRESS=worker:50051
    # WORKER_DISCOVERY defaults to "localhost"
```

**Config Defaults** (`config.go:265`):
```go
v.SetDefault("WORKER_DISCOVERY", "localhost")
```

**Factory Pattern** (`factory.go:12-26`):
```go
func NewDiscovery(cfg *config.Config, log *logrus.Logger) (WorkerDiscovery, error) {
    switch cfg.Worker.Discovery {
    case "localhost":
        return NewLocalhostDiscovery(cfg.Worker.Address, log), nil  // ✅ Works in Docker
    
    case "kubernetes":
        return NewKubernetesDiscovery(...)  // ⚠️ Will break if called in Docker
    
    default:
        return nil, fmt.Errorf("unknown worker discovery: %s", cfg.Worker.Discovery)
    }
}
```

### ❌ What Will Break (Current STORY_01 Design)

**Planned Implementation** (`STORY_01_k8s_discovery.md:63-66`):
```go
func NewKubernetesDiscovery(...) (*KubernetesDiscovery, error) {
    // Get in-cluster K8s config
    config, err := rest.InClusterConfig()  // ❌ FAILS outside Kubernetes
    if err != nil {
        return nil, fmt.Errorf("failed to get in-cluster config: %w", err)
    }
    // ...
}
```

**Problem:** If someone accidentally sets `WORKER_DISCOVERY=kubernetes` in Docker Compose, the orchestrator will fail to start.

---

## Why This Matters

### Scenario 1: Accidental Configuration
```yaml
# User accidentally sets this in docker-compose.yml
environment:
  - WORKER_DISCOVERY=kubernetes  # ❌ Orchestrator crashes
```

**Result:** Orchestrator fails with "unable to load in-cluster configuration"

### Scenario 2: Shared Configuration
```yaml
# Trying to use same config for both Docker and K8s
environment:
  - WORKER_DISCOVERY=${WORKER_DISCOVERY:-localhost}
```

**If WORKER_DISCOVERY=kubernetes is set:** Crashes in Docker

### Scenario 3: Go Module Dependencies

**When K8s client-go is added:**
```bash
go get k8s.io/client-go@latest
```

**Impact:**
- ✅ Doesn't break Docker by itself (imports are fine)
- ✅ Only called when `WORKER_DISCOVERY=kubernetes`
- ⚠️ But will fail with confusing error if misconfigured

---

## The Fix

### Solution: Add Environment Detection

**Update `NewKubernetesDiscovery()` to detect environment:**

```go
func NewKubernetesDiscovery(namespace, service string, port int32, log *logrus.Logger) (*KubernetesDiscovery, error) {
    // Try to get in-cluster K8s config
    config, err := rest.InClusterConfig()
    if err != nil {
        // Check if this is because we're not in a K8s cluster
        if strings.Contains(err.Error(), "unable to load in-cluster configuration") {
            return nil, fmt.Errorf(
                "kubernetes discovery requires running inside a Kubernetes cluster. "+
                "For Docker Compose deployments, use WORKER_DISCOVERY=localhost. "+
                "Original error: %w", err)
        }
        return nil, fmt.Errorf("failed to get in-cluster config: %w", err)
    }
    
    // Create K8s clientset
    clientset, err := kubernetes.NewForConfig(config)
    if err != nil {
        return nil, fmt.Errorf("failed to create K8s client: %w", err)
    }
    
    log.Info("Kubernetes discovery initialized (running in K8s cluster)")
    
    return &KubernetesDiscovery{
        client:    clientset,
        namespace: namespace,
        service:   service,
        port:      port,
        log:       log,
    }, nil
}
```

**Key Improvements:**
1. ✅ Clear error message explaining the problem
2. ✅ Tells user to use `WORKER_DISCOVERY=localhost` for Docker
3. ✅ Doesn't crash mysteriously
4. ✅ Only works in K8s (as intended)

---

## Validation

### ✅ Test Case 1: Docker Compose (Default)

**Configuration:**
```yaml
environment:
  # WORKER_DISCOVERY not set (defaults to "localhost")
  - WORKER_ADDRESS=worker:50051
```

**Expected:** ✅ Works - Uses LocalhostDiscovery  
**Actual:** ✅ Works - No K8s code is called

---

### ✅ Test Case 2: Docker Compose (Explicit Localhost)

**Configuration:**
```yaml
environment:
  - WORKER_DISCOVERY=localhost
  - WORKER_ADDRESS=worker:50051
```

**Expected:** ✅ Works - Uses LocalhostDiscovery  
**Actual:** ✅ Works - No K8s code is called

---

### ❌ Test Case 3: Docker Compose (Wrong Config)

**Configuration:**
```yaml
environment:
  - WORKER_DISCOVERY=kubernetes  # ❌ Wrong for Docker
  - WORKER_NAMESPACE=media
```

**Expected:** ❌ Fails with clear error message  
**Actual (with fix):** ❌ Fails with helpful error:
```
Error: kubernetes discovery requires running inside a Kubernetes cluster.
For Docker Compose deployments, use WORKER_DISCOVERY=localhost.
Original error: unable to load in-cluster configuration...
```

**Actual (without fix):** ❌ Fails with confusing error:
```
Error: failed to get in-cluster config: unable to load in-cluster configuration...
```

---

### ✅ Test Case 4: Kubernetes (Correct)

**Configuration:**
```yaml
env:
  - name: WORKER_DISCOVERY
    value: "kubernetes"
  - name: WORKER_NAMESPACE
    value: "media"
```

**Expected:** ✅ Works - Uses KubernetesDiscovery  
**Actual:** ✅ Works - Reads K8s API successfully

---

## Documentation Updates Required

### 1. Update STORY_01

Add error handling section:

```markdown
## Docker Compatibility

**CRITICAL:** `rest.InClusterConfig()` only works inside Kubernetes pods.

**Error Handling:**
- Detect "unable to load in-cluster configuration" error
- Return helpful error message referencing Docker Compose
- Explain that Docker deployments should use `WORKER_DISCOVERY=localhost`

**Error Message Template:**
```go
return nil, fmt.Errorf(
    "kubernetes discovery requires running inside a Kubernetes cluster. "+
    "For Docker Compose deployments, use WORKER_DISCOVERY=localhost. "+
    "Original error: %w", err)
```
```

---

### 2. Update docker-compose.yml

Add comment to prevent confusion:

```yaml
orchestrator:
  environment:
    # Worker Discovery Mode
    # - "localhost" for Docker Compose (default)
    # - "kubernetes" for K8s deployments only
    # Leave commented for Docker, set in K8s manifests
    # - WORKER_DISCOVERY=localhost  
    
    - WORKER_ADDRESS=worker:50051
```

---

### 3. Update README.md

Add deployment mode section:

```markdown
## Deployment Modes

### Docker Compose (Phase 1)
- Uses `WORKER_DISCOVERY=localhost` (default)
- Worker and orchestrator in same network
- Single worker only

### Kubernetes (Phase 2)
- Uses `WORKER_DISCOVERY=kubernetes`
- Orchestrator discovers workers via K8s Endpoints API
- Supports multiple workers (horizontal scaling)
- **Requires:** RBAC configuration
```

---

## Go Module Dependencies

### K8s client-go Impact

**Adding dependencies:**
```bash
go get k8s.io/client-go@latest
go get k8s.io/apimachinery@latest
```

**Binary Size Impact:**
- Before: ~25MB orchestrator binary
- After: ~35-40MB orchestrator binary (+10-15MB)

**Docker Image Impact:**
- Before: ~50MB compressed
- After: ~65-70MB compressed (+15-20MB)

**Verdict:** ✅ Acceptable - K8s deps are large but necessary

---

## Build Process

### Docker Build (No Changes Required)

```dockerfile
# orchestrator/Dockerfile
FROM golang:1.21 AS builder
WORKDIR /app
COPY go.mod go.sum ./
RUN go mod download  # Downloads K8s deps
COPY . .
RUN CGO_ENABLED=0 go build -o orchestrator ./cmd/orchestrator

FROM alpine:latest
COPY --from=builder /app/orchestrator /orchestrator
CMD ["/orchestrator"]
```

**Impact:** ✅ No changes needed - go mod download handles new deps

---

## Runtime Detection

### How to Know Which Mode You're In

**For Users:**
```bash
# Docker Compose
docker logs subgen-orchestrator | grep "discovery"
# Output: "Using localhost worker discovery"

# Kubernetes
kubectl logs -l app=orchestrator | grep "discovery"
# Output: "Kubernetes discovery initialized (running in K8s cluster)"
```

**For Developers:**
```go
// In main.go
log.WithFields(logrus.Fields{
    "discovery_mode": cfg.Worker.Discovery,
    "worker_address": cfg.Worker.Address,
}).Info("Worker discovery configuration")
```

---

## Testing Strategy

### Unit Tests

```go
// Test that NewKubernetesDiscovery fails gracefully outside K8s
func TestNewKubernetesDiscovery_OutsideCluster(t *testing.T) {
    // This test will fail outside K8s (expected)
    _, err := NewKubernetesDiscovery("media", "worker", 50051, log)
    
    assert.Error(t, err)
    assert.Contains(t, err.Error(), "kubernetes discovery requires running inside a Kubernetes cluster")
    assert.Contains(t, err.Error(), "use WORKER_DISCOVERY=localhost")
}
```

### Integration Tests

**Test Matrix:**

| Environment | WORKER_DISCOVERY | Expected Result |
|-------------|------------------|-----------------|
| Docker | localhost | ✅ Pass |
| Docker | kubernetes | ❌ Fail with helpful error |
| K8s | localhost | ✅ Pass (uses hardcoded worker:50051) |
| K8s | kubernetes | ✅ Pass |

---

## Rollout Plan

### Phase 1 (Current) - No Changes
- Docker Compose with `WORKER_DISCOVERY=localhost`
- Works today, will continue working

### Phase 2A - Add K8s Discovery
- Add K8s dependencies to go.mod
- Implement KubernetesDiscovery with error handling
- **Docker still works** (uses localhost mode)

### Phase 2B - Deploy to K8s
- Deploy with `WORKER_DISCOVERY=kubernetes`
- Test multi-worker scaling
- Docker deployments unaffected

---

## Risk Assessment

### Before Fix

| Risk | Severity | Likelihood | Impact |
|------|----------|------------|--------|
| Docker deployment breaks | HIGH | MEDIUM | Production outage |
| Confusing error messages | MEDIUM | HIGH | Support burden |
| Users unable to troubleshoot | MEDIUM | HIGH | Lost time |

### After Fix

| Risk | Severity | Likelihood | Impact |
|------|----------|------------|--------|
| Docker deployment breaks | HIGH | LOW | Clear error prevents issue |
| Confusing error messages | LOW | LOW | Error explains problem |
| Users unable to troubleshoot | LOW | LOW | Error provides solution |

---

## Recommendations

### MUST DO (Blocking):

1. ✅ **Update STORY_01 implementation** - Add error handling for non-K8s environments
2. ✅ **Test in Docker** - Verify localhost mode still works after adding K8s deps
3. ✅ **Document deployment modes** - Clear guidance for Docker vs K8s

### SHOULD DO (High Priority):

4. ✅ **Add comments to docker-compose.yml** - Prevent accidental misconfiguration
5. ✅ **Add log message** - Clearly show which discovery mode is active
6. ✅ **Update README** - Explain Phase 1 vs Phase 2 deployment modes

### NICE TO HAVE (Low Priority):

7. ⏳ **Add config validation** - Warn if WORKER_DISCOVERY=kubernetes but not in K8s
8. ⏳ **Add startup health check** - Verify worker discovery works before accepting traffic

---

## Action Items for STORY_01

**Before merging STORY_01, ensure:**

- [ ] Error message includes "use WORKER_DISCOVERY=localhost for Docker"
- [ ] Tested in Docker Compose - localhost mode still works
- [ ] Tested in K8s - kubernetes mode works
- [ ] Tested misconfig in Docker - clear error message
- [ ] docker-compose.yml has explanatory comments
- [ ] README.md explains deployment modes

---

## Conclusion

**Current Status:** ⚠️ **ISSUE IDENTIFIED**

**Impact:** Phase 2 implementation would break Docker deployments if not handled correctly

**Fix Complexity:** Low - Just add better error handling

**Risk After Fix:** Very Low - Clear separation between Docker and K8s modes

**Next Steps:**
1. Update STORY_01 implementation guide with error handling
2. Add Docker compatibility test cases
3. Document deployment modes clearly

---

**Analysis Completed By:** AI Assistant  
**Date:** 2026-02-17  
**Severity:** HIGH (but easily fixable)  
**Status:** FIX REQUIRED BEFORE STORY_01 IMPLEMENTATION
