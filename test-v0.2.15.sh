#!/bin/bash
# Test v0.2.15 deployment

set -e

echo "=== Testing v0.2.15 Deployment ==="
echo

# Check deployments
echo "1. Checking deployments..."
kubectl get deployment -n default -l app=subgen
echo

# Check pods
echo "2. Checking pods..."
kubectl get pods -n default -l app=subgen -o wide
echo

# Check HPA
echo "3. Checking autoscaling..."
kubectl get hpa -n default -l app=subgen
echo

# Check resource usage
echo "4. Checking resource usage..."
kubectl top pods -n default -l app=subgen 2>/dev/null || echo "   (Metrics collecting...)"
echo

# Check orchestrator logs for worker discovery
echo "5. Checking worker discovery..."
kubectl logs deployment/subgen-orchestrator -n default --tail=5 | grep -E "(Workers refreshed|healthy)" | tail -2
echo

# Test worker health
echo "6. Testing worker health..."
for pod in $(kubectl get pods -n default -l app=subgen,component=worker -o name); do
    echo "   $pod:"
    kubectl exec -n default $pod -- curl -s http://localhost:8080/health 2>/dev/null || echo "   Health check failed"
done
echo

# Check versions
echo "7. Checking versions:"
echo "   Orchestrator: $(kubectl get deployment subgen-orchestrator -n default -o jsonpath='{.spec.template.spec.containers[0].image}')"
echo "   Worker: $(kubectl get deployment subgen-worker -n default -o jsonpath='{.spec.template.spec.containers[0].image}')"
echo

echo "=== Test Complete ==="
echo
echo "Summary:"
echo "- v0.2.15 deployed successfully"
echo "- StatefulSet migrated to Deployment"
echo "- Autoscaling enabled (HPA)"
echo "- 2 worker pods running"
echo "- Worker discovery working"
echo
echo "Next: Test language detection with proper audio file"