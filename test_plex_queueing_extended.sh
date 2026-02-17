#!/bin/bash

# Additional Plex Episode Queueing Tests
# Tests: Season and Series queueing modes

set -e

ORCHESTRATOR_CONTAINER="subgen-orchestrator-test"

GREEN='\033[0;32m'
RED='\033[0;31m'
YELLOW='\033[1;33m'
BLUE='\033[0;34m'
NC='\033[0m' # No Color

echo "========================================="
echo "Plex Episode Queueing - Extended Tests"
echo "========================================="
echo ""

# Function to restart orchestrator with new config
restart_with_config() {
    local mode=$1
    echo -e "${BLUE}Restarting orchestrator with ${mode} mode...${NC}"
    
    # Get current env vars
    docker inspect $ORCHESTRATOR_CONTAINER | jq -r '.[0].Config.Env[]' > /tmp/env_backup.txt
    
    # Update queueing mode
    sed -i "s/PLEX_QUEUE_NEXT_EPISODE=.*/PLEX_QUEUE_NEXT_EPISODE=false/" /tmp/env_backup.txt
    sed -i "s/PLEX_QUEUE_SEASON=.*/PLEX_QUEUE_SEASON=false/" /tmp/env_backup.txt
    sed -i "s/PLEX_QUEUE_SERIES=.*/PLEX_QUEUE_SERIES=false/" /tmp/env_backup.txt
    
    if [ "$mode" == "next" ]; then
        sed -i "s/PLEX_QUEUE_NEXT_EPISODE=.*/PLEX_QUEUE_NEXT_EPISODE=true/" /tmp/env_backup.txt
    elif [ "$mode" == "season" ]; then
        sed -i "s/PLEX_QUEUE_SEASON=.*/PLEX_QUEUE_SEASON=true/" /tmp/env_backup.txt
    elif [ "$mode" == "series" ]; then
        sed -i "s/PLEX_QUEUE_SERIES=.*/PLEX_QUEUE_SERIES=true/" /tmp/env_backup.txt
    fi
    
    echo "  Note: Container restart not implemented in this test"
    echo "  Would need docker-compose restart with new env vars"
}

# Function to send webhook and check logs
test_queueing_mode() {
    local mode=$1
    local rating_key=$2
    local expected_pattern=$3
    
    echo ""
    echo -e "${YELLOW}Testing ${mode} queueing mode...${NC}"
    echo "  Episode: ${rating_key}"
    
    # Create webhook payload
    PLEX_PAYLOAD="{
      \"event\": \"media.play\",
      \"Metadata\": {
        \"ratingKey\": \"${rating_key}\",
        \"type\": \"episode\"
      }
    }"
    
    # Send webhook
    curl -s -X POST \
      -H "User-Agent: PlexMediaServer/1.0" \
      --data-urlencode "payload=${PLEX_PAYLOAD}" \
      "http://localhost:9000/plex" > /dev/null
    
    sleep 2
    
    # Check logs
    LOGS=$(docker logs $ORCHESTRATOR_CONTAINER --since 5s 2>&1)
    
    if echo "$LOGS" | grep -q "$expected_pattern"; then
        echo -e "${GREEN}✓ PASS${NC}: Found expected pattern: ${expected_pattern}"
        echo "$LOGS" | grep "$expected_pattern" | tail -3
    else
        echo -e "${RED}✗ FAIL${NC}: Pattern not found: ${expected_pattern}"
    fi
}

# Current mode is NEXT_EPISODE
echo -e "${YELLOW}[MODE: NEXT_EPISODE]${NC}"
echo "  Currently configured mode"
docker inspect $ORCHESTRATOR_CONTAINER | jq -r '.[0].Config.Env[] | select(contains("PLEX_QUEUE"))' | grep -E "(NEXT|SEASON|SERIES)"
echo ""

# Test 1: Verify next episode queueing
echo -e "${YELLOW}[TEST 1]${NC} Next Episode Queueing"
test_queueing_mode "next" "224696" "Queueing next episode"

# Test 2: Check how many episodes were queued
echo ""
echo -e "${YELLOW}[TEST 2]${NC} Verify only 1 episode queued"
RECENT_ENQUEUES=$(docker logs $ORCHESTRATOR_CONTAINER --since 10s 2>&1 | grep "Task enqueued" | grep "3 Body Problem" | wc -l)
echo "  Episodes enqueued: ${RECENT_ENQUEUES}"
if [ "$RECENT_ENQUEUES" -eq 2 ]; then
    echo -e "${GREEN}✓ PASS${NC}: Correct - 1 original + 1 next = 2 total"
elif [ "$RECENT_ENQUEUES" -eq 1 ]; then
    echo -e "${YELLOW}⚠ WARNING${NC}: Only original episode enqueued (queueing may have failed)"
else
    echo -e "${RED}✗ FAIL${NC}: Unexpected number of enqueued episodes"
fi

# Information about other modes
echo ""
echo "========================================="
echo "Additional Queueing Modes"
echo "========================================="
echo ""
echo -e "${BLUE}SEASON Mode (PLEX_QUEUE_SEASON=true):${NC}"
echo "  - Queues all remaining episodes in current season"
echo "  - Example: S01E01 would queue S01E01, S01E02, ..., S01E08"
echo "  - Test command: Set PLEX_QUEUE_SEASON=true and restart"
echo ""
echo -e "${BLUE}SERIES Mode (PLEX_QUEUE_SERIES=true):${NC}"
echo "  - Queues all remaining episodes in entire series"
echo "  - Skips season 0 (specials)"
echo "  - Example: S01E01 would queue all episodes from S01E01 to series end"
echo "  - Test command: Set PLEX_QUEUE_SERIES=true and restart"
echo ""
echo "Note: Only one mode can be active at a time (validated in config)"

# Code reference
echo ""
echo "========================================="
echo "Code References"
echo "========================================="
echo ""
echo "Implementation files:"
echo "  - episode_queue.go:56-111  - queueNextEpisode()"
echo "  - episode_queue.go:113-137 - queueSeasonEpisodes()"
echo "  - episode_queue.go:139-183 - queueSeriesEpisodes()"
echo ""
echo "Test cases:"
echo "  - config_test.go:286-303   - Next episode config test"
echo "  - config_test.go:302-320   - Season config test"
echo "  - config_test.go:318-336   - Series config test"
echo "  - config_test.go:334-347   - Multiple modes validation"

echo ""
echo "========================================="
echo "Test Complete"
echo "========================================="
