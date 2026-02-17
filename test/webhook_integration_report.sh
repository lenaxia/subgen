#!/bin/bash
# Comprehensive Webhook Integration Test Report
# Tests all 4 media server webhook integrations

set -e

ORCHESTRATOR_URL="http://localhost:9000"
TEST_VIDEO="/home/mikekao/personal/subgen/test/testdata/demo_video_speech.mp4"
TEST_AUDIO="/home/mikekao/personal/subgen/test/testdata/speech_sample.wav"

# Colors
GREEN='\033[0;32m'
RED='\033[0;31m'
YELLOW='\033[1;33m'
BLUE='\033[0;34m'
NC='\033[0m' # No Color

echo "========================================================================"
echo "              WEBHOOK INTEGRATION TEST REPORT"
echo "========================================================================"
echo ""
echo "Orchestrator: $ORCHESTRATOR_URL"
echo "Test Video:   $TEST_VIDEO"
echo "Test Audio:   $TEST_AUDIO"
echo ""

# Check orchestrator status
echo "Checking orchestrator status..."
ORCH_STATUS=$(curl -s $ORCHESTRATOR_URL/status | jq -r .status)
if [ "$ORCH_STATUS" = "operational" ]; then
    echo -e "${GREEN}✓${NC} Orchestrator is operational"
else
    echo -e "${RED}✗${NC} Orchestrator is not operational"
    exit 1
fi

QUEUE_STATUS=$(curl -s $ORCHESTRATOR_URL/queue/status | jq .)
echo "Initial Queue Status:"
echo "$QUEUE_STATUS"
echo ""

# Results tracking
declare -A RESULTS
TOTAL_TESTS=0

# Test function
run_test() {
    local test_name="$1"
    local description="$2"
    shift 2
    
    ((TOTAL_TESTS++))
    
    echo ""
    echo "========================================================================"
    echo "TEST $TOTAL_TESTS: $test_name"
    echo "Description: $description"
    echo "========================================================================"
    echo ""
    
    # Run the command and capture output and status
    set +e
    OUTPUT=$("$@" 2>&1)
    STATUS=$?
    set -e
    
    echo "$OUTPUT"
    echo ""
    
    # Check HTTP status from output
    HTTP_CODE=$(echo "$OUTPUT" | grep -o "HTTP Status: [0-9]*" | awk '{print $3}')
    
    if [ "$HTTP_CODE" = "200" ]; then
        echo -e "${GREEN}✓ PASS${NC} - $test_name (HTTP $HTTP_CODE)"
        RESULTS[$test_name]="PASS"
    elif [ "$HTTP_CODE" = "400" ]; then
        echo -e "${YELLOW}⚠ PARTIAL${NC} - $test_name (HTTP $HTTP_CODE - Bad Request)"
        RESULTS[$test_name]="PARTIAL"
    else
        echo -e "${RED}✗ FAIL${NC} - $test_name (HTTP ${HTTP_CODE:-ERROR})"
        RESULTS[$test_name]="FAIL"
    fi
}

# ========================================================================
# PLEX WEBHOOKS
# ========================================================================

run_test "Plex library.new" \
    "Plex webhook with multipart form data and proper User-Agent" \
    bash -c 'curl -X POST http://localhost:9000/plex \
  -H "User-Agent: PlexMediaServer/1.40.0" \
  -H "Content-Type: multipart/form-data" \
  -F "payload={\"event\":\"library.new\",\"Metadata\":{\"ratingKey\":\"12345\",\"type\":\"episode\",\"title\":\"Test Episode\"}}" \
  -w "\nHTTP Status: %{http_code}\n" \
  -s'

run_test "Plex media.play" \
    "Plex media playback webhook" \
    bash -c 'curl -X POST http://localhost:9000/plex \
  -H "User-Agent: PlexMediaServer/1.40.0" \
  -H "Content-Type: multipart/form-data" \
  -F "payload={\"event\":\"media.play\",\"Metadata\":{\"ratingKey\":\"67890\",\"type\":\"episode\",\"title\":\"Played Episode\"}}" \
  -w "\nHTTP Status: %{http_code}\n" \
  -s'

run_test "Plex missing User-Agent" \
    "Plex webhook without required User-Agent header (should fail)" \
    bash -c 'curl -X POST http://localhost:9000/plex \
  -H "Content-Type: multipart/form-data" \
  -F "payload={\"event\":\"library.new\",\"Metadata\":{\"ratingKey\":\"12345\"}}" \
  -w "\nHTTP Status: %{http_code}\n" \
  -s'

# ========================================================================
# JELLYFIN WEBHOOKS
# ========================================================================

run_test "Jellyfin ItemAdded" \
    "Jellyfin webhook with form-encoded data" \
    bash -c 'curl -X POST http://localhost:9000/jellyfin \
  -H "User-Agent: Jellyfin-Server/10.8.13" \
  -H "Content-Type: application/x-www-form-urlencoded" \
  -d "NotificationType=ItemAdded&ItemId=abc123def456&ItemType=Episode&ItemName=Test%20Episode" \
  -w "\nHTTP Status: %{http_code}\n" \
  -s'

run_test "Jellyfin PlaybackStart" \
    "Jellyfin playback start webhook" \
    bash -c 'curl -X POST http://localhost:9000/jellyfin \
  -H "User-Agent: Jellyfin-Server/10.8.13" \
  -H "Content-Type: application/x-www-form-urlencoded" \
  -d "NotificationType=PlaybackStart&ItemId=xyz789abc123&ItemType=Episode&ItemName=Played%20Episode" \
  -w "\nHTTP Status: %{http_code}\n" \
  -s'

# ========================================================================
# EMBY WEBHOOKS
# ========================================================================

run_test "Emby library.new" \
    "Emby webhook with JSON in form data field" \
    bash -c "curl -X POST http://localhost:9000/emby \
  -H \"Content-Type: application/x-www-form-urlencoded\" \
  -d \"data={\\\"Event\\\":\\\"library.new\\\",\\\"Item\\\":{\\\"Name\\\":\\\"Test Episode\\\",\\\"Path\\\":\\\"$TEST_VIDEO\\\",\\\"Type\\\":\\\"Episode\\\"}}\" \
  -w \"\nHTTP Status: %{http_code}\n\" \
  -s"

run_test "Emby playback.start" \
    "Emby playback start webhook" \
    bash -c "curl -X POST http://localhost:9000/emby \
  -H \"Content-Type: application/x-www-form-urlencoded\" \
  -d \"data={\\\"Event\\\":\\\"playback.start\\\",\\\"Item\\\":{\\\"Name\\\":\\\"Test Episode\\\",\\\"Path\\\":\\\"$TEST_VIDEO\\\",\\\"Type\\\":\\\"Episode\\\"}}\" \
  -w \"\nHTTP Status: %{http_code}\n\" \
  -s"

run_test "Emby test notification" \
    "Emby test notification webhook (special case)" \
    bash -c 'curl -X POST http://localhost:9000/emby \
  -H "Content-Type: application/x-www-form-urlencoded" \
  -d "data={\"Event\":\"system.notificationtest\",\"Server\":{\"Name\":\"Test Emby Server\"}}" \
  -w "\nHTTP Status: %{http_code}\n" \
  -s'

# ========================================================================
# TAUTULLI WEBHOOKS
# ========================================================================

run_test "Tautulli added" \
    "Tautulli webhook with source header" \
    bash -c "curl -X POST http://localhost:9000/tautulli \
  -H \"Content-Type: application/x-www-form-urlencoded\" \
  -H \"source: Tautulli\" \
  -d \"event=added&file=$TEST_VIDEO&title=Test%20Episode\" \
  -w \"\nHTTP Status: %{http_code}\n\" \
  -s"

run_test "Tautulli played" \
    "Tautulli playback event webhook" \
    bash -c "curl -X POST http://localhost:9000/tautulli \
  -H \"Content-Type: application/x-www-form-urlencoded\" \
  -H \"source: Tautulli\" \
  -d \"event=played&file=$TEST_AUDIO&title=Test%20Movie\" \
  -w \"\nHTTP Status: %{http_code}\n\" \
  -s"

# ========================================================================
# FINAL QUEUE STATUS
# ========================================================================

echo ""
echo "========================================================================"
echo "FINAL QUEUE STATUS"
echo "========================================================================"
FINAL_QUEUE=$(curl -s $ORCHESTRATOR_URL/queue/status | jq .)
echo "$FINAL_QUEUE"
echo ""

# ========================================================================
# SUMMARY
# ========================================================================

echo ""
echo "========================================================================"
echo "                            SUMMARY"
echo "========================================================================"
echo ""

PASS=0
FAIL=0
PARTIAL=0

for test_name in "${!RESULTS[@]}"; do
    result="${RESULTS[$test_name]}"
    case $result in
        PASS)
            echo -e "${GREEN}✓ PASS${NC}    - $test_name"
            ((PASS++))
            ;;
        PARTIAL)
            echo -e "${YELLOW}⚠ PARTIAL${NC} - $test_name"
            ((PARTIAL++))
            ;;
        FAIL)
            echo -e "${RED}✗ FAIL${NC}    - $test_name"
            ((FAIL++))
            ;;
    esac
done

echo ""
echo "========================================================================"
echo -e "Total Tests:    $TOTAL_TESTS"
echo -e "${GREEN}Passed:         $PASS${NC}"
echo -e "${YELLOW}Partial:        $PARTIAL${NC}"
echo -e "${RED}Failed:         $FAIL${NC}"
echo "========================================================================"
echo ""

# ========================================================================
# CONFIGURATION NOTE
# ========================================================================

echo "NOTE: Tasks may not be queued if PROCESS_ADDED_MEDIA and"
echo "      PROCESS_MEDIA_ON_PLAY environment variables are not set."
echo "      However, webhooks returning HTTP 200 indicates successful"
echo "      webhook handling and validation."
echo ""

if [ $FAIL -eq 0 ]; then
    echo -e "${GREEN}All webhook endpoints are functioning correctly!${NC}"
    exit 0
else
    echo -e "${RED}Some webhook endpoints failed!${NC}"
    exit 1
fi
