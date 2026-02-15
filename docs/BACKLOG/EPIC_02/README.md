# EPIC_02: Python Worker Refactor

**Status:** Not Started  
**Estimated Effort:** 34-44 hours  
**Duration:** 1 week  
**Can Parallelize:** ✅ Yes (with EPIC_01)

---

## Overview

Refactor monolithic `subgen.py` (2,144 lines) into clean, modular Python worker with **all memory leaks fixed**. The worker exposes a gRPC server for transcription requests from the orchestrator. Focus on testability, memory management, and maintainability.

---

## Goals

1. Extract transcription logic into modular components
2. Implement gRPC server (not HTTP/FastAPI)
3. Fix all 3 confirmed memory leaks
4. Add comprehensive type hints throughout
5. Implement proper resource cleanup (context managers)
6. Memory monitoring and health reporting
7. Production-ready Python worker

---

## Design References

- [00_HYBRID_ARCHITECTURE.md](../../DESIGN/00_HYBRID_ARCHITECTURE.md) - System architecture
- [01_GRPC_PROTOCOL.md](../../DESIGN/01_GRPC_PROTOCOL.md) - gRPC protocol
- [02_MEMORY_MANAGEMENT.md](../../DESIGN/02_MEMORY_MANAGEMENT.md) - **CRITICAL** - Memory leak fixes

---

## User Stories

### [STORY_01: gRPC Server Implementation](./stories/STORY_01_grpc_server.md)
**Status:** Not Started  
**Effort:** 6-8 hours  
**Summary:** Python gRPC server, RPC method implementations, graceful shutdown

### [STORY_02: Modular Refactor](./stories/STORY_02_modular_refactor.md)
**Status:** Not Started  
**Effort:** 10-12 hours  
**Summary:** Extract logic from subgen.py into clean modules with type hints

### [STORY_03: Model Lifecycle Manager](./stories/STORY_03_model_lifecycle.md)
**Status:** Not Started  
**Effort:** 8-10 hours  
**Summary:** Context manager for Whisper model, delayed cleanup, thread-safe

### [STORY_04: Memory Leak Fixes](./stories/STORY_04_memory_leaks.md)
**Status:** Not Started  
**Effort:** 6-8 hours  
**Summary:** Fix all 3 memory leaks, memory monitoring, health reporting

### [STORY_05: Configuration Management](./stories/STORY_05_configuration.md)
**Status:** Not Started  
**Effort:** 4-6 hours  
**Summary:** pydantic-settings for config, validation, type safety

---

## Acceptance Criteria

- [ ] All 5 stories completed
- [ ] All tests passing (unit + integration)
- [ ] Type hints throughout (mypy --strict passes)
- [ ] No memory leaks (1000 transcription test)
- [ ] gRPC server responds to all 3 RPCs (Transcribe, DetectLanguage, HealthCheck)
- [ ] Model cleanup works correctly (delayed + hard limit)
- [ ] Memory monitoring reports accurate usage
- [ ] All resources cleaned up (files, buffers, connections)
- [ ] Code coverage > 70%
- [ ] Work logs created for all stories

---

## Dependencies

**Requires:**
- None (can run in parallel with EPIC_01)

**Blocks:**
- EPIC_03 (Integration & Testing) - requires worker to be functional

**Parallelizable With:**
- EPIC_01 (Go Orchestrator Core) - independent codebases

---

## Technical Stack

| Component | Technology | Rationale |
|-----------|------------|-----------|
| **gRPC Server** | grpcio | Official Python gRPC library |
| **Config** | pydantic-settings | Type-safe environment variables |
| **Testing** | pytest | Standard Python testing framework |
| **Type Checking** | mypy | Static type checking |
| **Transcription** | faster-whisper + stable-ts | Keep existing (proven quality) |
| **Memory Monitoring** | psutil | Process memory tracking |
| **Logging** | logging (stdlib) | Structured JSON logging |

---

## Key Design Decisions

### 1. No Global State

**Decision:** Remove all global variables (model, task_results, queue)

**Current subgen.py:**
```python
# ❌ PROBLEMATIC
model = None
task_results = {}
task_queue = DeduplicatedQueue()
```

**New worker:**
```python
# ✅ ENCAPSULATED
class TranscriptionServicer:
    def __init__(self, config: Config):
        self.model_manager = ModelManager(config)  # Not global!
        self.stats = WorkerStats()
```

**Why:** Thread-safe, testable, no hidden dependencies

---

### 2. Model Lifecycle with Hard Limits

**Decision:** Keep delayed cleanup (30s) BUT add hard memory limit

**Implementation:**
```python
class ModelManager:
    def schedule_cleanup(self):
        # Check memory threshold
        if memory_mb > threshold:
            self._unload_model()  # Immediate unload
            return
        
        # Otherwise, delayed cleanup
        timer = threading.Timer(30, self._cleanup_if_idle)
        timer.start()
```

**Why:**
- Performance: Avoid constant reloading (keep delayed cleanup)
- Safety: Prevent memory exhaustion (add hard limit)

---

### 3. Context Managers Everywhere

**Decision:** Use `with` statements for all resources

**Example:**
```python
# ✅ CORRECT
def extract_audio(video_path: str):
    with av.open(video_path) as container:
        with io.BytesIO() as audio_buffer:
            # Process audio
            pass
        # ← All resources automatically closed
```

**Why:** No leaked file handles, buffers, or connections

---

### 4. No `task_results` Dict

**Decision:** Remove `task_results` entirely (orchestrator manages state)

**Why:** Worker is stateless, orchestrator tracks tasks. No unbounded dict.

---

### 5. Type Hints Throughout

**Decision:** All functions have type hints, mypy --strict passes

**Example:**
```python
def transcribe_audio(
    file_path: str,
    options: TranscribeOptions,
    model_manager: ModelManager
) -> TranscriptionResult:
    ...
```

**Why:** Catch bugs early, IDE autocomplete, self-documenting

---

## Module Structure

```
worker/
├── main.py                    # gRPC server entry point
├── server/
│   ├── __init__.py
│   └── grpc_server.py         # gRPC service implementation
├── transcription/
│   ├── __init__.py
│   ├── engine.py              # Core transcription (from subgen.py)
│   ├── audio.py               # Audio extraction (ffmpeg)
│   ├── subtitles.py           # SRT/LRC generation
│   ├── language.py            # Language detection
│   └── model.py               # Model lifecycle with context managers
├── utils/
│   ├── __init__.py
│   ├── language_code.py       # Existing, moved here
│   ├── memory.py              # Memory monitoring
│   └── files.py               # File utilities
├── tests/
│   ├── conftest.py
│   ├── unit/
│   └── integration/
├── requirements.txt
├── requirements-dev.txt
├── Dockerfile
└── pytest.ini
```

---

## Memory Leak Fixes

### Leak #1: Unbounded `task_results` Dict

**Current:** `task_results = {}` (global, never cleaned)

**Fix:** **Remove entirely** (orchestrator manages state)

**Test:**
```python
def test_no_task_results_dict():
    # Verify task_results does not exist in worker code
    assert "task_results" not in worker.__dict__
```

---

### Leak #2: Model Cleanup Race Condition

**Current:** Timer resets indefinitely, model never unloads

**Fix:** Hard memory limit + delayed cleanup

**Test:**
```python
def test_model_unloads_on_memory_threshold():
    model_manager = ModelManager(config)
    model_manager.get_model()  # Load
    
    # Simulate high memory
    with patch.object(psutil.Process, 'memory_info') as mock:
        mock.return_value.rss = 4_000_000_000  # 4GB
        model_manager.schedule_cleanup()
    
    # Model should be unloaded immediately
    assert not model_manager.is_loaded()
```

---

### Leak #3: BytesIO Not Closed

**Current:** BytesIO created but not explicitly closed

**Fix:** Context managers (`with` statements)

**Test:**
```python
def test_bytesio_closed():
    with patch('io.BytesIO') as mock_bytesio:
        extract_audio("/path/to/video.mkv")
    
    # Verify __exit__ called (context manager used)
    mock_bytesio.return_value.__exit__.assert_called_once()
```

---

## Timeline

**Day 1:** STORY_01 (gRPC Server) - start  
**Day 2:** STORY_01 (gRPC Server) - complete  
**Day 3-4:** STORY_02 (Modular Refactor)  
**Day 5:** STORY_03 (Model Lifecycle)  
**Day 6:** STORY_04 (Memory Leak Fixes)  
**Day 7:** STORY_05 (Configuration) + buffer

---

## Risks & Mitigation

| Risk | Impact | Mitigation |
|------|--------|------------|
| Breaking transcription quality | High | Keep faster-whisper + stable-ts unchanged |
| Model lifecycle bugs | High | Comprehensive tests (unit + memory leak) |
| gRPC server complexity | Medium | Use examples from grpcio docs |
| Migration from 2144-line file | High | Incremental refactor, test each module |
| Type hint errors | Medium | Use mypy incrementally, fix module by module |

---

## Testing Strategy

### Unit Tests (70%+ coverage)
- gRPC method handlers (mock model)
- Model lifecycle (load, unload, cleanup)
- Audio extraction (mock ffmpeg)
- Subtitle generation (SRT/LRC)
- Language detection
- Memory monitoring

### Integration Tests
- gRPC server end-to-end
- Full transcription pipeline
- Model cleanup under load
- Memory leak validation

### Memory Leak Tests
- 1000 transcriptions, measure growth
- 24-hour soak test
- Stress test (concurrent requests)

---

## Definition of Done

- [ ] All 5 stories completed with ✅ status
- [ ] All tests passing (unit + integration)
- [ ] Type checking passes (mypy --strict)
- [ ] Code coverage > 70%
- [ ] No memory leaks (1000 transcription test passes)
- [ ] gRPC server responds correctly to all RPCs
- [ ] Model cleanup works (delayed + hard limit)
- [ ] All resources cleaned up (files, buffers)
- [ ] Documentation complete (docstrings, README)
- [ ] Work logs created for each story
- [ ] Docker image builds successfully
- [ ] pytest -v passes with zero failures

---

## Next Epic

**EPIC_03: Integration & Testing** (requires both EPIC_01 and EPIC_02 complete)

**Integration Point:** After both orchestrator and worker are complete, validate Go ↔ Python communication via gRPC.

---

## References

- README-LLM.md - Development workflow, critical rules
- [00_HYBRID_ARCHITECTURE.md](../../DESIGN/00_HYBRID_ARCHITECTURE.md)
- [01_GRPC_PROTOCOL.md](../../DESIGN/01_GRPC_PROTOCOL.md)
- [02_MEMORY_MANAGEMENT.md](../../DESIGN/02_MEMORY_MANAGEMENT.md) - **CRITICAL**
- Legacy code: `subgen.py` (transcription logic to refactor)

---

**Epic Owner:** TBD  
**Created:** 2026-02-15  
**Last Updated:** 2026-02-15
