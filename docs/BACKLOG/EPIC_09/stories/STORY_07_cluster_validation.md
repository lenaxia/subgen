# STORY_07: Real Cluster Integration Testing & Validation

**Epic:** EPIC_09  
**Status:** Not Started  
**Assignee:** External Validation  
**Effort:** 3-4 hours  
**Priority:** HIGH - Required to validate all Phase 2 work  
**Dependencies:** STORY_01, STORY_02, STORY_03, STORY_06A, STORY_06B (all complete)

---

## User Story

As a **platform engineer with a real Kubernetes cluster**,  
I want to **validate all Phase 2 implementations in a live environment**,  
So that **we can confirm everything works before marking Epic 9 complete**.

---

## Scope

This story validates ALL previous stories (01-03, 06A, 06B) in a real Kubernetes cluster. This is **integration testing** that cannot be done without actual K8s infrastructure.

**What This Tests:**
- K8s worker discovery (STORY_01)
- RBAC permissions (STORY_02)
- Watch API real-time updates (STORY_03)
- Worker HTTP health probes (STORY_06A)
- Orchestrator HTTP health probes (STORY_06B)

**What This Does NOT Test:**
- Load balancing (covered by STORY_05)
- Code functionality (covered by unit tests)

---

## Prerequisites

### Required Infrastructure
- [ ] Kubernetes cluster (any of):
  - kind (local)
  - minikube (local)
  - k3s (local)
  - EKS/GKE/AKS (cloud)
  - Self-hosted K8s
- [ ] kubectl installed and configured
- [ ] Helm 3 installed
- [ ] Container images available:
  - `ghcr.io/your-username/subgen-orchestrator:latest`
  - `ghcr.io/your-username/subgen-worker:latest`

### Required Files (Already Created)
- [ ] `deploy/rbac.yaml`
- [ ] `deploy/values-phase2-orchestrator.yaml`
- [ ] `deploy/values-phase2-workers.yaml`

### Access Requirements
- [ ] Cluster admin access (for RBAC creation)
- [ ] Ability to create namespace
- [ ] Ability to expose LoadBalancer or NodePort services

---

## Test Plan

### Phase 1: Cluster Setup & RBAC (STORY_02 Validation)

#### 1.1 Create Namespace
```bash
kubectl create namespace media
kubectl config set-context --current --namespace=media
```

**Expected**: Namespace created successfully

#### 1.2 Apply RBAC
```bash
kubectl apply -f deploy/rbac.yaml
```

**Expected Output**:
```
serviceaccount/subgen-orchestrator created
role.rbac.authorization.k8s.io/subgen-orchestrator created
rolebinding.rbac.authorization.k8s.io/subgen-orchestrator created
```

#### 1.3 Verify RBAC Permissions
```bash
# Should all return "yes"
kubectl auth can-i get endpoints \
  --as=system:serviceaccount:media:subgen-orchestrator \
  -n media

kubectl auth can-i list endpoints \
  --as=system:serviceaccount:media:subgen-orchestrator \
  -n media

kubectl auth can-i watch endpoints \
  --as=system:serviceaccount:media:subgen-orchestrator \
  -n media

# Should return "no" (read-only permissions)
kubectl auth can-i delete endpoints \
  --as=system:serviceaccount:media:subgen-orchestrator \
  -n media
```

**Expected**: First 3 return "yes", last returns "no"

**Acceptance Criterion**: ✅ RBAC permissions correctly scoped (read-only)

---

### Phase 2: Worker Deployment (STORY_06A Validation)

#### 2.1 Deploy Workers
```bash
# Add bjw-s Helm repo
helm repo add bjw-s https://bjw-s-labs.github.io/helm-charts
helm repo update

# Deploy workers (start with 1)
helm install subgen-worker bjw-s/app-template \
  --namespace media \
  --values deploy/values-phase2-workers.yaml \
  --set controllers.main.replicas=1
```

**Expected**: Helm release created

#### 2.2 Wait for Worker Startup
```bash
kubectl wait --for=condition=ready pod \
  -l app.kubernetes.io/name=subgen-worker \
  -n media \
  --timeout=180s
```

**Expected**: Pod becomes Ready within 180s

#### 2.3 Check Worker Probes
```bash
# Check pod details
kubectl describe pod -l app.kubernetes.io/name=subgen-worker -n media

# Look for successful probes
kubectl get events -n media --field-selector involvedObject.kind=Pod | grep -i probe
```

**Expected**: 
- Liveness probe: Success (HTTP /health on port 8080)
- Readiness probe: Success (HTTP /ready on port 8080)
- Startup probe: Success (HTTP /health on port 8080)

#### 2.4 Test Worker Health Endpoints
```bash
# Port-forward to worker
kubectl port-forward -n media svc/subgen-worker-main 8080:8080 &
PF_PID=$!

# Test /health
curl -f http://localhost:8080/health
# Expected: {"status":"alive","timestamp":...}

# Test /ready  
curl -f http://localhost:8080/ready
# Expected: {"status":"ready","memory_mb":...,"jobs_active":0,...}

# Test /metrics
curl -s http://localhost:8080/metrics | jq
# Expected: Full JSON with jobs_active, memory_mb, model_loaded, etc.

# Verify jobs_active exists
curl -s http://localhost:8080/metrics | jq -e '.jobs_active == 0'
# Expected: true (exit code 0)

# Kill port-forward
kill $PF_PID
```

**Expected**: All endpoints return 200, jobs_active field exists

**Acceptance Criterion**: ✅ Worker HTTP health server works, exposes jobs_active

---

### Phase 3: Orchestrator Deployment (STORY_01, STORY_06B Validation)

#### 3.1 Deploy Orchestrator
```bash
# Create secrets (use dummy values for testing)
kubectl create secret generic subgen-secrets \
  --namespace media \
  --from-literal=PLEX_TOKEN='dummy-token' \
  --from-literal=JELLYFIN_TOKEN='dummy-token'

# Deploy orchestrator
helm install subgen-orchestrator bjw-s/app-template \
  --namespace media \
  --values deploy/values-phase2-orchestrator.yaml
```

**Expected**: Helm release created

#### 3.2 Wait for Orchestrator Startup
```bash
kubectl wait --for=condition=ready pod \
  -l app.kubernetes.io/name=subgen-orchestrator \
  -n media \
  --timeout=60s
```

**Expected**: Pod becomes Ready within 60s

#### 3.3 Check Orchestrator Logs for Discovery
```bash
kubectl logs -n media -l app.kubernetes.io/name=subgen-orchestrator --tail=50
```

**Expected Log Messages**:
```
level=info msg="Kubernetes discovery initialized successfully (running in K8s cluster)"
level=info msg="Starting Kubernetes endpoints watch for dynamic worker discovery"
level=info msg="Kubernetes watch established successfully"
level=info msg="Discovered workers from K8s Endpoints API" count=1 namespace=media service=subgen-worker
```

**Acceptance Criterion**: ✅ Orchestrator discovers worker via K8s API

#### 3.4 Test Orchestrator Health Endpoints
```bash
# Port-forward to orchestrator
kubectl port-forward -n media svc/subgen-orchestrator-main 9000:9000 &
PF_PID=$!

# Test original endpoints
curl -f http://localhost:9000/health
# Expected: {"status":"alive",...}

curl -f http://localhost:9000/ready
# Expected: {"status":"ready","workers_available":1,...}

curl -f http://localhost:9000/live
# Expected: {"status":"alive","uptime_seconds":...}

# Test K8s aliases
curl -f http://localhost:9000/healthz
# Expected: Same as /health

curl -f http://localhost:9000/readyz
# Expected: Same as /ready

curl -f http://localhost:9000/livez
# Expected: Same as /live

# Kill port-forward
kill $PF_PID
```

**Expected**: All endpoints return 200, /readyz shows workers_available

**Acceptance Criterion**: ✅ Orchestrator health checks work, K8s aliases function

---

### Phase 4: Worker Scaling & Watch API (STORY_03 Validation)

#### 4.1 Scale Workers Up (1 → 3)
```bash
# Record timestamp
echo "Scaling up at $(date +%H:%M:%S)"

# Scale to 3 workers
kubectl scale statefulset subgen-worker --replicas=3 -n media

# Immediately watch orchestrator logs
kubectl logs -n media -l app.kubernetes.io/name=subgen-orchestrator -f --tail=20
```

**Expected Log Messages (within 2-5 seconds)**:
```
level=info msg="New worker discovered" worker_id=worker-1 address=10.42.1.5:50051
level=info msg="New worker discovered" worker_id=worker-2 address=10.42.1.6:50051
level=info msg="Worker added" worker_id=worker-1
level=info msg="Worker added" worker_id=worker-2
```

**Timing Check**: Note how long it takes from `kubectl scale` to "Worker added" logs
- Target: < 5 seconds (watch API)
- Acceptable: < 30 seconds (periodic refresh)

#### 4.2 Verify All Workers Discovered
```bash
# Wait for pods to be ready
kubectl wait --for=condition=ready pod \
  -l app.kubernetes.io/name=subgen-worker \
  -n media \
  --timeout=180s

# Check worker count in logs
kubectl logs -n media -l app.kubernetes.io/name=subgen-orchestrator --tail=100 | \
  grep -i "Discovered workers" | tail -1
```

**Expected**: "Discovered 3 workers" or similar

#### 4.3 Scale Workers Down (3 → 1)
```bash
# Record timestamp
echo "Scaling down at $(date +%H:%M:%S)"

# Scale to 1 worker
kubectl scale statefulset subgen-worker --replicas=1 -n media

# Watch orchestrator logs
kubectl logs -n media -l app.kubernetes.io/name=subgen-orchestrator -f --tail=20
```

**Expected Log Messages (within 2-5 seconds)**:
```
level=info msg="Worker removed from endpoints" worker_id=worker-1
level=info msg="Worker removed from endpoints" worker_id=worker-2
level=info msg="Worker removed" worker_id=worker-1
level=info msg="Worker removed" worker_id=worker-2
```

**Acceptance Criterion**: ✅ Watch API detects scaling events in < 5 seconds

---

### Phase 5: Readiness During Worker Failures

#### 5.1 Test Orchestrator Readiness with Workers
```bash
# Port-forward to orchestrator
kubectl port-forward -n media svc/subgen-orchestrator-main 9000:9000 &
PF_PID=$!

# Check readiness (should be ready)
curl -s http://localhost:9000/readyz | jq
# Expected: {"status":"ready","workers_available":1,...}
```

#### 5.2 Scale Workers to Zero
```bash
kubectl scale statefulset subgen-worker --replicas=0 -n media

# Wait for pods to terminate
sleep 10

# Check readiness (should be NOT ready)
curl -s http://localhost:9000/readyz
# Expected: HTTP 503
# Expected body: {"status":"not_ready","reason":"no_workers_available"}
```

#### 5.3 Restore Workers
```bash
kubectl scale statefulset subgen-worker --replicas=1 -n media

# Wait for worker to be ready
kubectl wait --for=condition=ready pod \
  -l app.kubernetes.io/name=subgen-worker \
  -n media \
  --timeout=180s

# Wait for discovery (give watch API time)
sleep 5

# Check readiness (should be ready again)
curl -s http://localhost:9000/readyz | jq
# Expected: {"status":"ready","workers_available":1,...}

# Kill port-forward
kill $PF_PID
```

**Acceptance Criterion**: ✅ Readiness accurately reflects worker availability

---

### Phase 6: Watch Metrics Validation

#### 6.1 Check Watch Metrics
```bash
# Port-forward to orchestrator metrics
kubectl port-forward -n media svc/subgen-orchestrator-main 9090:9090 &
PF_PID=$!

# Get Prometheus metrics
curl -s http://localhost:9090/metrics | grep -E "subgen_worker_watch|subgen_worker_discovery"
```

**Expected Metrics**:
```
# Watch events
subgen_worker_watch_events_total{type="added"} 3
subgen_worker_watch_events_total{type="removed"} 2

# Reconnection count (should be 0 or very low)
subgen_worker_watch_reconnects_total 0

# Discovery errors (should be 0)
subgen_worker_discovery_errors_total 0

# Worker count
subgen_worker_count{status="healthy"} 1
```

**Acceptance Criterion**: ✅ Watch metrics are exposed and accurate

---

### Phase 7: RBAC Validation (Negative Tests)

#### 7.1 Test Without RBAC
```bash
# Delete RBAC
kubectl delete rolebinding subgen-orchestrator -n media

# Wait a moment
sleep 5

# Check orchestrator logs for RBAC errors
kubectl logs -n media -l app.kubernetes.io/name=subgen-orchestrator --tail=50 | \
  grep -i "forbidden\|permission denied\|RBAC"
```

**Expected**: Errors mentioning "forbidden: endpoints" or RBAC

#### 7.2 Restore RBAC
```bash
kubectl apply -f deploy/rbac.yaml

# Restart orchestrator to pick up permissions
kubectl rollout restart deployment subgen-orchestrator -n media

# Wait for pod to be ready
kubectl wait --for=condition=ready pod \
  -l app.kubernetes.io/name=subgen-orchestrator \
  -n media \
  --timeout=60s

# Check logs - should see successful discovery
kubectl logs -n media -l app.kubernetes.io/name=subgen-orchestrator --tail=20
```

**Expected**: No more RBAC errors, worker discovery succeeds

**Acceptance Criterion**: ✅ RBAC is required and sufficient for discovery

---

## Acceptance Criteria Summary

### Must Pass (Critical)
- [x] RBAC permissions correctly configured (read-only endpoints)
- [x] Worker HTTP health server responding on port 8080
- [x] Worker /metrics includes `jobs_active` field
- [x] Orchestrator discovers workers via K8s API
- [x] Orchestrator K8s aliases work (/healthz, /livez, /readyz)
- [x] Watch API detects worker scaling in < 5 seconds
- [x] Orchestrator readiness reflects worker availability

### Should Pass (Important)
- [x] Worker probes pass (liveness, readiness, startup)
- [x] Orchestrator probes pass
- [x] Watch metrics exposed and accurate
- [x] No RBAC errors in logs (after applying rbac.yaml)
- [x] Pods remain stable (no crash loops)

### Nice to Have (Optional)
- [ ] Watch reconnection count is 0 (no disconnections)
- [ ] All operations complete in expected timeframes
- [ ] Resource usage is reasonable (no memory leaks)

---

## Expected Test Duration

- **Phase 1** (Setup): 5 minutes
- **Phase 2** (Workers): 10 minutes
- **Phase 3** (Orchestrator): 10 minutes
- **Phase 4** (Scaling): 15 minutes
- **Phase 5** (Readiness): 10 minutes
- **Phase 6** (Metrics): 5 minutes
- **Phase 7** (RBAC): 10 minutes

**Total**: ~65 minutes (1 hour)

---

## Troubleshooting Guide

### Issue: Pods Stuck in Pending
```bash
kubectl describe pod <pod-name> -n media
# Look for events like "Insufficient CPU" or "No nodes available"
```

**Fix**: Adjust resource requests in values file or add cluster capacity

### Issue: Image Pull Errors
```bash
kubectl describe pod <pod-name> -n media | grep -A 5 "Failed to pull image"
```

**Fix**: Verify images exist in registry, update image repository/tag

### Issue: RBAC Forbidden Errors
```bash
kubectl logs -n media -l app.kubernetes.io/name=subgen-orchestrator | grep -i forbidden
```

**Fix**: Ensure `deploy/rbac.yaml` is applied and ServiceAccount is set in values

### Issue: Workers Not Discovered
```bash
# Check if endpoints exist
kubectl get endpoints -n media subgen-worker -o yaml

# Check if orchestrator can access them
kubectl exec -n media -it <orchestrator-pod> -- wget -O- http://subgen-worker-main:8080/health
```

**Fix**: Verify service selector matches pod labels, check network policies

### Issue: Health Probes Failing
```bash
kubectl describe pod <pod-name> -n media | grep -A 5 "Liveness\|Readiness"
```

**Fix**: Check if health server is running, verify port configuration

---

## Cleanup

```bash
# Uninstall Helm releases
helm uninstall subgen-orchestrator -n media
helm uninstall subgen-worker -n media

# Delete RBAC
kubectl delete -f deploy/rbac.yaml

# Delete secrets
kubectl delete secret subgen-secrets -n media

# Optional: Delete namespace
kubectl delete namespace media
```

---

## Deliverables

### Test Report Document

Create: `docs/WORKLOGS/00XX_YYYY-MM-DD_story_07_cluster_validation.md`

**Include**:
1. **Environment Details**:
   - Cluster type (kind/minikube/EKS/etc.)
   - Kubernetes version
   - Node count and specs
   - Container runtime

2. **Test Results Table**:
   ```markdown
   | Phase | Test | Expected | Actual | Pass/Fail | Notes |
   |-------|------|----------|--------|-----------|-------|
   | 1.1   | Create namespace | Success | Success | PASS | |
   | 1.2   | Apply RBAC | 3 resources created | 3 resources created | PASS | |
   | ...   | ... | ... | ... | ... | ... |
   ```

3. **Timing Measurements**:
   - Worker scale-up detection time
   - Worker scale-down detection time
   - Pod startup times

4. **Screenshots/Logs** (Optional):
   - Watch API logs showing real-time discovery
   - Metrics output
   - Probe success events

5. **Issues Found**:
   - List any failures or unexpected behavior
   - Include error messages and logs
   - Suggest fixes or improvements

6. **Overall Assessment**:
   - Are all acceptance criteria met?
   - Is Phase 2 production-ready?
   - Recommendations for improvement

---

## Success Criteria

**Story is COMPLETE when**:
- [ ] All "Must Pass" acceptance criteria pass
- [ ] Test report document created
- [ ] No critical issues found (or all issues resolved)
- [ ] Screenshots/logs provided as evidence
- [ ] Validation handed back to original LLM

**Story FAILS if**:
- Any "Must Pass" criteria fails
- Critical bugs discovered that block production use
- RBAC doesn't work correctly
- Workers not discovered
- Watch API doesn't detect scaling

---

## Notes for Validator

### What This Story Validates

This story validates **everything** we've implemented in STORY_01-03 and STORY_06A-B:

1. **STORY_01**: K8s discovery works, orchestrator finds workers via Endpoints API
2. **STORY_02**: RBAC permissions are correct and sufficient
3. **STORY_03**: Watch API detects scaling in real-time (< 5s)
4. **STORY_06A**: Worker HTTP health server works in real K8s probes
5. **STORY_06B**: Orchestrator health aliases work in real K8s probes

### What to Focus On

**Critical**:
- Does discovery work?
- Does watch API work?
- Do health probes work?

**Important**:
- Timing measurements (how fast is watch?)
- Metrics accuracy
- Log quality

**Nice-to-have**:
- Resource usage
- Stability over time
- Edge case behavior

### Expected Failures

**It's OK if these fail** (nice-to-have):
- Watch reconnection count > 0 (K8s API restarts are normal)
- Slower timing on resource-constrained clusters
- Minor YAML style warnings

**It's NOT OK if these fail** (critical):
- Worker discovery doesn't work
- RBAC errors persist after applying rbac.yaml
- Health probes never succeed
- Watch API doesn't detect scaling at all

---

**Story Created**: 2026-02-17  
**Created By**: OpenCode AI  
**For**: External validation in real K8s cluster  
**Status**: Ready for handoff
