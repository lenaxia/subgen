# Story 03: Model Lifecycle Management

**Epic**: EPIC_02 - Python Worker Refactor  
**Status**: Not Started  
**Priority**: Critical  
**Estimated Effort**: 8-10 hours  
**Assignee**: TBD

---

## User Story

As a **Python developer**,  
I want **a ModelManager class that handles Whisper model loading/unloading with proper lifecycle management**,  
So that **models are loaded on-demand, cleaned up properly, and VRAM is managed efficiently**.

---

## Background

The legacy `subgen.py` has model lifecycle logic scattered across multiple functions with potential race conditions and memory leaks. This story creates a centralized `ModelManager` class that:

- Loads models lazily (on first use)
- Schedules cleanup with configurable delays
- Cancels cleanup timers properly (prevents leak)
- Handles CUDA cache clearing
- Prevents race conditions with thread-safe locking
- Integrates with the queue to check if system is idle

**Critical Issues in Legacy Code**:
1. **Timer cancellation leak** (lines 1149-1163): Timers can be cancelled indefinitely without cleanup
2. **Global state management** (lines 204-206, 1143-1147): Model and timers are globals
3. **Race conditions** (lines 1165-1197): Cleanup can happen while model is being loaded

---

## Acceptance Criteria

- [ ] `worker/transcription/model.py` created
- [ ] `ModelManager` class with lazy loading
- [ ] `load()` method - loads model on first use
- [ ] `unload()` method - unloads model and clears VRAM
- [ ] `schedule_cleanup()` method - schedules cleanup with configurable delay
- [ ] `cancel_cleanup()` method - cancels pending cleanup timer
- [ ] `is_loaded()` method - checks if model is loaded
- [ ] Thread-safe with `Lock()` for all operations
- [ ] Integration with DeduplicatedQueue for idle detection
- [ ] Configuration from environment variables
- [ ] Unit tests for all methods (12+ tests)
- [ ] Integration test with actual model loading (marked `@pytest.mark.slow`)
- [ ] Memory leak test (verifies timer cleanup)
- [ ] Work log created

---

## Legacy Code Analysis

### 1. Global State (Lines 204-206)

**Location**: `subgen.py:204-206`

**Current Implementation**:
```python
model = None
model_cleanup_timer = None
model_cleanup_lock = Lock()
```

**Analysis**:
- Global variables make testing difficult
- State shared across all threads
- Cleanup timer is global (prone to leaks)

**New Design**:
- Encapsulate all state in `ModelManager` class
- Pass instance to components that need it
- No global state

---

### 2. start_model Function (Lines 1143-1147)

**Location**: `subgen.py:1143-1147`

**Current Implementation**:
```python
def start_model():
    global model
    if model is None:
        logging.debug("Model was purged, need to re-create")
        model = stable_whisper.load_faster_whisper(
            whisper_model, 
            download_root=model_location, 
            device=transcribe_device, 
            cpu_threads=whisper_threads, 
            num_workers=concurrent_transcriptions, 
            compute_type=compute_type
        )
```

**Analysis**:
- Loads model synchronously (blocks worker thread)
- Uses global `model` variable
- No error handling
- No timeout for loading
- Called every time before transcription (lines 237, 826, 917, 1063)

**Integration Points**:
- Called from `gen_subtitles()` (line 1237)
- Called from `asr_task_worker()` (line 825)
- Called from `detect_language_task()` (line 1063)
- Called from `/detect-language` endpoint (line 916)

**New Design**:
- `ModelManager.load()` method
- Returns existing model if already loaded (idempotent)
- Proper exception handling
- Logging with timing information
- Thread-safe with lock

---

### 3. schedule_model_cleanup Function (Lines 1149-1163)

**Location**: `subgen.py:1149-1163`

**Current Implementation**:
```python
def schedule_model_cleanup():
    """Schedule model cleanup with a delay to allow concurrent requests."""
    global model_cleanup_timer, model_cleanup_lock
    
    with model_cleanup_lock:
        # Cancel any existing timer
        if model_cleanup_timer is not None:
            model_cleanup_timer.cancel()
            logging.debug("Cancelled previous model cleanup timer")
        
        # Schedule a new cleanup timer
        model_cleanup_timer = Timer(model_cleanup_delay, perform_model_cleanup)
        model_cleanup_timer.daemon = True
        model_cleanup_timer.start()
        logging.debug(f"Model cleanup scheduled in {model_cleanup_delay} seconds")
```

**Analysis**:
- **MEMORY LEAK**: When timer is cancelled, it's never fully cleaned up
- Timer is replaced but old timer thread may not terminate
- Called repeatedly during high load (every worker completion)
- Global `model_cleanup_timer` is reassigned without cleanup

**Problem Scenario**:
```
Time 0: Worker 1 finishes → schedules cleanup timer (30s)
Time 5: Worker 2 finishes → cancels timer, schedules new (30s)
Time 10: Worker 3 finishes → cancels timer, schedules new (30s)
Time 15: Worker 4 finishes → cancels timer, schedules new (30s)
...
Result: Old cancelled timers accumulate in memory
```

**New Design**:
- Proper timer cleanup in `ModelManager.cancel_cleanup()`
- Join cancelled timer threads
- Track timer state explicitly
- Prevent timer accumulation

---

### 4. perform_model_cleanup Function (Lines 1165-1197)

**Location**: `subgen.py:1165-1197`

**Current Implementation**:
```python
def perform_model_cleanup():
    """Actually perform the model cleanup."""
    global model, model_cleanup_timer, model_cleanup_lock
    
    with model_cleanup_lock: 
        logging.debug("Executing scheduled model cleanup")
        
        if clear_vram_on_complete and task_queue.is_idle():
            logging.debug("Queue idle; clearing model from memory.")
            if model: 
                try:
                    model.model.unload_model()
                    del model
                    model = None
                    logging.info("Model unloaded from memory")
                except Exception as e:
                    logging.error(f"Error unloading model: {e}")
            
            if transcribe_device.lower() == 'cuda' and torch.cuda.is_available():
                try:
                    torch.cuda.empty_cache()
                    logging.debug("CUDA cache cleared.")
                except Exception as e: 
                    logging.error(f"Error clearing CUDA cache: {e}")
        else:
            logging.debug("Queue not idle or clear_vram disabled; skipping model cleanup")
        
        if os.name != 'nt': # don't garbage collect on Windows
            gc.collect()
            ctypes.CDLL(ctypes.util.find_library('c')).malloc_trim(0)
        
        model_cleanup_timer = None
```

**Analysis**:
- Proper CUDA cache clearing
- Checks if queue is idle before cleanup
- Calls `malloc_trim(0)` on Linux (returns memory to OS)
- Garbage collection after cleanup
- **Issue**: Sets timer to None but doesn't clean up Timer object

**Key Operations**:
1. `model.model.unload_model()` - Unloads Whisper model from VRAM
2. `del model` - Deletes Python reference
3. `torch.cuda.empty_cache()` - Clears CUDA cache
4. `gc.collect()` - Runs garbage collection
5. `malloc_trim(0)` - Returns memory to OS (Linux only)

**New Design**:
- Same cleanup logic
- Better error handling
- Metrics tracking (cleanup count, memory freed)
- Proper timer cleanup

---

### 5. delete_model Function (Lines 1198-1213)

**Location**: `subgen.py:1198-1213`

**Current Implementation**:
```python
def delete_model():
    """
    Only schedules a cleanup timer if the system is actually idle.
    This prevents unnecessary timer resets when a large batch is being processed.
    """
    # 1. If we aren't supposed to clear VRAM, don't bother with timers at all.
    if not clear_vram_on_complete:
        return

    # 2. Only schedule cleanup if the queue is empty AND no other workers are processing.
    if task_queue.is_idle():
        schedule_model_cleanup()
    else:
        # If there are 10 items left in the queue, we simply do nothing. 
        # The very last worker to finish the last item will trigger the timer.
        logging.debug("Tasks still in queue or processing; skipping model cleanup scheduling.")
```

**Analysis**:
- Smart: Only schedules cleanup when idle
- Prevents timer churn during batch processing
- Called from worker finally block (line 390)
- Called from asr_task_worker finally (line 857)
- Called from detect_language_task finally (line 1079)
- Called from /detect-language endpoint finally (line 938)

**Integration Points**:
- Requires `task_queue.is_idle()` method
- Checks `clear_vram_on_complete` config

**New Design**:
- Same logic in `ModelManager.schedule_cleanup_if_idle(queue)`
- Takes queue as parameter (dependency injection)
- Returns boolean (cleanup scheduled yes/no)

---

## Technical Design

### Model Manager Class

**File: `worker/transcription/model.py`**

```python
"""
Model lifecycle management for Whisper models.

Handles loading, unloading, and cleanup scheduling with proper
resource management and thread safety.
"""

import logging
import time
import os
import gc
import ctypes
import ctypes.util
from threading import Lock, Timer, Event
from typing import Optional
from dataclasses import dataclass

import stable_whisper
import torch

logger = logging.getLogger(__name__)


@dataclass
class ModelConfig:
    """Configuration for Whisper model."""
    model_name: str = "medium"
    model_path: str = "./models"
    device: str = "cpu"
    cpu_threads: int = 4
    num_workers: int = 2
    compute_type: str = "auto"
    cleanup_delay: int = 30
    clear_vram: bool = True


class ModelManager:
    """
    Manages Whisper model lifecycle with lazy loading and cleanup.
    
    Extracted from: subgen.py:204-206, 1143-1213
    
    Features:
    - Lazy loading (loads on first use)
    - Scheduled cleanup with configurable delay
    - Thread-safe operations
    - CUDA cache management
    - Integration with task queue for idle detection
    
    Usage:
        config = ModelConfig(model_name="medium", device="cuda")
        manager = ModelManager(config)
        
        # Load model (idempotent)
        model = manager.load()
        
        # Use model
        result = model.transcribe(audio)
        
        # Schedule cleanup when done
        manager.schedule_cleanup_if_idle(task_queue)
    """
    
    def __init__(self, config: ModelConfig):
        self.config = config
        self._model = None
        self._lock = Lock()
        self._cleanup_timer: Optional[Timer] = None
        self._cleanup_event = Event()
        self._load_count = 0
        self._cleanup_count = 0
        self._total_cleanup_time = 0.0
        
    def load(self):
        """
        Load Whisper model if not already loaded.
        
        Extracted from: subgen.py:1143-1147 (start_model)
        
        Returns:
            Loaded stable_whisper model
            
        Raises:
            RuntimeError: If model fails to load
        """
        with self._lock:
            if self._model is not None:
                logger.debug("Model already loaded, reusing existing instance")
                return self._model
            
            logger.info(f"Loading Whisper model: {self.config.model_name} on {self.config.device}")
            start_time = time.time()
            
            try:
                self._model = stable_whisper.load_faster_whisper(
                    self.config.model_name,
                    download_root=self.config.model_path,
                    device=self.config.device,
                    cpu_threads=self.config.cpu_threads,
                    num_workers=self.config.num_workers,
                    compute_type=self.config.compute_type
                )
                
                load_time = time.time() - start_time
                self._load_count += 1
                
                logger.info(
                    f"Model loaded successfully in {load_time:.2f}s "
                    f"(total loads: {self._load_count})"
                )
                
                return self._model
                
            except Exception as e:
                logger.error(f"Failed to load model: {e}", exc_info=True)
                raise RuntimeError(f"Model loading failed: {e}")
    
    def unload(self) -> None:
        """
        Unload model and clear VRAM.
        
        Extracted from: subgen.py:1165-1197 (perform_model_cleanup)
        
        Performs:
        1. Unload Whisper model
        2. Clear CUDA cache (if GPU)
        3. Run garbage collection
        4. Return memory to OS (Linux only)
        """
        with self._lock:
            if self._model is None:
                logger.debug("Model already unloaded, nothing to do")
                return
            
            logger.info("Unloading Whisper model from memory")
            start_time = time.time()
            
            try:
                # Step 1: Unload Whisper model
                self._model.model.unload_model()
                del self._model
                self._model = None
                logger.debug("Model unloaded from memory")
                
                # Step 2: Clear CUDA cache if using GPU
                if self.config.device.lower() == 'cuda' and torch.cuda.is_available():
                    try:
                        torch.cuda.empty_cache()
                        logger.debug("CUDA cache cleared")
                    except Exception as e:
                        logger.error(f"Failed to clear CUDA cache: {e}")
                
                # Step 3: Garbage collection (not on Windows)
                if os.name != 'nt':
                    gc.collect()
                    
                    # Step 4: Return memory to OS (Linux only)
                    try:
                        libc = ctypes.CDLL(ctypes.util.find_library('c'))
                        libc.malloc_trim(0)
                        logger.debug("Memory returned to OS")
                    except Exception as e:
                        logger.debug(f"malloc_trim not available: {e}")
                
                cleanup_time = time.time() - start_time
                self._cleanup_count += 1
                self._total_cleanup_time += cleanup_time
                
                logger.info(
                    f"Model cleanup completed in {cleanup_time:.2f}s "
                    f"(total cleanups: {self._cleanup_count}, "
                    f"avg time: {self._total_cleanup_time / self._cleanup_count:.2f}s)"
                )
                
            except Exception as e:
                logger.error(f"Error during model cleanup: {e}", exc_info=True)
    
    def schedule_cleanup(self, delay: Optional[int] = None) -> None:
        """
        Schedule model cleanup after delay.
        
        Extracted from: subgen.py:1149-1163 (schedule_model_cleanup)
        
        Properly cancels previous timer to prevent memory leaks.
        
        Args:
            delay: Cleanup delay in seconds (uses config default if None)
        """
        if delay is None:
            delay = self.config.cleanup_delay
        
        with self._lock:
            # Cancel existing timer
            if self._cleanup_timer is not None:
                self._cleanup_timer.cancel()
                self._cleanup_event.set()  # Signal timer to exit
                logger.debug("Cancelled previous cleanup timer")
                self._cleanup_timer = None
            
            # Reset event for new timer
            self._cleanup_event.clear()
            
            # Schedule new cleanup
            self._cleanup_timer = Timer(delay, self._perform_cleanup)
            self._cleanup_timer.daemon = True
            self._cleanup_timer.start()
            
            logger.debug(f"Cleanup scheduled in {delay}s")
    
    def cancel_cleanup(self) -> None:
        """
        Cancel pending cleanup timer.
        
        Properly cleans up timer resources to prevent memory leak.
        """
        with self._lock:
            if self._cleanup_timer is not None:
                self._cleanup_timer.cancel()
                self._cleanup_event.set()
                logger.debug("Cleanup timer cancelled")
                self._cleanup_timer = None
    
    def schedule_cleanup_if_idle(self, task_queue) -> bool:
        """
        Schedule cleanup only if queue is idle.
        
        Extracted from: subgen.py:1198-1213 (delete_model)
        
        Args:
            task_queue: DeduplicatedQueue instance to check idle state
            
        Returns:
            True if cleanup was scheduled, False otherwise
        """
        if not self.config.clear_vram:
            logger.debug("VRAM clearing disabled, skipping cleanup")
            return False
        
        if task_queue.is_idle():
            self.schedule_cleanup()
            return True
        else:
            logger.debug("Queue not idle, skipping cleanup scheduling")
            return False
    
    def _perform_cleanup(self) -> None:
        """
        Internal method called by Timer.
        
        Checks if cleanup event was set (timer cancelled) before proceeding.
        """
        # Check if we were cancelled
        if self._cleanup_event.is_set():
            logger.debug("Cleanup cancelled, timer exiting")
            return
        
        logger.debug("Executing scheduled cleanup")
        self.unload()
    
    def is_loaded(self) -> bool:
        """Check if model is currently loaded."""
        with self._lock:
            return self._model is not None
    
    def get_model(self):
        """
        Get currently loaded model.
        
        Returns:
            Loaded model or None
        """
        with self._lock:
            return self._model
    
    def get_stats(self) -> dict:
        """
        Get model manager statistics.
        
        Returns:
            Dictionary with load/cleanup counts and timing
        """
        with self._lock:
            return {
                'model_loaded': self._model is not None,
                'load_count': self._load_count,
                'cleanup_count': self._cleanup_count,
                'avg_cleanup_time': (
                    self._total_cleanup_time / self._cleanup_count
                    if self._cleanup_count > 0
                    else 0.0
                ),
                'cleanup_scheduled': self._cleanup_timer is not None,
            }
```

---

## Integration with Transcription Engine

**Update `worker/transcription/engine.py`**:

```python
class TranscriptionEngine:
    """Core transcription engine with ModelManager integration."""
    
    def __init__(self, config, model_manager: ModelManager):
        self.config = config
        self.model_manager = model_manager
    
    def transcribe(self, file_path: str, task_type: str, 
                   force_language: Optional[str], 
                   options: TranscribeOptions) -> TranscriptionResult:
        """Transcribe with model lifecycle management."""
        
        try:
            # Load model (lazy)
            model = self.model_manager.load()
            
            # ... transcription logic ...
            result = model.transcribe(data, **args)
            
            return TranscriptionResult(success=True, ...)
            
        finally:
            # Schedule cleanup if idle
            # (queue passed from higher level)
            pass  # Will be called by worker
```

---

## Testing Strategy

### Unit Tests (12+ tests)

**File: `worker/tests/unit/test_model.py`**

```python
"""Unit tests for ModelManager."""

import pytest
import time
from unittest.mock import Mock, patch, MagicMock
from transcription.model import ModelManager, ModelConfig


@pytest.fixture
def config():
    """Model configuration for testing."""
    return ModelConfig(
        model_name="tiny",  # Use tiny for tests
        model_path="./test_models",
        device="cpu",
        cleanup_delay=1,  # Short delay for tests
        clear_vram=True
    )


@pytest.fixture
def mock_stable_whisper():
    """Mock stable_whisper.load_faster_whisper."""
    with patch('transcription.model.stable_whisper') as mock:
        mock_model = Mock()
        mock_model.model.unload_model = Mock()
        mock.load_faster_whisper = Mock(return_value=mock_model)
        yield mock


def test_manager_initialization(config):
    """Test ModelManager initializes with correct state."""
    manager = ModelManager(config)
    
    assert not manager.is_loaded()
    assert manager.get_model() is None
    assert manager._cleanup_timer is None


def test_load_model(config, mock_stable_whisper):
    """Test model loading."""
    manager = ModelManager(config)
    
    model = manager.load()
    
    assert model is not None
    assert manager.is_loaded()
    mock_stable_whisper.load_faster_whisper.assert_called_once()


def test_load_model_idempotent(config, mock_stable_whisper):
    """Test loading same model twice reuses instance."""
    manager = ModelManager(config)
    
    model1 = manager.load()
    model2 = manager.load()
    
    assert model1 is model2
    # Should only call load once
    assert mock_stable_whisper.load_faster_whisper.call_count == 1


def test_unload_model(config, mock_stable_whisper):
    """Test model unloading."""
    manager = ModelManager(config)
    
    # Load then unload
    model = manager.load()
    manager.unload()
    
    assert not manager.is_loaded()
    model.model.unload_model.assert_called_once()


def test_unload_when_not_loaded(config):
    """Test unloading when no model is loaded."""
    manager = ModelManager(config)
    
    # Should not raise
    manager.unload()
    
    assert not manager.is_loaded()


def test_schedule_cleanup(config, mock_stable_whisper):
    """Test cleanup scheduling."""
    manager = ModelManager(config)
    manager.load()
    
    manager.schedule_cleanup(delay=1)
    
    assert manager._cleanup_timer is not None
    
    # Wait for cleanup
    time.sleep(1.5)
    
    assert not manager.is_loaded()


def test_cancel_cleanup(config, mock_stable_whisper):
    """Test cleanup cancellation."""
    manager = ModelManager(config)
    manager.load()
    
    manager.schedule_cleanup(delay=2)
    assert manager._cleanup_timer is not None
    
    manager.cancel_cleanup()
    assert manager._cleanup_timer is None
    
    # Wait past cleanup time
    time.sleep(2.5)
    
    # Model should still be loaded
    assert manager.is_loaded()


def test_cleanup_timer_no_leak(config, mock_stable_whisper):
    """Test that cancelled timers don't leak."""
    manager = ModelManager(config)
    manager.load()
    
    # Schedule and cancel 10 times
    for _ in range(10):
        manager.schedule_cleanup(delay=5)
        manager.cancel_cleanup()
    
    # Should only have cleanup cancelled, no accumulation
    assert manager._cleanup_timer is None
    assert manager.is_loaded()


def test_schedule_cleanup_if_idle_when_idle(config, mock_stable_whisper):
    """Test scheduling cleanup when queue is idle."""
    manager = ModelManager(config)
    manager.load()
    
    mock_queue = Mock()
    mock_queue.is_idle = Mock(return_value=True)
    
    result = manager.schedule_cleanup_if_idle(mock_queue)
    
    assert result is True
    assert manager._cleanup_timer is not None


def test_schedule_cleanup_if_idle_when_busy(config, mock_stable_whisper):
    """Test cleanup NOT scheduled when queue is busy."""
    manager = ModelManager(config)
    manager.load()
    
    mock_queue = Mock()
    mock_queue.is_idle = Mock(return_value=False)
    
    result = manager.schedule_cleanup_if_idle(mock_queue)
    
    assert result is False
    assert manager._cleanup_timer is None


def test_schedule_cleanup_if_idle_vram_disabled(config, mock_stable_whisper):
    """Test cleanup skipped when clear_vram is False."""
    config.clear_vram = False
    manager = ModelManager(config)
    manager.load()
    
    mock_queue = Mock()
    mock_queue.is_idle = Mock(return_value=True)
    
    result = manager.schedule_cleanup_if_idle(mock_queue)
    
    assert result is False


def test_get_stats(config, mock_stable_whisper):
    """Test statistics collection."""
    manager = ModelManager(config)
    
    stats = manager.get_stats()
    assert stats['model_loaded'] is False
    assert stats['load_count'] == 0
    
    manager.load()
    stats = manager.get_stats()
    assert stats['model_loaded'] is True
    assert stats['load_count'] == 1
    
    manager.unload()
    stats = manager.get_stats()
    assert stats['cleanup_count'] == 1
```

### Integration Test (Slow)

```python
@pytest.mark.slow
@pytest.mark.requires_model
def test_real_model_loading():
    """Integration test with real model (tiny)."""
    config = ModelConfig(
        model_name="tiny",
        device="cpu",
        cleanup_delay=1
    )
    
    manager = ModelManager(config)
    
    # Load model
    model = manager.load()
    assert model is not None
    
    # Test transcription
    import numpy as np
    audio = np.zeros(16000, dtype=np.float32)  # 1 second silence
    result = model.transcribe(audio, input_sr=16000)
    
    assert result is not None
    
    # Cleanup
    manager.unload()
    assert not manager.is_loaded()
```

---

## Definition of Done

- [ ] All acceptance criteria met
- [ ] `worker/transcription/model.py` created with ModelManager class
- [ ] All methods implemented with docstrings
- [ ] Thread-safe with Lock for all operations
- [ ] Timer cleanup prevents memory leaks
- [ ] Unit tests passing (12+ tests)
- [ ] Integration test with real model passing (marked slow)
- [ ] Type hints throughout (mypy --strict passes)
- [ ] Code coverage > 90% for model.py
- [ ] Work log created: `docs/WORKLOGS/NNNN_YYYY-MM-DD_EPIC_02_story_03_model_lifecycle.md`
- [ ] Code committed and pushed

---

## Validation Commands

```bash
# Run unit tests
cd worker
pytest tests/unit/test_model.py -v

# Run integration tests (slow)
pytest tests/unit/test_model.py -v -m "slow or requires_model"

# Check type hints
mypy transcription/model.py --strict

# Test model loading manually
python -c "
from transcription.model import ModelManager, ModelConfig
config = ModelConfig(model_name='tiny', device='cpu')
manager = ModelManager(config)
model = manager.load()
print('Model loaded:', manager.is_loaded())
manager.unload()
print('Model unloaded:', not manager.is_loaded())
"
```

---

## Dependencies

**Requires:**
- STORY_01 (gRPC Server Setup) - needs server structure
- STORY_02 (Modular Refactor) - needs transcription engine

**Blocks:**
- STORY_04 (Memory Leaks) - fixes timer leak in this module
- STORY_05 (Configuration) - uses config for model settings

---

## References

- Legacy code: `subgen.py:204-206` (global state)
- Legacy code: `subgen.py:1143-1147` (start_model)
- Legacy code: `subgen.py:1149-1163` (schedule_model_cleanup)
- Legacy code: `subgen.py:1165-1197` (perform_model_cleanup)
- Legacy code: `subgen.py:1198-1213` (delete_model)
- stable-whisper docs: https://github.com/jianfch/stable-ts
- PyTorch CUDA cache: https://pytorch.org/docs/stable/notes/cuda.html

---

**Created**: 2026-02-15  
**Last Updated**: 2026-02-15
