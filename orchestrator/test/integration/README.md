# Webhook Integration Tests

Comprehensive integration tests for webhook handlers validating the flow: `webhook → queue → dispatch`.

## Test Files

1. **`webhook_integration_test.go`** (675 lines) - 15 test cases
2. **`mock_media_server.go`** (193 lines) - Mock Plex/Jellyfin API server
3. **`webhook_payloads.go`** (96 lines) - Realistic webhook payloads

## Running Tests

```bash
# From test/integration directory
go test -v

# Run specific test
go test -v -run TestPlex_LibraryNew

# With race detector
go test -race -v

# With coverage
go test -cover -v
```

## Test Coverage

### Webhook Types (15 tests)
- **Plex**: 7 tests (library.new, media.play, errors, filtering, deduplication, API failures, queue full)
- **Jellyfin**: 2 tests (ItemAdded, PlaybackStart)
- **Emby**: 2 tests (library.new, test notification)
- **Tautulli**: 1 test (added)
- **Edge Cases**: 3 tests (duplicates, concurrent requests, multi-source webhooks)

### Test Categories
- **Happy Paths**: 6 tests
- **Error Scenarios**: 5 tests  
- **Edge Cases**: 4 tests

## Test Architecture

```
Test → Fiber App (in-memory) → Webhook Handler → Queue Adapter → Queue
         ↓
    Mock Media Server (HTTP)
```

## Current Status

**Compilation**: ✅ All tests compile  
**Execution**: ✅ All tests run  
**Pass Rate**: 13/15 tests ready (2 blocked by media server integration)

### Expected Failures

Two tests are expected to fail until media server clients are integrated into webhook handlers:

1. **TestPlex_LibraryNew_Success** - Expects media server API call
2. **TestWebhook_ConcurrentRequests** - Expects unique file paths from API

**Reason**: Webhook handlers in `internal/webhooks/server.go` are currently stubs that:
- Accept webhooks ✅
- Queue tasks ✅
- But don't fetch file paths from media server APIs yet ⏳

**When Fixed**: When `internal/mediaserver` clients are wired into webhook handlers, all tests will pass.

## Mock Media Server

The `MockMediaServer` simulates:
- **Plex API**: `/library/metadata/:id` → file path
- **Jellyfin API**: `/Items/:id` → file path, `/Users` → admin user

Features:
- Thread-safe (sync.Mutex)
- Call counting for verification
- Configurable responses
- Failure simulation
- Proper cleanup

## Test Data

**Location**: `../testdata/`

- `short_audio.wav` - 2-second 16kHz mono audio file for testing

## Integration with Other Components

**Dependencies**:
- `internal/config` - Configuration management ✅
- `internal/queue` - Priority queue with deduplication ✅
- `internal/webhooks` - Webhook handlers ✅
- `internal/mediaserver` - Plex/Jellyfin clients ⏳ (not wired yet)

**Blocks**:
- End-to-end tests (STORY_03) - needs these webhook tests passing
- Load testing (STORY_05) - needs webhook infrastructure

## Next Steps

1. Wire `internal/mediaserver` into webhook handlers
2. Update webhook handlers to call `GetFilePath()` for Plex/Jellyfin
3. Validate all 15 tests pass
4. Add Docker Compose environment (STORY_01)
5. Add worker dispatch validation

## References

- Story: `docs/BACKLOG/EPIC_03/stories/STORY_02_webhook_integration_tests.md`
- Webhook Handlers: `orchestrator/internal/webhooks/server.go`
- Queue: `orchestrator/internal/queue/queue.go`
- Media Server Clients: `orchestrator/internal/mediaserver/`
