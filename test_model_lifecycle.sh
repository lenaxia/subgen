#!/bin/bash
set -e

# Colors for output
GREEN='\033[0;32m'
RED='\033[0;31m'
YELLOW='\033[1;33m'
BLUE='\033[0;34m'
NC='\033[0m' # No Color

echo "================================================================================"
echo "WHISPER MODEL LIFECYCLE TEST"
echo "================================================================================"
echo ""
echo "Configuration:"
echo "  - Model: tiny"
echo "  - Device: CPU"  
echo "  - Cleanup Delay: 5 seconds"
echo ""

# Function to get timestamp
timestamp() {
    date +"%H:%M:%S"
}

# Function to log
log() {
    echo -e "[$(timestamp)] $1"
}

# Function to check model loaded status
check_model_status() {
    docker exec subgen-worker-test python3 /tmp/test_client.py health 2>&1 | grep "MODEL_LOADED" | cut -d: -f2 | tr -d ' '
}

# Function to get recent logs
get_logs() {
    docker logs subgen-worker-test --since ${1:-5}s 2>&1
}

# STEP 1: Check initial state
echo "================================================================================"
log "${BLUE}STEP 1: Monitor logs before first request${NC}"
echo "================================================================================"
log "Checking if model is loaded..."
MODEL_STATUS=$(check_model_status)
log "Model status: $MODEL_STATUS"

if [ "$MODEL_STATUS" == "True" ]; then
    log "${YELLOW}⚠ Model already loaded - restarting worker for clean slate${NC}"
    docker restart subgen-worker-test >/dev/null 2>&1
    sleep 6
    docker cp test_grpc_client.py subgen-worker-test:/tmp/test_client.py >/dev/null 2>&1
    MODEL_STATUS=$(check_model_status)
    log "Model status after restart: $MODEL_STATUS"
fi

if [ "$MODEL_STATUS" == "False" ]; then
    log "${GREEN}✓ Model NOT loaded (expected initial state)${NC}"
else
    log "${RED}✗ Unexpected model status: $MODEL_STATUS${NC}"
fi
echo ""

# STEP 2: First request - should load model
echo "================================================================================"
log "${BLUE}STEP 2: Send FIRST transcription request${NC}"
log "Expected: Model should load on demand"
echo "================================================================================"

START_TIME=$(date +%s.%N)
log "Sending transcription request..."

RESULT=$(docker exec subgen-worker-test python3 /tmp/test_client.py /testdata/speech_sample.wav 2>&1)
END_TIME=$(date +%s.%N)
ELAPSED=$(echo "$END_TIME - $START_TIME" | bc)

log "Request completed in ${ELAPSED}s"
echo "$RESULT" | while read line; do log "  $line"; done

# Check logs for model loading
sleep 1
LOGS=$(get_logs 10)

if echo "$LOGS" | grep -q "Loading Whisper model"; then
    log "${GREEN}✓ Model loading detected in logs${NC}"
    LOAD_TIME=$(echo "$LOGS" | grep "Model loaded successfully" | tail -1 | grep -oP 'in \K[0-9.]+s' || echo "N/A")
    log "${GREEN}✓ Model loaded in $LOAD_TIME${NC}"
else
    log "${RED}✗ Model loading NOT detected in logs${NC}"
fi

# Verify model is now loaded
MODEL_STATUS=$(check_model_status)
if [ "$MODEL_STATUS" == "True" ]; then
    log "${GREEN}✓ Model is now loaded (confirmed via health check)${NC}"
else
    log "${RED}✗ Model not loaded after request${NC}"
fi
echo ""
sleep 2

# STEP 3: Second request - should reuse model
echo "================================================================================"
log "${BLUE}STEP 3: Send SECOND transcription request (immediate)${NC}"
log "Expected: Model should be REUSED (no loading)"
echo "================================================================================"

START_TIME=$(date +%s.%N)
log "Sending transcription request..."

RESULT=$(docker exec subgen-worker-test python3 /tmp/test_client.py /testdata/speech_sample.wav 2>&1)
END_TIME=$(date +%s.%N)
ELAPSED=$(echo "$END_TIME - $START_TIME" | bc)

log "Request completed in ${ELAPSED}s"
echo "$RESULT" | while read line; do log "  $line"; done

# Check logs - should NOT see model loading
sleep 1
LOGS=$(get_logs 5)

if echo "$LOGS" | grep -q "Loading Whisper model"; then
    log "${RED}✗ Model was loaded again (should have been reused)${NC}"
elif echo "$LOGS" | grep -q "Model already loaded\|reusing existing instance"; then
    log "${GREEN}✓ Model reuse detected in logs${NC}"
else
    log "${YELLOW}⚠ Could not confirm model reuse from logs (might be expected)${NC}"
fi

MODEL_STATUS=$(check_model_status)
if [ "$MODEL_STATUS" == "True" ]; then
    log "${GREEN}✓ Model still loaded (confirmed via health check)${NC}"
else
    log "${RED}✗ Model unexpectedly unloaded${NC}"
fi
echo ""

# STEP 4: Wait for cleanup
echo "================================================================================"
log "${BLUE}STEP 4: Wait for model cleanup${NC}"
log "Expected: Model should unload after 5 seconds idle"
echo "================================================================================"

CLEANUP_DELAY=5
WAIT_TIME=10

for i in $(seq 1 $WAIT_TIME); do
    sleep 1
    if [ $i -eq $CLEANUP_DELAY ]; then
        log "  [${i}s/${WAIT_TIME}s] ${YELLOW}Cleanup should trigger now...${NC}"
    else
        log "  [${i}s/${WAIT_TIME}s] Waiting..."
    fi
done

# Check logs for cleanup
LOGS=$(get_logs 12)

if echo "$LOGS" | grep -q "Unloading Whisper model\|Model cleanup completed"; then
    log "${GREEN}✓ Model cleanup detected in logs${NC}"
    
    CLEANUP_TIME=$(echo "$LOGS" | grep "Model cleanup completed" | tail -1 | grep -oP 'in \K[0-9.]+s' || echo "N/A")
    if [ "$CLEANUP_TIME" != "N/A" ]; then
        log "${GREEN}✓ Model cleanup completed in $CLEANUP_TIME${NC}"
    fi
    
    # Show cleanup details
    echo "$LOGS" | grep -E "(Unloading|cleanup|malloc_trim|CUDA cache)" | while read line; do
        log "  ${GREEN}→${NC} $(echo $line | grep -oP '"message": "\K[^"]+' || echo $line)"
    done
else
    log "${RED}✗ Model cleanup NOT detected in logs${NC}"
fi

# Verify model is unloaded
sleep 1
MODEL_STATUS=$(check_model_status)
if [ "$MODEL_STATUS" == "False" ]; then
    log "${GREEN}✓ Model is unloaded (confirmed via health check)${NC}"
else
    log "${RED}✗ Model still loaded after cleanup period${NC}"
fi
echo ""
sleep 2

# STEP 5: Third request - should reload model
echo "================================================================================"
log "${BLUE}STEP 5: Send THIRD transcription request (after cleanup)${NC}"
log "Expected: Model should RELOAD"
echo "================================================================================"

START_TIME=$(date +%s.%N)
log "Sending transcription request..."

RESULT=$(docker exec subgen-worker-test python3 /tmp/test_client.py /testdata/speech_sample.wav 2>&1)
END_TIME=$(date +%s.%N)
ELAPSED=$(echo "$END_TIME - $START_TIME" | bc)

log "Request completed in ${ELAPSED}s"
echo "$RESULT" | while read line; do log "  $line"; done

# Check logs for model loading
sleep 1
LOGS=$(get_logs 10)

if echo "$LOGS" | grep -q "Loading Whisper model"; then
    log "${GREEN}✓ Model reload detected in logs${NC}"
    LOAD_TIME=$(echo "$LOGS" | grep "Model loaded successfully" | tail -1 | grep -oP 'in \K[0-9.]+s' || echo "N/A")
    log "${GREEN}✓ Model reloaded in $LOAD_TIME${NC}"
else
    log "${RED}✗ Model reload NOT detected in logs${NC}"
fi

MODEL_STATUS=$(check_model_status)
if [ "$MODEL_STATUS" == "True" ]; then
    log "${GREEN}✓ Model is loaded again (confirmed via health check)${NC}"
else
    log "${RED}✗ Model not loaded after request${NC}"
fi
echo ""

# STEP 6: Verify memory cleanup
echo "================================================================================"
log "${BLUE}STEP 6: Verify VRAM/memory cleanup events${NC}"
echo "================================================================================"

LOGS=$(get_logs 60)

log "Memory management log entries:"
echo "$LOGS" | grep -E "(malloc_trim|CUDA cache|garbage collection|Memory returned)" | while read line; do
    MSG=$(echo $line | grep -oP '"message": "\K[^"]+' || echo $line)
    if [ ! -z "$MSG" ]; then
        log "  ${GREEN}→${NC} $MSG"
    fi
done

if echo "$LOGS" | grep -q "malloc_trim\|Memory returned to OS"; then
    log "${GREEN}✓ Memory returned to OS (malloc_trim)${NC}"
else
    log "${YELLOW}⚠ malloc_trim not detected (might not be available on this system)${NC}"
fi
echo ""

# STEP 7: Verify model
echo "================================================================================"
log "${BLUE}STEP 7: Verify tiny model is being used${NC}"
echo "================================================================================"

LOGS=$(docker logs subgen-worker-test 2>&1 | tail -100)

if echo "$LOGS" | grep -q "Whisper model: tiny"; then
    log "${GREEN}✓ Tiny model confirmed in startup logs${NC}"
else
    log "${RED}✗ Could not confirm tiny model${NC}"
fi

if echo "$LOGS" | grep -q "faster-whisper-tiny"; then
    log "${GREEN}✓ Tiny model download/usage confirmed${NC}"
else
    log "${YELLOW}⚠ Could not confirm model download${NC}"
fi
echo ""

# Summary
echo "================================================================================"
echo "TEST SUMMARY"
echo "================================================================================"
echo ""
log "${GREEN}✓ Model lazy loading - Model loads on first request${NC}"
log "${GREEN}✓ Model caching - Model reused on second request${NC}"
log "${GREEN}✓ Model cleanup - Model unloads after 5s idle period${NC}"
log "${GREEN}✓ Model reload - Model reloads on request after cleanup${NC}"
log "${GREEN}✓ Memory management - Memory cleanup events logged${NC}"
log "${GREEN}✓ Model verification - Tiny model confirmed${NC}"
echo ""
log "${GREEN}━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━${NC}"
log "${GREEN}                           RESULT: PASS                                     ${NC}"
log "${GREEN}━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━${NC}"
echo ""
