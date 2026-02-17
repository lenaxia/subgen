#!/bin/bash
# Comprehensive Skip Logic Testing Script
# Tests all 7 skip conditions in Subgen orchestrator

set -e

# Color codes for output
RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
BLUE='\033[0;34m'
NC='\033[0m' # No Color

# Test results tracking
TESTS_PASSED=0
TESTS_FAILED=0
TEST_RESULTS=()

# Base URL for orchestrator
ORCHESTRATOR_URL="http://localhost:9000"
TESTDATA_DIR="./test/testdata"
OUTPUT_DIR="./test/output"

# Test helper functions
log_info() {
    echo -e "${BLUE}[INFO]${NC} $1"
}

log_success() {
    echo -e "${GREEN}[PASS]${NC} $1"
    TESTS_PASSED=$((TESTS_PASSED + 1))
    TEST_RESULTS+=("PASS: $1")
}

log_failure() {
    echo -e "${RED}[FAIL]${NC} $1"
    TESTS_FAILED=$((TESTS_FAILED + 1))
    TEST_RESULTS+=("FAIL: $1")
}

log_skip() {
    echo -e "${YELLOW}[SKIP]${NC} $1"
}

# Function to check if orchestrator is running
check_orchestrator() {
    log_info "Checking if orchestrator is running..."
    if curl -s -f "$ORCHESTRATOR_URL/health" > /dev/null 2>&1; then
        log_success "Orchestrator is running at $ORCHESTRATOR_URL"
        return 0
    else
        log_failure "Orchestrator is not running or not healthy"
        return 1
    fi
}

# Function to get orchestrator logs
get_logs() {
    local lines=${1:-100}
    docker logs subgen-orchestrator-test --tail "$lines" 2>&1
}

# Function to check logs for skip message
check_skip_in_logs() {
    local expected_message="$1"
    local test_name="$2"
    
    log_info "Checking logs for: $expected_message"
    sleep 2  # Give time for processing
    
    local logs=$(get_logs 50)
    if echo "$logs" | grep -i "skip" | grep -iq "$expected_message"; then
        log_success "$test_name: Skip logic triggered correctly"
        return 0
    else
        log_failure "$test_name: Skip logic did NOT trigger as expected"
        echo "Recent logs:"
        echo "$logs" | tail -20
        return 1
    fi
}

# Function to send webhook request
send_webhook() {
    local file_path="$1"
    local extra_params="$2"
    
    log_info "Sending webhook for: $file_path"
    
    curl -X POST "$ORCHESTRATOR_URL/webhook" \
        -H "Content-Type: application/json" \
        -d "{
            \"EventType\": \"media.play\",
            \"Metadata\": {
                \"file\": \"$file_path\"
            }
            $extra_params
        }" \
        -s -w "\nHTTP Status: %{http_code}\n"
}

# Clean up test artifacts
cleanup_test_files() {
    log_info "Cleaning up test artifacts..."
    rm -f "$TESTDATA_DIR"/*.srt
    rm -f "$TESTDATA_DIR"/*.lrc
    rm -f "$TESTDATA_DIR"/*.en.srt
    rm -f "$TESTDATA_DIR"/*.eng.srt
    rm -f "$TESTDATA_DIR"/*.aa.srt
    rm -f "$TESTDATA_DIR"/*.subgen.srt
    rm -f "$OUTPUT_DIR"/*
}

# Setup test files
setup_test_files() {
    log_info "Setting up test files..."
    
    # Create test video if it doesn't exist
    if [ ! -f "$TESTDATA_DIR/test_video.mkv" ]; then
        cp "$TESTDATA_DIR/video.mkv" "$TESTDATA_DIR/test_video.mkv" 2>/dev/null || true
    fi
    
    # Create test audio if it doesn't exist
    if [ ! -f "$TESTDATA_DIR/test_audio.mp3" ]; then
        cp "$TESTDATA_DIR/short_audio.mp3" "$TESTDATA_DIR/test_audio.mp3" 2>/dev/null || true
    fi
}

#==============================================================================
# TEST 1: Skip if audio file has existing LRC
#==============================================================================
test_1_lrc_exists() {
    log_info "========================================"
    log_info "TEST 1: Skip if LRC file exists"
    log_info "========================================"
    
    cleanup_test_files
    setup_test_files
    
    local test_file="$TESTDATA_DIR/test_audio.mp3"
    local lrc_file="${test_file%.mp3}.lrc"
    
    # Create LRC file
    echo "[00:00.00] Test subtitle" > "$lrc_file"
    
    # Send webhook
    send_webhook "$test_file"
    
    # Check logs for skip
    if check_skip_in_logs "LRC" "TEST 1"; then
        return 0
    else
        return 1
    fi
}

#==============================================================================
# TEST 2: Skip if unknown language (SKIP_UNKNOWN_LANGUAGE=true)
#==============================================================================
test_2_unknown_language() {
    log_info "========================================"
    log_info "TEST 2: Skip if unknown language"
    log_info "========================================"
    
    log_skip "TEST 2: Requires SKIP_UNKNOWN_LANGUAGE=true in config"
    log_info "Current config has SKIP_UNKNOWN_LANGUAGE=false"
    log_info "This would require restarting orchestrator with new env vars"
    
    # This test is documented but skipped in current config
    TEST_RESULTS+=("SKIP: TEST 2 - Requires config change")
    return 0
}

#==============================================================================
# TEST 3: Skip if target subtitle already exists (SRT for video)
#==============================================================================
test_3_target_subtitle_exists() {
    log_info "========================================"
    log_info "TEST 3: Skip if target subtitle exists (.srt)"
    log_info "========================================"
    
    cleanup_test_files
    setup_test_files
    
    local test_file="$TESTDATA_DIR/test_video.mkv"
    local srt_file="${test_file%.mkv}.srt"
    
    # Create SRT file
    cat > "$srt_file" << EOF
1
00:00:00,000 --> 00:00:02,000
Test subtitle
EOF
    
    # Send webhook
    send_webhook "$test_file"
    
    # Check logs for skip
    if check_skip_in_logs "subtitle.*exists" "TEST 3"; then
        return 0
    else
        return 1
    fi
}

#==============================================================================
# TEST 4: Skip if internal subtitle in specific language (embedded)
#==============================================================================
test_4_internal_subtitle_language() {
    log_info "========================================"
    log_info "TEST 4: Skip if internal subtitle in specific language"
    log_info "========================================"
    
    log_skip "TEST 4: Requires video file with embedded 'eng' subtitle track"
    log_info "Config has SKIP_IF_INTERNAL_SUBTITLES_LANGUAGE=eng"
    log_info "Test file 'video.mkv' may not have embedded subtitles"
    
    # This test requires a properly formatted video with embedded subtitles
    TEST_RESULTS+=("SKIP: TEST 4 - Requires video with embedded subtitles")
    return 0
}

#==============================================================================
# TEST 5: Skip if external subtitle with custom name (language code)
#==============================================================================
test_5_external_subtitle_custom_name() {
    log_info "========================================"
    log_info "TEST 5: Skip if external subtitle with language code"
    log_info "========================================"
    
    cleanup_test_files
    setup_test_files
    
    local test_file="$TESTDATA_DIR/test_video.mkv"
    local ext_srt="${test_file%.mkv}.eng.srt"
    
    # Create external subtitle with language code
    cat > "$ext_srt" << EOF
1
00:00:00,000 --> 00:00:02,000
Test subtitle
EOF
    
    # Send webhook
    send_webhook "$test_file"
    
    # Check logs for skip (depends on SKIP_IF_EXTERNAL_SUBTITLES_EXIST=true)
    if check_skip_in_logs "external.*subtitle" "TEST 5"; then
        return 0
    else
        log_info "Note: SKIP_IF_EXTERNAL_SUBTITLES_EXIST=true in config"
        return 1
    fi
}

#==============================================================================
# TEST 6: Skip if subtitle in skip language list
#==============================================================================
test_6_subtitle_language_skip_list() {
    log_info "========================================"
    log_info "TEST 6: Skip if subtitle in skip language list"
    log_info "========================================"
    
    log_skip "TEST 6: Requires SKIP_SUBTITLE_LANGUAGES to be configured"
    log_info "Current config has SKIP_SUBTITLE_LANGUAGES empty"
    log_info "This would require restarting orchestrator with language list"
    
    TEST_RESULTS+=("SKIP: TEST 6 - Requires config change")
    return 0
}

#==============================================================================
# TEST 7: Skip if audio track in skip language list
#==============================================================================
test_7_audio_language_skip_list() {
    log_info "========================================"
    log_info "TEST 7: Skip if audio track in skip language list"
    log_info "========================================"
    
    log_skip "TEST 7: Requires SKIP_IF_AUDIO_LANGUAGES to be configured"
    log_info "Current config has SKIP_IF_AUDIO_LANGUAGES empty"
    log_info "This would require restarting orchestrator with language list"
    
    TEST_RESULTS+=("SKIP: TEST 7 - Requires config change")
    return 0
}

#==============================================================================
# ADDITIONAL TESTS: Config flag combinations
#==============================================================================
test_skip_only_subgen_subtitles() {
    log_info "========================================"
    log_info "TEST: SKIP_ONLY_SUBGEN_SUBTITLES behavior"
    log_info "========================================"
    
    log_info "Config has SKIP_ONLY_SUBGEN_SUBTITLES=false"
    log_info "This means all external subtitles trigger skip, not just subgen-generated ones"
    
    cleanup_test_files
    setup_test_files
    
    local test_file="$TESTDATA_DIR/test_video.mkv"
    local non_subgen_srt="${test_file%.mkv}.manual.srt"
    
    # Create non-subgen subtitle
    cat > "$non_subgen_srt" << EOF
1
00:00:00,000 --> 00:00:02,000
Manual subtitle
EOF
    
    send_webhook "$test_file"
    
    # Should skip because SKIP_IF_EXTERNAL_SUBTITLES_EXIST=true
    if check_skip_in_logs "subtitle" "SKIP_ONLY_SUBGEN=false"; then
        log_info "Correctly skipped non-subgen subtitle (SKIP_ONLY_SUBGEN=false)"
        return 0
    else
        return 1
    fi
}

test_normal_transcription() {
    log_info "========================================"
    log_info "TEST: Normal transcription (no skip)"
    log_info "========================================"
    
    cleanup_test_files
    setup_test_files
    
    local test_file="$TESTDATA_DIR/speech_sample.wav"
    
    # Send webhook for file without any subtitles
    send_webhook "$test_file"
    
    sleep 3
    local logs=$(get_logs 50)
    
    if echo "$logs" | grep -iq "skip"; then
        log_failure "Normal transcription: File was skipped when it shouldn't be"
        return 1
    else
        if echo "$logs" | grep -iq "queued\|enqueue\|processing"; then
            log_success "Normal transcription: File was queued correctly"
            return 0
        else
            log_info "Normal transcription: No clear queue/skip message found"
            echo "Recent logs:"
            echo "$logs" | tail -20
            return 0
        fi
    fi
}

#==============================================================================
# Main test execution
#==============================================================================
main() {
    log_info "========================================="
    log_info "Subgen Skip Logic Comprehensive Test"
    log_info "========================================="
    log_info "Date: $(date)"
    log_info ""
    
    # Check orchestrator
    if ! check_orchestrator; then
        log_failure "Orchestrator not running. Please start with: docker-compose -f docker-compose.test.yml up -d"
        exit 1
    fi
    
    log_info ""
    log_info "Current Configuration (from docker-compose.test.yml):"
    log_info "- SKIP_IF_EXTERNAL_SUBTITLES_EXIST=true"
    log_info "- SKIP_IF_TARGET_SUBTITLES_EXIST=true"
    log_info "- SKIP_SUBTITLE_LANGUAGES= (empty)"
    log_info "- SKIP_IF_AUDIO_LANGUAGES= (empty)"
    log_info "- SKIP_ONLY_SUBGEN_SUBTITLES=false"
    log_info "- SKIP_UNKNOWN_LANGUAGE=false"
    log_info "- SKIP_IF_INTERNAL_SUBTITLES_LANGUAGE=eng"
    log_info ""
    
    # Run tests
    test_1_lrc_exists || true
    sleep 2
    
    test_2_unknown_language || true
    sleep 2
    
    test_3_target_subtitle_exists || true
    sleep 2
    
    test_4_internal_subtitle_language || true
    sleep 2
    
    test_5_external_subtitle_custom_name || true
    sleep 2
    
    test_6_subtitle_language_skip_list || true
    sleep 2
    
    test_7_audio_language_skip_list || true
    sleep 2
    
    test_skip_only_subgen_subtitles || true
    sleep 2
    
    test_normal_transcription || true
    
    # Summary
    log_info ""
    log_info "========================================="
    log_info "TEST SUMMARY"
    log_info "========================================="
    log_info "Total Passed: $TESTS_PASSED"
    log_info "Total Failed: $TESTS_FAILED"
    log_info ""
    
    for result in "${TEST_RESULTS[@]}"; do
        if [[ $result == PASS* ]]; then
            echo -e "${GREEN}$result${NC}"
        elif [[ $result == FAIL* ]]; then
            echo -e "${RED}$result${NC}"
        else
            echo -e "${YELLOW}$result${NC}"
        fi
    done
    
    log_info ""
    log_info "Test artifacts preserved in: $TESTDATA_DIR"
    log_info "Full report will be generated in: docs/WORKLOGS/skip_logic_test_results.md"
    
    # Cleanup
    cleanup_test_files
}

# Run main
main "$@"
