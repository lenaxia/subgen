#!/bin/bash
# Webhook Integration Test Script
# Tests all 4 media server webhooks: Plex, Jellyfin, Emby, Tautulli

set -e

ORCHESTRATOR_URL="http://localhost:9000"
TEST_VIDEO="/home/mikekao/personal/subgen/test/testdata/demo_video_speech.mp4"
TEST_AUDIO="/home/mikekao/personal/subgen/test/testdata/speech_sample.wav"

echo "================================================"
echo "WEBHOOK INTEGRATION TESTS"
echo "================================================"
echo ""

# Colors for output
GREEN='\033[0;32m'
RED='\033[0;31m'
YELLOW='\033[1;33m'
NC='\033[0m' # No Color

# Test counter
PASS=0
FAIL=0

# Function to test webhook
test_webhook() {
    local name="$1"
    local response="$2"
    local http_code=$(echo "$response" | tail -n1)
    
    echo "Testing: $name"
    echo "Response: $response"
    
    if [ "$http_code" = "200" ]; then
        echo -e "${GREEN}✓ PASS${NC} - $name (HTTP $http_code)"
        ((PASS++))
    else
        echo -e "${RED}✗ FAIL${NC} - $name (HTTP $http_code)"
        ((FAIL++))
    fi
    echo ""
}

echo "================================================"
echo "TEST 1: PLEX WEBHOOK"
echo "================================================"

# Plex sends JSON payload in multipart form data with specific User-Agent
PLEX_PAYLOAD=$(cat <<'EOF'
{
  "event": "library.new",
  "user": true,
  "owner": true,
  "Account": {
    "id": 1,
    "thumb": "https://plex.tv/users/1/avatar",
    "title": "Test User"
  },
  "Server": {
    "title": "Test Plex Server",
    "uuid": "test-uuid-123"
  },
  "Metadata": {
    "librarySectionType": "show",
    "ratingKey": "12345",
    "key": "/library/metadata/12345",
    "guid": "plex://episode/5d9c086fe9d5a1001f4d4c1d",
    "type": "episode",
    "title": "Test Episode",
    "grandparentTitle": "Test Show",
    "parentTitle": "Season 1",
    "index": 1,
    "parentIndex": 1,
    "year": 2024
  }
}
EOF
)

echo "Payload: $PLEX_PAYLOAD"
echo ""
echo "Sending Plex webhook..."

PLEX_RESPONSE=$(curl -s -w "\n%{http_code}" \
  -X POST "$ORCHESTRATOR_URL/plex" \
  -H "User-Agent: PlexMediaServer/1.40.0" \
  -H "Content-Type: multipart/form-data" \
  -F "payload=$PLEX_PAYLOAD")

test_webhook "Plex library.new" "$PLEX_RESPONSE"

echo "================================================"
echo "TEST 2: PLEX MEDIA.PLAY WEBHOOK"
echo "================================================"

PLEX_PLAY_PAYLOAD=$(cat <<'EOF'
{
  "event": "media.play",
  "user": true,
  "owner": true,
  "Account": {
    "id": 1,
    "title": "Test User"
  },
  "Metadata": {
    "ratingKey": "67890",
    "type": "episode",
    "title": "Played Episode",
    "key": "/library/metadata/67890"
  }
}
EOF
)

echo "Payload: $PLEX_PLAY_PAYLOAD"
echo ""
echo "Sending Plex media.play webhook..."

PLEX_PLAY_RESPONSE=$(curl -s -w "\n%{http_code}" \
  -X POST "$ORCHESTRATOR_URL/plex" \
  -H "User-Agent: PlexMediaServer/1.40.0" \
  -H "Content-Type: multipart/form-data" \
  -F "payload=$PLEX_PLAY_PAYLOAD")

test_webhook "Plex media.play" "$PLEX_PLAY_RESPONSE"

echo "================================================"
echo "TEST 3: JELLYFIN WEBHOOK"
echo "================================================"

# Jellyfin sends form-encoded data
JELLYFIN_PAYLOAD="NotificationType=ItemAdded&ItemId=abc123def456&ItemType=Episode&ItemName=Test%20Episode&SeriesName=Test%20Show&SeasonNumber=1&EpisodeNumber=1"

echo "Payload: $JELLYFIN_PAYLOAD"
echo ""
echo "Sending Jellyfin ItemAdded webhook..."

JELLYFIN_RESPONSE=$(curl -s -w "\n%{http_code}" \
  -X POST "$ORCHESTRATOR_URL/jellyfin" \
  -H "User-Agent: Jellyfin-Server/10.8.13" \
  -H "Content-Type: application/x-www-form-urlencoded" \
  -d "$JELLYFIN_PAYLOAD")

test_webhook "Jellyfin ItemAdded" "$JELLYFIN_RESPONSE"

echo "================================================"
echo "TEST 4: JELLYFIN PLAYBACK START WEBHOOK"
echo "================================================"

JELLYFIN_PLAYBACK_PAYLOAD="NotificationType=PlaybackStart&ItemId=xyz789abc123&ItemType=Episode&ItemName=Played%20Episode"

echo "Payload: $JELLYFIN_PLAYBACK_PAYLOAD"
echo ""
echo "Sending Jellyfin PlaybackStart webhook..."

JELLYFIN_PLAYBACK_RESPONSE=$(curl -s -w "\n%{http_code}" \
  -X POST "$ORCHESTRATOR_URL/jellyfin" \
  -H "User-Agent: Jellyfin-Server/10.8.13" \
  -H "Content-Type: application/x-www-form-urlencoded" \
  -d "$JELLYFIN_PLAYBACK_PAYLOAD")

test_webhook "Jellyfin PlaybackStart" "$JELLYFIN_PLAYBACK_RESPONSE"

echo "================================================"
echo "TEST 5: EMBY WEBHOOK"
echo "================================================"

# Emby sends form-encoded with JSON in 'data' field
EMBY_PAYLOAD="data={\"Event\":\"library.new\",\"Item\":{\"Name\":\"Test Episode\",\"Path\":\"$TEST_VIDEO\",\"Type\":\"Episode\",\"ServerId\":\"abc123\",\"Id\":\"item123\"},\"Server\":{\"Name\":\"Test Emby Server\",\"Id\":\"server123\"}}"

echo "Payload: $EMBY_PAYLOAD"
echo ""
echo "Sending Emby library.new webhook..."

EMBY_RESPONSE=$(curl -s -w "\n%{http_code}" \
  -X POST "$ORCHESTRATOR_URL/emby" \
  -H "Content-Type: application/x-www-form-urlencoded" \
  -d "$EMBY_PAYLOAD")

test_webhook "Emby library.new" "$EMBY_RESPONSE"

echo "================================================"
echo "TEST 6: EMBY TEST NOTIFICATION"
echo "================================================"

EMBY_TEST_PAYLOAD='data={"Event":"system.notificationtest","Server":{"Name":"Test Emby Server"}}'

echo "Payload: $EMBY_TEST_PAYLOAD"
echo ""
echo "Sending Emby test notification..."

EMBY_TEST_RESPONSE=$(curl -s -w "\n%{http_code}" \
  -X POST "$ORCHESTRATOR_URL/emby" \
  -H "Content-Type: application/x-www-form-urlencoded" \
  -d "$EMBY_TEST_PAYLOAD")

test_webhook "Emby test notification" "$EMBY_TEST_RESPONSE"

echo "================================================"
echo "TEST 7: TAUTULLI WEBHOOK"
echo "================================================"

# Tautulli sends form-encoded data with 'source' header
TAUTULLI_PAYLOAD="event=added&file=$TEST_VIDEO&title=Test%20Episode&show_name=Test%20Show&season_num=1&episode_num=1"

echo "Payload: $TAUTULLI_PAYLOAD"
echo ""
echo "Sending Tautulli added webhook..."

TAUTULLI_RESPONSE=$(curl -s -w "\n%{http_code}" \
  -X POST "$ORCHESTRATOR_URL/tautulli" \
  -H "Content-Type: application/x-www-form-urlencoded" \
  -H "source: Tautulli" \
  -d "$TAUTULLI_PAYLOAD")

test_webhook "Tautulli added" "$TAUTULLI_RESPONSE"

echo "================================================"
echo "TEST 8: TAUTULLI PLAYED WEBHOOK"
echo "================================================"

TAUTULLI_PLAYED_PAYLOAD="event=played&file=$TEST_AUDIO&title=Test%20Movie"

echo "Payload: $TAUTULLI_PLAYED_PAYLOAD"
echo ""
echo "Sending Tautulli played webhook..."

TAUTULLI_PLAYED_RESPONSE=$(curl -s -w "\n%{http_code}" \
  -X POST "$ORCHESTRATOR_URL/tautulli" \
  -H "Content-Type: application/x-www-form-urlencoded" \
  -H "source: Tautulli" \
  -d "$TAUTULLI_PLAYED_PAYLOAD")

test_webhook "Tautulli played" "$TAUTULLI_PLAYED_RESPONSE"

echo "================================================"
echo "SUMMARY"
echo "================================================"
echo -e "${GREEN}PASSED:${NC} $PASS"
echo -e "${RED}FAILED:${NC} $FAIL"
echo ""

if [ $FAIL -eq 0 ]; then
    echo -e "${GREEN}ALL TESTS PASSED!${NC}"
    exit 0
else
    echo -e "${RED}SOME TESTS FAILED!${NC}"
    exit 1
fi
