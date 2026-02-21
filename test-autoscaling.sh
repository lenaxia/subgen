#!/bin/bash
# Test script for Subgen worker autoscaling

set -e

echo "=== Subgen Worker Autoscaling Test ==="
echo

# Check if metrics server is installed
echo "1. Checking metrics server..."
if kubectl top nodes &>/dev/null; then
    echo "   ✓ Metrics server is installed"
else
    echo "   ✗ Metrics server not found. Installing..."
    kubectl apply -f https://github.com/kubernetes-sigs/metrics-server/releases/latest/download/components.yaml
    sleep 10
fi

# First, migrate from StatefulSet to Deployment
echo "2. Migrating from StatefulSet to Deployment..."
echo "   Current StatefulSet will be deleted and replaced with Deployment"
read -p "   Continue? (y/n): " -n 1 -r
echo
if [[ ! $REPLY =~ ^[Yy]$ ]]; then
    echo "   Skipping migration"
else
    echo "   Applying updated deployment..."
    kubectl apply -f deploy-working.yaml
    echo "   Waiting for Deployment rollout..."
    kubectl rollout status deployment/subgen-worker -n default --timeout=120s
fi

# Deploy HPA
echo "3. Deploying HorizontalPodAutoscaler..."
kubectl apply -f hpa-worker.yaml

# Wait for HPA to be ready
echo "4. Waiting for HPA to initialize..."
sleep 5

# Show current state
echo "5. Current state:"
echo "   HPA status:"
kubectl get hpa subgen-worker -n default
echo
echo "   Worker pods:"
kubectl get pods -n default -l app=subgen,component=worker
echo
echo "   Resource usage:"
kubectl top pods -n default -l app=subgen,component=worker 2>/dev/null || echo "   (Metrics not available yet)"
echo

# Create load test (optional)
echo "6. Load test options:"
echo "   a) Run batch transcription to generate CPU load"
echo "   b) Monitor scaling with: watch kubectl get hpa subgen-worker"
echo "   c) Monitor pods with: watch kubectl get pods -l app=subgen,component=worker"
echo
echo "To generate load, you can run:"
echo "  curl -X POST 'http://localhost:9000/batch?directory=/media&recursive=true&format=json'"
echo
echo "=== Test Complete ==="
echo
echo "Notes:"
echo "- HPA will scale based on CPU (>70%) and Memory (>80%) utilization"
echo "- Scale up: max 2 pods at once, 1 minute cooldown"
echo "- Scale down: max 1 pod at once, 5 minute cooldown"
echo "- Min replicas: 1, Max replicas: 10"
echo
echo "To clean up:"
echo "  kubectl delete -f hpa-worker.yaml"