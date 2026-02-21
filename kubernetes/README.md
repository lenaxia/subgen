# Subgen Kubernetes Deployment Files

This directory contains Helm values files and Kubernetes manifests for deploying Subgen.

---

## Files Overview

| File | Purpose | Phase |
|------|---------|-------|
| `values.yaml` | Single-pod deployment (orchestrator + worker) | Phase 1 |
| `values-phase2-orchestrator.yaml` | Orchestrator-only deployment | Phase 2 |
| `values-phase2-workers.yaml` | Worker StatefulSet deployment | Phase 2 |
| `rbac.yaml` | RBAC resources (ServiceAccount, Role, RoleBinding) | Phase 2 |

---

## Phase 1 Deployment (Recommended for Getting Started)

**Architecture**: Single pod with 2 containers (orchestrator + worker)

**Best for**:
- Small to medium transcription workloads
- Getting started with Subgen
- Single-node Kubernetes clusters
- Testing and development

### Prerequisites

1. **Kubernetes cluster** with kubectl configured
2. **Helm 3** installed
3. **MetalLB** (or cloud LoadBalancer) for external IP
4. **NFS server** for media file access
5. **Storage class** for PVC (e.g., local-path, nfs-client)

### Installation Steps

```bash
# 1. Add bjw-s Helm repository
helm repo add bjw-s https://bjw-s-labs.github.io/helm-charts
helm repo update

# 2. Create namespace
kubectl create namespace media

# 3. Create secret with Plex/Jellyfin tokens
kubectl create secret generic subgen-secrets \
  --namespace media \
  --from-literal=PLEX_TOKEN='your-plex-token-here' \
  --from-literal=JELLYFIN_TOKEN='your-jellyfin-token-here'

# 4. Edit values.yaml and update these fields:
#    - metallb.universe.tf/loadBalancerIPs (your static IP)
#    - persistence.media.server (your NAS IP)
#    - persistence.media.path (your NFS export path)
#    - persistence.models.storageClass (your storage class)
#    - image repositories (if using custom images)
nano deploy/values.yaml

# 5. Install Subgen
helm install subgen bjw-s/app-template \
  --namespace media \
  --values deploy/values.yaml

# 6. Watch deployment
kubectl get pods -n media -l app.kubernetes.io/name=subgen -w

# 7. Check logs
kubectl logs -n media -l app.kubernetes.io/name=subgen -c orchestrator --tail=50 -f
kubectl logs -n media -l app.kubernetes.io/name=subgen -c worker --tail=50 -f
```

### Verification

```bash
# 1. Get service external IP
kubectl get svc -n media
# NAME          TYPE           EXTERNAL-IP      PORT(S)
# subgen-main   LoadBalancer   192.168.1.100    9000:30xxx/TCP

# 2. Test health endpoint
curl http://192.168.1.100:9000/health
# Expected: {"status": "healthy", ...}

# 3. Test metrics endpoint
curl http://192.168.1.100:9090/metrics
# Expected: Prometheus metrics

# 4. Configure Plex/Jellyfin webhooks
# Point to: http://192.168.1.100:9000/plex (or /jellyfin)
```

---

## Phase 2 Deployment (For Production/Scaling)

**Architecture**: Separate orchestrator and worker deployments

**Best for**:
- High transcription workloads
- Horizontal scaling (multiple workers)
- Production environments
- Multi-node Kubernetes clusters

### Prerequisites

All Phase 1 prerequisites, plus:
- **RBAC enabled** in your cluster (standard in most K8s distributions)

### Installation Steps

```bash
# 1-3. Same as Phase 1 (add Helm repo, create namespace, create secrets)

# 4. Apply RBAC resources (REQUIRED for Phase 2)
kubectl apply -f deploy/rbac.yaml

# 5. Verify RBAC permissions
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

# 6. Edit Phase 2 values files
nano deploy/values-phase2-orchestrator.yaml  # Update IPs, storage, etc.
nano deploy/values-phase2-workers.yaml       # Update IPs, storage, replicas

# 7. Install workers FIRST
helm install subgen-worker bjw-s/app-template \
  --namespace media \
  --values deploy/values-phase2-workers.yaml

# 8. Wait for workers to be ready
kubectl wait --for=condition=ready pod \
  -l app.kubernetes.io/name=subgen-worker \
  -n media \
  --timeout=300s

# 9. Install orchestrator
helm install subgen-orchestrator bjw-s/app-template \
  --namespace media \
  --values deploy/values-phase2-orchestrator.yaml

# 10. Verify worker discovery
kubectl logs -n media -l app.kubernetes.io/name=subgen-orchestrator --tail=50
# Should see: "Discovered N workers from K8s Endpoints API"
```

### Scaling Workers

```bash
# Option 1: kubectl scale
kubectl scale statefulset subgen-worker --replicas=5 -n media

# Option 2: Helm upgrade
helm upgrade subgen-worker bjw-s/app-template \
  --namespace media \
  --values deploy/values-phase2-workers.yaml \
  --set controllers.main.replicas=5

# Verify scaling
kubectl get pods -n media -l app.kubernetes.io/name=subgen-worker
# Should show 5 worker pods

# Check orchestrator discovered new workers (real-time via Watch API)
kubectl logs -n media -l app.kubernetes.io/name=subgen-orchestrator --tail=20
# Should see: "New worker discovered: subgen-worker-3"
# Should see: "New worker discovered: subgen-worker-4"
# Should see: "Discovered 5 workers from K8s Endpoints API"
```

**Note**: Orchestrator uses Kubernetes Watch API for real-time worker discovery. New workers are detected within seconds (typically <30s) without requiring orchestrator restart. The watch automatically reconnects if the connection is lost.

---

## RBAC Details (Phase 2 Only)

### What is RBAC?

Role-Based Access Control (RBAC) allows the orchestrator pod to read Kubernetes API resources.

### What Does rbac.yaml Create?

1. **ServiceAccount** (`subgen-orchestrator`)
   - Identity for orchestrator pod
   - Used to authenticate to K8s API

2. **Role** (`subgen-orchestrator`)
   - Grants read-only access to `endpoints` resource
   - Scoped to `media` namespace only
   - Permissions: `get`, `list`, `watch`

3. **RoleBinding** (`subgen-orchestrator`)
   - Links ServiceAccount to Role
   - Grants permissions to orchestrator pod

### Security Notes

- ✅ **Least privilege**: Read-only access to endpoints only
- ✅ **Namespace-scoped**: No cluster-wide access
- ✅ **No write access**: Cannot modify endpoints, pods, or services
- ✅ **No secret access**: Cannot read Kubernetes secrets
- ✅ **Auto-rotated tokens**: K8s handles token lifecycle

### Verifying RBAC

```bash
# Check ServiceAccount exists
kubectl get sa -n media subgen-orchestrator

# Check Role exists
kubectl get role -n media subgen-orchestrator

# Check RoleBinding exists
kubectl get rolebinding -n media subgen-orchestrator

# Test specific permissions
kubectl auth can-i get endpoints \
  --as=system:serviceaccount:media:subgen-orchestrator \
  -n media
# Expected: yes

kubectl auth can-i delete endpoints \
  --as=system:serviceaccount:media:subgen-orchestrator \
  -n media
# Expected: no (read-only)
```

---

## Troubleshooting

### Pod Won't Start

```bash
# Check pod status
kubectl get pods -n media

# Check events
kubectl describe pod <pod-name> -n media

# Check logs
kubectl logs <pod-name> -n media -c orchestrator
kubectl logs <pod-name> -n media -c worker
```

### RBAC Permission Denied (Phase 2)

**Symptom**: Orchestrator logs show "forbidden: endpoints"

**Solution**:
```bash
# 1. Verify RBAC applied
kubectl get sa,role,rolebinding -n media | grep subgen-orchestrator

# 2. If missing, apply RBAC
kubectl apply -f deploy/rbac.yaml

# 3. Restart orchestrator
kubectl rollout restart deployment subgen-orchestrator -n media
```

### Workers Not Discovered (Phase 2)

**Symptom**: Orchestrator logs show "no healthy workers found"

**Check**:
```bash
# 1. Verify workers are running
kubectl get pods -n media -l app.kubernetes.io/name=subgen-worker

# 2. Check Service endpoints
kubectl get endpoints -n media subgen-worker
# Should show worker pod IPs

# 3. Verify Service selector matches pods
kubectl get svc subgen-worker -n media -o yaml | grep selector
kubectl get pods -n media --show-labels | grep subgen-worker
# Selectors must match

# 4. Check orchestrator RBAC
kubectl auth can-i get endpoints \
  --as=system:serviceaccount:media:subgen-orchestrator \
  -n media
# Must return: yes
```

### NFS Mount Fails

**Symptom**: Pod stuck in `ContainerCreating`, events show mount errors

**Solutions**:
```bash
# 1. Test NFS mount from K8s node
ssh <node-ip>
sudo mount -t nfs 192.168.1.10:/mnt/pool/media /mnt/test
ls /mnt/test  # Should show media files
sudo umount /mnt/test

# 2. Check NFS exports on server
showmount -e 192.168.1.10

# 3. Ensure no_root_squash in /etc/exports (on NFS server)
# /mnt/pool/media 192.168.1.0/24(rw,sync,no_subtree_check,no_root_squash)
```

---

## Configuration Quick Reference

### Required Customizations

Before deploying, update these values in values files:

| Field | Location | Example | Description |
|-------|----------|---------|-------------|
| **LoadBalancer IP** | `service.main.annotations` | `192.168.1.100` | Static IP for orchestrator |
| **NFS server** | `persistence.media.server` | `192.168.1.10` | Your NAS IP |
| **NFS path** | `persistence.media.path` | `/mnt/pool/media` | NFS export path |
| **Storage class** | `persistence.models.storageClass` | `local-path` | K8s storage class |
| **Plex server** | `env.PLEX_SERVER` | `http://plex:32400` | Plex URL |
| **Jellyfin server** | `env.JELLYFIN_SERVER` | `http://jellyfin:8096` | Jellyfin URL |
| **Image repos** | `image.repository` | `ghcr.io/user/image` | Container registry |
| **Replicas** (Phase 2) | `controllers.main.replicas` | `3` | Number of workers |

### Optional Customizations

| Field | Default | Options | Description |
|-------|---------|---------|-------------|
| **Whisper model** | `medium` | `tiny`, `base`, `small`, `medium`, `large`, `large-v3` | Accuracy vs speed |
| **Transcribe device** | `cpu` | `cpu`, `cuda` | GPU acceleration |
| **Queue size** | `1000` (Phase 1)<br>`5000` (Phase 2) | Any integer | Max queued jobs |
| **Memory limit** | `4Gi` | Any memory size | Worker memory |
| **CPU limit** | `2000m` | Any CPU count | Worker CPU |

---

## Migration from Docker Compose

If migrating from Docker Compose to Kubernetes:

1. **Export configuration**: Note all environment variables from docker-compose.yml
2. **Transfer secrets**: Create K8s secret with Plex/Jellyfin tokens
3. **Map volumes**: Configure NFS persistence for media files
4. **Start with Phase 1**: Test single-pod deployment first
5. **Scale to Phase 2**: Once stable, migrate to Phase 2 for scaling

**Key differences**:
- Docker: `WORKER_DISCOVERY=localhost` (default)
- K8s Phase 1: `WORKER_DISCOVERY=localhost`
- K8s Phase 2: `WORKER_DISCOVERY=kubernetes`

---

## Monitoring

### Prometheus Metrics

Orchestrator exposes metrics on port 9090:

```bash
# Access metrics directly
kubectl port-forward -n media svc/subgen-orchestrator 9090:9090
curl http://localhost:9090/metrics
```

If using Prometheus Operator, ServiceMonitor is auto-configured.

### Key Metrics

- `subgen_queue_size` - Jobs in queue
- `subgen_worker_memory_mb` - Worker memory usage
- `subgen_transcription_duration_seconds` - Job duration
- `subgen_worker_healthy` - Worker health status

---

## Support

For more details, see:
- **Deployment guide**: `docs/DESIGN/04_K8S_DEPLOYMENT.md`
- **Architecture**: `docs/DESIGN/00_HYBRID_ARCHITECTURE.md`
- **Main README**: `../README.md`

For issues, check:
- Orchestrator logs: `kubectl logs -n media -l app.kubernetes.io/name=subgen-orchestrator`
- Worker logs: `kubectl logs -n media -l app.kubernetes.io/name=subgen-worker`
- Pod events: `kubectl describe pod <pod-name> -n media`
