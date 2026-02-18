# Phase 1 to Phase 2 Migration Guide

**Document Version:** 1.0  
**Last Updated:** 2026-02-17  
**Status:** Final  
**Related Documents:**
- [03_SCALING_STRATEGY.md](../DESIGN/03_SCALING_STRATEGY.md)
- [04_K8S_DEPLOYMENT.md](../DESIGN/04_K8S_DEPLOYMENT.md)
- [EPIC_09 README](../BACKLOG/EPIC_09/README.md)

---

## Table of Contents

1. [Overview](#overview)
2. [Pre-Migration Checklist](#pre-migration-checklist)
3. [Migration Strategies](#migration-strategies)
4. [Step-by-Step Migration (Zero-Downtime)](#step-by-step-migration-zero-downtime)
5. [Rollback Procedure](#rollback-procedure)
6. [Validation Steps](#validation-steps)
7. [Troubleshooting](#troubleshooting)

---

## Overview

### What Changes in Phase 2?

| Aspect | Phase 1 | Phase 2 |
|--------|---------|---------|
| **Pod Structure** | Single pod (orchestrator + worker) | Separate deployments |
| **Worker Count** | 1 (fixed) | 1-N (scalable) |
| **Worker Discovery** | `WORKER_DISCOVERY=localhost` | `WORKER_DISCOVERY=kubernetes` |
| **Service Architecture** | Internal communication only | Worker service (ClusterIP) |
| **RBAC** | Not required | ServiceAccount + Role + RoleBinding |
| **Health Checks** | Basic | Enhanced (HTTP probes) |
| **Scaling** | Manual pod restart | `kubectl scale` or HPA |

### Migration Timeline

**Estimated duration**: 30-45 minutes (zero downtime)  
**Recommended window**: During low-traffic period (but not required)

---

## Pre-Migration Checklist

### 1. Verify Current State

```bash
# Check Phase 1 deployment is healthy
kubectl get pods -n media -l app.kubernetes.io/name=subgen
kubectl logs -n media -l app.kubernetes.io/name=subgen --tail=50

# Verify tasks are completing
kubectl logs -n media -l app.kubernetes.io/name=subgen | grep "transcription complete"

# Check queue is not full
curl http://subgen.media.svc.cluster.local:9000/queue
```

### 2. Backup Configuration

```bash
# Export current Helm values
helm get values subgen -n media > backup-phase1-values.yaml

# Backup PVCs (if using local storage)
kubectl get pvc -n media -o yaml > backup-pvcs.yaml

# Note current image tags
kubectl get pods -n media -o jsonpath='{.items[0].spec.containers[*].image}'
```

### 3. Prepare Phase 2 Resources

```bash
# Create RBAC resources
kubectl apply -f deploy/rbac.yaml

# Verify RBAC is correct
kubectl auth can-i get endpoints \
  --as=system:serviceaccount:media:subgen-orchestrator \
  -n media
# Should output: yes
```

### 4. Prepare Helm Values

**Create**: `values-phase2-workers.yaml`
**Create**: `values-phase2-orchestrator.yaml`

(See Step-by-Step Migration section for full files)

---

## Migration Strategies

### Strategy A: Blue-Green Deployment (Recommended)

**Pros**:
- ✅ Zero downtime
- ✅ Easy rollback (switch back to old deployment)
- ✅ Test Phase 2 before switching traffic

**Cons**:
- ❌ Requires 2x resources temporarily
- ❌ More complex setup

**Best for**: Production environments, critical workloads

---

### Strategy B: In-Place Migration

**Pros**:
- ✅ Simpler process
- ✅ No extra resources needed

**Cons**:
- ❌ Brief downtime (30-60 seconds)
- ❌ More risk if issues occur

**Best for**: Development/staging, non-critical workloads

---

### Strategy C: Canary Deployment

**Pros**:
- ✅ Gradual rollout (10% → 50% → 100%)
- ✅ Easy to detect issues early

**Cons**:
- ❌ Most complex
- ❌ Requires advanced routing

**Best for**: Large-scale deployments, risk-averse environments

**This guide focuses on Strategy A (Blue-Green)** - recommended for most users.

---

## Step-by-Step Migration (Zero-Downtime)

### Phase 1: Deploy Phase 2 Workers (Blue)

#### Step 1.1: Create Worker Values

**File**: `values-phase2-workers.yaml`

```yaml
controllers:
  main:
    type: statefulset
    replicas: 3  # Start with 3 workers
    
    containers:
      worker:
        image:
          repository: ghcr.io/lenaxia/subgen-worker
          tag: "latest"  # Or specific version
        
        env:
          # Whisper configuration
          TRANSCRIBE_DEVICE: "cuda"
          WHISPER_MODEL: "medium"
          CONCURRENT_TRANSCRIPTIONS: "2"
          MODEL_CLEANUP_DELAY: "300"
          COMPUTE_TYPE: "auto"
        
        ports:
          - name: grpc
            containerPort: 50051
            protocol: TCP
          - name: http-health
            containerPort: 8080
            protocol: TCP
        
        probes:
          liveness:
            enabled: true
            type: HTTP
            port: 8080
            path: /health
            spec:
              initialDelaySeconds: 30
              periodSeconds: 30
              timeoutSeconds: 5
              failureThreshold: 3
          
          readiness:
            enabled: true
            type: HTTP
            port: 8080
            path: /ready
            spec:
              initialDelaySeconds: 10
              periodSeconds: 10
              timeoutSeconds: 3
              failureThreshold: 3
          
          startup:
            enabled: true
            type: HTTP
            port: 8080
            path: /health
            spec:
              initialDelaySeconds: 10
              periodSeconds: 10
              timeoutSeconds: 5
              failureThreshold: 30  # 5 minutes for model download

service:
  main:
    controller: main
    type: ClusterIP
    ports:
      grpc:
        port: 50051
        targetPort: 50051
        protocol: TCP

persistence:
  models:
    enabled: true
    type: persistentVolumeClaim
    existingClaim: subgen-models
    globalMounts:
      - path: /models
  
  media:
    enabled: true
    type: nfs
    server: 192.168.1.10
    path: /mnt/pool/media
    globalMounts:
      - path: /media
        readOnly: true
```

#### Step 1.2: Deploy Workers

```bash
# Deploy workers
helm install subgen-worker bjw-s/app-template \
  --namespace media \
  --values values-phase2-workers.yaml \
  --wait --timeout=300s

# Verify workers are ready
kubectl wait --for=condition=Ready pod \
  -l app.kubernetes.io/name=subgen-worker \
  -n media \
  --timeout=300s

# Check worker health
kubectl run -n media health-check --image=curlimages/curl --rm -i --restart=Never -- \
  curl -s http://subgen-worker:8080/health

echo "✅ Phase 2 workers deployed successfully"
```

---

### Phase 2: Deploy Phase 2 Orchestrator (Blue)

#### Step 2.1: Create Orchestrator Values

**File**: `values-phase2-orchestrator.yaml`

```yaml
defaultPodOptions:
  serviceAccountName: subgen-orchestrator  # NEW: Use RBAC ServiceAccount
  automountServiceAccountToken: true      # NEW: Need K8s API access

controllers:
  main:
    type: deployment
    replicas: 1
    
    containers:
      orchestrator:
        image:
          repository: ghcr.io/lenaxia/subgen-orchestrator
          tag: "latest"
        
        env:
          # Phase 2: Kubernetes discovery
          WORKER_DISCOVERY: "kubernetes"              # CHANGED
          WORKER_SERVICE_NAME: "subgen-worker"        # NEW
          WORKER_NAMESPACE: "media"                   # NEW
          WORKER_PORT: "50051"                        # NEW
          LOAD_BALANCE_STRATEGY: "least_loaded"       # NEW
          
          # Queue configuration
          QUEUE_MAX_SIZE: "1000"
          
          # Webhook ports
          WEBHOOK_PORT: "9000"
          METRICS_PORT: "9090"
          
          # Plex configuration (same as Phase 1)
          PLEX_SERVER: "http://plex.media.svc.cluster.local:32400"
          PLEX_TOKEN: "${PLEX_TOKEN}"
          
          # Skip logic (same as Phase 1)
          SKIP_IF_SYNCED: "true"
          SKIP_IF_EMBEDDED: "true"
          # ... other skip variables ...
        
        ports:
          - name: http
            containerPort: 9000
            protocol: TCP
          - name: metrics
            containerPort: 9090
            protocol: TCP
        
        probes:
          liveness:
            enabled: true
            type: HTTP
            port: 9000
            path: /healthz
            spec:
              initialDelaySeconds: 10
              periodSeconds: 30
              timeoutSeconds: 5
              failureThreshold: 3
          
          readiness:
            enabled: true
            type: HTTP
            port: 9000
            path: /readyz
            spec:
              initialDelaySeconds: 5
              periodSeconds: 10
              timeoutSeconds: 3
              failureThreshold: 3

service:
  main:
    controller: main
    type: LoadBalancer
    ports:
      http:
        port: 9000
        targetPort: 9000
        protocol: TCP
      metrics:
        port: 9090
        targetPort: 9090
        protocol: TCP

persistence:
  media:
    enabled: true
    type: nfs
    server: 192.168.1.10
    path: /mnt/pool/media
    globalMounts:
      - path: /media
        readOnly: true
```

#### Step 2.2: Deploy Orchestrator (Blue)

```bash
# Deploy Phase 2 orchestrator with different name
helm install subgen-orchestrator-blue bjw-s/app-template \
  --namespace media \
  --values values-phase2-orchestrator.yaml \
  --wait --timeout=60s

# Verify orchestrator is ready
kubectl wait --for=condition=Ready pod \
  -l app.kubernetes.io/name=subgen-orchestrator-blue \
  -n media \
  --timeout=60s

# Check orchestrator discovered workers
kubectl logs -n media -l app.kubernetes.io/name=subgen-orchestrator-blue --tail=50 | grep "Discovered"

# Should see: "Discovered 3 workers from K8s"

echo "✅ Phase 2 orchestrator deployed successfully"
```

---

### Phase 3: Validate Phase 2 (Blue)

#### Step 3.1: Test Worker Discovery

```bash
# Check /ready endpoint
kubectl run -n media test-ready --image=curlimages/curl --rm -i --restart=Never -- \
  curl -s http://subgen-orchestrator-blue:9000/ready

# Should return:
# {"status":"ready","workers_total":3,"workers_healthy":3, ...}
```

#### Step 3.2: Test Task Processing

```bash
# Queue a test task
kubectl run -n media test-task --image=curlimages/curl --rm -i --restart=Never -- \
  curl -X POST http://subgen-orchestrator-blue:9000/batch \
  -d '{"path":"/media/test/sample.mp4"}'

# Monitor logs
kubectl logs -n media -l app.kubernetes.io/name=subgen-orchestrator-blue -f

# Should see:
# - "Task queued"
# - "Dispatching task to worker-X"
# - "Transcription complete"
```

#### Step 3.3: Test Health Endpoints

```bash
# Test orchestrator health
curl http://$(kubectl get svc subgen-orchestrator-blue -n media -o jsonpath='{.status.loadBalancer.ingress[0].ip}'):9000/health

# Test worker health
kubectl run -n media test-worker-health --image=curlimages/curl --rm -i --restart=Never -- \
  curl -s http://subgen-worker-0.subgen-worker:8080/health
```

**If all tests pass**, proceed to traffic cutover. **If tests fail**, see Troubleshooting section.

---

### Phase 4: Switch Traffic (Green → Blue)

#### Step 4.1: Update Service Selector

**Option A: kubectl patch (Quick)**

```bash
# Patch LoadBalancer service to point to Phase 2 orchestrator
kubectl patch service subgen -n media --type='json' \
  -p='[{"op": "replace", "path": "/spec/selector/app.kubernetes.io~1name", "value": "subgen-orchestrator-blue"}]'

echo "✅ Traffic switched to Phase 2"
```

**Option B: Update ingress/external configuration**

If using Ingress or external load balancer, update DNS/configuration to point to new service IP.

#### Step 4.2: Verify Traffic Flow

```bash
# Send webhook to production endpoint
curl -X POST http://<YOUR_EXTERNAL_IP>:9000/plex \
  -H "Content-Type: application/json" \
  -d '{...}'  # Real Plex webhook payload

# Check Phase 2 orchestrator received request
kubectl logs -n media -l app.kubernetes.io/name=subgen-orchestrator-blue --tail=20
```

---

### Phase 5: Monitor and Decommission Phase 1

#### Step 5.1: Monitor Phase 2 (15-30 minutes)

```bash
# Watch logs for errors
kubectl logs -n media -l app.kubernetes.io/name=subgen-orchestrator-blue -f

# Monitor metrics
curl http://<ORCHESTRATOR_IP>:9090/metrics | grep subgen_

# Check queue size
curl http://<ORCHESTRATOR_IP>:9000/queue
```

**Watch for**:
- ✅ Tasks completing successfully
- ✅ No error logs
- ✅ Queue draining properly
- ✅ Worker health checks passing

**If issues occur**, see Rollback Procedure section.

#### Step 5.2: Decommission Phase 1

**Only after 30 minutes of successful Phase 2 operation**:

```bash
# Uninstall Phase 1 deployment
helm uninstall subgen -n media

# Rename Phase 2 to production
helm uninstall subgen-orchestrator-blue -n media
helm install subgen-orchestrator bjw-s/app-template \
  --namespace media \
  --values values-phase2-orchestrator.yaml

echo "✅ Migration complete!"
```

---

## Rollback Procedure

### When to Rollback

Rollback immediately if:
- ❌ Workers not discovered (check logs for "Discovered 0 workers")
- ❌ Tasks failing consistently (> 50% failure rate)
- ❌ RBAC errors (check logs for "Forbidden")
- ❌ Orchestrator crashing (pod restarts)
- ❌ Health checks failing

### Quick Rollback (< 2 minutes)

```bash
echo "🔴 Rolling back to Phase 1..."

# Step 1: Switch traffic back to Phase 1
kubectl patch service subgen -n media --type='json' \
  -p='[{"op": "replace", "path": "/spec/selector/app.kubernetes.io~1name", "value": "subgen"}]'

# Step 2: Verify Phase 1 is still running
kubectl get pods -n media -l app.kubernetes.io/name=subgen

# If Phase 1 pod was deleted, redeploy from backup
helm install subgen bjw-s/app-template \
  --namespace media \
  --values backup-phase1-values.yaml

# Step 3: Verify traffic restored
curl http://<EXTERNAL_IP>:9000/health

echo "✅ Rollback complete - Phase 1 restored"

# Step 4: Investigate Phase 2 issues
kubectl logs -n media -l app.kubernetes.io/name=subgen-orchestrator-blue --tail=100
kubectl describe pod -n media -l app.kubernetes.io/name=subgen-orchestrator-blue
```

### Post-Rollback Cleanup

```bash
# Leave Phase 2 resources running for investigation
# DO NOT delete until issues are resolved

# To fully remove Phase 2:
# helm uninstall subgen-orchestrator-blue -n media
# helm uninstall subgen-worker -n media
```

---

## Validation Steps

### Validation Checklist

After migration (or after rollback), verify:

#### Orchestrator Health

- [ ] Pod is running: `kubectl get pods -n media -l app.kubernetes.io/name=subgen-orchestrator`
- [ ] Health endpoint returns 200: `curl http://<IP>:9000/health`
- [ ] Ready endpoint returns 200: `curl http://<IP>:9000/ready`
- [ ] Workers discovered: Check logs for "Discovered X workers"
- [ ] No error logs: `kubectl logs -n media -l app.kubernetes.io/name=subgen-orchestrator --tail=100`

#### Worker Health

- [ ] All worker pods running: `kubectl get pods -n media -l app.kubernetes.io/name=subgen-worker`
- [ ] Health endpoints return 200: `curl http://subgen-worker-0.subgen-worker:8080/health`
- [ ] Ready endpoints return 200: `curl http://subgen-worker-0.subgen-worker:8080/ready`
- [ ] No error logs: `kubectl logs -n media subgen-worker-0 --tail=100`

#### Task Processing

- [ ] Queue test task successfully
- [ ] Task dispatched to worker
- [ ] Transcription completes
- [ ] Subtitle file created
- [ ] Plex metadata refreshed

#### Load Balancing (Phase 2 only)

- [ ] Tasks distributed across workers (check metrics)
- [ ] No single worker overloaded
- [ ] Unhealthy workers not receiving tasks

---

## Troubleshooting

### Issue: Workers Not Discovered

**Symptoms**:
- Logs show "Discovered 0 workers"
- `/ready` endpoint returns 503 with "no_workers_available"

**Diagnosis**:
```bash
# Check if workers are running
kubectl get pods -n media -l app.kubernetes.io/name=subgen-worker

# Check worker service exists
kubectl get svc subgen-worker -n media

# Check endpoints exist
kubectl get endpoints subgen-worker -n media

# Check RBAC permissions
kubectl auth can-i get endpoints \
  --as=system:serviceaccount:media:subgen-orchestrator \
  -n media
```

**Solutions**:

1. **If workers not running**: Deploy workers first
2. **If service missing**: Check Helm values (service section)
3. **If endpoints empty**: Workers not ready yet (wait 2-3 minutes)
4. **If RBAC denied**: Apply `deploy/rbac.yaml`

---

### Issue: RBAC Permission Denied

**Symptoms**:
- Logs show "Forbidden: endpoints ... cannot get resource"

**Solution**:
```bash
# Apply RBAC configuration
kubectl apply -f deploy/rbac.yaml

# Verify
kubectl auth can-i get endpoints \
  --as=system:serviceaccount:media:subgen-orchestrator \
  -n media

# Restart orchestrator pod to pick up new permissions
kubectl rollout restart deployment subgen-orchestrator -n media
```

---

### Issue: Tasks Not Completing

**Symptoms**:
- Tasks queue but never complete
- Worker logs show no activity

**Diagnosis**:
```bash
# Check orchestrator logs
kubectl logs -n media -l app.kubernetes.io/name=subgen-orchestrator --tail=100

# Check worker logs
kubectl logs -n media subgen-worker-0 --tail=100

# Check network connectivity
kubectl run -n media nettest --image=curlimages/curl --rm -i --restart=Never -- \
  curl -v telnet://subgen-worker-0.subgen-worker:50051
```

**Solutions**:

1. **gRPC connection failed**: Check worker service name/namespace in orchestrator env
2. **Worker not processing**: Check worker health with `curl http://worker:8080/ready`
3. **NFS mount issues**: Check media persistence configuration

---

### Issue: Queue Filling Up

**Symptoms**:
- Queue size keeps growing
- `/ready` returns 503 with "queue_full"

**Diagnosis**:
```bash
# Check queue status
curl http://<ORCHESTRATOR_IP>:9000/queue

# Check worker active jobs
curl http://<ORCHESTRATOR_IP>:9090/metrics | grep subgen_worker_active_jobs
```

**Solutions**:

1. **Workers slow**: Scale up workers (`kubectl scale statefulset subgen-worker --replicas=5`)
2. **Workers unhealthy**: Check worker logs for errors
3. **Large files**: Increase `QUEUE_MAX_SIZE` or add more workers

---

## Summary

### Migration Checklist

- [ ] Pre-migration validation complete
- [ ] RBAC resources applied
- [ ] Phase 2 workers deployed (3 replicas)
- [ ] Phase 2 orchestrator deployed
- [ ] Worker discovery validated
- [ ] Test task completed successfully
- [ ] Traffic switched to Phase 2
- [ ] Monitored for 30 minutes
- [ ] Phase 1 decommissioned
- [ ] Migration documented in runbook

### Key Differences to Remember

| Configuration | Phase 1 Value | Phase 2 Value |
|--------------|---------------|---------------|
| WORKER_DISCOVERY | `localhost` | `kubernetes` |
| WORKER_SERVICE_NAME | N/A | `subgen-worker` |
| ServiceAccount | Default | `subgen-orchestrator` |
| Replicas | 1 (pod) | 1 (orch) + 3 (workers) |
| Health Probes | Basic | HTTP on port 8080 |

---

**Document Status**: ✅ Final  
**Tested**: No (will be tested during Epic 9 execution)  
**Next Review**: After first production migration
