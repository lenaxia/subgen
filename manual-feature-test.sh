#!/bin/bash

# Manual Feature Testing for Subgen v0.2.18
# Testing remaining features against k8s pods

set -e

echo "=== MANUAL FEATURE TESTING v0.2.18 ==="
echo "Date: $(date)"
echo ""

ORCHESTRATOR_URL="http://192.168.5.145:9000"
TEST_DATA="./test/testdata"
ORCH_TEST_DATA="./orchestrator/test/testdata"

# Test data files
SHORT_AUDIO="$ORCH_TEST_DATA/short_audio.wav"
SPEECH_SAMPLE="$TEST_DATA/speech_sample.wav"
DEMO_VIDEO="$TEST_DATA/demo_video_speech.mp4"
MULTI_AUDIO="$TEST_DATA/multi_audio_test/multi_audio_test.mkv"

echo "=== 1. CORE TRANSCRIPTION (Remaining Features) ==="
echo ""

# 1.1 Test multiple audio track handling
echo "1.1 Multiple audio track handling:"
if [ -f "$MULTI_AUDIO" ]; then
    echo "  Testing with multi-audio MKV file..."
    RESPONSE=$(curl -s -X POST -F "audio_file=@$MULTI_AUDIO" -F "task=transcribe" -F "output=srt" "$ORCHESTRATOR_URL/asr")
    if echo "$RESPONSE" | grep -q "^[0-9]\+$"; then
        echo "  ✅ Multi-audio file processed"
    else
        echo "  ❌ Multi-audio file failed"
    fi
else
    echo "  ⚠️  Multi-audio test file not found: $MULTI_AUDIO"
fi

# 1.2 Test force language override
echo ""
echo "1.2 Force language override:"
echo "  Testing with Spanish language force..."
RESPONSE=$(curl -s -X POST -F "audio_file=@$SHORT_AUDIO" -F "task=transcribe" -F "language=es" -F "output=srt" "$ORCHESTRATOR_URL/asr")
if echo "$RESPONSE" | grep -q "^[0-9]\+$"; then
    echo "  ✅ Language override working (forced Spanish)"
else
    echo "  ❌ Language override failed"
fi

# 1.3 Test translate task
echo ""
echo "1.3 Translate task type:"
echo "  Testing translation task..."
RESPONSE=$(curl -s -X POST -F "audio_file=@$SHORT_AUDIO" -F "task=translate" -F "output=srt" "$ORCHESTRATOR_URL/asr")
if echo "$RESPONSE" | grep -q "^[0-9]\+$"; then
    echo "  ✅ Translation task working"
else
    echo "  ❌ Translation task failed"
fi

echo ""
echo "=== 2. OUTPUT FORMATS (Remaining) ==="
echo ""

# 2.1 Test TXT format
echo "2.1 TXT format:"
RESPONSE=$(curl -s -X POST -F "audio_file=@$SHORT_AUDIO" -F "task=transcribe" -F "output=txt" "$ORCHESTRATOR_URL/asr")
if [ -n "$RESPONSE" ] && [ ${#RESPONSE} -gt 10 ]; then
    echo "  ✅ TXT format working"
    echo "  Sample: $(echo "$RESPONSE" | head -c 50)..."
else
    echo "  ❌ TXT format failed"
    echo "  Response: $RESPONSE"
fi

# 2.2 Test TSV format
echo ""
echo "2.2 TSV format:"
RESPONSE=$(curl -s -X POST -F "audio_file=@$SHORT_AUDIO" -F "task=transcribe" -F "output=tsv" "$ORCHESTRATOR_URL/asr")
if echo "$RESPONSE" | grep -q $'\t'; then
    echo "  ✅ TSV format working"
    echo "  First line: $(echo "$RESPONSE" | head -1 | cut -c1-50)..."
else
    echo "  ❌ TSV format failed"
fi

# 2.3 Test JSON format (comprehensive)
echo ""
echo "2.3 JSON format (comprehensive):"
RESPONSE=$(curl -s -X POST -F "audio_file=@$SHORT_AUDIO" -F "task=transcribe" -F "output=json" "$ORCHESTRATOR_URL/asr")
if echo "$RESPONSE" | python3 -c "import json, sys; data=json.load(sys.stdin); print('  ✅ JSON format working'); print('  Text length:', len(data.get('text', ''))); print('  Segments:', len(data.get('segments', [])))" 2>/dev/null; then
    echo "  JSON structure valid"
else
    echo "  ❌ JSON format failed"
fi

echo ""
echo "=== 3. SKIP LOGIC SYSTEM ==="
echo ""

# Create test directory for skip logic
SKIP_TEST_DIR="/tmp/subgen_skip_test_$(date +%s)"
mkdir -p "$SKIP_TEST_DIR"

echo "3.1 Skip if existing LRC:"
cp "$SHORT_AUDIO" "$SKIP_TEST_DIR/test1.wav"
echo "[00:00.00] Test" > "$SKIP_TEST_DIR/test1.lrc"

echo "3.2 Skip if existing SRT:"
cp "$SHORT_AUDIO" "$SKIP_TEST_DIR/test2.wav"
echo "1" > "$SKIP_TEST_DIR/test2.srt"
echo "00:00:00,000 --> 00:00:02,000" >> "$SKIP_TEST_DIR/test2.srt"
echo "Test" >> "$SKIP_TEST_DIR/test2.srt"

echo "3.3 File without subtitles (should process):"
cp "$SHORT_AUDIO" "$SKIP_TEST_DIR/test3.wav"

# Test with batch endpoint (POST request)
echo ""
echo "Testing batch endpoint with skip logic..."
BATCH_RESPONSE=$(curl -s -X POST "$ORCHESTRATOR_URL/batch?directory=$SKIP_TEST_DIR")
echo "Batch response: $BATCH_RESPONSE"

# Check orchestrator logs for skip logic
echo ""
echo "Checking orchestrator logs for skip logic..."
kubectl logs -l app=subgen,component=orchestrator --tail=50 | grep -i "skip\|skipped" | tail -10

echo ""
echo "=== 4. MEDIA SERVER INTEGRATIONS ==="
echo ""

# 4.1 Test Plex webhook endpoint
echo "4.1 Plex webhook endpoint:"
PLEX_PAYLOAD='{"event": "media.scrobble", "Metadata": {"librarySectionType": "movie", "ratingKey": "12345"}}'
PLEX_RESPONSE=$(curl -s -X POST "$ORCHESTRATOR_URL/plex" \
    -H "Content-Type: application/json" \
    -d "$PLEX_PAYLOAD")
echo "  Plex response: $PLEX_RESPONSE"

# 4.2 Test Jellyfin webhook endpoint
echo ""
echo "4.2 Jellyfin webhook endpoint:"
JELLYFIN_PAYLOAD='{"NotificationType": "ItemAdded", "ItemId": "12345"}'
JELLYFIN_RESPONSE=$(curl -s -X POST "$ORCHESTRATOR_URL/jellyfin" \
    -H "Content-Type: application/json" \
    -d "$JELLYFIN_PAYLOAD")
echo "  Jellyfin response: $JELLYFIN_RESPONSE"

# 4.3 Test Emby webhook endpoint
echo ""
echo "4.3 Emby webhook endpoint:"
EMBY_PAYLOAD='{"Event": "library.new", "Item": {"Id": "12345"}}'
EMBY_RESPONSE=$(curl -s -X POST "$ORCHESTRATOR_URL/emby" \
    -H "Content-Type: application/json" \
    -d "$EMBY_PAYLOAD")
echo "  Emby response: $EMBY_RESPONSE"

# 4.4 Test Tautulli webhook endpoint
echo ""
echo "4.4 Tautulli webhook endpoint:"
TAUTULLI_PAYLOAD='{"event": "played", "rating_key": "12345"}'
TAUTULLI_RESPONSE=$(curl -s -X POST "$ORCHESTRATOR_URL/tautulli" \
    -H "Content-Type: application/json" \
    -d "$TAUTULLI_PAYLOAD")
echo "  Tautulli response: $TAUTULLI_RESPONSE"

echo ""
echo "=== 5. PATH MAPPING ==="
echo ""

# Check current path mapping configuration
echo "5.1 Path mapping configuration:"
USE_MAPPING=$(kubectl get configmap subgen-orchestrator-config -o jsonpath='{.data.USE_PATH_MAPPING}')
FROM_PATH=$(kubectl get configmap subgen-orchestrator-config -o jsonpath='{.data.PATH_MAPPING_FROM}')
TO_PATH=$(kubectl get configmap subgen-orchestrator-config -o jsonpath='{.data.PATH_MAPPING_TO}')

echo "  USE_PATH_MAPPING: $USE_MAPPING"
echo "  PATH_MAPPING_FROM: $FROM_PATH"
echo "  PATH_MAPPING_TO: $TO_PATH"

# Test with a path that should be mapped
echo ""
echo "5.2 Testing path mapping with ASR endpoint..."
# This would require actual file at the mapped location
echo "  ⚠️  Requires actual file at mapped location: $TO_PATH"

echo ""
echo "=== 6. MODEL LIFECYCLE ==="
echo ""

# Check worker model status
echo "6.1 Worker model status:"
WORKER_POD=$(kubectl get pods -l app=subgen,component=worker -o jsonpath='{.items[0].metadata.name}')
MODEL_STATUS=$(kubectl exec $WORKER_POD -- curl -s http://localhost:8080/readyz | grep -o '"model_loaded":[a-z]*' | cut -d: -f2)
echo "  Model loaded: $MODEL_STATUS"

# Check model configuration
echo ""
echo "6.2 Model configuration:"
WHISPER_MODEL=$(kubectl get configmap subgen-worker-config -o jsonpath='{.data.WHISPER_MODEL}')
COMPUTE_TYPE=$(kubectl get configmap subgen-worker-config -o jsonpath='{.data.COMPUTE_TYPE}')
CLEANUP_DELAY=$(kubectl get configmap subgen-worker-config -o jsonpath='{.data.MODEL_CLEANUP_DELAY}')

echo "  WHISPER_MODEL: $WHISPER_MODEL"
echo "  COMPUTE_TYPE: $COMPUTE_TYPE"
echo "  MODEL_CLEANUP_DELAY: ${CLEANUP_DELAY}s"

echo ""
echo "=== 7. QUEUE SYSTEM ==="
echo ""

# Check queue metrics
echo "7.1 Queue metrics from orchestrator:"
curl -s "http://192.168.5.145:9090/metrics" | grep -E "(queue|task)" | head -10

# Check recent queue activity
echo ""
echo "7.2 Recent queue activity:"
kubectl logs -l app=subgen,component=orchestrator --tail=20 | grep -i "queue\|enqueue\|dequeue\|task" | tail -10

echo ""
echo "=== 8. FILE SYSTEM MONITORING ==="
echo ""

# Check if monitoring is enabled
echo "8.1 File monitoring configuration:"
# This would require MONITOR environment variable
echo "  ⚠️  File monitoring requires MONITOR=true and TRANSCRIBE_FOLDERS"
echo "  Current config not set for file monitoring"

echo ""
echo "=== 9. BATCH ENDPOINT ==="
echo ""

# Test batch endpoint with POST
echo "9.1 Batch endpoint test:"
echo "  Testing with directory: $SKIP_TEST_DIR"
BATCH_RESPONSE=$(curl -s -X POST "$ORCHESTRATOR_URL/batch?directory=$SKIP_TEST_DIR")
echo "  Response: $BATCH_RESPONSE"

# Check if scanner is initialized
echo ""
echo "9.2 Scanner initialization:"
# Check orchestrator logs for scanner status
kubectl logs -l app=subgen,component=orchestrator --tail=20 | grep -i "scanner" | tail -5

echo ""
echo "=== 10. PLEX EPISODE QUEUEING ==="
echo ""

# Check configuration
echo "10.1 Plex episode queueing configuration:"
QUEUE_NEXT=$(kubectl get configmap subgen-orchestrator-config -o jsonpath='{.data.PLEX_QUEUE_NEXT_EPISODE}')
QUEUE_SEASON=$(kubectl get configmap subgen-orchestrator-config -o jsonpath='{.data.PLEX_QUEUE_SEASON}')
QUEUE_SERIES=$(kubectl get configmap subgen-orchestrator-config -o jsonpath='{.data.PLEX_QUEUE_SERIES}')

echo "  PLEX_QUEUE_NEXT_EPISODE: $QUEUE_NEXT"
echo "  PLEX_QUEUE_SEASON: $QUEUE_SEASON"
echo "  PLEX_QUEUE_SERIES: $QUEUE_SERIES"

echo ""
echo "=== SUMMARY ==="
echo ""
echo "Tests completed for remaining features."
echo "Cleanup test directory: rm -rf $SKIP_TEST_DIR"
echo ""
echo "Key findings will be documented in worklog."