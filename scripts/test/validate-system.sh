#!/bin/bash
set -e

echo "========================================="
echo "Subgen Hybrid Architecture Validation"
echo "========================================="
echo ""

# Colors for output
GREEN='\033[0;32m'
RED='\033[0;31m'
YELLOW='\033[1;33m'
NC='\033[0m' # No Color

# Check if docker-compose is installed
if ! command -v docker-compose &> /dev/null; then
    echo -e "${RED}✗ docker-compose not found${NC}"
    exit 1
fi

echo -e "${GREEN}✓ docker-compose found${NC}"

# Build images
echo ""
echo "Building Docker images..."
docker-compose -f docker-compose.hybrid.yml build

# Start services
echo ""
echo "Starting services..."
docker-compose -f docker-compose.hybrid.yml up -d

# Wait for services to be healthy
echo ""
echo "Waiting for services to be healthy..."
sleep 5

# Check orchestrator health
echo ""
echo "Checking orchestrator health..."
if docker-compose -f docker-compose.hybrid.yml ps orchestrator | grep -q "Up"; then
    echo -e "${GREEN}✓ Orchestrator is running${NC}"
else
    echo -e "${RED}✗ Orchestrator failed to start${NC}"
    docker-compose -f docker-compose.hybrid.yml logs orchestrator
    exit 1
fi

# Check worker health
echo ""
echo "Checking worker health..."
if docker-compose -f docker-compose.hybrid.yml ps worker | grep -q "Up"; then
    echo -e "${GREEN}✓ Worker is running${NC}"
else
    echo -e "${RED}✗ Worker failed to start${NC}"
    docker-compose -f docker-compose.hybrid.yml logs worker
    exit 1
fi

# Test webhook endpoint
echo ""
echo "Testing webhook endpoint..."
response=$(curl -s -o /dev/null -w "%{http_code}" http://localhost:9000/health || echo "000")
if [ "$response" = "200" ]; then
    echo -e "${GREEN}✓ Webhook endpoint responding${NC}"
else
    echo -e "${YELLOW}⚠ Webhook endpoint returned: $response${NC}"
fi

# Test metrics endpoint
echo ""
echo "Testing metrics endpoint..."
response=$(curl -s -o /dev/null -w "%{http_code}" http://localhost:9090/metrics || echo "000")
if [ "$response" = "200" ]; then
    echo -e "${GREEN}✓ Metrics endpoint responding${NC}"
else
    echo -e "${YELLOW}⚠ Metrics endpoint returned: $response${NC}"
fi

# Check logs for gRPC connection
echo ""
echo "Checking logs for worker discovery..."
if docker-compose -f docker-compose.hybrid.yml logs orchestrator | grep -q "worker"; then
    echo -e "${GREEN}✓ Orchestrator discovered worker${NC}"
else
    echo -e "${YELLOW}⚠ No worker discovery logs found${NC}"
fi

# Send test webhook (optional - requires media file)
echo ""
echo "========================================="
echo "Manual Test: Send a test webhook"
echo "========================================="
echo ""
echo "Example Plex webhook:"
echo "curl -X POST http://localhost:9000/plex \\"
echo "  -H 'Content-Type: application/json' \\"
echo "  -d '{\"event\":\"media.play\",\"Metadata\":{\"librarySectionType\":\"movie\",\"ratingKey\":\"12345\"}}'"
echo ""
echo "Example Jellyfin webhook:"
echo "curl -X POST http://localhost:9000/jellyfin \\"
echo "  -H 'Content-Type: application/json' \\"
echo "  -d '{\"NotificationType\":\"PlaybackStart\",\"ItemType\":\"Movie\",\"ItemId\":\"12345\"}'"
echo ""

# Summary
echo ""
echo "========================================="
echo "Validation Summary"
echo "========================================="
echo -e "${GREEN}✓ Both services are running${NC}"
echo ""
echo "To view logs:"
echo "  docker-compose -f docker-compose.hybrid.yml logs -f"
echo ""
echo "To scale workers:"
echo "  docker-compose -f docker-compose.hybrid.yml up -d --scale worker=3"
echo ""
echo "To stop services:"
echo "  docker-compose -f docker-compose.hybrid.yml down"
echo ""
