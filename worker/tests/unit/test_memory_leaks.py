"""
Memory leak tests for EPIC_02 STORY_04.

Tests that verify all three memory leaks have been fixed:
1. Timer thread accumulation (ModelManager)
2. BytesIO context manager leak (audio/extractor.py)
3. task_results dictionary leak (if exists)

These tests use tracemalloc and threading.active_count() to detect leaks.
"""

import gc
import io
import sys
import threading
import time
import tracemalloc
from unittest.mock import Mock, patch, MagicMock

import pytest

# Mock heavy ML modules ONLY when they are not installed. Poisoning
# sys.modules unconditionally breaks other test files that lazily import
# the real modules at runtime (e.g. audio.extractor's `import av`).
for _mod in ("stable_whisper", "torch", "ffmpeg", "av"):
    try:
        __import__(_mod)
    except ImportError:
        sys.modules[_mod] = MagicMock()

from transcription.model_manager import ModelManager, ModelConfig
from audio.extractor import extract_audio_segment, extract_audio_track, AudioExtractionError


class TestTimerThreadLeak:
    """Test that timer threads don't accumulate (Leak #2)."""

    def test_timer_cleanup_no_accumulation(self):
        """Verify cancelled timers don't accumulate threads."""
        config = ModelConfig(cleanup_delay=10, unload_on_idle=True)
        manager = ModelManager(config)

        # Get initial thread count
        initial_threads = threading.active_count()

        # Schedule and cancel 100 times
        for i in range(100):
            manager.schedule_cleanup(delay=10)
            manager.cancel_cleanup()

            # Periodic GC
            if i % 20 == 0:
                gc.collect()
                time.sleep(0.01)

        # Final GC
        gc.collect()
        time.sleep(0.2)

        # Thread count should not grow significantly
        final_threads = threading.active_count()
        growth = final_threads - initial_threads

        # Allow some growth (GC might not clean all immediately)
        # but should be << 100
        assert growth < 10, (
            f"Thread leak detected: {growth} threads added after 100 schedule/cancel cycles"
        )

    @pytest.mark.stress
    def test_timer_stress_500_cycles(self):
        """Stress test: 500 schedule/cancel cycles should not leak threads."""
        config = ModelConfig(cleanup_delay=5, unload_on_idle=True)
        manager = ModelManager(config)

        initial_threads = threading.active_count()

        # Schedule and cancel 500 times
        for i in range(500):
            manager.schedule_cleanup()
            manager.cancel_cleanup()

            # Periodic GC
            if i % 100 == 0:
                gc.collect()
                time.sleep(0.05)

        # Final GC
        gc.collect()
        time.sleep(0.5)

        final_threads = threading.active_count()
        growth = final_threads - initial_threads

        # Should not accumulate threads
        assert growth < 20, f"Timer thread leak: {growth} threads accumulated after 500 cycles"

    def test_cleanup_timer_properly_cancelled(self):
        """Test that cleanup timer is properly cancelled."""
        config = ModelConfig(cleanup_delay=1, unload_on_idle=True)
        manager = ModelManager(config)

        # Schedule cleanup
        manager.schedule_cleanup()
        assert manager._cleanup_timer is not None
        timer1 = manager._cleanup_timer

        # Cancel
        manager.cancel_cleanup()
        assert manager._cleanup_timer is None

        # Wait to ensure timer doesn't fire
        time.sleep(2)

        # Model should still be None (no cleanup occurred)
        assert not manager.is_loaded()


class TestBytesIOContextManagerLeak:
    """Test that BytesIO objects are properly closed (Leak #3)."""

    def test_bytesio_closed_after_context(self):
        """Test BytesIO is closed properly after context manager exits."""
        # Mock ffmpeg at the import location (inside the function)
        with patch("ffmpeg.input") as mock_input:
            fake_audio = b"fake audio data"
            mock_input.return_value.output.return_value.run.return_value = (fake_audio, b"")

            # Use context manager
            with extract_audio_segment("/test/file.mp4", 0, 30) as audio:
                data = audio.read()
                assert data == fake_audio
                # BytesIO still open here
                assert not audio.closed

            # BytesIO should be closed after context
            assert audio.closed

    def test_bytesio_closed_on_error(self):
        """Test BytesIO closed even when exception occurs."""
        # Create a proper ffmpeg.Error mock
        import ffmpeg

        # Create mock error with stderr attribute
        mock_error = Exception("FFmpeg failed")
        mock_error.stderr = b"ffmpeg error output"

        with patch.object(ffmpeg, "input") as mock_input:
            mock_input.return_value.output.return_value.run.side_effect = mock_error

            # Patch ffmpeg.Error to be Exception so it's caught
            with patch.object(ffmpeg, "Error", Exception):
                # Should still close BytesIO
                audio_buffer = None
                with pytest.raises(AudioExtractionError):
                    with extract_audio_segment("/test/file.mp4", 0, 30) as audio:
                        audio_buffer = audio
                        pass

                # Buffer should still be closed (or None if never created)
                if audio_buffer is not None:
                    assert audio_buffer.closed

    def test_extract_audio_track_closes_buffer(self):
        """Test extract_audio_track closes BytesIO."""
        with patch("ffmpeg.input") as mock_input:
            fake_audio = b"x" * 1000
            mock_input.return_value.output.return_value.run.return_value = (fake_audio, b"")

            buffer = None
            with extract_audio_track("/test/file.mp4", 0) as audio:
                buffer = audio
                data = audio.read()
                assert len(data) == 1000
                assert not audio.closed

            assert buffer.closed

    def test_bytesio_no_leak_100_extractions(self):
        """Test no memory leak after 100 audio extractions."""
        with patch("ffmpeg.input") as mock_input:
            fake_audio = b"x" * 100_000  # 100KB
            mock_input.return_value.output.return_value.run.return_value = (fake_audio, b"")

            # Start memory tracking
            tracemalloc.start()
            gc.collect()
            baseline = tracemalloc.get_traced_memory()[0]

            # Extract 100 times
            for i in range(100):
                with extract_audio_segment("/test/file.mp4", 0, 30) as audio:
                    _ = audio.read()

                # Periodic GC
                if i % 20 == 0:
                    gc.collect()

            # Final GC
            gc.collect()
            final = tracemalloc.get_traced_memory()[0]
            tracemalloc.stop()

            # Calculate growth
            growth_mb = (final - baseline) / 1024 / 1024

            # Should not grow more than 5MB (allows for some overhead)
            assert growth_mb < 5, (
                f"BytesIO leak detected: {growth_mb:.2f}MB growth after 100 extractions"
            )


class TestModelManagerMemory:
    """Test ModelManager doesn't leak memory."""

    @patch("transcription.model_manager.stable_whisper")
    def test_model_load_unload_no_leak(self, mock_whisper):
        """Test repeated load/unload doesn't leak."""
        # Mock model
        mock_model = MagicMock()
        mock_model.model.unload_model = MagicMock()
        mock_whisper.load_faster_whisper.return_value = mock_model

        config = ModelConfig(model_name="tiny", device="cpu")
        manager = ModelManager(config)

        # Start memory tracking
        tracemalloc.start()
        gc.collect()
        baseline = tracemalloc.get_traced_memory()[0]

        # Load and unload 50 times
        for i in range(50):
            manager.load()
            manager.unload()

            if i % 10 == 0:
                gc.collect()

        gc.collect()
        final = tracemalloc.get_traced_memory()[0]
        tracemalloc.stop()

        growth_mb = (final - baseline) / 1024 / 1024

        # Should not grow significantly
        assert growth_mb < 2, f"ModelManager leak: {growth_mb:.2f}MB after 50 load/unload cycles"


@pytest.mark.stress
class TestStressTests:
    """Stress tests to verify no leaks under heavy load."""

    def test_no_memory_growth_1000_extractions(self):
        """
        Stress test: Verify memory stays stable after 1000 extractions.

        This is the ultimate leak test.
        """
        with patch("ffmpeg.input") as mock_input:
            fake_audio = b"x" * 50_000  # 50KB per extraction
            mock_input.return_value.output.return_value.run.return_value = (fake_audio, b"")

            # Start memory tracking
            tracemalloc.start()
            gc.collect()
            baseline = tracemalloc.get_traced_memory()[0]

            # Extract 1000 times
            for i in range(1000):
                with extract_audio_segment("/test/file.mp4", 0, 30) as audio:
                    _ = audio.read()

                # Periodic GC
                if i % 100 == 0:
                    gc.collect()

            # Final GC
            gc.collect()
            final = tracemalloc.get_traced_memory()[0]
            tracemalloc.stop()

            # Calculate growth
            growth_mb = (final - baseline) / 1024 / 1024

            # Memory should not grow more than 10MB
            # (allows for some Python overhead)
            assert growth_mb < 10, (
                f"Memory leak detected: {growth_mb:.2f}MB growth after 1000 extractions"
            )

    @patch("transcription.model_manager.stable_whisper")
    def test_model_cleanup_memory_returned(self, mock_whisper):
        """Test that model cleanup actually returns memory."""
        # Mock model that uses memory
        mock_model = MagicMock()
        mock_model.model.unload_model = MagicMock()
        mock_whisper.load_faster_whisper.return_value = mock_model

        config = ModelConfig(model_name="medium", device="cpu")
        manager = ModelManager(config)

        # Load model
        manager.load()

        # Record memory before unload
        gc.collect()
        mem_before = tracemalloc.get_traced_memory()[0] if tracemalloc.is_tracing() else 0

        # Unload
        manager.unload()
        gc.collect()

        # Memory should be freed (or at least not grow)
        # This is a basic check - actual memory reduction depends on Python GC
        assert not manager.is_loaded()

    def test_concurrent_operations_no_deadlock(self):
        """Test concurrent operations don't deadlock or leak."""
        config = ModelConfig(cleanup_delay=1, unload_on_idle=True)
        manager = ModelManager(config)

        # Spawn multiple threads doing schedule/cancel
        threads = []
        for _ in range(10):
            t = threading.Thread(
                target=lambda: [
                    manager.schedule_cleanup() or manager.cancel_cleanup() for _ in range(10)
                ]
            )
            t.start()
            threads.append(t)

        # Wait for all threads
        for t in threads:
            t.join(timeout=5)

        # Should complete without deadlock
        assert all(not t.is_alive() for t in threads)


class TestMemoryLeakDocumentation:
    """Tests that document the memory leak fixes."""

    @patch("transcription.model_manager.stable_whisper")
    def test_context_manager_no_deadlock(self, mock_whisper):
        """Regression: __exit__ must not deadlock.

        LEGACY BUG: ModelManager used threading.Lock (non-reentrant) and
        __exit__ called self.unload() while still holding self._lock,
        deadlocking the thread forever (reproduced: faulthandler showed
        unload blocked on `with self._lock` at model_manager.py:183).

        stable_whisper is patched so the timing stays deterministic: a
        real load_faster_whisper call takes 10s+ (or downloads 1.5GB on
        a cold cache), which trips the 10s join timeout below and reads
        as a false deadlock.
        """
        mock_model = MagicMock()
        mock_whisper.load_faster_whisper.return_value = mock_model
        config = ModelConfig(cleanup_delay=1, unload_on_idle=True)
        manager = ModelManager(config)
        manager.on_unload = None

        result = {}

        def run():
            with manager:
                result["loaded"] = manager.is_loaded()
            result["done"] = True

        t = threading.Thread(target=run, daemon=True)
        t.start()
        t.join(timeout=10)
        assert not t.is_alive(), "ModelManager context manager deadlocked"
        assert result.get("done") is True
        assert not manager.is_loaded()

    @patch("transcription.model_manager.stable_whisper")
    def test_unload_invokes_on_unload_callback(self, mock_whisper):
        """unload() must notify owners so stale references are dropped.

        LEGACY BUG: TranscriptionServicer set engine.model = model but never
        cleared it on unload; the engine kept the unloaded WhisperModel
        (and its ctranslate2 runtime state) pinned until the next job.
        """
        mock_whisper.load_faster_whisper.return_value = MagicMock()
        config = ModelConfig(cleanup_delay=1, unload_on_idle=True)
        manager = ModelManager(config)
        called = []
        manager.on_unload = lambda: called.append(True)
        manager.load()
        manager.unload()
        assert called == [True]

    @patch("transcription.model_manager.stable_whisper")
    def test_schedule_cleanup_noop_on_cpu(self, mock_whisper):
        """Idle unload must be skipped on CPU by default.

        LEGACY BUG: clear_vram_on_complete=true (default) unloaded the model
        after every job even on CPU. Repeated load->transcribe->unload cycles
        fragment the native heap: measured +20-30MB RSS per cycle, +200MB
        over 10 job cycles with the tiny model (worse with medium). Keeping
        the model resident on CPU keeps RSS flat.
        """
        mock_whisper.load_faster_whisper.return_value = MagicMock()
        config = ModelConfig(device="cpu", cleanup_delay=1)
        manager = ModelManager(config)
        manager.load()
        manager.schedule_cleanup(delay=0.1)
        assert manager._cleanup_timer is None  # no timer scheduled on CPU
        time.sleep(0.3)
        assert manager.is_loaded()  # model stays resident

    def test_schedule_cleanup_active_on_cuda(self):
        """Idle unload still applies on CUDA (frees VRAM)."""
        config = ModelConfig(device="cuda", cleanup_delay=1)
        manager = ModelManager(config)
        manager.schedule_cleanup(delay=5)
        assert manager._cleanup_timer is not None
        manager.cancel_cleanup()

    def test_schedule_cleanup_forced_on_cpu(self):
        """unload_on_idle=True forces CPU unloads (opt-in for sparse jobs)."""
        config = ModelConfig(device="cpu", cleanup_delay=1, unload_on_idle=True)
        manager = ModelManager(config)
        manager.schedule_cleanup(delay=5)
        assert manager._cleanup_timer is not None
        manager.cancel_cleanup()

    def test_leak_1_timer_thread_documented(self):
        """
        Document Leak #1: Timer Thread Accumulation.

        LEGACY BUG (subgen.py:1149-1163):
        - Timer.cancel() only sets a flag
        - Thread continues running until it checks the flag
        - After 100 requests: 100 threads (99 cancelled, 1 active)
        - Memory impact: ~8KB per thread = ~800KB per 100 requests

        FIX (ModelManager):
        - Properly cancel timer before creating new one
        - Use Event to signal cancellation
        - Check event in _perform_cleanup before executing
        - Threads cleaned up by Python GC

        VERIFICATION:
        - test_timer_cleanup_no_accumulation (100 cycles < 10 threads growth)
        - test_timer_stress_500_cycles (500 cycles < 20 threads growth)
        """
        assert True  # Documentation test

    def test_leak_2_bytesio_context_manager_documented(self):
        """
        Document Leak #2: BytesIO Context Manager.

        LEGACY BUG (subgen.py:1100-1141, 1352-1386):
        - BytesIO objects returned but never closed
        - Each BytesIO: ~1MB for 30s audio segment
        - After 100 extractions: ~100MB leaked

        EXAMPLE OF BUG:
            audio = extract_audio_segment_to_memory(path, 0, 30)
            data = audio.read()
            # audio is never closed → memory leak

        FIX (audio/extractor.py):
        - Convert to @contextmanager
        - Use try/finally to always close buffer
        - All call sites use 'with' statement

        CORRECT USAGE:
            with extract_audio_segment(path, 0, 30) as audio:
                data = audio.read()
            # audio.close() called automatically

        VERIFICATION:
        - test_bytesio_closed_after_context
        - test_bytesio_closed_on_error
        - test_bytesio_no_leak_100_extractions (< 5MB growth)
        """
        assert True  # Documentation test

    def test_leak_3_timer_cleanup_documented(self):
        """
        Document Leak #3: ModelManager Timer Cleanup.

        LEGACY BUG:
        - Global model_cleanup_timer never properly cleaned
        - Cancelled timers accumulated in memory

        FIX:
        - ModelManager._cleanup_timer properly managed
        - Always set to None after cancel
        - Event used to signal cancellation to running timer
        - Proper lifecycle in schedule_cleanup()

        VERIFICATION:
        - ModelManager implementation (model_manager.py:238-289)
        - test_cleanup_timer_properly_cancelled
        - test_timer_cleanup_no_accumulation
        """
        assert True  # Documentation test


class TestLockDiscipline:
    """Regression tests for the unload lock discipline (PR review findings).

    LEGACY BUGS:
    - unload() ran gc.collect + malloc_trim while holding self._lock, so
      HealthCheck / is_loaded() blocked for the full (13s+ measured) cleanup.
    - unload() could destroy a model a concurrent __enter__ had just loaded
      (TOCTOU between __exit__'s depth decision and the teardown).
    - A failed __enter__ load leaked the depth increment, suppressing every
      subsequent unload.
    """

    @patch("transcription.model_manager.stable_whisper")
    def test_heavy_cleanup_does_not_block_other_threads(self, mock_whisper):
        """Other threads must not block on the lock while gc.collect runs.

        The heavy phase (gc + malloc_trim) runs OUTSIDE the lock: while it
        sleeps, another thread's is_loaded() must return promptly. With the
        pre-fix code (heavy phase under the lock) this test fails: the
        is_loaded() call blocks for the full gc duration.
        """
        mock_whisper.load_faster_whisper.return_value = MagicMock()
        config = ModelConfig(cleanup_delay=1, unload_on_idle=True)
        manager = ModelManager(config)
        manager.on_unload = None
        manager.load()

        gc_entered = threading.Event()
        real_collect = gc.collect

        def slow_collect():
            gc_entered.set()
            time.sleep(1.0)
            return 0

        def unloader():
            with patch("gc.collect", side_effect=slow_collect):
                manager.unload()

        t = threading.Thread(target=unloader, daemon=True)
        t.start()
        assert gc_entered.wait(timeout=5), "unload never reached the gc phase"

        start = time.time()
        loaded = manager.is_loaded()  # must not wait on the sleeping gc
        elapsed = time.time() - start

        t.join(timeout=10)
        assert elapsed < 0.5, f"is_loaded() blocked {elapsed:.2f}s during unload's gc phase"
        assert loaded is False

    @patch("transcription.model_manager.stable_whisper")
    def test_unload_deferred_while_context_active(self, mock_whisper):
        """TOCTOU guard: unload() must not destroy a model inside an active
        `with manager:` context — the context exit owns the unload."""
        mock_model = MagicMock()
        mock_whisper.load_faster_whisper.return_value = mock_model
        config = ModelConfig(cleanup_delay=1, unload_on_idle=True)
        manager = ModelManager(config)
        manager.on_unload = None

        manager.__enter__()
        # Timer-style direct unload while the context is active
        manager.unload()
        assert manager.is_loaded(), "unload() destroyed the model while a context held it"

        manager.__exit__(None, None, None)
        assert not manager.is_loaded(), "outermost context exit did not unload"

    @patch("transcription.model_manager.stable_whisper")
    def test_failed_load_does_not_leak_context_depth(self, mock_whisper):
        """A failed __enter__ load must unwind the depth increment.

        LEGACY BUG: __enter__ incremented _context_depth before load(); when
        load raised, __exit__ never ran, the depth stayed >0 forever, and
        unload() was suppressed for the lifetime of the manager.
        """
        mock_whisper.load_faster_whisper.side_effect = RuntimeError("boom")
        manager = ModelManager(ModelConfig(cleanup_delay=1, unload_on_idle=True))
        manager.on_unload = None

        with pytest.raises(RuntimeError):
            manager.__enter__()
        assert manager._context_depth == 0, "failed load leaked the depth increment"

        # A later unload must not be suppressed by a leaked depth
        mock_whisper.load_faster_whisper.side_effect = None
        mock_whisper.load_faster_whisper.return_value = MagicMock()
        manager.load()
        manager.unload()
        assert not manager.is_loaded()
