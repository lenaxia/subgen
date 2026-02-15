"""
gRPC Server Implementation Stub

This is a stub implementation of the gRPC server that allows the worker
to start without crashing. The actual gRPC service implementation will
be added in EPIC_02 STORY_02.

Status: STUB - Functional but not implementing actual transcription service yet
"""

import logging
from concurrent import futures

import grpc

from config.settings import WorkerSettings


logger = logging.getLogger(__name__)


def create_grpc_server(config: WorkerSettings) -> grpc.Server:
    """
    Create and configure a gRPC server.

    This is a stub implementation that creates a basic gRPC server
    without any registered services. The actual TranscriptionService
    will be added in EPIC_02 STORY_02.

    Args:
        config: Worker configuration settings

    Returns:
        Configured gRPC server instance
    """
    logger.info("Creating gRPC server (stub implementation)")

    # Create server with thread pool
    max_workers = config.whisper_threads * 2  # 2x threads for I/O + compute
    server = grpc.server(
        futures.ThreadPoolExecutor(max_workers=max_workers),
        options=[
            ("grpc.max_send_message_length", 100 * 1024 * 1024),  # 100MB
            ("grpc.max_receive_message_length", 100 * 1024 * 1024),  # 100MB
        ],
    )

    logger.info(f"gRPC server created with {max_workers} worker threads")
    logger.warning("⚠️  No services registered yet - stub implementation")
    logger.warning("⚠️  Actual TranscriptionService will be added in STORY_02")

    return server
