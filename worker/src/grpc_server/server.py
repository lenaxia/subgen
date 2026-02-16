"""
gRPC Server Implementation

Creates and configures the gRPC server with TranscriptionService registered.
"""

import logging
from concurrent import futures

import grpc

from config.settings import WorkerSettings
from grpc_server.service import TranscriptionServicer
from pb import transcription_pb2_grpc


logger = logging.getLogger(__name__)


def create_grpc_server(config: WorkerSettings) -> tuple[grpc.Server, TranscriptionServicer]:
    """
    Create and configure a gRPC server with TranscriptionService.

    Args:
        config: Worker configuration settings

    Returns:
        Tuple of (configured gRPC server instance, servicer instance)
    """
    logger.info("Creating gRPC server with TranscriptionService")

    # Create server with thread pool
    max_workers = config.system.max_workers
    server = grpc.server(
        futures.ThreadPoolExecutor(max_workers=max_workers),
        options=[
            ("grpc.max_send_message_length", 100 * 1024 * 1024),  # 100MB
            ("grpc.max_receive_message_length", 100 * 1024 * 1024),  # 100MB
        ],
    )

    # Create and register TranscriptionService
    servicer = TranscriptionServicer(config)
    transcription_pb2_grpc.add_TranscriptionServiceServicer_to_server(servicer, server)

    logger.info(f"gRPC server created with {max_workers} worker threads")
    logger.info("✅ TranscriptionService registered successfully")

    return server, servicer
