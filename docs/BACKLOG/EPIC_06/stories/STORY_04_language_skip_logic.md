# Story 04: Language-Based Skip Logic

**Epic**: EPIC_06  
**Status**: In Progress  
**Assignee**: Orchestrator Agent  
**Effort**: 8-10 hours  
**Priority**: MEDIUM

---

## User Story

As a **media server operator**,  
I want **Subgen to skip files based on subtitle or audio language criteria**,  
So that **I can filter transcription based on languages I want to avoid or target**.

---

## Acceptance Criteria

- [ ] Story file created with complete details
- [ ] Skip if subtitle in skip language list (`SKIP_SUBTITLE_LANGUAGES`)
- [ ] Skip if audio in skip language list (`SKIP_IF_AUDIO_LANGUAGES`)
- [ ] Skip if internal subtitle in specific language (already done in STORY_02)
- [ ] Audio track language detection via FFprobe
- [ ] Multiple language codes support (pipe-separated: "eng|jpn|kor")
- [ ] AudioDetector struct for detecting audio tracks
- [ ] Parse audio language codes from FFprobe output
- [ ] Configuration: `SKIP_SUBTITLE_LANGUAGES`, `SKIP_IF_AUDIO_LANGUAGES`
- [ ] Integration with existing Checker interface
- [ ] Comprehensive tests (happy/unhappy paths, edge cases)
- [ ] All tests passing (unit + integration)
- [ ] Type checking passes (Go build succeeds)
- [ ] Integration points documented
- [ ] Work log created

---

## Technical Design

### Approach

Implement language-based skip filtering with:

1. **Audio Track Detection**: Use FFprobe to detect audio track languages
2. **Language List Parsing**: Parse pipe-separated language lists (e.g., "eng|jpn|kor")
3. **Subtitle Language Filtering**: Skip if any subtitle matches skip language list
4. **Audio Language Filtering**: Skip if any audio track matches skip language list
5. **Configuration**: Add `SKIP_SUBTITLE_LANGUAGES` and `SKIP_IF_AUDIO_LANGUAGES` env vars
6. **Integration**: Extend BasicChecker to check language criteria

### Files to Create

- `orchestrator/internal/skip/language_filter.go` - Language filtering logic
  - `AudioDetector` struct for FFprobe integration
  - `GetAudioTracks(filePath string) ([]AudioTrack, error)` method
  - `ParseLanguageList(langStr string) []string` helper
  - `MatchesAnyLanguage(targetLang string, langList []string) bool` helper
  
- `orchestrator/internal/skip/language_filter_test.go` - Comprehensive tests
  - Happy path: detect English audio track
  - Happy path: detect multiple audio tracks with different languages
  - Happy path: parse pipe-separated language list
  - Happy path: match language against list
  - Unhappy path: FFprobe command fails
  - Unhappy path: invalid language list format
  - Unhappy path: no audio tracks found
  - Edge case: audio track with no language metadata
  - Edge case: empty language list
  - Edge case: ISO 639-1 vs 639-2 matching

### Files to Modify

- `orchestrator/internal/skip/checker.go` - Add skip reason constants
  - `ReasonSubtitleLanguageSkip` constant
  - `ReasonAudioLanguageSkip` constant
  
- `orchestrator/internal/skip/config.go` - Add configuration fields
  - `SkipSubtitleLanguages []string` (default: empty)
  - `SkipIfAudioLanguages []string` (default: empty)
  - Update `NewConfig()` to read env vars and parse language lists
  - Update `Validate()` if needed

- `orchestrator/internal/skip/basic_checker.go` - Integrate language filtering
  - Add `AudioDetector` field to `BasicChecker`
  - Update `Check()` method to check audio language filters
  - Check subtitle language against skip list

### Integration Points

**IMPLEMENTED: Skip Checker Module** (`orchestrator/internal/skip/`):
- ✅ Interface defined: `Checker` with `Check(ctx, filePath)` method
- ✅ `CheckResult` struct for skip decisions
- ✅ `BasicChecker` with file existence, embedded, and external checking
- ✅ Configuration system with environment variables

**NEW: Language-Based Filtering**:
- ⚠️ `AudioDetector` for FFprobe audio track detection
- ⚠️ Language list parsing (pipe-separated)
- ⚠️ Extended `CheckResult` with `ReasonSubtitleLanguageSkip`, `ReasonAudioLanguageSkip`
- ⚠️ Configuration for language filtering
- ⚠️ Integration into `BasicChecker`

**INTEGRATION NEEDED: Webhook Handlers**:
- ⏱️ Future story (will use existing `Checker` interface)

**Queue Module**:
- ✅ Not directly integrated (skip happens before enqueue)

**Observability**:
- ⏱️ Future: Add skip metrics for language filtering

---

## Testing Strategy

### Unit Tests

**AudioDetector Tests:**

**Happy Paths:**
1. **Detect single English audio track**: Returns 1 track with "eng" language
2. **Detect multiple audio tracks**: Returns all audio tracks with languages
3. **Detect various codecs**: Handles AAC, AC3, DTS, etc.
4. **Parse pipe-separated language list**: "eng|jpn|kor" → ["eng", "jpn", "kor"]
5. **Match language in list**: "eng" matches ["eng", "jpn", "kor"]
6. **Don't match language not in list**: "fre" doesn't match ["eng", "jpn", "kor"]

**Unhappy Paths:**
1. **FFprobe command fails**: Returns error
2. **Invalid JSON response**: Returns error
3. **No audio tracks found**: Returns empty slice
4. **Empty language list**: Never matches
5. **Empty file path**: Returns error

**Edge Cases:**
1. **Audio track with no language metadata**: Track with empty language field
2. **Audio track with multiple language tags**: Extracts primary language
3. **ISO 639-1 vs 639-2 matching**: "en" matches "eng"
4. **Case insensitive matching**: "ENG" matches "eng"
5. **Whitespace in language list**: "eng | jpn | kor" parsed correctly
6. **Empty parts in language list**: "eng||kor" handled gracefully

**Integration Tests:**
1. **Skip with audio language match**: CheckResult.ShouldSkip = true when audio is "eng" and skip list is ["eng"]
2. **Don't skip without audio language match**: CheckResult.ShouldSkip = false when audio is "jpn" and skip list is ["eng"]
3. **Skip with subtitle language match**: Skip when subtitle language in skip list
4. **Don't skip with language mismatch**: Don't skip when no languages match
5. **Disabled language filtering**: Never skip based on languages when lists are empty

### Test Data (JSON Fixtures)

```json
// testdata/ffprobe_audio_english.json
{
  "streams": [
    {
      "index": 0,
      "codec_type": "video",
      "codec_name": "h264"
    },
    {
      "index": 1,
      "codec_type": "audio",
      "codec_name": "aac",
      "tags": {
        "language": "eng",
        "title": "English"
      }
    }
  ]
}

// testdata/ffprobe_audio_multiple.json
{
  "streams": [
    {
      "index": 0,
      "codec_type": "video",
      "codec_name": "h264"
    },
    {
      "index": 1,
      "codec_type": "audio",
      "codec_name": "aac",
      "tags": {
        "language": "jpn",
        "title": "Japanese"
      }
    },
    {
      "index": 2,
      "codec_type": "audio",
      "codec_name": "ac3",
      "tags": {
        "language": "eng",
        "title": "English"
      }
    }
  ]
}

// testdata/ffprobe_audio_no_language.json
{
  "streams": [
    {
      "index": 0,
      "codec_type": "video",
      "codec_name": "h264"
    },
    {
      "index": 1,
      "codec_type": "audio",
      "codec_name": "aac"
    }
  ]
}
```

### Manual Testing

Not required for this story (can be fully mocked in unit tests).

---

## Implementation Details

### AudioTrack Type (language_filter.go)

```go
package skip

// AudioTrack represents an audio track detected in a media file
type AudioTrack struct {
	// Index is the stream index in the container
	Index int
	// Language is the ISO 639-2/639-1 language code (e.g., "eng", "jpn")
	Language string
	// Title is the audio track title/description
	Title string
	// Codec is the audio codec name (e.g., "aac", "ac3", "dts")
	Codec string
	// Channels is the number of audio channels
	Channels int
}
```

### AudioDetector Implementation (language_filter.go)

```go
package skip

import (
	"context"
	"encoding/json"
	"fmt"
	"os/exec"
	"strings"
)

// AudioDetector detects audio tracks using FFprobe
type AudioDetector struct {
	ffprobePath string
}

// NewAudioDetector creates a new AudioDetector
func NewAudioDetector() *AudioDetector {
	return &AudioDetector{
		ffprobePath: "ffprobe", // Assumes ffprobe is in PATH
	}
}

// GetAudioTracks detects audio tracks in a media file
func (d *AudioDetector) GetAudioTracks(ctx context.Context, filePath string) ([]AudioTrack, error) {
	if filePath == "" {
		return nil, fmt.Errorf("filePath cannot be empty")
	}

	// Run FFprobe
	output, err := d.runFFprobe(ctx, filePath)
	if err != nil {
		return nil, fmt.Errorf("ffprobe command failed: %w", err)
	}

	// Parse JSON output
	probe, err := parseFFprobeOutput(output)
	if err != nil {
		return nil, fmt.Errorf("failed to parse ffprobe output: %w", err)
	}

	// Extract audio tracks
	tracks := d.extractAudioTracks(probe)

	return tracks, nil
}

// runFFprobe executes FFprobe and returns the JSON output
func (d *AudioDetector) runFFprobe(ctx context.Context, filePath string) ([]byte, error) {
	cmd := exec.CommandContext(ctx, d.ffprobePath,
		"-v", "quiet",
		"-print_format", "json",
		"-show_streams",
		"-select_streams", "a", // Select only audio streams
		filePath,
	)

	output, err := cmd.Output()
	if err != nil {
		return nil, fmt.Errorf("ffprobe execution error: %w", err)
	}

	return output, nil
}

// extractAudioTracks extracts audio track information from FFprobe output
func (d *AudioDetector) extractAudioTracks(probe *FFProbeOutput) []AudioTrack {
	var tracks []AudioTrack

	for _, stream := range probe.Streams {
		if stream.CodecType == "audio" {
			tracks = append(tracks, AudioTrack{
				Index:    stream.Index,
				Language: stream.Tags.Language,
				Title:    stream.Tags.Title,
				Codec:    stream.CodecName,
				Channels: stream.Channels,
			})
		}
	}

	return tracks
}

// HasLanguage checks if any audio track matches the given language
func (d *AudioDetector) HasLanguage(tracks []AudioTrack, language string) bool {
	if language == "" {
		return false
	}

	for _, track := range tracks {
		if track.Language == "" {
			continue
		}

		// Exact match
		if strings.EqualFold(track.Language, language) {
			return true
		}

		// ISO 639 code translation (en <-> eng, ja <-> jpn, etc.)
		if languagesMatch(track.Language, language) {
			return true
		}
	}

	return false
}

// ParseLanguageList parses a pipe-separated language list
// "eng|jpn|kor" → ["eng", "jpn", "kor"]
// Handles whitespace: "eng | jpn | kor" → ["eng", "jpn", "kor"]
func ParseLanguageList(langStr string) []string {
	if langStr == "" {
		return nil
	}

	parts := strings.Split(langStr, "|")
	var languages []string

	for _, part := range parts {
		trimmed := strings.TrimSpace(part)
		if trimmed != "" {
			languages = append(languages, strings.ToLower(trimmed))
		}
	}

	return languages
}

// MatchesAnyLanguage checks if a language matches any language in the list
// Supports ISO 639-1 vs 639-2 matching (e.g., "en" matches "eng")
func MatchesAnyLanguage(targetLang string, langList []string) bool {
	if targetLang == "" || len(langList) == 0 {
		return false
	}

	targetLower := strings.ToLower(targetLang)

	for _, lang := range langList {
		langLower := strings.ToLower(lang)

		// Exact match
		if targetLower == langLower {
			return true
		}

		// ISO 639 code translation
		if languagesMatch(targetLower, langLower) {
			return true
		}
	}

	return false
}

// languagesMatch checks if two language codes refer to the same language
// Handles ISO 639-1 vs ISO 639-2 matching (e.g., "en" matches "eng")
func languagesMatch(lang1, lang2 string) bool {
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

### FFprobe Types Updates (ffprobe_types.go)

```go
// Add to FFProbeStream struct:
type FFProbeStream struct {
	Index     int               `json:"index"`
	CodecType string            `json:"codec_type"`
	CodecName string            `json:"codec_name"`
	Tags      FFProbeStreamTags `json:"tags,omitempty"`
	Channels  int               `json:"channels,omitempty"` // NEW: Audio channel count
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
	SkipSubtitleLanguages           []string // NEW: List of subtitle languages to skip
	SkipIfAudioLanguages            []string // NEW: List of audio languages to skip
}

// Update NewConfig():
func NewConfig() (*Config, error) {
	// ... existing code ...

	// Skip subtitle languages (default: empty)
	skipSubLangStr := os.Getenv("SKIP_SUBTITLE_LANGUAGES")
	skipSubLangs := ParseLanguageList(skipSubLangStr)

	// Skip if audio languages (default: empty)
	skipAudioLangStr := os.Getenv("SKIP_IF_AUDIO_LANGUAGES")
	skipAudioLangs := ParseLanguageList(skipAudioLangStr)

	return &Config{
		SkipIfTargetSubtitleExists:      skip,
		CheckEmbeddedSubtitles:          checkEmbedded,
		SkipIfInternalSubtitlesLanguage: skipInternalLang,
		SkipIfExternalSubtitlesExist:    skipExternal,
		SkipOnlySubgenSubtitles:         skipOnlySubgen,
		SkipSubtitleLanguages:           skipSubLangs,
		SkipIfAudioLanguages:            skipAudioLangs,
	}, nil
}
```

### Integration into BasicChecker (basic_checker.go)

```go
// Add to BasicChecker struct:
type BasicChecker struct {
	config          *Config
	detector        *SubtitleDetector
	externalScanner *ExternalScanner
	audioDetector   *AudioDetector // NEW
}

// Update NewBasicChecker:
func NewBasicChecker(config *Config) (*BasicChecker, error) {
	// ... existing validation ...

	return &BasicChecker{
		config:          config,
		detector:        NewSubtitleDetector(),
		externalScanner: NewExternalScanner(),
		audioDetector:   NewAudioDetector(),
	}, nil
}

// Update Check method:
func (c *BasicChecker) Check(ctx context.Context, filePath string) (*CheckResult, error) {
	// ... existing checks (file existence, embedded, external) ...

	// Check audio language filtering (if enabled)
	if len(c.config.SkipIfAudioLanguages) > 0 && isVideoFile(filePath) {
		audioTracks, err := c.audioDetector.GetAudioTracks(ctx, filePath)
		if err != nil {
			// Log error but don't fail the check
			// Continue with other checks
		} else {
			for _, track := range audioTracks {
				if MatchesAnyLanguage(track.Language, c.config.SkipIfAudioLanguages) {
					return &CheckResult{
						ShouldSkip: true,
						Reason:     ReasonAudioLanguageSkip,
						Details:    fmt.Sprintf("audio track language matches skip list: %s", track.Language),
					}, nil
				}
			}
		}
	}

	// Check subtitle language filtering (if enabled)
	if len(c.config.SkipSubtitleLanguages) > 0 {
		// Check embedded subtitles
		if c.config.CheckEmbeddedSubtitles && isVideoFile(filePath) {
			tracks, err := c.detector.GetEmbeddedSubtitles(ctx, filePath)
			if err == nil {
				for _, track := range tracks {
					if MatchesAnyLanguage(track.Language, c.config.SkipSubtitleLanguages) {
						return &CheckResult{
							ShouldSkip: true,
							Reason:     ReasonSubtitleLanguageSkip,
							Details:    fmt.Sprintf("embedded subtitle language matches skip list: %s", track.Language),
						}, nil
					}
				}
			}
		}

		// Check external subtitles
		subtitles, err := c.externalScanner.ScanForSubtitles(filePath)
		if err == nil {
			for _, sub := range subtitles {
				if MatchesAnyLanguage(sub.Language, c.config.SkipSubtitleLanguages) {
					return &CheckResult{
						ShouldSkip: true,
						Reason:     ReasonSubtitleLanguageSkip,
						Details:    fmt.Sprintf("external subtitle language matches skip list: %s", sub.Language),
					}, nil
				}
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

	// ReasonSubtitleLanguageSkip indicates subtitle language is in skip list
	ReasonSubtitleLanguageSkip SkipReason = "subtitle_language_in_skip_list"
	// ReasonAudioLanguageSkip indicates audio language is in skip list
	ReasonAudioLanguageSkip SkipReason = "audio_language_in_skip_list"
)
```

---

## Definition of Done

- [ ] Story file created with complete details
- [ ] Tests written FIRST (must fail initially)
- [ ] AudioDetector implemented with FFprobe integration
- [ ] Language list parsing implemented
- [ ] Configuration updated with language filtering options
- [ ] BasicChecker extended to check language criteria
- [ ] All tests passing (unit + integration)
- [ ] Go build succeeds (type checking)
- [ ] Integration points documented
- [ ] Code follows Go best practices
- [ ] Work log created in docs/WORKLOGS/
- [ ] Code committed

---

## References

- **Epic README**: docs/BACKLOG/EPIC_06/README.md lines 154-174
- **README-LLM.md**: Complete development guidelines
- **STORY_01**: docs/BACKLOG/EPIC_06/stories/STORY_01_basic_skip_logic.md
- **STORY_02**: docs/BACKLOG/EPIC_06/stories/STORY_02_embedded_subtitles.md
- **STORY_03**: docs/BACKLOG/EPIC_06/stories/STORY_03_external_subtitle_scan.md
- **Original Implementation**: 
  - subgen.py lines 1564-1632 (should_skip_file function)
  - subgen.py lines 1660-1668 (get_audio_languages function)

---

**Created**: 2026-02-15  
**Last Updated**: 2026-02-15
