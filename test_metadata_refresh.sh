#!/bin/bash

set -e

echo "============================================"
echo "Metadata Refresh Test"
echo "============================================"
echo ""
echo "Test Configuration:"
echo "- Plex Server: http://192.168.5.104:32400"
echo "- Jellyfin Server: http://192.168.5.144:8096"
echo "- Using dummy tokens (expect 401 errors)"
echo ""

# Create a test webhook payload
WEBHOOK_PAYLOAD=$(cat <<'EOF'
{
  "Account": {
    "title": "TestUser"
  },
  "event": "library.new",
  "Metadata": {
    "ratingKey": "12345",
    "type": "episode",
    "title": "Test Episode",
    "librarySectionTitle": "TV Shows"
  },
  "Server": {
    "title": "Test Plex Server"
  }
}
EOF
)

echo "Step 1: Trigger Plex webhook"
echo "------------------------------"
PLEX_RESPONSE=$(curl -s -w "\nHTTP_CODE:%{http_code}" \
  -X POST http://localhost:9000/plex \
  -H "Content-Type: multipart/form-data" \
  -H "User-Agent: PlexMediaServer/1.40.0" \
  -F "payload=$WEBHOOK_PAYLOAD")

PLEX_HTTP_CODE=$(echo "$PLEX_RESPONSE" | grep HTTP_CODE | cut -d: -f2)
PLEX_BODY=$(echo "$PLEX_RESPONSE" | grep -v HTTP_CODE)

echo "HTTP Status Code: $PLEX_HTTP_CODE"
echo "Response: $PLEX_BODY"
echo ""

# Jellyfin webhook
echo "Step 2: Trigger Jellyfin webhook"
echo "---------------------------------"
JELLYFIN_RESPONSE=$(curl -s -w "\nHTTP_CODE:%{http_code}" \
  -X POST http://localhost:9000/jellyfin \
  -H "Content-Type: application/x-www-form-urlencoded" \
  -H "User-Agent: Jellyfin-Server/10.8.13" \
  -d "NotificationType=ItemAdded" \
  -d "ItemId=abc123def456" \
  -d "ItemType=Episode" \
  -d "Name=Test Episode" \
  -d "Year=2024")

JELLYFIN_HTTP_CODE=$(echo "$JELLYFIN_RESPONSE" | grep HTTP_CODE | cut -d: -f2)
JELLYFIN_BODY=$(echo "$JELLYFIN_RESPONSE" | grep -v HTTP_CODE)

echo "HTTP Status Code: $JELLYFIN_HTTP_CODE"
echo "Response: $JELLYFIN_BODY"
echo ""

# Wait for processing
echo "Step 3: Wait for processing (10 seconds)"
echo "----------------------------------------"
sleep 10

# Check orchestrator logs for metadata refresh attempts
echo "Step 4: Check orchestrator logs"
echo "--------------------------------"
echo ""
echo "Looking for Plex metadata refresh attempts:"
docker logs subgen-orchestrator-test --tail 100 2>&1 | grep -i "plex.*metadata\|refresh.*plex" || echo "  No Plex refresh logs found"
echo ""
echo "Looking for Jellyfin metadata refresh attempts:"
docker logs subgen-orchestrator-test --tail 100 2>&1 | grep -i "jellyfin.*metadata\|refresh.*jellyfin" || echo "  No Jellyfin refresh logs found"
echo ""

# Check for HTTP errors (401 expected with dummy tokens)
echo "Step 5: Check for API call attempts (401 expected)"
echo "---------------------------------------------------"
echo ""
echo "Plex API errors:"
docker logs subgen-orchestrator-test --tail 100 2>&1 | grep -i "plex.*401\|plex.*unauthorized\|plex.*token" || echo "  No Plex API error logs found"
echo ""
echo "Jellyfin API errors:"
docker logs subgen-orchestrator-test --tail 100 2>&1 | grep -i "jellyfin.*401\|jellyfin.*unauthorized\|jellyfin.*token" || echo "  No Jellyfin API error logs found"
echo ""

# Full log dump for analysis
echo "Step 6: Full orchestrator log dump (last 50 lines)"
echo "---------------------------------------------------"
docker logs subgen-orchestrator-test --tail 50 2>&1

echo ""
echo "============================================"
echo "Test Complete"
echo "============================================"
