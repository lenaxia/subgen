#!/bin/bash
# Integration test for health check endpoints
# Tests both worker and orchestrator health endpoints

set -e

ORCHESTRATOR_URL="${ORCHESTRATOR_URL:-http://localhost:9000}"
WORKER_URL="${WORKER_URL:-http://localhost:8080}"

PASSED=0
FAILED=0

# Colors for output
RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
NC='\033[0m' # No Color

log_test() {
    echo -e "${YELLOW}TEST:${NC} $1"
}

log_pass() {
    echo -e "${GREEN}✓ PASS:${NC} $1"
    PASSED=$((PASSED + 1))
}

log_fail() {
    echo -e "${RED}✗ FAIL:${NC} $1"
    FAILED=$((FAILED + 1))
}

test_endpoint() {
    local name=$1
    local url=$2
    local expected_code=$3
    local expected_field=$4

    log_test "$name"
    
    response=$(curl -s -w "\n%{http_code}" "$url")
    body=$(echo "$response" | head -n -1)
    code=$(echo "$response" | tail -n 1)
    
    if [ "$code" != "$expected_code" ]; then
        log_fail "$name - Expected HTTP $expected_code, got $code"
        return 1
    fi
    
    if [ -n "$expected_field" ]; then
        if echo "$body" | grep -q "$expected_field"; then
            log_pass "$name - HTTP $code, contains '$expected_field'"
        else
            log_fail "$name - Response missing expected field: $expected_field"
            echo "Response: $body"
            return 1
        fi
    else
        log_pass "$name - HTTP $code"
    fi
}

echo "========================================="
echo "  Health Endpoint Integration Tests"
echo "========================================="
echo ""
echo "Orchestrator: $ORCHESTRATOR_URL"
echo "Worker: $WORKER_URL"
echo ""

# Wait for services to be ready
echo "Waiting for services to start..."
sleep 5

echo ""
echo "========================================="
echo "  Worker Health Endpoints"
echo "========================================="

test_endpoint "Worker /health" "$WORKER_URL/health" "200" "alive"
test_endpoint "Worker /ready" "$WORKER_URL/ready" "200" "status"
test_endpoint "Worker /metrics" "$WORKER_URL/metrics" "200" "memory_mb"

echo ""
echo "========================================="
echo "  Orchestrator Health Endpoints"
echo "========================================="

test_endpoint "Orchestrator /health" "$ORCHESTRATOR_URL/health" "200" "alive"
test_endpoint "Orchestrator /ready" "$ORCHESTRATOR_URL/ready" "200" "status"
test_endpoint "Orchestrator /live" "$ORCHESTRATOR_URL/live" "200" "alive"

echo ""
echo "========================================="
echo "  Docker Health Status"
echo "========================================="

if command -v docker &> /dev/null; then
    echo ""
    echo "Orchestrator container health:"
    docker inspect subgen-orchestrator --format='{{.State.Health.Status}}' 2>/dev/null || echo "Not running or health check not configured"
    
    echo ""
    echo "Worker container health:"
    docker inspect subgen-worker --format='{{.State.Health.Status}}' 2>/dev/null || echo "Not running or health check not configured"
fi

echo ""
echo "========================================="
echo "  Test Summary"
echo "========================================="
echo -e "${GREEN}Passed: $PASSED${NC}"
echo -e "${RED}Failed: $FAILED${NC}"
echo ""

if [ $FAILED -eq 0 ]; then
    echo -e "${GREEN}✓ All health endpoint tests passed!${NC}"
    exit 0
else
    echo -e "${RED}✗ Some tests failed${NC}"
    exit 1
fi
