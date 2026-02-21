#!/bin/bash
# Check if v0.2.15 release is available

set -e

echo "=== Checking v0.2.15 Release Status ==="
echo

# Check git tag
echo "1. Git tag status:"
if git tag -l | grep -q v0.2.15; then
    echo "   ✓ v0.2.15 tag exists locally"
else
    echo "   ✗ v0.2.15 tag not found locally"
fi

# Check if tag is pushed
echo "2. Checking remote tags..."
git ls-remote --tags origin | grep v0.2.15 && echo "   ✓ v0.2.15 tag is on remote" || echo "   ✗ v0.2.15 tag not on remote"

echo
echo "3. Release workflow should be triggered by tag push."
echo "   Check GitHub Actions at: https://github.com/lenaxia/subgen/actions"
echo

# Check current deployed versions
echo "4. Current deployment versions:"
echo "   Orchestrator:"
kubectl get deployment subgen-orchestrator -n default -o jsonpath='{.spec.template.spec.containers[0].image}' 2>/dev/null || echo "   Not found"
echo
echo "   Worker:"
kubectl get deployment subgen-worker -n default -o jsonpath='{.spec.template.spec.containers[0].image}' 2>/dev/null || echo "   Not found (may still be StatefulSet)"
echo

# Check if worker is StatefulSet or Deployment
echo "5. Checking worker type:"
if kubectl get statefulset subgen-worker -n default &>/dev/null; then
    echo "   ⚠️  Worker is still a StatefulSet"
    echo "   Run: kubectl apply -f deploy-working.yaml to migrate to Deployment"
elif kubectl get deployment subgen-worker -n default &>/dev/null; then
    echo "   ✓ Worker is a Deployment"
else
    echo "   ✗ Worker not found"
fi
echo

# Wait for release and update instructions
echo "6. Once release completes, update deployments:"
echo "   kubectl set image deployment/subgen-orchestrator -n default orchestrator=ghcr.io/lenaxia/subgen-orchestrator:v0.2.15"
echo "   kubectl set image deployment/subgen-worker -n default worker=ghcr.io/lenaxia/subgen-worker:v0.2.15-cpu"
echo
echo "7. Then enable autoscaling:"
echo "   kubectl apply -f hpa-worker.yaml"
echo
echo "=== Check Complete ==="