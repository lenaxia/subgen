# STORY_02: RBAC Configuration

**Epic:** EPIC_09  
**Status:** Not Started  
**Assignee:** TBD  
**Effort:** 3-4 hours

---

## User Story

As an **orchestrator pod**,  
I want to **have Kubernetes API permissions to read Endpoints**,  
So that **I can discover worker pods without authentication errors**.

---

## Acceptance Criteria

- [ ] ServiceAccount created for orchestrator
- [ ] Role created with Endpoints read permissions
- [ ] RoleBinding links ServiceAccount to Role
- [ ] orchestrator pod uses new ServiceAccount
- [ ] `kubectl auth can-i` test passes
- [ ] orchestrator logs show successful K8s API access
- [ ] RBAC files documented in deploy/

---

## Technical Design

### RBAC Resources

Create `deploy/rbac.yaml`:

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

### bjw-s Values Update

Update `deploy/values-phase2-orchestrator.yaml`:

```yaml
defaultPodOptions:
  serviceAccountName: subgen-orchestrator
  automountServiceAccountToken: true  # Required for K8s API access
```

---

## Testing Strategy

### Manual Verification

```bash
# 1. Apply RBAC
kubectl apply -f deploy/rbac.yaml

# 2. Test permissions
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

kubectl auth can-i delete endpoints \
  --as=system:serviceaccount:media:subgen-orchestrator \
  -n media
# Expected: no
```

### Integration Test

```bash
# Deploy orchestrator with new SA
helm upgrade subgen-orchestrator bjw-s/app-template \
  --namespace media \
  --values deploy/values-phase2-orchestrator.yaml

# Check orchestrator can access K8s API
kubectl logs -n media -l app.kubernetes.io/name=subgen-orchestrator --tail=20
# Should NOT see: "failed to get in-cluster config"
# Should NOT see: "forbidden: endpoints"
# Should see: "Discovered N workers from K8s"
```

---

## Security Considerations

### Principle of Least Privilege

**Only grant:**
- Read access (`get`, `list`, `watch`)
- To `endpoints` resource only
- In `media` namespace only

**Do NOT grant:**
- Write access (`create`, `update`, `patch`, `delete`)
- Access to other resources (pods, services, secrets)
- Cluster-wide access (use Role, not ClusterRole)

### Service Account Token

- Token auto-mounted at `/var/run/secrets/kubernetes.io/serviceaccount/token`
- Used by K8s client-go automatically
- Valid only while pod is running
- Rotated by K8s automatically

---

## Files to Create/Modify

- `deploy/rbac.yaml` - RBAC resources (NEW)
- `deploy/values-phase2-orchestrator.yaml` - ServiceAccount config
- `deploy/README.md` - Document RBAC setup
- `docs/DESIGN/04_K8S_DEPLOYMENT.md` - Update with RBAC section

---

## Documentation Updates

### Update Deployment Instructions

Add to `deploy/README.md`:

```markdown
## RBAC Setup (Phase 2 Only)

Phase 2 requires orchestrator to access K8s API:

1. Apply RBAC resources:
   ```bash
   kubectl apply -f deploy/rbac.yaml
   ```

2. Verify permissions:
   ```bash
   kubectl auth can-i get endpoints --as=system:serviceaccount:media:subgen-orchestrator -n media
   ```

3. Install orchestrator (will use ServiceAccount):
   ```bash
   helm install subgen-orchestrator bjw-s/app-template \
     --namespace media \
     --values deploy/values-phase2-orchestrator.yaml
   ```
```

---

## Definition of Done

- [ ] RBAC files created (`deploy/rbac.yaml`)
- [ ] bjw-s values updated (serviceAccountName)
- [ ] Manual verification tests pass
- [ ] Integration test passes (orchestrator accesses K8s API)
- [ ] Documentation updated
- [ ] Security review complete
- [ ] Work log created

---

**Story Owner:** TBD  
**Created:** 2026-02-17
