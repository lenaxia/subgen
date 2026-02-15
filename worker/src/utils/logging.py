"""
Structured logging setup for the worker.
Uses Python's standard logging module with JSON formatting.
"""

import logging
import sys
from typing import Any


def setup_logging(debug: bool = False) -> None:
    """
    Setup structured logging for the worker.

    Args:
        debug: Enable debug logging if True
    """
    log_level = logging.DEBUG if debug else logging.INFO

    # Create formatter
    formatter = logging.Formatter(
        fmt='{"timestamp": "%(asctime)s", "level": "%(levelname)s", '
        '"logger": "%(name)s", "message": "%(message)s"}',
        datefmt="%Y-%m-%dT%H:%M:%S%z",
    )

    # Configure root logger
    root_logger = logging.getLogger()
    root_logger.setLevel(log_level)

    # Remove existing handlers
    for handler in root_logger.handlers[:]:
        root_logger.removeHandler(handler)

    # Add console handler
    console_handler = logging.StreamHandler(sys.stdout)
    console_handler.setLevel(log_level)
    console_handler.setFormatter(formatter)
    root_logger.addHandler(console_handler)

    # Set log levels for noisy libraries
    logging.getLogger("urllib3").setLevel(logging.WARNING)
    logging.getLogger("grpc").setLevel(logging.INFO)
