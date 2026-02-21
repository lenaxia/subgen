#!/bin/bash

# Script to update deployment to v0.2.18 once images are available

set -e

echo "=== Checking v0.2.18 Image Availability ==="
echo ""

# Check orchestrator image
echo "1. Checking orchestrator image..."
if docker pull ghcr.io/lenaxia/subgen-orchestrator:v0.2.18 >/dev/null 2>&1; then
    echo "✅ ghcr.io/lenaxia/subgen-orchestrator:v0.2.18 is available"
    ORCHESTRATOR_AVAILABLE=true
else
    echo "❌ ghcr.io/lenaxia/subgen-orchestrator:v0.2.18 not available yet"
    ORCHESTRATOR_AVAILABLE=false
fi

# Check worker CPU image
echo "2. Checking worker CPU image..."
if docker pull ghcr.io/lenaxia/subgen-worker:v0.2.18-cpu >/dev/null 2>&1; then
    echo "✅ ghcr.io/lenaxia/subgen-worker:v0.2.18-cpu is available"
    WORKER_CPU_AVAILABLE=true
else
    echo "❌ ghcr.io/lenaxia/subgen-worker:v0.2.18-cpu not available yet"
    WORKER_CPU_AVAILABLE=false
fi

echo ""

if [ "$ORCHESTRATOR_AVAILABLE" = true ] && [ "$WORKER_CPU_AVAILABLE" = true ]; then
    echo "=== Updating Deployment to v0.2.18 ==="
    
    # Update deployment file
    echo "1. Updating deploy-working.yaml..."
    sed -i 's|ghcr.io/lenaxia/subgen-orchestrator:v0.2.16|ghcr.io/lenaxia/subgen-orchestrator:v0.2.18|g' deploy-working.yaml
    sed -i 's|ghcr.io/lenaxia/subgen-worker:v0.2.16-cpu|ghcr.io/lenaxia/subgen-worker:v0.2.18-cpu|g' deploy-working.yaml
    
    echo "2. Applying updated deployment..."
    kubectl apply -f deploy-working.yaml
    
    echo "3. Waiting for rollout to complete..."
    kubectl rollout status deployment/subgen-orchestrator --timeout=300s
    kubectl rollout status deployment/subgen-worker --timeout=300s
    
    echo "4. Checking pod status..."
    kubectl get pods -l app=subgen
    
    echo ""
    echo "✅ Deployment updated to v0.2.18"
    echo ""
    echo "=== Next Steps ==="
    echo "1. Test HTTP health endpoints: ./test-health-endpoints.sh"
    echo "2. Test language detection: ./test-language-detection.sh"
    echo "3. Test autoscaling: ./test-autoscaling.sh"
    echo "4. Verify orchestrator discovers workers via HTTP health checks"
    echo ""
    echo "=== v0.2.18 Changes ==="
    echo "- HTTP health checking architecture with /healthz and /readyz endpoints"
    echo "- Orchestrator uses HTTP /readyz to get jobs_active (not gRPC)"
    echo "- Fixed Docker build issue with EXPOSE port comments"
    
else
    echo "❌ Not all images are available yet."
    echo "Please wait for the GitHub Actions release workflow to complete."
    echo ""
    echo "Check release status: https://github.com/lenaxia/subgen/actions"
    exit 1
fi