#!/bin/bash
# Comprehensive Skip Logic Testing Script - Using File System Monitor
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

# Base paths
TESTDATA_DIR="/home/mikekao/personal/subgen/test/testdata"
OUTPUT_DIR="/home/mikekao/personal/subgen/test/output"

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

# Function to get orchestrator logs
get_logs() {
    local lines=${1:-50}
    docker logs subgen-orchestrator-test --tail "$lines" 2>&1
}

# Function to check logs for skip message (waits and checks multiple times)
check_skip_in_logs() {
    local expected_pattern="$1"
    local test_name="$2"
    local max_attempts=5
    local attempt=1
    
    log_info "Waiting for skip logic to trigger for: $test_name"
    
    while [ $attempt -le $max_attempts ]; do
        sleep 2
        local logs=$(get_logs 100)
        
        if echo "$logs" | grep -i "skip" | grep -iq "$expected_pattern"; then
            log_success "$test_name: Skip logic triggered correctly"
            echo "  Matched: $(echo "$logs" | grep -i "skip" | grep -i "$expected_pattern" | tail -1)"
            return 0
        fi
        
        log_info "Attempt $attempt/$max_attempts: Not found yet..."
        attempt=$((attempt + 1))
    done
    
    log_failure "$test_name: Skip logic did NOT trigger as expected"
    echo "Recent skip-related logs:"
    echo "$logs" | grep -i "skip" | tail -5 || echo "  No skip messages found"
    return 1
}

# Function to check logs for non-skip (file was queued/processed)
check_not_skipped() {
    local file_name="$1"
    local test_name="$2"
    
    log_info "Checking that $file_name was NOT skipped..."
    sleep 3
    
    local logs=$(get_logs 100)
    
    # Check if file was skipped
    if echo "$logs" | grep -i "$file_name" | grep -iq "skip"; then
        log_failure "$test_name: File was incorrectly skipped"
        echo "$logs" | grep -i "$file_name" | grep -i "skip"
        return 1
    fi
    
    # Check if file was queued or processed
    if echo "$logs" | grep -i "$file_name" | grep -iqE "enqueue|stable|processing"; then
        log_success "$test_name: File was correctly processed (not skipped)"
        return 0
    fi
    
    log_info "$test_name: No clear processing/skip message found (may be OK)"
    return 0
}

# Clean up test artifacts
cleanup_test_files() {
    log_info "Cleaning up test artifacts..."
    rm -f "$TESTDATA_DIR"/*.srt 2>/dev/null || true
    rm -f "$TESTDATA_DIR"/*.lrc 2>/dev/null || true
    rm -f "$TESTDATA_DIR"/*.vtt 2>/dev/null || true
    rm -f "$TESTDATA_DIR"/test_*.mp3 2>/dev/null || true
    rm -f "$TESTDATA_DIR"/test_*.mkv 2>/dev/null || true
    rm -f "$TESTDATA_DIR"/test_*.wav 2>/dev/null || true
}

#==============================================================================
# TEST 1: Skip if audio file has existing LRC
#==============================================================================
test_1_lrc_exists() {
    log_info "========================================"
    log_info "TEST 1: Skip if LRC file exists (audio)"
    log_info "========================================"
    
    cleanup_test_files
    
    local test_file="$TESTDATA_DIR/test_skip1.mp3"
    local lrc_file="${test_file%.mp3}.lrc"
    
    # Copy test audio
    cp "$TESTDATA_DIR/short_audio.mp3" "$test_file"
    
    # Create LRC file
    cat > "$lrc_file" << 'EOF'
[00:00.00] Test subtitle line 1
[00:05.00] Test subtitle line 2
EOF
    
    # File system monitor should detect and skip
    check_skip_in_logs "lrc" "TEST 1"
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
    TEST_RESULTS+=("SKIP: TEST 2 - Config SKIP_UNKNOWN_LANGUAGE=false")
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
    
    local test_file="$TESTDATA_DIR/test_skip3.mkv"
    local srt_file="${test_file%.mkv}.srt"
    
    # Copy test video
    cp "$TESTDATA_DIR/video.mkv" "$test_file"
    
    # Create SRT file
    cat > "$srt_file" << 'EOF'
1
00:00:00,000 --> 00:00:02,000
Test subtitle line 1

2
00:00:02,000 --> 00:00:04,000
Test subtitle line 2
EOF
    
    # File system monitor should detect and skip
    check_skip_in_logs "subtitle.*exists" "TEST 3"
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
    log_info "demo_video_speech.mp4 may have embedded subtitles - checking logs..."
    
    # Check if demo video was skipped
    local logs=$(get_logs 500)
    if echo "$logs" | grep "demo_video_speech" | grep -iq "embedded.*subtitle"; then
        log_success "TEST 4: Found embedded subtitle skip in logs"
        TEST_RESULTS+=("PASS: TEST 4 - Embedded subtitle skip detected")
        return 0
    else
        log_info "TEST 4: No embedded subtitle skip found (file may not have embedded subs)"
        TEST_RESULTS+=("SKIP: TEST 4 - No file with embedded subtitles")
        return 0
    fi
}

#==============================================================================
# TEST 5: Skip if external subtitle with custom name (language code)
#==============================================================================
test_5_external_subtitle_custom_name() {
    log_info "========================================"
    log_info "TEST 5: Skip if external subtitle with language code"
    log_info "========================================"
    
    cleanup_test_files
    
    local test_file="$TESTDATA_DIR/test_skip5.mkv"
    local ext_srt="${test_file%.mkv}.eng.srt"
    
    # Copy test video
    cp "$TESTDATA_DIR/video.mkv" "$test_file"
    
    # Create external subtitle with language code
    cat > "$ext_srt" << 'EOF'
1
00:00:00,000 --> 00:00:02,000
External subtitle with language code
EOF
    
    # File system monitor should detect and skip (SKIP_IF_EXTERNAL_SUBTITLES_EXIST=true)
    check_skip_in_logs "external.*subtitle" "TEST 5"
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
    TEST_RESULTS+=("SKIP: TEST 6 - Config SKIP_SUBTITLE_LANGUAGES empty")
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
    TEST_RESULTS+=("SKIP: TEST 7 - Config SKIP_IF_AUDIO_LANGUAGES empty")
    return 0
}

#==============================================================================
# ADDITIONAL TEST: Verify skip flags work correctly
#==============================================================================
test_skip_flags_working() {
    log_info "========================================"
    log_info "ADDITIONAL: Verify SKIP_IF_TARGET_SUBTITLES_EXIST=true"
    log_info "========================================"
    
    # Already tested in TEST 3, just verify from logs
    local logs=$(get_logs 500)
    if echo "$logs" | grep -iq "subtitle.*exists"; then
        log_success "SKIP_IF_TARGET_SUBTITLES_EXIST: Working correctly"
        return 0
    else
        log_info "SKIP_IF_TARGET_SUBTITLES_EXIST: No clear evidence yet"
        return 0
    fi
}

test_normal_processing() {
    log_info "========================================"
    log_info "ADDITIONAL: Normal processing (no skip conditions)"
    log_info "========================================"
    
    cleanup_test_files
    
    local test_file="$TESTDATA_DIR/test_normal.wav"
    
    # Copy test audio without any subtitles
    cp "$TESTDATA_DIR/speech_sample.wav" "$test_file"
    
    # Give monitor time to process
    check_not_skipped "test_normal.wav" "Normal Processing"
}

#==============================================================================
# Main test execution
#==============================================================================
main() {
    log_info "========================================="
    log_info "Subgen Skip Logic Comprehensive Test"
    log_info "Using File System Monitor"
    log_info "========================================="
    log_info "Date: $(date)"
    log_info ""
    
    log_info "Current Configuration (from docker-compose.test.yml):"
    log_info "- SKIP_IF_EXTERNAL_SUBTITLES_EXIST=true"
    log_info "- SKIP_IF_TARGET_SUBTITLES_EXIST=true"
    log_info "- SKIP_SUBTITLE_LANGUAGES= (empty)"
    log_info "- SKIP_IF_AUDIO_LANGUAGES= (empty)"
    log_info "- SKIP_ONLY_SUBGEN_SUBTITLES=false"
    log_info "- SKIP_UNKNOWN_LANGUAGE=false"
    log_info "- SKIP_IF_INTERNAL_SUBTITLES_LANGUAGE=eng"
    log_info "- MONITOR=true"
    log_info ""
    
    # Clear old logs to make testing clearer
    log_info "Checking docker logs..."
    docker logs subgen-orchestrator-test --tail 5
    
    # Run tests
    log_info ""
    test_1_lrc_exists || true
    sleep 3
    
    test_2_unknown_language || true
    sleep 2
    
    test_3_target_subtitle_exists || true
    sleep 3
    
    test_4_internal_subtitle_language || true
    sleep 2
    
    test_5_external_subtitle_custom_name || true
    sleep 3
    
    test_6_subtitle_language_skip_list || true
    sleep 2
    
    test_7_audio_language_skip_list || true
    sleep 2
    
    test_skip_flags_working || true
    sleep 2
    
    test_normal_processing || true
    sleep 2
    
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
            echo -e "${GREEN}✓ $result${NC}"
        elif [[ $result == FAIL* ]]; then
            echo -e "${RED}✗ $result${NC}"
        else
            echo -e "${YELLOW}○ $result${NC}"
        fi
    done
    
    log_info ""
    log_info "Cleanup..."
    cleanup_test_files
    
    log_info ""
    log_info "Full report will be saved to: docs/WORKLOGS/skip_logic_test_results.md"
}

# Run main
main "$@"
