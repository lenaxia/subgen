# Feature Parity Gap Analysis: Checklist vs Production Tests

**Date**: 2026-02-17  
**Analysis By**: OpenCode AI Agent  
**Documents Analyzed**:
- Feature Checklist: `docs/WORKLOGS/0064_2026-02-16_feature_parity_checklist.md`
- Test Report: `docs/WORKLOGS/0065_2026-02-17_docker_production_testing_comprehensive_report.md`

---

## Executive Summary

**Critical Finding**: The 43% "feature parity" claim in the checklist is **misleading**. While 43 tests passed in production, these tests only covered **~30% of total features** listed in the checklist. The true feature completion rate is much lower when accounting for untested features.

### Key Findings

| Metric | Count | Percentage |
|--------|-------|------------|
| **Total Features in Checklist** | 88 | 100% |
| **Features Marked "COMPLETED"** | 42 | 48% |
| **Features Actually Tested** | 43 tests | 49% coverage |
| **Features Marked "PARTIAL"** | 21 | 24% |
| **Features Marked "MISSING"** | 25 | 28% |
| **TRUE Working Features** | ~38 | **43%** ✅ |

### Gap Categories

1. **Features Claimed Complete but NOT Tested**: 8 features
2. **Features Partially Implemented**: 21 features (only 5 tested)
3. **Features Completely Missing**: 25 features (0% tested)
4. **Features Tested and Working**: 38 features

---

## Section 1: Features Claimed "COMPLETED" but NOT Actually Tested

These 8 features were marked as ✅ COMPLETED in the checklist but have **ZERO test coverage** in the production test report.

| # | Feature | Checklist Line | Why Not Tested | Impact | Complexity |
|---|---------|----------------|----------------|--------|------------|
| 1 | **Compute type selection** | Line 16 | Only tested int8, not float16/float32/auto | MEDIUM | EASY |
| 2 | **Force language override** | Line 18 | Auto-detection tested, but not force override | LOW | EASY |
| 3 | **Multiple audio track handling** | Line 20 | No multi-track test files used | MEDIUM | MEDIUM |
| 4 | **Metadata refresh (Plex/Jellyfin)** | Line 28 | Noted "not tested - requires token" | HIGH | MEDIUM |
| 5 | **Event filtering** | Line 33 | Webhooks tested but not all event types | MEDIUM | EASY |
| 6 | **Process on added media config** | Line 34 | Config disabled during test, not validated | HIGH | EASY |
| 7 | **Process on playback config** | Line 35 | Config disabled during test, not validated | MEDIUM | EASY |
| 8 | **Idle detection** | Line 42 | Not explicitly tested, only cleanup timing | LOW | EASY |

### Analysis

**Most Critical Gap**: Metadata refresh (Feature #4)
- **Why Critical**: Without this, media servers won't detect new subtitles
- **User Impact**: Manual library refresh required after every transcription
- **Recommendation**: HIGH priority to test with API tokens

**Recommended Test Plan**:
```bash
# Test 1: Compute type variations
curl -X POST "http://localhost:9000/asr?compute_type=float16" ...
curl -X POST "http://localhost:9000/asr?compute_type=float32" ...

# Test 2: Force language override
curl -X POST "http://localhost:9000/asr?language=es" ...  # Spanish audio

# Test 3: Multi-audio track video
# Use MKV with multiple audio streams, verify correct track selection

# Test 4: Metadata refresh
# Enable PLEX_TOKEN and verify library update after transcription

# Test 5: Event filtering
curl -X POST http://localhost:9000/plex \
  -F 'payload={"event":"media.play","Metadata":{"ratingKey":"123"}}'
  
# Test 6 & 7: Config toggles
# Set PROCESS_ADDED_MEDIA=true and verify task queueing
# Set PROCESS_MEDIA_ON_PLAY=true and verify playback triggers

# Test 8: Idle detection
# Queue 5 tasks, verify worker idles between batches
```

---

## Section 2: Features Marked "PARTIALLY IMPLEMENTED" - What's Actually Missing

21 features are marked as ⚠️ PARTIALLY IMPLEMENTED. Here's the breakdown of what's complete vs. missing:

### 2.1 ASR Endpoint (Lines 73-82)

| Component | Status | Tested | Gap |
|-----------|--------|--------|-----|
| File upload support | ✅ Complete | Yes | None |
| Query parameters | ✅ Complete | Yes | None |
| File size validation | ✅ Complete | Yes | None |
| Task queuing | ✅ Complete | Yes | None |
| **Blocking/synchronous response** | ❌ Missing | No | Returns immediately, not blocking |
| **Return subtitle content** | ❌ Missing | No | Returns placeholder, not actual subtitles |
| **Hash-based deduplication** | ✅ Complete | Yes | Actually working! (contradicts checklist) |
| **Multiple output formats** | ✅ Complete | Yes | VTT/TXT/TSV/JSON all tested |
| **AudioContent in gRPC** | ❌ Missing | No | Worker expects file paths only |

**Impact**: HIGH - Bazarr integration requires blocking response  
**Complexity**: MEDIUM (2-4 hours) - Needs result channel system  
**Priority**: HIGH

**What's Needed**:
```go
// orchestrator/internal/webhooks/asr.go
// Current: Returns immediately after queueing
c.JSON(fiber.Map{"message": "Processing started"})

// Required: Wait for completion
resultChan := make(chan *TaskResult)
s.queue.EnqueueWithCallback(task, resultChan)
result := <-resultChan  // Block here
return c.SendString(result.SubtitleContent)
```

### 2.2 Path Mapping (Lines 84-87)

| Component | Status | Tested | Gap |
|-----------|--------|--------|-----|
| Configuration options exist | ✅ Complete | Yes | Config validated |
| **Path translation logic** | ❌ Missing | No | Not implemented in orchestrator |
| **Applied before transcription** | ❌ Missing | No | No path_mapping() call |

**Impact**: HIGH - Docker/container deployments need path translation  
**Complexity**: EASY (30 minutes) - Simple string replacement  
**Priority**: HIGH

**Bug Found During Testing**: Path mapping was manually fixed during tests but not automated.

**What's Needed**:
```go
// orchestrator/internal/queue/path_mapper.go
func TranslatePath(hostPath string, config PathMappingConfig) string {
    if !config.Enabled {
        return hostPath
    }
    return strings.Replace(hostPath, config.From, config.To, 1)
}
```

### 2.3 Language Detection Endpoint (Lines 89-94)

| Component | Status | Tested | Gap |
|-----------|--------|--------|-----|
| DetectLanguage RPC working | ✅ Complete | Yes | Tested and working |
| Auto-detection during transcription | ✅ Complete | Yes | Working |
| Sample offset/length configurable | ✅ Complete | Yes | Tested with parameters |
| **Standalone /detect-language endpoint** | ✅ Complete | Yes | **Actually implemented!** |
| **Bypass queue for immediate results** | ✅ Complete | Yes | **Actually working!** |

**Impact**: NONE - This is actually complete!  
**Complexity**: EASY  
**Priority**: LOW  
**Status**: ✅ **Checklist is WRONG** - This feature is fully implemented and tested.

### 2.4 Model Management (Lines 96-103)

| Component | Status | Tested | Gap |
|-----------|--------|--------|-----|
| Lazy loading | ✅ Complete | Yes | Tested |
| Scheduled cleanup with delay | ✅ Complete | Yes | 5s delay verified |
| VRAM clearing | ✅ Complete | Yes | Cleanup tested |
| Garbage collection | ✅ Complete | Yes | GC confirmed |
| malloc_trim on Linux | ✅ Complete | Yes | Memory returned to OS |
| **Configurable cleanup delay** | ✅ Complete | Yes | **Bug fixed during test** |
| **Model statistics** | ❌ Missing | No | Not tracked in orchestrator |

**Impact**: LOW - Statistics are nice-to-have  
**Complexity**: EASY (1 hour) - Track load/cleanup events  
**Priority**: LOW

### 2.5 Advanced Audio Filtering (Lines 191-196)

| Component | Status | Tested | Gap |
|-----------|--------|--------|-----|
| PREFERRED_AUDIO_LANGUAGES config exists | ✅ Complete | No | Not tested |
| **LIMIT_TO_PREFERRED_AUDIO_LANGUAGE** | ❌ Missing | No | Not implemented |
| **find_language_audio_track() filtering** | ❌ Missing | No | No filtering logic |
| **Skip non-matching audio languages** | ❌ Missing | No | Processes all tracks |

**Impact**: MEDIUM - Useful for multi-language libraries  
**Complexity**: MEDIUM (2-3 hours) - FFprobe + filtering logic  
**Priority**: MEDIUM

### 2.6 Multiple Output Formats (Lines 200-209)

| Component | Status | Tested | Gap |
|-----------|--------|--------|-----|
| SRT format | ✅ Complete | Yes | Tested |
| LRC format | ✅ Complete | Yes | Tested |
| **VTT format** | ✅ Complete | Yes | **Actually working!** |
| **TXT format** | ✅ Complete | Yes | **Actually working!** |
| **TSV format** | ✅ Complete | Yes | **Actually working!** |
| **JSON format** | ✅ Complete | Yes | **Actually working!** |

**Impact**: NONE - All formats working!  
**Priority**: LOW  
**Status**: ✅ **Checklist is WRONG** - All 6 formats are fully implemented and tested.

### 2.7 Additional Whisper Options (Lines 212-223)

| Component | Status | Tested | Gap |
|-----------|--------|--------|-----|
| Model selection | ✅ Complete | Yes | tiny tested |
| Device selection | ✅ Complete | Yes | CPU tested |
| CPU threads | ✅ Complete | No | Not tested |
| Compute type | ✅ Complete | Partial | Only int8 tested |
| **CUSTOM_REGROUP** | ❌ Missing | No | stable-ts not integrated |
| **SUBGEN_KWARGS** | ❌ Missing | No | Not implemented |
| **Word-level highlighting** | ❌ Missing | No | Not tested |
| **Custom prompt support** | ❌ Missing | No | Config exists but not used |

**Impact**: LOW - Advanced features for power users  
**Complexity**: MEDIUM (3-4 hours each)  
**Priority**: LOW

### 2.8 Advanced Model Lifecycle (Lines 249-255)

| Component | Status | Tested | Gap |
|-----------|--------|--------|-----|
| Scheduled cleanup | ✅ Complete | Yes | Tested |
| VRAM clearing | ✅ Complete | Yes | Tested |
| **Idle detection before cleanup** | ❌ Missing | No | Cleans after fixed delay |
| **delete_model() only when queue idle** | ❌ Missing | No | No queue state check |

**Impact**: MEDIUM - Prevents unnecessary reloads during batch processing  
**Complexity**: MEDIUM (2 hours) - Check queue state before cleanup  
**Priority**: MEDIUM

### 2.9 Subtitle Language Naming Flexibility (Lines 276-284)

| Component | Status | Tested | Gap |
|-----------|--------|--------|-----|
| ISO 639-2 B format (default) | ✅ Complete | Yes | Tested |
| **5 naming formats** | ❌ Missing | No | Only one format supported |
| **Custom override via NAMESUBLANG** | ❌ Missing | No | Not implemented |
| **Auto-English for translations** | ❌ Missing | No | Not implemented |

**Impact**: LOW - Customization feature  
**Complexity**: EASY (1-2 hours) - Config-based filename generation  
**Priority**: LOW

### 2.10 Audio Segment Extraction (Lines 286-294)

| Component | Status | Tested | Gap |
|-----------|--------|--------|-----|
| Works for file paths | ✅ Complete | Yes | Tested |
| **UploadFile support** | ❌ Missing | No | Not for uploaded audio |
| **In-memory extraction** | ❌ Missing | No | Only from files |
| **Configurable format** | ❌ Missing | No | Hardcoded WAV 16kHz mono |

**Impact**: LOW - Optimization for language detection  
**Complexity**: MEDIUM (2-3 hours) - In-memory audio processing  
**Priority**: LOW

---

## Section 3: Features Completely MISSING - Zero Implementation

25 features from the checklist have **0% implementation**. Prioritized by impact:

### 3.1 HIGH IMPACT - Production Blockers

| # | Feature | Checklist Lines | Impact | Complexity | Est. Time |
|---|---------|-----------------|--------|------------|-----------|
| 1 | **Skip Logic System** | 109-138 | HIGH | HARD | 2-3 days |
| 2 | **File System Monitoring** | 142-153 | HIGH | HARD | 1-2 days |
| 3 | **External Subtitle Search** | 258-266 | MEDIUM | MEDIUM | 4-6 hours |

#### 1. Skip Logic System (Lines 109-138)
**Why Missing**: 0% implemented, requires significant architecture changes

**7 Skip Conditions Missing**:
1. Skip if audio file has existing LRC
2. Skip if unknown language
3. Skip if target subtitle already exists (internal or external)
4. Skip if internal subtitle in specific language
5. Skip if external subtitle with custom name
6. Skip if subtitle in skip language list
7. Skip if audio track in skip language list

**8 Configuration Options Missing**:
- SKIP_IF_EXTERNAL_SUBTITLES_EXIST
- SKIP_IF_TARGET_SUBTITLES_EXIST (default: True!)
- SKIP_IF_INTERNAL_SUBTITLES_LANGUAGE
- SKIP_SUBTITLE_LANGUAGES (pipe-separated)
- SKIP_IF_AUDIO_LANGUAGES (pipe-separated)
- SKIP_ONLY_SUBGEN_SUBTITLES
- SKIP_IF_NO_LANGUAGE_BUT_SUBTITLES_EXIST
- SKIP_UNKNOWN_LANGUAGE

**Functions Not Implemented**:
- should_skip_file(file_path, target_language)
- has_subtitle_language(video_file, target_language)
- has_subtitle_language_in_file(video_file, target_language) - Uses pyav
- has_subtitle_of_language_in_folder(video_file, target_language, recursion, only_skip_if_subgen)
- get_subtitle_languages(video_path) - Extracts embedded subtitle tracks
- get_audio_languages(video_path) - Extracts audio track languages
- is_valid_subtitle_language(subtitle_parts, target_language)

**Why Critical**: 
- Original script skips 90%+ of files in production (already have subtitles)
- Without skip logic, every webhook triggers transcription
- Wastes CPU/GPU resources
- Creates duplicate subtitle files

**Estimated Effort**: 2-3 days (HARD)

#### 2. File System Monitoring (Lines 142-153)
**Why Missing**: Requires watchdog integration, stability checks

**8 Components Missing**:
- Watchdog integration (PollingObserver)
- MONITOR environment variable
- TRANSCRIBE_FOLDERS (pipe-separated directories)
- NewFileHandler class for file events
- File stability checking (wait for upload completion)
- Startup folder scanning with transcribe_existing()
- Recursive directory watching
- 3-check stability algorithm with 2-second intervals

**Why Important**:
- Fully automated workflow (no webhooks needed)
- Process entire libraries on startup
- Watch folders for new files
- Self-contained operation mode

**Estimated Effort**: 1-2 days (HARD)

#### 3. External Subtitle Search (Lines 258-266)
**Why Missing**: Part of skip logic, not implemented

**Components Missing**:
- Folder scanning for .srt, .vtt, etc.
- Recursive search option
- Filename pattern matching
- Subgen-specific filtering
- 11 subtitle format support

**Why Important**: Essential for comprehensive skip logic

**Estimated Effort**: 4-6 hours (MEDIUM)

### 3.2 MEDIUM IMPACT - Convenience Features

| # | Feature | Checklist Lines | Impact | Complexity | Est. Time |
|---|---------|-----------------|--------|------------|-----------|
| 4 | **Plex Episode Queueing** | 156-165 | MEDIUM | MEDIUM | 1 day |
| 5 | **Batch Processing Endpoint** | 168-176 | MEDIUM | EASY | 2-3 hours |
| 6 | **Advanced Skip Options** | 226-234 | MEDIUM | MEDIUM | 4-6 hours |
| 7 | **Task Result System** | 296-305 | MEDIUM | MEDIUM | 4-6 hours |

#### 4. Plex Episode Queueing (Lines 156-165)
**3 Features Missing**:
- PLEX_QUEUE_NEXT_EPISODE - Auto-queue next episode
- PLEX_QUEUE_SEASON - Auto-queue entire season
- PLEX_QUEUE_SERIES - Auto-queue entire series
- get_next_plex_episode() function with XML navigation
- Season boundary detection
- Error handling at series end

**Estimated Effort**: 1 day (MEDIUM)

#### 5. Batch Processing Endpoint (Lines 168-176)
**Why Missing**: Scanner not initialized (noted in test report)

**Components Missing**:
- /batch POST endpoint
- Directory parameter (query string)
- Optional force language
- Recursive folder processing
- Returns count of queued files

**Why Important**: Bulk operations for entire directories

**Estimated Effort**: 2-3 hours (EASY)

#### 6. Advanced Skip Options (Lines 226-234)
**4 Features Missing**:
- Only skip subgen-generated subtitles (ignore others)
- Skip recursive folder search for external subs
- Custom subtitle language name matching
- Multiple language code formats in skip lists

**Estimated Effort**: 4-6 hours (MEDIUM)

#### 7. Task Result System (Lines 296-305)
**Why Missing**: No result storage or event signaling

**Components Missing**:
- TaskResult class for result storage
- Thread-safe event signaling
- Timeout support
- Error handling
- Used by ASR to return results synchronously

**Why Important**: Required for blocking ASR endpoint

**Estimated Effort**: 4-6 hours (MEDIUM)

### 3.3 LOW IMPACT - Nice to Have

| # | Feature | Checklist Lines | Impact | Complexity | Est. Time |
|---|---------|-----------------|--------|------------|-----------|
| 8 | **Progress Reporting** | 236-245 | LOW | MEDIUM | 3-4 hours |
| 9 | **Audio Hash Deduplication** | 307-317 | LOW | EASY | 2 hours |
| 10 | **Hot Reload** | 269-274 | LOW | EASY | 1 hour |

#### 8. Progress Reporting (Lines 236-245)
**6 Features Missing**:
- Real-time progress display
- Percentage complete
- Time estimates (ETA)
- Processing speed
- Queue status display
- ProgressHandler class

**Estimated Effort**: 3-4 hours (MEDIUM)

#### 9. Audio Hash Deduplication (Lines 307-317)
**Note**: Test report shows hash-based deduplication **IS WORKING** for ASR endpoint. Checklist may be outdated.

**Components Claimed Missing**:
- generate_audio_hash() function
- SHA256 hashing of audio content
- Includes task type and language
- 16-character shortened IDs
- Result sharing for identical requests

**Status**: ⚠️ **Needs verification** - Test report contradicts checklist

**Estimated Effort**: 2 hours (EASY) if truly missing

#### 10. Hot Reload (Lines 269-274)
**2 Features Missing**:
- RELOAD_SCRIPT_ON_CHANGE
- Uvicorn auto-reload integration

**Why Low Priority**: Development convenience only

**Estimated Effort**: 1 hour (EASY)

---

## Section 4: TRUE Feature Parity Calculation

### Methodology

**Total Features Counted**: 88 discrete features from checklist

**Categories**:
1. Core Transcription: 9 features
2. Media Server Integrations: 4 features
3. Webhook Server: 5 features
4. Queue System: 5 features
5. Language Support: 4 features
6. Output Formats: 6 features
7. Docker Support: 5 features
8. System Features: 4 features
9. ASR Endpoint: 9 features
10. Path Mapping: 2 features
11. Language Detection: 5 features
12. Model Management: 6 features
13. Skip Logic System: 8 features
14. File Monitoring: 8 features
15. Plex Episode Queueing: 3 features
16. Batch Endpoint: 1 feature
17. Additional Features: 4 features

### Feature Status Breakdown

| Status | Count | Percentage | Tested |
|--------|-------|------------|--------|
| **✅ Fully Working** | 38 | 43% | Yes (43 tests) |
| **⚠️ Partially Working** | 10 | 11% | Partial (5 tested) |
| **❌ Completely Missing** | 25 | 28% | No (0 tests) |
| **📝 Claimed but Untested** | 8 | 9% | No (0 tests) |
| **❓ Status Unknown** | 7 | 8% | No (0 tests) |
| **TOTAL** | **88** | **100%** | 48 tested |

### True Feature Parity Metrics

**Working Features**: 38 fully working + 5 partially tested = **43 working features**

**Total Features**: 88

**TRUE FEATURE PARITY**: **43 / 88 = 48.9%** ✅

**Test Coverage**: 48 features tested / 88 total features = **54.5%**

### Comparison to Checklist Claims

| Metric | Checklist Claim | Reality | Difference |
|--------|----------------|---------|------------|
| "COMPLETED" Features | 42 (48%) | 38 (43%) | -4 features |
| "PARTIAL" Features | 21 (24%) | 10 (11%) | -11 features |
| "MISSING" Features | 25 (28%) | 40 (45%) | +15 features |
| Overall Parity | 43% | 49% | +6% (more accurate) |

### Key Insight

The checklist's 43% completion metric is **approximately correct** when properly calculated, but:
1. It undercounts working features (Language Detection and Output Formats work better than claimed)
2. It overcounts partial implementations (many "partial" features have zero implementation)
3. Test coverage (54.5%) is decent but leaves gaps in critical features

---

## Section 5: Top 5 Most Critical Gaps to Address

Ranked by **Impact × Urgency × User Demand**:

### 1. Skip Logic System ⚠️ CRITICAL
**Impact**: HIGH  
**Complexity**: HARD  
**Estimated Time**: 2-3 days  
**Priority**: P0 (Must Have)

**Why #1**:
- Without this, system will re-process 90%+ of files unnecessarily
- Wastes compute resources (CPU/GPU)
- Creates duplicate subtitle files
- Makes system unusable for production libraries

**What's Needed**:
- Implement 7 skip conditions
- Add 8 configuration options
- Create skip checker service in Go
- FFprobe integration for embedded subtitles
- Folder scanning for external subtitles

**Recommended Approach**:
1. **Phase 1** (4 hours): Basic skip - check if subtitle file exists
2. **Phase 2** (1 day): External subtitle search with folder scanning
3. **Phase 3** (1 day): Internal subtitle detection with FFprobe/pyav

### 2. Metadata Refresh (Plex/Jellyfin) ⚠️ HIGH
**Impact**: HIGH  
**Complexity**: MEDIUM  
**Estimated Time**: 4-6 hours  
**Priority**: P0 (Must Have)

**Why #2**:
- Feature marked as "complete" but not tested
- Without this, users must manually refresh libraries
- Breaks automated workflow promise
- Simple to fix with API tokens

**What's Needed**:
- Test with PLEX_TOKEN and JELLYFIN_API_KEY
- Verify library refresh API calls
- Add retry logic for refresh failures
- Document token requirements

**Recommended Approach**:
```go
// After transcription success
if s.plexClient != nil {
    if err := s.plexClient.RefreshMetadata(ctx, ratingKey); err != nil {
        log.Error("Failed to refresh Plex metadata", "error", err)
    }
}
```

### 3. ASR Blocking Response ⚠️ HIGH
**Impact**: HIGH  
**Complexity**: MEDIUM  
**Estimated Time**: 4-6 hours  
**Priority**: P0 (Must Have)

**Why #3**:
- Required for Bazarr integration
- Currently returns immediately (placeholder)
- Bazarr needs actual subtitle content
- Breaks promise of "Bazarr support"

**What's Needed**:
- Implement result channel system
- Add timeout support (default 5 minutes)
- Return actual subtitle content
- Handle worker failures gracefully

**Recommended Approach**:
```go
// orchestrator/internal/webhooks/asr.go
resultChan := make(chan *TaskResult, 1)
s.queue.EnqueueWithCallback(task, resultChan)

select {
case result := <-resultChan:
    return c.SendString(result.SubtitleContent)
case <-time.After(5 * time.Minute):
    return c.Status(504).SendString("Timeout")
}
```

### 4. Path Mapping Logic ⚠️ HIGH
**Impact**: HIGH  
**Complexity**: EASY  
**Estimated Time**: 30 minutes  
**Priority**: P1 (Should Have)

**Why #4**:
- Required for Docker/container deployments
- Configuration exists but not implemented
- Bug was manually fixed during testing
- Simple string replacement

**What's Needed**:
```go
// orchestrator/internal/queue/path_mapper.go
func TranslatePath(hostPath string, config PathMappingConfig) string {
    if !config.UsePathMapping {
        return hostPath
    }
    return strings.Replace(hostPath, config.PathMappingFrom, config.PathMappingTo, 1)
}
```

**Recommended Approach**:
1. Create path_mapper.go service
2. Apply in queue before worker RPC call
3. Add unit tests for translation
4. Document in README

### 5. File System Monitoring ⚠️ MEDIUM-HIGH
**Impact**: HIGH (for automated workflows)  
**Complexity**: HARD  
**Estimated Time**: 1-2 days  
**Priority**: P1 (Should Have)

**Why #5**:
- Enables fully automated library processing
- No webhooks needed
- Watches folders for new files
- Self-contained operation mode

**What's Needed**:
- Watchdog/fsnotify integration
- File stability checks (wait for complete upload)
- Recursive directory scanning
- Integration with queue system

**Recommended Approach**:
1. Use fsnotify (Go native) instead of Python watchdog
2. Implement 3-check stability (2s intervals)
3. Scan existing files on startup
4. Add MONITOR and TRANSCRIBE_FOLDERS configs

---

## Section 6: Recommended Testing Plan

### 6.1 Test Coverage Gaps

**Current Coverage**: 48/88 features tested (54.5%)

**Untested Features**: 40 features (45.5%)

**Breakdown by Priority**:
- HIGH priority untested: 8 features
- MEDIUM priority untested: 12 features
- LOW priority untested: 20 features

### 6.2 Test Plan by Phase

#### Phase 1: Critical Feature Testing (HIGH Priority)

**Goal**: Test all features marked "COMPLETED" to verify claims

**Duration**: 4-6 hours

**Test Cases**:

1. **Compute Type Selection**
   ```bash
   # Test int8 (default)
   curl -X POST "http://localhost:9000/asr?compute_type=int8" -F "audio_file=@test.wav"
   
   # Test float16 (faster)
   curl -X POST "http://localhost:9000/asr?compute_type=float16" -F "audio_file=@test.wav"
   
   # Test float32 (most accurate)
   curl -X POST "http://localhost:9000/asr?compute_type=float32" -F "audio_file=@test.wav"
   
   # Test auto (let Whisper decide)
   curl -X POST "http://localhost:9000/asr?compute_type=auto" -F "audio_file=@test.wav"
   ```

2. **Force Language Override**
   ```bash
   # Spanish audio forced to Spanish
   curl -X POST "http://localhost:9000/asr?language=es&task=transcribe" -F "audio_file=@spanish.wav"
   
   # Spanish audio translated to English
   curl -X POST "http://localhost:9000/asr?language=es&task=translate" -F "audio_file=@spanish.wav"
   ```

3. **Multiple Audio Track Handling**
   ```bash
   # MKV with English + Japanese audio tracks
   # Expected: Use PREFERRED_AUDIO_LANGUAGES config
   curl -X POST http://localhost:9000/tautulli \
     -H "source: Tautulli" \
     -d "event=played&file=/testdata/multi_audio.mkv"
   ```

4. **Metadata Refresh**
   ```bash
   # Set Plex token in docker-compose
   PLEX_TOKEN=your_token_here
   
   # Trigger transcription
   curl -X POST http://localhost:9000/plex \
     -H "User-Agent: PlexMediaServer/1.40.0" \
     -F 'payload={"event":"library.new","Metadata":{"ratingKey":"12345"}}'
   
   # Verify: Check Plex library for new subtitle
   # Verify: Check orchestrator logs for "Refreshing Plex metadata"
   ```

5. **Event Filtering**
   ```bash
   # Test library.new (should process)
   curl -X POST http://localhost:9000/plex \
     -F 'payload={"event":"library.new","Metadata":{"ratingKey":"123"}}'
   
   # Test media.play (should process if PROCESS_MEDIA_ON_PLAY=true)
   curl -X POST http://localhost:9000/plex \
     -F 'payload={"event":"media.play","Metadata":{"ratingKey":"123"}}'
   
   # Test media.pause (should ignore)
   curl -X POST http://localhost:9000/plex \
     -F 'payload={"event":"media.pause","Metadata":{"ratingKey":"123"}}'
   ```

6. **Process on Added Media Config**
   ```bash
   # Test with PROCESS_ADDED_MEDIA=true
   docker compose -f docker-compose.test.yml down
   # Edit docker-compose: PROCESS_ADDED_MEDIA=true
   docker compose -f docker-compose.test.yml up -d
   
   # Send library.new webhook
   curl -X POST http://localhost:9000/plex \
     -F 'payload={"event":"library.new","Metadata":{"ratingKey":"123"}}'
   
   # Expected: Task queued
   # Verify: Check logs for "Queued task"
   ```

7. **Process on Playback Config**
   ```bash
   # Test with PROCESS_MEDIA_ON_PLAY=false
   docker compose -f docker-compose.test.yml down
   # Edit docker-compose: PROCESS_MEDIA_ON_PLAY=false
   docker compose -f docker-compose.test.yml up -d
   
   # Send media.play webhook
   curl -X POST http://localhost:9000/plex \
     -F 'payload={"event":"media.play","Metadata":{"ratingKey":"123"}}'
   
   # Expected: No task queued
   # Verify: Check logs for "Playback processing disabled"
   ```

8. **Idle Detection**
   ```bash
   # Queue 5 tasks rapidly
   for i in {1..5}; do
     curl -X POST "http://localhost:9000/asr?task=transcribe" \
       -F "audio_file=@test.wav"
   done
   
   # Expected: Worker processes with CONCURRENT_TRANSCRIPTIONS=2
   # Verify: Logs show idle state between batches
   ```

#### Phase 2: Partial Feature Validation (MEDIUM Priority)

**Goal**: Verify what's working in "partial" features and document gaps

**Duration**: 2-3 hours

**Test Cases**:

9. **ASR Blocking Response**
   ```bash
   # Current behavior (non-blocking)
   time curl -X POST "http://localhost:9000/asr?task=transcribe" \
     -F "audio_file=@test.wav"
   # Expected: Returns immediately (~50ms)
   # Actual: Should wait for transcription (~4s)
   
   # TODO: Implement blocking with result channel
   ```

10. **Path Mapping**
    ```bash
    # Test with path mapping enabled
    USE_PATH_MAPPING=true
    PATH_MAPPING_FROM=/host/media
    PATH_MAPPING_TO=/media
    
    # Send file with host path
    curl -X POST http://localhost:9000/tautulli \
      -H "source: Tautulli" \
      -d "event=played&file=/host/media/test.mkv"
    
    # Expected: Worker receives /media/test.mkv
    # Verify: Check worker logs for translated path
    ```

11. **Audio Language Filtering**
    ```bash
    # Test with LIMIT_TO_PREFERRED_AUDIO_LANGUAGE=true
    PREFERRED_AUDIO_LANGUAGES=eng|jpn
    LIMIT_TO_PREFERRED_AUDIO_LANGUAGE=true
    
    # Send multi-audio video (eng + spa tracks)
    # Expected: Only process English track (skip Spanish)
    ```

12. **Model Statistics Tracking**
    ```bash
    # Check metrics for model stats
    curl http://localhost:9090/metrics | grep model
    
    # Expected metrics:
    # subgen_model_loads_total
    # subgen_model_cleanups_total
    # subgen_model_load_duration_seconds
    ```

#### Phase 3: Integration Testing (HIGH Priority)

**Goal**: Test real-world scenarios end-to-end

**Duration**: 4-6 hours

**Test Cases**:

13. **Plex Full Workflow**
    ```bash
    # 1. Add new video to Plex library
    # 2. Plex sends webhook to orchestrator
    # 3. Orchestrator queues transcription
    # 4. Worker transcribes and saves subtitle
    # 5. Orchestrator refreshes Plex metadata
    # 6. Plex detects new subtitle
    
    # Verify each step in logs
    ```

14. **Jellyfin Full Workflow**
    ```bash
    # Same as Plex but with Jellyfin
    # Use JELLYFIN_API_KEY for metadata refresh
    ```

15. **Bazarr ASR Integration**
    ```bash
    # 1. Configure Bazarr to use subgen ASR endpoint
    # 2. Request subtitles via Bazarr UI
    # 3. Bazarr sends POST to /asr
    # 4. Subgen processes and returns subtitles
    # 5. Bazarr saves subtitle file
    
    # Current issue: ASR returns immediately (placeholder)
    # TODO: Implement blocking response first
    ```

16. **Concurrent Load Test**
    ```bash
    # Send 10 simultaneous webhook requests
    for i in {1..10}; do
      curl -X POST http://localhost:9000/tautulli \
        -H "source: Tautulli" \
        -d "event=played&file=/testdata/video_$i.mkv" &
    done
    wait
    
    # Expected: Queue handles all 10, processes with concurrency limit
    # Verify: Check metrics for queue_size and processing_time
    ```

#### Phase 4: Missing Feature Implementation Testing (As Implemented)

**Goal**: Test features as they are implemented

**Duration**: Ongoing

**Test Cases** (TBD based on implementation):

17. **Skip Logic System**
    - Test all 7 skip conditions
    - Test 8 configuration options
    - Test embedded subtitle detection
    - Test external subtitle search

18. **File System Monitoring**
    - Test folder watching
    - Test file stability checks
    - Test recursive scanning
    - Test startup folder processing

19. **Batch Processing Endpoint**
    - Test /batch with directory parameter
    - Test recursive folder processing
    - Test force_language parameter
    - Test response with queued file count

20. **Plex Episode Queueing**
    - Test PLEX_QUEUE_NEXT_EPISODE
    - Test PLEX_QUEUE_SEASON
    - Test PLEX_QUEUE_SERIES
    - Test season boundary detection

### 6.3 Automated Test Suite

**Recommendation**: Create automated integration tests

**Test Framework**: Go testing + testify

**Test Files to Create**:
```
test/integration/
├── health_test.go           # Health checks
├── transcription_test.go    # Core transcription
├── language_test.go         # Language detection
├── formats_test.go          # Output formats
├── webhooks_test.go         # Media server webhooks
├── asr_test.go              # ASR endpoint
├── queue_test.go            # Queue system
├── model_test.go            # Model lifecycle
├── skip_test.go             # Skip logic (when implemented)
└── monitor_test.go          # File monitoring (when implemented)
```

**CI/CD Integration**:
```yaml
# .github/workflows/integration-tests.yml
name: Integration Tests

on: [push, pull_request]

jobs:
  test:
    runs-on: ubuntu-latest
    steps:
      - uses: actions/checkout@v3
      
      - name: Start Docker containers
        run: docker compose -f docker-compose.test.yml up -d
      
      - name: Wait for health checks
        run: |
          timeout 60 bash -c 'until curl -sf http://localhost:9000/health; do sleep 1; done'
      
      - name: Run integration tests
        run: go test -v ./test/integration/...
      
      - name: Stop containers
        run: docker compose -f docker-compose.test.yml down
```

### 6.4 Test Coverage Goals

**Current Coverage**: 54.5% (48/88 features)

**Phase 1 Goal**: 70% (62/88 features) - Test all "COMPLETED" features

**Phase 2 Goal**: 80% (70/88 features) - Validate all "PARTIAL" features

**Phase 3 Goal**: 90% (79/88 features) - Integration testing

**Phase 4 Goal**: 100% (88/88 features) - Full feature parity

### 6.5 Testing Timeline

| Phase | Duration | Completion Date | Coverage Goal |
|-------|----------|-----------------|---------------|
| Phase 1: Critical Features | 4-6 hours | 2026-02-18 | 70% |
| Phase 2: Partial Features | 2-3 hours | 2026-02-18 | 75% |
| Phase 3: Integration Tests | 4-6 hours | 2026-02-19 | 80% |
| Phase 4: Missing Features | 2-3 weeks | 2026-03-10 | 100% |

---

## Section 7: Summary and Recommendations

### Key Findings

1. **Checklist Accuracy**: The 43% feature parity claim is approximately correct when properly calculated (TRUE: 49%)

2. **Test Coverage Gap**: 45.5% of features have ZERO test coverage, including 8 features claimed as "COMPLETED"

3. **Critical Gaps**: 5 high-impact features are blocking production use:
   - Skip Logic System (0% implemented)
   - Metadata Refresh (not tested)
   - ASR Blocking Response (returns placeholder)
   - Path Mapping (not implemented)
   - File System Monitoring (0% implemented)

4. **Misleading Claims**: Several "PARTIAL" features are actually complete:
   - Language Detection endpoint (fully working)
   - Multiple output formats (all 6 working)
   - Hash-based deduplication (working)

5. **Positive Surprise**: Some features work better than documented (VTT, TXT, TSV, JSON formats all working)

### Recommendations

#### Immediate Actions (This Week)

1. **Test Metadata Refresh** with real API tokens (4-6 hours)
   - HIGH impact, EASY to verify
   - Blocking automated workflow promise

2. **Implement Path Mapping Logic** (30 minutes)
   - HIGH impact, EASY implementation
   - Required for Docker deployments

3. **Run Phase 1 Test Suite** to verify all "COMPLETED" claims (4-6 hours)
   - Validate or invalidate 8 untested features
   - Update checklist with accurate status

#### Short-Term (Next 2 Weeks)

4. **Implement ASR Blocking Response** (4-6 hours)
   - HIGH impact, MEDIUM complexity
   - Required for Bazarr integration

5. **Implement Basic Skip Logic** (4 hours)
   - HIGH impact, MEDIUM complexity
   - Check if subtitle file exists (Phase 1 only)

6. **Create Automated Test Suite** (1-2 days)
   - Prevent regression
   - Enable CI/CD confidence

#### Medium-Term (Next Month)

7. **Implement Full Skip Logic System** (2-3 days)
   - HIGH impact, HARD complexity
   - All 7 skip conditions + 8 configs

8. **Implement File System Monitoring** (1-2 days)
   - HIGH impact, HARD complexity
   - Enables fully automated workflows

9. **Implement Batch Processing Endpoint** (2-3 hours)
   - MEDIUM impact, EASY complexity
   - Bulk operations

#### Long-Term (Next Quarter)

10. **Implement Plex Episode Queueing** (1 day)
    - MEDIUM impact, MEDIUM complexity
    - TV show automation

11. **Implement Progress Reporting** (3-4 hours)
    - LOW impact, MEDIUM complexity
    - User experience improvement

12. **Full Feature Parity** (remaining features)
    - Target: 100% feature parity by 2026-Q2

### Priority Matrix

```
HIGH IMPACT, EASY
├── Metadata Refresh (test only) ⭐ DO FIRST
├── Path Mapping (30 min impl) ⭐ DO FIRST
└── Phase 1 Testing (verify claims) ⭐ DO FIRST

HIGH IMPACT, MEDIUM
├── ASR Blocking Response (4-6 hours) ⭐⭐ DO SECOND
├── Basic Skip Logic (4 hours) ⭐⭐ DO SECOND
└── External Subtitle Search (4-6 hours)

HIGH IMPACT, HARD
├── Full Skip Logic System (2-3 days) ⭐⭐⭐ DO THIRD
└── File System Monitoring (1-2 days) ⭐⭐⭐ DO THIRD

MEDIUM IMPACT, EASY
├── Batch Endpoint (2-3 hours)
└── Event Filtering Tests (1 hour)

MEDIUM IMPACT, MEDIUM
├── Plex Episode Queueing (1 day)
├── Task Result System (4-6 hours)
└── Advanced Skip Options (4-6 hours)

LOW IMPACT
└── All remaining features (nice-to-have)
```

### Final Assessment

**Production Readiness**: 
- ✅ Ready for webhook-based workflows (no skip logic)
- ⚠️ NOT ready for production libraries (will re-process everything)
- ⚠️ NOT ready for Bazarr (ASR not blocking)
- ❌ NOT ready for automated monitoring (no file watching)

**True Feature Parity**: **49%** (43/88 features fully working)

**Test Coverage**: **54.5%** (48/88 features tested)

**Recommended Action**: Focus on **Top 5 Critical Gaps** before production deployment.

**Estimated Time to Production-Ready**: 1-2 weeks (implement critical gaps + testing)

**Estimated Time to Full Feature Parity**: 2-3 months (all 88 features)

---

**END OF GAP ANALYSIS**

