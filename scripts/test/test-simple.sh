#!/bin/bash

# Simple test for Subgen v0.2.16

set -e

ORCHESTRATOR_IP="192.168.5.145"
ORCHESTRATOR_PORT="9000"
URL="http://${ORCHESTRATOR_IP}:${ORCHESTRATOR_PORT}"
AUDIO_FILE="test/testdata/speech_sample.wav"

echo "=== Simple Subgen Test ==="
echo "Target: ${URL}"
echo ""

# Test 1: Health check
echo "1. Health check:"
curl -s "${URL}/health" | jq .
echo ""

# Test 2: Single language detection
echo "2. Language detection (120s timeout):"
time curl -s -X POST \
  -F "audio_file=@${AUDIO_FILE}" \
  "${URL}/detect-language?offset=0&length=5" \
  --max-time 120 | jq .
echo ""

# Test 3: Check HPA status
echo "3. HPA Status:"
kubectl get hpa subgen-worker
echo ""

# Test 4: Check pods
echo "4. Worker Pods:"
kubectl get pods -l app=subgen,component=worker
echo ""

# Test 5: Resource usage
echo "5. Resource Usage:"
kubectl top pods -l app=subgen 2>/dev/null || echo "Metrics not available yet"
echo ""

echo "=== Test Complete ==="