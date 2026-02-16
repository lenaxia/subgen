# Story 05: Audio Language Filtering

**Epic**: EPIC_06  
**Status**: In Progress  
**Assignee**: Delegation Agent  
**Effort**: 6-8 hours  
**Priority**: MEDIUM

---

## User Story

As a **media server operator**,  
I want **Subgen to only process files with audio in my preferred languages**,  
So that **I can skip files with audio languages I don't understand or want to transcribe**.

---

## Acceptance Criteria

- [ ] Story file created with complete details
- [ ] Extend AudioDetector with preferred language filtering
- [ ] Configuration: `PREFERRED_AUDIO_LANGUAGES` (pipe-separated: "eng|jpn|kor")
- [ ] Configuration: `LIMIT_TO_PREFERRED_AUDIO_LANGUAGE` (bool, default: false)
- [ ] Skip files WITHOUT any preferred audio languages
- [ ] Support multiple preferred languages
- [ ] Integration with existing BasicChecker
- [ ] New skip reason: `ReasonAudioLanguageMismatch`
- [ ] Comprehensive tests (happy/unhappy paths, edge cases)
- [ ] All tests passing (unit + integration)
- [ ] Type checking passes (Go build succeeds)
- [ ] Integration points documented
- [ ] Work log created

---

## Technical Design

### Approach

Implement preferred audio language filtering with:

1. **Configuration**: Add `PREFERRED_AUDIO_LANGUAGES` and `LIMIT_TO_PREFERRED_AUDIO_LANGUAGE` env vars
2. **AudioDetector Extension**: Add method to check if audio tracks match preferred languages
3. **Skip Logic**: Skip files that have NO audio tracks in preferred languages
4. **Integration**: Extend BasicChecker to apply filtering when enabled

### Key Design Decisions

**Decision**: Only filter when LIMIT_TO_PREFERRED_AUDIO_LANGUAGE=true
**Rationale**: Users may have preferred languages set but not want filtering enabled

**Decision**: Skip files WITHOUT preferred audio (whitelist approach)
**Rationale**: Different from SKIP_IF_AUDIO_LANGUAGES which skips files WITH specific audio (blacklist)

**Decision**: Match ANY preferred language (OR logic)
**Rationale**: User may want multiple languages (e.g., "eng|jpn" means accept English OR Japanese)

### Files to Modify

- `orchestrator/internal/skip/language_filter.go` - Add HasAnyPreferredLanguage() method
- `orchestrator/internal/skip/language_filter_test.go` - Add tests for new method
- `orchestrator/internal/skip/config.go` - Add preferred language config fields
- `orchestrator/internal/skip/config_test.go` - Test new config fields
- `orchestrator/internal/skip/checker.go` - Add ReasonAudioLanguageMismatch constant
- `orchestrator/internal/skip/basic_checker.go` - Integrate preferred audio filtering
- `orchestrator/internal/skip/basic_checker_test.go` - Test integration

### Integration Points

**IMPLEMENTED: AudioDetector** (`orchestrator/internal/skip/language_filter.go`):
- ✅ AudioTrack struct defined
- ✅ GetAudioTracks(ctx, filePath) method
- ✅ HasLanguage(tracks, language) method
- ✅ ParseLanguageList() helper
- ✅ MatchesAnyLanguage() helper

**NEW: Preferred Language Filtering**:
- ⚠️ HasAnyPreferredLanguage(tracks, preferredLangs) method
- ⚠️ Config.PreferredAudioLanguages []string
- ⚠️ Config.LimitToPreferredAudioLanguage bool
- ⚠️ ReasonAudioLanguageMismatch skip reason

**INTEGRATION NEEDED: BasicChecker**:
- ⏱️ Add preferred audio filtering check in Check() method
- ⏱️ Only apply when LimitToPreferredAudioLanguage=true

---

## Testing Strategy

### Unit Tests (TDD - Write FIRST)

**AudioDetector.HasAnyPreferredLanguage() Tests:**

**Happy Paths:**
1. **Audio has preferred language**: tracks with "eng", preferred ["eng"] → true
2. **Audio has one of multiple preferred**: tracks with "jpn", preferred ["eng", "jpn", "kor"] → true
3. **Multiple audio tracks, one matches**: tracks with ["fre", "eng"], preferred ["eng"] → true
4. **ISO 639-1 vs 639-2 matching**: tracks with "en", preferred ["eng"] → true
5. **Case insensitive matching**: tracks with "ENG", preferred ["eng"] → true

**Unhappy Paths:**
1. **No audio matches preferred**: tracks with "fre", preferred ["eng", "jpn"] → false
2. **Empty tracks**: tracks [], preferred ["eng"] → false
3. **Empty preferred list**: tracks with "eng", preferred [] → false
4. **Audio with no language metadata**: tracks with "", preferred ["eng"] → false

**Edge Cases:**
1. **Multiple audio tracks, none match**: tracks with ["fre", "spa"], preferred ["eng", "jpn"] → false
2. **Whitespace in preferred list**: preferred [" eng ", "jpn "] → handles correctly
3. **Mixed ISO codes**: tracks with "en", preferred ["eng", "ja", "ko"] → true

**Integration Tests:**

1. **Skip when no preferred audio**: File with French audio, preferred ["eng"] → ShouldSkip=true, Reason=ReasonAudioLanguageMismatch
2. **Don't skip when preferred audio found**: File with English audio, preferred ["eng"] → ShouldSkip=false
3. **Don't skip when filtering disabled**: File with French audio, LIMIT_TO_PREFERRED_AUDIO_LANGUAGE=false → ShouldSkip=false
4. **Skip with multiple audio, none match**: File with ["fre", "spa"], preferred ["eng", "jpn"] → ShouldSkip=true
5. **Don't skip with multiple audio, one matches**: File with ["fre", "eng"], preferred ["eng"] → ShouldSkip=false

### Test Data

```go
// Mock audio tracks for testing
var (
	englishAudioTrack = AudioTrack{
		Index:    1,
		Language: "eng",
		Title:    "English",
		Codec:    "aac",
		Channels: 2,
	}
	
	japaneseAudioTrack = AudioTrack{
		Index:    1,
		Language: "jpn",
		Title:    "Japanese",
		Codec:    "aac",
		Channels: 2,
	}
	
	frenchAudioTrack = AudioTrack{
		Index:    1,
		Language: "fre",
		Title:    "French",
		Codec:    "aac",
		Channels: 2,
	}
	
	noLanguageAudioTrack = AudioTrack{
		Index:    1,
		Language: "",
		Title:    "Unknown",
		Codec:    "aac",
		Channels: 2,
	}
)
```

### Manual Testing

Not required for this story (can be fully tested with unit tests).

---

## Implementation Details

### HasAnyPreferredLanguage Method (language_filter.go)

```go
// HasAnyPreferredLanguage checks if any audio track matches any preferred language
// Returns true if at least one audio track has a language in the preferred list
// Returns false if no tracks match or if tracks/preferred is empty
func (d *AudioDetector) HasAnyPreferredLanguage(tracks []AudioTrack, preferredLangs []string) bool {
	if len(tracks) == 0 || len(preferredLangs) == 0 {
		return false
	}

	for _, track := range tracks {
		if track.Language == "" {
			continue
		}

		if MatchesAnyLanguage(track.Language, preferredLangs) {
			return true
		}
	}

	return false
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
	// NEW: Preferred audio language filtering
	PreferredAudioLanguages         []string // Preferred audio languages (pipe-separated)
	LimitToPreferredAudioLanguage   bool     // Only process files with preferred audio
}

// Update NewConfig():
func NewConfig() (*Config, error) {
	// ... existing code ...

	// Preferred audio languages (default: empty)
	preferredAudioLangStr := os.Getenv("PREFERRED_AUDIO_LANGUAGES")
	preferredAudioLangs := ParseLanguageList(preferredAudioLangStr)

	// Limit to preferred audio language (default: false)
	limitToPreferredStr := os.Getenv("LIMIT_TO_PREFERRED_AUDIO_LANGUAGE")
	if limitToPreferredStr == "" {
		limitToPreferredStr = "false"
	}

	limitToPreferred, err := strconv.ParseBool(limitToPreferredStr)
	if err != nil {
		return nil, fmt.Errorf("invalid LIMIT_TO_PREFERRED_AUDIO_LANGUAGE value: %w", err)
	}

	return &Config{
		// ... existing fields ...
		SkipSubtitleLanguages:           skipSubLangs,
		SkipIfAudioLanguages:            skipAudioLangs,
		PreferredAudioLanguages:         preferredAudioLangs,
		LimitToPreferredAudioLanguage:   limitToPreferred,
	}, nil
}
```

### Update Checker Constants (checker.go)

```go
const (
	// ... existing constants ...
	ReasonAudioLanguageSkip SkipReason = "audio_language_in_skip_list"
	// ReasonAudioLanguageMismatch indicates audio language doesn't match preferred list
	ReasonAudioLanguageMismatch SkipReason = "audio_language_mismatch"
	ReasonNotApplicable SkipReason = "not_applicable"
)
```

### Integration into BasicChecker (basic_checker.go)

```go
// Update Check method (add after audio language skip check):
func (c *BasicChecker) Check(ctx context.Context, filePath string) (*CheckResult, error) {
	// ... existing checks ...

	// Check preferred audio language filtering (if enabled)
	if c.config.LimitToPreferredAudioLanguage && len(c.config.PreferredAudioLanguages) > 0 && isVideoFile(filePath) {
		audioTracks, err := c.audioDetector.GetAudioTracks(ctx, filePath)
		if err != nil {
			// Log error but don't fail the check
			// Continue with other checks
		} else {
			// Check if file has any preferred audio language
			hasPreferred := c.audioDetector.HasAnyPreferredLanguage(audioTracks, c.config.PreferredAudioLanguages)
			if !hasPreferred {
				return &CheckResult{
					ShouldSkip: true,
					Reason:     ReasonAudioLanguageMismatch,
					Details:    "no audio tracks match preferred languages",
				}, nil
			}
		}
	}

	// ... rest of existing logic ...
}
```

---

## Definition of Done

- [ ] Story file created with complete details
- [ ] Tests written FIRST (must fail initially)
- [ ] HasAnyPreferredLanguage() method implemented
- [ ] Configuration updated with preferred audio filtering options
- [ ] ReasonAudioLanguageMismatch constant added
- [ ] BasicChecker extended to apply preferred audio filtering
- [ ] All tests passing (unit + integration)
- [ ] Go build succeeds (type checking)
- [ ] Integration points documented
- [ ] Code follows Go best practices
- [ ] Work log created in docs/WORKLOGS/
- [ ] Code committed

---

## Use Case Examples

### Example 1: Only Process Japanese and Korean Media
```env
PREFERRED_AUDIO_LANGUAGES="jpn|kor"
LIMIT_TO_PREFERRED_AUDIO_LANGUAGE=true
```

Result:
- File with Japanese audio → Processed
- File with Korean audio → Processed
- File with English audio → Skipped (audio_language_mismatch)
- File with French audio → Skipped (audio_language_mismatch)

### Example 2: Preferred Languages Set But Filtering Disabled
```env
PREFERRED_AUDIO_LANGUAGES="eng"
LIMIT_TO_PREFERRED_AUDIO_LANGUAGE=false
```

Result:
- File with English audio → Processed
- File with Japanese audio → Processed (filtering disabled)
- File with French audio → Processed (filtering disabled)

### Example 3: Multiple Preferred Languages
```env
PREFERRED_AUDIO_LANGUAGES="eng|jpn|kor"
LIMIT_TO_PREFERRED_AUDIO_LANGUAGE=true
```

Result:
- File with English OR Japanese OR Korean audio → Processed
- File with Spanish audio only → Skipped

---

## References

- **Epic README**: docs/BACKLOG/EPIC_06/README.md lines 176-197
- **README-LLM.md**: Complete development guidelines
- **STORY_04**: docs/BACKLOG/EPIC_06/stories/STORY_04_language_skip_logic.md (AudioDetector implementation)
- **Original Implementation**: 
  - subgen.py lines 1564-1632 (should_skip_file function)
  - subgen.py line 1627 (limit_to_preferred_audio_languages check)
  - subgen.py lines 1660-1668 (get_audio_languages function)

---

**Created**: 2026-02-15  
**Last Updated**: 2026-02-15
