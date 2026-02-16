# EPIC_08: Advanced Features & Polish

**Status:** Not Started  
**Estimated Effort:** 32-42 hours  
**Duration:** 4-6 days  
**Priority:** 🟢 MEDIUM (Enhancement & Quality of Life)  
**Can Parallelize:** Yes (all stories independent)

---

## Overview

Implement remaining features from the original subgen.py that enhance usability, flexibility, and power user workflows. These are **quality of life improvements** rather than core functionality, but significantly improve the user experience and feature completeness.

**Scope:** This epic covers all remaining "nice to have" features identified in the feature parity checklist, including multiple output formats, batch processing, Plex episode queueing, advanced configuration options, and API enhancements.

---

## Problem Statement

**Current State:**
- Core transcription works perfectly ✅
- Basic webhook integration complete ✅
- Skip logic and monitoring planned (EPIC_06, EPIC_07)
- **Missing:** Convenience features, bulk operations, advanced options, format flexibility

**User Pain Points:**
1. **Limited output formats** - Only SRT/LRC, no VTT/TXT/TSV/JSON
2. **No bulk operations** - Can't batch process directories via API
3. **Manual TV show processing** - No automatic episode/season queueing
4. **Limited language detection** - No standalone endpoint
5. **Inflexible ASR** - Can't choose output format for Bazarr
6. **No path mapping applied** - Config exists but not used
7. **No progress reporting** - Can't monitor queue or transcription progress
8. **Limited customization** - SUBGEN_KWARGS, custom prompts not implemented

---

## Goals

1. Add multiple subtitle output formats (VTT, TXT, TSV, JSON)
2. Implement batch processing API endpoint
3. Add Plex episode queueing (next/season/series)
4. Create standalone language detection endpoint
5. Complete ASR endpoint with format selection
6. Apply path mapping logic
7. Add progress reporting and queue status
8. Implement advanced Whisper options (SUBGEN_KWARGS, prompts)
9. Polish user experience with better logging and error messages

---

## User Stories

### [STORY_01: Multiple Output Formats](./stories/STORY_01_output_formats.md)
**Status:** Not Started  
**Effort:** 8-10 hours  
**Priority:** MEDIUM  
**Summary:** Support VTT, TXT, TSV, and JSON subtitle formats

**Acceptance Criteria:**
- [ ] **VTT format** - WebVTT for web players
- [ ] **TXT format** - Plain text transcript (no timestamps)
- [ ] **TSV format** - Tab-separated values (timestamp, text)
- [ ] **JSON format** - Structured data with segments
- [ ] Configuration: `SUBTITLE_FORMAT` env var (default: auto)
- [ ] API parameter: `?format=vtt` on ASR endpoint
- [ ] LRC remains default for audio files

**Format Examples:**

**VTT (WebVTT):**
```vtt
WEBVTT

00:00:00.000 --> 00:00:03.200
Hello, this is a test subtitle.

00:00:03.400 --> 00:00:06.800
This is the second line of text.
```

**TXT (Plain Text):**
```txt
Hello, this is a test subtitle.
This is the second line of text.
The audio continues with more dialogue.
```

**TSV (Tab-Separated Values):**
```tsv
start	end	text
0.000	3.200	Hello, this is a test subtitle.
3.400	6.800	This is the second line of text.
```

**JSON (Structured Data):**
```json
{
  "language": "en",
  "duration": 120.5,
  "segments": [
    {
      "start": 0.0,
      "end": 3.2,
      "text": "Hello, this is a test subtitle."
    },
    {
      "start": 3.4,
      "end": 6.8,
      "text": "This is the second line of text."
    }
  ]
}
```

**Implementation:**
```go
// orchestrator/pkg/formats/
├── vtt_writer.go    # WebVTT format
├── txt_writer.go    # Plain text
├── tsv_writer.go    # Tab-separated
├── json_writer.go   # JSON structure
└── writer.go        # Interface + factory
```

---

### [STORY_02: Batch Processing Endpoint](./stories/STORY_02_batch_endpoint.md)
**Status:** Not Started  
**Effort:** 4-6 hours  
**Priority:** MEDIUM  
**Summary:** Add `/batch` endpoint for bulk transcription via API

**Acceptance Criteria:**
- [ ] `POST /batch?directory=/path/to/folder`
- [ ] Optional: `?language=en` to force language
- [ ] Optional: `?recursive=true` for subdirectories
- [ ] Returns: `{"scanned": 150, "queued": 23, "skipped": 127}`
- [ ] Respects skip logic (EPIC_06)
- [ ] Authentication optional (for security)

**API Design:**
```http
POST /batch?directory=/movies&recursive=true&language=en HTTP/1.1
Content-Type: application/json

# Response
{
  "status": "success",
  "scanned": 150,
  "queued": 23,
  "skipped": 127,
  "skip_reasons": {
    "subtitle_exists": 120,
    "audio_language_mismatch": 5,
    "unknown_language": 2
  }
}
```

**Implementation:**
```go
// orchestrator/internal/webhooks/batch.go
func (s *Server) handleBatch(c *fiber.Ctx) error {
    directory := c.Query("directory")
    recursive := c.QueryBool("recursive", false)
    language := c.Query("language", "")
    
    scanner := monitor.NewScanner(s.queue, s.skipChecker)
    scanned, queued, skipReasons := scanner.ScanDirectory(directory, recursive)
    
    return c.JSON(fiber.Map{
        "status": "success",
        "scanned": scanned,
        "queued": queued,
        "skipped": scanned - queued,
        "skip_reasons": skipReasons,
    })
}
```

---

### [STORY_03: Plex Episode Queueing](./stories/STORY_03_plex_episode_queue.md)
**Status:** Not Started  
**Effort:** 8-10 hours  
**Priority:** MEDIUM  
**Summary:** Auto-queue next episode, season, or entire series in Plex

**Acceptance Criteria:**
- [ ] Configuration: `PLEX_QUEUE_NEXT_EPISODE`, `PLEX_QUEUE_SEASON`, `PLEX_QUEUE_SERIES`
- [ ] Plex XML API navigation (parent/children relationships)
- [ ] Detect season boundaries (don't queue across seasons)
- [ ] Error handling at series end
- [ ] Skip files that fail skip logic
- [ ] Logging: "Queued 10 episodes from Season 1"

**Original Behavior (subgen.py lines 582-623, 1790-1889):**
- `PLEX_QUEUE_NEXT_EPISODE=true` → Queue next episode in series
- `PLEX_QUEUE_SEASON=true` → Queue all episodes in current season
- `PLEX_QUEUE_SERIES=true` → Queue entire TV series

**Use Case:**
```bash
# User watches S01E01 of a new TV series
# Webhook triggers for S01E01
# With PLEX_QUEUE_SEASON=true:
#   → Transcribe S01E01 immediately
#   → Queue S01E02, S01E03, ..., S01E10 for later
#   → User has subtitles for entire season by next episode
```

**Implementation:**
```go
// orchestrator/internal/plex/episode_queue.go
func (p *PlexClient) QueueNextEpisodes(itemID string, mode string) error {
    // Get item metadata
    item, err := p.GetMetadata(itemID)
    
    // Determine what to queue based on mode
    switch mode {
    case "next":
        nextEpisode := p.GetNextEpisode(item)
        p.queueItem(nextEpisode)
    case "season":
        episodes := p.GetSeasonEpisodes(item.ParentKey)
        for _, ep := range episodes {
            p.queueItem(ep)
        }
    case "series":
        allEpisodes := p.GetSeriesEpisodes(item.GrandparentKey)
        for _, ep := range allEpisodes {
            p.queueItem(ep)
        }
    }
    
    return nil
}
```

---

### [STORY_04: Standalone Language Detection](./stories/STORY_04_detect_language_endpoint.md)
**Status:** Not Started  
**Effort:** 3-4 hours  
**Priority:** LOW  
**Summary:** Add `/detect-language` endpoint for quick language identification

**Acceptance Criteria:**
- [ ] `POST /detect-language` accepts uploaded audio file
- [ ] Query params: `?offset=0&length=30` for sample selection
- [ ] Bypasses queue (immediate processing)
- [ ] Returns: `{"language": "English", "code": "en", "confidence": 0.99}`
- [ ] Uses existing DetectLanguage RPC
- [ ] Timeout: 30 seconds

**API Design:**
```http
POST /detect-language?offset=0&length=30 HTTP/1.1
Content-Type: multipart/form-data

--boundary
Content-Disposition: form-data; name="file"; filename="audio.mp3"
Content-Type: audio/mpeg

[audio data]
--boundary--

# Response
{
  "language": "English",
  "code": "en",
  "confidence": 0.99
}
```

**Use Case:**
- Quick language identification without full transcription
- Testing Whisper model on samples
- Integration with other tools

---

### [STORY_05: ASR Format Selection](./stories/STORY_05_asr_format_selection.md)
**Status:** Not Started  
**Effort:** 3-4 hours  
**Priority:** MEDIUM  
**Summary:** Allow format selection on ASR endpoint for Bazarr integration

**Acceptance Criteria:**
- [ ] Query parameter: `?output=srt` (default), `?output=vtt`, `?output=lrc`
- [ ] Return subtitle in requested format
- [ ] Still block until completion
- [ ] Content-Type headers match format
- [ ] Works with existing ASR deduplication

**API Enhancement:**
```http
POST /asr?task=transcribe&language=en&output=vtt HTTP/1.1
Content-Type: multipart/form-data

[audio file]

# Response
Content-Type: text/vtt
WEBVTT

00:00:00.000 --> 00:00:03.200
Transcribed text here.
```

**Rationale:**
Bazarr may prefer VTT format for web players, but currently only SRT is returned.

---

### [STORY_06: Path Mapping Application](./stories/STORY_06_path_mapping.md)
**Status:** Not Started  
**Effort:** 2-3 hours  
**Priority:** HIGH  
**Summary:** Apply path mapping configuration before transcription

**Acceptance Criteria:**
- [ ] Use existing `USE_PATH_MAPPING`, `PATH_MAPPING_FROM`, `PATH_MAPPING_TO` config
- [ ] Apply mapping in webhook handlers before queueing
- [ ] Support multiple mappings (comma-separated)
- [ ] Bidirectional mapping (from media server to Subgen paths)
- [ ] Validation: Warn if mapped path doesn't exist

**Use Case (Docker):**
```yaml
# docker-compose.yml
services:
  plex:
    volumes:
      - /host/media:/data
  subgen:
    volumes:
      - /host/media:/mnt/media
    environment:
      USE_PATH_MAPPING: true
      PATH_MAPPING_FROM: /data
      PATH_MAPPING_TO: /mnt/media
```

**Implementation:**
```go
// orchestrator/internal/util/path_mapper.go
func (pm *PathMapper) Map(path string) (string, error) {
    if !pm.config.UsePathMapping {
        return path, nil
    }
    
    for _, mapping := range pm.mappings {
        if strings.HasPrefix(path, mapping.From) {
            mapped := strings.Replace(path, mapping.From, mapping.To, 1)
            
            // Validate mapped path exists
            if _, err := os.Stat(mapped); err != nil {
                return "", fmt.Errorf("mapped path does not exist: %s", mapped)
            }
            
            return mapped, nil
        }
    }
    
    return path, nil
}
```

---

### [STORY_07: Queue Status & Progress Reporting](./stories/STORY_07_progress_reporting.md)
**Status:** Not Started  
**Effort:** 4-6 hours  
**Priority:** LOW  
**Summary:** Add endpoints to monitor queue and transcription progress

**Acceptance Criteria:**
- [ ] `GET /queue/status` - Current queue state
- [ ] `GET /queue/processing` - Active transcriptions
- [ ] `GET /queue/history` - Recent completions
- [ ] `GET /tasks/{task_id}` - Individual task status
- [ ] WebSocket endpoint for real-time updates (optional)
- [ ] Metrics: queue depth, processing time, success rate

**API Endpoints:**

**Queue Status:**
```http
GET /queue/status HTTP/1.1

{
  "queued": 15,
  "processing": 2,
  "completed_last_hour": 47,
  "failed_last_hour": 1,
  "idle": false
}
```

**Processing Tasks:**
```http
GET /queue/processing HTTP/1.1

{
  "tasks": [
    {
      "id": "task-12345",
      "file_path": "/movies/action/movie.mkv",
      "started_at": "2026-02-16T12:34:56Z",
      "progress": 65,
      "eta_seconds": 120
    },
    {
      "id": "task-12346",
      "file_path": "/tv/show/s01e02.mkv",
      "started_at": "2026-02-16T12:35:10Z",
      "progress": 23,
      "eta_seconds": 300
    }
  ]
}
```

**Use Case:**
- Dashboards and monitoring UIs
- User feedback ("How many files left?")
- Debugging and troubleshooting

---

### [STORY_08: Advanced Whisper Options](./stories/STORY_08_advanced_whisper.md)
**Status:** Not Started  
**Effort:** 4-6 hours  
**Priority:** LOW  
**Summary:** Implement SUBGEN_KWARGS, custom prompts, and advanced Whisper parameters

**Acceptance Criteria:**
- [ ] `SUBGEN_KWARGS` - JSON string with arbitrary Whisper parameters
- [ ] `USE_MODEL_PROMPT` - Enable prompt usage
- [ ] `CUSTOM_MODEL_PROMPT` - Custom prompt text
- [ ] `CUSTOM_REGROUP` - stable-ts regrouping algorithm
- [ ] Pass parameters to worker via gRPC
- [ ] Validate parameters before transcription
- [ ] Documentation with examples

**Configuration:**
```env
# Custom Whisper parameters (advanced users)
SUBGEN_KWARGS='{"temperature": 0.0, "compression_ratio_threshold": 2.4, "condition_on_previous_text": false}'

# Custom prompt (force punctuation)
USE_MODEL_PROMPT=true
CUSTOM_MODEL_PROMPT="This is a transcript with proper punctuation and capitalization."

# Stable-TS regrouping
CUSTOM_REGROUP="cm_sl=84_sl=42++++++1"
```

**Implementation:**
```go
// orchestrator/internal/config/whisper.go
type WhisperConfig struct {
    Model        string
    Device       string
    ComputeType  string
    Threads      int
    
    // Advanced options
    UsePrompt    bool
    Prompt       string
    Regroup      string
    ExtraKwargs  map[string]interface{}  // Parsed from SUBGEN_KWARGS
}

// Parse JSON kwargs
func ParseSubgenKwargs(jsonStr string) (map[string]interface{}, error) {
    var kwargs map[string]interface{}
    if err := json.Unmarshal([]byte(jsonStr), &kwargs); err != nil {
        return nil, err
    }
    return kwargs, nil
}
```

---

### [STORY_09: Enhanced Logging & Error Messages](./stories/STORY_09_logging_polish.md)
**Status:** Not Started  
**Effort:** 2-4 hours  
**Priority:** LOW  
**Summary:** Improve log quality, error messages, and troubleshooting info

**Acceptance Criteria:**
- [ ] Structured logging with consistent fields (file_path, task_id, duration, etc.)
- [ ] Clear error messages with actionable solutions
- [ ] Log level filtering (DEBUG, INFO, WARN, ERROR)
- [ ] Request IDs for tracing
- [ ] Performance metrics in logs (processing time, queue depth)
- [ ] Startup banner with version and configuration summary

**Example Logs:**

**Startup:**
```
INFO  [2026-02-16 12:00:00] Subgen Orchestrator v2026.02.16
INFO  [2026-02-16 12:00:00] Configuration:
  - Whisper Model: medium
  - Device: cuda
  - Concurrent Workers: 2
  - Skip Logic: enabled (7 conditions)
  - File Monitoring: enabled (3 folders)
INFO  [2026-02-16 12:00:00] Webhook server listening on :9000
INFO  [2026-02-16 12:00:00] Metrics server listening on :9091
```

**Processing:**
```
INFO  [2026-02-16 12:01:23] task_id=abc123 file=/movies/action.mkv event=task_queued priority=2
INFO  [2026-02-16 12:01:24] task_id=abc123 file=/movies/action.mkv event=task_started worker=1
INFO  [2026-02-16 12:03:45] task_id=abc123 file=/movies/action.mkv event=task_completed duration=141s output=/movies/action.eng.srt
```

**Errors:**
```
ERROR [2026-02-16 12:05:00] task_id=def456 file=/movies/broken.mkv event=task_failed error="audio extraction failed: no audio tracks found"
  Troubleshooting:
    - Verify file has audio tracks: ffprobe /movies/broken.mkv
    - Check file is not corrupted
    - Try re-encoding with: ffmpeg -i broken.mkv -c copy fixed.mkv
```

---

## Architecture

### Component Structure

```
orchestrator/
├── pkg/formats/           # STORY_01: Output format writers
│   ├── vtt_writer.go
│   ├── txt_writer.go
│   ├── tsv_writer.go
│   ├── json_writer.go
│   └── writer.go
├── internal/
│   ├── webhooks/
│   │   ├── batch.go       # STORY_02: Batch endpoint
│   │   └── status.go      # STORY_07: Queue status
│   ├── plex/
│   │   └── episode_queue.go  # STORY_03: Episode queueing
│   ├── util/
│   │   └── path_mapper.go # STORY_06: Path mapping
│   └── config/
│       └── whisper.go     # STORY_08: Advanced Whisper
```

---

## Dependencies

**Requires:**
- EPIC_01 (Go Orchestrator) - ✅ Complete
- EPIC_06 (Skip Logic) - ⚠️ For batch endpoint efficiency

**External Libraries:**
- No new dependencies (all use standard library)

**Parallelizable:**
- All stories are independent and can be developed in parallel

---

## Testing Strategy

### Unit Tests
- Format writers (VTT, TXT, TSV, JSON)
- Path mapping logic
- Whisper parameter parsing
- Plex XML navigation

### Integration Tests
- Batch endpoint with real directories
- ASR with format selection
- Episode queueing with mock Plex API
- Path mapping with Docker volumes

### Manual Testing
```bash
# Test 1: Multiple formats
curl -X POST http://localhost:9000/asr?output=vtt -F file=@audio.mp3
curl -X POST http://localhost:9000/asr?output=txt -F file=@audio.mp3

# Test 2: Batch processing
curl -X POST "http://localhost:9000/batch?directory=/movies&recursive=true"

# Test 3: Queue status
curl http://localhost:9000/queue/status

# Test 4: Path mapping
# Configure PATH_MAPPING_FROM=/data, PATH_MAPPING_TO=/mnt/media
# Trigger webhook with /data/movie.mkv
# Verify transcription uses /mnt/media/movie.mkv
```

---

## Success Metrics

- [ ] **Format support:** 6 formats total (SRT, LRC, VTT, TXT, TSV, JSON)
- [ ] **Batch performance:** Process 1000-file directory scan in < 10 seconds
- [ ] **Episode queueing:** Correctly queue entire TV season (20 episodes)
- [ ] **Path mapping:** 100% success rate translating paths
- [ ] **Advanced options:** SUBGEN_KWARGS working for 10+ parameters
- [ ] **API completeness:** 8 new endpoints functional

---

## Timeline

**Day 1:** STORY_01 (Multiple output formats) - 8-10 hours  
**Day 2:** STORY_02 (Batch endpoint) + STORY_04 (Detect language) - 7-10 hours  
**Day 3:** STORY_03 (Plex episode queueing) + STORY_06 (Path mapping) - 10-13 hours  
**Day 4:** STORY_05 (ASR format) + STORY_08 (Advanced Whisper) - 7-10 hours  
**Day 5:** STORY_07 (Progress reporting) + STORY_09 (Logging polish) - 6-10 hours

---

## Definition of Done

- [ ] All 9 stories completed with ✅ status
- [ ] 6 subtitle formats supported (SRT, LRC, VTT, TXT, TSV, JSON)
- [ ] Batch processing endpoint functional
- [ ] Plex episode queueing working
- [ ] Standalone language detection endpoint
- [ ] ASR format selection implemented
- [ ] Path mapping applied correctly
- [ ] Queue status and progress endpoints
- [ ] Advanced Whisper options (SUBGEN_KWARGS, prompts)
- [ ] Enhanced logging throughout
- [ ] Unit tests (>80% coverage)
- [ ] Integration tests for all endpoints
- [ ] Documentation (API reference + configuration guide)
- [ ] Work logs for each story

---

## References

- **Feature Parity Checklist:** `/home/mikekao/personal/subgen/docs/WORKLOGS/FEATURE_PARITY_CHECKLIST.md`
- **Original Implementation:**
  - Multiple formats: Lines 843-856
  - Batch endpoint: Lines 687-692
  - Plex episode queue: Lines 582-623, 1790-1889
  - Detect language: Lines 896-939
  - Path mapping: Lines 2062-2066
  - SUBGEN_KWARGS: Lines 138-139, 1389-1410
  - Custom prompt: Lines 140-142, 1411-1418

---

**Epic Owner:** TBD  
**Created:** 2026-02-16  
**Last Updated:** 2026-02-16
