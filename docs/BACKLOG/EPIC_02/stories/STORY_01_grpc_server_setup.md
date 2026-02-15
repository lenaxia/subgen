# Story 01: gRPC Server Setup & Implementation

**Epic**: EPIC_02 - Python Worker Refactor  
**Status**: Not Started  
**Priority**: Critical  
**Estimated Effort**: 6-8 hours  
**Assignee**: TBD

---

## User Story

As a **Go orchestrator developer**,  
I want **a gRPC server in the Python worker that implements all 3 RPC methods**,  
So that **the orchestrator can communicate with the worker to perform transcriptions**.

---

## Background

The new architecture requires the Python worker to expose a gRPC API instead of HTTP/FastAPI. The worker must implement three RPC methods as defined in `api/transcription.proto`:

1. **Transcribe** - Main transcription workload
2. **DetectLanguage** - Language detection from audio sample
3. **HealthCheck** - Worker health monitoring

This story focuses on setting up the gRPC server infrastructure and implementing the RPC method handlers. The actual transcription logic will be refactored in subsequent stories.

**Current State (Legacy)**:
- FastAPI HTTP server with endpoints like `/asr`, `/detect_language`, etc.
- Location: `subgen.py:203-804`

**Target State**:
- Python gRPC server with TranscriptionServiceServicer
- Clean separation of gRPC layer from business logic
- Type-safe message handling with protobuf

---

## Acceptance Criteria

- [ ] gRPC server dependencies installed (grpcio, grpcio-tools)
- [ ] Protobuf code generated for Python (`transcription_pb2.py`, `transcription_pb2_grpc.py`)
- [ ] `worker/` directory structure created
- [ ] `worker/server/grpc_server.py` created with TranscriptionServicer class
- [ ] All 3 RPC methods implemented with proper signatures
- [ ] Transcribe RPC method handler implemented (delegates to transcription engine)
- [ ] DetectLanguage RPC method handler implemented
- [ ] HealthCheck RPC method handler implemented
- [ ] Server graceful shutdown implemented
- [ ] Configuration loaded from environment variables
- [ ] Unit tests for each RPC method (mocked business logic)
- [ ] Integration test with gRPC client
- [ ] Server starts successfully on port 50051
- [ ] Work log created

---

## Technical Design

### Directory Structure

```
worker/
├── __init__.py
├── main.py                           # Entry point, starts gRPC server
├── server/
│   ├── __init__.py
│   └── grpc_server.py                # gRPC service implementation
├── transcription/
│   ├── __init__.py
│   └── engine.py                     # Business logic (placeholder for now)
├── generated/                        # Protobuf generated code
│   ├── __init__.py
│   ├── transcription_pb2.py
│   ├── transcription_pb2.pyi
│   └── transcription_pb2_grpc.py
├── config/
│   ├── __init__.py
│   └── settings.py                   # Configuration (pydantic-settings)
├── utils/
│   ├── __init__.py
│   └── logging_config.py             # Structured logging setup
├── tests/
│   ├── __init__.py
│   ├── conftest.py
│   ├── unit/
│   │   └── test_grpc_server.py
│   └── integration/
│       └── test_grpc_integration.py
├── requirements.txt
├── requirements-dev.txt
└── pytest.ini
```

### gRPC Server Implementation

**File: `worker/server/grpc_server.py`**

```python
import grpc
import logging
from concurrent import futures
from typing import Optional

from generated import transcription_pb2
from generated import transcription_pb2_grpc
from transcription.engine import TranscriptionEngine
from utils.memory import MemoryMonitor
from config.settings import Config

logger = logging.getLogger(__name__)


class TranscriptionServicer(transcription_pb2_grpc.TranscriptionServiceServicer):
    """gRPC service implementation for transcription worker."""
    
    def __init__(self, config: Config):
        self.config = config
        self.engine = TranscriptionEngine(config)
        self.memory_monitor = MemoryMonitor(config.memory_threshold_mb)
        self.stats = {
            'jobs_processed': 0,
            'jobs_active': 0,
        }
        self.start_time = None
        
    def Transcribe(
        self,
        request: transcription_pb2.TranscribeRequest,
        context: grpc.ServicerContext
    ) -> transcription_pb2.TranscribeResponse:
        """Transcribe audio file to subtitles."""
        logger.info(f"Transcribe request received: {request.file_path}")
        
        self.stats['jobs_active'] += 1
        
        try:
            # Validate request
            if not request.file_path:
                context.abort(
                    grpc.StatusCode.INVALID_ARGUMENT,
                    "file_path is required"
                )
            
            # Delegate to transcription engine
            result = self.engine.transcribe(
                file_path=request.file_path,
                task_type=request.task_type,
                force_language=request.force_language or None,
                options=request.options
            )
            
            self.stats['jobs_processed'] += 1
            
            return transcription_pb2.TranscribeResponse(
                success=True,
                subtitle_path=result.subtitle_path,
                detected_language=result.detected_language,
                stats=transcription_pb2.TranscriptionStats(
                    duration_seconds=result.duration_seconds,
                    segment_count=result.segment_count,
                    transcription_time_ms=result.transcription_time_ms,
                    peak_memory_mb=result.peak_memory_mb
                )
            )
            
        except FileNotFoundError as e:
            logger.error(f"File not found: {e}")
            context.abort(grpc.StatusCode.NOT_FOUND, str(e))
            
        except MemoryError as e:
            logger.error(f"Out of memory: {e}")
            context.abort(grpc.StatusCode.RESOURCE_EXHAUSTED, str(e))
            
        except Exception as e:
            logger.exception("Transcription failed")
            context.abort(grpc.StatusCode.INTERNAL, str(e))
            
        finally:
            self.stats['jobs_active'] -= 1
    
    def DetectLanguage(
        self,
        request: transcription_pb2.DetectLanguageRequest,
        context: grpc.ServicerContext
    ) -> transcription_pb2.DetectLanguageResponse:
        """Detect language from audio sample."""
        logger.info("DetectLanguage request received")
        
        try:
            # Extract file path or audio content
            if request.HasField('file_path'):
                source = request.file_path
            elif request.HasField('audio_content'):
                source = request.audio_content
            else:
                context.abort(
                    grpc.StatusCode.INVALID_ARGUMENT,
                    "Either file_path or audio_content is required"
                )
            
            # Delegate to engine
            result = self.engine.detect_language(
                source=source,
                sample_length=request.sample_length or 30,
                sample_offset=request.sample_offset or 0
            )
            
            return transcription_pb2.DetectLanguageResponse(
                success=True,
                language_code=result.language_code,
                language_name=result.language_name,
                confidence=result.confidence
            )
            
        except Exception as e:
            logger.exception("Language detection failed")
            return transcription_pb2.DetectLanguageResponse(
                success=False,
                error_message=str(e)
            )
    
    def HealthCheck(
        self,
        request: transcription_pb2.HealthCheckRequest,
        context: grpc.ServicerContext
    ) -> transcription_pb2.HealthCheckResponse:
        """Check worker health and memory usage."""
        import time
        
        mem_status = self.memory_monitor.check_memory()
        
        # Determine health status
        if mem_status['healthy']:
            status = transcription_pb2.HealthCheckResponse.HEALTHY
        else:
            status = transcription_pb2.HealthCheckResponse.UNHEALTHY
        
        uptime = int(time.time() - self.start_time) if self.start_time else 0
        
        return transcription_pb2.HealthCheckResponse(
            status=status,
            memory_mb=mem_status['current_mb'],
            model_loaded=self.engine.is_model_loaded(),
            jobs_processed=self.stats['jobs_processed'],
            jobs_active=self.stats['jobs_active'],
            version="1.0.0",
            uptime_seconds=uptime
        )


def serve(config: Config):
    """Start gRPC server."""
    import time
    
    server = grpc.server(
        futures.ThreadPoolExecutor(max_workers=config.max_workers),
        options=[
            ('grpc.max_send_message_length', 50 * 1024 * 1024),  # 50MB
            ('grpc.max_receive_message_length', 50 * 1024 * 1024),
        ]
    )
    
    servicer = TranscriptionServicer(config)
    servicer.start_time = time.time()
    
    transcription_pb2_grpc.add_TranscriptionServiceServicer_to_server(
        servicer,
        server
    )
    
    server.add_insecure_port(f'[::]:{config.grpc_port}')
    
    logger.info(f"Starting gRPC server on port {config.grpc_port}")
    server.start()
    
    try:
        server.wait_for_termination()
    except KeyboardInterrupt:
        logger.info("Shutting down gRPC server...")
        server.stop(grace=5)
```

**File: `worker/main.py`**

```python
import logging
from config.settings import load_config
from server.grpc_server import serve
from utils.logging_config import setup_logging


def main():
    """Entry point for worker process."""
    # Load configuration
    config = load_config()
    
    # Setup logging
    setup_logging(config.log_level)
    
    logger = logging.getLogger(__name__)
    logger.info(f"Worker starting with config: {config}")
    
    # Start gRPC server
    serve(config)


if __name__ == '__main__':
    main()
```

---

## Integration Points

### Legacy Code Integration (Temporary)

For this story, we'll create placeholder implementations that return mock data:

**Location: `worker/transcription/engine.py`**

```python
from dataclasses import dataclass
from typing import Optional


@dataclass
class TranscriptionResult:
    subtitle_path: str
    detected_language: str
    duration_seconds: float
    segment_count: int
    transcription_time_ms: int
    peak_memory_mb: int


@dataclass
class LanguageDetectionResult:
    language_code: str
    language_name: str
    confidence: float


class TranscriptionEngine:
    """Placeholder transcription engine (will be implemented in STORY_02)."""
    
    def __init__(self, config):
        self.config = config
        self._model_loaded = False
    
    def transcribe(self, file_path: str, task_type: str, 
                   force_language: Optional[str], options) -> TranscriptionResult:
        """Placeholder - will be implemented in STORY_02."""
        raise NotImplementedError("Transcription logic not yet implemented")
    
    def detect_language(self, source, sample_length: int, 
                       sample_offset: int) -> LanguageDetectionResult:
        """Placeholder - will be implemented in STORY_02."""
        raise NotImplementedError("Language detection not yet implemented")
    
    def is_model_loaded(self) -> bool:
        return self._model_loaded
```

### Protobuf Code Generation

```bash
# From repository root
cd api

# Generate Python code
python -m grpc_tools.protoc \
  -I. \
  --python_out=../worker/generated \
  --pyi_out=../worker/generated \
  --grpc_python_out=../worker/generated \
  transcription.proto
```

### Configuration Management

**File: `worker/config/settings.py`**

```python
from pydantic_settings import BaseSettings
from typing import Optional


class Config(BaseSettings):
    """Worker configuration from environment variables."""
    
    # gRPC Server
    grpc_port: int = 50051
    max_workers: int = 4
    
    # Memory Management
    memory_threshold_mb: int = 3000
    
    # Logging
    log_level: str = "INFO"
    
    # Whisper Model (placeholder for STORY_03)
    whisper_model: str = "medium"
    model_path: str = "./models"
    
    class Config:
        env_file = ".env"
        env_file_encoding = "utf-8"


def load_config() -> Config:
    """Load configuration from environment."""
    return Config()
```

---

## Testing Strategy

### Unit Tests

**File: `worker/tests/unit/test_grpc_server.py`**

```python
import pytest
from unittest.mock import Mock, patch
from generated import transcription_pb2
from server.grpc_server import TranscriptionServicer
from config.settings import Config


@pytest.fixture
def config():
    return Config()


@pytest.fixture
def servicer(config):
    return TranscriptionServicer(config)


@pytest.fixture
def mock_context():
    context = Mock()
    context.abort = Mock(side_effect=Exception("gRPC abort"))
    return context


def test_transcribe_validates_file_path(servicer, mock_context):
    """Test that Transcribe validates file_path."""
    request = transcription_pb2.TranscribeRequest(
        file_path="",  # Empty path
        task_type="transcribe"
    )
    
    with pytest.raises(Exception, match="gRPC abort"):
        servicer.Transcribe(request, mock_context)
    
    mock_context.abort.assert_called_once()
    args = mock_context.abort.call_args[0]
    assert args[0].name == 'INVALID_ARGUMENT'


def test_detect_language_requires_source(servicer, mock_context):
    """Test that DetectLanguage requires file_path or audio_content."""
    request = transcription_pb2.DetectLanguageRequest()
    # No file_path or audio_content set
    
    with pytest.raises(Exception, match="gRPC abort"):
        servicer.DetectLanguage(request, mock_context)
    
    mock_context.abort.assert_called_once()


def test_health_check_returns_status(servicer, mock_context):
    """Test HealthCheck returns current status."""
    request = transcription_pb2.HealthCheckRequest()
    
    response = servicer.HealthCheck(request, mock_context)
    
    assert response.status in [
        transcription_pb2.HealthCheckResponse.HEALTHY,
        transcription_pb2.HealthCheckResponse.UNHEALTHY,
    ]
    assert response.memory_mb > 0
    assert response.jobs_processed >= 0


def test_transcribe_updates_stats(servicer, mock_context):
    """Test that Transcribe updates job statistics."""
    initial_processed = servicer.stats['jobs_processed']
    
    request = transcription_pb2.TranscribeRequest(
        file_path="/test/video.mp4",
        task_type="transcribe",
        options=transcription_pb2.TranscribeOptions(
            whisper_model="tiny"
        )
    )
    
    # Mock the engine to return success
    mock_result = Mock()
    mock_result.subtitle_path = "/test/video.srt"
    mock_result.detected_language = "en"
    mock_result.duration_seconds = 120.5
    mock_result.segment_count = 42
    mock_result.transcription_time_ms = 5000
    mock_result.peak_memory_mb = 1500
    
    with patch.object(servicer.engine, 'transcribe', return_value=mock_result):
        response = servicer.Transcribe(request, mock_context)
    
    assert response.success
    assert servicer.stats['jobs_processed'] == initial_processed + 1
```

### Integration Tests

**File: `worker/tests/integration/test_grpc_integration.py`**

```python
import pytest
import grpc
from concurrent import futures
from generated import transcription_pb2, transcription_pb2_grpc
from server.grpc_server import TranscriptionServicer
from config.settings import Config


@pytest.fixture
def grpc_server():
    """Start gRPC server for integration testing."""
    config = Config(grpc_port=50052)  # Use different port
    
    server = grpc.server(futures.ThreadPoolExecutor(max_workers=2))
    servicer = TranscriptionServicer(config)
    transcription_pb2_grpc.add_TranscriptionServiceServicer_to_server(
        servicer, server
    )
    server.add_insecure_port('[::]:50052')
    server.start()
    
    yield server
    
    server.stop(grace=1)


@pytest.fixture
def grpc_client(grpc_server):
    """Create gRPC client for testing."""
    channel = grpc.insecure_channel('localhost:50052')
    client = transcription_pb2_grpc.TranscriptionServiceStub(channel)
    yield client
    channel.close()


def test_health_check_integration(grpc_client):
    """Test HealthCheck RPC end-to-end."""
    request = transcription_pb2.HealthCheckRequest()
    response = grpc_client.HealthCheck(request)
    
    assert response.status == transcription_pb2.HealthCheckResponse.HEALTHY
    assert response.memory_mb > 0
    assert response.version == "1.0.0"


def test_detect_language_integration(grpc_client):
    """Test DetectLanguage RPC (will fail until STORY_02)."""
    request = transcription_pb2.DetectLanguageRequest(
        file_path="/test/audio.mp3",
        sample_length=30
    )
    
    # Should return error for now (not implemented)
    response = grpc_client.DetectLanguage(request)
    assert not response.success
    assert "not yet implemented" in response.error_message.lower()
```

---

## Definition of Done

- [ ] All acceptance criteria met
- [ ] gRPC server starts successfully on port 50051
- [ ] All 3 RPC methods implemented with proper signatures
- [ ] Unit tests passing (8+ tests)
- [ ] Integration tests passing (2+ tests)
- [ ] Code follows Python style guide (PEP 8)
- [ ] Type hints throughout (mypy checks pass)
- [ ] Documentation strings for all public methods
- [ ] Work log created: `docs/WORKLOGS/NNNN_YYYY-MM-DD_EPIC_02_story_01_grpc_server.md`
- [ ] Code committed and pushed
- [ ] Next story (STORY_02) can begin

---

## Validation Commands

```bash
# Generate protobuf code
cd api
python -m grpc_tools.protoc -I. \
  --python_out=../worker/generated \
  --pyi_out=../worker/generated \
  --grpc_python_out=../worker/generated \
  transcription.proto

# Install dependencies
cd worker
pip install -r requirements.txt
pip install -r requirements-dev.txt

# Run tests
pytest tests/ -v

# Run server (test manually)
python main.py

# Test with grpcurl (if available)
grpcurl -plaintext localhost:50051 list
grpcurl -plaintext localhost:50051 \
  subgen.v1.TranscriptionService/HealthCheck
```

---

## Dependencies

**Requirements:**
- None (first story in EPIC_02)

**Blocks:**
- STORY_02 (Modular Refactor) - needs gRPC server structure
- STORY_03 (Model Lifecycle) - needs gRPC server to integrate
- STORY_04 (Memory Leaks) - needs server to test
- STORY_05 (Configuration) - needs server to configure

---

## References

- [01_GRPC_PROTOCOL.md](../../DESIGN/01_GRPC_PROTOCOL.md) - gRPC protocol specification
- [api/transcription.proto](../../../api/transcription.proto) - Protobuf schema
- gRPC Python Tutorial: https://grpc.io/docs/languages/python/basics/
- pydantic-settings: https://docs.pydantic.dev/latest/concepts/pydantic_settings/

---

**Created**: 2026-02-15  
**Last Updated**: 2026-02-15
