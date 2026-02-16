# Story 02: Embedded Subtitle Detection

**Epic**: EPIC_06  
**Status**: Complete  
**Assignee**: Delegation Agent  
**Effort**: 10-12 hours (Actual: ~4 hours)  
**Priority**: HIGH

---

## User Story

As a **media server operator**,  
I want **Subgen to detect subtitles embedded in video files**,  
So that **I don't waste compute resources transcribing files that already have subtitles in the container**.

---

## Acceptance Criteria

- [x] Story file created with complete details
- [x] FFprobe integration to detect embedded subtitle tracks
- [x] Extract subtitle track metadata (language, codec, title, index)
- [x] Skip if embedded subtitle matches target language
- [x] Configuration: Check embedded subtitles by default
- [x] Support all common subtitle formats (SRT, SSA, ASS, PGS, VOBSUB, etc.)
- [x] FFprobe types for JSON parsing (streams, tags, format)
- [x] SubtitleDetector implements detection logic
- [x] Comprehensive tests (unit tests with mocked FFprobe responses)
- [x] Integration with existing Checker interface
- [x] All tests passing (unit + integration)
- [x] Type checking passes (Go build succeeds)
- [x] Integration points documented
- [x] Work log created

---

## Technical Design

### Approach

Implement FFprobe-based embedded subtitle detection with:

1. **FFprobe Integration**: Execute FFprobe to extract subtitle stream metadata
2. **JSON Parsing**: Parse FFprobe JSON output into Go structs
3. **Subtitle Detection**: Identify subtitle tracks and extract language codes
4. **Skip Logic Extension**: Add embedded subtitle checking to skip checker
5. **Configuration**: Enable embedded subtitle checking by default
6. **Testing**: Mock FFprobe responses for comprehensive testing

### Files to Create

- `orchestrator/internal/skip/embedded_detector.go` - FFprobe integration
  - `SubtitleDetector` struct
  - `GetEmbeddedSubtitles(filePath string) ([]SubtitleTrack, error)` method
  - `runFFprobe(filePath string) ([]byte, error)` helper
  - `parseFFprobeOutput(output []byte) (*FFProbeOutput, error)` helper
  
- `orchestrator/internal/skip/ffprobe_types.go` - FFprobe JSON types
  - `FFProbeOutput` struct (top-level response)
  - `FFProbeStream` struct (stream information)
  - `FFProbeStreamTags` struct (stream tags)
  - `SubtitleTrack` struct (parsed subtitle info)
  
- `orchestrator/internal/skip/embedded_detector_test.go` - Comprehensive tests
  - Mock FFprobe responses (JSON fixtures)
  - Happy path: detect embedded English subtitle
  - Happy path: detect multiple subtitle tracks
  - Happy path: detect various subtitle codecs (SRT, ASS, PGS, etc.)
  - Unhappy path: FFprobe command fails
  - Unhappy path: invalid JSON response
  - Unhappy path: no subtitle tracks found
  - Edge case: subtitle track with no language metadata
  - Edge case: subtitle track with multiple language tags

### Files to Modify

- `orchestrator/internal/skip/checker.go` - Add skip reason constants
  - `ReasonEmbeddedSubtitle` constant
  
- `orchestrator/internal/skip/config.go` - Add configuration fields
  - `CheckEmbeddedSubtitles bool` (default: true)
  - `SkipIfInternalSubtitlesLanguage string` (default: "eng")
  - Update `NewConfig()` to read env vars
  - Update `Validate()` if needed

- `orchestrator/internal/skip/basic_checker.go` - Integrate embedded checking
  - Add `SubtitleDetector` field to `BasicChecker`
  - Update `Check()` method to call embedded subtitle detection
  - OR create new `EmbeddedChecker` that wraps `BasicChecker`

### Integration Points

**IMPLEMENTED: Skip Checker Module** (`orchestrator/internal/skip/`):
- ✅ Interface defined: `Checker` with `Check(ctx, filePath)` method
- ✅ `CheckResult` struct for skip decisions
- ✅ `BasicChecker` for file existence checks

**NEW: Embedded Subtitle Detection**:
- ⚠️ `SubtitleDetector` for FFprobe integration
- ⚠️ Extended `CheckResult` with `ReasonEmbeddedSubtitle`
- ⚠️ Configuration for embedded subtitle checking
- ⚠️ Integration into `BasicChecker` or new `EmbeddedChecker`

**INTEGRATION NEEDED: Webhook Handlers**:
- ⏱️ Future story (will use existing `Checker` interface)

**Queue Module**:
- ✅ Not directly integrated (skip happens before enqueue)

**Observability**:
- ⏱️ Future: Add skip metrics for embedded subtitles

---

## Testing Strategy

### Unit Tests

**FFprobe Types Tests:**
1. **Parse valid FFprobe JSON**: Unmarshal sample FFprobe output
2. **Parse multiple subtitle streams**: JSON with 3+ subtitle tracks
3. **Parse various codecs**: SRT, SSA, ASS, PGS, VOBSUB, SUBRIP, etc.
4. **Parse with missing fields**: Gracefully handle missing tags
5. **Parse invalid JSON**: Return error for malformed JSON

**SubtitleDetector Tests (Mocked FFprobe):**
1. **Detect single English subtitle**: Returns 1 track with "eng" language
2. **Detect multiple subtitles**: Returns all subtitle tracks
3. **Detect various codecs**: Handles SRT, ASS, PGS, etc.
4. **No subtitles found**: Returns empty slice, no error
5. **FFprobe command fails**: Returns error
6. **FFprobe returns invalid JSON**: Returns error
7. **Subtitle with no language**: Track with empty language field
8. **Subtitle with multiple language tags**: Extracts primary language

**Integration Tests:**
1. **Skip with embedded English subtitle**: CheckResult.ShouldSkip = true
2. **Don't skip without embedded subtitle**: CheckResult.ShouldSkip = false
3. **Skip with configured language match**: Respects SKIP_IF_INTERNAL_SUBTITLES_LANGUAGE
4. **Don't skip with language mismatch**: English subtitle, looking for Japanese
5. **Disabled embedded checking**: Never skip based on embedded subs

### Test Data (JSON Fixtures)

```json
// testdata/ffprobe_with_subtitle.json
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
    },
    {
      "index": 2,
      "codec_type": "subtitle",
      "codec_name": "subrip",
      "tags": {
        "language": "eng",
        "title": "English"
      }
    }
  ]
}

// testdata/ffprobe_multiple_subtitles.json
{
  "streams": [
    {
      "index": 0,
      "codec_type": "video",
      "codec_name": "h264"
    },
    {
      "index": 1,
      "codec_type": "subtitle",
      "codec_name": "ass",
      "tags": {
        "language": "eng",
        "title": "English (Full)"
      }
    },
    {
      "index": 2,
      "codec_type": "subtitle",
      "codec_name": "subrip",
      "tags": {
        "language": "jpn",
        "title": "Japanese"
      }
    },
    {
      "index": 3,
      "codec_type": "subtitle",
      "codec_name": "hdmv_pgs_subtitle",
      "tags": {
        "language": "eng",
        "title": "English (Signs/Songs)"
      }
    }
  ]
}

// testdata/ffprobe_no_subtitles.json
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

Not required for this story (FFprobe can be fully mocked in unit tests).

---

## Implementation Details

### FFprobe Types (ffprobe_types.go)

```go
package skip

// FFProbeOutput represents the top-level FFprobe JSON response
type FFProbeOutput struct {
	Streams []FFProbeStream `json:"streams"`
}

// FFProbeStream represents a single stream in the FFprobe output
type FFProbeStream struct {
	Index     int              `json:"index"`
	CodecType string           `json:"codec_type"`
	CodecName string           `json:"codec_name"`
	Tags      FFProbeStreamTags `json:"tags,omitempty"`
}

// FFProbeStreamTags represents stream metadata tags
type FFProbeStreamTags struct {
	Language string `json:"language,omitempty"`
	Title    string `json:"title,omitempty"`
}

// SubtitleTrack represents a parsed subtitle track
type SubtitleTrack struct {
	Index    int    // Stream index
	Language string // ISO 639-2 language code (e.g., "eng", "jpn")
	Title    string // Subtitle title/description
	Codec    string // Codec name (e.g., "subrip", "ass", "hdmv_pgs_subtitle")
}
```

### Subtitle Detector (embedded_detector.go)

```go
package skip

import (
	"context"
	"encoding/json"
	"fmt"
	"os/exec"
)

// SubtitleDetector detects embedded subtitles using FFprobe
type SubtitleDetector struct {
	ffprobePath string
}

// NewSubtitleDetector creates a new SubtitleDetector
func NewSubtitleDetector() *SubtitleDetector {
	return &SubtitleDetector{
		ffprobePath: "ffprobe", // Assumes ffprobe is in PATH
	}
}

// GetEmbeddedSubtitles detects embedded subtitle tracks in a media file
func (d *SubtitleDetector) GetEmbeddedSubtitles(ctx context.Context, filePath string) ([]SubtitleTrack, error) {
	if filePath == "" {
		return nil, fmt.Errorf("filePath cannot be empty")
	}

	// Run FFprobe
	output, err := d.runFFprobe(ctx, filePath)
	if err != nil {
		return nil, fmt.Errorf("ffprobe command failed: %w", err)
	}

	// Parse JSON output
	probe, err := d.parseFFprobeOutput(output)
	if err != nil {
		return nil, fmt.Errorf("failed to parse ffprobe output: %w", err)
	}

	// Extract subtitle tracks
	tracks := d.extractSubtitleTracks(probe)

	return tracks, nil
}

// runFFprobe executes FFprobe and returns the JSON output
func (d *SubtitleDetector) runFFprobe(ctx context.Context, filePath string) ([]byte, error) {
	cmd := exec.CommandContext(ctx, d.ffprobePath,
		"-v", "quiet",
		"-print_format", "json",
		"-show_streams",
		"-select_streams", "s", // Select only subtitle streams
		filePath,
	)

	output, err := cmd.Output()
	if err != nil {
		return nil, fmt.Errorf("ffprobe execution error: %w", err)
	}

	return output, nil
}

// parseFFprobeOutput parses FFprobe JSON output
func (d *SubtitleDetector) parseFFprobeOutput(output []byte) (*FFProbeOutput, error) {
	var probe FFProbeOutput
	if err := json.Unmarshal(output, &probe); err != nil {
		return nil, fmt.Errorf("json unmarshal error: %w", err)
	}

	return &probe, nil
}

// extractSubtitleTracks extracts subtitle track information from FFprobe output
func (d *SubtitleDetector) extractSubtitleTracks(probe *FFProbeOutput) []SubtitleTrack {
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

	return tracks
}

// HasLanguage checks if any subtitle track matches the given language
func (d *SubtitleDetector) HasLanguage(tracks []SubtitleTrack, language string) bool {
	if language == "" {
		return false
	}

	for _, track := range tracks {
		if track.Language == language {
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
	CheckEmbeddedSubtitles          bool   // NEW: Check for embedded subtitles
	SkipIfInternalSubtitlesLanguage string // NEW: Language to skip if embedded (default: "eng")
}

// Update NewConfig():
func NewConfig() (*Config, error) {
	// ... existing code ...
	
	// Check embedded subtitles (default: true)
	checkEmbeddedStr := os.Getenv("CHECK_EMBEDDED_SUBTITLES")
	if checkEmbeddedStr == "" {
		checkEmbeddedStr = "true"
	}
	checkEmbedded, err := strconv.ParseBool(checkEmbeddedStr)
	if err != nil {
		return nil, fmt.Errorf("invalid CHECK_EMBEDDED_SUBTITLES value: %w", err)
	}
	
	// Skip if internal subtitles language
	skipInternalLang := os.Getenv("SKIP_IF_INTERNAL_SUBTITLES_LANGUAGE")
	if skipInternalLang == "" {
		skipInternalLang = "eng" // Default to English
	}
	
	return &Config{
		SkipIfTargetSubtitleExists:      skip,
		CheckEmbeddedSubtitles:          checkEmbedded,
		SkipIfInternalSubtitlesLanguage: skipInternalLang,
	}, nil
}
```

### Integration into BasicChecker (basic_checker.go)

```go
// Add to BasicChecker struct:
type BasicChecker struct {
	config   *Config
	detector *SubtitleDetector // NEW
}

// Update NewBasicChecker:
func NewBasicChecker(config *Config) (*BasicChecker, error) {
	// ... existing validation ...
	
	return &BasicChecker{
		config:   config,
		detector: NewSubtitleDetector(),
	}, nil
}

// Update Check method:
func (c *BasicChecker) Check(ctx context.Context, filePath string) (*CheckResult, error) {
	// ... existing file existence checks ...
	
	// Check for embedded subtitles (if enabled)
	if c.config.CheckEmbeddedSubtitles && isVideoFile(filePath) {
		tracks, err := c.detector.GetEmbeddedSubtitles(ctx, filePath)
		if err != nil {
			// Log error but don't fail the check
			// (FFprobe might not be available or file might be corrupted)
			// Continue with other checks
		} else if c.detector.HasLanguage(tracks, c.config.SkipIfInternalSubtitlesLanguage) {
			return &CheckResult{
				ShouldSkip: true,
				Reason:     ReasonEmbeddedSubtitle,
				Details:    fmt.Sprintf("embedded subtitle found: language=%s", c.config.SkipIfInternalSubtitlesLanguage),
			}, nil
		}
	}
	
	// ... rest of existing logic ...
}

// Add helper:
func isVideoFile(filePath string) bool {
	ext := strings.ToLower(filepath.Ext(filePath))
	videoExts := []string{".mkv", ".mp4", ".avi", ".mov", ".m4v", ".wmv", ".flv", ".webm", ".ts", ".m2ts"}
	
	for _, videoExt := range videoExts {
		if ext == videoExt {
			return true
		}
	}
	
	return false
}
```

### Update Checker Constants (checker.go)

```go
const (
	// ... existing constants ...
	
	// ReasonEmbeddedSubtitle indicates an embedded subtitle was found
	ReasonEmbeddedSubtitle SkipReason = "embedded_subtitle_exists"
)
```

---

## Definition of Done

- [x] Story file created with full details
- [x] Tests written FIRST (must fail initially)
- [x] FFprobe types implemented (JSON unmarshaling)
- [x] SubtitleDetector implemented with FFprobe integration
- [x] Configuration updated with embedded subtitle options
- [x] BasicChecker extended to check embedded subtitles
- [x] All tests passing (unit + integration)
- [x] Go build succeeds (type checking)
- [x] Integration points documented
- [x] Code follows Go best practices
- [x] Work log created in docs/WORKLOGS/
- [x] Code committed

---

## References

- **Epic README**: docs/BACKLOG/EPIC_06/README.md lines 84-128
- **README-LLM.md**: Complete development guidelines
- **STORY_01**: docs/BACKLOG/EPIC_06/stories/STORY_01_basic_skip_logic.md
- **Original Implementation**: subgen.py lines 1686-1727 (has_subtitle_language_in_file function)
- **FFprobe Documentation**: https://ffmpeg.org/ffprobe.html

---

**Created**: 2026-02-16  
**Last Updated**: 2026-02-16
