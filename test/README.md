# Integration Tests - gRPC Communication

This directory contains integration tests that validate gRPC communication between the Go orchestrator and Python worker.

## Directory Structure

```
test/
├── integration/              # Go integration tests
│   ├── grpc_integration_test.go   # 16 comprehensive test cases
│   ├── go.mod                     # Go module for tests
│   └── go.sum                     # Go dependencies
├── testdata/                # Shared test data
│   ├── short_audio.mp3      # 30 second test audio
│   ├── corrupt_audio.mp3    # Invalid audio file
│   └── video.mkv            # Test video file
├── scripts/                 # Helper scripts
│   ├── generate_test_audio.sh         # Generate test audio files
│   └── run_integration_tests.sh       # Test runner (recommended)
└── docker-compose.grpc-test.yml   # Docker Compose for testing
```

## Prerequisites

- Docker and Docker Compose
- Go 1.25+
- ffmpeg (for generating test audio)

## Quick Start

### 1. Generate Test Audio Files

```bash
cd test/scripts
./generate_test_audio.sh
```

### 2. Run Tests (Recommended Method)

```bash
cd test/scripts
./run_integration_tests.sh
```

This script will:
- Start Docker Compose services
- Wait for services to be healthy
- Run all integration tests
- Show test results

### 3. Run Tests (Manual Method)

```bash
# Start services
cd test
docker-compose -f docker-compose.grpc-test.yml up -d

# Wait for services to be healthy (30-60s)
docker-compose -f docker-compose.grpc-test.yml ps

# Run Go tests
cd integration
go test -v

# Stop services
cd ..
docker-compose -f docker-compose.grpc-test.yml down
```

## Test Coverage

The integration tests cover:

### HealthCheck RPC (3 tests)
- Basic health check functionality
- Repeated calls (10x)
- All protobuf fields populated

### DetectLanguage RPC (3 tests)
- File path input
- Audio bytes input
- Missing audio source (error handling)

### Transcribe RPC (4 tests)
- Basic request validation
- Missing file path (error handling)
- All protobuf fields populated
- Timeout handling

### Concurrent & Stress Tests (2 tests)
- 20 concurrent HealthCheck calls
- Multiple independent clients

### Protocol Validation (4 tests)
- Connection establishment
- Service method availability
- Large metadata maps
- Empty/nil options handling

**Total: 16 integration tests**

## Configuration

### Environment Variables

The Docker Compose configuration uses these key settings:

**Worker (Python):**
- `WHISPER_MODEL=tiny` - Fast model for testing
- `GRPC_PORT=50051`
- `LOG_LEVEL=DEBUG`

**Orchestrator (Go):**
- `WORKER_ADDRESS=worker:50051`
- `WEBHOOK_PORT=9000`
- `METRICS_PORT=9090`

### Custom Configuration

Edit `docker-compose.grpc-test.yml` to customize:
- Whisper model size
- Memory thresholds
- Timeout values
- Log levels

## Debugging

### View Service Logs

```bash
cd test
docker-compose -f docker-compose.grpc-test.yml logs -f
```

### View Specific Service

```bash
# Worker logs
docker-compose -f docker-compose.grpc-test.yml logs -f worker

# Orchestrator logs
docker-compose -f docker-compose.grpc-test.yml logs -f orchestrator
```

### Check Service Health

```bash
docker-compose -f docker-compose.grpc-test.yml ps
```

### Connect to Worker Directly

```bash
# Health check
grpcurl -plaintext localhost:50051 subgen.v1.TranscriptionService/HealthCheck

# List services
grpcurl -plaintext localhost:50051 list
```

## Test Execution Time

- **HealthCheck tests**: ~1 second
- **DetectLanguage tests**: ~5 seconds (stub returns immediately)
- **Transcribe tests**: ~5 seconds (stub returns immediately)
- **Concurrent tests**: ~2 seconds
- **Protocol tests**: ~1 second

**Total: ~15 seconds** (excluding service startup)

Service startup time: 30-60 seconds (Whisper model download)

## Expected Results (Current State)

Since the Python worker currently has stub implementations, the tests validate:

✅ **gRPC Protocol Works**: Connection, method availability, protobuf serialization
✅ **Error Handling Works**: Proper gRPC status codes (INVALID_ARGUMENT, etc.)
✅ **HealthCheck Works**: Returns real worker status
✅ **Stub Responses**: DetectLanguage and Transcribe return "not yet implemented" errors

When EPIC_02 STORY_02 completes the actual implementation:
- DetectLanguage will return language codes
- Transcribe will return subtitle paths
- All tests will pass with real data

## CI/CD Integration

Add to `.github/workflows/integration-tests.yml`:

```yaml
name: Integration Tests

on: [push, pull_request]

jobs:
  integration-tests:
    runs-on: ubuntu-latest
    steps:
      - uses: actions/checkout@v3
      
      - name: Set up Go
        uses: actions/setup-go@v4
        with:
          go-version: '1.25'
      
      - name: Generate test audio
        run: |
          cd test/scripts
          ./generate_test_audio.sh
      
      - name: Run integration tests
        run: |
          cd test/scripts
          ./run_integration_tests.sh -stop
```

## Troubleshooting

### Services won't start
- Check Docker is running: `docker ps`
- Check ports 9000, 9090, 50051 are available
- View logs: `docker-compose logs`

### Tests timeout
- Increase wait time in script
- Check worker health: `docker inspect subgen-worker-integration-test`
- May need to download Whisper model (first run takes longer)

### Connection refused
- Ensure services are healthy: `docker-compose ps`
- Check worker is listening on :50051: `docker logs subgen-worker-integration-test`

## Related Documentation

- [EPIC_03 STORY_01](../../docs/BACKLOG/EPIC_03/stories/STORY_01_grpc_integration_tests.md) - Full story specification
- [01_GRPC_PROTOCOL.md](../../docs/DESIGN/01_GRPC_PROTOCOL.md) - gRPC protocol design
- [api/transcription.proto](../../api/transcription.proto) - Protobuf schema

## Work Log

- **Created**: 2026-02-15
- **Story**: EPIC_03 STORY_01 - gRPC Integration Tests
- **Status**: Complete (16/16 tests)
