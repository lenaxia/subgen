#!/bin/bash

# Script to update deployment to v0.2.16 once images are available

set -e

echo "=== Checking v0.2.16 Image Availability ==="
echo ""

# Check orchestrator image
echo "1. Checking orchestrator image..."
if docker pull ghcr.io/lenaxia/subgen-orchestrator:v0.2.16 >/dev/null 2>&1; then
    echo "✅ ghcr.io/lenaxia/subgen-orchestrator:v0.2.16 is available"
    ORCHESTRATOR_AVAILABLE=true
else
    echo "❌ ghcr.io/lenaxia/subgen-orchestrator:v0.2.16 not available yet"
    ORCHESTRATOR_AVAILABLE=false
fi

# Check worker CPU image
echo "2. Checking worker CPU image..."
if docker pull ghcr.io/lenaxia/subgen-worker:v0.2.16-cpu >/dev/null 2>&1; then
    echo "✅ ghcr.io/lenaxia/subgen-worker:v0.2.16-cpu is available"
    WORKER_CPU_AVAILABLE=true
else
    echo "❌ ghcr.io/lenaxia/subgen-worker:v0.2.16-cpu not available yet"
    WORKER_CPU_AVAILABLE=false
fi

echo ""

if [ "$ORCHESTRATOR_AVAILABLE" = true ] && [ "$WORKER_CPU_AVAILABLE" = true ]; then
    echo "=== Updating Deployment to v0.2.16 ==="
    
    # Update deployment file
    echo "1. Updating deploy-working.yaml..."
    sed -i 's|ghcr.io/lenaxia/subgen-orchestrator:v0.2.15|ghcr.io/lenaxia/subgen-orchestrator:v0.2.16|g' deploy-working.yaml
    sed -i 's|ghcr.io/lenaxia/subgen-worker:v0.2.15-cpu|ghcr.io/lenaxia/subgen-worker:v0.2.16-cpu|g' deploy-working.yaml
    
    echo "2. Applying updated deployment..."
    kubectl apply -f deploy-working.yaml
    
    echo "3. Waiting for rollout to complete..."
    kubectl rollout status deployment/subgen-orchestrator --timeout=300s
    kubectl rollout status deployment/subgen-worker --timeout=300s
    
    echo "4. Checking pod status..."
    kubectl get pods -l app=subgen
    
    echo ""
    echo "✅ Deployment updated to v0.2.16"
    echo ""
    echo "=== Next Steps ==="
    echo "1. Test language detection: ./test-language-detection.sh"
    echo "2. Test autoscaling: ./test-autoscaling.sh"
    echo "3. Validate features from feature status document"
    
else
    echo "❌ Not all images are available yet."
    echo "Please wait for the GitHub Actions release workflow to complete."
    echo ""
    echo "Check release status: https://github.com/lenaxia/subgen/actions"
    exit 1
fi