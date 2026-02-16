# STORY_03: Webhook Handlers

**Status:** In Progress  
**Effort:** 10-12 hours  
**Epic:** EPIC_01 (Go Orchestrator Core)  
**Created:** 2026-02-15

---

## User Story

**As a** media server administrator  
**I want** webhook endpoints to receive notifications from Plex/Jellyfin/Emby/Tautulli  
**So that** subtitle generation is triggered automatically when media is added or played

---

## Acceptance Criteria

- [ ] HTTP server using Fiber framework
- [ ] 5 webhook handlers implemented:
  - [ ] `/plex` - Plex Media Server webhooks
  - [ ] `/jellyfin` - Jellyfin webhooks
  - [ ] `/emby` - Emby webhooks
  - [ ] `/tautulli` - Tautulli webhooks
  - [ ] `/asr` - Bazarr ASR endpoint (for later integration)
- [ ] All handlers validate payloads and return appropriate errors
- [ ] Integration with config from STORY_02
- [ ] Placeholder queue integration (STORY_04 will implement actual queue)
- [ ] 15+ tests (3 per handler minimum):
  - Happy path with valid payload
  - Validation errors (missing fields, wrong format)
  - Malformed payload (invalid JSON)
- [ ] GET requests return helpful error messages
- [ ] `/status` endpoint returns version information
- [ ] Work log created

---

## Integration Points

### Legacy Webhook Handlers (subgen.py)

**Locations:**
- Plex: `subgen.py:550-628` 
- Jellyfin: `subgen.py:630-659`
- Emby: `subgen.py:661-685`
- Tautulli: `subgen.py:531-548`
- ASR: `subgen.py:698-802`

### Payload Structures

#### 1. Plex Webhook (`/plex`)

**Validation:**
- User-Agent header must contain "PlexMediaServer"
- Payload is form-encoded with `payload` field containing JSON
- JSON must have `event` field
- For library.new/media.play events: requires `Metadata.ratingKey`

**Payload Structure:**
```json
{
  "event": "library.new",
  "Metadata": {
    "ratingKey": "12345",
    "type": "movie"
  }
}
```

**Events to Process:**
- `library.new` (if PROCESS_ADDED_MEDIA=true)
- `media.play` (if PROCESS_MEDIA_ON_PLAY=true)

**Logic:**
1. Validate User-Agent header
2. Parse form-encoded payload
3. Extract event type
4. Check if event should be processed based on config
5. Extract rating key from metadata
6. Get file path from Plex API (using rating key)
7. Queue transcription task

#### 2. Jellyfin Webhook (`/jellyfin`)

**Validation:**
- User-Agent header must contain "Jellyfin-Server"
- Form-encoded body with fields: `NotificationType`, `file`, `ItemId`

**Events to Process:**
- `ItemAdded` (if PROCESS_ADDED_MEDIA=true)
- `PlaybackStart` (if PROCESS_MEDIA_ON_PLAY=true)

**Logic:**
1. Validate User-Agent header
2. Parse NotificationType from form body
3. Check if event should be processed
4. Extract ItemId
5. Get file path from Jellyfin API (using ItemId)
6. Queue transcription task with Jellyfin metadata for refresh

#### 3. Emby Webhook (`/emby`)

**Validation:**
- Form-encoded body with `data` field containing JSON
- JSON must have `Event` field

**Events to Process:**
- `library.new` (if PROCESS_ADDED_MEDIA=true)
- `playback.start` (if PROCESS_MEDIA_ON_PLAY=true)
- `system.notificationtest` (return success message)

**Payload Structure:**
```json
{
  "Event": "library.new",
  "Item": {
    "Path": "/media/movies/example.mkv"
  }
}
```

**Logic:**
1. Parse form-encoded data field
2. Extract Event type
3. Handle test notification specially
4. Check if event should be processed
5. Extract file path from Item.Path
6. Queue transcription task

#### 4. Tautulli Webhook (`/tautulli`)

**Validation:**
- `source` header must be "Tautulli"
- Form-encoded body with `event` and `file` fields

**Events to Process:**
- `added` (if PROCESS_ADDED_MEDIA=true)
- `played` (if PROCESS_MEDIA_ON_PLAY=true)

**Logic:**
1. Validate source header
2. Parse event and file from body
3. Check if event should be processed
4. Use file path directly (already full path)
5. Queue transcription task

#### 5. ASR Endpoint (`/asr`)

**Purpose:** Direct audio transcription endpoint for Bazarr integration

**Query Parameters:**
- `task` (optional): "transcribe" or "translate" (default: "transcribe")
- `language` (optional): Language code
- `video_file` (optional): Original video filename
- `initial_prompt` (optional): Whisper initial prompt
- `encode` (optional): Whether to encode audio (default: true)
- `output` (optional): Format - "txt", "vtt", "srt", "tsv", "json" (default: "srt")
- `word_timestamps` (optional): Enable word-level timestamps (default: false)

**Request:**
- Multipart form with `audio_file` field
- Audio content is read and hashed for deduplication

**Response:**
- Blocking: waits for transcription to complete
- Returns subtitle content with appropriate Content-Type
- Timeout after configurable duration (ASR_TIMEOUT)

**Logic:**
1. Read audio file from multipart form
2. Generate hash from audio content + task + language
3. Check if identical task already processing (deduplicate)
4. Queue ASR task
5. Block until worker completes (respects CONCURRENT_TRANSCRIPTIONS)
6. Return result or error

---

## Technical Design

### File Structure

```
internal/webhooks/
├── server.go           # Fiber HTTP server setup
├── server_test.go      # Server setup tests
├── plex.go             # Plex webhook handler
├── plex_test.go        # Plex handler tests
├── jellyfin.go         # Jellyfin webhook handler
├── jellyfin_test.go    # Jellyfin handler tests
├── emby.go             # Emby webhook handler
├── emby_test.go        # Emby handler tests
├── tautulli.go         # Tautulli webhook handler
├── tautulli_test.go    # Tautulli handler tests
├── asr.go              # ASR endpoint handler
├── asr_test.go         # ASR endpoint tests
├── common.go           # Shared utilities
└── common_test.go      # Shared utilities tests
```

### Server Setup (server.go)

```go
package webhooks

import (
	"github.com/gofiber/fiber/v2"
	"github.com/sirupsen/logrus"
	"subgen/orchestrator/internal/config"
)

type Server struct {
	app    *fiber.App
	config *config.Config
	queue  QueueInterface // Placeholder until STORY_04
	log    *logrus.Logger
}

type QueueInterface interface {
	Enqueue(task Task) error
}

type Task struct {
	FilePath            string
	TranscriptionType   string // "transcribe" or "translate"
	ForceLanguage       string
	PlexItemID          string
	PlexServer          string
	PlexToken           string
	JellyfinItemID      string
	JellyfinServer      string
	JellyfinToken       string
}

func NewServer(cfg *config.Config, queue QueueInterface, log *logrus.Logger) *Server {
	app := fiber.New(fiber.Config{
		DisableStartupMessage: true,
	})

	s := &Server{
		app:    app,
		config: cfg,
		queue:  queue,
		log:    log,
	}

	s.setupRoutes()
	return s
}

func (s *Server) setupRoutes() {
	// GET handlers (return error messages)
	s.app.Get("/plex", s.handleGetError)
	s.app.Get("/webhook", s.handleGetError)
	s.app.Get("/jellyfin", s.handleGetError)
	s.app.Get("/emby", s.handleGetError)
	s.app.Get("/tautulli", s.handleGetError)
	s.app.Get("/asr", s.handleGetError)

	// Status endpoint
	s.app.Get("/", s.handleRoot)
	s.app.Get("/status", s.handleStatus)

	// POST handlers
	s.app.Post("/plex", s.handlePlex)
	s.app.Post("/jellyfin", s.handleJellyfin)
	s.app.Post("/emby", s.handleEmby)
	s.app.Post("/tautulli", s.handleTautulli)
	s.app.Post("/asr", s.handleASR)
}

func (s *Server) Start() error {
	addr := fmt.Sprintf(":%d", s.config.WebhookPort)
	s.log.Infof("Starting webhook server on %s", addr)
	return s.app.Listen(addr)
}

func (s *Server) Shutdown() error {
	return s.app.Shutdown()
}
```

### Common Utilities (common.go)

```go
package webhooks

import "github.com/gofiber/fiber/v2"

func handleGetError(c *fiber.Ctx) error {
	return c.Status(400).JSON(fiber.Map{
		"error": "You accessed this request incorrectly via a GET request. See https://github.com/McCloudS/subgen for proper configuration",
	})
}

func handleRoot(c *fiber.Ctx) error {
	return c.JSON(fiber.Map{
		"message": "The webui for configuration was removed on 1 October 2024, please configure via environment variables or in your Docker settings.",
	})
}

func handleStatus(c *fiber.Ctx) error {
	return c.JSON(fiber.Map{
		"version": "Subgen Go Orchestrator v0.1.0", // TODO: Get from build ldflags
		"status":  "operational",
	})
}
```

---

## Testing Strategy

### Unit Tests (15+ tests)

**Plex Tests (3+):**
1. Valid library.new event → queues task
2. Missing User-Agent → returns error
3. Malformed JSON payload → returns error
4. Event filtering (PROCESS_ADDED_MEDIA=false) → skips event

**Jellyfin Tests (3+):**
1. Valid ItemAdded event → queues task
2. Missing User-Agent → returns error
3. Missing ItemId → returns error

**Emby Tests (3+):**
1. Valid library.new event → queues task
2. Test notification → returns success
3. Empty data field → returns empty response

**Tautulli Tests (3+):**
1. Valid added event → queues task
2. Missing source header → returns error
3. Event filtering → skips event

**ASR Tests (3+):**
1. Valid audio file → queues task and returns result
2. Empty audio file → returns error
3. Timeout handling → returns timeout error

### Test Fixtures

Create mock queue that captures enqueued tasks for verification:

```go
type MockQueue struct {
	tasks []Task
}

func (m *MockQueue) Enqueue(task Task) error {
	m.tasks = append(m.tasks, task)
	return nil
}

func (m *MockQueue) GetTasks() []Task {
	return m.tasks
}
```

---

## Dependencies

**Requires:**
- STORY_01 (Project Setup) ✅ Complete
- STORY_02 (Configuration Management) ✅ Complete

**Blocks:**
- STORY_04 (Priority Queue System) - will replace QueueInterface placeholder

---

## Implementation Plan (TDD)

### Phase 1: Server Setup (1-2 hours)
1. Write test for server initialization
2. Implement Server struct and NewServer
3. Write test for GET endpoints returning errors
4. Implement handleGetError and status endpoints
5. Write test for server start/shutdown
6. Implement Start/Shutdown methods

### Phase 2: Plex Handler (2 hours)
1. Write test: valid library.new event
2. Write test: invalid User-Agent
3. Write test: malformed JSON
4. Implement handlePlex
5. Refactor and add logging

### Phase 3: Jellyfin Handler (1.5 hours)
1. Write tests (3+)
2. Implement handleJellyfin
3. Refactor and add logging

### Phase 4: Emby Handler (1.5 hours)
1. Write tests (3+)
2. Implement handleEmby
3. Refactor and add logging

### Phase 5: Tautulli Handler (1.5 hours)
1. Write tests (3+)
2. Implement handleTautulli
3. Refactor and add logging

### Phase 6: ASR Endpoint (2 hours)
1. Write tests (3+)
2. Implement handleASR
3. Refactor and add logging

### Phase 7: Integration (1 hour)
1. Test all endpoints together
2. Update main.go to start server
3. Manual testing with curl

---

## Work Log Template

Location: `docs/WORKLOGS/EPIC_01/0003_2026-02-15_STORY_03_webhook_handlers.md`

Include:
- Research findings (payload structures)
- Implementation approach
- Test results
- Known limitations
- Integration points for STORY_04

---

## Definition of Done

- [ ] All 5 webhook handlers implemented
- [ ] 15+ tests written and passing
- [ ] Test coverage > 80% for webhook package
- [ ] Server starts and responds to requests
- [ ] GET requests return helpful errors
- [ ] All handlers integrate with config
- [ ] Mock queue captures tasks correctly
- [ ] Code follows Go best practices (gofmt, golint)
- [ ] Work log created
- [ ] COORDINATION.md updated

---

**Story Owner:** TBD  
**Created:** 2026-02-15  
**Started:** 2026-02-15  
**Completed:** TBD
