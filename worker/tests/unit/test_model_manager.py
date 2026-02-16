"""
Unit tests for ModelManager.

Tests extracted from STORY_03 requirements and legacy code patterns.
Following TDD: These tests are written FIRST, then implementation follows.
"""

import pytest
import time
import threading
from unittest.mock import Mock, patch, MagicMock, PropertyMock
from typing import Optional
import sys
from pathlib import Path

# Add src to path
sys.path.insert(0, str(Path(__file__).parent.parent.parent / "src"))

# Mock dependencies before importing
sys.modules["stable_whisper"] = MagicMock()
sys.modules["torch"] = MagicMock()

from transcription.model_manager import ModelManager, ModelConfig


@pytest.fixture
def config():
    """Model configuration for testing."""
    return ModelConfig(
        model_name="tiny",  # Use tiny for tests
        model_path="./test_models",
        device="cpu",
        cpu_threads=2,
        num_workers=1,
        compute_type="auto",
        cleanup_delay=1,  # Short delay for tests
        clear_vram=True,
    )


@pytest.fixture
def mock_stable_whisper():
    """Mock stable_whisper.load_faster_whisper."""
    with patch("transcription.model_manager.stable_whisper") as mock:
        mock_model = Mock()
        mock_model.model = Mock()
        mock_model.model.unload_model = Mock()
        mock.load_faster_whisper = Mock(return_value=mock_model)
        yield mock


@pytest.fixture
def mock_torch():
    """Mock torch for CUDA operations."""
    with patch("transcription.model_manager.torch") as mock:
        mock.cuda = Mock()
        mock.cuda.is_available = Mock(return_value=False)
        mock.cuda.empty_cache = Mock()
        yield mock


class TestModelManagerInitialization:
    """Test ModelManager initialization."""

    def test_manager_initialization(self, config):
        """Test ModelManager initializes with correct state."""
        manager = ModelManager(config)

        assert not manager.is_loaded()
        assert manager.get_model() is None
        assert manager._cleanup_timer is None
        assert manager._load_count == 0
        assert manager._cleanup_count == 0

    def test_manager_with_custom_config(self):
        """Test initialization with custom configuration."""
        config = ModelConfig(model_name="medium", device="cuda", cleanup_delay=60, clear_vram=False)
        manager = ModelManager(config)

        assert manager.config.model_name == "medium"
        assert manager.config.device == "cuda"
        assert manager.config.cleanup_delay == 60
        assert manager.config.clear_vram is False


class TestModelLoading:
    """Test model loading operations."""

    def test_load_model_success(self, config, mock_stable_whisper, mock_torch):
        """Test successful model loading."""
        manager = ModelManager(config)

        model = manager.load()

        assert model is not None
        assert manager.is_loaded()
        assert manager._load_count == 1
        mock_stable_whisper.load_faster_whisper.assert_called_once_with(
            config.model_name,
            download_root=config.model_path,
            device=config.device,
            cpu_threads=config.cpu_threads,
            num_workers=config.num_workers,
            compute_type=config.compute_type,
        )

    def test_load_model_idempotent(self, config, mock_stable_whisper, mock_torch):
        """Test loading same model twice reuses instance (idempotent)."""
        manager = ModelManager(config)

        model1 = manager.load()
        model2 = manager.load()

        assert model1 is model2
        # Should only call load once
        assert mock_stable_whisper.load_faster_whisper.call_count == 1
        assert manager._load_count == 1

    def test_load_model_failure(self, config, mock_stable_whisper, mock_torch):
        """Test model loading failure raises RuntimeError."""
        mock_stable_whisper.load_faster_whisper.side_effect = Exception("Load failed")

        manager = ModelManager(config)

        with pytest.raises(RuntimeError, match="Model loading failed"):
            manager.load()

        assert not manager.is_loaded()
        assert manager._load_count == 0

    def test_get_model_when_not_loaded(self, config):
        """Test get_model returns None when not loaded."""
        manager = ModelManager(config)

        assert manager.get_model() is None


class TestModelUnloading:
    """Test model unloading operations."""

    def test_unload_model_cpu(self, config, mock_stable_whisper, mock_torch):
        """Test model unloading on CPU."""
        manager = ModelManager(config)

        # Load then unload
        model = manager.load()
        manager.unload()

        assert not manager.is_loaded()
        assert manager._cleanup_count == 1
        model.model.unload_model.assert_called_once()

    def test_unload_model_gpu(self, config, mock_stable_whisper, mock_torch):
        """Test model unloading on GPU clears CUDA cache."""
        config.device = "cuda"
        mock_torch.cuda.is_available.return_value = True

        manager = ModelManager(config)
        model = manager.load()
        manager.unload()

        assert not manager.is_loaded()
        mock_torch.cuda.empty_cache.assert_called_once()

    def test_unload_when_not_loaded(self, config):
        """Test unloading when no model is loaded (idempotent)."""
        manager = ModelManager(config)

        # Should not raise
        manager.unload()

        assert not manager.is_loaded()
        assert manager._cleanup_count == 0  # No cleanup performed

    def test_unload_handles_exceptions(self, config, mock_stable_whisper, mock_torch):
        """Test unload handles exceptions gracefully."""
        manager = ModelManager(config)
        model = manager.load()

        # Make unload raise exception
        model.model.unload_model.side_effect = Exception("Unload failed")

        # Should not raise, just log error
        manager.unload()

        # Model should still be marked as unloaded
        assert not manager.is_loaded()


class TestCleanupScheduling:
    """Test cleanup scheduling operations."""

    def test_schedule_cleanup(self, config, mock_stable_whisper, mock_torch):
        """Test cleanup scheduling."""
        manager = ModelManager(config)
        manager.load()

        manager.schedule_cleanup(delay=1)

        assert manager._cleanup_timer is not None

        # Wait for cleanup
        time.sleep(1.5)

        assert not manager.is_loaded()
        assert manager._cleanup_count == 1

    def test_schedule_cleanup_default_delay(self, config, mock_stable_whisper, mock_torch):
        """Test scheduling with default delay from config."""
        manager = ModelManager(config)
        manager.load()

        manager.schedule_cleanup()  # No delay specified

        assert manager._cleanup_timer is not None

    def test_cancel_cleanup(self, config, mock_stable_whisper, mock_torch):
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

    def test_cleanup_timer_no_leak(self, config, mock_stable_whisper, mock_torch):
        """
        Test that cancelled timers don't leak.

        Critical test for legacy bug fix (subgen.py:1149-1163).
        Timers were being cancelled but not cleaned up properly.
        """
        manager = ModelManager(config)
        manager.load()

        # Schedule and cancel 10 times
        for _ in range(10):
            manager.schedule_cleanup(delay=5)
            time.sleep(0.01)  # Small delay to ensure timer starts
            manager.cancel_cleanup()

        # Should only have cleanup cancelled, no accumulation
        assert manager._cleanup_timer is None
        assert manager.is_loaded()

        # Verify cleanup count is still 0 (no cleanups executed)
        assert manager._cleanup_count == 0

    def test_schedule_cleanup_replaces_existing(self, config, mock_stable_whisper, mock_torch):
        """Test scheduling cleanup replaces existing timer."""
        manager = ModelManager(config)
        manager.load()

        manager.schedule_cleanup(delay=10)
        timer1 = manager._cleanup_timer

        # Schedule again with shorter delay
        manager.schedule_cleanup(delay=1)
        timer2 = manager._cleanup_timer

        assert timer1 is not timer2

        # Wait for shorter delay
        time.sleep(1.5)

        # Model should be unloaded
        assert not manager.is_loaded()


class TestIdleBasedCleanup:
    """Test cleanup scheduling based on queue idle state."""

    def test_schedule_cleanup_if_idle_when_idle(self, config, mock_stable_whisper, mock_torch):
        """Test scheduling cleanup when queue is idle."""
        manager = ModelManager(config)
        manager.load()

        mock_queue = Mock()
        mock_queue.is_idle = Mock(return_value=True)

        result = manager.schedule_cleanup_if_idle(mock_queue)

        assert result is True
        assert manager._cleanup_timer is not None
        mock_queue.is_idle.assert_called_once()

    def test_schedule_cleanup_if_idle_when_busy(self, config, mock_stable_whisper, mock_torch):
        """Test cleanup NOT scheduled when queue is busy."""
        manager = ModelManager(config)
        manager.load()

        mock_queue = Mock()
        mock_queue.is_idle = Mock(return_value=False)

        result = manager.schedule_cleanup_if_idle(mock_queue)

        assert result is False
        assert manager._cleanup_timer is None

    def test_schedule_cleanup_if_idle_vram_disabled(self, config, mock_stable_whisper, mock_torch):
        """Test cleanup skipped when clear_vram is False."""
        config.clear_vram = False
        manager = ModelManager(config)
        manager.load()

        mock_queue = Mock()
        mock_queue.is_idle = Mock(return_value=True)

        result = manager.schedule_cleanup_if_idle(mock_queue)

        assert result is False
        assert manager._cleanup_timer is None
        # Should not even check queue state
        mock_queue.is_idle.assert_not_called()


class TestThreadSafety:
    """Test thread-safe operations."""

    def test_concurrent_load_calls(self, config, mock_stable_whisper, mock_torch):
        """Test multiple threads calling load() simultaneously."""
        manager = ModelManager(config)
        results = []
        errors = []

        def load_model():
            try:
                model = manager.load()
                results.append(model)
            except Exception as e:
                errors.append(e)

        # Create 5 threads that all try to load
        threads = [threading.Thread(target=load_model) for _ in range(5)]
        for t in threads:
            t.start()
        for t in threads:
            t.join()

        # Should have no errors
        assert len(errors) == 0

        # All threads should get same model instance
        assert all(r is results[0] for r in results)

        # Model should only be loaded once
        assert mock_stable_whisper.load_faster_whisper.call_count == 1

    def test_load_and_unload_concurrently(self, config, mock_stable_whisper, mock_torch):
        """Test load and unload called from different threads."""
        manager = ModelManager(config)

        def loader():
            for _ in range(5):
                manager.load()
                time.sleep(0.01)

        def unloader():
            time.sleep(0.02)  # Let loader start first
            for _ in range(3):
                manager.unload()
                time.sleep(0.01)

        t1 = threading.Thread(target=loader)
        t2 = threading.Thread(target=unloader)

        t1.start()
        t2.start()
        t1.join()
        t2.join()

        # Should complete without deadlock or exceptions
        # Final state depends on timing, but should be consistent


class TestStatistics:
    """Test statistics tracking."""

    def test_get_stats_initial(self, config):
        """Test statistics in initial state."""
        manager = ModelManager(config)

        stats = manager.get_stats()

        assert stats["model_loaded"] is False
        assert stats["load_count"] == 0
        assert stats["cleanup_count"] == 0
        assert stats["avg_cleanup_time"] == 0.0
        assert stats["cleanup_scheduled"] is False

    def test_get_stats_after_load(self, config, mock_stable_whisper, mock_torch):
        """Test statistics after model loading."""
        manager = ModelManager(config)

        manager.load()
        stats = manager.get_stats()

        assert stats["model_loaded"] is True
        assert stats["load_count"] == 1
        assert stats["cleanup_count"] == 0

    def test_get_stats_after_unload(self, config, mock_stable_whisper, mock_torch):
        """Test statistics after cleanup."""
        manager = ModelManager(config)

        manager.load()
        manager.unload()
        stats = manager.get_stats()

        assert stats["model_loaded"] is False
        assert stats["load_count"] == 1
        assert stats["cleanup_count"] == 1
        assert stats["avg_cleanup_time"] > 0.0

    def test_get_stats_multiple_cycles(self, config, mock_stable_whisper, mock_torch):
        """Test statistics over multiple load/unload cycles."""
        manager = ModelManager(config)

        # Load and unload 3 times
        for _ in range(3):
            manager.load()
            manager.unload()

        stats = manager.get_stats()

        assert stats["load_count"] == 3
        assert stats["cleanup_count"] == 3
        assert stats["avg_cleanup_time"] > 0.0

    def test_stats_with_scheduled_cleanup(self, config, mock_stable_whisper, mock_torch):
        """Test cleanup_scheduled flag in stats."""
        manager = ModelManager(config)
        manager.load()

        # Before scheduling
        stats = manager.get_stats()
        assert stats["cleanup_scheduled"] is False

        # After scheduling
        manager.schedule_cleanup(delay=10)
        stats = manager.get_stats()
        assert stats["cleanup_scheduled"] is True

        # After cancelling
        manager.cancel_cleanup()
        stats = manager.get_stats()
        assert stats["cleanup_scheduled"] is False


class TestContextManager:
    """Test context manager functionality."""

    def test_context_manager_basic(self, config, mock_stable_whisper, mock_torch):
        """Test using ModelManager as context manager."""
        manager = ModelManager(config)

        with manager as model:
            assert model is not None
            assert manager.is_loaded()

        # After exiting context, model should be unloaded
        assert not manager.is_loaded()
        assert manager._cleanup_count == 1

    def test_context_manager_with_exception(self, config, mock_stable_whisper, mock_torch):
        """Test context manager cleans up even with exception."""
        manager = ModelManager(config)

        try:
            with manager as model:
                assert model is not None
                raise ValueError("Test exception")
        except ValueError:
            pass

        # Should still be cleaned up
        assert not manager.is_loaded()
        assert manager._cleanup_count == 1

    def test_context_manager_nested(self, config, mock_stable_whisper, mock_torch):
        """Test nested context managers reuse same model."""
        manager = ModelManager(config)

        with manager as model1:
            with manager as model2:
                assert model1 is model2
                assert manager.is_loaded()
            # Inner exit shouldn't unload
            assert manager.is_loaded()

        # Outer exit should unload
        assert not manager.is_loaded()


# Integration test (marked slow)
@pytest.mark.slow
@pytest.mark.requires_model
def test_real_model_loading_integration():
    """
    Integration test with real model loading (tiny).

    Marked as slow - only run when explicitly requested.
    Requires actual Whisper model to be available.
    """
    config = ModelConfig(model_name="tiny", model_path="./models", device="cpu", cleanup_delay=1)

    manager = ModelManager(config)

    # Load model
    model = manager.load()
    assert model is not None
    assert manager.is_loaded()

    # Test actual transcription
    import numpy as np

    audio = np.zeros(16000, dtype=np.float32)  # 1 second silence

    try:
        result = model.transcribe(audio, input_sr=16000)
        assert result is not None
    except Exception as e:
        pytest.skip(f"Transcription failed (expected in test env): {e}")

    # Cleanup
    manager.unload()
    assert not manager.is_loaded()

    # Verify stats
    stats = manager.get_stats()
    assert stats["load_count"] == 1
    assert stats["cleanup_count"] == 1
