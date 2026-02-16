# Story 03: External Subtitle Scanning

**Epic**: EPIC_06  
**Status**: Complete  
**Assignee**: Delegation Agent  
**Effort**: 8-10 hours (Actual: ~3 hours)  
**Priority**: HIGH

---

## User Story

As a **media server operator**,  
I want **Subgen to scan directories for external subtitle files and parse their filenames for language codes**,  
So that **I can skip transcription when subtitles already exist in various formats and languages**.

---

## Acceptance Criteria

- [x] Story file created with complete details
- [x] ExternalScanner struct for scanning directories
- [x] Support 11 subtitle formats: .srt, .vtt, .sub, .ass, .ssa, .idx, .sbv, .pgs, .ttml, .lrc, .smi
- [x] Parse subtitle filenames for language codes (movie.eng.srt, movie.en.srt, movie.english.srt)
- [x] Support multiple filename patterns (forced, subgen, language variations)
- [x] Match subtitles against target language
- [x] Configuration: SKIP_IF_EXTERNAL_SUBTITLES_EXIST (default: false)
- [x] Optional: SKIP_ONLY_SUBGEN_SUBTITLES to only skip subgen-generated subtitles
- [x] Case-insensitive language code matching
- [x] Support ISO 639-1 (en), ISO 639-2 (eng), and full names (english, English)
- [x] Detect "subgen" in filename for SKIP_ONLY_SUBGEN_SUBTITLES option
- [x] Comprehensive tests (happy/unhappy paths, edge cases)
- [x] All tests passing (unit tests)
- [x] Type checking passes (Go build succeeds)
- [x] Integration with existing Checker interface
- [x] Integration points documented
- [x] Work log created

---

## Technical Design

### Approach

Implement external subtitle scanning with:

1. **Directory Scanning**: Scan the same directory as the media file for subtitle files
2. **Filename Parsing**: Parse subtitle filenames to extract language codes
3. **Language Matching**: Match parsed language codes against target language
4. **Format Support**: Support 11 subtitle formats (.srt, .vtt, .sub, .ass, .ssa, .idx, .sbv, .pgs, .ttml, .lrc, .smi)
5. **Subgen Detection**: Detect "subgen" in filename for optional filtering
6. **Configuration**: Add SKIP_IF_EXTERNAL_SUBTITLES_EXIST and SKIP_ONLY_SUBGEN_SUBTITLES options
7. **Integration**: Extend existing Checker interface

### Filename Patterns to Support

```
movie.eng.srt              → English (ISO 639-2)
movie.en.srt               → English (ISO 639-1)
movie.english.srt          → English (full name)
movie.English.srt          → English (case insensitive)
movie.subgen.eng.srt       → English (subgen-generated)
movie.forced.eng.srt       → English (forced subtitles)
movie.eng.forced.srt       → English (forced, alternative order)
movie.sdh.eng.srt          → English (SDH - subtitles for deaf/hard of hearing)
movie.eng.sdh.srt          → English (SDH, alternative order)
movie.hi.eng.srt           → English (hearing impaired)
movie.cc.eng.srt           → English (closed captions)
movie.2.eng.srt            → English (track number)
```

### Files to Create

- `orchestrator/internal/skip/external_scanner.go` - External subtitle scanner
  - `ExternalScanner` struct
  - `ScanForSubtitles(filePath string) ([]ExternalSubtitle, error)` method
  - `ParseLanguageFromFilename(filename string) (string, bool)` method
  - `HasLanguage(subtitles []ExternalSubtitle, targetLang string) bool` method
  - `IsSubgenGenerated(filename string) bool` method
  - Helper functions for language code matching

- `orchestrator/internal/skip/external_scanner_test.go` - Comprehensive tests
  - Happy path: detect English subtitle with various patterns
  - Happy path: detect multiple subtitle formats
  - Happy path: detect subgen-generated subtitles
  - Happy path: case-insensitive language matching
  - Unhappy path: no subtitle files found
  - Unhappy path: invalid directory
  - Unhappy path: subtitle without language code
  - Edge case: subtitle with multiple language codes
  - Edge case: subtitle with complex filename patterns
  - Edge case: hidden subtitle files
  - Edge case: symlinks to subtitle files

### Files to Modify

- `orchestrator/internal/skip/checker.go` - Add skip reason
  - `ReasonExternalSubtitle` constant

- `orchestrator/internal/skip/config.go` - Add configuration fields
  - `SkipIfExternalSubtitlesExist bool` (default: false)
  - `SkipOnlySubgenSubtitles bool` (default: false)
  - Update `NewConfig()` to read env vars
  - Update `Validate()` if needed

- `orchestrator/internal/skip/basic_checker.go` - Integrate external scanning
  - Add `ExternalScanner` field to `BasicChecker`
  - Update `Check()` method to scan for external subtitles
  - Handle SKIP_ONLY_SUBGEN_SUBTITLES option

### Integration Points

**IMPLEMENTED: Skip Checker Module** (`orchestrator/internal/skip/`):
- ✅ Interface defined: `Checker` with `Check(ctx, filePath)` method
- ✅ `CheckResult` struct for skip decisions
- ✅ `BasicChecker` for file existence and embedded subtitle checks
- ✅ Configuration system with environment variables

**NEW: External Subtitle Scanning**:
- ⚠️ `ExternalScanner` for directory scanning
- ⚠️ Filename parsing with language code extraction
- ⚠️ Extended `CheckResult` with `ReasonExternalSubtitle`
- ⚠️ Configuration for external subtitle skipping
- ⚠️ Integration into `BasicChecker`

**INTEGRATION NEEDED: Webhook Handlers**:
- ⏱️ Future story (will use existing `Checker` interface)

**Queue Module**:
- ✅ Not directly integrated (skip happens before enqueue)

**Observability**:
- ⏱️ Future: Add skip metrics for external subtitles

---

## Testing Strategy

### Unit Tests

**ExternalScanner Tests:**

**Happy Paths:**
1. **Detect English subtitle (ISO 639-2)**: movie.eng.srt detected
2. **Detect English subtitle (ISO 639-1)**: movie.en.srt detected
3. **Detect English subtitle (full name)**: movie.english.srt detected
4. **Detect multiple formats**: .srt, .vtt, .ass, .ssa, .sub, etc.
5. **Case insensitive matching**: movie.English.srt, movie.ENGLISH.srt
6. **Detect subgen subtitle**: movie.subgen.eng.srt detected as subgen-generated
7. **Detect forced subtitle**: movie.forced.eng.srt, movie.eng.forced.srt
8. **Multiple subtitle files**: Find all matching subtitles in directory
9. **Match target language**: HasLanguage returns true when language matches
10. **Don't match different language**: HasLanguage returns false for Japanese vs English

**Unhappy Paths:**
1. **No subtitle files**: Directory contains no subtitle files
2. **Invalid directory**: Non-existent directory path
3. **Permission error**: Directory not readable
4. **Empty filename**: Handle empty string gracefully
5. **Subtitle without language**: movie.srt (no language code)
6. **Malformed filename**: Files with weird patterns

**Edge Cases:**
1. **Multiple extensions**: movie.eng.hi.srt (multiple modifiers)
2. **Complex patterns**: movie.subgen.forced.eng.cc.srt
3. **Hidden files**: .movie.eng.srt (starts with dot)
4. **Symlinks**: Subtitle files are symlinks
5. **Track numbers**: movie.2.eng.srt (track number)
6. **SDH/CC/HI subtitles**: movie.sdh.eng.srt, movie.cc.eng.srt
7. **No extension on video**: Handle video files without extensions
8. **Very long filenames**: Handle 255+ character filenames
9. **Unicode filenames**: Handle UTF-8 characters in filenames
10. **Dots in video name**: my.movie.2024.eng.mkv → my.movie.2024.eng.srt

**Integration Tests:**
1. **Skip with external English subtitle**: CheckResult.ShouldSkip = true
2. **Don't skip without external subtitle**: CheckResult.ShouldSkip = false
3. **Skip only subgen subtitles**: Skip movie.subgen.eng.srt, not movie.eng.srt
4. **Don't skip with language mismatch**: English subtitle, looking for Japanese
5. **Disabled external checking**: Never skip based on external subs
6. **Match various language formats**: ISO 639-1, ISO 639-2, full names

### Test File Structure

```
orchestrator/internal/skip/testdata/external/
├── video.mkv                      # Video without subtitles
├── video_with_sub.mkv             # Video with various subtitle formats
├── video_with_sub.eng.srt         # English subtitle (ISO 639-2)
├── video_with_sub.en.srt          # English subtitle (ISO 639-1)
├── video_with_sub.english.srt     # English subtitle (full name)
├── video_with_sub.subgen.eng.srt  # Subgen-generated English subtitle
├── video_with_sub.forced.eng.srt  # Forced English subtitle
├── video_with_sub.jpn.srt         # Japanese subtitle
├── video_with_sub.eng.vtt         # WebVTT format
├── video_with_sub.eng.ass         # ASS format
├── movie.2024.mkv                 # Movie with year
├── movie.2024.eng.srt             # Matching subtitle
└── my.movie.title.mkv             # Movie with dots
    └── my.movie.title.eng.srt     # Matching subtitle
```

### Manual Testing

Not required for this story (can be fully unit tested with test fixtures).

---

## Implementation Details

### ExternalSubtitle Type (external_scanner.go)

```go
package skip

// ExternalSubtitle represents a subtitle file found in the same directory
type ExternalSubtitle struct {
	// FilePath is the full path to the subtitle file
	FilePath string
	// Language is the detected language code (ISO 639-2/639-1/name)
	Language string
	// IsSubgenGenerated indicates if "subgen" is in the filename
	IsSubgenGenerated bool
	// Format is the subtitle format extension (e.g., ".srt", ".vtt")
	Format string
}
```

### ExternalScanner Implementation (external_scanner.go)

```go
package skip

import (
	"os"
	"path/filepath"
	"strings"
)

// ExternalScanner scans directories for external subtitle files
type ExternalScanner struct {
	// supportedExtensions is the list of subtitle file extensions to scan for
	supportedExtensions []string
}

// NewExternalScanner creates a new ExternalScanner
func NewExternalScanner() *ExternalScanner {
	return &ExternalScanner{
		supportedExtensions: []string{
			".srt", ".vtt", ".sub", ".ass", ".ssa",
			".idx", ".sbv", ".pgs", ".ttml", ".lrc", ".smi",
		},
	}
}

// ScanForSubtitles scans the directory containing the media file for subtitle files
// Returns a list of ExternalSubtitle found
func (s *ExternalScanner) ScanForSubtitles(filePath string) ([]ExternalSubtitle, error) {
	if filePath == "" {
		return nil, fmt.Errorf("filePath cannot be empty")
	}

	// Get directory and base filename
	dir := filepath.Dir(filePath)
	baseName := getBaseFileName(filePath)

	// Read directory
	entries, err := os.ReadDir(dir)
	if err != nil {
		return nil, fmt.Errorf("failed to read directory: %w", err)
	}

	var subtitles []ExternalSubtitle

	// Scan for subtitle files
	for _, entry := range entries {
		// Skip directories
		if entry.IsDir() {
			continue
		}

		filename := entry.Name()
		fullPath := filepath.Join(dir, filename)

		// Check if file has subtitle extension
		if !s.isSubtitleFile(filename) {
			continue
		}

		// Check if filename starts with base video name
		if !s.matchesVideoFile(filename, baseName) {
			continue
		}

		// Parse language from filename
		language, hasLanguage := s.ParseLanguageFromFilename(filename)
		
		// Create ExternalSubtitle entry
		subtitle := ExternalSubtitle{
			FilePath:          fullPath,
			Language:          language,
			IsSubgenGenerated: s.IsSubgenGenerated(filename),
			Format:            filepath.Ext(filename),
		}

		subtitles = append(subtitles, subtitle)
	}

	return subtitles, nil
}

// ParseLanguageFromFilename extracts language code from subtitle filename
// Returns (language_code, true) if found, ("", false) otherwise
// Supports ISO 639-1 (en), ISO 639-2 (eng), and full names (english)
func (s *ExternalScanner) ParseLanguageFromFilename(filename string) (string, bool) {
	// Remove extension
	nameWithoutExt := strings.TrimSuffix(filename, filepath.Ext(filename))
	
	// Split by dots to get parts
	parts := strings.Split(nameWithoutExt, ".")
	
	// Check each part for language code
	for _, part := range parts {
		// Convert to lowercase for comparison
		partLower := strings.ToLower(part)
		
		// Skip common non-language parts
		if s.isNonLanguagePart(partLower) {
			continue
		}
		
		// Check if it's a valid language code
		if s.isValidLanguageCode(partLower) {
			return partLower, true
		}
	}
	
	return "", false
}

// HasLanguage checks if any subtitle matches the target language
func (s *ExternalScanner) HasLanguage(subtitles []ExternalSubtitle, targetLang string) bool {
	if targetLang == "" {
		return false
	}
	
	targetLower := strings.ToLower(targetLang)
	
	for _, subtitle := range subtitles {
		if subtitle.Language == "" {
			continue
		}
		
		// Match if languages are equal
		if strings.ToLower(subtitle.Language) == targetLower {
			return true
		}
		
		// Also try to match ISO 639-1 vs ISO 639-2
		// e.g., "en" matches "eng", "ja" matches "jpn"
		if s.languagesMatch(subtitle.Language, targetLang) {
			return true
		}
	}
	
	return false
}

// IsSubgenGenerated checks if "subgen" appears in the filename
func (s *ExternalScanner) IsSubgenGenerated(filename string) bool {
	// Remove extension
	nameWithoutExt := strings.TrimSuffix(filename, filepath.Ext(filename))
	
	// Split by dots and check each part
	parts := strings.Split(nameWithoutExt, ".")
	
	for _, part := range parts {
		if strings.ToLower(part) == "subgen" {
			return true
		}
	}
	
	return false
}

// Helper functions

// getBaseFileName returns the base filename without extension and path
func getBaseFileName(filePath string) string {
	filename := filepath.Base(filePath)
	return strings.TrimSuffix(filename, filepath.Ext(filename))
}

// isSubtitleFile checks if filename has a subtitle extension
func (s *ExternalScanner) isSubtitleFile(filename string) bool {
	ext := strings.ToLower(filepath.Ext(filename))
	
	for _, supportedExt := range s.supportedExtensions {
		if ext == supportedExt {
			return true
		}
	}
	
	return false
}

// matchesVideoFile checks if subtitle filename matches video filename
func (s *ExternalScanner) matchesVideoFile(subtitleName, videoBaseName string) bool {
	// Remove extension from subtitle
	subtitleBase := strings.TrimSuffix(subtitleName, filepath.Ext(subtitleName))
	
	// Subtitle should start with video base name
	return strings.HasPrefix(subtitleBase, videoBaseName)
}

// isNonLanguagePart checks if a part is not a language code
func (s *ExternalScanner) isNonLanguagePart(part string) bool {
	nonLanguageParts := []string{
		"subgen", "forced", "sdh", "cc", "hi", "full",
		"signs", "songs", "commentary", "default",
	}
	
	for _, nonLang := range nonLanguageParts {
		if part == nonLang {
			return true
		}
	}
	
	// Check if it's a number (track number)
	if _, err := strconv.Atoi(part); err == nil {
		return true
	}
	
	return false
}

// isValidLanguageCode checks if a string is a valid language code
// Supports ISO 639-1 (2 chars), ISO 639-2 (3 chars), and full names
func (s *ExternalScanner) isValidLanguageCode(code string) bool {
	// For now, simple heuristic:
	// - ISO 639-1: 2 letters (e.g., "en", "ja")
	// - ISO 639-2: 3 letters (e.g., "eng", "jpn")
	// - Full names: 4+ letters (e.g., "english", "japanese")
	
	// Must be at least 2 characters
	if len(code) < 2 {
		return false
	}
	
	// Must be alphabetic
	for _, ch := range code {
		if !('a' <= ch && ch <= 'z') {
			return false
		}
	}
	
	// Valid if 2-10 characters (reasonable language code length)
	return len(code) >= 2 && len(code) <= 10
}

// languagesMatch checks if two language codes refer to the same language
// Handles ISO 639-1 vs ISO 639-2 matching (e.g., "en" matches "eng")
func (s *ExternalScanner) languagesMatch(lang1, lang2 string) bool {
	// Simple mapping for common languages
	// In production, this should use a proper ISO 639 library
	mappings := map[string]string{
		"en":  "eng",
		"eng": "en",
		"ja":  "jpn",
		"jpn": "ja",
		"fr":  "fre",
		"fre": "fr",
		"de":  "ger",
		"ger": "de",
		"es":  "spa",
		"spa": "es",
		"it":  "ita",
		"ita": "it",
		"pt":  "por",
		"por": "pt",
		"ko":  "kor",
		"kor": "ko",
		"zh":  "chi",
		"chi": "zh",
		"ru":  "rus",
		"rus": "ru",
	}
	
	lang1Lower := strings.ToLower(lang1)
	lang2Lower := strings.ToLower(lang2)
	
	// Check if they're the same
	if lang1Lower == lang2Lower {
		return true
	}
	
	// Check mapping
	if mapped, ok := mappings[lang1Lower]; ok {
		if mapped == lang2Lower {
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
	SkipIfExternalSubtitlesExist    bool   // NEW: Skip if external subtitle exists
	SkipOnlySubgenSubtitles         bool   // NEW: Only skip subgen-generated subtitles
}

// Update NewConfig():
func NewConfig() (*Config, error) {
	// ... existing code ...
	
	// Skip if external subtitles exist (default: false)
	skipExternalStr := os.Getenv("SKIP_IF_EXTERNAL_SUBTITLES_EXIST")
	if skipExternalStr == "" {
		skipExternalStr = "false"
	}
	skipExternal, err := strconv.ParseBool(skipExternalStr)
	if err != nil {
		return nil, fmt.Errorf("invalid SKIP_IF_EXTERNAL_SUBTITLES_EXIST value: %w", err)
	}
	
	// Skip only subgen subtitles (default: false)
	skipOnlySubgenStr := os.Getenv("SKIP_ONLY_SUBGEN_SUBTITLES")
	if skipOnlySubgenStr == "" {
		skipOnlySubgenStr = "false"
	}
	skipOnlySubgen, err := strconv.ParseBool(skipOnlySubgenStr)
	if err != nil {
		return nil, fmt.Errorf("invalid SKIP_ONLY_SUBGEN_SUBTITLES value: %w", err)
	}
	
	return &Config{
		SkipIfTargetSubtitleExists:      skip,
		CheckEmbeddedSubtitles:          checkEmbedded,
		SkipIfInternalSubtitlesLanguage: skipInternalLang,
		SkipIfExternalSubtitlesExist:    skipExternal,
		SkipOnlySubgenSubtitles:         skipOnlySubgen,
	}, nil
}
```

### Integration into BasicChecker (basic_checker.go)

```go
// Add to BasicChecker struct:
type BasicChecker struct {
	config          *Config
	detector        *SubtitleDetector
	externalScanner *ExternalScanner // NEW
}

// Update NewBasicChecker:
func NewBasicChecker(config *Config) (*BasicChecker, error) {
	// ... existing validation ...
	
	return &BasicChecker{
		config:          config,
		detector:        NewSubtitleDetector(),
		externalScanner: NewExternalScanner(),
	}, nil
}

// Update Check method:
func (c *BasicChecker) Check(ctx context.Context, filePath string) (*CheckResult, error) {
	// ... existing checks (file existence, embedded subtitles) ...
	
	// Check for external subtitles (if enabled)
	if c.config.SkipIfExternalSubtitlesExist {
		subtitles, err := c.externalScanner.ScanForSubtitles(filePath)
		if err != nil {
			// Log error but don't fail the check
			// Continue with other checks
		} else {
			// Determine target language (for now, use config language)
			targetLang := c.config.SkipIfInternalSubtitlesLanguage
			
			// Filter subtitles if SKIP_ONLY_SUBGEN_SUBTITLES is enabled
			var filteredSubtitles []ExternalSubtitle
			if c.config.SkipOnlySubgenSubtitles {
				for _, sub := range subtitles {
					if sub.IsSubgenGenerated {
						filteredSubtitles = append(filteredSubtitles, sub)
					}
				}
			} else {
				filteredSubtitles = subtitles
			}
			
			// Check if any filtered subtitle matches target language
			if c.externalScanner.HasLanguage(filteredSubtitles, targetLang) {
				details := fmt.Sprintf("external subtitle found: language=%s", targetLang)
				if c.config.SkipOnlySubgenSubtitles {
					details += " (subgen-generated only)"
				}
				
				return &CheckResult{
					ShouldSkip: true,
					Reason:     ReasonExternalSubtitle,
					Details:    details,
				}, nil
			}
		}
	}
	
	// ... rest of existing logic ...
}
```

### Update Checker Constants (checker.go)

```go
const (
	// ... existing constants ...
	
	// ReasonExternalSubtitle indicates an external subtitle file was found
	ReasonExternalSubtitle SkipReason = "external_subtitle_exists"
)
```

---

## Definition of Done

- [x] Story file created with complete details
- [x] Tests written FIRST (must fail initially)
- [x] ExternalScanner implemented with directory scanning
- [x] Filename parsing implemented with language extraction
- [x] Support for 11 subtitle formats
- [x] Configuration updated with external subtitle options
- [x] BasicChecker extended to scan for external subtitles
- [x] All tests passing (unit tests)
- [x] Go build succeeds (type checking)
- [x] Integration points documented
- [x] Code follows Go best practices
- [x] Work log created in docs/WORKLOGS/
- [ ] Code committed

---

## References

- **Epic README**: docs/BACKLOG/EPIC_06/README.md lines 130-153
- **README-LLM.md**: Complete development guidelines
- **STORY_01**: docs/BACKLOG/EPIC_06/stories/STORY_01_basic_skip_logic.md
- **STORY_02**: docs/BACKLOG/EPIC_06/stories/STORY_02_embedded_subtitles.md
- **Original Implementation**: subgen.py lines 1729-1788 (has_subtitle_of_language_in_folder function)
- **Language Matching**: subgen.py lines 1786-1788 (is_valid_subtitle_language function)

---

**Created**: 2026-02-16  
**Last Updated**: 2026-02-16
