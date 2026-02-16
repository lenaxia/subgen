# Story 07: Skip Logic Integration & Testing

**Epic**: EPIC_06  
**Status**: In Progress  
**Assignee**: Orchestrator Agent  
**Effort**: 4-6 hours  
**Priority**: HIGH

---

## User Story

As a **media server operator**,  
I want **skip logic fully integrated into the webhook pipeline with comprehensive testing and performance benchmarks**,  
So that **I can confidently deploy the skip system in production knowing it works correctly and efficiently**.

---

## Acceptance Criteria

- [ ] Story file created with complete details
- [ ] AdvancedChecker integrated into BasicChecker
- [ ] Skip checker called in webhook handlers before enqueueing
- [ ] Skip statistics tracking (skipped vs processed)
- [ ] Batch endpoint respects skip logic
- [ ] Comprehensive integration tests (20+ test cases)
- [ ] End-to-end tests with real webhook payloads
- [ ] Performance benchmarks (<100ms per check)
- [ ] All tests passing (unit + integration + e2e)
- [ ] Type checking passes (Go build succeeds)
- [ ] Documentation updated (README, configuration guide)
- [ ] Work log created

---

## Technical Design

### Approach

Integrate skip logic into the main webhook processing flow:

1. **BasicChecker Integration**: Add AdvancedChecker to BasicChecker
2. **Webhook Handler Integration**: Call skip checker before enqueueing tasks
3. **Skip Statistics**: Track skip counts by reason (Prometheus metrics)
4. **Batch Endpoint**: Respect skip logic when processing multiple files
5. **Comprehensive Testing**: Integration tests, e2e tests, performance benchmarks
6. **Documentation**: Configuration guide, troubleshooting, examples

### Key Design Decisions

**Decision**: Skip checker called BEFORE task enqueue
- **Rationale**: Avoid queueing work that will be skipped anyway
- **Benefit**: Reduces queue size, faster response times

**Decision**: Track skip statistics with Prometheus metrics
- **Rationale**: Observability is critical for production deployment
- **Metrics**: `subgen_files_skipped_total{reason}`, `subgen_skip_check_duration_seconds`

**Decision**: Fail open on skip check errors
- **Rationale**: Don't block processing if skip check fails
- **Behavior**: Log error, proceed with processing

**Decision**: Language detection placeholder for now
- **Rationale**: Language detection service not yet integrated
- **Implementation**: Use empty string for detectedLang, TODO comment for integration point

### Files to Modify

- `orchestrator/internal/skip/basic_checker.go` - MODIFY: Integrate AdvancedChecker
  - Add `advancedChecker` field to BasicChecker
  - Call advanced checks in `Check()` method
  - Determine subtitle existence (embedded + external)
  - Pass detected language (placeholder: empty string)

- `orchestrator/internal/webhooks/server.go` - MODIFY: Integrate skip checker
  - Add skip checker field to Server
  - Call skip checker in Plex/Jellyfin/Emby/Tautulli handlers
  - Track skip statistics
  - Log skip decisions

- `orchestrator/internal/observability/metrics.go` - MODIFY: Add skip metrics
  - `FilesSkipped` counter by reason
  - `SkipCheckDuration` histogram

- `orchestrator/internal/webhooks/server_test.go` - MODIFY: Add skip integration tests

- `orchestrator/internal/skip/integration_test.go` - NEW: End-to-end integration tests

- `orchestrator/internal/skip/benchmark_test.go` - NEW: Performance benchmarks

### Integration Points

**IMPLEMENTED: Skip Logic** (`orchestrator/internal/skip/`):
- ✅ BasicChecker with file/embedded/external/language/audio checks
- ✅ AdvancedChecker with unknown language and no-language-but-subs checks
- ✅ Configuration system with 11 environment variables
- ✅ Comprehensive unit tests for all checkers

**NEW: Webhook Integration**:
- ⚠️ Call BasicChecker.Check() before task enqueue
- ⚠️ Handle CheckResult (ShouldSkip, Reason, Details)
- ⚠️ Log skip decisions with structured logging
- ⚠️ Return 200 OK even when skipped (idempotent webhook handling)

**NEW: Observability**:
- ⚠️ Prometheus metrics for skip counts by reason
- ⚠️ Performance metrics for skip check duration
- ⚠️ Dashboard queries documented

**INTEGRATION NEEDED**:
- ⏱️ Language detection service (future enhancement)
- ⏱️ Subtitle refresh after processing (Plex/Jellyfin API calls)

---

## Testing Strategy

### Integration Tests (TDD - Write FIRST)

**Webhook Handler Integration Tests:**

**Happy Paths:**
1. **Skip on existing subtitle**: File with .srt → ShouldSkip=true → 200 OK, task not queued
2. **Skip on embedded subtitle**: Video with embedded subs → ShouldSkip=true → Not queued
3. **Skip on external subtitle**: Video with external .srt → ShouldSkip=true → Not queued
4. **Skip on audio language mismatch**: French audio, preferred=eng → ShouldSkip=true
5. **Skip on unknown language**: detectedLang="" → ShouldSkip=true (if enabled)
6. **Don't skip valid file**: Video without subs → ShouldSkip=false → Task queued
7. **Batch processing**: 10 files, 5 have subs → 5 skipped, 5 queued

**Unhappy Paths:**
1. **Skip check error**: File not found → Log error, proceed with processing (fail open)
2. **Invalid file path**: Empty path → Error response
3. **Permission error**: Unreadable file → Log error, proceed

**Edge Cases:**
1. **Multiple skip conditions**: File matches multiple skip rules → Skip with first matching reason
2. **Skip disabled**: All skip conditions off → Never skip
3. **Concurrent skip checks**: 100 parallel webhook requests → All handled correctly

### End-to-End Tests

Create test scenarios with real file structures:

```
orchestrator/internal/skip/testdata/integration/
├── video_with_srt.mkv           # Should skip
├── video_with_srt.eng.srt       # Subtitle file
├── video_with_embedded.mkv      # Should skip (embedded subs)
├── video_without_subs.mkv       # Should NOT skip
├── audio_with_lrc.mp3           # Should skip
├── audio_with_lrc.lrc           # LRC file
├── audio_without_lrc.mp3        # Should NOT skip
└── batch_folder/                # For batch testing
    ├── movie1.mkv (has srt)
    ├── movie1.eng.srt
    ├── movie2.mkv (no srt)
    ├── movie3.mkv (has srt)
    └── movie3.eng.srt
```

**E2E Test Cases:**
1. **Plex webhook with existing subtitle**: Should skip
2. **Jellyfin webhook without subtitle**: Should enqueue
3. **Batch processing folder**: Should skip files with subs, enqueue others
4. **Tautulli webhook with embedded subs**: Should skip
5. **ASR request (never skip)**: Should always process

### Performance Benchmarks

**Benchmark Requirements:**
- **File exists check**: <50ms
- **Embedded subtitle detection (FFprobe)**: <100ms
- **External subtitle scan**: <50ms per 100 files
- **Full skip check (all conditions)**: <200ms
- **Parallel checks (100 files)**: <2 seconds total

**Benchmark Tests:**
```go
func BenchmarkBasicChecker_FileExists(b *testing.B)
func BenchmarkBasicChecker_EmbeddedSubtitles(b *testing.B)
func BenchmarkBasicChecker_ExternalSubtitles(b *testing.B)
func BenchmarkBasicChecker_FullCheck(b *testing.B)
func BenchmarkBasicChecker_ParallelChecks(b *testing.B)
```

---

## Implementation Details

### BasicChecker Integration (basic_checker.go)

```go
// Add to BasicChecker struct:
type BasicChecker struct {
	config          *Config
	detector        *SubtitleDetector
	externalScanner *ExternalScanner
	audioDetector   *AudioDetector
	advancedChecker *AdvancedChecker
}

// Update NewBasicChecker:
func NewBasicChecker(config *Config) (*BasicChecker, error) {
	if config == nil {
		return nil, fmt.Errorf("config cannot be nil")
	}

	advancedChecker, err := NewAdvancedChecker(config)
	if err != nil {
		return nil, fmt.Errorf("failed to create advanced checker: %w", err)
	}

	return &BasicChecker{
		config:          config,
		detector:        NewSubtitleDetector(),
		externalScanner: NewExternalScanner(),
		audioDetector:   NewAudioDetector(),
		advancedChecker: advancedChecker,
	}, nil
}

// Update Check method to call advanced checks:
func (c *BasicChecker) Check(ctx context.Context, filePath string) (*CheckResult, error) {
	// ... existing checks (file exists, embedded, external, language, audio) ...

	// Determine if subtitles exist (for advanced checks)
	hasSubtitles := false

	// Check embedded subtitles
	if c.config.CheckEmbeddedSubtitles && isVideoFile(filePath) {
		tracks, err := c.detector.GetEmbeddedSubtitles(ctx, filePath)
		if err == nil && len(tracks) > 0 {
			hasSubtitles = true
		}
	}

	// Check external subtitles (if not already found embedded)
	if !hasSubtitles {
		subtitles, err := c.externalScanner.ScanForSubtitles(filePath)
		if err == nil && len(subtitles) > 0 {
			hasSubtitles = true
		}
	}

	// TODO: Get detected language from language detection service (FUTURE)
	// For now, use empty string (will trigger advanced checks if enabled)
	detectedLang := ""

	// Advanced checks
	if c.config.SkipUnknownLanguage {
		if shouldSkip, details := c.advancedChecker.CheckUnknownLanguage(detectedLang); shouldSkip {
			return &CheckResult{
				ShouldSkip: true,
				Reason:     ReasonUnknownLanguage,
				Details:    details,
			}, nil
		}
	}

	if c.config.SkipIfNoLanguageButSubtitlesExist {
		if shouldSkip, details := c.advancedChecker.CheckNoLanguageButSubtitlesExist(detectedLang, hasSubtitles); shouldSkip {
			return &CheckResult{
				ShouldSkip: true,
				Reason:     ReasonNoLanguageButSubtitlesExist,
				Details:    details,
			}, nil
		}
	}

	// No skip conditions matched
	return &CheckResult{
		ShouldSkip: false,
		Reason:     ReasonNotApplicable,
		Details:    "no skip conditions matched",
	}, nil
}
```

### Webhook Handler Integration (server.go)

```go
// Add to Server struct:
type Server struct {
	// ... existing fields ...
	skipChecker skip.Checker
}

// Update NewServer:
func NewServer(config *Config, queue QueueInterface, logger *logrus.Logger, skipChecker skip.Checker) *Server {
	return &Server{
		// ... existing fields ...
		skipChecker: skipChecker,
	}
}

// Update webhook handlers (example: handlePlex):
func (s *Server) handlePlex(c *fiber.Ctx) error {
	// ... parse webhook payload ...

	// Extract file path
	filePath := getFilePathFromPlexPayload(payload)
	if filePath == "" {
		return c.Status(400).SendString("missing file path")
	}

	// Check if file should be skipped
	ctx := c.Context()
	skipResult, err := s.skipChecker.Check(ctx, filePath)
	if err != nil {
		// Log error but don't fail (fail open)
		s.log.WithError(err).Warn("Skip check failed, proceeding with processing")
	} else if skipResult.ShouldSkip {
		// Log skip decision
		s.log.WithFields(logrus.Fields{
			"file_path": filePath,
			"reason":    skipResult.Reason,
			"details":   skipResult.Details,
		}).Info("File skipped")

		// Update metrics
		FilesSkipped.WithLabelValues(string(skipResult.Reason)).Inc()

		// Return 200 OK (idempotent webhook handling)
		return c.SendString("file skipped")
	}

	// Proceed with enqueue
	task := queue.Task{
		FilePath: filePath,
		// ... other fields ...
	}
	s.queue.Enqueue(task)

	return c.SendString("task queued")
}
```

### Metrics (observability/metrics.go)

```go
// Add skip metrics:
var (
	FilesSkipped = prometheus.NewCounterVec(
		prometheus.CounterOpts{
			Name: "subgen_files_skipped_total",
			Help: "Number of files skipped by reason",
		},
		[]string{"reason"},
	)

	SkipCheckDuration = prometheus.NewHistogram(
		prometheus.HistogramOpts{
			Name:    "subgen_skip_check_duration_seconds",
			Help:    "Duration of skip checks in seconds",
			Buckets: prometheus.DefBuckets, // 0.005s to 10s
		},
	)
)

func init() {
	prometheus.MustRegister(FilesSkipped)
	prometheus.MustRegister(SkipCheckDuration)
}
```

---

## Definition of Done

- [ ] Story file created with complete details
- [ ] Tests written FIRST (must fail initially)
- [ ] AdvancedChecker integrated into BasicChecker
- [ ] Skip checker integrated into webhook handlers
- [ ] Skip statistics tracking implemented
- [ ] Batch endpoint respects skip logic
- [ ] Integration tests complete (20+ test cases)
- [ ] End-to-end tests with real files
- [ ] Performance benchmarks meet requirements
- [ ] All tests passing (unit + integration + e2e + benchmarks)
- [ ] Go build succeeds (type checking)
- [ ] Metrics implemented and tested
- [ ] Documentation updated
- [ ] Work log created in docs/WORKLOGS/
- [ ] Code committed

---

## Configuration Summary

After this story, the complete skip configuration will be:

```env
# Basic skip conditions (STORY_01)
SKIP_IF_TARGET_SUBTITLES_EXIST=true  # Default: true

# Embedded subtitle checking (STORY_02)
CHECK_EMBEDDED_SUBTITLES=true  # Default: true
SKIP_IF_INTERNAL_SUBTITLES_LANGUAGE=eng  # Default: eng

# External subtitle checking (STORY_03)
SKIP_IF_EXTERNAL_SUBTITLES_EXIST=false  # Default: false
SKIP_ONLY_SUBGEN_SUBTITLES=false  # Default: false

# Language filtering (STORY_04)
SKIP_SUBTITLE_LANGUAGES=""  # Pipe-separated, default: empty
SKIP_IF_AUDIO_LANGUAGES=""  # Pipe-separated, default: empty

# Audio language filtering (STORY_05)
PREFERRED_AUDIO_LANGUAGES=""  # Pipe-separated, default: empty
LIMIT_TO_PREFERRED_AUDIO_LANGUAGE=false  # Default: false

# Advanced skip conditions (STORY_06)
SKIP_UNKNOWN_LANGUAGE=false  # Default: false
SKIP_IF_NO_LANGUAGE_BUT_SUBTITLES_EXIST=false  # Default: false
```

---

## Success Metrics

After implementation:
- [ ] **Skip rate in staging**: 85-95% of webhooks result in skips
- [ ] **Processing reduction**: 90%+ fewer transcriptions
- [ ] **Performance**: Skip checks complete in <100ms average
- [ ] **False negative rate**: <1% (files incorrectly skipped)
- [ ] **Test coverage**: >90% for skip package
- [ ] **Zero production errors** in skip logic during testing

---

## References

- **Epic README**: docs/BACKLOG/EPIC_06/README.md lines 215-228
- **README-LLM.md**: Complete development guidelines, TDD workflow
- **STORY_01-06**: Previous stories with skip logic components
- **Original Implementation**: subgen.py lines 1564-1788 (should_skip_file and related functions)

---

**Created**: 2026-02-16  
**Last Updated**: 2026-02-16
