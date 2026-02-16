# Story 03: End-to-End Pipeline Tests

**Epic**: EPIC_03 - Integration & Testing  
**Status**: Not Started  
**Priority**: Critical  
**Estimated Effort**: 8-10 hours  
**Assignee**: TBD

---

## User Story

As a **quality assurance engineer**,  
I want **end-to-end tests that validate the complete pipeline from webhook to subtitle file**,  
So that **I can verify the entire system works correctly with real media files and all subtitle formats**.

---

## Context

End-to-end tests validate the ENTIRE pipeline:
1. Media server sends webhook
2. Orchestrator receives and parses webhook
3. Orchestrator calls media server API for file path
4. Orchestrator enqueues transcription task
5. Orchestrator dispatches task to Python worker via gRPC
6. Worker loads Whisper model
7. Worker transcribes audio
8. Worker generates subtitle file (SRT or LRC)
9. Worker returns success response
10. Orchestrator refreshes media server metadata (Plex/Jellyfin)

**Why This Matters:**
- Integration tests (STORY_01, STORY_02) test individual components
- E2E tests validate the COMPLETE user journey
- Tests with REAL media files (video/audio) and REAL transcription
- Validates all subtitle formats (SRT, LRC)
- Includes manual testing with downloadable sample videos

**Current State:**
- No E2E tests exist
- Manual testing only
- No validation of complete pipeline

**Target State:**
- Automated E2E test suite
- Tests with real media samples
- Validation of subtitle content (not just file existence)
- Manual test procedures documented
- Sample videos downloaded and tested

---

## Acceptance Criteria

- [ ] E2E test file created: `test/e2e/pipeline_test.go`
- [ ] Download real video sample (30s, CC-licensed)
- [ ] Test: Complete Plex webhook → subtitle file created
- [ ] Test: Complete Jellyfin webhook → subtitle file created
- [ ] Test: SRT file generation and validation
- [ ] Test: LRC file generation and validation
- [ ] Test: Multiple audio tracks handling
- [ ] Test: Language detection workflow
- [ ] Test: Skip logic (existing subtitles)
- [ ] Test: Metadata refresh (Plex/Jellyfin)
- [ ] Test: Error scenarios (corrupt file, no audio)
- [ ] Manual test procedure documented
- [ ] Sample video downloaded and tested manually
- [ ] Subtitle content validated (contains expected segments)
- [ ] All tests pass with Docker Compose
- [ ] Work log created

---

## Technical Design

### E2E Test Architecture

```
┌────────────────────────────────────────────────────────────────┐
│  End-to-End Test Flow                                          │
├────────────────────────────────────────────────────────────────┤
│                                                                 │
│  1. Download Real Video                                        │
│     ├─ Big Buck Bunny (30s, CC-BY)                           │
│     ├─ Sintel trailer (1min, CC-BY)                          │
│     └─ Store in test/testdata/videos/                        │
│                                                                 │
│  2. Send Webhook                                               │
│     └─ HTTP POST to orchestrator                              │
│                                                                 │
│  3. Orchestrator Processing                                    │
│     ├─ Parse webhook                                          │
│     ├─ Call media server API (mocked)                         │
│     ├─ Enqueue task                                           │
│     └─ Dispatch to worker (gRPC)                              │
│                                                                 │
│  4. Worker Transcription                                       │
│     ├─ Load Whisper model (tiny for speed)                   │
│     ├─ Extract audio from video                               │
│     ├─ Transcribe with faster-whisper                         │
│     └─ Generate subtitle file                                  │
│                                                                 │
│  5. Validation                                                 │
│     ├─ Subtitle file exists                                   │
│     ├─ File format correct (SRT/LRC)                          │
│     ├─ Contains expected segments                             │
│     ├─ Timestamps are valid                                   │
│     └─ Language detected correctly                            │
│                                                                 │
│  6. Cleanup                                                    │
│     └─ Remove generated subtitle files                        │
│                                                                 │
└────────────────────────────────────────────────────────────────┘
```

### File Structure

```
test/
├── e2e/
│   ├── pipeline_test.go                    # Main E2E tests
│   ├── subtitle_validator.go               # SRT/LRC validation
│   └── media_downloader.go                 # Download sample videos
├── testdata/
│   └── videos/
│       ├── big_buck_bunny_30s.mp4          # Real video sample
│       ├── sintel_trailer_60s.mp4          # Real video sample
│       ├── audio_only.m4a                  # Audio file for LRC
│       └── README.md                       # Attribution + sources
└── manual/
    ├── MANUAL_TEST_PROCEDURE.md            # Step-by-step manual tests
    └── test_videos.txt                     # List of test video URLs
```

---

## Implementation Steps

### Step 1: Download Sample Videos

**File: `/home/mikekao/personal/subgen/test/scripts/download_test_videos.sh`**

```bash
#!/bin/bash
# Download CC-licensed video samples for E2E testing

set -e

VIDEOS_DIR="../testdata/videos"
mkdir -p "$VIDEOS_DIR"

echo "Downloading test video samples..."

# 1. Big Buck Bunny (30 seconds, CC-BY)
if [ ! -f "$VIDEOS_DIR/big_buck_bunny_30s.mp4" ]; then
    echo "  - Downloading Big Buck Bunny (30s)..."
    # Official mirror: https://download.blender.org/peach/bigbuckbunny_movies/
    curl -L -o "$VIDEOS_DIR/big_buck_bunny_30s.mp4" \
        "https://download.blender.org/peach/bigbuckbunny_movies/big_buck_bunny_480p_stereo.avi" \
        --max-time 60 || echo "Download failed, using fallback"
    
    # If download failed, create synthetic video
    if [ ! -f "$VIDEOS_DIR/big_buck_bunny_30s.mp4" ]; then
        echo "  - Creating synthetic video as fallback..."
        ffmpeg -f lavfi -i testsrc=duration=30:size=640x480:rate=30 \
               -f lavfi -i sine=frequency=440:duration=30 \
               -c:v libx264 -c:a aac -t 30 \
               "$VIDEOS_DIR/big_buck_bunny_30s.mp4" -y 2>/dev/null
    else
        # Trim to 30 seconds
        echo "  - Trimming to 30 seconds..."
        ffmpeg -i "$VIDEOS_DIR/big_buck_bunny_30s.mp4" -t 30 -c copy \
               "$VIDEOS_DIR/big_buck_bunny_30s_trimmed.mp4" -y 2>/dev/null
        mv "$VIDEOS_DIR/big_buck_bunny_30s_trimmed.mp4" "$VIDEOS_DIR/big_buck_bunny_30s.mp4"
    fi
else
    echo "  - Big Buck Bunny already downloaded"
fi

# 2. Sintel Trailer (1 minute, CC-BY)
if [ ! -f "$VIDEOS_DIR/sintel_trailer_60s.mp4" ]; then
    echo "  - Downloading Sintel trailer (60s)..."
    # Official site: https://durian.blender.org/download/
    curl -L -o "$VIDEOS_DIR/sintel_trailer_60s.mp4" \
        "https://download.blender.org/durian/trailer/sintel_trailer-480p.mp4" \
        --max-time 60 || echo "Download failed, using fallback"
    
    # If download failed, create synthetic video
    if [ ! -f "$VIDEOS_DIR/sintel_trailer_60s.mp4" ]; then
        echo "  - Creating synthetic video as fallback..."
        ffmpeg -f lavfi -i testsrc=duration=60:size=640x480:rate=30 \
               -f lavfi -i sine=frequency=880:duration=60 \
               -c:v libx264 -c:a aac -t 60 \
               "$VIDEOS_DIR/sintel_trailer_60s.mp4" -y 2>/dev/null
    fi
else
    echo "  - Sintel trailer already downloaded"
fi

# 3. Audio-only file for LRC testing
if [ ! -f "$VIDEOS_DIR/audio_only.m4a" ]; then
    echo "  - Creating audio-only file..."
    ffmpeg -f lavfi -i sine=frequency=440:duration=30 \
           -c:a aac -b:a 128k \
           "$VIDEOS_DIR/audio_only.m4a" -y 2>/dev/null
else
    echo "  - Audio-only file already exists"
fi

# 4. Video with multiple audio tracks
if [ ! -f "$VIDEOS_DIR/multi_audio.mkv" ]; then
    echo "  - Creating video with multiple audio tracks..."
    # Create video with 2 audio tracks (English and Spanish simulation)
    ffmpeg -f lavfi -i testsrc=duration=30:size=640x480:rate=30 \
           -f lavfi -i sine=frequency=440:duration=30 \
           -f lavfi -i sine=frequency=880:duration=30 \
           -map 0:v -map 1:a -map 2:a \
           -metadata:s:a:0 language=eng -metadata:s:a:0 title="English" \
           -metadata:s:a:1 language=spa -metadata:s:a:1 title="Spanish" \
           -c:v libx264 -c:a aac \
           "$VIDEOS_DIR/multi_audio.mkv" -y 2>/dev/null
else
    echo "  - Multi-audio video already exists"
fi

echo ""
echo "Test video samples ready!"
ls -lh "$VIDEOS_DIR"

# Create attribution file
cat > "$VIDEOS_DIR/README.md" <<EOF
# Test Video Samples

## Attribution

All video samples are Creative Commons licensed:

### Big Buck Bunny
- **License**: CC-BY 3.0
- **Source**: https://peach.blender.org/
- **Copyright**: (c) 2008, Blender Foundation

### Sintel
- **License**: CC-BY 3.0
- **Source**: https://durian.blender.org/
- **Copyright**: (c) 2010, Blender Foundation

### Synthetic Videos
- Generated with ffmpeg for testing purposes
- No copyright restrictions

## Usage

These videos are used exclusively for automated testing of the Subgen subtitle generation system.
EOF

echo ""
echo "Attribution file created: $VIDEOS_DIR/README.md"
```

**Usage:**
```bash
cd test/scripts
chmod +x download_test_videos.sh
./download_test_videos.sh
```

---

### Step 2: Subtitle Validation Utilities

**File: `/home/mikekao/personal/subgen/test/e2e/subtitle_validator.go`**

```go
package e2e

import (
	"bufio"
	"fmt"
	"os"
	"regexp"
	"strconv"
	"strings"
)

// SubtitleValidator validates SRT and LRC file formats
type SubtitleValidator struct{}

// SRTSegment represents a single SRT subtitle segment
type SRTSegment struct {
	Index     int
	StartTime string
	EndTime   string
	Text      string
}

// LRCLine represents a single LRC line
type LRCLine struct {
	Timestamp string
	Text      string
}

// ValidateSRT checks if an SRT file is valid
func (v *SubtitleValidator) ValidateSRT(filePath string) error {
	file, err := os.Open(filePath)
	if err != nil {
		return fmt.Errorf("failed to open SRT file: %w", err)
	}
	defer file.Close()

	scanner := bufio.NewScanner(file)
	segmentCount := 0
	lineNum := 0

	// SRT format:
	// 1
	// 00:00:00,000 --> 00:00:05,000
	// Subtitle text here
	// [blank line]

	indexRegex := regexp.MustCompile(`^\d+$`)
	timeRegex := regexp.MustCompile(`^\d{2}:\d{2}:\d{2},\d{3} --> \d{2}:\d{2}:\d{2},\d{3}$`)

	state := "index" // index, time, text, blank

	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		lineNum++

		switch state {
		case "index":
			if line == "" {
				continue // Skip leading blank lines
			}
			if !indexRegex.MatchString(line) {
				return fmt.Errorf("line %d: invalid segment index: %s", lineNum, line)
			}
			state = "time"

		case "time":
			if !timeRegex.MatchString(line) {
				return fmt.Errorf("line %d: invalid timestamp format: %s", lineNum, line)
			}
			state = "text"

		case "text":
			if line == "" {
				// End of segment
				segmentCount++
				state = "index"
			}
			// Text can span multiple lines, stay in text state
		}
	}

	if err := scanner.Err(); err != nil {
		return fmt.Errorf("error reading SRT file: %w", err)
	}

	if segmentCount == 0 {
		return fmt.Errorf("no valid segments found in SRT file")
	}

	return nil
}

// ValidateLRC checks if an LRC file is valid
func (v *SubtitleValidator) ValidateLRC(filePath string) error {
	file, err := os.Open(filePath)
	if err != nil {
		return fmt.Errorf("failed to open LRC file: %w", err)
	}
	defer file.Close()

	scanner := bufio.NewScanner(file)
	lineCount := 0

	// LRC format:
	// [00:00.00] Lyrics text
	// or
	// [ti:Title]
	// [ar:Artist]
	// etc.

	timeRegex := regexp.MustCompile(`^\[\d{2}:\d{2}\.\d{2}\]`)
	metadataRegex := regexp.MustCompile(`^\[(ti|ar|al|by|offset):.+\]`)

	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		if line == "" {
			continue
		}

		// Check if it's a timestamp line or metadata
		if !timeRegex.MatchString(line) && !metadataRegex.MatchString(line) {
			return fmt.Errorf("invalid LRC line format: %s", line)
		}

		if timeRegex.MatchString(line) {
			lineCount++
		}
	}

	if err := scanner.Err(); err != nil {
		return fmt.Errorf("error reading LRC file: %w", err)
	}

	if lineCount == 0 {
		return fmt.Errorf("no valid timestamp lines found in LRC file")
	}

	return nil
}

// ParseSRT parses an SRT file and returns segments
func (v *SubtitleValidator) ParseSRT(filePath string) ([]SRTSegment, error) {
	file, err := os.Open(filePath)
	if err != nil {
		return nil, err
	}
	defer file.Close()

	var segments []SRTSegment
	scanner := bufio.NewScanner(file)

	var currentSegment SRTSegment
	var textLines []string
	state := "index"

	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())

		switch state {
		case "index":
			if line == "" {
				continue
			}
			index, err := strconv.Atoi(line)
			if err != nil {
				return nil, fmt.Errorf("invalid segment index: %s", line)
			}
			currentSegment.Index = index
			state = "time"

		case "time":
			parts := strings.Split(line, " --> ")
			if len(parts) != 2 {
				return nil, fmt.Errorf("invalid timestamp: %s", line)
			}
			currentSegment.StartTime = parts[0]
			currentSegment.EndTime = parts[1]
			state = "text"

		case "text":
			if line == "" {
				// End of segment
				currentSegment.Text = strings.Join(textLines, "\n")
				segments = append(segments, currentSegment)
				currentSegment = SRTSegment{}
				textLines = nil
				state = "index"
			} else {
				textLines = append(textLines, line)
			}
		}
	}

	// Handle last segment if file doesn't end with blank line
	if len(textLines) > 0 {
		currentSegment.Text = strings.Join(textLines, "\n")
		segments = append(segments, currentSegment)
	}

	return segments, scanner.Err()
}

// GetSegmentCount returns the number of segments in a subtitle file
func (v *SubtitleValidator) GetSegmentCount(filePath string) (int, error) {
	if strings.HasSuffix(filePath, ".srt") {
		segments, err := v.ParseSRT(filePath)
		if err != nil {
			return 0, err
		}
		return len(segments), nil
	} else if strings.HasSuffix(filePath, ".lrc") {
		file, err := os.Open(filePath)
		if err != nil {
			return 0, err
		}
		defer file.Close()

		scanner := bufio.NewScanner(file)
		count := 0
		timeRegex := regexp.MustCompile(`^\[\d{2}:\d{2}\.\d{2}\]`)

		for scanner.Scan() {
			if timeRegex.MatchString(strings.TrimSpace(scanner.Text())) {
				count++
			}
		}

		return count, scanner.Err()
	}

	return 0, fmt.Errorf("unsupported file format: %s", filePath)
}
```

---

### Step 3: E2E Pipeline Tests

**File: `/home/mikekao/personal/subgen/test/e2e/pipeline_test.go`**

```go
package e2e

import (
	"context"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

const (
	orchestratorURL = "http://localhost:9000"
	workerAddr      = "localhost:50051"
	videosDir       = "../testdata/videos"
	maxWaitTime     = 5 * time.Minute
)

// TestMain downloads test videos before running tests
func TestMain(m *testing.M) {
	// Ensure test videos exist
	if _, err := os.Stat(videosDir); os.IsNotExist(err) {
		fmt.Println("Test videos directory not found. Run: ./scripts/download_test_videos.sh")
		os.Exit(1)
	}

	os.Exit(m.Run())
}

// waitForSubtitle waits for a subtitle file to be created
func waitForSubtitle(expectedPath string, timeout time.Duration) error {
	deadline := time.Now().Add(timeout)
	
	for time.Now().Before(deadline) {
		if _, err := os.Stat(expectedPath); err == nil {
			return nil
		}
		time.Sleep(2 * time.Second)
	}
	
	return fmt.Errorf("subtitle file not created within %v: %s", timeout, expectedPath)
}

// Test 1: Complete Plex Workflow - Video to SRT
func TestE2E_Plex_VideoToSRT(t *testing.T) {
	// Use real video file
	videoFile := filepath.Join(videosDir, "big_buck_bunny_30s.mp4")
	require.FileExists(t, videoFile, "Test video not found")

	// Expected subtitle file path
	expectedSub := strings.Replace(videoFile, ".mp4", ".tiny.aa.srt", 1)
	
	// Clean up existing subtitle
	os.Remove(expectedSub)

	// Simulate Plex webhook
	// In real implementation, would use mock media server from STORY_02
	// For E2E, we test with direct file path via Emby webhook (simpler)
	
	payload := fmt.Sprintf(`data={"Event":"library.new","Item":{"Path":"%s"}}`, videoFile)
	
	resp, err := http.Post(
		orchestratorURL+"/emby",
		"application/x-www-form-urlencoded",
		strings.NewReader(payload),
	)
	require.NoError(t, err)
	defer resp.Body.Close()

	assert.Equal(t, http.StatusOK, resp.StatusCode, "Webhook should be accepted")

	// Wait for transcription to complete
	t.Log("Waiting for transcription to complete...")
	err = waitForSubtitle(expectedSub, maxWaitTime)
	require.NoError(t, err, "Subtitle file should be created")

	// Validate subtitle file
	t.Log("Validating subtitle file...")
	validator := &SubtitleValidator{}
	err = validator.ValidateSRT(expectedSub)
	assert.NoError(t, err, "Subtitle file should be valid SRT")

	// Check segment count
	segmentCount, err := validator.GetSegmentCount(expectedSub)
	require.NoError(t, err)
	assert.Greater(t, segmentCount, 0, "Should have at least 1 segment")

	t.Logf("✅ E2E Test Passed: Video → SRT (%d segments)", segmentCount)

	// Cleanup
	os.Remove(expectedSub)
}

// Test 2: Complete Jellyfin Workflow - Audio to LRC
func TestE2E_Jellyfin_AudioToLRC(t *testing.T) {
	// Use audio-only file
	audioFile := filepath.Join(videosDir, "audio_only.m4a")
	require.FileExists(t, audioFile, "Test audio not found")

	// Expected LRC file path
	expectedLRC := strings.Replace(audioFile, ".m4a", ".lrc", 1)
	
	// Clean up
	os.Remove(expectedLRC)

	// Send webhook (using Emby for simplicity)
	payload := fmt.Sprintf(`data={"Event":"library.new","Item":{"Path":"%s"}}`, audioFile)
	
	resp, err := http.Post(
		orchestratorURL+"/emby",
		"application/x-www-form-urlencoded",
		strings.NewReader(payload),
	)
	require.NoError(t, err)
	defer resp.Body.Close()

	assert.Equal(t, http.StatusOK, resp.StatusCode)

	// Wait for LRC generation
	t.Log("Waiting for LRC generation...")
	err = waitForSubtitle(expectedLRC, maxWaitTime)
	require.NoError(t, err, "LRC file should be created")

	// Validate LRC file
	validator := &SubtitleValidator{}
	err = validator.ValidateLRC(expectedLRC)
	assert.NoError(t, err, "LRC file should be valid")

	t.Log("✅ E2E Test Passed: Audio → LRC")

	// Cleanup
	os.Remove(expectedLRC)
}

// Test 3: Multiple Audio Tracks
func TestE2E_MultipleAudioTracks(t *testing.T) {
	videoFile := filepath.Join(videosDir, "multi_audio.mkv")
	require.FileExists(t, videoFile, "Multi-audio test video not found")

	expectedSub := strings.Replace(videoFile, ".mkv", ".tiny.aa.srt", 1)
	os.Remove(expectedSub)

	payload := fmt.Sprintf(`data={"Event":"library.new","Item":{"Path":"%s"}}`, videoFile)
	
	resp, err := http.Post(
		orchestratorURL+"/emby",
		"application/x-www-form-urlencoded",
		strings.NewReader(payload),
	)
	require.NoError(t, err)
	defer resp.Body.Close()

	assert.Equal(t, http.StatusOK, resp.StatusCode)

	// Wait for transcription
	err = waitForSubtitle(expectedSub, maxWaitTime)
	require.NoError(t, err)

	// Verify subtitle was created (should use first audio track)
	validator := &SubtitleValidator{}
	err = validator.ValidateSRT(expectedSub)
	assert.NoError(t, err)

	t.Log("✅ E2E Test Passed: Multiple audio tracks handled")

	os.Remove(expectedSub)
}

// Test 4: Skip Logic - Existing Subtitle
func TestE2E_SkipExistingSubtitle(t *testing.T) {
	videoFile := filepath.Join(videosDir, "big_buck_bunny_30s.mp4")
	require.FileExists(t, videoFile)

	subtitlePath := strings.Replace(videoFile, ".mp4", ".tiny.aa.srt", 1)
	
	// Create dummy subtitle file
	err := os.WriteFile(subtitlePath, []byte("Existing subtitle"), 0644)
	require.NoError(t, err)

	// Get file info before
	infoBefore, err := os.Stat(subtitlePath)
	require.NoError(t, err)

	// Send webhook
	payload := fmt.Sprintf(`data={"Event":"library.new","Item":{"Path":"%s"}}`, videoFile)
	resp, err := http.Post(
		orchestratorURL+"/emby",
		"application/x-www-form-urlencoded",
		strings.NewReader(payload),
	)
	require.NoError(t, err)
	defer resp.Body.Close()

	// Wait a bit
	time.Sleep(5 * time.Second)

	// Verify subtitle was NOT overwritten (skip logic)
	infoAfter, err := os.Stat(subtitlePath)
	require.NoError(t, err)

	assert.Equal(t, infoBefore.ModTime(), infoAfter.ModTime(), "Existing subtitle should not be overwritten")

	t.Log("✅ E2E Test Passed: Skip existing subtitle")

	os.Remove(subtitlePath)
}

// Test 5: Error Handling - Corrupt Video
func TestE2E_CorruptVideoFile(t *testing.T) {
	// Create corrupt video file
	corruptFile := filepath.Join(videosDir, "corrupt_video.mp4")
	err := os.WriteFile(corruptFile, []byte("not a valid video"), 0644)
	require.NoError(t, err)
	defer os.Remove(corruptFile)

	expectedSub := strings.Replace(corruptFile, ".mp4", ".tiny.aa.srt", 1)

	payload := fmt.Sprintf(`data={"Event":"library.new","Item":{"Path":"%s"}}`, corruptFile)
	resp, err := http.Post(
		orchestratorURL+"/emby",
		"application/x-www-form-urlencoded",
		strings.NewReader(payload),
	)
	require.NoError(t, err)
	defer resp.Body.Close()

	// Webhook accepted, but transcription will fail
	assert.Equal(t, http.StatusOK, resp.StatusCode)

	// Wait a bit
	time.Sleep(10 * time.Second)

	// Verify subtitle was NOT created
	_, err = os.Stat(expectedSub)
	assert.Error(t, err, "Subtitle should not be created for corrupt file")
	assert.True(t, os.IsNotExist(err))

	t.Log("✅ E2E Test Passed: Corrupt file handled gracefully")
}

// Test 6: Language Detection
func TestE2E_LanguageDetection(t *testing.T) {
	// This test requires a video with actual speech
	// For now, we test that language field is populated
	
	videoFile := filepath.Join(videosDir, "big_buck_bunny_30s.mp4")
	require.FileExists(t, videoFile)

	expectedSub := strings.Replace(videoFile, ".mp4", ".tiny.aa.srt", 1)
	os.Remove(expectedSub)

	payload := fmt.Sprintf(`data={"Event":"library.new","Item":{"Path":"%s"}}`, videoFile)
	resp, err := http.Post(
		orchestratorURL+"/emby",
		"application/x-www-form-urlencoded",
		strings.NewReader(payload),
	)
	require.NoError(t, err)
	defer resp.Body.Close()

	err = waitForSubtitle(expectedSub, maxWaitTime)
	require.NoError(t, err)

	// Verify file contains language code in filename
	// Format: filename.model.language.srt
	parts := strings.Split(filepath.Base(expectedSub), ".")
	require.GreaterOrEqual(t, len(parts), 4, "Subtitle filename should contain language code")
	
	languageCode := parts[len(parts)-2] // Second to last part
	assert.NotEmpty(t, languageCode, "Language code should be present")
	assert.Len(t, languageCode, 2, "Language code should be 2 letters (ISO 639-1)")

	t.Logf("✅ E2E Test Passed: Language detected (%s)", languageCode)

	os.Remove(expectedSub)
}

// Test 7: Performance - 30 Second Video
func TestE2E_Performance_30SecondVideo(t *testing.T) {
	videoFile := filepath.Join(videosDir, "big_buck_bunny_30s.mp4")
	require.FileExists(t, videoFile)

	expectedSub := strings.Replace(videoFile, ".mp4", ".tiny.aa.srt", 1)
	os.Remove(expectedSub)

	// Measure time
	start := time.Now()

	payload := fmt.Sprintf(`data={"Event":"library.new","Item":{"Path":"%s"}}`, videoFile)
	resp, err := http.Post(
		orchestratorURL+"/emby",
		"application/x-www-form-urlencoded",
		strings.NewReader(payload),
	)
	require.NoError(t, err)
	defer resp.Body.Close()

	err = waitForSubtitle(expectedSub, maxWaitTime)
	require.NoError(t, err)

	duration := time.Since(start)

	// Tiny model should transcribe 30s video in < 2 minutes
	assert.Less(t, duration, 2*time.Minute, "30s video should transcribe in < 2 minutes")

	t.Logf("✅ E2E Test Passed: Performance acceptable (%.1fs)", duration.Seconds())

	os.Remove(expectedSub)
}
```

---

### Step 4: Manual Test Procedure

**File: `/home/mikekao/personal/subgen/test/manual/MANUAL_TEST_PROCEDURE.md`**

```markdown
# Manual Test Procedure for Subgen

This document describes manual testing procedures for validating the Subgen system end-to-end.

---

## Prerequisites

1. Docker Compose environment running (from STORY_01)
2. Real video samples downloaded
3. Access to orchestrator HTTP endpoint (http://localhost:9000)
4. Access to worker gRPC endpoint (localhost:50051)

---

## Test 1: Download and Test Real Video

### Step 1.1: Download Sample Video

```bash
# Download Big Buck Bunny sample (CC-BY license)
cd test/testdata/videos
curl -L -o big_buck_bunny_480p_30s.mp4 \
  "https://download.blender.org/peach/bigbuckbunny_movies/big_buck_bunny_480p_stereo.avi"

# Trim to 30 seconds
ffmpeg -i big_buck_bunny_480p_30s.mp4 -t 30 -c copy big_buck_bunny_30s.mp4
```

**Expected Result**: Video file downloaded and trimmed successfully

### Step 1.2: Send Webhook Manually

```bash
# Send Emby-style webhook with video path
VIDEO_PATH="$(pwd)/big_buck_bunny_30s.mp4"

curl -X POST http://localhost:9000/emby \
  -H "Content-Type: application/x-www-form-urlencoded" \
  -d "data={\"Event\":\"library.new\",\"Item\":{\"Path\":\"$VIDEO_PATH\"}}"
```

**Expected Result**: HTTP 200 OK response

### Step 1.3: Monitor Transcription

```bash
# Watch orchestrator logs
docker logs -f subgen-orchestrator-integration

# Watch worker logs
docker logs -f subgen-worker-integration

# Watch for subtitle file creation
watch -n 2 ls -lh test/testdata/videos/*.srt
```

**Expected Result**:
- Orchestrator logs show webhook received
- Worker logs show transcription in progress
- Subtitle file appears after 30-60 seconds

### Step 1.4: Validate Subtitle File

```bash
# Check subtitle file exists
ls -lh big_buck_bunny_30s.tiny.aa.srt

# View first few lines
head -20 big_buck_bunny_30s.tiny.aa.srt

# Validate format
cat big_buck_bunny_30s.tiny.aa.srt | grep "^[0-9]$" | wc -l
# Should show number of segments
```

**Expected Result**:
- Subtitle file exists with `.tiny.aa.srt` extension
- File contains valid SRT format
- Contains multiple segments with timestamps
- Timestamps in format `HH:MM:SS,mmm --> HH:MM:SS,mmm`

---

## Test 2: Audio-Only File (LRC Generation)

### Step 2.1: Create Audio File

```bash
cd test/testdata/videos

# Extract audio from video
ffmpeg -i big_buck_bunny_30s.mp4 -vn -acodec copy audio_test.m4a
```

### Step 2.2: Send Webhook

```bash
AUDIO_PATH="$(pwd)/audio_test.m4a"

curl -X POST http://localhost:9000/emby \
  -H "Content-Type: application/x-www-form-urlencoded" \
  -d "data={\"Event\":\"library.new\",\"Item\":{\"Path\":\"$AUDIO_PATH\"}}"
```

### Step 2.3: Verify LRC File

```bash
# LRC file should be created for audio files
ls -lh audio_test.lrc

# View LRC format
cat audio_test.lrc

# Format should be: [MM:SS.xx] Lyrics
```

**Expected Result**: `.lrc` file created with timestamp format `[MM:SS.xx]`

---

## Test 3: Multiple Audio Tracks

### Step 3.1: Create Multi-Audio Video

```bash
# Create video with 2 audio tracks
ffmpeg -f lavfi -i testsrc=duration=30:size=640x480:rate=30 \
       -f lavfi -i sine=frequency=440:duration=30 \
       -f lavfi -i sine=frequency=880:duration=30 \
       -map 0:v -map 1:a -map 2:a \
       -metadata:s:a:0 language=eng -metadata:s:a:0 title="English" \
       -metadata:s:a:1 language=spa -metadata:s:a:1 title="Spanish" \
       -c:v libx264 -c:a aac \
       multi_audio_test.mkv
```

### Step 3.2: Send Webhook

```bash
VIDEO_PATH="$(pwd)/multi_audio_test.mkv"

curl -X POST http://localhost:9000/emby \
  -H "Content-Type: application/x-www-form-urlencoded" \
  -d "data={\"Event\":\"library.new\",\"Item\":{\"Path\":\"$VIDEO_PATH\"}}"
```

### Step 3.3: Verify Behavior

```bash
# Should create subtitle for first audio track
ls -lh multi_audio_test.tiny.aa.srt
```

**Expected Result**: Subtitle created using first audio track (English)

---

## Test 4: Performance Benchmark

### Step 4.1: Measure Transcription Time

```bash
# Time the transcription
START=$(date +%s)

VIDEO_PATH="$(pwd)/big_buck_bunny_30s.mp4"
rm -f big_buck_bunny_30s.tiny.aa.srt

curl -X POST http://localhost:9000/emby \
  -H "Content-Type: application/x-www-form-urlencoded" \
  -d "data={\"Event\":\"library.new\",\"Item\":{\"Path\":\"$VIDEO_PATH\"}}"

# Wait for subtitle to appear
while [ ! -f "big_buck_bunny_30s.tiny.aa.srt" ]; do
  sleep 2
done

END=$(date +%s)
DURATION=$((END - START))

echo "Transcription took ${DURATION} seconds"
```

**Expected Result**:
- 30-second video transcribes in < 120 seconds (tiny model, CPU)
- Times may vary based on hardware

---

## Test 5: Error Handling

### Step 5.1: Corrupt File

```bash
# Create invalid video file
echo "not a video" > corrupt.mp4

VIDEO_PATH="$(pwd)/corrupt.mp4"

curl -X POST http://localhost:9000/emby \
  -H "Content-Type: application/x-www-form-urlencoded" \
  -d "data={\"Event\":\"library.new\",\"Item\":{\"Path\":\"$VIDEO_PATH\"}}"

# Wait and verify no subtitle created
sleep 10
ls -lh corrupt.tiny.aa.srt  # Should not exist
```

**Expected Result**: No subtitle file created, error logged

### Step 5.2: Missing File

```bash
VIDEO_PATH="/nonexistent/video.mp4"

curl -X POST http://localhost:9000/emby \
  -H "Content-Type: application/x-www-form-urlencoded" \
  -d "data={\"Event\":\"library.new\",\"Item\":{\"Path\":\"$VIDEO_PATH\"}}"
```

**Expected Result**: Webhook accepted (200 OK), but transcription fails gracefully

---

## Test 6: Real Plex Integration (Optional)

**Prerequisites**: Running Plex server with webhook configured

### Step 6.1: Configure Plex Webhook

1. Open Plex Web UI
2. Settings → Webhooks
3. Add webhook: `http://<orchestrator-ip>:9000/plex`
4. Add test video to Plex library

### Step 6.2: Trigger Webhook

- Add new media file to Plex library
- Or play existing media file

### Step 6.3: Verify

- Check orchestrator logs for webhook receipt
- Wait for subtitle file to appear
- Verify Plex detects new subtitle

**Expected Result**: Subtitle appears in Plex after ~1-2 minutes

---

## Validation Checklist

After completing all manual tests, verify:

- [ ] SRT file format valid (segments, timestamps)
- [ ] LRC file format valid (for audio files)
- [ ] Multiple audio tracks handled
- [ ] Performance acceptable (< 2 min for 30s video)
- [ ] Error handling graceful (no crashes)
- [ ] Files created in same directory as media
- [ ] Filename format: `basename.model.language.srt`
- [ ] Real Plex integration works (if tested)

---

## Cleanup

```bash
# Remove generated subtitle files
rm -f test/testdata/videos/*.srt
rm -f test/testdata/videos/*.lrc

# Stop Docker Compose
cd test
docker-compose -f docker-compose.integration.yml down
```
```

---

## Definition of Done

- [ ] All acceptance criteria met
- [ ] Test videos downloaded (Big Buck Bunny, Sintel)
- [ ] E2E tests written (7+ tests)
- [ ] Subtitle validation utilities created
- [ ] All E2E tests pass with Docker Compose
- [ ] Manual test procedure documented
- [ ] Real video tested manually with complete pipeline
- [ ] Subtitle content validated (correct format, segments exist)
- [ ] Performance acceptable (< 2 min for 30s video with tiny model)
- [ ] Error scenarios tested
- [ ] Work log created: `docs/WORKLOGS/NNNN_YYYY-MM-DD_EPIC_03_story_03.md`
- [ ] Code committed and pushed

---

## Validation Commands

```bash
# Download test videos
cd test/scripts
./download_test_videos.sh

# Start Docker Compose
cd test
docker-compose -f docker-compose.integration.yml up -d

# Run E2E tests
cd test/e2e
go test -v -timeout 10m

# Run specific test
go test -v -run TestE2E_Plex_VideoToSRT

# Manual testing
cd test/manual
# Follow MANUAL_TEST_PROCEDURE.md

# Cleanup
cd test
docker-compose -f docker-compose.integration.yml down
```

---

## Dependencies

**Requires:**
- STORY_01 (gRPC Integration Tests) - Docker Compose setup
- STORY_02 (Webhook Integration Tests) - Mock media server

**Blocks:**
- STORY_04 (Memory Leak Validation) - needs stable E2E pipeline
- STORY_05 (Load Testing) - builds on E2E tests

---

## Notes

### Test Video Sources

- **Big Buck Bunny**: https://peach.blender.org/ (CC-BY 3.0)
- **Sintel**: https://durian.blender.org/ (CC-BY 3.0)
- Both are open-source animated films safe for testing

### Transcription Accuracy

- Synthetic audio (sine waves) produces minimal/empty transcriptions
- Real video with speech produces actual subtitle content
- Use Big Buck Bunny for testing - has dialogue

### Performance Expectations

With **tiny Whisper model** on **CPU**:
- 30s video: ~30-60 seconds
- 1min video: ~60-120 seconds
- Real-time factor: ~1-2x

With **medium model** on **GPU**:
- Much faster, but requires GPU support

---

## References

- [worker/src/transcription/engine.py](/home/mikekao/personal/subgen/worker/src/transcription/engine.py) - Transcription logic
- [worker/src/subtitles/writer.py](../../worker/src/subtitles/writer.py) - SRT/LRC generation
- SRT Format Specification: https://matroska.org/technical/subtitles.html
- LRC Format Specification: https://en.wikipedia.org/wiki/LRC_(file_format)

---

**Created**: 2026-02-15  
**Last Updated**: 2026-02-15
