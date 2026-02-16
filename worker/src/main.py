"""
Python Worker gRPC Server Entry Point

This is the main entry point for the Python transcription worker.
The worker listens for gRPC requests from the Go orchestrator and
performs audio transcription using faster-whisper.
"""

import logging
import signal
import sys
import time
from concurrent import futures
from typing import NoReturn

import grpc

from config.settings import get_settings
from grpc_server.server import create_grpc_server
from utils.logging import setup_logging


logger = logging.getLogger(__name__)


def serve() -> NoReturn:
    """Start the gRPC server and block until termination."""
    # Load configuration
    config = get_settings()

    # Setup logging
    setup_logging(config.system.debug)

    logger.info("Starting Python transcription worker")
    logger.info(f"gRPC server will listen on 0.0.0.0:{config.system.grpc_port}")
    logger.info(f"Whisper model: {config.whisper.model_name}")
    logger.info(f"Device: {config.whisper.device}")

    # Create gRPC server and servicer
    server, servicer = create_grpc_server(config)

    # Set servicer start time for uptime calculation
    servicer.start_time = time.time()

    # Bind to port
    server_address = f"0.0.0.0:{config.system.grpc_port}"
    server.add_insecure_port(server_address)

    # Start server
    server.start()
    logger.info(f"✅ gRPC server started successfully on {server_address}")

    # Setup graceful shutdown
    def handle_signal(signum: int, frame: object) -> None:
        logger.info(f"Received signal {signum}, shutting down gracefully...")
        server.stop(grace=30)  # 30 second grace period
        sys.exit(0)

    signal.signal(signal.SIGINT, handle_signal)
    signal.signal(signal.SIGTERM, handle_signal)

    # Block until termination
    try:
        server.wait_for_termination()
    except KeyboardInterrupt:
        logger.info("Interrupted by user, shutting down...")
        server.stop(grace=30)

    # This should never be reached, but satisfy type checker
    sys.exit(0)


def main() -> None:
    """Main entry point."""
    try:
        serve()
    except Exception as e:
        logger.exception(f"Fatal error in main: {e}")
        sys.exit(1)


if __name__ == "__main__":
    main()
