# Work Log: Production Test Scenarios (Corrected)
## Date: February 19, 2026
## Epic: Production Validation Based on Feature Status Analysis

## Executive Summary

Based on the corrected feature status analysis (0067), this document outlines the specific production test scenarios needed to validate the ~91% implemented features. Focuses on testing the features that were previously incorrectly marked as "missing" or "partially implemented" but are actually fully implemented.

## Critical Features That Need Production Testing

Based on worklog 0067, these features were incorrectly assessed as missing but are actually **fully implemented** and need production validation:

### 1. Skip Logic System (8/8 features - 100% implemented)
**Previously incorrectly marked as "0% COMPLETELY MISSING"**

**Test Scenarios**:
1. **Skip if audio file has existing LRC**
   - Place `.wav` file with existing `.lrc` file
   - Trigger processing via monitoring or batch
   - Verify file is skipped with reason `lrc_file_exists`

2. **Skip if unknown language**
   - Use audio file with unintelligible/gibberish content
   - Verify language detection returns "unknown" or low confidence
   - Verify file is skipped with appropriate reason

3. **Skip if target subtitle already exists (internal or external)**
   - Test with video file that has embedded subtitles (use FFprobe)
   - Test with video file that has external `.srt` file
   - Verify both cases are skipped

4. **Skip if internal subtitle in specific language**
   - Configure `SKIP_IF_INTERNAL_SUBTITLES_LANGUAGE=eng`
   - Use video with embedded English subtitles
   - Verify file is skipped

5. **Skip if external subtitle with custom name**
   - Test custom subtitle naming patterns
   - Verify external subtitle detection works

6. **Skip if subtitle in skip language list**
   - Configure `SKIP_SUBTITLE_LANGUAGES=jpn,spa`
   - Test with files containing Japanese/Spanish subtitles
   - Verify files are skipped

7. **Skip if audio track in skip language list**
   - Configure `SKIP_AUDIO_LANGUAGES=fr,de`
   - Test with French/German audio files
   - Verify files are skipped

8. **All 8 configuration options validation**
   - Test each config option independently
   - Test combinations of options
   - Verify skip reasons logged correctly

### 2. File System Monitoring (8/8 features - 100% implemented)
**Previously incorrectly marked as "0% NOT IMPLEMENTED"**

**Test Scenarios**:
1. **Watchdog integration (fsnotify)**
   - Create new file in monitored directory
   - Verify file creation event detected (< 1s)
   - Verify event logged with structured data

2. **MONITOR environment variable**
   - Test with `MONITOR=true` and `MONITOR=false`
   - Verify monitoring starts/stops accordingly

3. **TRANSCRIBE_FOLDERS with pipe-separated directories**
   - Configure multiple directories: `/media/videos|/media/audio`
   - Verify both directories are monitored
   - Test file creation in each directory

4. **File stability checking**
   - Simulate slow file upload (copy large file)
   - Verify 3 stability checks with 2-second intervals
   - Verify file queued only after stability confirmed

5. **Startup folder scanning**
   - Place files in directory before orchestrator starts
   - Start orchestrator with `SCAN_ON_STARTUP=true`
   - Verify all files scanned and processed/skipped appropriately

6. **Recursive directory watching**
   - Create subdirectories within monitored directory
   - Place files in subdirectories
   - Verify files detected and processed

7. **File event handler integration**
   - Test various file events: create, modify, delete
   - Verify only create events trigger processing
   - Verify modified/deleted files ignored appropriately

8. **Integration with skip logic**
   - Place file that should be skipped in monitored directory
   - Verify file detected but skipped (not queued)
   - Verify skip reason logged

### 3. Path Mapping (2/2 features - 100% implemented)
**Previously incorrectly marked as "0% NOT IMPLEMENTED"**

**Test Scenarios**:
1. **Configuration options validation**
   - Test `USE_PATH_MAPPING=true/false`
   - Test `PATH_MAPPING_FROM` and `PATH_MAPPING_TO` patterns
   - Verify path translation works correctly

2. **Path translation in all webhook handlers**
   - Test Plex webhook with path mapping
   - Test Jellyfin webhook with path mapping
   - Test Emby webhook with path mapping
   - Test Tautulli webhook with path mapping
   - Test ASR handler with path mapping
   - Verify all handlers apply path mapping before processing

### 4. ASR Endpoint Blocking Response (9/9 features - 100% implemented)
**Previously incorrectly marked as "PARTIALLY IMPLEMENTED - returns placeholder"**

**Test Scenarios**:
1. **Blocking/synchronous response**
   - Send ASR request with audio file
   - Verify request blocks until transcription complete
   - Verify response contains actual subtitle content (not placeholder)

2. **Return subtitle content in all 6 formats**
   - Test each format parameter: `srt`, `lrc`, `vtt`, `txt`, `tsv`, `json`
   - Verify response contains correct format content
   - Verify content is valid for each format

3. **Hash-based deduplication**
   - Send same audio file twice
   - Verify second request returns cached result
   - Verify deduplication logged

4. **AudioContent in gRPC**
   - Verify audio bytes sent directly via gRPC (no temp files)
   - Test with various audio formats
   - Verify no `/media` directory permission issues

5. **File size validation (max 100MB)**
   - Test with file > 100MB
   - Verify appropriate error response
   - Test with file < 100MB
   - Verify successful processing

6. **Task queuing with priority**
   - Send multiple ASR requests simultaneously
   - Verify tasks queued with ASR priority (1)
   - Verify processing order respects priority

7. **Query parameters validation**
   - Test all parameters: `task`, `language`, `output`, `video_file`
   - Test invalid parameter values
   - Verify appropriate error responses

8. **Timeout handling**
   - Send request that will timeout
   - Verify timeout response (504)
   - Verify no orphaned tasks

9. **Integration with path mapping**
   - Test ASR with path mapping enabled
   - Verify paths translated correctly
   - Verify processing works with mapped paths

### 5. Batch Processing Endpoint (1/1 features - 100% implemented)
**Previously incorrectly marked as "NOT IMPLEMENTED"**

**Test Scenarios**:
1. **/batch POST endpoint**
   - Send POST to `/batch` with directory parameter
   - Verify response contains scan results
   - Verify skip reasons included in response

2. **Directory parameter handling**
   - Test with absolute paths
   - Test with relative paths
   - Test with non-existent directory
   - Verify appropriate responses

3. **Optional force language**
   - Test with `language=en` parameter
   - Test without language parameter
   - Verify language applied correctly

4. **Recursive folder processing**
   - Test with `recursive=true` parameter
   - Test with `recursive=false` (default)
   - Verify recursive scanning works

5. **Integration with skip logic**
   - Test batch on directory with mixed files (some should skip)
   - Verify skip counts and reasons in response
   - Verify only appropriate files queued

### 6. Plex Episode Queueing (3/3 features - 100% implemented)
**Previously incorrectly marked as "NOT IMPLEMENTED"**

**Test Scenarios**:
1. **PLEX_QUEUE_NEXT_EPISODE**
   - Configure `PLEX_QUEUE_NEXT_EPISODE=true`
   - Process a TV show episode
   - Verify next episode in season is queued
   - Verify season boundary detection works

2. **PLEX_QUEUE_SEASON**
   - Configure `PLEX_QUEUE_SEASON=true`
   - Process a TV show episode
   - Verify all remaining episodes in season are queued
   - Verify season completion detection

3. **PLEX_QUEUE_SERIES**
   - Configure `PLEX_QUEUE_SERIES=true`
   - Process a TV show episode
   - Verify all episodes in series are queued
   - Verify series completion handling

4. **get_next_plex_episode() function**
   - Test XML navigation logic
   - Test error handling at series end
   - Test with various Plex metadata structures

### 7. Multiple Output Formats (6/6 features - 100% implemented)
**Previously incorrectly marked as "PARTIALLY IMPLEMENTED - only SRT/LRC"**

**Test Scenarios**:
1. **SRT format validation**
   - Request SRT format
   - Verify valid SRT structure with timestamps
   - Verify sequential numbering

2. **LRC format validation**
   - Request LRC format
   - Verify valid LRC structure with [mm:ss.xx] timestamps
   - Verify appropriate for audio files

3. **VTT format validation**
   - Request VTT format
   - Verify WebVTT header and structure
   - Verify compatibility with web players

4. **TXT format validation**
   - Request TXT format
   - Verify plain text without timestamps
   - Verify readable format

5. **TSV format validation**
   - Request TSV format
   - Verify tab-separated values
   - Verify columns: start, end, text

6. **JSON format validation**
   - Request JSON format
   - Verify valid JSON structure
   - Verify includes segments array with start/end/text

## Test Execution Priority

### Priority 1: Critical Production Validation (Week 1)
**Features that were most incorrectly assessed**:
1. Skip Logic System (all 8 conditions)
2. File System Monitoring (fsnotify integration)
3. ASR Endpoint Blocking Response
4. Path Mapping (all webhook handlers)

### Priority 2: Core Feature Validation (Week 2)
**Features essential for production workflows**:
1. Batch Processing Endpoint
2. Multiple Output Formats (all 6 formats)
3. Plex Episode Queueing
4. Media Server Integrations

### Priority 3: Integration & Edge Cases (Week 3)
**Complete end-to-end validation**:
1. All features together in realistic scenarios
2. Error handling and recovery
3. Performance under load
4. Multi-worker distribution

## Test Environment Setup

### Required Test Data
1. **Audio Files**:
   - Various languages (EN, ES, FR, DE, JP, etc.)
   - Various durations (30s, 2min, 10min)
   - Various formats (wav, mp3, m4a)
   - Some with existing LRC files

2. **Video Files**:
   - With/without embedded subtitles
   - Various codecs (h264, h265)
   - Various durations (1min, 5min, 30min)
   - Some with external subtitle files

3. **Plex Metadata**:
   - TV show series structure
   - Season/episode numbering
   - XML metadata files

### Test Configuration
```bash
# Skip Logic Configuration
SKIP_IF_SUBTITLE_EXISTS=true
CHECK_EMBEDDED_SUBTITLES=true
SKIP_IF_INTERNAL_SUBTITLES_LANGUAGE=eng
SKIP_AUDIO_LANGUAGES=spa
PREFERRED_AUDIO_LANGUAGES=eng
LIMIT_TO_PREFERRED_AUDIO_LANGUAGE=false
SKIP_SUBTITLE_LANGUAGES=jpn

# File Monitoring Configuration
MONITOR=true
TRANSCRIBE_FOLDERS=/media/videos|/media/audio
SCAN_ON_STARTUP=true
FILE_STABILITY_CHECKS=3
FILE_STABILITY_WAIT=2s

# Path Mapping Configuration
USE_PATH_MAPPING=true
PATH_MAPPING_FROM=/media
PATH_MAPPING_TO=/mnt/nas/media

# Plex Configuration
PLEX_QUEUE_NEXT_EPISODE=true
PLEX_QUEUE_SEASON=false
PLEX_QUEUE_SERIES=false
```

## Success Criteria

### Quantitative Metrics
- **Skip Logic**: 100% correct skip decisions
- **File Monitoring**: < 1s detection latency, 100% file processing
- **ASR Endpoint**: < 30s response time for 1min audio
- **Batch Processing**: < 10s for directory with 100 files
- **Output Formats**: All 6 formats valid and correct

### Qualitative Metrics
- **Logging**: Clear skip reasons, processing status
- **Error Handling**: Graceful errors, no crashes
- **Integration**: All components work together
- **Usability**: Clear API responses, easy troubleshooting

## Risk Areas Identified from 0067

### High Confidence Areas (Code verified)
- Skip logic implementation (4,334 lines)
- File monitoring (2,657 lines)
- Path mapping (integrated in all handlers)
- ASR blocking response (lines 884-951 server.go)

### Medium Confidence Areas (Implemented but less tested)
- Plex episode queueing (needs Plex server)
- Advanced skip conditions (needs real media files)
- Some output formats (VTT, TXT, TSV, JSON)

### Unknown Areas (Need verification)
- CUSTOM_REGROUP (stable-ts)
- SUBGEN_KWARGS (Whisper parameters)
- Word-level highlighting
- Custom prompt support

## Test Reporting Template

For each test scenario:
```
Test: [Scenario Name]
Feature: [Feature Category]
Priority: [1/2/3]
Status: [PASS/FAIL/SKIPPED]

Configuration:
- [Config options used]

Test Steps:
1. [Step 1]
2. [Step 2]
3. [Step 3]

Expected Results:
- [Expected outcome 1]
- [Expected outcome 2]

Actual Results:
- [Actual outcome 1]
- [Actual outcome 2]

Logs:
[Relevant log excerpts]

Issues Found:
- [Issue 1]
- [Issue 2]

Recommendations:
- [Recommendation 1]
- [Recommendation 2]
```

## Conclusion

This corrected test plan focuses on validating the features that were **incorrectly assessed as missing** in previous analysis. The system is actually ~91% feature complete with all critical production features implemented.

**Immediate Focus**: Validate skip logic, file monitoring, ASR blocking, and path mapping - the features most incorrectly assessed.

**Success will confirm**: The system is production-ready with comprehensive skip logic, automated file processing, reliable ASR integration, and flexible path mapping for containerized deployments.

**Next after validation**: Address the actual ~9% missing features (progress reporting, hot reload, etc.) which are lower priority for production deployment.