#!/bin/bash

# Test language detection with v0.2.16 timeout fix
# This script tests the /detect-language endpoint with a sample audio file

set -e

ORCHESTRATOR_IP="192.168.5.145"
ORCHESTRATOR_PORT="9000"
URL="http://${ORCHESTRATOR_IP}:${ORCHESTRATOR_PORT}"

echo "=== Testing Language Detection with v0.2.16 ==="
echo "Orchestrator URL: ${URL}"
echo ""

# Check if we have a test audio file
TEST_AUDIO="test/testdata/sample-english.mp3"
if [ ! -f "$TEST_AUDIO" ]; then
    echo "❌ Test audio file not found: $TEST_AUDIO"
    echo "Creating a simple test audio file..."
    
    # Create a simple test directory if it doesn't exist
    mkdir -p test/testdata
    
    # Try to find any audio file in the test directory
    TEST_AUDIO=$(find test/testdata -name "*.mp3" -o -name "*.wav" -o -name "*.m4a" | head -1)
    
    if [ -z "$TEST_AUDIO" ]; then
        echo "No audio files found in test/testdata/"
        echo "Please add an audio file to test/testdata/ or specify one with:"
        echo "  ./test-language-detection.sh /path/to/audio/file.mp3"
        exit 1
    fi
    echo "Using found audio file: $TEST_AUDIO"
fi

# Use provided audio file if specified
if [ $# -eq 1 ]; then
    TEST_AUDIO="$1"
    if [ ! -f "$TEST_AUDIO" ]; then
        echo "❌ Audio file not found: $TEST_AUDIO"
        exit 1
    fi
fi

echo "Testing with audio file: $TEST_AUDIO"
echo "File size: $(ls -lh "$TEST_AUDIO" | awk '{print $5}')"
echo ""

# Test 1: Basic health check
echo "=== Test 1: Health Check ==="
if curl -s "${URL}/health" | grep -q "ok"; then
    echo "✅ Health check passed"
else
    echo "❌ Health check failed"
    exit 1
fi
echo ""

# Test 2: Detect language with default parameters (30 seconds)
echo "=== Test 2: Language Detection (30s timeout) ==="
echo "This test should complete within 30 seconds with v0.2.15"
echo "With v0.2.16, it should have 120s timeout for model loading"
echo ""

START_TIME=$(date +%s)
RESPONSE=$(curl -s -X POST \
    -F "audio_file=@${TEST_AUDIO}" \
    "${URL}/detect-language?offset=0&length=30" \
    --max-time 35)  # Allow 35 seconds for the request

END_TIME=$(date +%s)
DURATION=$((END_TIME - START_TIME))

echo "Request duration: ${DURATION} seconds"
echo "Response:"
echo "$RESPONSE" | jq . 2>/dev/null || echo "$RESPONSE"

if echo "$RESPONSE" | grep -q "language"; then
    echo "✅ Language detection successful in ${DURATION}s"
elif echo "$RESPONSE" | grep -q "timeout\|Timeout"; then
    echo "⚠️ Language detection timed out after ${DURATION}s"
    echo "This is expected with v0.2.15, should be fixed in v0.2.16"
else
    echo "❌ Language detection failed"
fi
echo ""

# Test 3: Detect language with shorter segment (10 seconds)
echo "=== Test 3: Language Detection (10s segment) ==="
START_TIME=$(date +%s)
RESPONSE=$(curl -s -X POST \
    -F "audio_file=@${TEST_AUDIO}" \
    "${URL}/detect-language?offset=0&length=10" \
    --max-time 35)

END_TIME=$(date +%s)
DURATION=$((END_TIME - START_TIME))

echo "Request duration: ${DURATION} seconds"
echo "Response:"
echo "$RESPONSE" | jq . 2>/dev/null || echo "$RESPONSE"

if echo "$RESPONSE" | grep -q "language"; then
    echo "✅ Language detection successful in ${DURATION}s"
else
    echo "❌ Language detection failed"
fi
echo ""

# Test 4: Check worker status
echo "=== Test 4: Worker Status ==="
if curl -s "${URL}/workers" | grep -q "address"; then
    echo "✅ Workers are registered"
    curl -s "${URL}/workers" | jq .
else
    echo "❌ No workers found"
fi
echo ""

echo "=== Test Complete ==="
echo "Summary:"
echo "- v0.2.15: Language detection may timeout at 30s"
echo "- v0.2.16: Should have 120s timeout for model loading"
echo ""
echo "After deploying v0.2.16, language detection should work correctly."