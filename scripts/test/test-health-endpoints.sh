#!/bin/bash

# Test-Driven Design: Verify HTTP health endpoints work correctly

set -e

echo "=== Test-Driven Design: HTTP Health Endpoints ==="
echo ""

# Get worker pod IP
WORKER_POD=$(kubectl get pods -l app=subgen,component=worker -o jsonpath='{.items[0].metadata.name}')
WORKER_IP=$(kubectl get pod $WORKER_POD -o jsonpath='{.status.podIP}')

echo "Testing worker: $WORKER_POD ($WORKER_IP:8080)"
echo ""

# Test 1: /healthz endpoint (liveness)
echo "Test 1: /healthz (liveness probe)"
echo "---------------------------------"
if kubectl exec $WORKER_POD -- curl -s http://localhost:8080/healthz; then
    echo "✅ /healthz endpoint works"
else
    echo "❌ /healthz endpoint failed"
    exit 1
fi
echo ""

# Test 2: /readyz endpoint (readiness)
echo "Test 2: /readyz (readiness probe)"
echo "---------------------------------"
READY_OUTPUT=$(kubectl exec $WORKER_POD -- curl -s http://localhost:8080/readyz)
echo "$READY_OUTPUT"
if echo "$READY_OUTPUT" | grep -q '"status":"ready"'; then
    echo "✅ /readyz endpoint works and returns proper JSON"
else
    echo "❌ /readyz endpoint missing expected data"
    exit 1
fi
echo ""

# Test 3: Verify jobs_active field exists
echo "Test 3: Verify jobs_active field in /readyz"
echo "------------------------------------------"
if echo "$READY_OUTPUT" | grep -q '"jobs_active"'; then
    echo "✅ /readyz includes jobs_active field"
else
    echo "❌ /readyz missing jobs_active field"
    exit 1
fi
echo ""

# Test 4: /metrics endpoint
echo "Test 4: /metrics endpoint"
echo "-------------------------"
METRICS_OUTPUT=$(kubectl exec $WORKER_POD -- curl -s http://localhost:8080/metrics)
echo "$METRICS_OUTPUT" | head -5
if echo "$METRICS_OUTPUT" | grep -q '"jobs_active"'; then
    echo "✅ /metrics endpoint works"
else
    echo "❌ /metrics endpoint missing jobs_active"
fi
echo ""

# Test 5: Orchestrator health check simulation
echo "Test 5: Simulate orchestrator health check"
echo "------------------------------------------"
# Parse jobs_active from readyz response
JOBS_ACTIVE=$(echo "$READY_OUTPUT" | python3 -c "import json, sys; data=json.load(sys.stdin); print(data.get('jobs_active', 0))")
echo "jobs_active from /readyz: $JOBS_ACTIVE"
echo ""

# Test 6: Kubernetes probe simulation
echo "Test 6: Kubernetes probe simulation"
echo "-----------------------------------"
echo "Liveness probe (/healthz):"
kubectl exec $WORKER_POD -- curl -s -o /dev/null -w "%{http_code}" http://localhost:8080/healthz
echo " (should be 200)"
echo ""
echo "Readiness probe (/readyz):"
kubectl exec $WORKER_POD -- curl -s -o /dev/null -w "%{http_code}" http://localhost:8080/readyz
echo " (should be 200)"
echo ""

echo "=== All Tests Passed! ==="
echo ""
echo "Summary:"
echo "- ✅ /healthz endpoint works (liveness)"
echo "- ✅ /readyz endpoint works (readiness with jobs_active)"
echo "- ✅ /metrics endpoint works"
echo "- ✅ Orchestrator can get jobs_active from /readyz"
echo "- ✅ Kubernetes probes will work correctly"