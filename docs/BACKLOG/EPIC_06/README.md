# EPIC_06: Skip Logic & Intelligence System

**Status:** Not Started  
**Estimated Effort:** 40-52 hours  
**Duration:** 5-7 days  
**Priority:** 🔴 CRITICAL (Production Blocker)  
**Can Parallelize:** Partially (some stories independent)

---

## Overview

Implement the comprehensive skip logic system that prevents redundant transcription of files that already have subtitles or don't meet processing criteria. **This is the #1 missing feature** preventing production deployment.

**Impact:** Without skip logic, the system will re-process every file on every webhook, wasting 90%+ of compute resources in typical production environments.

---

## Problem Statement

In a production home media server:
- **90%+ of files already have subtitles** (from previous transcriptions, downloads, or other sources)
- **Every webhook event triggers processing** (library scan, playback start, etc.)
- **Without skip logic:** System wastes resources transcribing files unnecessarily
- **Original subgen.py:** Has 7 sophisticated skip conditions to avoid this

**Example:**
- User has 10,000 movie library
- Plex sends webhook for library scan: 10,000 events
- **Without skip logic:** Attempts to transcribe all 10,000 movies
- **With skip logic:** Skips 9,800 files, processes only 200 new ones

---

## Goals

1. Implement all 7 skip conditions from original subgen.py
2. Create skip checker abstraction for orchestrator
3. Implement embedded subtitle detection (pyav/FFprobe)
4. Implement external subtitle file scanning
5. Add skip configuration to orchestrator
6. Test skip logic with comprehensive test suite
7. Benchmark skip performance (should be < 100ms per file)

---

## User Stories

### [STORY_01: Basic Skip Logic](./stories/STORY_01_basic_skip_logic.md)
**Status:** Not Started  
**Effort:** 8-10 hours  
**Priority:** CRITICAL  
**Summary:** Check if subtitle file exists before transcribing

**Acceptance Criteria:**
- [ ] Skip if `.srt` file exists next to video
- [ ] Skip if `.lrc` file exists next to audio
- [ ] Configuration: `SKIP_IF_TARGET_SUBTITLES_EXIST` (default: true)
- [ ] Works with all webhook types
- [ ] Logs skip reason clearly

**Implementation:**
```go
// orchestrator/internal/skip/basic_checker.go
func (c *BasicChecker) ShouldSkip(filePath string) (bool, string, error) {
    // Check for SRT
    srtPath := strings.TrimSuffix(filePath, filepath.Ext(filePath)) + ".srt"
    if exists(srtPath) {
        return true, "subtitle file exists", nil
    }
    
    // Check for LRC (audio files)
    if isAudioFile(filePath) {
        lrcPath := strings.TrimSuffix(filePath, filepath.Ext(filePath)) + ".lrc"
        if exists(lrcPath) {
            return true, "LRC file exists", nil
        }
    }
    
    return false, "", nil
}
```

---

### [STORY_02: Embedded Subtitle Detection](./stories/STORY_02_embedded_subtitles.md)
**Status:** Not Started  
**Effort:** 10-12 hours  
**Priority:** HIGH  
**Summary:** Detect subtitles embedded in video files (MKV, MP4 containers)

**Acceptance Criteria:**
- [ ] Use FFprobe to detect embedded subtitle tracks
- [ ] Extract language codes from subtitle tracks
- [ ] Skip if embedded subtitle matches target language
- [ ] Configuration: Check embedded subtitles by default
- [ ] Support all common formats (SRT, SSA, ASS, PGS, etc.)

**Implementation:**
```go
// orchestrator/internal/skip/subtitle_detector.go
func (d *SubtitleDetector) GetEmbeddedSubtitles(filePath string) ([]SubtitleTrack, error) {
    // Run FFprobe
    cmd := exec.Command("ffprobe", "-v", "quiet", "-print_format", "json", 
                        "-show_streams", "-select_streams", "s", filePath)
    output, err := cmd.Output()
    
    // Parse JSON
    var probe FFProbeOutput
    json.Unmarshal(output, &probe)
    
    // Extract subtitle tracks
    var tracks []SubtitleTrack
    for _, stream := range probe.Streams {
        if stream.CodecType == "subtitle" {
            tracks = append(tracks, SubtitleTrack{
                Index:    stream.Index,
                Language: stream.Tags.Language,
                Title:    stream.Tags.Title,
                Codec:    stream.CodecName,
            })
        }
    }
    
    return tracks, nil
}
```

---

### [STORY_03: External Subtitle Scanning](./stories/STORY_03_external_subtitle_scan.md)
**Status:** Not Started  
**Effort:** 8-10 hours  
**Priority:** HIGH  
**Summary:** Scan directory for external subtitle files (.srt, .vtt, etc.)

**Acceptance Criteria:**
- [ ] Scan folder for 11 subtitle formats (.srt, .vtt, .sub, .ass, .ssa, .idx, .sbv, .pgs, .ttml, .lrc)
- [ ] Parse subtitle filenames for language codes
- [ ] Match against target language
- [ ] Configuration: `SKIP_IF_EXTERNAL_SUBTITLES_EXIST`
- [ ] Optional: Only skip subgen-generated subtitles (`SKIP_ONLY_SUBGEN_SUBTITLES`)

**Filename Patterns:**
```
movie.eng.srt          → English
movie.en.srt           → English
movie.english.srt      → English
movie.subgen.eng.srt   → English (subgen-generated)
movie.forced.eng.srt   → English (forced)
```

---

### [STORY_04: Language-Based Skip Logic](./stories/STORY_04_language_skip_logic.md)
**Status:** Not Started  
**Effort:** 8-10 hours  
**Priority:** MEDIUM  
**Summary:** Skip files based on subtitle or audio language criteria

**Acceptance Criteria:**
- [ ] Skip if subtitle in skip language list (`SKIP_SUBTITLE_LANGUAGES`)
- [ ] Skip if audio in skip language list (`SKIP_IF_AUDIO_LANGUAGES`)
- [ ] Skip if internal subtitle in specific language (`SKIP_IF_INTERNAL_SUBTITLES_LANGUAGE`)
- [ ] Audio track language detection via FFprobe
- [ ] Multiple language codes support (pipe-separated)

**Configuration:**
```env
SKIP_SUBTITLE_LANGUAGES="eng|jpn|kor"  # Skip if subtitles in these languages
SKIP_IF_AUDIO_LANGUAGES="eng"          # Skip if audio in these languages
SKIP_IF_INTERNAL_SUBTITLES_LANGUAGE="eng"  # Skip if internal English subs
```

---

### [STORY_05: Audio Language Filtering](./stories/STORY_05_audio_filtering.md)
**Status:** Not Started  
**Effort:** 6-8 hours  
**Priority:** MEDIUM  
**Summary:** Only process files with preferred audio languages

**Acceptance Criteria:**
- [ ] Detect audio track languages from video file
- [ ] Filter based on `PREFERRED_AUDIO_LANGUAGES`
- [ ] Configuration: `LIMIT_TO_PREFERRED_AUDIO_LANGUAGE`
- [ ] Log skip reason when audio doesn't match
- [ ] Support multiple preferred languages (pipe-separated)

**Use Case:**
```env
PREFERRED_AUDIO_LANGUAGES="eng|jpn"
LIMIT_TO_PREFERRED_AUDIO_LANGUAGE=true
# Only process files with English or Japanese audio
# Skip files with French, Spanish, etc. audio tracks
```

---

### [STORY_06: Advanced Skip Conditions](./stories/STORY_06_advanced_skip.md)
**Status:** Not Started  
**Effort:** 6-8 hours  
**Priority:** LOW  
**Summary:** Implement remaining specialized skip conditions

**Acceptance Criteria:**
- [ ] Skip unknown language: `SKIP_UNKNOWN_LANGUAGE`
- [ ] Skip if no language but subtitles exist: `SKIP_IF_NO_LANGUAGE_BUT_SUBTITLES_EXIST`
- [ ] Skip only subgen subtitles: `SKIP_ONLY_SUBGEN_SUBTITLES`
- [ ] Custom subtitle name matching: `SUBTITLE_LANGUAGE_NAME`
- [ ] Audio file + existing LRC skip logic

---

### [STORY_07: Skip Logic Integration & Testing](./stories/STORY_07_skip_integration.md)
**Status:** Not Started  
**Effort:** 4-6 hours  
**Priority:** HIGH  
**Summary:** Integrate skip logic into orchestrator pipeline and test thoroughly

**Acceptance Criteria:**
- [ ] Skip checker called before enqueueing tasks
- [ ] Webhook handlers check skip conditions
- [ ] Batch endpoint respects skip logic
- [ ] Skip statistics tracked (skipped vs processed)
- [ ] Comprehensive test suite (20+ test cases)
- [ ] Performance benchmark (< 100ms per check)

---

## Architecture

### Skip Checker Interface

```go
// orchestrator/internal/skip/checker.go
package skip

type SkipReason string

const (
    ReasonSubtitleExists        SkipReason = "subtitle_file_exists"
    ReasonEmbeddedSubtitle      SkipReason = "embedded_subtitle_exists"
    ReasonExternalSubtitle      SkipReason = "external_subtitle_exists"
    ReasonAudioLanguageMismatch SkipReason = "audio_language_mismatch"
    ReasonSubtitleLanguageSkip  SkipReason = "subtitle_language_in_skip_list"
    ReasonUnknownLanguage       SkipReason = "unknown_language"
    ReasonLRCExists             SkipReason = "lrc_file_exists"
)

type CheckResult struct {
    ShouldSkip bool
    Reason     SkipReason
    Details    string
}

type Checker interface {
    // Check if file should be skipped
    Check(ctx context.Context, filePath string, targetLang string) (*CheckResult, error)
    
    // Get configuration
    GetConfig() *Config
}

type Config struct {
    SkipIfTargetSubtitleExists      bool
    SkipIfExternalSubtitlesExist    bool
    SkipIfInternalSubtitlesLanguage string
    SkipSubtitleLanguages           []string
    SkipIfAudioLanguages            []string
    LimitToPreferredAudioLanguage   bool
    PreferredAudioLanguages         []string
    SkipOnlySubgenSubtitles         bool
    SkipUnknownLanguage             bool
    SkipIfNoLanguageButSubtitlesExist bool
}
```

### Component Structure

```
orchestrator/internal/skip/
├── checker.go              # Main interface & composite checker
├── config.go               # Configuration struct & validation
├── basic_checker.go        # STORY_01: File exists check
├── embedded_detector.go    # STORY_02: FFprobe integration
├── external_scanner.go     # STORY_03: Folder scanning
├── language_filter.go      # STORY_04: Language-based filtering
├── audio_filter.go         # STORY_05: Audio track filtering
├── advanced_checker.go     # STORY_06: Specialized conditions
└── checker_test.go         # Comprehensive tests
```

### Integration Points

**1. Webhook Handlers** (orchestrator/internal/webhooks/server.go)
```go
// Before enqueueing
result, err := s.skipChecker.Check(ctx, filePath, targetLang)
if err != nil {
    return err
}
if result.ShouldSkip {
    s.log.WithFields(logrus.Fields{
        "file_path": filePath,
        "reason":    result.Reason,
        "details":   result.Details,
    }).Info("File skipped")
    return c.SendString("") // 200 OK but no work done
}

// Proceed with enqueue...
```

**2. Batch Endpoint** (orchestrator/internal/webhooks/server.go)
```go
for _, filePath := range files {
    result, _ := s.skipChecker.Check(ctx, filePath, targetLang)
    if result.ShouldSkip {
        skipped++
        continue
    }
    s.queue.Enqueue(Task{FilePath: filePath})
    queued++
}
```

**3. Metrics** (orchestrator/internal/observability/metrics.go)
```go
FilesSkipped = prometheus.NewCounterVec(
    prometheus.CounterOpts{
        Name: "subgen_files_skipped_total",
        Help: "Number of files skipped by reason",
    },
    []string{"reason"},
)
```

---

## Testing Strategy

### Unit Tests (per story)
- Test each checker in isolation
- Mock FFprobe responses
- Test all configuration combinations
- Edge cases: missing files, invalid formats, etc.

### Integration Tests (STORY_07)
- Test skip logic with real files
- Verify webhook handlers respect skip decisions
- Test all 7 skip conditions together
- Performance benchmarks

### Performance Requirements
- **< 100ms per check** (embedded subtitle detection)
- **< 50ms per check** (basic file exists)
- **< 200ms per check** (external subtitle scan with 100 files)
- **< 1s per check** (full skip logic with all conditions)

---

## Configuration Examples

### Minimal (Default Behavior)
```env
SKIP_IF_TARGET_SUBTITLES_EXIST=true  # Only this enabled by default
```

### Conservative (Skip Most Duplicates)
```env
SKIP_IF_TARGET_SUBTITLES_EXIST=true
SKIP_IF_EXTERNAL_SUBTITLES_EXIST=true
SKIP_ONLY_SUBGEN_SUBTITLES=false  # Skip any subtitles, not just ours
```

### Aggressive (Skip Everything with Subs)
```env
SKIP_IF_TARGET_SUBTITLES_EXIST=true
SKIP_IF_EXTERNAL_SUBTITLES_EXIST=true
SKIP_SUBTITLE_LANGUAGES="eng|jpn|kor|spa|fre|ger"
SKIP_IF_AUDIO_LANGUAGES="eng"  # Skip English audio (already understood)
```

### Selective Processing Only
```env
PREFERRED_AUDIO_LANGUAGES="jpn|kor"
LIMIT_TO_PREFERRED_AUDIO_LANGUAGE=true
# Only process Japanese and Korean audio
# Skip everything else
```

---

## Dependencies

**Requires:**
- EPIC_01 (Go Orchestrator) - ✅ Complete
- EPIC_03 (Integration Testing) - ⚠️ Partially complete
- FFprobe binary in orchestrator container

**Blocks:**
- EPIC_05 (Migration & Cutover) - Can't migrate without skip logic
- Production deployment

**Parallelizable:**
- STORY_01-06 can be developed independently
- STORY_07 requires all others complete

---

## Risks & Mitigation

| Risk | Impact | Mitigation |
|------|--------|------------|
| FFprobe not available in Alpine | HIGH | Install ffmpeg package in orchestrator Dockerfile |
| Slow subtitle detection | MEDIUM | Cache results, add timeout limits |
| False positives (skipping files incorrectly) | MEDIUM | Comprehensive test suite, default to processing |
| Complex configuration overwhelming users | LOW | Provide sensible defaults, document presets |

---

## Success Metrics

- [ ] **Skip rate in production:** 85-95% of webhooks result in skips
- [ ] **Processing reduction:** 90%+ fewer transcriptions
- [ ] **Performance:** Skip checks complete in < 100ms average
- [ ] **False negative rate:** < 1% (files incorrectly skipped)
- [ ] **Configuration usability:** Users can enable with 1-2 env vars

---

## Timeline

**Day 1-2:** STORY_01 (Basic skip - file exists check)  
**Day 3:** STORY_02 (Embedded subtitle detection)  
**Day 4:** STORY_03 (External subtitle scanning)  
**Day 5:** STORY_04 (Language-based filtering)  
**Day 6:** STORY_05 (Audio language filtering) + STORY_06 (Advanced conditions)  
**Day 7:** STORY_07 (Integration & comprehensive testing)

---

## Definition of Done

- [ ] All 7 stories completed with ✅ status
- [ ] All 7 skip conditions implemented
- [ ] Skip checker integrated into all webhook handlers
- [ ] Configuration complete with all env vars
- [ ] Unit tests for all skip conditions (>90% coverage)
- [ ] Integration tests with real files
- [ ] Performance benchmarks meet targets
- [ ] Documentation complete (user guide + API docs)
- [ ] Work logs for each story
- [ ] Production deployment blocked on this epic

---

## References

- **Original Implementation:** `/home/mikekao/personal/subgen/subgen.py` lines 1564-1788
- **Feature Parity:** `/home/mikekao/personal/subgen/docs/WORKLOGS/FEATURE_PARITY_CHECKLIST.md`
- **Skip Functions:**
  - `should_skip_file()` - Lines 1564-1632
  - `has_subtitle_language()` - Lines 1670-1684
  - `has_subtitle_language_in_file()` - Lines 1686-1727
  - `has_subtitle_of_language_in_folder()` - Lines 1729-1784
  - `get_subtitle_languages()` - Lines 1634-1654
  - `get_audio_languages()` - Lines 1660-1668

---

**Epic Owner:** TBD  
**Created:** 2026-02-16  
**Last Updated:** 2026-02-16
