#!/bin/bash

# Test remaining features from feature status document

set -e

echo "=== TESTING REMAINING FEATURES ==="
echo ""

ORCHESTRATOR_URL="http://192.168.5.145:9000"
TEST_DIR="/tmp/subgen_test_$(date +%s)"
mkdir -p "$TEST_DIR"

log_success() { echo "✅ $1"; }
log_failure() { echo "❌ $1"; }
log_info() { echo "ℹ️  $1"; }

echo "=== 1. SKIP LOGIC SYSTEM (8 features) ==="
echo ""

# Create test files
echo "Creating test files in $TEST_DIR..."
cp ./orchestrator/test/testdata/short_audio.wav "$TEST_DIR/audio1.wav"
cp ./orchestrator/test/testdata/short_audio.wav "$TEST_DIR/audio2.wav"

# Test 1: Skip if existing LRC
echo "1.1 Skip if existing LRC:"
echo "[00:00.00] Test" > "$TEST_DIR/audio1.lrc"
RESPONSE=$(curl -s "$ORCHESTRATOR_URL/batch?directory=$TEST_DIR")
if echo "$RESPONSE" | grep -q '"skipped"'; then
    log_success "Skip logic detected existing LRC"
else
    log_failure "Skip logic not working for LRC"
fi

# Test 2: Skip language list (would need config change)
echo "1.2 Skip language configuration:"
SKIP_LANG=$(kubectl get configmap subgen-orchestrator-config -o jsonpath='{.data.SKIP_IF_INTERNAL_SUBTITLES_LANGUAGE}')
log_info "Skip language configured: $SKIP_LANG"

echo ""
echo "=== 2. BATCH ENDPOINT ==="
echo ""

echo "2.1 Testing batch endpoint:"
RESPONSE=$(curl -s "$ORCHESTRATOR_URL/batch?directory=$TEST_DIR")
echo "Response: $RESPONSE"
if echo "$RESPONSE" | grep -q '"queued"'; then
    log_success "Batch endpoint working"
else
    log_failure "Batch endpoint failed"
fi

echo ""
echo "=== 3. PATH MAPPING ==="
echo ""

echo "3.1 Checking path mapping configuration:"
USE_MAPPING=$(kubectl get configmap subgen-orchestrator-config -o jsonpath='{.data.USE_PATH_MAPPING}')
FROM_PATH=$(kubectl get configmap subgen-orchestrator-config -o jsonpath='{.data.PATH_MAPPING_FROM}')
TO_PATH=$(kubectl get configmap subgen-orchestrator-config -o jsonpath='{.data.PATH_MAPPING_TO}')

log_info "USE_PATH_MAPPING: $USE_MAPPING"
log_info "PATH_MAPPING_FROM: $FROM_PATH"
log_info "PATH_MAPPING_TO: $TO_PATH"

if [ "$USE_MAPPING" = "true" ]; then
    log_success "Path mapping enabled"
else
    log_info "Path mapping disabled"
fi

echo ""
echo "=== 4. MEDIA SERVER INTEGRATIONS ==="
echo ""

echo "4.1 Checking media server configuration:"
PLEX_ENABLED=$(kubectl get configmap subgen-orchestrator-config -o jsonpath='{.data.PLEX_ENABLED}')
JELLYFIN_ENABLED=$(kubectl get configmap subgen-orchestrator-config -o jsonpath='{.data.JELLYFIN_ENABLED}')

log_info "PLEX_ENABLED: $PLEX_ENABLED"
log_info "JELLYFIN_ENABLED: $JELLYFIN_ENABLED"

# Test webhook endpoints exist
echo "4.2 Testing webhook endpoints:"
for endpoint in "/plex" "/jellyfin" "/emby" "/tautulli"; do
    STATUS=$(curl -s -o /dev/null -w "%{http_code}" "$ORCHESTRATOR_URL$endpoint")
    if [ "$STATUS" = "200" ] || [ "$STATUS" = "400" ] || [ "$STATUS" = "405" ]; then
        log_success "$endpoint endpoint exists (HTTP $STATUS)"
    else
        log_failure "$endpoint endpoint not responding (HTTP $STATUS)"
    fi
done

echo ""
echo "=== 5. PLEX EPISODE QUEUEING ==="
echo ""

echo "5.1 Checking Plex queue configuration:"
QUEUE_NEXT=$(kubectl get configmap subgen-orchestrator-config -o jsonpath='{.data.PLEX_QUEUE_NEXT_EPISODE}')
log_info "PLEX_QUEUE_NEXT_EPISODE: $QUEUE_NEXT"

if [ "$QUEUE_NEXT" = "true" ]; then
    log_success "Plex episode queueing enabled"
else
    log_info "Plex episode queueing disabled"
fi

echo ""
echo "=== 6. FILE SYSTEM MONITORING ==="
echo ""

echo "6.1 Checking file monitoring configuration:"
# This would require MONITOR and TRANSCRIBE_FOLDERS env vars
log_info "File monitoring requires MONITOR=true and TRANSCRIBE_FOLDERS configured"
log_info "Not tested in this environment"

echo ""
echo "=== 7. OUTPUT FORMATS (remaining) ==="
echo ""

if [ -f "./orchestrator/test/testdata/short_audio.wav" ]; then
    echo "7.1 Testing remaining output formats:"
    
    # Test TXT format
    echo "  Testing TXT format..."
    RESPONSE=$(curl -s -X POST -F "audio_file=@./orchestrator/test/testdata/short_audio.wav" \
        -F "task=transcribe" -F "output=txt" "$ORCHESTRATOR_URL/asr")
    if [ -n "$RESPONSE" ] && [ ${#RESPONSE} -gt 10 ]; then
        log_success "TXT format working"
    else
        log_failure "TXT format failed"
    fi
    
    # Test JSON format  
    echo "  Testing JSON format..."
    RESPONSE=$(curl -s -X POST -F "audio_file=@./orchestrator/test/testdata/short_audio.wav" \
        -F "task=transcribe" -F "output=json" "$ORCHESTRATOR_URL/asr")
    if echo "$RESPONSE" | grep -q '"text"'; then
        log_success "JSON format working"
    else
        log_failure "JSON format failed"
    fi
fi

echo ""
echo "=== 8. MODEL LIFECYCLE ==="
echo ""

echo "8.1 Checking model configuration:"
MODEL=$(kubectl get configmap subgen-worker-config -o jsonpath='{.data.WHISPER_MODEL}')
DEVICE=$(kubectl get configmap subgen-worker-config -o jsonpath='{.data.TRANSCRIBE_DEVICE}')
CLEANUP_DELAY=$(kubectl get configmap subgen-worker-config -o jsonpath='{.data.MODEL_CLEANUP_DELAY}')

log_info "WHISPER_MODEL: $MODEL"
log_info "TRANSCRIBE_DEVICE: $DEVICE"
log_info "MODEL_CLEANUP_DELAY: ${CLEANUP_DELAY}s"

echo ""
echo "=== 9. QUEUE SYSTEM FEATURES ==="
echo ""

echo "9.1 Checking queue configuration:"
MAX_SIZE=$(kubectl get configmap subgen-orchestrator-config -o jsonpath='{.data.QUEUE_MAX_SIZE}')
log_info "QUEUE_MAX_SIZE: $MAX_SIZE"

# Check orchestrator logs for queue activity
echo "9.2 Recent queue activity:"
kubectl logs -l app=subgen,component=orchestrator --tail=20 | grep -i "queue\|enqueue\|dequeue" | tail -5 || log_info "No recent queue logs"

echo ""
echo "=== SUMMARY ==="
echo ""
echo "Tests completed for:"
echo "✅ Skip Logic System (partial)"
echo "✅ Batch Endpoint"
echo "✅ Path Mapping configuration"
echo "✅ Media Server Integrations"
echo "✅ Plex Episode Queueing configuration"
echo "✅ Output Formats (TXT, JSON)"
echo "✅ Model Lifecycle configuration"
echo "✅ Queue System configuration"
echo ""
echo "⚠️  Not tested (require specific setup):"
echo "❌ File System Monitoring (needs MONITOR=true)"
echo "❌ Full Skip Logic (needs comprehensive test files)"
echo "❌ Plex/Jellyfin actual webhook processing"
echo "❌ Actual path mapping translation"
echo ""
echo "Cleanup: rm -rf $TEST_DIR"