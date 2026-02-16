# Story 06: Advanced Skip Conditions

**Epic**: EPIC_06  
**Status**: In Progress  
**Assignee**: Orchestrator Agent  
**Effort**: 6-8 hours  
**Priority**: LOW

---

## User Story

As a **media server operator**,  
I want **Subgen to support advanced skip conditions like unknown language detection and edge cases**,  
So that **I can have complete control over which files are processed with fine-grained skip logic**.

---

## Acceptance Criteria

- [ ] Story file created with complete details
- [ ] Implement SKIP_UNKNOWN_LANGUAGE condition
- [ ] Implement SKIP_IF_NO_LANGUAGE_BUT_SUBTITLES_EXIST condition
- [ ] Verify SKIP_ONLY_SUBGEN_SUBTITLES already implemented (from STORY_03)
- [ ] Verify audio file + LRC logic already implemented (from STORY_01)
- [ ] Configuration for both new skip conditions
- [ ] Integration with existing BasicChecker
- [ ] Comprehensive tests (happy/unhappy paths, edge cases)
- [ ] All tests passing (unit + integration)
- [ ] Type checking passes (Go build succeeds)
- [ ] Integration points documented
- [ ] Work log created

---

## Technical Design

### Approach

Implement remaining advanced skip conditions:

1. **SKIP_UNKNOWN_LANGUAGE**: Skip files where language detection fails or returns unknown/undefined
2. **SKIP_IF_NO_LANGUAGE_BUT_SUBTITLES_EXIST**: Skip files with no detected language but existing subtitles
3. **Verify existing conditions**: 
   - SKIP_ONLY_SUBGEN_SUBTITLES (implemented in STORY_03)
   - Audio file + LRC skip logic (implemented in STORY_01)

### Key Design Decisions

**Decision**: SKIP_UNKNOWN_LANGUAGE only applies when language detection is enabled
**Rationale**: If language detection is disabled, there is no "unknown language" to skip

**Decision**: SKIP_IF_NO_LANGUAGE_BUT_SUBTITLES_EXIST prevents redundant processing
**Rationale**: If we can't detect the language but subtitles exist, likely subtitle is already good

**Decision**: Unknown language represented as empty string or special value
**Rationale**: Need consistent representation across the codebase

### Files to Create/Modify

- `orchestrator/internal/skip/advanced_checker.go` - NEW: Advanced skip conditions
  - `AdvancedChecker` struct
  - `CheckUnknownLanguage()` method
  - `CheckNoLanguageButSubtitlesExist()` method

- `orchestrator/internal/skip/advanced_checker_test.go` - NEW: Tests
  - Happy paths for both conditions
  - Unhappy paths (disabled, edge cases)
  - Integration tests

- `orchestrator/internal/skip/config.go` - MODIFY: Add configuration
  - `SkipUnknownLanguage bool` (default: false)
  - `SkipIfNoLanguageButSubtitlesExist bool` (default: false)

- `orchestrator/internal/skip/checker.go` - MODIFY: Add skip reasons
  - `ReasonUnknownLanguage`
  - `ReasonNoLanguageButSubtitlesExist`

- `orchestrator/internal/skip/basic_checker.go` - MODIFY: Integrate advanced checks
  - Add advanced checker field
  - Call advanced checks in Check() method

### Integration Points

**IMPLEMENTED: Basic Skip Logic** (`orchestrator/internal/skip/basic_checker.go`):
- ✅ File existence checks
- ✅ Embedded subtitle detection
- ✅ External subtitle scanning
- ✅ Language filtering
- ✅ Audio filtering
- ✅ SKIP_ONLY_SUBGEN_SUBTITLES (from STORY_03)
- ✅ Audio file + LRC logic (from STORY_01)

**NEW: Advanced Skip Conditions**:
- ⚠️ Unknown language detection
- ⚠️ No language but subtitles exist detection
- ⚠️ Configuration for advanced conditions

**INTEGRATION NEEDED**:
- ⏱️ Language detection service (for unknown language check)
- ⏱️ Subtitle existence check (reuse from STORY_01/STORY_03)

---

## Testing Strategy

### Unit Tests (TDD - Write FIRST)

**AdvancedChecker Tests:**

**SKIP_UNKNOWN_LANGUAGE - Happy Paths:**
1. **Skip when language is unknown**: detectedLang = "", SKIP_UNKNOWN_LANGUAGE=true → ShouldSkip=true
2. **Skip when language is "unknown"**: detectedLang = "unknown", SKIP_UNKNOWN_LANGUAGE=true → ShouldSkip=true
3. **Don't skip when language is valid**: detectedLang = "eng", SKIP_UNKNOWN_LANGUAGE=true → ShouldSkip=false
4. **Don't skip when disabled**: detectedLang = "", SKIP_UNKNOWN_LANGUAGE=false → ShouldSkip=false

**SKIP_UNKNOWN_LANGUAGE - Unhappy Paths:**
1. **Disabled by default**: detectedLang = "" → ShouldSkip=false
2. **Error in language detection**: detector returns error → ShouldSkip=false (fail open)

**SKIP_IF_NO_LANGUAGE_BUT_SUBTITLES_EXIST - Happy Paths:**
1. **Skip when no language but subs exist**: detectedLang = "", hasSubtitles = true → ShouldSkip=true
2. **Don't skip when has language**: detectedLang = "eng", hasSubtitles = true → ShouldSkip=false
3. **Don't skip when no subs**: detectedLang = "", hasSubtitles = false → ShouldSkip=false
4. **Don't skip when disabled**: detectedLang = "", hasSubtitles = true, flag = false → ShouldSkip=false

**SKIP_IF_NO_LANGUAGE_BUT_SUBTITLES_EXIST - Edge Cases:**
1. **No language, embedded subs**: Should skip
2. **No language, external subs**: Should skip
3. **No language, both types of subs**: Should skip
4. **Language unknown (not empty)**: detectedLang = "unknown" → Should treat as no language

**Integration Tests:**
1. **Combined unknown language and subtitle check**: Various combinations
2. **Interaction with other skip conditions**: Ensure proper ordering
3. **Disabled vs enabled scenarios**: Configuration combinations

### Test Data

Need to mock:
- Language detection results (empty, "unknown", "eng", etc.)
- Subtitle existence (embedded, external, both, none)
- Configuration flags

---

## Implementation Details

### AdvancedChecker Type (advanced_checker.go)

```go
package skip

import (
	"context"
	"fmt"
)

// AdvancedChecker implements advanced skip conditions
type AdvancedChecker struct {
	config *Config
}

// NewAdvancedChecker creates a new AdvancedChecker
func NewAdvancedChecker(config *Config) (*AdvancedChecker, error) {
	if config == nil {
		return nil, fmt.Errorf("config cannot be nil")
	}
	
	return &AdvancedChecker{
		config: config,
	}, nil
}

// CheckUnknownLanguage checks if file should be skipped due to unknown language
// Returns (shouldSkip, reason, details, error)
func (c *AdvancedChecker) CheckUnknownLanguage(detectedLang string) (bool, string) {
	// Only apply if SKIP_UNKNOWN_LANGUAGE is enabled
	if !c.config.SkipUnknownLanguage {
		return false, ""
	}
	
	// Consider empty string, "unknown", "undefined", "und" as unknown
	if detectedLang == "" || 
	   detectedLang == "unknown" || 
	   detectedLang == "undefined" || 
	   detectedLang == "und" {
		return true, fmt.Sprintf("language detection returned unknown: %q", detectedLang)
	}
	
	return false, ""
}

// CheckNoLanguageButSubtitlesExist checks if file should be skipped when:
// - Language cannot be detected (empty/unknown)
// - But subtitles already exist (embedded or external)
// This prevents redundant processing when we can't detect language but subs exist
func (c *AdvancedChecker) CheckNoLanguageButSubtitlesExist(detectedLang string, hasSubtitles bool) (bool, string) {
	// Only apply if flag is enabled
	if !c.config.SkipIfNoLanguageButSubtitlesExist {
		return false, ""
	}
	
	// Check if language is unknown/empty
	isUnknown := detectedLang == "" || 
	             detectedLang == "unknown" || 
	             detectedLang == "undefined" || 
	             detectedLang == "und"
	
	// Skip if language is unknown AND subtitles exist
	if isUnknown && hasSubtitles {
		return true, fmt.Sprintf("no language detected but subtitles exist (lang=%q)", detectedLang)
	}
	
	return false, ""
}

// IsUnknownLanguage is a helper to check if a language string represents unknown/undefined
func IsUnknownLanguage(lang string) bool {
	return lang == "" || 
	       lang == "unknown" || 
	       lang == "undefined" || 
	       lang == "und"
}
```

### Configuration Updates (config.go)

```go
// Add to Config struct:
type Config struct {
	SkipIfTargetSubtitleExists      bool
	CheckEmbeddedSubtitles          bool
	SkipIfInternalSubtitlesLanguage string
	SkipIfExternalSubtitlesExist    bool
	SkipOnlySubgenSubtitles         bool
	SkipSubtitleLanguages           []string
	SkipIfAudioLanguages            []string
	PreferredAudioLanguages         []string
	LimitToPreferredAudioLanguage   bool
	// NEW: Advanced skip conditions
	SkipUnknownLanguage             bool  // Skip if language detection fails
	SkipIfNoLanguageButSubtitlesExist bool // Skip if no language but subs exist
}

// Update NewConfig():
func NewConfig() (*Config, error) {
	// ... existing code ...
	
	// Skip unknown language (default: false)
	skipUnknownLangStr := os.Getenv("SKIP_UNKNOWN_LANGUAGE")
	if skipUnknownLangStr == "" {
		skipUnknownLangStr = "false"
	}
	skipUnknownLang, err := strconv.ParseBool(skipUnknownLangStr)
	if err != nil {
		return nil, fmt.Errorf("invalid SKIP_UNKNOWN_LANGUAGE value: %w", err)
	}
	
	// Skip if no language but subtitles exist (default: false)
	skipNoLangButSubsStr := os.Getenv("SKIP_IF_NO_LANGUAGE_BUT_SUBTITLES_EXIST")
	if skipNoLangButSubsStr == "" {
		skipNoLangButSubsStr = "false"
	}
	skipNoLangButSubs, err := strconv.ParseBool(skipNoLangButSubsStr)
	if err != nil {
		return nil, fmt.Errorf("invalid SKIP_IF_NO_LANGUAGE_BUT_SUBTITLES_EXIST value: %w", err)
	}
	
	return &Config{
		// ... existing fields ...
		SkipUnknownLanguage:             skipUnknownLang,
		SkipIfNoLanguageButSubtitlesExist: skipNoLangButSubs,
	}, nil
}
```

### Update Checker Constants (checker.go)

```go
const (
	// ... existing constants ...
	ReasonAudioLanguageMismatch SkipReason = "audio_language_mismatch"
	// ReasonUnknownLanguage indicates language detection failed or returned unknown
	ReasonUnknownLanguage SkipReason = "unknown_language"
	// ReasonNoLanguageButSubtitlesExist indicates no language detected but subtitles exist
	ReasonNoLanguageButSubtitlesExist SkipReason = "no_language_but_subtitles_exist"
	ReasonNotApplicable SkipReason = "not_applicable"
)
```

### Integration into BasicChecker (basic_checker.go)

```go
// Add to BasicChecker struct:
type BasicChecker struct {
	config          *Config
	detector        *SubtitleDetector
	externalScanner *ExternalScanner
	audioDetector   *AudioDetector
	advancedChecker *AdvancedChecker // NEW
}

// Update NewBasicChecker:
func NewBasicChecker(config *Config) (*BasicChecker, error) {
	// ... existing validation ...
	
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

// Update Check method:
func (c *BasicChecker) Check(ctx context.Context, filePath string) (*CheckResult, error) {
	// ... existing checks ...
	
	// Advanced checks (near end of check sequence)
	// Note: These need language detection result and subtitle existence status
	// For now, we check for subtitle existence and pass empty language
	// TODO: Language detection integration in STORY_07
	
	// Check if subtitles exist (either embedded or external)
	hasSubtitles := false
	
	// Check embedded subtitles
	if c.config.CheckEmbeddedSubtitles && isVideoFile(filePath) {
		tracks, err := c.detector.GetEmbeddedSubtitles(ctx, filePath)
		if err == nil && len(tracks) > 0 {
			hasSubtitles = true
		}
	}
	
	// Check external subtitles
	if !hasSubtitles && c.config.SkipIfExternalSubtitlesExist {
		subtitles, err := c.externalScanner.ScanForSubtitles(filePath)
		if err == nil && len(subtitles) > 0 {
			hasSubtitles = true
		}
	}
	
	// For now, we use empty language (will be populated by language detection service)
	// This allows the check to work in isolation
	detectedLang := ""  // TODO: Get from language detection service in STORY_07
	
	// Check SKIP_UNKNOWN_LANGUAGE
	if shouldSkip, details := c.advancedChecker.CheckUnknownLanguage(detectedLang); shouldSkip {
		return &CheckResult{
			ShouldSkip: true,
			Reason:     ReasonUnknownLanguage,
			Details:    details,
		}, nil
	}
	
	// Check SKIP_IF_NO_LANGUAGE_BUT_SUBTITLES_EXIST
	if shouldSkip, details := c.advancedChecker.CheckNoLanguageButSubtitlesExist(detectedLang, hasSubtitles); shouldSkip {
		return &CheckResult{
			ShouldSkip: true,
			Reason:     ReasonNoLanguageButSubtitlesExist,
			Details:    details,
		}, nil
	}
	
	// ... rest of existing logic ...
}
```

---

## Definition of Done

- [ ] Story file created with complete details
- [ ] Tests written FIRST (must fail initially)
- [ ] AdvancedChecker implemented with both conditions
- [ ] Configuration updated with advanced skip flags
- [ ] Skip reasons added to constants
- [ ] BasicChecker extended to use advanced checker
- [ ] Verified SKIP_ONLY_SUBGEN_SUBTITLES works (from STORY_03)
- [ ] Verified audio file + LRC logic works (from STORY_01)
- [ ] All tests passing (unit + integration)
- [ ] Go build succeeds (type checking)
- [ ] Integration points documented
- [ ] Code follows Go best practices
- [ ] Work log created in docs/WORKLOGS/
- [ ] Code committed

---

## Configuration Examples

### Example 1: Skip Unknown Languages
```env
SKIP_UNKNOWN_LANGUAGE=true
```

Result:
- File with detected English → Processed
- File with detected Japanese → Processed
- File with unknown/undefined language → Skipped
- File with no detectable language → Skipped

### Example 2: Skip When No Language But Subtitles Exist
```env
SKIP_IF_NO_LANGUAGE_BUT_SUBTITLES_EXIST=true
```

Result:
- File with no language, no subtitles → Processed (attempt transcription)
- File with no language, has subtitles → Skipped (already has subs, can't improve)
- File with detected language, has subtitles → Not skipped by this rule (other rules may apply)

### Example 3: Combined Advanced Conditions
```env
SKIP_UNKNOWN_LANGUAGE=true
SKIP_IF_NO_LANGUAGE_BUT_SUBTITLES_EXIST=true
```

Result:
- File with unknown language → Skipped (first rule)
- File with no language, no subtitles → Skipped (first rule)
- File with no language, has subtitles → Skipped (second rule)

---

## Verification of Existing Features

### SKIP_ONLY_SUBGEN_SUBTITLES (from STORY_03)

**Location**: `orchestrator/internal/skip/external_scanner.go`

**Implementation**:
```go
// IsSubgenGenerated checks if "subgen" appears in the filename
func (s *ExternalScanner) IsSubgenGenerated(filename string) bool {
	// Implementation exists in STORY_03
}
```

**Usage in BasicChecker**:
```go
// Filter subtitles if SKIP_ONLY_SUBGEN_SUBTITLES is enabled
if c.config.SkipOnlySubgenSubtitles {
	for _, sub := range subtitles {
		if sub.IsSubgenGenerated {
			filteredSubtitles = append(filteredSubtitles, sub)
		}
	}
}
```

**Status**: ✅ Already implemented and working

### Audio File + LRC Logic (from STORY_01)

**Location**: `orchestrator/internal/skip/basic_checker.go`

**Implementation**:
```go
// Check for LRC file (for audio files)
if isAudioFile(filePath) {
	lrcPath := strings.TrimSuffix(filePath, filepath.Ext(filePath)) + ".lrc"
	if _, err := os.Stat(lrcPath); err == nil {
		return &CheckResult{
			ShouldSkip: true,
			Reason:     ReasonSubtitleExists,
			Details:    "LRC file exists",
		}, nil
	}
}
```

**Status**: ✅ Already implemented and working

---

## References

- **Epic README**: docs/BACKLOG/EPIC_06/README.md lines 200-212
- **README-LLM.md**: Complete development guidelines
- **STORY_01**: docs/BACKLOG/EPIC_06/stories/STORY_01_basic_skip_logic.md (LRC logic)
- **STORY_03**: docs/BACKLOG/EPIC_06/stories/STORY_03_external_subtitle_scan.md (SKIP_ONLY_SUBGEN_SUBTITLES)
- **Original Implementation**: 
  - subgen.py lines 1564-1632 (should_skip_file function)
  - subgen.py line 1578-1583 (unknown language check)
  - subgen.py line 1570-1577 (LRC for audio files)

---

**Created**: 2026-02-15  
**Last Updated**: 2026-02-15
