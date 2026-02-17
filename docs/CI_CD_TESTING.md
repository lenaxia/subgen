# CI/CD Testing Strategy

## Overview

This document describes the comprehensive testing strategy implemented in the Subgen project's CI/CD pipelines.

## Test Workflows

### 1. Go Orchestrator Tests (`test-orchestrator.yml`)

Comprehensive testing for the Go orchestrator component.

#### Test Stages

**Unit Tests**
- Runs all unit tests in `orchestrator/internal/*/`
- Uses race detector (`-race`) to catch data races
- Generates code coverage reports
- Uploads coverage to Codecov
- Matrix: Go 1.25

**Integration Tests**
- Tests webhook handlers with mock media servers
- Tests queue and deduplication logic
- Tests skip logic with real FFprobe data
- Requires FFmpeg/FFprobe installed
- Timeout: 5 minutes

**Real-World Tests**
- Tests skip logic against actual media files in `test/testdata/`
- Tests language detection with real audio samples
- Tests batch processing functionality
- Validates FFprobe integration with sample files
- Uses actual test data from testdata directory

**Benchmark Tests**
- Runs Go benchmarks for performance regression testing
- Runs for 5 seconds per benchmark
- Generates benchmark reports

**Lint**
- Uses golangci-lint for code quality checks
- 5-minute timeout

**Summary**
- Aggregates all test results
- Fails if any test stage fails

### 2. Python Worker Tests (`test-worker.yml`)

Comprehensive testing for the Python worker component.

#### Test Stages

**Unit Tests**
- Tests in `worker/tests/unit/`
- Python version matrix: 3.11, 3.12
- Includes coverage reporting
- Tests all core modules:
  - Language detection
  - Audio extraction
  - Transcription engine
  - Subtitle writer
  - gRPC server
  - Config management
  - Error handling
- Uploads coverage to Codecov

**Integration Tests**
- Tests in `worker/tests/integration/`
- Tests gRPC server integration
- Tests end-to-end flows
- Uses real test data
- Timeout: 300 seconds (5 minutes)

**Memory Leak Tests**
- Dedicated tests for memory leak prevention
- Critical for long-running worker processes
- Tests model loading/unloading
- Tests CUDA memory cleanup
- Timeout: 600 seconds (10 minutes)

**Real-World Transcription Tests**
- Downloads Whisper tiny model
- Tests language detection on real audio samples
- Tests audio extraction from video files
- Tests full transcription pipeline with tiny model
- Uses actual files from `test/testdata/`:
  - `short_audio.mp3`
  - `demo_video_speech.mp4`
  - Other sample media files
- Timeout: 5 minutes

**Lint**
- Ruff for linting
- Black for formatting
- isort for import sorting
- mypy for type checking

**Summary**
- Aggregates all test results
- Fails if any test stage fails

### 3. End-to-End Integration Tests (`test-e2e.yml`)

System-level integration tests that verify the complete orchestrator + worker stack.

#### Test Stages

**gRPC Orchestrator-Worker Integration**
- Starts Python worker with tiny model
- Runs gRPC integration tests from `test/integration/`
- Validates:
  - HealthCheck RPC
  - DetectLanguage RPC
  - Transcribe RPC
  - Protocol compliance
  - Error handling
- Timeout: 10 minutes

**Webhook to Transcription Flow**
- Starts both orchestrator and worker
- Tests complete webhook → transcription flow
- Tests endpoints:
  - `/status` - Health check
  - `/batch` - Batch transcription
  - `/detect-language` - Language detection
- Uses real audio files from testdata
- Validates subtitle file generation
- Timeout: 30 seconds for processing

**Docker Compose Integration**
- Builds both services as Docker containers
- Tests orchestrator + worker via Docker Compose
- Validates health checks
- Tests HTTP endpoints
- Captures logs for debugging

**Summary**
- Aggregates all E2E test results
- Fails if any test stage fails

## Build Workflows with Test Dependencies

### Updated Build Strategy

All build workflows now require successful test runs before building Docker images:

**build-go.yml**
1. Calls `test-orchestrator.yml` as reusable workflow
2. Verifies tests passed
3. Only then proceeds with build
4. Lint also depends on test success

**build_GPU.yml**
1. Runs `test-worker.yml`
2. Runs `test-orchestrator.yml`
3. Runs `test-e2e.yml`
4. Verifies all tests passed
5. Only then builds GPU Docker image

**build_CPU.yml**
1. Runs `test-worker.yml`
2. Runs `test-orchestrator.yml`
3. Runs `test-e2e.yml`
4. Verifies all tests passed
5. Only then builds CPU Docker image for amd64 + arm64

## Sample Data

Test data is located in `test/testdata/`:

```
test/testdata/
├── short_audio.mp3          # Short audio for quick tests
├── speech_sample.wav        # Speech sample for detection
├── demo_video_speech.mp4    # Video with speech
├── video.mkv                # Video file for extraction tests
├── sample.mp3               # Sample audio file
├── corrupt_audio.mp3        # Corrupted file for error handling
└── media/
    └── test_movie.wav       # Test movie audio
```

Additional test data in `orchestrator/internal/skip/testdata/`:
- FFprobe JSON outputs for various scenarios
- Used for skip logic testing

## Coverage Reporting

- Unit test coverage uploaded to Codecov
- Coverage reports available as artifacts
- HTML coverage reports generated for both Go and Python
- Separate flags for different test types:
  - `orchestrator-unit`
  - `worker-unit-py3.11`
  - `worker-unit-py3.12`

## Continuous Integration Flow

```
┌─────────────────────────────────────────┐
│  Push to main / Pull Request           │
└─────────────────────────────────────────┘
                  ↓
    ┌─────────────────────────────┐
    │  Parallel Test Execution    │
    ├─────────────────────────────┤
    │  • Orchestrator Tests       │
    │  • Worker Tests             │
    │  • E2E Integration Tests    │
    └─────────────────────────────┘
                  ↓
    ┌─────────────────────────────┐
    │  All Tests Must Pass        │
    └─────────────────────────────┘
                  ↓
    ┌─────────────────────────────┐
    │  Build Docker Images        │
    │  • GPU Image (self-hosted)  │
    │  • CPU Image (multi-arch)   │
    └─────────────────────────────┘
                  ↓
    ┌─────────────────────────────┐
    │  Push to Docker Hub         │
    │  • mccloud/subgen:latest    │
    │  • mccloud/subgen:cpu       │
    │  • mccloud/subgen:VERSION   │
    └─────────────────────────────┘
```

## Quality Gates

All of the following must pass before Docker images are built:

1. ✅ Go unit tests (with race detector)
2. ✅ Go integration tests
3. ✅ Go real-world tests with sample data
4. ✅ Go lint (golangci-lint)
5. ✅ Python unit tests (Python 3.11 + 3.12)
6. ✅ Python integration tests
7. ✅ Python memory leak tests
8. ✅ Python real-world transcription tests
9. ✅ Python lint (ruff, black, isort, mypy)
10. ✅ E2E gRPC integration tests
11. ✅ E2E webhook-to-transcription tests
12. ✅ E2E Docker Compose tests

## Running Tests Locally

### Go Orchestrator Tests
```bash
cd orchestrator

# Unit tests
go test -v -race ./internal/...

# Integration tests
go test -v -race ./test/integration/...

# All tests with coverage
go test -v -race -coverprofile=coverage.out ./...
go tool cover -html=coverage.out
```

### Python Worker Tests
```bash
cd worker

# Install dependencies
pip install -r requirements.txt
pip install pytest pytest-cov pytest-asyncio pytest-mock

# Unit tests
pytest tests/unit/ -v

# Integration tests
pytest tests/integration/ -v

# All tests with coverage
pytest tests/ -v --cov=src --cov-report=html
```

### E2E Integration Tests
```bash
# Start worker in one terminal
cd worker
WHISPER_MODEL=tiny TRANSCRIBE_DEVICE=cpu python src/main.py

# In another terminal, run integration tests
cd test/integration
go test -v ./...
```

## Test Maintenance

- Add new test files to appropriate directories
- Update sample data in `test/testdata/` as needed
- Keep test execution time reasonable (< 10 minutes per workflow)
- Use tiny Whisper model in CI for speed
- Mock external services (Plex, Jellyfin, etc.)
- Clean up temporary files in tests

## Debugging Failed Tests

1. Check workflow logs in GitHub Actions
2. Download artifacts (coverage reports, benchmarks)
3. Look for specific failure messages
4. Reproduce locally using commands above
5. Check if sample data files are missing
6. Verify FFmpeg/FFprobe are installed
7. For E2E tests, check service startup logs

## Future Improvements

- [ ] Add performance regression tests
- [ ] Add load testing for webhook handlers
- [ ] Test with larger media files
- [ ] Add tests for all media server integrations
- [ ] Test path mapping logic more thoroughly
- [ ] Add chaos engineering tests (network failures, etc.)
- [ ] Measure and track test execution time
- [ ] Add mutation testing
