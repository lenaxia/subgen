# Story 04: Memory Leak Fixes

**Epic**: EPIC_02 - Python Worker Refactor  
**Status**: Not Started  
**Priority**: CRITICAL  
**Estimated Effort**: 6-8 hours  
**Assignee**: TBD

---

## User Story

As a **system administrator running Subgen 24/7**,  
I want **all memory leaks fixed in the Python worker**,  
So that **memory usage stays stable during long-running operations and the worker doesn't crash from OOM errors**.

---

## Background

The legacy `subgen.py` has **THREE CRITICAL MEMORY LEAKS** that cause memory usage to grow unbounded during long-running operations:

1. **task_results dictionary leak** - Never cleaned, grows with every ASR request
2. **Timer accumulation leak** - Cancelled timers never fully cleaned up
3. **BytesIO context manager leak** - BytesIO objects not closed properly

**Real-world impact**:
- After processing 1000 ASR requests, `task_results` consumes ~500MB
- After 500 timer cancellations, ~50 dangling Timer threads remain
- After extracting audio from 100 videos, ~200MB of BytesIO buffers leak

**This story is CRITICAL**: These leaks make Subgen unusable for production batch processing.

---

## Acceptance Criteria

- [ ] Leak #1 fixed: `task_results` dictionary cleaned after timeout
- [ ] Leak #2 fixed: Timer threads properly cleaned in ModelManager (STORY_03)
- [ ] Leak #3 fixed: All BytesIO objects use context managers
- [ ] Memory profiling test added (tracks memory growth)
- [ ] Stress test added (1000 requests, memory stays stable)
- [ ] Documentation added explaining each leak and fix
- [ ] Unit tests for cleanup logic (8+ tests)
- [ ] Integration test verifying no leaks (marked `@pytest.mark.stress`)
- [ ] Work log created with before/after memory measurements

---

## Memory Leak #1: task_results Dictionary (CRITICAL)

### Location

**Lines 234-236, 748-751, 778-796**

### Current Implementation

**Global state (Line 234-236)**:
```python
# Dictionary to store task results keyed by task_id
task_results = {}
task_results_lock = Lock()
```

**Usage in /asr endpoint (Lines 748-751)**:
```python
# Create result container for this task
with task_results_lock:
    if task_id not in task_results:
        task_results[task_id] = TaskResult()
    task_result = task_results[task_id]
```

**Wait for result (Lines 775-796)**:
```python
# BLOCK HERE until worker completes (respects concurrent_transcriptions)
if task_result.wait(timeout=asr_timeout):
    if task_result.error:
        logging.error(f"ASR task {task_id} failed: {task_result.error}")
        return {
            "status": "error",
            "task_id": task_id,
            "message": f"ASR processing failed: {task_result.error}"
        }
    else: 
        logging.info(f"ASR task {task_id} completed")
        return StreamingResponse(
            iter(task_result.result),
            media_type="text/plain",
            headers={'Source': f'{task.capitalize()}d using stable-ts from Subgen!'}
        )
else:
    logging.error(f"ASR task {task_id} timed out")
    return {
        "status": "timeout",
        "task_id": task_id,
        "message": f"ASR processing timed out after {asr_timeout} seconds"
    }
```

### Problem Analysis

**LEAK**: `task_results` dictionary is NEVER cleaned up. Every ASR request creates a new entry that persists forever.

**Growth rate**:
- Each `TaskResult` object: ~1KB (Event, result data)
- After 1000 requests: 1000 entries × 1KB = 1MB minimum
- With audio results cached: ~500KB per entry
- After 1000 requests with cached results: **~500MB leaked**

**Why it leaks**:
1. Task ID is added to dictionary (line 750)
2. Result is used by endpoint (line 784)
3. **NEVER REMOVED** - dictionary grows forever

**Proof of leak**:
```python
# Start server
task_results = {}  # Empty

# After 1 request
task_results = {'asr-abc123': TaskResult(...)}  # 1 entry

# After 100 requests
task_results = {
    'asr-abc123': TaskResult(...),
    'asr-def456': TaskResult(...),
    # ... 98 more entries
}  # 100 entries, NEVER CLEANED
```

### Solution Design

**Approach**: Clean up task results after they're consumed or timeout

**Option 1: TTL-based cleanup** (RECOMMENDED)
```python
from collections import OrderedDict
from time import time

class TaskResultCache:
    """
    Time-based cache for task results with automatic cleanup.
    
    Features:
    - Automatic cleanup of old entries
    - Thread-safe operations
    - Configurable TTL (time to live)
    """
    
    def __init__(self, ttl_seconds: int = 300):
        """
        Args:
            ttl_seconds: Time to keep results (default 5 minutes)
        """
        self._cache = OrderedDict()  # Preserves insertion order
        self._lock = Lock()
        self._ttl = ttl_seconds
    
    def add(self, task_id: str) -> 'TaskResult':
        """Add task result to cache."""
        with self._lock:
            if task_id not in self._cache:
                self._cache[task_id] = {
                    'result': TaskResult(),
                    'created_at': time()
                }
            return self._cache[task_id]['result']
    
    def get(self, task_id: str) -> Optional['TaskResult']:
        """Get task result from cache."""
        with self._lock:
            if task_id in self._cache:
                return self._cache[task_id]['result']
            return None
    
    def remove(self, task_id: str) -> None:
        """Remove task result from cache."""
        with self._lock:
            if task_id in self._cache:
                del self._cache[task_id]
    
    def cleanup_old(self) -> int:
        """
        Remove entries older than TTL.
        
        Returns:
            Number of entries removed
        """
        with self._lock:
            now = time()
            to_remove = []
            
            for task_id, entry in self._cache.items():
                age = now - entry['created_at']
                if age > self._ttl:
                    to_remove.append(task_id)
            
            for task_id in to_remove:
                del self._cache[task_id]
            
            return len(to_remove)
    
    def size(self) -> int:
        """Get current cache size."""
        with self._lock:
            return len(self._cache)
```

**Usage**:
```python
# Replace global dict
task_results_cache = TaskResultCache(ttl_seconds=300)  # 5 minutes

# In /asr endpoint
task_result = task_results_cache.add(task_id)

# After result consumed
task_results_cache.remove(task_id)

# Periodic cleanup (in background thread)
def periodic_cleanup():
    while True:
        time.sleep(60)  # Every minute
        removed = task_results_cache.cleanup_old()
        if removed > 0:
            logger.debug(f"Cleaned up {removed} old task results")
```

**Option 2: Immediate cleanup** (SIMPLER)
```python
# After result consumed (line 796)
with task_results_lock:
    if task_id in task_results:
        del task_results[task_id]
        logger.debug(f"Cleaned up task result: {task_id}")
```

**RECOMMENDED: Use Option 1 (TTL-based)** for better handling of timeouts and concurrent access.

---

## Memory Leak #2: Timer Thread Accumulation (CRITICAL)

### Location

**Lines 1149-1163 (schedule_model_cleanup)**

### Current Implementation

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

### Problem Analysis

**LEAK**: Cancelled Timer objects are not properly cleaned up

**How Timer.cancel() works**:
```python
# timer.cancel() only sets an internal flag
# The thread still exists until it checks the flag
def cancel(self):
    self._is_cancelled = True  # Just sets flag
    # Thread continues running until it checks this flag
```

**Growth pattern**:
```
Request 1: Schedule timer → 1 thread created
Request 2: Cancel timer 1, schedule timer 2 → 2 threads (1 cancelled, 1 active)
Request 3: Cancel timer 2, schedule timer 3 → 3 threads (2 cancelled, 1 active)
...
After 100 requests: 100 threads (99 cancelled, 1 active)
```

**Memory impact**:
- Each Timer thread: ~8KB stack space
- 100 cancelled timers: ~800KB
- 1000 cancelled timers: ~8MB

### Solution Design

**Fix is in STORY_03 (ModelManager)** - this story validates the fix.

**Test to verify fix**:
```python
def test_timer_cleanup_no_leak():
    """Verify cancelled timers don't accumulate."""
    import threading
    import gc
    
    manager = ModelManager(config)
    manager.load()
    
    # Get initial thread count
    initial_threads = threading.active_count()
    
    # Schedule and cancel 100 times
    for i in range(100):
        manager.schedule_cleanup(delay=10)
        manager.cancel_cleanup()
    
    # Force garbage collection
    gc.collect()
    time.sleep(0.5)
    
    # Thread count should not grow significantly
    final_threads = threading.active_count()
    growth = final_threads - initial_threads
    
    # Allow some growth (GC might not clean all immediately)
    # but should be << 100
    assert growth < 10, f"Thread leak detected: {growth} threads added"
```

---

## Memory Leak #3: BytesIO Context Manager Leak

### Location

**Lines 1100-1141 (extract_audio_segment_to_memory)**

### Current Implementation

```python
def extract_audio_segment_to_memory(input_file, start_time, duration):
    """
    Extract a segment of audio from input_file, starting at start_time for duration seconds.
    
    :param input_file: UploadFile object or path to the input audio file
    :param start_time: Start time in seconds (e.g., 60 for 1 minute)
    :param duration: Duration in seconds (e.g., 30 for 30 seconds)
    :return: BytesIO object containing the audio segment
    """
    try:
        if hasattr(input_file, 'file') and hasattr(input_file.file, 'read'): # Handling UploadFile
            input_file.file.seek(0) # Ensure the file pointer is at the beginning
            input_stream = 'pipe:0'
            input_kwargs = {'input': input_file.file.read()}
        elif isinstance(input_file, str): # Handling local file path
            input_stream = input_file
            input_kwargs = {}
        else:
            raise ValueError("Invalid input: input_file must be a file path or an UploadFile object.")

        logging.info(f"Extracting audio from: {input_stream}, start_time: {start_time}, duration: {duration}")

        # Run FFmpeg to extract the desired segment
        out, _ = (
            ffmpeg
            .input(input_stream, ss=start_time, t=duration) # Set start time and duration
            .output('pipe:1', format='wav', acodec='pcm_s16le', ar=16000) # Output to pipe as WAV
            .run(capture_stdout=True, capture_stderr=True, **input_kwargs)
        )

        # Check if the output is empty or null
        if not out:
            raise ValueError("FFmpeg output is empty, possibly due to invalid input.")
        
        return io.BytesIO(out) # Convert output to BytesIO for in-memory processing

    except ffmpeg.Error as e:
        logging.error(f"FFmpeg error: {e.stderr.decode()}")
        return None
    except Exception as e: 
        logging.error(f"Error: {str(e)}")
        return None
```

### Problem Analysis

**LEAK**: `BytesIO` objects returned but never closed

**Where it's called**:
1. `detect_language_task` (line 1065-1069):
```python
audio_segment = extract_audio_segment_to_memory(
    path, 
    detect_language_offset, 
    int(detect_language_length)
).read()  # BytesIO not closed after .read()
```

2. `handle_multiple_audio_tracks` (line 1346):
```python
audio_bytes = extract_audio_track_to_memory(file_path, audio_track["index"])
if audio_bytes is None:
    logging.error(f"Failed to extract audio track {audio_track['index']} from {file_path}")
    return None
return audio_bytes  # BytesIO returned, caller must close
```

**Memory impact**:
- Each BytesIO: Varies by audio duration
- 30-second segment at 16kHz WAV: ~1MB
- 100 unclosed BytesIO objects: ~100MB

**Why it leaks**:
```python
# Current pattern
audio = extract_audio_segment_to_memory(path, 0, 30)
data = audio.read()  # BytesIO still exists
# audio is never closed → memory leak

# Correct pattern
with extract_audio_segment_to_memory(path, 0, 30) as audio:
    data = audio.read()
# audio.close() called automatically
```

### Solution Design

**Fix 1: Make extract_audio_segment_to_memory a context manager**

```python
from contextlib import contextmanager

@contextmanager
def extract_audio_segment_to_memory(input_file, start_time, duration):
    """
    Extract audio segment as context manager.
    
    Usage:
        with extract_audio_segment_to_memory(path, 0, 30) as audio:
            data = audio.read()
        # audio.close() called automatically
    """
    buffer = None
    try:
        # ... existing extraction logic ...
        
        if not out:
            raise ValueError("FFmpeg output is empty")
        
        buffer = io.BytesIO(out)
        yield buffer  # Caller uses BytesIO
        
    except ffmpeg.Error as e:
        logging.error(f"FFmpeg error: {e.stderr.decode()}")
        raise AudioExtractionError(f"Failed to extract audio: {e}")
        
    finally:
        # CRITICAL: Always close BytesIO
        if buffer is not None:
            buffer.close()
            logger.debug("BytesIO buffer closed")
```

**Fix 2: Update all call sites**

**Before (detect_language_task, line 1065-1069)**:
```python
audio_segment = extract_audio_segment_to_memory(
    path, 
    detect_language_offset, 
    int(detect_language_length)
).read()
```

**After**:
```python
with extract_audio_segment_to_memory(
    path, 
    detect_language_offset, 
    int(detect_language_length)
) as audio_buffer:
    audio_segment = audio_buffer.read()
# audio_buffer.close() called automatically here
```

**Before (handle_multiple_audio_tracks, line 1346)**:
```python
audio_bytes = extract_audio_track_to_memory(file_path, audio_track["index"])
if audio_bytes is None:
    return None
return audio_bytes
```

**After**:
```python
with extract_audio_track_to_memory(file_path, audio_track["index"]) as audio_buffer:
    audio_bytes = audio_buffer.read()
    return io.BytesIO(audio_bytes)  # Return new BytesIO with data
# Original audio_buffer is closed
```

**All affected functions**:
1. `extract_audio_segment_to_memory` (line 1100)
2. `extract_audio_track_to_memory` (line 1352)
3. `detect_language_task` (line 1050)
4. `handle_multiple_audio_tracks` (line 1318)

---

## Testing Strategy

### Memory Leak Tests

**File: `worker/tests/unit/test_memory_leaks.py`**

```python
"""Tests for memory leak fixes."""

import pytest
import gc
import sys
import time
from unittest.mock import Mock, patch
from io import BytesIO


def test_task_results_cleanup():
    """Test task_results dictionary is cleaned up."""
    from server.task_cache import TaskResultCache
    
    cache = TaskResultCache(ttl_seconds=1)  # 1 second TTL
    
    # Add 100 entries
    for i in range(100):
        cache.add(f"task-{i}")
    
    assert cache.size() == 100
    
    # Wait for TTL
    time.sleep(2)
    
    # Cleanup
    removed = cache.cleanup_old()
    assert removed == 100
    assert cache.size() == 0


def test_task_results_immediate_cleanup():
    """Test immediate cleanup after result consumed."""
    from server.task_cache import TaskResultCache
    
    cache = TaskResultCache()
    
    # Add and remove
    task_id = "test-task"
    result = cache.add(task_id)
    assert cache.size() == 1
    
    cache.remove(task_id)
    assert cache.size() == 0


def test_bytesio_context_manager():
    """Test BytesIO is closed properly."""
    from transcription.audio import extract_audio_segment
    
    with patch('transcription.audio.ffmpeg') as mock_ffmpeg:
        # Mock ffmpeg output
        mock_ffmpeg.input.return_value.output.return_value.run.return_value = (
            b"fake audio data",
            b""
        )
        
        # Use context manager
        with extract_audio_segment("/test/file.mp4", 0, 30) as audio:
            data = audio.read()
            assert data == b"fake audio data"
            # BytesIO still open here
            assert not audio.closed
        
        # BytesIO should be closed after context
        assert audio.closed


def test_bytesio_closed_on_error():
    """Test BytesIO closed even on error."""
    from transcription.audio import extract_audio_segment, AudioExtractionError
    
    with patch('transcription.audio.ffmpeg') as mock_ffmpeg:
        # Mock ffmpeg error
        mock_ffmpeg.Error = Exception
        mock_ffmpeg.input.return_value.output.return_value.run.side_effect = Exception("FFmpeg failed")
        
        # Should still close BytesIO
        with pytest.raises(AudioExtractionError):
            with extract_audio_segment("/test/file.mp4", 0, 30) as audio:
                pass


@pytest.mark.stress
def test_no_memory_growth_after_1000_requests():
    """
    Stress test: Verify memory stays stable after 1000 requests.
    
    This is the ultimate leak test.
    """
    import tracemalloc
    from server.task_cache import TaskResultCache
    
    # Start memory tracking
    tracemalloc.start()
    gc.collect()
    
    # Get baseline memory
    baseline_memory = tracemalloc.get_traced_memory()[0]
    
    # Simulate 1000 requests
    cache = TaskResultCache(ttl_seconds=60)
    
    for i in range(1000):
        task_id = f"task-{i}"
        
        # Add task result
        result = cache.add(task_id)
        result.set_result(b"x" * 1000)  # 1KB result
        
        # Consume and clean up
        cache.remove(task_id)
        
        # Periodic cleanup
        if i % 100 == 0:
            cache.cleanup_old()
            gc.collect()
    
    # Final cleanup
    cache.cleanup_old()
    gc.collect()
    
    # Get final memory
    final_memory = tracemalloc.get_traced_memory()[0]
    
    # Stop tracking
    tracemalloc.stop()
    
    # Calculate growth
    growth_mb = (final_memory - baseline_memory) / 1024 / 1024
    
    # Memory should not grow more than 10MB
    # (allows for some Python overhead)
    assert growth_mb < 10, f"Memory leak detected: {growth_mb:.2f}MB growth"


@pytest.mark.stress
def test_timer_thread_no_accumulation():
    """Test that timer threads don't accumulate."""
    import threading
    from transcription.model import ModelManager, ModelConfig
    
    config = ModelConfig(cleanup_delay=10)
    manager = ModelManager(config)
    
    with patch('transcription.model.stable_whisper'):
        manager.load()
    
    initial_threads = threading.active_count()
    
    # Schedule and cancel 500 times
    for i in range(500):
        manager.schedule_cleanup()
        manager.cancel_cleanup()
        
        # Periodic GC
        if i % 100 == 0:
            gc.collect()
            time.sleep(0.1)
    
    # Final GC
    gc.collect()
    time.sleep(0.5)
    
    final_threads = threading.active_count()
    growth = final_threads - initial_threads
    
    # Should not accumulate threads
    assert growth < 20, f"Timer thread leak: {growth} threads accumulated"
```

### Integration Tests

```python
@pytest.mark.integration
def test_full_pipeline_no_leaks(sample_video):
    """Test full transcription pipeline doesn't leak."""
    import tracemalloc
    from transcription.engine import TranscriptionEngine
    
    tracemalloc.start()
    baseline = tracemalloc.get_traced_memory()[0]
    
    engine = TranscriptionEngine(config, model_manager)
    
    # Process 10 files
    for i in range(10):
        result = engine.transcribe(
            sample_video,
            "transcribe",
            None,
            TranscribeOptions()
        )
        
        assert result.success
        gc.collect()
    
    final = tracemalloc.get_traced_memory()[0]
    tracemalloc.stop()
    
    growth_mb = (final - baseline) / 1024 / 1024
    
    # Allow 50MB growth (model caching, etc)
    assert growth_mb < 50, f"Memory leak: {growth_mb:.2f}MB"
```

---

## Implementation Checklist

### Leak #1: task_results Dictionary

- [ ] Create `TaskResultCache` class in `worker/server/task_cache.py`
- [ ] Implement `add()`, `get()`, `remove()`, `cleanup_old()` methods
- [ ] Replace global `task_results` dict with `TaskResultCache` instance
- [ ] Update /asr endpoint to use cache
- [ ] Add cleanup after result consumed
- [ ] Add periodic cleanup background thread
- [ ] Add unit tests (5 tests)

### Leak #2: Timer Threads (Validated from STORY_03)

- [ ] Verify `ModelManager.cancel_cleanup()` properly cleans timers
- [ ] Add timer thread counting test
- [ ] Add stress test (500 schedule/cancel cycles)
- [ ] Verify no thread accumulation

### Leak #3: BytesIO Context Managers

- [ ] Convert `extract_audio_segment_to_memory` to context manager
- [ ] Convert `extract_audio_track_to_memory` to context manager
- [ ] Update `detect_language_task` to use context manager
- [ ] Update `handle_multiple_audio_tracks` to use context manager
- [ ] Add unit test for BytesIO closure
- [ ] Add error handling test (BytesIO closed on exception)

---

## Definition of Done

- [ ] All acceptance criteria met
- [ ] All 3 leaks fixed and verified
- [ ] Memory profiling tests added
- [ ] Stress tests passing (1000 requests)
- [ ] Unit tests passing (8+ tests)
- [ ] Integration tests passing
- [ ] Code coverage > 85% for new code
- [ ] Documentation added explaining each leak
- [ ] Work log created with before/after measurements
- [ ] Code committed and pushed

---

## Validation Commands

```bash
# Run leak tests
cd worker
pytest tests/unit/test_memory_leaks.py -v

# Run stress tests (slow)
pytest tests/unit/test_memory_leaks.py -v -m stress

# Memory profiling with tracemalloc
python -m pytest tests/unit/test_memory_leaks.py::test_no_memory_growth_after_1000_requests -v -s

# Visual memory profiling (optional)
pip install memory_profiler
python -m memory_profiler your_script.py
```

---

## Dependencies

**Requires:**
- STORY_01 (gRPC Server) - needs server structure
- STORY_02 (Modular Refactor) - needs audio extraction functions
- STORY_03 (Model Lifecycle) - timer leak fix validated here

**Blocks:**
- None (can be deployed independently)

---

## References

- Legacy code: `subgen.py:234-236` (task_results leak)
- Legacy code: `subgen.py:748-796` (task_results usage)
- Legacy code: `subgen.py:1149-1163` (timer leak)
- Legacy code: `subgen.py:1100-1141` (BytesIO leak)
- Python tracemalloc: https://docs.python.org/3/library/tracemalloc.html
- Context managers: https://docs.python.org/3/library/contextlib.html

---

**Created**: 2026-02-15  
**Last Updated**: 2026-02-15
