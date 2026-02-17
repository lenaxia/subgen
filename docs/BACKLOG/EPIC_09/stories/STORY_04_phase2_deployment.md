# STORY_04: Phase 2 Deployment Configuration

**Epic:** EPIC_09  
**Status:** Not Started  
**Assignee:** TBD  
**Effort:** 4-6 hours

---

## User Story

As a **DevOps engineer**,  
I want to **have separate Helm configurations for orchestrator and workers**,  
So that **I can scale them independently in Kubernetes**.

---

## Acceptance Criteria

- [ ] `values-phase2-orchestrator.yaml` created with K8s discovery config
- [ ] `values-phase2-workers.yaml` created with StatefulSet
- [ ] Workers use ClusterIP service for internal communication
- [ ] Orchestrator configured with `WORKER_DISCOVERY=kubernetes`
- [ ] Deployment guide complete with step-by-step instructions
- [ ] Installation tested with 1 worker (minimal config)
- [ ] Installation tested with 3 workers (production config)
- [ ] Scaling tested (3→5→3 workers)
- [ ] Documentation includes troubleshooting section

---

## Technical Design

### File Structure

```
deploy/
├── values.yaml                      # Phase 1 (single pod) - EXISTING
├── values-phase2-orchestrator.yaml  # Phase 2 orchestrator - NEW
├── values-phase2-workers.yaml       # Phase 2 workers - NEW
├── rbac.yaml                        # RBAC resources - NEW
├── README.md                        # General deployment guide
└── README-PHASE2.md                 # Phase 2 specific guide - NEW
```

---

## Configuration Files

### 1. values-phase2-orchestrator.yaml

```yaml
# yaml-language-server: $schema=https://raw.githubusercontent.com/bjw-s-labs/helm-charts/app-template-4.6.2/charts/other/app-template/values.schema.json

# ============================================================================
# Phase 2: Orchestrator Only
# ============================================================================
# This configuration deploys ONLY the orchestrator.
# Workers are deployed separately using values-phase2-workers.yaml

defaultPodOptions:
  serviceAccountName: subgen-orchestrator  # For K8s API access
  automountServiceAccountToken: true       # Required for worker discovery
  
  securityContext:
    fsGroup: 568
    fsGroupChangePolicy: "OnRootMismatch"
  
  annotations:
    reloader.stakater.com/auto: "true"

# ============================================================================
# Controllers
# ============================================================================
controllers:
  main:
    type: deployment
    replicas: 1  # Keep at 1 (orchestrator is singleton)
    strategy: Recreate
    
    containers:
      orchestrator:
        image:
          repository: ghcr.io/lenaxia/subgen-orchestrator
          tag: "latest"
          pullPolicy: Always
        
        env:
          # Server config
          WEBHOOK_PORT: "9000"
          METRICS_PORT: "9090"
          LOG_LEVEL: "info"
          LOG_FORMAT: "json"
          
          # ===================================================
          # PHASE 2: Kubernetes Worker Discovery
          # ===================================================
          WORKER_DISCOVERY: "kubernetes"
          WORKER_SERVICE_NAME: "subgen-worker"
          WORKER_NAMESPACE: "media"
          WORKER_PORT: "50051"
          LOAD_BALANCE_STRATEGY: "least_loaded"  # or "round_robin"
          
          # Queue config (higher capacity for Phase 2)
          QUEUE_MAX_SIZE: "5000"
          QUEUE_WORKER_TIMEOUT: "18000"
          
          # Processing flags
          PROCESS_ADDED_MEDIA: "true"
          PROCESS_MEDIA_ON_PLAY: "true"
          TRANSCRIBE_OR_TRANSLATE: "transcribe"
          
          # Skip conditions
          SKIP_IF_INTERNAL_SUBTITLES_LANGUAGE: "eng"
          SKIP_IF_TARGET_SUBTITLES_EXIST: "true"
          
          # Plex config
          PLEX_SERVER: "http://plex.media.svc.cluster.local:32400"
          PLEX_QUEUE_NEXT_EPISODE: "false"
          PLEX_QUEUE_SEASON: "false"
          PLEX_QUEUE_SERIES: "false"
          
          # Jellyfin config
          JELLYFIN_SERVER: "http://jellyfin.media.svc.cluster.local:8096"
        
        envFrom:
          - secretRef:
              name: subgen-secrets
        
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
            path: /health
            port: 9000
            spec:
              initialDelaySeconds: 10
              periodSeconds: 30
              timeoutSeconds: 5
              failureThreshold: 3
          
          readiness:
            enabled: true
            type: HTTP
            path: /health
            port: 9000
            spec:
              initialDelaySeconds: 5
              periodSeconds: 10
              timeoutSeconds: 3
        
        resources:
          requests:
            memory: 64Mi
            cpu: 100m
          limits:
            memory: 256Mi
            cpu: 500m

# ============================================================================
# Services
# ============================================================================
service:
  main:
    controller: main
    type: LoadBalancer
    annotations:
      metallb.universe.tf/loadBalancerIPs: 192.168.1.100
    ports:
      http:
        port: 9000
        targetPort: http
        protocol: HTTP
      metrics:
        port: 9090
        targetPort: metrics
        protocol: HTTP

# ============================================================================
# Persistence
# ============================================================================
persistence:
  # Media files on NFS (orchestrator needs read-write for metadata refresh)
  media:
    enabled: true
    type: nfs
    server: 192.168.1.10
    path: /mnt/pool/media
    globalMounts:
      - path: /media
        readOnly: false

# ============================================================================
# Monitoring
# ============================================================================
serviceMonitor:
  main:
    enabled: true
    serviceName: subgen-orchestrator
    labels:
      release: prometheus
    endpoints:
      - port: metrics
        scheme: http
        path: /metrics
        interval: 30s
        scrapeTimeout: 10s
```

---

### 2. values-phase2-workers.yaml

```yaml
# yaml-language-server: $schema=https://raw.githubusercontent.com/bjw-s-labs/helm-charts/app-template-4.6.2/charts/other/app-template/values.schema.json

# ============================================================================
# Phase 2: Workers Only (StatefulSet)
# ============================================================================
# This configuration deploys worker pods that are discovered by orchestrator.
# Scale by changing 'replicas' value.

defaultPodOptions:
  automountServiceAccountToken: false  # Workers don't need K8s API access
  
  securityContext:
    fsGroup: 568
    fsGroupChangePolicy: "OnRootMismatch"
  
  annotations:
    reloader.stakater.com/auto: "true"

# ============================================================================
# Controllers
# ============================================================================
controllers:
  main:
    type: statefulset  # StatefulSet for stable pod names
    replicas: 3        # ← SCALE HERE (change to 1, 3, 5, etc.)
    
    containers:
      worker:
        image:
          repository: ghcr.io/lenaxia/subgen-worker
          tag: "latest"
          pullPolicy: Always
        
        env:
          # Worker config
          GRPC_PORT: "50051"
          LOG_LEVEL: "info"
          LOG_FORMAT: "json"
          
          # Whisper config
          WHISPER_MODEL: "medium"
          WHISPER_THREADS: "4"
          TRANSCRIBE_DEVICE: "cpu"
          COMPUTE_TYPE: "auto"
          MODEL_PATH: "/models"
          
          # Memory management
          MEMORY_THRESHOLD_MB: "3000"
          MODEL_CLEANUP_DELAY: "300"
          CLEAR_VRAM_ON_COMPLETE: "true"
          
          # Subtitle config
          WORD_LEVEL_HIGHLIGHT: "false"
          CUSTOM_REGROUP: "cm_sl=84_sl=42++++++1"
          LRC_FOR_AUDIO_FILES: "true"
          APPEND: "false"
          SUBTITLE_LANGUAGE_NAME: "aa"
          
          # Cache directories
          XDG_CACHE_HOME: "/cache"
          HF_HOME: "/cache/huggingface"
          MPLCONFIGDIR: "/cache/matplotlib"
        
        ports:
          - name: grpc
            containerPort: 50051
            protocol: TCP
        
        probes:
          liveness:
            enabled: true
            type: GRPC
            port: 50051
            spec:
              initialDelaySeconds: 60
              periodSeconds: 60
              timeoutSeconds: 10
              failureThreshold: 3
          
          readiness:
            enabled: true
            type: GRPC
            port: 50051
            spec:
              initialDelaySeconds: 30
              periodSeconds: 30
              timeoutSeconds: 5
          
          startup:
            enabled: true
            type: GRPC
            port: 50051
            spec:
              initialDelaySeconds: 10
              periodSeconds: 10
              timeoutSeconds: 5
              failureThreshold: 30  # 5 min for model download
        
        resources:
          requests:
            memory: 2Gi
            cpu: 500m
          limits:
            memory: 4Gi
            cpu: 2000m

# ============================================================================
# Services
# ============================================================================
service:
  main:
    controller: main
    type: ClusterIP  # Internal only (orchestrator discovers via Endpoints)
    ports:
      grpc:
        port: 50051
        targetPort: grpc
        protocol: TCP

# ============================================================================
# Persistence
# ============================================================================
persistence:
  # Media files on NFS
  media:
    enabled: true
    type: nfs
    server: 192.168.1.10
    path: /mnt/pool/media
    globalMounts:
      - path: /media
        readOnly: false
  
  # Whisper models (PVC per worker)
  models:
    enabled: true
    type: persistentVolumeClaim
    accessMode: ReadWriteOnce
    size: 5Gi
    storageClass: local-path
    retain: true
    globalMounts:
      - path: /models
  
  # Cache (tmpfs)
  cache:
    enabled: true
    type: emptyDir
    medium: Memory
    sizeLimit: 1Gi
    globalMounts:
      - path: /cache
```

---

### 3. README-PHASE2.md

```markdown
# Phase 2 Deployment: Multiple Workers

This guide covers deploying orchestrator and workers **separately** for horizontal scaling.

## Overview

**Phase 2** separates orchestrator and workers into independent deployments:

- **Orchestrator**: Single deployment, discovers workers via K8s Endpoints API
- **Workers**: StatefulSet, scale independently from 1 to N workers

## Prerequisites

1. Phase 1 working (single-pod deployment tested)
2. RBAC resources created (`deploy/rbac.yaml`)
3. K8s cluster with RBAC enabled
4. Storage class for PVCs

## Installation Steps

### 1. Create RBAC Resources

```bash
kubectl apply -f deploy/rbac.yaml
```

Verify:
```bash
kubectl auth can-i get endpoints \
  --as=system:serviceaccount:media:subgen-orchestrator \
  -n media
# Expected: yes
```

### 2. Install Workers First

```bash
helm install subgen-worker bjw-s/app-template \
  --namespace media \
  --values deploy/values-phase2-workers.yaml
```

Wait for workers to be ready:
```bash
kubectl wait --for=condition=ready pod \
  -l app.kubernetes.io/name=subgen-worker \
  -n media \
  --timeout=300s
```

Verify:
```bash
kubectl get pods -n media -l app.kubernetes.io/name=subgen-worker
# NAME                READY   STATUS    RESTARTS   AGE
# subgen-worker-0     1/1     Running   0          2m
# subgen-worker-1     1/1     Running   0          2m
# subgen-worker-2     1/1     Running   0          2m
```

### 3. Install Orchestrator

```bash
helm install subgen-orchestrator bjw-s/app-template \
  --namespace media \
  --values deploy/values-phase2-orchestrator.yaml
```

Verify:
```bash
kubectl get pods -n media -l app.kubernetes.io/name=subgen-orchestrator
# NAME                            READY   STATUS    RESTARTS   AGE
# subgen-orchestrator-xxx         1/1     Running   0          1m
```

### 4. Verify Worker Discovery

```bash
kubectl logs -n media -l app.kubernetes.io/name=subgen-orchestrator --tail=50
```

Look for:
```
INFO: Discovered 3 workers from K8s
INFO: Worker pool started with healthy workers (healthy=3, total=3)
```

## Scaling Workers

### Scale Up

```bash
# Scale to 5 workers
kubectl scale statefulset subgen-worker --replicas=5 -n media

# Or via Helm
helm upgrade subgen-worker bjw-s/app-template \
  --namespace media \
  --values deploy/values-phase2-workers.yaml \
  --set controllers.main.replicas=5
```

Verify discovery (within 30 seconds):
```bash
kubectl logs -n media -l app.kubernetes.io/name=subgen-orchestrator --tail=20
# INFO: Worker added: subgen-worker-3
# INFO: Worker added: subgen-worker-4
# INFO: Discovered 5 workers from K8s
```

### Scale Down

```bash
kubectl scale statefulset subgen-worker --replicas=3 -n media
```

Verify (within 60 seconds):
```bash
kubectl logs -n media -l app.kubernetes.io/name=subgen-orchestrator --tail=20
# INFO: Worker removed: subgen-worker-4
# INFO: Worker removed: subgen-worker-3
# INFO: Discovered 3 workers from K8s
```

## Validation Tests

### 1. Worker Health

```bash
curl http://<orchestrator-ip>:9090/metrics | grep worker_healthy
# subgen_worker_healthy{worker="subgen-worker-0"} 1
# subgen_worker_healthy{worker="subgen-worker-1"} 1
# subgen_worker_healthy{worker="subgen-worker-2"} 1
```

### 2. Load Distribution

Queue 10 tasks:
```bash
for i in {1..10}; do
  curl -X POST http://<orchestrator-ip>:9000/batch \
    -d '{"path":"/media/test/file'$i'.mp4"}'
done
```

Check distribution:
```bash
curl http://<orchestrator-ip>:9090/metrics | grep worker_active_jobs
# subgen_worker_active_jobs{worker="subgen-worker-0"} 3
# subgen_worker_active_jobs{worker="subgen-worker-1"} 4
# subgen_worker_active_jobs{worker="subgen-worker-2"} 3
```

## Troubleshooting

### Workers Not Discovered

**Symptom**: Orchestrator logs show "no healthy workers found"

**Check**:
```bash
# 1. Service exists
kubectl get svc -n media subgen-worker

# 2. Endpoints exist
kubectl get endpoints -n media subgen-worker
# Should show worker IPs

# 3. RBAC permissions
kubectl auth can-i get endpoints \
  --as=system:serviceaccount:media:subgen-orchestrator \
  -n media
# Should be "yes"

# 4. Orchestrator logs
kubectl logs -n media -l app.kubernetes.io/name=subgen-orchestrator --tail=50
```

### Worker Not Removed After Scale Down

**Symptom**: Old workers still in metrics after scale down

**Wait**: Discovery refresh happens every 30 seconds

**Force refresh**: Restart orchestrator
```bash
kubectl rollout restart deployment subgen-orchestrator -n media
```

## Comparison: Phase 1 vs Phase 2

| Aspect | Phase 1 | Phase 2 |
|--------|---------|---------|
| Pods | 1 pod (2 containers) | 1 orchestrator + N workers |
| Scaling | Vertical only | Horizontal workers |
| Worker discovery | localhost | Kubernetes Endpoints |
| Deployment | Single Helm release | 2 separate releases |
| Complexity | Simple | Moderate |
| RBAC | Not needed | Required |

## When to Use Phase 2

Use Phase 2 when:
- Processing >10 files/hour consistently
- Want autoscaling capability
- Have sufficient cluster resources (each worker = 2-4GB RAM)
- Need fault tolerance (tasks redistributed if worker fails)

Stick with Phase 1 when:
- Processing <10 files/hour
- Simple home lab setup
- Limited cluster resources
- Don't need scaling complexity
```

---

## Deployment Testing Checklist

### Minimal Config (1 Worker)

```bash
# 1. Deploy with replicas=1
helm install subgen-worker bjw-s/app-template \
  --namespace media \
  --values deploy/values-phase2-workers.yaml \
  --set controllers.main.replicas=1

# 2. Verify single worker discovered
kubectl logs -n media -l app.kubernetes.io/name=subgen-orchestrator | grep "Discovered"
# Expected: "Discovered 1 workers from K8s"

# 3. Test transcription
curl -X POST http://<orchestrator-ip>:9000/batch \
  -d '{"path":"/media/test/sample.mp4"}'

# 4. Verify task processed
kubectl logs -n media subgen-worker-0 --tail=20
# Should show transcription logs
```

### Production Config (3 Workers)

```bash
# 1. Deploy with replicas=3 (default)
helm install subgen-worker bjw-s/app-template \
  --namespace media \
  --values deploy/values-phase2-workers.yaml

# 2. Verify all workers discovered
kubectl logs -n media -l app.kubernetes.io/name=subgen-orchestrator | grep "Discovered"
# Expected: "Discovered 3 workers from K8s"

# 3. Test load distribution (10 tasks)
for i in {1..10}; do
  curl -X POST http://<orchestrator-ip>:9000/batch \
    -d '{"path":"/media/test/file'$i'.mp4"}'
  sleep 1
done

# 4. Check distribution
curl http://<orchestrator-ip>:9090/metrics | grep worker_active_jobs
# Should show jobs distributed across workers
```

### Scaling Test

```bash
# 1. Start with 3 workers
helm install subgen-worker bjw-s/app-template \
  --namespace media \
  --values deploy/values-phase2-workers.yaml

# 2. Scale up to 5
kubectl scale statefulset subgen-worker --replicas=5 -n media

# 3. Wait and verify
sleep 30
kubectl logs -n media -l app.kubernetes.io/name=subgen-orchestrator --tail=10
# Expected: "Worker added: subgen-worker-3/4"

# 4. Scale down to 3
kubectl scale statefulset subgen-worker --replicas=3 -n media

# 5. Wait and verify
sleep 60
kubectl logs -n media -l app.kubernetes.io/name=subgen-orchestrator --tail=10
# Expected: "Worker removed: subgen-worker-3/4"
```

---

## Files to Create

1. `deploy/values-phase2-orchestrator.yaml` - Orchestrator config
2. `deploy/values-phase2-workers.yaml` - Workers config
3. `deploy/README-PHASE2.md` - Phase 2 deployment guide
4. Update `deploy/README.md` - Add Phase 2 reference

---

## Documentation Updates

### Update Main README.md

Add section:

```markdown
## Phase 2: Multiple Workers

For high-volume workloads, deploy orchestrator and workers separately:

See [deploy/README-PHASE2.md](deploy/README-PHASE2.md) for complete guide.

**Quick Start:**
```bash
kubectl apply -f deploy/rbac.yaml
helm install subgen-worker bjw-s/app-template -n media --values deploy/values-phase2-workers.yaml
helm install subgen-orchestrator bjw-s/app-template -n media --values deploy/values-phase2-orchestrator.yaml
```
```

---

## Definition of Done

- [ ] `values-phase2-orchestrator.yaml` created and tested
- [ ] `values-phase2-workers.yaml` created and tested
- [ ] `README-PHASE2.md` created with complete guide
- [ ] Main README.md updated with Phase 2 reference
- [ ] Deployment tested with 1 worker (minimal)
- [ ] Deployment tested with 3 workers (production)
- [ ] Scaling tested (3→5→3)
- [ ] Worker discovery verified (<30 seconds)
- [ ] Load balancing verified (tasks distributed)
- [ ] Troubleshooting section complete
- [ ] Work log created

---

**Story Owner:** TBD  
**Created:** 2026-02-17
