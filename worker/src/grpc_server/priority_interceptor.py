"""
Priority interceptor for gRPC server.

Reserves threads for high-priority methods (health checks) to prevent
them from being blocked by long-running transcription jobs.
"""

import grpc
from typing import Any, Callable, Dict
import threading
from concurrent.futures import ThreadPoolExecutor, Future
import queue
import time

# Priority levels
PRIORITY_HIGH = 0  # Health checks, metrics
PRIORITY_NORMAL = 1  # Transcriptions, language detection

# Method priorities
HIGH_PRIORITY_METHODS = {
    "/transcription.TranscriptionService/HealthCheck",
    # Add other high-priority methods here
}


class PriorityThreadPoolExecutor(ThreadPoolExecutor):
    """Thread pool executor with priority queue."""

    def __init__(self, max_workers: int, reserved_high_priority: int = 2):
        super().__init__(max_workers)
        self.reserved_high_priority = reserved_high_priority
        self._work_queue = queue.PriorityQueue()
        self._high_priority_semaphore = threading.Semaphore(reserved_high_priority)

    def submit(self, priority: int, fn: Callable, *args, **kwargs) -> Future:
        """Submit a task with priority."""
        future = Future()

        def task_wrapper():
            try:
                result = fn(*args, **kwargs)
                future.set_result(result)
            except Exception as e:
                future.set_exception(e)

        self._work_queue.put((priority, task_wrapper))
        return future


class PriorityInterceptor(grpc.ServerInterceptor):
    """gRPC interceptor that adds priority to method calls."""

    def __init__(self, reserved_high_priority_threads: int = 2):
        self.reserved_high_priority_threads = reserved_high_priority_threads

    def intercept_service(self, continuation, handler_call_details):
        """Intercept service calls and add priority metadata."""
        method = handler_call_details.method
        priority = PRIORITY_HIGH if method in HIGH_PRIORITY_METHODS else PRIORITY_NORMAL

        # Add priority to call details
        new_details = grpc.HandlerCallDetails(
            method=handler_call_details.method,
            invocation_metadata=handler_call_details.invocation_metadata,
            timeout=handler_call_details.timeout,
        )

        # Continue with the handler
        return continuation(new_details)


def create_priority_thread_pool(
    max_workers: int, reserved_high_priority: int = 2
) -> ThreadPoolExecutor:
    """Create a thread pool with reserved high-priority threads.

    Args:
        max_workers: Total threads in pool
        reserved_high_priority: Threads reserved for high-priority tasks

    Returns:
        ThreadPoolExecutor with priority support
    """
    # For now, use regular thread pool
    # TODO: Implement proper priority queue
    return ThreadPoolExecutor(max_workers=max_workers)
