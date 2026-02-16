#!/bin/bash
# Run gRPC integration tests with Docker Compose
#
# Usage:
#   ./run_integration_tests.sh         # Run all tests
#   ./run_integration_tests.sh -v      # Verbose output
#   ./run_integration_tests.sh -stop   # Stop services after tests

set -e

# Colors for output
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
RED='\033[0;31m'
NC='\033[0m' # No Color

# Configuration
COMPOSE_FILE="../docker-compose.grpc-test.yml"
TEST_DIR="../integration"
VERBOSE=""
STOP_AFTER=false

# Parse arguments
for arg in "$@"; do
    case $arg in
        -v|--verbose)
            VERBOSE="-v"
            shift
            ;;
        -stop|--stop)
            STOP_AFTER=true
            shift
            ;;
        -h|--help)
            echo "Usage: $0 [-v] [-stop] [-h]"
            echo "  -v, --verbose    Verbose test output"
            echo "  -stop, --stop    Stop Docker services after tests"
            echo "  -h, --help       Show this help"
            exit 0
            ;;
    esac
done

echo -e "${GREEN}=== gRPC Integration Test Runner ===${NC}"
echo ""

# Step 1: Check Docker Compose is available
echo -e "${YELLOW}[1/5]${NC} Checking Docker Compose..."
if ! command -v docker-compose &> /dev/null; then
    echo -e "${RED}Error: docker-compose not found${NC}"
    exit 1
fi
echo "  ✓ Docker Compose available"
echo ""

# Step 2: Start services
echo -e "${YELLOW}[2/5]${NC} Starting Docker Compose services..."
cd ..
docker-compose -f docker-compose.grpc-test.yml up -d
echo "  ✓ Services started"
echo ""

# Step 3: Wait for services to be healthy
echo -e "${YELLOW}[3/5]${NC} Waiting for services to be healthy..."
echo "  This may take 30-60 seconds for worker to download Whisper model..."
MAX_WAIT=120
ELAPSED=0
while [ $ELAPSED -lt $MAX_WAIT ]; do
    WORKER_HEALTH=$(docker inspect --format='{{.State.Health.Status}}' subgen-worker-integration-test 2>/dev/null || echo "starting")
    ORCHESTRATOR_HEALTH=$(docker inspect --format='{{.State.Health.Status}}' subgen-orchestrator-integration-test 2>/dev/null || echo "starting")
    
    if [ "$WORKER_HEALTH" = "healthy" ] && [ "$ORCHESTRATOR_HEALTH" = "healthy" ]; then
        echo "  ✓ All services healthy"
        break
    fi
    
    echo "  Worker: $WORKER_HEALTH, Orchestrator: $ORCHESTRATOR_HEALTH (${ELAPSED}s elapsed)"
    sleep 5
    ELAPSED=$((ELAPSED + 5))
done

if [ $ELAPSED -ge $MAX_WAIT ]; then
    echo -e "${RED}  ✗ Services failed to become healthy after ${MAX_WAIT}s${NC}"
    echo ""
    echo "Logs:"
    docker-compose -f docker-compose.grpc-test.yml logs --tail=50
    exit 1
fi
echo ""

# Step 4: Run Go integration tests
echo -e "${YELLOW}[4/5]${NC} Running Go integration tests..."
cd integration
if go test $VERBOSE ./...; then
    echo -e "${GREEN}  ✓ All Go integration tests passed${NC}"
    TEST_RESULT=0
else
    echo -e "${RED}  ✗ Some Go integration tests failed${NC}"
    TEST_RESULT=1
fi
echo ""

# Step 5: Cleanup
if [ "$STOP_AFTER" = true ]; then
    echo -e "${YELLOW}[5/5]${NC} Stopping Docker Compose services..."
    cd ..
    docker-compose -f docker-compose.grpc-test.yml down
    echo "  ✓ Services stopped"
else
    echo -e "${YELLOW}[5/5]${NC} Services still running (use -stop to stop)"
    echo "  To stop services: docker-compose -f docker-compose.grpc-test.yml down"
    echo "  To view logs: docker-compose -f docker-compose.grpc-test.yml logs -f"
fi
echo ""

# Final result
if [ $TEST_RESULT -eq 0 ]; then
    echo -e "${GREEN}=== All tests passed ===${NC}"
else
    echo -e "${RED}=== Some tests failed ===${NC}"
fi

exit $TEST_RESULT
