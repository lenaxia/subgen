#!/bin/bash

# Load test script for Subgen autoscaling
# Generates concurrent language detection requests to trigger HPA scaling

set -e

ORCHESTRATOR_IP="192.168.5.145"
ORCHESTRATOR_PORT="9000"
URL="http://${ORCHESTRATOR_IP}:${ORCHESTRATOR_PORT}"
AUDIO_FILE="test/testdata/speech_sample.wav"
CONCURRENT_REQUESTS=5
TOTAL_REQUESTS=20

echo "=== Subgen Load Test ==="
echo "Target: ${URL}"
echo "Audio file: ${AUDIO_FILE}"
echo "Concurrent requests: ${CONCURRENT_REQUESTS}"
echo "Total requests: ${TOTAL_REQUESTS}"
echo ""

# Check if audio file exists
if [ ! -f "$AUDIO_FILE" ]; then
    echo "❌ Audio file not found: $AUDIO_FILE"
    exit 1
fi

echo "=== Starting Load Test ==="
echo "This will generate load to trigger autoscaling (CPU > 70%)"
echo ""

# Get initial HPA status
echo "=== Initial HPA Status ==="
kubectl get hpa subgen-worker
echo ""

# Get initial pod count
INITIAL_PODS=$(kubectl get pods -l app=subgen,component=worker --no-headers | wc -l)
echo "Initial worker pods: ${INITIAL_PODS}"
echo ""

# Function to make a single request
make_request() {
    local req_num=$1
    echo "Request $req_num started at: $(date +%H:%M:%S)"
    
    START_TIME=$(date +%s)
    RESPONSE=$(curl -s -X POST \
        -F "audio_file=@${AUDIO_FILE}" \
        "${URL}/detect-language?offset=0&length=5" \
        --max-time 120 2>/dev/null)
    
    END_TIME=$(date +%s)
    DURATION=$((END_TIME - START_TIME))
    
    if echo "$RESPONSE" | grep -q "language"; then
        echo "✅ Request $req_num completed in ${DURATION}s"
    else
        echo "❌ Request $req_num failed or timed out after ${DURATION}s"
    fi
}

# Export function for parallel execution
export -f make_request
export AUDIO_FILE URL

# Run concurrent requests
echo "=== Generating Load ==="
seq 1 $TOTAL_REQUESTS | xargs -I {} -P $CONCURRENT_REQUESTS bash -c 'make_request "$@"' _ {}

echo ""
echo "=== Load Test Complete ==="
echo ""

# Wait for metrics to update
echo "Waiting 30 seconds for metrics to update..."
sleep 30

# Check final HPA status
echo "=== Final HPA Status ==="
kubectl describe hpa subgen-worker | grep -A10 "Metrics:"

# Check final pod count
FINAL_PODS=$(kubectl get pods -l app=subgen,component=worker --no-headers | wc -l)
echo ""
echo "=== Pod Scaling Summary ==="
echo "Initial pods: ${INITIAL_PODS}"
echo "Final pods: ${FINAL_PODS}"

if [ $FINAL_PODS -gt $INITIAL_PODS ]; then
    echo "✅ Autoscaling triggered! Pods scaled from ${INITIAL_PODS} to ${FINAL_PODS}"
else
    echo "⚠️  Autoscaling not triggered. Pods remain at ${FINAL_PODS}"
    echo "Check HPA metrics and thresholds."
fi

echo ""
echo "=== Resource Usage ==="
kubectl top pods -l app=subgen