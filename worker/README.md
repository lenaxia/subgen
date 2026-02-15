# Python Worker for Subgen

**Status**: Under Development (EPIC_02, STORY_01 in progress)

This is the Python transcription worker component of the hybrid Go/Python Subgen architecture.

## Overview

The Python worker is responsible for:
- gRPC server listening on port 50051
- Whisper model management (faster-whisper)
- Audio transcription using stable-ts
- Subtitle generation (SRT/LRC)
- Memory monitoring and health reporting

## Directory Structure

```
worker/
├── src/
│   ├── main.py                  # gRPC server entry point
│   ├── grpc_server/             # gRPC server implementation
│   │   ├── __init__.py
│   │   └── server.py            # TranscriptionServicer (TO BE IMPLEMENTED)
│   ├── transcription/           # Transcription logic (TO BE IMPLEMENTED)
│   ├── audio/                   # Audio extraction (TO BE IMPLEMENTED)
│   ├── language/                # Language detection (TO BE IMPLEMENTED)
│   ├── subtitles/               # Subtitle generation (TO BE IMPLEMENTED)
│   ├── config/                  # Configuration management
│   │   ├── __init__.py
│   │   └── settings.py          # Pydantic settings ✅
│   └── utils/                   # Utilities
│       ├── __init__.py
│       └── logging.py           # Structured logging ✅
├── pb/                          # Generated protobuf code
│   └── __init__.py              # Placeholder (needs generation)
├── tests/
│   ├── conftest.py              # Pytest fixtures ✅
│   ├── unit/                    # Unit tests (TO BE IMPLEMENTED)
│   └── integration/             # Integration tests (TO BE IMPLEMENTED)
├── requirements.txt             # Production dependencies ✅
├── requirements-dev.txt         # Development dependencies ✅
├── pyproject.toml               # Tool configuration (pytest, mypy, ruff) ✅
├── generate_proto.sh            # Protobuf generation script ✅
└── README.md                    # This file

✅ = Completed
🚧 = In Progress
⏳ = Not Started
```

## Setup (for development)

### Prerequisites
- Python 3.11+
- FFmpeg (for audio extraction)

### Installation

```bash
# Create virtual environment
python3 -m venv venv
source venv/activate  # Linux/Mac
# or: venv\Scripts\activate  # Windows

# Install dependencies
pip install -r requirements.txt
pip install -r requirements-dev.txt

# Generate protobuf code
./generate_proto.sh
```

## Configuration

Configuration is managed via environment variables (pydantic-settings):

| Variable | Default | Description |
|----------|---------|-------------|
| `GRPC_HOST` | 0.0.0.0 | gRPC server bind address |
| `GRPC_PORT` | 50051 | gRPC server port |
| `WHISPER_MODEL` | medium | Whisper model size |
| `WHISPER_THREADS` | 4 | CPU threads for transcription |
| `DEVICE` | cpu | Device for inference (cpu/cuda) |
| `MEMORY_THRESHOLD_MB` | 3000 | Memory threshold for model unload |
| `MODEL_CLEANUP_DELAY` | 30 | Seconds before unloading model |

See `src/config/settings.py` for full list.

## Running the Worker

```bash
# Development (from worker/ directory)
python -m src.main

# Production (with Docker)
docker build -t subgen-worker .
docker run -p 50051:50051 subgen-worker
```

## Testing

```bash
# Run all tests
pytest

# Run with coverage
pytest --cov=src --cov-report=html

# Run only unit tests
pytest tests/unit/

# Run only integration tests
pytest tests/integration/
```

## Development Workflow (STORY_01)

STORY_01 focuses on gRPC server setup. Implementation order:

1. ✅ **Project scaffolding** - Directory structure, requirements.txt, config
2. ✅ **Configuration** - Pydantic settings for type-safe config
3. ✅ **Logging** - Structured JSON logging
4. ✅ **Test fixtures** - Pytest conftest.py with mocks
5. 🚧 **Protobuf generation** - Generate Python gRPC code from .proto
6. ⏳ **gRPC server** - TranscriptionServicer implementation
7. ⏳ **Service stubs** - Transcribe, DetectLanguage, HealthCheck methods
8. ⏳ **Unit tests** - Test all RPC methods with mocks
9. ⏳ **Integration tests** - End-to-end gRPC server tests
10. ⏳ **Docker image** - Containerization
11. ⏳ **Work log** - Document completion

## Memory Leak Prevention

This implementation fixes 3 critical memory leaks from legacy subgen.py:

1. **No task_results dict** - Worker is stateless, orchestrator manages state
2. **ModelManager with hard limits** - Immediate unload on memory threshold
3. **Context managers** - All resources (BytesIO, files) use `with` statements

See `docs/DESIGN/02_MEMORY_MANAGEMENT.md` for details.

## gRPC Protocol

The worker implements 3 RPC methods defined in `api/transcription.proto`:

- **Transcribe**: Transcribe audio file to subtitles (SRT/LRC)
- **DetectLanguage**: Detect audio language from sample
- **HealthCheck**: Report worker health and memory usage

## Integration with Orchestrator

- **Phase 1**: Single pod, orchestrator → worker via localhost:50051
- **Phase 2**: Separate deployments, orchestrator → workers via K8s Service

## Next Steps (for STORY_01 completion)

1. Install dependencies: `pip install -r requirements.txt -r requirements-dev.txt`
2. Generate protobuf code: `./generate_proto.sh`
3. Implement `grpc_server/server.py` with TranscriptionServicer
4. Implement stub methods (return mock responses)
5. Write unit tests for each RPC method
6. Run tests: `pytest -v`
7. Create Dockerfile
8. Create work log in `docs/WORKLOGS/EPIC_02/`

## Related Documentation

- **README-LLM.md** - Development workflow, TDD, critical rules
- **EPIC_02/README.md** - Epic overview, 5 stories
- **00_HYBRID_ARCHITECTURE.md** - System architecture
- **02_MEMORY_MANAGEMENT.md** - Memory leak fixes (CRITICAL)
- **api/transcription.proto** - gRPC protocol definition

## License

Same as parent project.
