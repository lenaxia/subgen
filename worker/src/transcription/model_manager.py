"""
Model lifecycle management for Whisper models.

Handles loading, unloading, and cleanup scheduling with proper
resource management and thread safety.

Extracted from: subgen.py:204-206, 1143-1213

Critical fixes:
- Timer cancellation leak prevention (legacy bug at subgen.py:1149-1163)
- Race condition prevention with proper locking
- Memory leak prevention with proper cleanup
- Thread-safe operations throughout
"""

import logging
import time
import os
import gc
import ctypes
import ctypes.util
from threading import RLock, Timer, Event
from typing import Optional, Any, Callable
from dataclasses import dataclass, field
from types import TracebackType

import stable_whisper
import torch

logger = logging.getLogger(__name__)


@dataclass
class ModelConfig:
    """
    Configuration for Whisper model.

    Attributes:
        model_name: Whisper model name (tiny, base, small, medium, large, distil-*)
        model_path: Path to model storage directory
        device: Device to use (cpu, cuda, gpu)
        cpu_threads: Number of CPU threads for computation
        num_workers: Number of worker threads
        compute_type: Computation type (auto, int8, int8_float16, float16, float32)
        cleanup_delay: Delay in seconds before cleanup (default: 30)
        clear_vram: Whether to clear VRAM on cleanup (default: True)
        unload_on_idle: Whether to unload the model after the cleanup delay.
            None (default) = auto: unload only on CUDA devices. Repeated
            load/unload cycles on CPU fragment the native heap and cause RSS
            to ratchet upward (~20MB+ per cycle); keeping the model resident
            on CPU keeps memory flat. Set True to force unloading on CPU
            (e.g. memory-constrained deployments with sparse jobs), False to
            never unload.
    """

    model_name: str = "medium"
    model_path: str = "./models"
    device: str = "cpu"
    cpu_threads: int = 4
    num_workers: int = 2
    compute_type: str = "auto"
    cleanup_delay: int = 30
    clear_vram: bool = True
    unload_on_idle: Optional[bool] = None


class ModelManager:
    """
    Manages Whisper model lifecycle with lazy loading and cleanup.

    Extracted from: subgen.py:204-206, 1143-1213

    Features:
    - Lazy loading (loads on first use)
    - Scheduled cleanup with configurable delay
    - Thread-safe operations with RLock
    - CUDA cache management
    - Integration with task queue for idle detection
    - Context manager support for automatic cleanup
    - Memory leak prevention with proper timer cleanup

    Usage:
        config = ModelConfig(model_name="medium", device="cuda")
        manager = ModelManager(config)

        # Option 1: Manual lifecycle
        model = manager.load()
        result = model.transcribe(audio)
        manager.schedule_cleanup_if_idle(task_queue)

        # Option 2: Context manager (auto cleanup)
        with manager as model:
            result = model.transcribe(audio)

    Thread Safety:
        All public methods are thread-safe. Multiple threads can safely
        call load(), unload(), schedule_cleanup() etc. simultaneously.
    """

    def __init__(self, config: ModelConfig):
        """
        Initialize ModelManager with ModelConfig.

        Args:
            config: ModelConfig instance with model settings
        """
        self.config = config
        self._model: Optional[Any] = None
        self._lock = RLock()
        self._cleanup_timer: Optional[Timer] = None
        self._cleanup_event = Event()
        self._context_depth = 0  # For nested context managers

        # Optional callback invoked after the model has been unloaded,
        # so owners (e.g. TranscriptionEngine) can drop stale references.
        self.on_unload: Optional[Callable[[], None]] = None

        # Statistics
        self._load_count = 0
        self._cleanup_count = 0
        self._total_cleanup_time = 0.0

        logger.debug(
            f"ModelManager initialized: model={config.model_name}, "
            f"device={config.device}, cleanup_delay={config.cleanup_delay}s, "
            f"unload_on_idle={self._should_unload_on_idle()}"
        )

    def _should_unload_on_idle(self) -> bool:
        """Whether the model should be unloaded after the cleanup delay.

        Auto mode (unload_on_idle=None): only on CUDA devices, where the
        model occupies scarce VRAM. On CPU, load/unload churn fragments the
        native heap and RSS ratchets upward with each cycle.
        """
        if self.config.unload_on_idle is not None:
            return self.config.unload_on_idle
        return self.config.device.lower() in ("cuda", "gpu")

    def load(self) -> Any:
        """
        Load Whisper model if not already loaded.

        Extracted from: subgen.py:1143-1147 (start_model)

        This method is idempotent - calling it multiple times returns
        the same model instance without reloading.

        Returns:
            Loaded stable_whisper model instance

        Raises:
            RuntimeError: If model fails to load

        Thread Safety:
            Safe to call from multiple threads simultaneously.
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
                    compute_type=self.config.compute_type,
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
                raise RuntimeError(f"Model loading failed: {e}") from e

    def unload(self) -> None:
        """
        Unload model and clear VRAM.

        Extracted from: subgen.py:1165-1197 (perform_model_cleanup)

        Performs:
        1. Unload Whisper model from memory
        2. Clear CUDA cache (if GPU)
        3. Run garbage collection
        4. Return memory to OS (Linux only via malloc_trim)

        This method is idempotent - safe to call multiple times.

        Thread Safety:
            Safe to call from multiple threads simultaneously. Model
            reference release is done under the lock; the heavyweight
            cleanup steps (gc.collect / malloc_trim, which can take many
            seconds on a large heap) run OUTSIDE the lock so they never
            block HealthCheck or Transcribe threads.
        """
        with self._lock:
            if self._model is None:
                logger.debug("Model already unloaded, nothing to do")
                return

            logger.info("Unloading Whisper model from memory")
            start_time = time.time()

            # Step 1: Unload Whisper model
            try:
                self._model.model.unload_model()
            except Exception as e:
                logger.error(f"Failed to unload model: {e}")

            # Always clear model reference even if unload failed
            model = self._model
            self._model = None
            del model

        # ---- lock released: heavyweight cleanup must not block others ----

        try:
            # Step 2: Clear CUDA cache if using GPU
            if self.config.device.lower() == "cuda" and torch.cuda.is_available():
                try:
                    torch.cuda.empty_cache()
                    logger.debug("CUDA cache cleared")
                except Exception as e:
                    logger.error(f"Failed to clear CUDA cache: {e}")

            # Step 3: Garbage collection (not on Windows)
            if os.name != "nt":
                gc.collect()

                # Step 4: Return memory to OS (Linux only)
                try:
                    libc = ctypes.CDLL(ctypes.util.find_library("c"))
                    libc.malloc_trim(0)
                    logger.debug("Memory returned to OS via malloc_trim")
                except Exception as e:
                    logger.debug(f"malloc_trim not available: {e}")
        except Exception as e:
            logger.error(f"Error during model cleanup: {e}", exc_info=True)

        cleanup_time = time.time() - start_time
        with self._lock:
            self._cleanup_count += 1
            self._total_cleanup_time += cleanup_time

        logger.info(
            f"Model cleanup completed in {cleanup_time:.2f}s "
            f"(total cleanups: {self._cleanup_count}, "
            f"avg time: {self._total_cleanup_time / self._cleanup_count:.2f}s"
        )

        # Notify owners so they can drop stale model references
        # (e.g. TranscriptionEngine.model) that would otherwise pin
        # the unloaded model in memory.
        if self.on_unload is not None:
            try:
                self.on_unload()
            except Exception as e:
                logger.error(f"on_unload callback failed: {e}")

    def schedule_cleanup(self, delay: Optional[int] = None) -> None:
        """
        Schedule model cleanup after delay.

        Extracted from: subgen.py:1149-1163 (schedule_model_cleanup)

        Properly cancels previous timer to prevent memory leaks.
        This fixes a critical bug in legacy code where cancelled timers
        accumulated without proper cleanup.

        Args:
            delay: Cleanup delay in seconds (uses config default if None)

        Thread Safety:
            Safe to call from multiple threads simultaneously.
        """
        if delay is None:
            delay = self.config.cleanup_delay

        if not self._should_unload_on_idle():
            logger.debug(
                "Idle unload disabled for device=%s (model kept resident to avoid "
                "load/unload memory fragmentation); cancelling any pending cleanup",
                self.config.device,
            )
            self.cancel_cleanup()
            return

        with self._lock:
            # Cancel existing timer (prevents memory leak)
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

        Thread Safety:
            Safe to call from multiple threads simultaneously.
        """
        with self._lock:
            if self._cleanup_timer is not None:
                self._cleanup_timer.cancel()
                self._cleanup_event.set()
                logger.debug("Cleanup timer cancelled")
                self._cleanup_timer = None

    def schedule_cleanup_if_idle(self, task_queue: Any) -> bool:
        """
        Schedule cleanup only if queue is idle.

        Extracted from: subgen.py:1198-1213 (delete_model)

        This prevents unnecessary timer resets when a large batch
        is being processed. Only the last worker finishing the last
        item will trigger cleanup.

        Args:
            task_queue: Queue instance with is_idle() method

        Returns:
            True if cleanup was scheduled, False otherwise

        Thread Safety:
            Safe to call from multiple threads simultaneously.
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
        This prevents cleanup from running if timer was cancelled.
        """
        # Check if we were cancelled
        if self._cleanup_event.is_set():
            logger.debug("Cleanup cancelled, timer exiting")
            return

        logger.debug("Executing scheduled cleanup")
        self.unload()

        # Clear timer reference
        with self._lock:
            self._cleanup_timer = None

    def is_loaded(self) -> bool:
        """
        Check if model is currently loaded.

        Returns:
            True if model is loaded, False otherwise

        Thread Safety:
            Safe to call from multiple threads simultaneously.
        """
        with self._lock:
            return self._model is not None

    def get_model(self) -> Optional[Any]:
        """
        Get currently loaded model.

        Returns:
            Loaded model instance or None if not loaded

        Thread Safety:
            Safe to call from multiple threads simultaneously.
        """
        with self._lock:
            return self._model

    def get_stats(self) -> dict:
        """
        Get model manager statistics.

        Returns:
            Dictionary with load/cleanup counts and timing:
            - model_loaded: bool - whether model is currently loaded
            - load_count: int - total number of loads
            - cleanup_count: int - total number of cleanups
            - avg_cleanup_time: float - average cleanup time in seconds
            - cleanup_scheduled: bool - whether cleanup timer is active

        Thread Safety:
            Safe to call from multiple threads simultaneously.
        """
        with self._lock:
            return {
                "model_loaded": self._model is not None,
                "load_count": self._load_count,
                "cleanup_count": self._cleanup_count,
                "avg_cleanup_time": (
                    self._total_cleanup_time / self._cleanup_count
                    if self._cleanup_count > 0
                    else 0.0
                ),
                "cleanup_scheduled": self._cleanup_timer is not None,
            }

    # Context manager support
    def __enter__(self) -> Any:
        """
        Enter context manager - loads model.

        Supports nested context managers by tracking depth.
        Only the outermost exit will unload the model.

        Returns:
            Loaded model instance
        """
        with self._lock:
            self._context_depth += 1

        return self.load()

    def __exit__(
        self,
        exc_type: Optional[type],
        exc_val: Optional[BaseException],
        exc_tb: Optional[TracebackType],
    ) -> None:
        """
        Exit context manager - unloads model.

        Only unloads on outermost exit (when depth reaches 0).
        Always unloads even if exception occurred.

        Args:
            exc_type: Exception type if exception occurred
            exc_val: Exception value if exception occurred
            exc_tb: Exception traceback if exception occurred
        """
        with self._lock:
            self._context_depth -= 1
            # Decide under the lock; unload() manages its own locking so the
            # heavyweight cleanup runs outside of it.
            should_unload = self._context_depth == 0

        if should_unload:
            self.unload()
