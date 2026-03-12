# Work Log: Skip Logic Moved to Worker
## Date: February 23, 2026
## Epic: Production Readiness & Validation

---

## Summary

Moved all skip logic from the orchestrator to the worker, following the architecture decision that skip logic should be in the worker since it has NFS access and owns the transcription concern. This change simplifies the orchestrator and centralizes skip decision-making.

---

## Changes Made

### 1. Orchestrator - Skip Logic Removal

#### Removed from Orchestrator
- **internal/skip package**: All skip logic code removed from orchestrator
- **webhooks/server.go**: Removed skip checker integration from all webhook handlers
  - Plex webhook: Removed commented-out skip check (lines 330-354)
  - Emby webhook: Removed skip check (lines 609-627)
  - Tautulli webhook: Removed skip check (lines 708-726)
- **monitor/scanner.go**: Removed skip checker from directory scanner
  - Removed `skipChecker skip.Checker` field from BasicScanner
  - Removed skip logic from `ScanDirectory()` method
  - Updated `NewScanner()` and `NewScannerWithLogger()` signatures
- **cmd/orchestrator/main.go**: Removed skip package import and initialization
  - Removed skip checker creation
  - Removed skip check from file watcher callback
  - Updated scanner initialization

#### Test Updates
- **internal/monitor/scanner_test.go**: Updated to remove skip checker
  - Removed `MockSkipChecker` type
  - Updated all test functions to use new scanner signature
  - Removed skip-specific test: `TestScanner_ScanDirectory_SkipLogicIntegration`
  - Removed skip-specific test: `TestScanner_ScanDirectory_SkipReasonTracking`
- **internal/webhooks/batch_integration_test.go**: Updated to remove skip checker
  - Removed `mockSkipChecker` type
  - Removed skip-specific test: `TestBatchEndpointWithSkipLogic`
  - Updated scanner initialization calls

### 2. Worker - Comprehensive Skip Logic Implementation

#### New Modules Created

**subtitles/detector.py**: Subtitle detection utilities
- `scan_external_subtitles()`: Scans for external subtitle files (SRT/LRC)
- `get_embedded_subtitles()`: Detects embedded subtitles using FFmpeg probe
- `has_subtitle_language()`: Checks if subtitles match target language
- `is_audio_file()` / `is_video_file()`: File type detection
- `is_subgen_subtitle()`: Identifies subgen-generated subtitles
- `ExternalSubtitle` dataclass: External subtitle metadata
- `EmbeddedSubtitle` dataclass: Embedded subtitle metadata

**subtitles/skip_checker.py**: Comprehensive skip logic implementation
- `SkipReason` enum: All skip reasons (subtitle_exists, embedded_subtitle, external_subtitle, audio_language_skip, etc.)
- `SkipResult` dataclass: Skip decision with reason and details
- `SkipChecker` class: Main skip logic implementation
  - `check()`: Main entry point for skip decision
  - `_check_target_subtitles_exist()`: Checks SRT/LRC files
  - `_check_embedded_subtitles()`: Checks embedded subtitles in target language
  - `_check_external_subtitles()`: Checks external subtitles
  - `_check_audio_language_skip()`: Checks if audio language is in skip list
  - `_check_preferred_audio_language()`: Checks if audio matches preferred languages

#### Worker Configuration Updates

**config/settings.py - SkipConfig**: Added missing configuration fields
- `check_embedded_subtitles`: Enable/disable embedded subtitle checking
- `skip_if_internal_subtitles_language`: Target language for embedded subtitle skip
- `preferred_audio_languages`: Preferred audio language list
- `limit_to_preferred_audio_language`: Only process preferred audio languages
- `get_preferred_audio_languages()`: Helper method to get preferred languages as list

#### Worker Service Updates

**grpc_server/service.py - TranscriptionServicer**:
- Added import for `SkipChecker` and `SkipReason`
- Added `skip_checker` field initialization in `__init__()`
- Updated `Transcribe()` method to use comprehensive skip check
  - Calls `skip_checker.check(file_path)` before loading model
  - Returns existing subtitle path for subtitle_exists reasons
  - Returns success with empty subtitle for other skip reasons
  - Logs skip reason and details

### 3. Worker Tests

**tests/unit/test_skip_logic.py**: Comprehensive unit tests
- `TestSkipCheckerTargetSubtitles`: Target subtitle existence checks
  - Subgen SRT/LRC files
  - Regular SRT/LRC files
  - No subtitles scenario
- `TestSkipCheckerEmbeddedSubtitles`: Embedded subtitle checks
  - Matching target language
- `TestSkipCheckerExternalSubtitles`: External subtitle checks
  - External subtitle existence
  - Subgen-only filtering
- `TestSkipCheckerDisabled`: Skip logic when disabled
  - Config override behavior
- `TestSkipCheckerFileTypes`: Different file type handling
  - Video files (SRT checks)
  - Audio files (LRC checks)
  - Unsupported files
- `TestSkipCheckerLanguageFiltering`: Language-based filtering
  - Audio language skip list
  - Preferred audio language matching

**tests/integration/test_skip_e2e.py**: End-to-end integration tests
- `TestSkipLogicE2E`: Full integration with gRPC service
  - Skip with existing subgen subtitle
  - Skip with existing regular subtitle
  - Skip with existing LRC for audio
  - No skip when subtitles don't exist
  - Multiple language subtitles
  - External subtitle detection
  - Config override behavior
  - File not found error handling
  - Audio content bypasses skip logic

---

## Architecture Decisions

### Why Skip Logic Belongs in the Worker

1. **NFS Access**: The worker has direct access to the NFS share where media files live
2. **Ownership**: The worker owns the transcription concern
3. **Simplicity**: Centralizes skip decision-making in one place
4. **No Orchestrator Filesystem Access**: The orchestrator should not need filesystem access
5. **Separation of Concerns**: Orchestrator manages orchestration; worker manages transcription decisions

### Skip Check Priority Order

The worker's skip checker evaluates conditions in this order:
1. Target subtitle existence (SRT/LRC files)
2. Embedded subtitles (video files only, if enabled)
3. External subtitles (if enabled)
4. Audio language filtering (if configured)
5. Preferred audio language filtering (if configured)

---

## Testing Results

### Worker Unit Tests
- **13/15 tests passing**
- **2 tests failing**: External subtitle detection tests
  - Issue: Language code matching in `has_subtitle_language()` function
  - Status: Fixed language matching logic, needs retest

### Test Coverage
- Skip checker module: 80% coverage
- Detector module: 87% coverage
- Overall worker: 28% coverage (baseline)

---

## Remaining Work

1. **Fix External Subtitle Detection** (2 failing tests)
   - Verify language code matching works correctly
   - Ensure all test scenarios pass

2. **Run Full Worker Test Suite**
   - Verify no regressions in existing tests
   - Run integration tests

3. **Integration Testing**
   - Test with real media files
   - Verify end-to-end skip behavior

4. **Documentation Updates**
   - Update README-LLM.md to reflect skip logic in worker
   - Document all skip configuration options

5. **Production Deployment**
   - Test in production environment
   - Verify skip behavior matches expected behavior

---

## Files Changed

### Orchestrator (Removed skip logic)
- `orchestrator/internal/skip/` - Entire package removed
- `orchestrator/internal/webhooks/server.go` - Removed skip checker
- `orchestrator/internal/monitor/scanner.go` - Removed skip checker
- `orchestrator/cmd/orchestrator/main.go` - Removed skip initialization
- `orchestrator/internal/monitor/scanner_test.go` - Updated tests
- `orchestrator/internal/webhooks/batch_integration_test.go` - Updated tests

### Worker (Added comprehensive skip logic)
- `worker/src/subtitles/detector.py` - New module
- `worker/src/subtitles/skip_checker.py` - New module
- `worker/src/config/settings.py` - Added skip configuration fields
- `worker/src/grpc_server/service.py` - Integrated skip checker
- `worker/tests/unit/test_skip_logic.py` - New unit tests
- `worker/tests/integration/test_skip_e2e.py` - New integration tests

---

## Next Steps

1. Fix remaining 2 failing unit tests
2. Run full worker test suite
3. Create comprehensive E2E tests with real media
4. Update documentation
5. Deploy to production and verify

---

## Notes

- The orchestrator no longer has any skip logic
- All skip decisions are now made in the worker
- The worker's skip checker is comprehensive and covers all scenarios from the orchestrator's original skip logic
- Configuration environment variables remain the same for backwards compatibility
