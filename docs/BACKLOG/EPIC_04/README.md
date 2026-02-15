# EPIC_04: Kubernetes Deployment with bjw-s

**Status:** Not Started  
**Estimated Effort:** 12-17 hours  
**Duration:** 2 days  
**Can Parallelize:** ❌ No (depends on EPIC_03)

---

## Overview

Deploy subgen to Kubernetes using the **bjw-s app-template** Helm chart. This epic focuses on practical deployment, not custom Helm development. The bjw-s chart provides all necessary features out-of-the-box (multi-container pods, persistence, monitoring).

---

## Goals

1. Create production-ready `values.yaml` for bjw-s app-template
2. Configure NFS and PVC persistence
3. Set up secrets management
4. Configure Prometheus monitoring (ServiceMonitor)
5. Document deployment procedures
6. Test Phase 1 deployment (single pod)

---

## Design References

- [00_HYBRID_ARCHITECTURE.md](../../DESIGN/00_HYBRID_ARCHITECTURE.md) - System architecture
- [04_K8S_DEPLOYMENT.md](../../DESIGN/04_K8S_DEPLOYMENT.md) - **PRIMARY REFERENCE**
- [03_SCALING_STRATEGY.md](../../DESIGN/03_SCALING_STRATEGY.md) - Phase 1 → Phase 2 scaling

---

## User Stories

### [STORY_01: bjw-s values.yaml Configuration](./stories/STORY_01_bjws_values.md)
**Status:** Not Started  
**Effort:** 6-8 hours  
**Summary:** Complete values.yaml with 2 containers, persistence, monitoring

### [STORY_02: Secrets & ConfigMaps](./stories/STORY_02_secrets_config.md)
**Status:** Not Started  
**Effort:** 3-4 hours  
**Summary:** Secret management (Plex/Jellyfin tokens), config organization

### [STORY_03: Deployment Testing & Validation](./stories/STORY_03_deployment_testing.md)
**Status:** Not Started  
**Effort:** 3-5 hours  
**Summary:** Deploy to cluster, validate functionality, troubleshooting guide

---

## Acceptance Criteria

- [ ] All 3 stories completed
- [ ] `values.yaml` works with bjw-s app-template 4.6.2+
- [ ] Single pod deploys successfully (Phase 1)
- [ ] Both containers start and become ready
- [ ] NFS mount works (media files accessible)
- [ ] PVC mount works (models directory)
- [ ] Secrets loaded correctly (tokens present in env)
- [ ] ServiceMonitor created for Prometheus
- [ ] Health checks work (liveness + readiness + startup)
- [ ] Webhooks reachable via LoadBalancer
- [ ] Metrics endpoint accessible
- [ ] Documentation complete (README, troubleshooting)
- [ ] Work logs created for all stories

---

## Dependencies

**Requires:**
- EPIC_03 (Integration & Testing) - **MUST be complete** (validated code)
- Docker images built and pushed to ghcr.io

**Blocks:**
- EPIC_05 (Migration & Cutover) - needs working K8s deployment

**Parallelizable With:**
- None (sequential epic)

---

## bjw-s app-template Features Used

| Feature | Subgen Usage |
|---------|--------------|
| **Multi-container pods** | Orchestrator + Worker in same pod |
| **Deployments** | Main controller type (Phase 1) |
| **Services** | LoadBalancer for webhooks + metrics |
| **Persistence (NFS)** | Media files (read-write) |
| **Persistence (PVC)** | Whisper models (persistent) |
| **Persistence (emptyDir)** | tmpfs cache (fast, ephemeral) |
| **Probes** | Liveness, readiness, startup for both containers |
| **ServiceMonitor** | Prometheus Operator integration |
| **Secrets** | Plex/Jellyfin tokens |

**NOT USED:**
- Ingress (webhooks use LoadBalancer directly)
- StatefulSet (Phase 1 only, Phase 2 uses StatefulSet for workers)
- DaemonSet (not needed)

---

## Key Configuration Decisions

### 1. Container Dependency

**Decision:** Worker starts AFTER orchestrator

```yaml
containers:
  orchestrator:
    # Starts first
  worker:
    dependsOn: orchestrator  # ← Wait for orchestrator
```

**Why:** Orchestrator should be ready before worker tries to register

---

### 2. Resource Limits

**Decision:** Hard memory limits to prevent OOM

```yaml
orchestrator:
  resources:
    requests: {memory: 64Mi, cpu: 100m}
    limits:   {memory: 256Mi, cpu: 500m}

worker:
  resources:
    requests: {memory: 2Gi, cpu: 500m}
    limits:   {memory: 4Gi, cpu: 2000m}  # Hard OOM kill at 4GB
```

**Why:**
- Orchestrator: Lightweight, minimal memory
- Worker: Whisper models + processing = 2-4GB

---

### 3. Startup Probe for Worker

**Decision:** 5-minute startup window for model download

```yaml
worker:
  probes:
    startup:
      initialDelaySeconds: 10
      periodSeconds: 10
      failureThreshold: 30  # 30 × 10s = 5 minutes
```

**Why:** First-time model download can take 2-3 minutes

---

### 4. NFS Mount (Read-Write)

**Decision:** NFS for media files, read-write access

```yaml
persistence:
  media:
    type: nfs
    server: 192.168.1.10
    path: /mnt/pool/media
    advancedMounts:
      main:
        orchestrator: [{path: /media, readOnly: false}]
        worker:       [{path: /media, readOnly: false}]
```

**Why:** Both containers need write access (orchestrator: metadata refresh, worker: subtitle writing)

---

### 5. PVC for Models

**Decision:** Persistent volume for Whisper models

```yaml
persistence:
  models:
    type: persistentVolumeClaim
    size: 5Gi
    retain: true  # Don't delete on uninstall
    advancedMounts:
      main:
        worker: [{path: /models}]
```

**Why:** Models are large (1-3GB), shouldn't be re-downloaded on every restart

---

## File Structure

```
deploy/
├── values.yaml                  # Phase 1 (single pod)
├── values-phase2-orch.yaml      # Phase 2 (orchestrator)
├── values-phase2-workers.yaml   # Phase 2 (workers)
├── secret-example.yaml          # Secret template (DO NOT COMMIT REAL VALUES)
├── README.md                    # Deployment instructions
└── troubleshooting.md           # Common issues & fixes
```

---

## Deployment Procedure

### Prerequisites

1. Kubernetes cluster (1.24+)
2. MetalLB or cloud LoadBalancer
3. NFS server accessible from cluster
4. Storage class for PVCs (e.g., `local-path`)
5. Prometheus Operator (optional, for metrics)

---

### Installation Steps

```bash
# 1. Add bjw-s Helm repo
helm repo add bjw-s https://bjw-s-labs.github.io/helm-charts
helm repo update

# 2. Create namespace
kubectl create namespace media

# 3. Create secret
kubectl create secret generic subgen-secrets \
  --namespace media \
  --from-literal=PLEX_TOKEN='your-token-here' \
  --from-literal=JELLYFIN_TOKEN='your-token-here'

# 4. Install chart
helm install subgen bjw-s/app-template \
  --namespace media \
  --values deploy/values.yaml

# 5. Watch deployment
kubectl get pods -n media -l app.kubernetes.io/name=subgen -w

# 6. Check logs
kubectl logs -n media -l app.kubernetes.io/name=subgen -c orchestrator --tail=100 -f
kubectl logs -n media -l app.kubernetes.io/name=subgen -c worker --tail=100 -f

# 7. Get service IP
kubectl get svc -n media subgen-main

# 8. Test health
curl http://<external-ip>:9000/health
```

---

## Validation Tests

### 1. Pod Status

```bash
kubectl get pods -n media
# NAME                READY   STATUS    RESTARTS   AGE
# subgen-main-xxx     2/2     Running   0          2m
```

**Success:** `2/2` Running (both containers)

---

### 2. Container Readiness

```bash
kubectl describe pod -n media subgen-main-xxx
# Containers:
#   orchestrator:
#     State:          Running
#     Ready:          True
#   worker:
#     State:          Running
#     Ready:          True
```

---

### 3. NFS Mount

```bash
kubectl exec -it -n media subgen-main-xxx -c worker -- ls -la /media
# Should show media files from NAS
```

---

### 4. Model Cache

```bash
kubectl exec -it -n media subgen-main-xxx -c worker -- ls -la /models
# Initially empty, populated after first transcription
```

---

### 5. Health Endpoints

```bash
# Orchestrator health
curl http://<external-ip>:9000/health
# {"status": "healthy", "version": "1.0.0"}

# Worker health (via orchestrator)
# Should be logged in orchestrator logs
kubectl logs -n media subgen-main-xxx -c orchestrator | grep "worker health"
```

---

### 6. Metrics

```bash
curl http://<external-ip>:9090/metrics
# subgen_queue_size 0
# subgen_worker_memory_mb 1200
# ...
```

---

### 7. Prometheus Scraping

```bash
# Check ServiceMonitor created
kubectl get servicemonitor -n media
# NAME          AGE
# subgen-main   2m

# In Prometheus UI: Status → Targets
# Look for: serviceMonitor/media/subgen-main/0
```

---

## Timeline

**Day 1:**
- STORY_01 (bjw-s values.yaml) - 6-8 hours

**Day 2:**
- STORY_02 (Secrets & ConfigMaps) - 3-4 hours
- STORY_03 (Deployment Testing) - 3-5 hours

---

## Risks & Mitigation

| Risk | Impact | Mitigation |
|------|--------|------------|
| NFS mount permission denied | High | Test NFS exports, use no_root_squash |
| LoadBalancer IP not assigned | Medium | Install MetalLB, configure IP pool |
| Model download timeout | Medium | Increase startup probe failureThreshold |
| Worker OOM killed | High | Set appropriate memory limits (4Gi) |
| Secret not found | Medium | Create secret before install |

---

## Troubleshooting Guide

Common issues documented in `deploy/troubleshooting.md`:

1. Pod won't start (ImagePullBackOff)
2. NFS mount fails (permission denied)
3. Worker OOM killed
4. Webhook not reachable (LoadBalancer pending)
5. Prometheus not scraping (ServiceMonitor selector)

---

## Definition of Done

- [ ] All 3 stories completed with ✅ status
- [ ] `values.yaml` works with bjw-s 4.6.2+
- [ ] Deployment successful on test cluster
- [ ] Both containers running and ready
- [ ] All persistence volumes mounted correctly
- [ ] Health checks pass
- [ ] Metrics accessible
- [ ] ServiceMonitor created
- [ ] Documentation complete (README + troubleshooting)
- [ ] Work logs created for each story
- [ ] Helm install command documented
- [ ] Validation tests pass

---

## Next Epic

**EPIC_05: Migration & Cutover** - Migrate from Docker to Kubernetes

---

## References

- README-LLM.md - Development workflow
- [04_K8S_DEPLOYMENT.md](../../DESIGN/04_K8S_DEPLOYMENT.md) - **PRIMARY**
- bjw-s docs: https://bjw-s.github.io/helm-charts/docs/
- bjw-s examples: https://github.com/bjw-s/helm-charts/tree/main/charts/other/app-template/examples

---

**Epic Owner:** TBD  
**Created:** 2026-02-15  
**Last Updated:** 2026-02-15
