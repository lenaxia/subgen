#!/bin/bash
# Workflow validation script
# Run this before committing workflow changes

set -e

echo "=================================================="
echo "GitHub Actions Workflow Validation"
echo "=================================================="
echo ""

# Color codes
RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
NC='\033[0m' # No Color

ERRORS=0

# Check 1: YAML syntax
echo "🔍 Checking YAML syntax..."
for file in .github/workflows/*.yml; do
    if python3 -c "import yaml; yaml.safe_load(open('$file'))" 2>/dev/null; then
        echo -e "  ${GREEN}✓${NC} $(basename $file)"
    else
        echo -e "  ${RED}✗${NC} $(basename $file) - INVALID YAML"
        ERRORS=$((ERRORS + 1))
    fi
done
echo ""

# Check 2: Actions have version tags
echo "🔍 Checking action version tags..."
UNVERSIONED=$(grep -r "uses: actions/" .github/workflows/ | grep -v "@v" | wc -l)
if [ "$UNVERSIONED" -eq 0 ]; then
    echo -e "  ${GREEN}✓${NC} All actions have version tags"
else
    echo -e "  ${YELLOW}⚠${NC} Found $UNVERSIONED actions without version tags"
fi
echo ""

# Check 3: Test paths exist
echo "🔍 Checking test paths..."

# Go tests
if [ -d "orchestrator/internal" ]; then
    GO_TESTS=$(find orchestrator/internal -name "*_test.go" | wc -l)
    echo -e "  ${GREEN}✓${NC} Found $GO_TESTS Go unit tests"
else
    echo -e "  ${RED}✗${NC} orchestrator/internal not found"
    ERRORS=$((ERRORS + 1))
fi

# Go integration tests
if [ -d "orchestrator/test/integration" ]; then
    GO_INT_TESTS=$(find orchestrator/test/integration -name "*_test.go" | wc -l)
    echo -e "  ${GREEN}✓${NC} Found $GO_INT_TESTS Go integration tests"
else
    echo -e "  ${RED}✗${NC} orchestrator/test/integration not found"
    ERRORS=$((ERRORS + 1))
fi

# Python tests
if [ -d "worker/tests/unit" ]; then
    PY_UNIT_TESTS=$(find worker/tests/unit -name "test_*.py" | wc -l)
    echo -e "  ${GREEN}✓${NC} Found $PY_UNIT_TESTS Python unit tests"
else
    echo -e "  ${RED}✗${NC} worker/tests/unit not found"
    ERRORS=$((ERRORS + 1))
fi

if [ -d "worker/tests/integration" ]; then
    PY_INT_TESTS=$(find worker/tests/integration -name "test_*.py" | wc -l)
    echo -e "  ${GREEN}✓${NC} Found $PY_INT_TESTS Python integration tests"
else
    echo -e "  ${RED}✗${NC} worker/tests/integration not found"
    ERRORS=$((ERRORS + 1))
fi

# Test data
if [ -d "test/testdata" ]; then
    TEST_FILES=$(find test/testdata -type f \( -name "*.mp3" -o -name "*.wav" -o -name "*.mp4" -o -name "*.mkv" \) | wc -l)
    echo -e "  ${GREEN}✓${NC} Found $TEST_FILES test data files"
else
    echo -e "  ${RED}✗${NC} test/testdata not found"
    ERRORS=$((ERRORS + 1))
fi
echo ""

# Check 4: Workflow structure
echo "🔍 Checking workflow structure..."
for file in .github/workflows/test-*.yml; do
    JOBS=$(grep -c "^  [a-z_-]*:" "$file" 2>/dev/null || echo 0)
    echo "  $(basename $file): $JOBS jobs"
done
echo ""

# Summary
echo "=================================================="
if [ $ERRORS -eq 0 ]; then
    echo -e "${GREEN}✅ All checks passed!${NC}"
    echo ""
    echo "Workflow Summary:"
    echo "  • test-orchestrator.yml: Go tests (unit, integration, real-world)"
    echo "  • test-worker.yml: Python tests (unit, integration, memory leaks)"
    echo "  • test-e2e.yml: End-to-end integration tests"
    echo "  • build-go.yml: Builds after Go tests pass"
    echo "  • build_GPU.yml: Builds GPU Docker image"
    echo "  • build_CPU.yml: Builds CPU Docker image (multi-arch)"
    exit 0
else
    echo -e "${RED}❌ Found $ERRORS error(s)${NC}"
    exit 1
fi
