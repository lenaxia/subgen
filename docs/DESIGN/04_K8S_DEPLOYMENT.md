# Kubernetes Deployment with bjw-s app-template

**Document Version:** 1.0  
**Last Updated:** 2026-02-15  
**Status:** Draft  
**Related Documents:**
- [00_HYBRID_ARCHITECTURE.md](./00_HYBRID_ARCHITECTURE.md)
- [03_SCALING_STRATEGY.md](./03_SCALING_STRATEGY.md)

---

## Table of Contents

1. [Overview](#overview)
2. [bjw-s app-template Introduction](#bjw-s-app-template-introduction)
3. [Phase 1 Deployment](#phase-1-deployment)
4. [Phase 2 Deployment](#phase-2-deployment)
5. [Persistent Storage](#persistent-storage)
6. [Secrets Management](#secrets-management)
7. [Monitoring & Observability](#monitoring--observability)
8. [Troubleshooting](#troubleshooting)

---

## Overview

### Deployment Modes

Subgen supports two deployment modes for worker discovery:

| Mode | Environment | Configuration | Use Case |
|------|-------------|---------------|----------|
| **localhost** | Docker Compose | `WORKER_DISCOVERY=localhost` (default)<br>`WORKER_ADDRESS=worker:50051` | Single worker, simple setup |
| **kubernetes** | Kubernetes | `WORKER_DISCOVERY=kubernetes`<br>`WORKER_NAMESPACE=media`<br>`WORKER_SERVICE_NAME=worker` | Multiple workers, auto-scaling |

**IMPORTANT:** The `kubernetes` mode ONLY works inside a Kubernetes cluster. If you try to use it in Docker Compose, you'll see this helpful error:

```
kubernetes discovery requires running inside a Kubernetes cluster.
For Docker Compose deployments, use WORKER_DISCOVERY=localhost (or omit, it's the default).
```

**Default behavior:**
- Docker Compose: Uses `localhost` mode automatically (no configuration needed)
- Kubernetes: Must explicitly set `WORKER_DISCOVERY=kubernetes`

**How it works:**
- **localhost mode**: Connects to a single hardcoded worker address (Phase 1 deployment pattern)
- **kubernetes mode**: Discovers workers dynamically via K8s Endpoints API (Phase 2 deployment pattern)

---

### Why bjw-s app-template?

**Decision**: Use bjw-s Helm chart instead of custom chart

**Rationale:**
- ✅ **Multi-container pods**: Native support (orchestrator + worker in same pod)
- ✅ **Common patterns built-in**: Probes, services, ingress, persistence, monitoring
- ✅ **Battle-tested**: Used by thousands in homelab community
- ✅ **Well-documented**: Extensive examples and schema
- ✅ **No custom Helm required**: Just values.yaml

**Alternative Rejected**: Custom Helm chart
- ❌ More maintenance overhead
- ❌ Reinventing solved problems
- ❌ Steeper learning curve for users

---

## bjw-s app-template Introduction

### What is bjw-s app-template?

A **universal Helm chart** for deploying applications on Kubernetes, originally designed for home media servers but applicable to any workload.

### Key Features

| Feature | Description | Subgen Usage |
|---------|-------------|--------------|
| **Multi-container pods** | Multiple containers in single pod | Orchestrator + Worker |
| **Controllers** | Deployment, StatefulSet, DaemonSet, Job | Deployment (Phase 1), StatefulSet (Phase 2) |
| **Services** | LoadBalancer, ClusterIP, NodePort | LoadBalancer for webhooks |
| **Persistence** | PVC, NFS, hostPath, emptyDir | NFS for media, PVC for models |
| **Probes** | Liveness, readiness, startup | Health checks on both containers |
| **ServiceMonitor** | Prometheus Operator integration | Metrics scraping |
| **Ingress** | Optional HTTP/HTTPS routing | Not needed (webhooks via LB) |

### Version

- **Current**: 4.6.2 (as of 2026-02-15)
- **Chart repo**: `https://bjw-s-labs.github.io/helm-charts`
- **Documentation**: https://bjw-s.github.io/helm-charts/docs/

---

## Phase 1 Deployment

### Overview

**Single pod with 2 containers: orchestrator + worker**

```
┌─────────────────────────────────────────────────────────────┐
│  Pod: subgen-main-xxx                                       │
├─────────────────────────────────────────────────────────────┤
│  Container: orchestrator (Go)                               │
│  • Port 9000: HTTP webhooks                                 │
│  • Port 9090: Prometheus metrics                            │
│  • Connects to: localhost:50051                             │
├─────────────────────────────────────────────────────────────┤
│  Container: worker (Python)                                 │
│  • Port 50051: gRPC server                                  │
│  • Model cache: /models (PVC)                               │
└─────────────────────────────────────────────────────────────┘
         ↓
┌─────────────────────────────────────────────────────────────┐
│  Service: subgen-main (LoadBalancer)                        │
│  • External IP: 192.168.1.100                               │
│  • Port 9000 → orchestrator:9000 (webhooks)                 │
│  • Port 9090 → orchestrator:9090 (metrics)                  │
└─────────────────────────────────────────────────────────────┘
```

---

### values.yaml (Phase 1)

**File: `deploy/values-phase1.yaml`**

```yaml
# yaml-language-server: $schema=https://raw.githubusercontent.com/bjw-s-labs/helm-charts/app-template-4.6.2/charts/other/app-template/values.schema.json

# ============================================================================
# Global Pod Configuration
# ============================================================================
defaultPodOptions:
  automountServiceAccountToken: false  # Not needed in Phase 1
  
  securityContext:
    fsGroup: 568  # Media server user
    fsGroupChangePolicy: "OnRootMismatch"
  
  annotations:
    reloader.stakater.com/auto: "true"  # Auto-reload on ConfigMap/Secret change

# ============================================================================
# Controllers
# ============================================================================
controllers:
  main:
    type: deployment
    replicas: 1
    strategy: Recreate  # Clean shutdown before new pod starts
    
    containers:
      # ──────────────────────────────────────────────────────────────────
      # Go Orchestrator Container
      # ──────────────────────────────────────────────────────────────────
      orchestrator:
        image:
          repository: ghcr.io/your-username/subgen-orchestrator
          tag: "latest"  # Change to specific version in production
          pullPolicy: Always
        
        env:
          # Orchestrator config
          WEBHOOK_PORT: "9000"
          METRICS_PORT: "9090"
          LOG_LEVEL: "info"
          LOG_FORMAT: "json"
          
          # Worker discovery (Phase 1: localhost)
          WORKER_DISCOVERY: "localhost"
          PYTHON_WORKER_ADDRESS: "localhost:50051"
          
          # Queue config
          QUEUE_MAX_SIZE: "1000"
          QUEUE_WORKER_TIMEOUT: "18000"  # 5 hours
          
          # Processing flags
          PROCESS_ADDED_MEDIA: "true"
          PROCESS_MEDIA_ON_PLAY: "false"
          TRANSCRIBE_OR_TRANSLATE: "transcribe"
          
          # Skip conditions
          SKIP_IF_INTERNAL_SUBTITLES_LANGUAGE: "eng"
          SKIP_IF_TARGET_SUBTITLES_EXIST: "true"
          
          # Plex config
          PLEX_SERVER: "http://plex.media.svc.cluster.local:32400"
          PLEX_QUEUE_NEXT_EPISODE: "false"
          
          # Jellyfin config
          JELLYFIN_SERVER: "http://jellyfin.media.svc.cluster.local:8096"
        
        envFrom:
          - secretRef:
              name: subgen-secrets  # PLEX_TOKEN, JELLYFIN_TOKEN
        
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
      
      # ──────────────────────────────────────────────────────────────────
      # Python Worker Container
      # ──────────────────────────────────────────────────────────────────
      worker:
        dependsOn: orchestrator  # Start orchestrator first
        
        image:
          repository: ghcr.io/your-username/subgen-worker
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
          MODEL_CLEANUP_DELAY: "30"
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
            memory: 4Gi  # Hard OOM kill if exceeded
            cpu: 2000m

# ============================================================================
# Services
# ============================================================================
service:
  main:
    controller: main
    type: LoadBalancer
    annotations:
      metallb.universe.tf/loadBalancerIPs: 192.168.1.100  # Your static IP
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
  # Media files on NFS (read-write)
  media:
    enabled: true
    type: nfs
    server: 192.168.1.10  # Your NAS IP
    path: /mnt/pool/media
    advancedMounts:
      main:
        orchestrator:
          - path: /media
            readOnly: false  # RW for subtitle writing
        worker:
          - path: /media
            readOnly: false
  
  # Whisper models (PVC)
  models:
    enabled: true
    type: persistentVolumeClaim
    accessMode: ReadWriteOnce
    size: 5Gi
    storageClass: local-path  # or your storage class
    retain: true
    advancedMounts:
      main:
        worker:
          - path: /models
  
  # Cache (tmpfs for fast model loading)
  cache:
    enabled: true
    type: emptyDir
    medium: Memory
    sizeLimit: 1Gi
    globalMounts:
      - path: /cache

# ============================================================================
# Monitoring (Prometheus)
# ============================================================================
serviceMonitor:
  main:
    enabled: true
    serviceName: subgen-main
    labels:
      release: prometheus  # Match your Prometheus Operator selector
    endpoints:
      - port: metrics
        scheme: http
        path: /metrics
        interval: 30s
        scrapeTimeout: 10s
```

---

### Installation (Phase 1)

```bash
# 1. Add bjw-s Helm repo
helm repo add bjw-s https://bjw-s-labs.github.io/helm-charts
helm repo update

# 2. Create namespace
kubectl create namespace media

# 3. Create secret with tokens
kubectl create secret generic subgen-secrets \
  --namespace media \
  --from-literal=PLEX_TOKEN='your-plex-token-here' \
  --from-literal=JELLYFIN_TOKEN='your-jellyfin-token-here'

# 4. Install chart
helm install subgen bjw-s/app-template \
  --namespace media \
  --values deploy/values-phase1.yaml

# 5. Watch deployment
kubectl get pods -n media -l app.kubernetes.io/name=subgen -w

# 6. Check logs
kubectl logs -n media -l app.kubernetes.io/name=subgen -c orchestrator --tail=100 -f
kubectl logs -n media -l app.kubernetes.io/name=subgen -c worker --tail=100 -f
```

---

### Verification (Phase 1)

```bash
# 1. Get service external IP
kubectl get svc -n media subgen-main
# NAME          TYPE           CLUSTER-IP      EXTERNAL-IP      PORT(S)
# subgen-main   LoadBalancer   10.43.100.100   192.168.1.100    9000:30100/TCP,9090:30200/TCP

# 2. Test health endpoint
curl http://192.168.1.100:9000/health
# {"status": "healthy", "version": "1.0.0"}

# 3. Test metrics endpoint
curl http://192.168.1.100:9090/metrics
# subgen_queue_size 0
# subgen_worker_memory_mb 1200
# ...

# 4. Test Plex webhook (manual)
curl -X POST http://192.168.1.100:9000/plex \
  -H "Content-Type: application/json" \
  -d '{"event": "library.new", "Metadata": {"ratingKey": "12345"}}'
```

---

## Phase 2 Deployment

### Overview

**Separate orchestrator and worker deployments**

```
┌─────────────────────────────────────────────────────────────┐
│  Pod: subgen-orchestrator-xxx                               │
│  Deployment: subgen-orchestrator (1 replica)                │
├─────────────────────────────────────────────────────────────┤
│  Container: orchestrator                                     │
│  • Discovers workers via K8s Service                        │
│  • Load balances across all workers                         │
└─────────────────────────────────────────────────────────────┘
         ↓ gRPC via subgen-worker Service
┌─────────────────────────────────────────────────────────────┐
│  Service: subgen-worker (ClusterIP)                         │
│  Port: 50051                                                 │
└─────────────────────────────────────────────────────────────┘
       ↓                    ↓                    ↓
┌───────────────┐  ┌───────────────┐  ┌───────────────┐
│ Pod: worker-0 │  │ Pod: worker-1 │  │ Pod: worker-2 │
│ StatefulSet   │  │ StatefulSet   │  │ StatefulSet   │
└───────────────┘  └───────────────┘  └───────────────┘
```

---

### values.yaml (Phase 2 - Orchestrator)

**File: `deploy/values-phase2-orchestrator.yaml`**

```yaml
defaultPodOptions:
  automountServiceAccountToken: true  # Needed for K8s discovery

controllers:
  main:
    type: deployment
    replicas: 1
    
    containers:
      orchestrator:
        image:
          repository: ghcr.io/your-username/subgen-orchestrator
          tag: "latest"
        
        env:
          # Worker discovery (Phase 2: Kubernetes)
          WORKER_DISCOVERY: "kubernetes"
          WORKER_SERVICE_NAME: "subgen-worker"
          WORKER_NAMESPACE: "media"
          WORKER_PORT: "50051"
          LOAD_BALANCE_STRATEGY: "least_loaded"
          
          # Queue config (higher capacity)
          QUEUE_MAX_SIZE: "5000"
          
          # ... (rest same as Phase 1)
        
        resources:
          requests:
            memory: 64Mi
            cpu: 100m
          limits:
            memory: 256Mi
            cpu: 500m

# Service for webhooks
service:
  main:
    controller: main
    type: LoadBalancer
    annotations:
      metallb.universe.tf/loadBalancerIPs: 192.168.1.100
    ports:
      http:
        port: 9000
      metrics:
        port: 9090

# Persistence (media only, no models)
persistence:
  media:
    enabled: true
    type: nfs
    server: 192.168.1.10
    path: /mnt/pool/media
    globalMounts:
      - path: /media
        readOnly: false
```

---

### values.yaml (Phase 2 - Workers)

**File: `deploy/values-phase2-workers.yaml`**

```yaml
controllers:
  main:
    type: statefulset
    replicas: 3  # Scale here!
    
    containers:
      worker:
        image:
          repository: ghcr.io/your-username/subgen-worker
          tag: "latest"
        
        env:
          GRPC_PORT: "50051"
          WHISPER_MODEL: "medium"
          WHISPER_THREADS: "4"
          # ... (rest same as Phase 1)
        
        ports:
          - name: grpc
            containerPort: 50051
        
        probes:
          # ... (same as Phase 1)
        
        resources:
          requests:
            memory: 2Gi
            cpu: 500m
          limits:
            memory: 4Gi
            cpu: 2000m

# Service for worker discovery
service:
  main:
    controller: main
    type: ClusterIP  # Internal only
    ports:
      grpc:
        port: 50051
        targetPort: grpc
        protocol: TCP

# Persistence
persistence:
  media:
    enabled: true
    type: nfs
    server: 192.168.1.10
    path: /mnt/pool/media
    globalMounts:
      - path: /media
        readOnly: false
  
  models:
    enabled: true
    type: persistentVolumeClaim
    accessMode: ReadWriteOnce
    size: 5Gi
    retain: true
    globalMounts:
      - path: /models
  
  cache:
    enabled: true
    type: emptyDir
    medium: Memory
    sizeLimit: 1Gi
    globalMounts:
      - path: /cache
```

---

### Installation (Phase 2)

**IMPORTANT**: Phase 2 requires RBAC setup before installation.

```bash
# 0. Apply RBAC resources (REQUIRED for Phase 2)
kubectl apply -f deploy/rbac.yaml

# Verify RBAC permissions
kubectl auth can-i get endpoints \
  --as=system:serviceaccount:media:subgen-orchestrator \
  -n media
# Expected: yes

kubectl auth can-i list endpoints \
  --as=system:serviceaccount:media:subgen-orchestrator \
  -n media
# Expected: yes

kubectl auth can-i watch endpoints \
  --as=system:serviceaccount:media:subgen-orchestrator \
  -n media
# Expected: yes

# 1. Install workers first
helm install subgen-worker bjw-s/app-template \
  --namespace media \
  --values deploy/values-phase2-workers.yaml

# 2. Wait for workers to be ready
kubectl wait --for=condition=ready pod \
  -l app.kubernetes.io/name=subgen-worker \
  -n media \
  --timeout=300s

# 3. Install orchestrator
helm install subgen-orchestrator bjw-s/app-template \
  --namespace media \
  --values deploy/values-phase2-orchestrator.yaml

# 4. Verify worker discovery
kubectl logs -n media -l app.kubernetes.io/name=subgen-orchestrator --tail=50
# Should see: "Discovered 3 workers: worker-subgen-worker-0, worker-subgen-worker-1, worker-subgen-worker-2"
```

---

### Scaling Workers (Phase 2)

```bash
# Scale to 5 workers
kubectl scale statefulset subgen-worker --replicas=5 -n media

# Or via Helm
helm upgrade subgen-worker bjw-s/app-template \
  --namespace media \
  --values deploy/values-phase2-workers.yaml \
  --set controllers.main.replicas=5

# Verify scaling
kubectl get pods -n media -l app.kubernetes.io/name=subgen-worker
```

**Orchestrator automatically detects new workers** (no restart needed).

---

## RBAC Configuration (Phase 2 Only)

### Overview

Phase 2 requires the orchestrator to access the Kubernetes API to discover worker pods dynamically. RBAC (Role-Based Access Control) grants these permissions securely.

### What Gets Created

The `deploy/rbac.yaml` file creates three resources:

1. **ServiceAccount** (`subgen-orchestrator`)
   - Identity for the orchestrator pod
   - Authenticates to Kubernetes API
   - Token auto-mounted at `/var/run/secrets/kubernetes.io/serviceaccount/token`

2. **Role** (`subgen-orchestrator`)
   - Defines permissions
   - Read-only access to `endpoints` resource
   - Verbs: `get`, `list`, `watch`
   - Scoped to `media` namespace only

3. **RoleBinding** (`subgen-orchestrator`)
   - Links ServiceAccount to Role
   - Grants orchestrator pod the defined permissions

### Security Considerations

**✅ Follows Principle of Least Privilege:**
- **Read-only**: No write access (`create`, `update`, `patch`, `delete`)
- **Single resource**: Only `endpoints` (no pods, services, secrets, configmaps)
- **Namespace-scoped**: Only `media` namespace (not cluster-wide)
- **Auto-rotated**: Kubernetes handles token lifecycle

**❌ Does NOT grant:**
- Write access to any resources
- Access to secrets or configmaps
- Access to pods directly
- Cluster-wide permissions

### Installation

```bash
# Apply RBAC resources
kubectl apply -f deploy/rbac.yaml

# Verify resources created
kubectl get sa,role,rolebinding -n media | grep subgen-orchestrator
# Expected:
# serviceaccount/subgen-orchestrator
# role.rbac.authorization.k8s.io/subgen-orchestrator
# rolebinding.rbac.authorization.k8s.io/subgen-orchestrator
```

### Verification

Test that the ServiceAccount has correct permissions:

```bash
# Should have read access
kubectl auth can-i get endpoints \
  --as=system:serviceaccount:media:subgen-orchestrator \
  -n media
# Expected: yes

kubectl auth can-i list endpoints \
  --as=system:serviceaccount:media:subgen-orchestrator \
  -n media
# Expected: yes

kubectl auth can-i watch endpoints \
  --as=system:serviceaccount:media:subgen-orchestrator \
  -n media
# Expected: yes

# Should NOT have write access
kubectl auth can-i delete endpoints \
  --as=system:serviceaccount:media:subgen-orchestrator \
  -n media
# Expected: no

kubectl auth can-i create pods \
  --as=system:serviceaccount:media:subgen-orchestrator \
  -n media
# Expected: no

kubectl auth can-i get secrets \
  --as=system:serviceaccount:media:subgen-orchestrator \
  -n media
# Expected: no
```

### Troubleshooting RBAC

**Symptom**: Orchestrator logs show `forbidden: endpoints`

**Cause**: RBAC not applied or ServiceAccount not configured

**Solution**:

```bash
# 1. Verify RBAC resources exist
kubectl get sa,role,rolebinding -n media | grep subgen-orchestrator

# 2. If missing, apply RBAC
kubectl apply -f deploy/rbac.yaml

# 3. Verify orchestrator pod uses ServiceAccount
kubectl get pod -n media -l app.kubernetes.io/name=subgen-orchestrator -o yaml | grep serviceAccountName
# Should show: serviceAccountName: subgen-orchestrator

# 4. If not using ServiceAccount, verify Helm values
grep serviceAccountName deploy/values-phase2-orchestrator.yaml
# Should show: serviceAccountName: subgen-orchestrator

# 5. Restart orchestrator to pick up changes
kubectl rollout restart deployment subgen-orchestrator -n media
```

### How It Works

1. **Pod starts**: Kubernetes mounts ServiceAccount token into pod
2. **Client-go initializes**: Reads token from `/var/run/secrets/kubernetes.io/serviceaccount/token`
3. **API request**: Orchestrator calls `client.CoreV1().Endpoints(namespace).Get()`
4. **K8s API server**: Validates token, checks RBAC permissions
5. **Authorization**: Allows request because Role grants `get endpoints`
6. **Response**: Returns endpoint data to orchestrator

### RBAC File Contents

See `deploy/rbac.yaml` for the complete RBAC configuration. Key sections:

```yaml
# ServiceAccount
apiVersion: v1
kind: ServiceAccount
metadata:
  name: subgen-orchestrator
  namespace: media

---
# Role (permissions)
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
# RoleBinding (grant permissions)
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

## Real-Time Worker Discovery (Watch API)

### Overview

Phase 2 uses Kubernetes Watch API for real-time worker discovery, eliminating the need for periodic polling and enabling instant response to worker scaling events.

### How It Works

```
┌─────────────────────────────────────────────────────────────┐
│  Orchestrator Pod                                           │
├─────────────────────────────────────────────────────────────┤
│  1. Periodic Refresh (30s)                                  │
│     • Full health checks                                    │
│     • Get worker status (active jobs)                       │
│     • Fallback if watch fails                               │
│                                                              │
│  2. Watch Loop (real-time)                                  │
│     • K8s API: Watch Endpoints                              │
│     • Events: Added, Modified, Deleted                      │
│     • No health checks (instant response)                   │
│     • Auto-reconnection with backoff                        │
└─────────────────────────────────────────────────────────────┘
           ↕ gRPC health checks (30s)    ↕ Watch events (instant)
┌──────────────────────────┐  ┌──────────────────────────┐
│  K8s Endpoints API       │  │  K8s Watch API           │
│  GET /api/v1/endpoints   │  │  WATCH /api/v1/endpoints │
└──────────────────────────┘  └──────────────────────────┘
```

### Dual-Mode Discovery

The orchestrator combines two discovery mechanisms:

| Mode | Interval | Purpose | Health Checks |
|------|----------|---------|---------------|
| **Periodic Refresh** | 30 seconds | Full health status, active jobs | Yes (5s timeout per worker) |
| **Watch Events** | Real-time (instant) | Worker additions/removals | No (assumes healthy) |

**Why both?**
- **Watch**: Fast response to scaling events (0-100ms latency)
- **Periodic**: Authoritative health status and load metrics

### Watch Events

The orchestrator responds to three event types:

#### 1. Worker Added (EventTypeAdded)
```go
// When: New worker pod becomes ready
// Action: Add to worker pool immediately
// Metrics: worker_watch_events_total{type="added"}
```

**Timeline:**
```
t=0:    kubectl scale statefulset worker --replicas=5
t=2s:   New pod worker-4 starts
t=15s:  Pod passes readiness probe
t=15.1s: K8s adds IP to Endpoints
t=15.1s: Watch event emitted
t=15.1s: Orchestrator adds worker-4 to pool
        ✓ New jobs can be routed to worker-4
```

#### 2. Worker Removed (EventTypeRemoved)
```go
// When: Worker pod deleted or becomes unready
// Action: Remove from worker pool immediately
// Metrics: worker_watch_events_total{type="removed"}
```

**Timeline:**
```
t=0:    kubectl delete pod worker-2
t=0.1s: K8s removes IP from Endpoints
t=0.1s: Watch event emitted
t=0.1s: Orchestrator removes worker-2 from pool
        ✓ No new jobs routed to worker-2
```

#### 3. Worker Updated (EventTypeUpdated)
```go
// When: Worker health status changes
// Action: Update worker metadata
// Metrics: worker_watch_events_total{type="updated"}
```

**Note:** Currently only detects health changes from periodic refresh, not watch events.

### Watch Reconnection

The watch connection can disconnect due to:
- Kubernetes API server restart
- Network interruptions
- Long-running watch timeout (K8s default: ~30 minutes)

**Reconnection Strategy:**

```
┌─────────────────────────────────────────────────────────────┐
│  Watch Reconnection with Exponential Backoff               │
├─────────────────────────────────────────────────────────────┤
│  Attempt 1:  Wait 1s  → Reconnect                           │
│  Attempt 2:  Wait 2s  → Reconnect                           │
│  Attempt 3:  Wait 4s  → Reconnect                           │
│  Attempt 4:  Wait 8s  → Reconnect                           │
│  Attempt 5:  Wait 16s → Reconnect                           │
│  Attempt 6:  Wait 30s → Reconnect (max backoff)             │
│  ...                                                         │
│  Attempt 10: Wait 30s → Reconnect                           │
│  Attempt 11: STOP - Fall back to periodic refresh only      │
│                                                              │
│  Metrics: worker_watch_reconnects_total                     │
└─────────────────────────────────────────────────────────────┘
```

**Configuration:**
- **Initial backoff**: 1 second
- **Max backoff**: 30 seconds (capped)
- **Max retries**: 10 attempts
- **Fallback**: Periodic refresh continues working

**Why 10 retries?**
- Prevents infinite reconnection loops on persistent K8s API failures
- After 10 failures, something is fundamentally wrong (investigate manually)
- Periodic refresh (30s) still provides eventual consistency

### Watch Metrics

Monitor watch health via Prometheus:

```promql
# Watch events by type
rate(subgen_worker_watch_events_total[5m])

# Watch reconnections (should be rare)
rate(subgen_worker_watch_reconnects_total[5m])

# Watch errors from K8s API
rate(subgen_worker_watch_errors_total[5m])

# Worker discovery errors (fallback to periodic)
rate(subgen_worker_discovery_errors_total[5m])
```

**Alert rules:**

```yaml
# High watch reconnection rate
- alert: HighWatchReconnectionRate
  expr: rate(subgen_worker_watch_reconnects_total[5m]) > 0.1
  for: 10m
  annotations:
    summary: "Orchestrator watch reconnecting frequently"
    description: "Watch reconnections: {{ $value }}/s over 5m"

# Watch fallback (no events for 10 minutes)
- alert: WatchEventsStopped
  expr: rate(subgen_worker_watch_events_total[10m]) == 0
  for: 15m
  annotations:
    summary: "No watch events received (possible watch failure)"
    description: "May have fallen back to periodic refresh only"
```

### Performance Comparison

| Scenario | Periodic Only (30s) | Watch + Periodic |
|----------|---------------------|------------------|
| Worker scale up (3→5) | 0-30s delay | 0-2s delay (instant) |
| Worker crash | 0-30s delay | 0-1s delay (instant) |
| Health check overhead | All workers every 30s | All workers every 30s |
| Network overhead | Endpoints GET every 30s | Endpoints GET + Watch stream |
| Resilience | Always works | Fallback to periodic on failure |

**Best of both worlds:**
- Watch provides instant response (UX)
- Periodic provides authoritative health (correctness)

### Troubleshooting Watch

#### Watch Not Connecting

**Symptom:** Logs show `Worker watch not supported`

**Check RBAC permissions:**
```bash
kubectl auth can-i watch endpoints \
  --as=system:serviceaccount:media:subgen-orchestrator \
  -n media
# Expected: yes
```

**If no:**
```bash
# Apply RBAC
kubectl apply -f deploy/rbac.yaml

# Restart orchestrator
kubectl rollout restart deployment subgen-orchestrator -n media
```

#### Watch Reconnecting Frequently

**Symptom:** Metrics show high `worker_watch_reconnects_total`

**Possible causes:**
- Kubernetes API server instability
- Network issues between pod and API server
- K8s version too old (watch timeouts)

**Check K8s API logs:**
```bash
# On control plane node
kubectl logs -n kube-system kube-apiserver-xxx
```

**Workaround:**
- Watch fallback handles this gracefully
- Periodic refresh (30s) continues working
- No action needed unless reconnections > 1/minute

#### Watch Events Not Received

**Symptom:** Worker scaled but orchestrator doesn't see it

**Verify watch is running:**
```bash
kubectl logs -n media -l app.kubernetes.io/name=subgen-orchestrator | grep -i watch
# Should see:
# level=info msg="Starting Kubernetes endpoints watch for dynamic worker discovery"
# level=info msg="Kubernetes watch established successfully"
```

**Check for watch errors:**
```bash
kubectl logs -n media -l app.kubernetes.io/name=subgen-orchestrator | grep -i "watch error"
```

**Verify Endpoints exist:**
```bash
kubectl get endpoints -n media subgen-worker -o yaml
# Should show IPs in subsets.addresses[]
```

#### Max Retries Exceeded

**Symptom:** Logs show `Max watch reconnection attempts exceeded, falling back to periodic refresh only`

**Impact:**
- Watch disabled (no real-time events)
- Periodic refresh continues (30s delay)
- System still functional, just slower

**Action required:**
```bash
# 1. Check K8s API health
kubectl get --raw /healthz

# 2. Check orchestrator pod events
kubectl describe pod -n media -l app.kubernetes.io/name=subgen-orchestrator

# 3. Restart orchestrator to retry
kubectl rollout restart deployment subgen-orchestrator -n media
```

---

## Persistent Storage

### NFS Mount (Media Files)

**Configuration:**

```yaml
persistence:
  media:
    enabled: true
    type: nfs
    server: 192.168.1.10      # NAS IP
    path: /mnt/pool/media     # NFS export path
    advancedMounts:
      main:
        orchestrator:
          - path: /media
            readOnly: false   # RW for subtitle writing
        worker:
          - path: /media
            readOnly: false
```

**Permissions:**

```bash
# On NAS server, ensure export allows RW from K8s nodes
# /etc/exports
/mnt/pool/media 192.168.1.0/24(rw,sync,no_subtree_check,no_root_squash)

# Restart NFS server
sudo exportfs -ra
sudo systemctl restart nfs-server
```

**Troubleshooting:**

```bash
# Test NFS mount from K8s node
sudo mount -t nfs 192.168.1.10:/mnt/pool/media /mnt/test

# Check mount in pod
kubectl exec -it -n media subgen-main-xxx -c worker -- ls -la /media

# Check NFS mount details
kubectl exec -it -n media subgen-main-xxx -c worker -- mount | grep media
```

---

### PVC (Whisper Models)

**Configuration:**

```yaml
persistence:
  models:
    enabled: true
    type: persistentVolumeClaim
    accessMode: ReadWriteOnce
    size: 5Gi
    storageClass: local-path  # or your storage class
    retain: true
    advancedMounts:
      main:
        worker:
          - path: /models
```

**Storage Classes:**

| Storage Class | Type | Use Case |
|---------------|------|----------|
| `local-path` | Hostpath | Single-node, fast, local storage |
| `nfs-client` | NFS | Multi-node, shared storage |
| `longhorn` | Distributed | Multi-node, replicated, resilient |

**Check Storage:**

```bash
# List PVCs
kubectl get pvc -n media

# Check PVC details
kubectl describe pvc subgen-models -n media

# Check model files in pod
kubectl exec -it -n media subgen-main-xxx -c worker -- ls -la /models
```

---

### tmpfs Cache (In-Memory)

**Configuration:**

```yaml
persistence:
  cache:
    enabled: true
    type: emptyDir
    medium: Memory       # tmpfs
    sizeLimit: 1Gi       # Hard limit
    globalMounts:
      - path: /cache
```

**Purpose:**
- Fast model loading (tmpfs = RAM speed)
- HuggingFace cache
- Matplotlib config
- Temporary files

**Tradeoff:**
- ✅ Fast (RAM speed)
- ❌ Lost on pod restart
- ❌ Counts against memory limit

---

## Secrets Management

### Creating Secrets

**Method 1: kubectl (Simple)**

```bash
kubectl create secret generic subgen-secrets \
  --namespace media \
  --from-literal=PLEX_TOKEN='your-plex-token-here' \
  --from-literal=JELLYFIN_TOKEN='your-jellyfin-token-here'
```

**Method 2: YAML (Version Control)**

```yaml
# deploy/secret-example.yaml (DO NOT COMMIT ACTUAL SECRETS)
apiVersion: v1
kind: Secret
metadata:
  name: subgen-secrets
  namespace: media
type: Opaque
stringData:
  PLEX_TOKEN: "your-plex-token-here"
  JELLYFIN_TOKEN: "your-jellyfin-token-here"
```

```bash
kubectl apply -f deploy/secret.yaml
```

**Method 3: Sealed Secrets (Production)**

```bash
# Install Sealed Secrets controller
kubectl apply -f https://github.com/bitnami-labs/sealed-secrets/releases/download/v0.24.0/controller.yaml

# Create sealed secret
kubectl create secret generic subgen-secrets \
  --namespace media \
  --from-literal=PLEX_TOKEN='...' \
  --dry-run=client -o yaml | \
  kubeseal -o yaml > deploy/sealed-secret.yaml

# Commit sealed-secret.yaml (safe to commit!)
git add deploy/sealed-secret.yaml
git commit -m "Add sealed secrets"
```

---

### Using Secrets in Pods

**values.yaml:**

```yaml
containers:
  orchestrator:
    envFrom:
      - secretRef:
          name: subgen-secrets
```

**Verification:**

```bash
# Check environment variables in pod
kubectl exec -it -n media subgen-main-xxx -c orchestrator -- env | grep TOKEN
```

---

## Monitoring & Observability

### Prometheus Integration

**ServiceMonitor (values.yaml):**

```yaml
serviceMonitor:
  main:
    enabled: true
    serviceName: subgen-main
    labels:
      release: prometheus  # Match your Prometheus Operator selector
    endpoints:
      - port: metrics
        scheme: http
        path: /metrics
        interval: 30s
        scrapeTimeout: 10s
```

**Verify Scraping:**

```bash
# Check ServiceMonitor created
kubectl get servicemonitor -n media

# Check Prometheus targets (if using Prometheus UI)
# Navigate to: Status → Targets
# Look for: serviceMonitor/media/subgen-main/0
```

---

### Grafana Dashboard

**Metrics to visualize:**

```promql
# Queue size
subgen_queue_size

# Worker memory
subgen_worker_memory_mb

# Transcription duration
histogram_quantile(0.95, rate(subgen_transcription_duration_seconds_bucket[5m]))

# Error rate
rate(subgen_grpc_errors_total[5m])

# Worker health
subgen_worker_healthy
```

**Dashboard JSON:** (to be created in EPIC_04)

---

### Logging

**Structured JSON logs:**

```json
{
  "timestamp": "2026-02-15T10:30:00Z",
  "level": "info",
  "component": "orchestrator",
  "message": "task queued",
  "task_id": "abc123",
  "file_path": "/media/tv/show/episode.mkv"
}
```

**Log aggregation options:**

1. **kubectl logs** (simple, no aggregation)
2. **Loki** (Grafana stack, lightweight)
3. **ELK Stack** (Elasticsearch, full-featured)
4. **Fluentd/Fluent Bit** (forwarding to external systems)

**View logs:**

```bash
# Orchestrator logs
kubectl logs -n media -l app.kubernetes.io/name=subgen -c orchestrator --tail=100 -f

# Worker logs
kubectl logs -n media -l app.kubernetes.io/name=subgen -c worker --tail=100 -f

# All containers
kubectl logs -n media -l app.kubernetes.io/name=subgen --all-containers=true --tail=100 -f
```

---

## Troubleshooting

### Pod Won't Start

**Check pod status:**

```bash
kubectl get pods -n media
# NAME                READY   STATUS             RESTARTS   AGE
# subgen-main-xxx     1/2     CrashLoopBackOff   3          2m
```

**Check events:**

```bash
kubectl describe pod -n media subgen-main-xxx
# Events:
#   Type     Reason     Message
#   Warning  BackOff    Back-off restarting failed container
```

**Check logs:**

```bash
# Which container failed?
kubectl logs -n media subgen-main-xxx -c orchestrator --previous
kubectl logs -n media subgen-main-xxx -c worker --previous
```

**Common issues:**

| Symptom | Cause | Fix |
|---------|-------|-----|
| `ImagePullBackOff` | Image doesn't exist | Check image repository and tag |
| `CrashLoopBackOff` | Container exits immediately | Check logs for errors |
| `Init:0/1` | Init container failed | Not used in subgen |
| `CreateContainerConfigError` | Secret missing | Create secret first |
| `Pending` (no events) | No nodes available | Check node capacity |

---

### Worker Not Discovered (Phase 2)

**Symptom:** Orchestrator logs show "no healthy workers found"

**Check Service endpoints:**

```bash
kubectl get endpoints -n media subgen-worker
# NAME             ENDPOINTS
# subgen-worker    10.244.1.5:50051,10.244.1.6:50051,10.244.1.7:50051
```

**If no endpoints:**

```bash
# Check worker pods running
kubectl get pods -n media -l app.kubernetes.io/name=subgen-worker

# Check Service selector matches pods
kubectl get svc -n media subgen-worker -o yaml | grep selector
kubectl get pods -n media --show-labels
```

**Check orchestrator RBAC:**

```bash
# Orchestrator needs permission to read Endpoints
kubectl auth can-i get endpoints --as=system:serviceaccount:media:default -n media
# yes
```

If "no", create ServiceAccount and RoleBinding:

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

Then update values.yaml:

```yaml
defaultPodOptions:
  serviceAccountName: subgen-orchestrator
```

---

### NFS Mount Fails

**Symptom:** Pod stuck in `ContainerCreating`

```bash
kubectl describe pod -n media subgen-main-xxx
# Events:
#   Warning  FailedMount  Unable to attach or mount volumes: ... permission denied
```

**Check NFS server:**

```bash
# On NFS server
showmount -e 192.168.1.10
# Export list for 192.168.1.10:
# /mnt/pool/media 192.168.1.0/24

# Check export options
cat /etc/exports
# /mnt/pool/media 192.168.1.0/24(rw,sync,no_subtree_check,no_root_squash)
```

**Test mount from K8s node:**

```bash
# SSH to K8s node
sudo mount -t nfs 192.168.1.10:/mnt/pool/media /mnt/test
ls /mnt/test
```

**Fix permission denied:**

```bash
# On NFS server, add no_root_squash
# /etc/exports
/mnt/pool/media 192.168.1.0/24(rw,sync,no_subtree_check,no_root_squash)

sudo exportfs -ra
```

---

### Worker OOM Killed

**Symptom:** Worker pod restarts frequently

```bash
kubectl describe pod -n media subgen-main-xxx
# Containers:
#   worker:
#     Last State:     Terminated
#       Reason:       OOMKilled
#       Exit Code:    137
```

**Check memory usage:**

```bash
kubectl top pod -n media subgen-main-xxx --containers
# POD                 NAME          CPU    MEMORY
# subgen-main-xxx     orchestrator  10m    50Mi
# subgen-main-xxx     worker        800m   4000Mi  # ← At limit!
```

**Solutions:**

1. **Increase memory limit** (temporary fix):
   ```yaml
   resources:
     limits:
       memory: 6Gi  # Increase from 4Gi
   ```

2. **Fix memory leak** (proper fix):
   - Check worker logs for memory growth
   - Ensure model cleanup is working
   - Add memory monitoring alerts

3. **Use smaller model**:
   ```yaml
   env:
     WHISPER_MODEL: "small"  # Instead of "medium"
   ```

---

### Webhook Not Reachable

**Symptom:** Plex/Jellyfin webhooks timeout

**Check Service external IP:**

```bash
kubectl get svc -n media subgen-main
# NAME          TYPE           EXTERNAL-IP     PORT(S)
# subgen-main   LoadBalancer   <pending>       9000:30100/TCP
```

**If `<pending>`:**

1. **No LoadBalancer controller** (MetalLB, cloud provider):
   ```bash
   # Install MetalLB
   kubectl apply -f https://raw.githubusercontent.com/metallb/metallb/v0.13.12/config/manifests/metallb-native.yaml
   
   # Configure IP pool
   kubectl apply -f - <<EOF
   apiVersion: metallb.io/v1beta1
   kind: IPAddressPool
   metadata:
     name: default
     namespace: metallb-system
   spec:
     addresses:
     - 192.168.1.100-192.168.1.110
   ---
   apiVersion: metallb.io/v1beta1
   kind: L2Advertisement
   metadata:
     name: default
     namespace: metallb-system
   EOF
   ```

2. **Use NodePort instead:**
   ```yaml
   service:
     main:
       type: NodePort
       ports:
         http:
           nodePort: 30900
   ```
   
   Access via: `http://<node-ip>:30900`

**Test connectivity:**

```bash
# From external network
curl -I http://192.168.1.100:9000/health

# From inside cluster
kubectl run -it --rm debug --image=curlimages/curl --restart=Never -- \
  curl http://subgen-main.media.svc.cluster.local:9000/health
```

---

## Summary

### Deployment Checklist

**Phase 1:**
- [ ] Add bjw-s Helm repo
- [ ] Create namespace (`media`)
- [ ] Create secrets (Plex/Jellyfin tokens)
- [ ] Update values.yaml with your NFS server IP
- [ ] Install chart with Phase 1 values
- [ ] Verify pods running
- [ ] Test health endpoint
- [ ] Configure Plex/Jellyfin webhooks
- [ ] Test full pipeline

**Phase 2:**
- [ ] Install worker StatefulSet
- [ ] Wait for workers to be ready
- [ ] Install orchestrator Deployment
- [ ] Verify worker discovery in logs
- [ ] Test load balancing across workers
- [ ] Scale workers and verify auto-discovery

---

### Key Files

| File | Purpose |
|------|---------|
| `deploy/values-phase1.yaml` | Single-pod deployment |
| `deploy/values-phase2-orchestrator.yaml` | Orchestrator deployment (Phase 2) |
| `deploy/values-phase2-workers.yaml` | Worker StatefulSet (Phase 2) |
| `deploy/secret-example.yaml` | Secret template (DO NOT COMMIT REAL VALUES) |
| `deploy/README.md` | Deployment instructions |

---

### Next Steps

1. Create `deploy/` directory structure (EPIC_04)
2. Test Phase 1 deployment on local cluster (EPIC_04 STORY_03)
3. Document migration from Docker to K8s (EPIC_05)
4. Create Grafana dashboard for monitoring (EPIC_04 STORY_04)

---

**Status:** Ready for implementation  
**Related Epics:** EPIC_04, EPIC_05  
**Owner:** TBD
